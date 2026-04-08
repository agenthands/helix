package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	serenav1 "github.com/postfix/serena/api/proto/serena/v1"
	"github.com/postfix/serena/internal/config"
	"github.com/postfix/serena/internal/kernel"
	"github.com/postfix/serena/internal/kernel/diag"
	"github.com/postfix/serena/internal/kernel/edit"
	"github.com/postfix/serena/internal/kernel/fileops"
	"github.com/postfix/serena/internal/kernel/lspool"
	"github.com/postfix/serena/internal/kernel/symbols"
	"github.com/postfix/serena/internal/langregistry"
	serenaMCP "github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/profile"
	"github.com/postfix/serena/internal/skill"
	"github.com/postfix/serena/internal/workspace"
)

// SkillToolExecutor is implemented by skills that support direct tool execution
// (memory, workflow). Kernel skill adapters skip this and register via RegisterTools.
type SkillToolExecutor interface {
	ExecuteTool(name string, args map[string]interface{}) (string, error)
}

// daemonSessionProvider is a minimal SessionProvider for the profile skill.
type daemonSessionProvider struct {
	session *serenaMCP.SessionInfo
}

func (p *daemonSessionProvider) CurrentSession() *serenaMCP.SessionInfo {
	return p.session
}

// Daemon is the persistent supervisor process (DMN-01).
// It manages workspace registry, MCP server, kernel, skills, listeners,
// and survives client disconnects (DMN-02).
type Daemon struct {
	config         *config.SerenaConfig
	logger         *slog.Logger
	workspaces     *workspace.Registry
	mcpServer      *serenaMCP.SerenaMCPServer
	grpcServer     *grpc.Server
	socketListener net.Listener
	kernel         *kernel.Kernel
	langRegistry   *langregistry.Registry
	profileStore   *profile.ProfileStore
	activeProfile  *profile.Profile
	diagStore      *diag.DiagnosticStore
	bodyExtractor  *edit.BodyExtractor
}

// New creates a new Daemon with the given config and logger.
// Fail-fast for core subsystems per D-06; degrade gracefully for optional ones per D-07.
func New(cfg *config.SerenaConfig, logger *slog.Logger) (*Daemon, error) {
	workspaces := workspace.NewRegistry()

	// 1. Language registry (fail-fast).
	langReg, err := langregistry.NewRegistry()
	if err != nil {
		return nil, fmt.Errorf("creating language registry: %w", err)
	}

	// 2. Installer for three-tier LS resolution.
	homeDir, _ := os.UserHomeDir()
	installer := langregistry.NewInstaller(langregistry.InstallerConfig{
		AutoInstall: true,
		BinDir:      filepath.Join(homeDir, ".serena", "bin"),
	}, logger)

	// 3. Memory pressure (platform-specific).
	pressure := newPlatformPressure()

	// 4. Convert WorkerPoolConfig to PoolConfig.
	poolCfg := lspool.PoolConfig{
		BaseTTL:               cfg.WorkerPool.BaseTTL,
		CeilingTTL:            cfg.WorkerPool.CeilingTTL,
		MaxWorkers:            cfg.WorkerPool.MaxWorkers,
		RSSHardCapMB:          cfg.WorkerPool.RSSHardCapMB,
		PressureCheckInterval: cfg.WorkerPool.PressureCheckInterval,
	}
	if poolCfg.BaseTTL == 0 {
		poolCfg = lspool.DefaultPoolConfig()
	}

	// 5. Create kernel (fail-fast).
	k := kernel.NewKernel(workspaces, langReg, installer, kernel.KernelConfig{Pool: poolCfg}, pressure, logger)

	// 6. Create diagnostic store and body extractor.
	diagStore := diag.NewDiagnosticStore()
	bodyExtractor := edit.NewBodyExtractor()

	// 7. Create MCP server.
	mcpServer := serenaMCP.NewSerenaMCPServer(workspaces, logger)

	// 8. Resolve profile per D-08.
	globalDir := filepath.Join(homeDir, ".serena")
	profileStore, activeProfile, err := config.ResolveProfile(cfg, globalDir)
	if err != nil {
		return nil, fmt.Errorf("resolving profile: %w", err)
	}
	logger.Info("profile resolved",
		"profile", cfg.Profile,
		"default_mode", activeProfile.DefaultMode,
	)

	// 9. Initialize skills per D-07 (degraded mode for optional providers).
	skillDeps := skill.SkillDeps{
		ProjectDir: filepath.Join(globalDir, "default-project"),
		GlobalDir:  globalDir,
		Logger:     logger,
	}
	if err := skill.InitAll(skillDeps); err != nil {
		logger.Warn("skill initialization partially failed, continuing in degraded mode", "error", err)
	}

	// 10. Register kernel tools with MCP server.
	var activeWSKey workspace.WorkspaceKey
	wsKeyFn := func() workspace.WorkspaceKey { return activeWSKey }
	workspaceRootFn := func() string { return activeWSKey.RepoRoot }

	symbols.RegisterTools(mcpServer, k, wsKeyFn)
	edit.RegisterTools(mcpServer, k, bodyExtractor, diagStore, wsKeyFn)
	fileops.RegisterTools(mcpServer, workspaceRootFn)

	// Diag lease provider.
	leaseFn := func(ctx context.Context, uri string) (*lspool.WorkerLease, error) {
		return k.Pool().AcquireLease(ctx, "diag-"+uri, activeWSKey, false)
	}
	diag.RegisterTools(mcpServer, diagStore, workspaceRootFn, leaseFn)

	// 11. Register skill-provided tools with MCP SDK.
	for _, tp := range skill.ToolProviders() {
		// Skip kernel skill adapters (already registered via RegisterTools above).
		switch tp.Name() {
		case "symbol-retrieval", "symbol-editing", "file-ops", "diagnostics":
			continue
		}
		registerSkillTools(mcpServer, tp, logger)
	}

	// 12. Wire profile skill session provider.
	defaultMode := activeProfile.DefaultMode
	if defaultMode == "" {
		defaultMode = "edit"
	}
	sessionProvider := &daemonSessionProvider{
		session: &serenaMCP.SessionInfo{
			Profile: cfg.Profile,
			Mode:    defaultMode,
		},
	}
	if ps := profile.GetProfileSkill(); ps != nil {
		ps.SetSessionProvider(sessionProvider)
	}

	// 13. Install ProfileFilterMiddleware on the MCP server.
	getSessionFn := func(ctx context.Context) *serenaMCP.SessionInfo {
		return sessionProvider.CurrentSession()
	}
	mcpServer.SDK().AddReceivingMiddleware(
		serenaMCP.ProfileFilterMiddleware(profileStore, getSessionFn, logger),
	)

	// 14. Update activate_project to also activate workspace in kernel.
	mcpServer.SetActivateCallback(func(ctx context.Context, repoPath string) error {
		rt, err := k.ActivateWorkspace(ctx, repoPath)
		if err != nil {
			return err
		}
		activeWSKey = workspace.WorkspaceKey{RepoRoot: repoPath}
		logger.Info("kernel workspace activated",
			"root", repoPath,
			"languages", rt.Languages(),
		)
		return nil
	})

	return &Daemon{
		config:        cfg,
		logger:        logger,
		workspaces:    workspaces,
		mcpServer:     mcpServer,
		kernel:        k,
		langRegistry:  langReg,
		profileStore:  profileStore,
		activeProfile: activeProfile,
		diagStore:     diagStore,
		bodyExtractor: bodyExtractor,
	}, nil
}

// MCPServer returns the MCP server for test wiring (e.g., HTTPHandler, SDK().Connect).
func (d *Daemon) MCPServer() *serenaMCP.SerenaMCPServer { return d.mcpServer }

// KernelInstance returns the kernel for lifecycle management in tests.
func (d *Daemon) KernelInstance() *kernel.Kernel { return d.kernel }

// registerSkillTools registers all tools from a ToolProvider with the MCP server.
// Skills with ExecuteTool (memory, workflow) get live handlers; others are catalog-only.
func registerSkillTools(server *serenaMCP.SerenaMCPServer, tp skill.ToolProvider, logger *slog.Logger) {
	executor, hasExecutor := tp.(SkillToolExecutor)
	for _, td := range tp.Tools() {
		if hasExecutor {
			server.AddSkillTool(td.Name, td.Description, executor)
		} else {
			// Catalog-only registration (e.g., profile skill with custom Execute* methods).
			server.Registry().Register(&serenaMCP.ToolDef{
				Name:        td.Name,
				Description: td.Description,
			})
		}
	}
	logger.Info("skill tools registered", "skill", tp.Name(), "tools", len(tp.Tools()), "has_executor", hasExecutor)
}

// Run starts the daemon and blocks until shutdown (DMN-01, DMN-12).
// Signal handlers are registered FIRST per Pitfall 3.
func (d *Daemon) Run(ctx context.Context) error {
	// Register signal handlers FIRST (Pitfall 3: before any goroutine starts)
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// Check for stale socket and clean up (DMN-13)
	if err := ensureSocket(d.config.Daemon.SocketPath); err != nil {
		return fmt.Errorf("socket check: %w", err)
	}
	if err := createSocketDir(d.config.Daemon.SocketPath); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}

	g, gctx := errgroup.WithContext(ctx)

	// Kernel runs in the errgroup (manages LS worker pool lifecycle).
	g.Go(func() error {
		return d.kernel.Run(gctx)
	})

	// Unix socket listener for forwarder connections via gRPC (DMN-03)
	g.Go(func() error {
		return d.listenSocket(gctx)
	})

	// HTTP listener for Streamable HTTP MCP (DMN-04, MCP-02)
	if d.config.Daemon.HTTPAddr != "" {
		g.Go(func() error {
			return d.listenHTTP(gctx)
		})
	}

	d.logger.Info("daemon started",
		"socket", d.config.Daemon.SocketPath,
		"http_addr", d.config.Daemon.HTTPAddr,
		"workspaces", d.workspaces.WorkspaceCount(),
	)

	err := g.Wait()
	d.shutdown()
	return err
}

// listenSocket starts the Unix domain socket listener with a gRPC server.
func (d *Daemon) listenSocket(ctx context.Context) error {
	ln, err := net.Listen("unix", d.config.Daemon.SocketPath)
	if err != nil {
		return fmt.Errorf("listen unix %s: %w", d.config.Daemon.SocketPath, err)
	}
	d.socketListener = ln
	d.logger.Info("unix socket listener started", "path", d.config.Daemon.SocketPath)

	// Create and register gRPC server
	d.grpcServer = grpc.NewServer()
	serenav1.RegisterForwarderServiceServer(d.grpcServer, &forwarderServiceHandler{
		mcpServer: d.mcpServer,
		logger:    d.logger,
	})

	// Serve in a goroutine so we can wait for context cancellation
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- d.grpcServer.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		d.grpcServer.GracefulStop()
		return ctx.Err()
	case err := <-serveDone:
		return err
	}
}

// listenHTTP starts the Streamable HTTP listener for MCP (DMN-04, MCP-02).
func (d *Daemon) listenHTTP(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/mcp", d.mcpServer.HTTPHandler())

	server := &http.Server{
		Addr:    d.config.Daemon.HTTPAddr,
		Handler: mux,
	}

	d.logger.Info("HTTP listener started", "addr", d.config.Daemon.HTTPAddr)

	// Serve in a goroutine
	serveDone := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveDone <- err
		}
		close(serveDone)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-serveDone:
		return err
	}
}

// Workspaces returns the workspace registry (for use by MCP tools).
func (d *Daemon) Workspaces() *workspace.Registry {
	return d.workspaces
}

// forwarderServiceHandler implements the gRPC ForwarderService.
type forwarderServiceHandler struct {
	serenav1.UnimplementedForwarderServiceServer
	mcpServer *serenaMCP.SerenaMCPServer
	logger    *slog.Logger
}

// StreamMCP handles a bidirectional MCP stream from a forwarder.
func (h *forwarderServiceHandler) StreamMCP(stream serenav1.ForwarderService_StreamMCPServer) error {
	// Read the first message to get the session ID
	firstMsg, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receiving first message: %w", err)
	}

	sessionID := firstMsg.SessionId
	h.logger.Info("new forwarder stream", "session_id", sessionID)

	// Create a GRPCTransport that bridges this stream to the MCP SDK.
	// Pass firstMsg so it gets replayed into the transport pipe.
	transport := serenaMCP.NewGRPCTransport(stream, sessionID, firstMsg)

	// Connect the MCP server to this transport
	session, err := h.mcpServer.SDK().Connect(stream.Context(), transport, nil)
	if err != nil {
		return fmt.Errorf("connecting MCP session: %w", err)
	}

	// Wait for the session to end
	err = session.Wait()
	h.logger.Info("forwarder stream ended", "session_id", sessionID)
	return err
}
