package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

	serenav1 "github.com/postfix/serena/api/proto/serena/v1"
	"github.com/postfix/serena/internal/config"
	"github.com/postfix/serena/internal/degrade"
	"github.com/postfix/serena/internal/kernel"
	"github.com/postfix/serena/internal/kernel/diag"
	"github.com/postfix/serena/internal/kernel/edit"
	"github.com/postfix/serena/internal/kernel/fileops"
	"github.com/postfix/serena/internal/kernel/lspool"
	"github.com/postfix/serena/internal/kernel/symbols"
	"github.com/postfix/serena/internal/langregistry"
	serenaMCP "github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/obs"
	"github.com/postfix/serena/internal/profile"
	repomapPkg "github.com/postfix/serena/internal/repomap"
	"github.com/postfix/serena/internal/skill"
	repomapSkill "github.com/postfix/serena/internal/skill/repomap"
	"github.com/postfix/serena/internal/treesitter"
	"github.com/postfix/serena/internal/workspace"
	gen "github.com/postfix/serena/protocol/gen"
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

// resolveAllowedToolsForMode returns the tool name whitelist for the given
// profile + mode by merging skill, include, and exclude lists from both specs.
// Mirrors profileSkill.ExecuteSwitchMode's resolution logic so the initial
// session state matches what a switch_mode call would produce.
func resolveAllowedToolsForMode(store *profile.ProfileStore, prof *profile.Profile, modeName string) []string {
	if store == nil || prof == nil {
		return nil
	}
	mode, ok := store.Mode(modeName)
	if !ok {
		return nil
	}

	skillSet := make(map[string]bool)
	for _, sk := range prof.Skills {
		skillSet[sk] = true
	}
	for _, sk := range mode.Skills {
		skillSet[sk] = true
	}
	skillNames := make([]string, 0, len(skillSet))
	for sk := range skillSet {
		skillNames = append(skillNames, sk)
	}

	includeTools := append([]string{}, prof.Tools...)
	includeTools = append(includeTools, mode.Tools...)

	excludeTools := append([]string{}, prof.ExcludeTools...)
	excludeTools = append(excludeTools, mode.ExcludeTools...)

	resolved := skill.ResolveTools(skillNames, includeTools, excludeTools)
	names := make([]string, len(resolved))
	for i, t := range resolved {
		names[i] = t.Name
	}
	return names
}

// Daemon is the persistent supervisor process (DMN-01).
// It manages workspace registry, MCP server, kernel, skills, listeners,
// and survives client disconnects (DMN-02).
type Daemon struct {
	config         *config.SerenaConfig
	logger         *slog.Logger
	obs            *obs.Provider
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
	// Build the observability provider from config. When TracingEndpoint is
	// configured, WithTracing creates a real SDK TracerProvider with an
	// OTLP/gRPC exporter; otherwise Noop uses tracenoop (D-17 budget).
	var observability *obs.Provider
	if cfg.Observability.TracingEndpoint != "" {
		observability = obs.WithTracing(logger.Handler(), obs.TracingConfig{
			Endpoint:    cfg.Observability.TracingEndpoint,
			ServiceName: cfg.Observability.ServiceName,
			SampleRatio: cfg.Observability.TracingSampleRatio,
		}, logger)
	} else {
		observability = obs.Noop(logger.Handler())
	}
	return newDaemon(cfg, logger, observability)
}

// NewWithObsProvider creates a Daemon with a pre-built obs.Provider. Intended
// for tests that need to inject a tracetest-backed provider for span assertions.
func NewWithObsProvider(cfg *config.SerenaConfig, logger *slog.Logger, provider *obs.Provider) (*Daemon, error) {
	return newDaemon(cfg, logger, provider)
}

// newDaemon is the shared daemon construction logic.
func newDaemon(cfg *config.SerenaConfig, logger *slog.Logger, observability *obs.Provider) (*Daemon, error) {
	workspaces := workspace.NewRegistry()

	// Wire soft memory limit from config (D-09). Only call SetMemoryLimit when
	// the config value is positive; zero means "don't set" and lets the
	// GOMEMLIMIT env var (if any) take effect undisturbed (Pitfall 4).
	if cfg.Degradation.MemoryLimitMB > 0 {
		limit := int64(cfg.Degradation.MemoryLimitMB) * 1024 * 1024
		debug.SetMemoryLimit(limit)
		logger.Info("soft memory limit set",
			"limit_mb", cfg.Degradation.MemoryLimitMB,
			"limit_bytes", limit,
		)
	}

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
		RestartBudget:         cfg.Degradation.RestartBudget,
	}
	if poolCfg.BaseTTL == 0 {
		poolCfg = lspool.DefaultPoolConfig()
	}

	// 5. Create kernel (fail-fast). obs.Metrics is wired as the lspool sink;
	// the compile-time check lives in internal/daemon/wiring_test.go.
	k := kernel.NewKernel(workspaces, langReg, installer, kernel.KernelConfig{Pool: poolCfg}, pressure, logger, observability.Metrics(), observability.Tracer())

	// 6. Create diagnostic store and body extractor.
	// NOTE: step numbering preserved from original New() for git-blame continuity.
	diagStore := diag.NewDiagnosticStore()
	grammarRegistry := treesitter.NewGrammarRegistry()
	bodyExtractor := edit.NewBodyExtractor(grammarRegistry)

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
	var activeWSLang string
	wsKeyFn := func() workspace.WorkspaceKey { return activeWSKey }
	workspaceRootFn := func() string { return activeWSKey.RepoRoot }

	symbols.RegisterTools(mcpServer, k, wsKeyFn)
	edit.RegisterTools(mcpServer, k, bodyExtractor, diagStore, wsKeyFn)
	fileops.RegisterTools(mcpServer, workspaceRootFn, observability.Tracer())

	// Diag lease provider.
	leaseFn := func(ctx context.Context, uri string) (*lspool.WorkerLease, error) {
		key := activeWSKey
		if key.Language == "" && activeWSLang != "" {
			key.Language = activeWSLang
		}
		return k.Pool().AcquireLease(ctx, "diag-"+uri, key, false)
	}
	diag.RegisterTools(mcpServer, diagStore, workspaceRootFn, leaseFn, observability.Tracer())

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
	// Initial mode precedence: cfg.Mode override > profile.DefaultMode > "edit" fallback.
	initialMode := activeProfile.DefaultMode
	if initialMode == "" {
		initialMode = "edit"
	}
	if cfg.Mode != "" {
		initialMode = cfg.Mode
	}
	// Resolve initial AllowedTools from profile + initial mode so that tools/list
	// is filtered from session start (not only after the first switch_mode call).
	initialAllowedTools := resolveAllowedToolsForMode(profileStore, activeProfile, initialMode)
	sessionProvider := &daemonSessionProvider{
		session: &serenaMCP.SessionInfo{
			Profile:      cfg.Profile,
			Mode:         initialMode,
			AllowedTools: initialAllowedTools,
		},
	}
	if ps := profile.GetProfileSkill(); ps != nil {
		ps.SetSessionProvider(sessionProvider)
	}

	// 12b. Wire repomap skill LSP enrichment callback (RMAP-08).
	if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
		tagCache := rs.Cache()
		rs.SetEnrichFn(func(g *repomapPkg.FileGraph) {
			wsKey := activeWSKey
			if wsKey.RepoRoot == "" {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			enrichRepoMapFromLSP(ctx, k, wsKey, g, tagCache, logger)
		})
	}

	// 14. Install middleware: TelemetryMiddleware (METRIC-02, absorbs Phase 8
	// logging) + ProfileFilterMiddleware (PRF-03). Ordering is independent
	// because telemetry emits on tools/call and profile filter only touches
	// tools/list.
	getSessionFn := func(ctx context.Context) *serenaMCP.SessionInfo {
		return sessionProvider.CurrentSession()
	}
	degradeCfg := cfg.Degradation
	budgetFn := serenaMCP.BudgetFunc(func(toolName string) time.Duration {
		return degrade.BudgetFor(toolName, degradeCfg)
	})
	serenaMCP.InstallMiddleware(mcpServer.SDK(), observability, profileStore, getSessionFn, budgetFn, logger)

	// 15. Update activate_project to also activate workspace in kernel. The
	// callback also publishes the resolved primary language into the session
	// so TelemetryMiddleware can surface it as the "language" metric label.
	mcpServer.SetActivateCallback(func(ctx context.Context, repoPath string) error {
		rt, err := k.ActivateWorkspace(ctx, repoPath)
		if err != nil {
			return err
		}
		activeWSKey = workspace.WorkspaceKey{RepoRoot: repoPath}
		// Propagate workspace root to repomap skill for tag extraction.
		if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
			rs.SetWorkspaceRoot(repoPath)
		}
		if langs := rt.Languages(); len(langs) > 0 {
			activeWSLang = langs[0]
		}
		if sess := sessionProvider.CurrentSession(); sess != nil {
			sess.SetLanguage(activeWSLang)
		}
		logger.Info("kernel workspace activated",
			"root", repoPath,
			"languages", rt.Languages(),
		)
		return nil
	})

	return &Daemon{
		config:        cfg,
		logger:        logger,
		obs:           observability,
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

// ObsProvider returns the observability provider for test assertions (e.g.,
// ShutdownTracing flush verification in trace_shutdown_test.go).
func (d *Daemon) ObsProvider() *obs.Provider { return d.obs }

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

	// Admin listener (OBS-03). Non-fatal per D-04 — bind failure is logged
	// and the daemon continues. listenAdmin is a no-op when AdminAddr == "".
	g.Go(func() error {
		if err := d.listenAdmin(gctx); err != nil && !errors.Is(err, context.Canceled) {
			d.logger.Error("admin listener failed, continuing without observability",
				"error", err,
				"addr", d.config.Observability.AdminAddr)
		}
		return nil // NEVER propagate — observability is instrumentation, not product
	})

	d.logger.Info("daemon started",
		"socket", d.config.Daemon.SocketPath,
		"http_addr", d.config.Daemon.HTTPAddr,
		"admin_addr", d.config.Observability.AdminAddr,
		"workspaces", d.workspaces.WorkspaceCount(),
	)

	// Flip readiness AFTER kernel/socket/http/admin goroutines are spawned and
	// the profile is resolved (profile resolution happens during daemon.New, so
	// by the time Run reaches this point the profile state is stable — Q3).
	// /readyz starts returning 200 from here. Pitfall #3 mitigation.
	ready.Store(1)

	err := g.Wait()
	// Reset ready so repeated Run invocations (tests) start from ready=0.
	ready.Store(0)
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

	// Create and register gRPC server with OTel tracing propagation (D-12).
	// WithTracerProvider is MANDATORY — omitting it falls back to the OTel
	// global which D-01 forbids.
	d.grpcServer = grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler(
			otelgrpc.WithTracerProvider(d.obs.TracerProvider()),
		)),
	)
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

// enrichRepoMapFromLSP opportunistically enriches the repomap graph with
// LSP cross-file references. Queries each defined symbol's actual position
// rather than a fixed 0:0, which yields meaningful cross-file references.
// Skips silently if no warm LSP session exists.
// Called via the enrichFn callback during graph rebuild (RMAP-08).
func enrichRepoMapFromLSP(ctx context.Context, k *kernel.Kernel, wsKey workspace.WorkspaceKey, g *repomapPkg.FileGraph, cache *repomapPkg.TagCache, logger *slog.Logger) {
	sessionID := "enrich-repomap"
	lease, err := k.Pool().AcquireLease(ctx, sessionID, wsKey, false)
	if err != nil {
		logger.Debug("LSP enrichment skipped: no warm session", "error", err)
		return
	}
	defer k.Pool().ReleaseLease(sessionID)

	// Load all cached tags so we can query at actual symbol positions.
	allTags, err := cache.AllFiles()
	if err != nil {
		logger.Debug("LSP enrichment skipped: failed to load tags", "error", err)
		return
	}

	enriched := 0
	for file := range g.Files {
		tags, ok := allTags[file]
		if !ok || len(tags) == 0 {
			continue
		}

		uri := "file://" + file
		for _, tag := range tags {
			if tag.Kind != repomapPkg.TagDef {
				continue
			}
			if ctx.Err() != nil {
				return // timeout reached
			}

			params := gen.ReferenceParams{
				TextDocumentPositionParams: gen.TextDocumentPositionParams{
					TextDocument: gen.TextDocumentIdentifier{URI: uri},
					Position:     gen.Position{Line: uint32(tag.Line), Character: uint32(tag.Column)},
				},
				Context: gen.ReferenceContext{IncludeDeclaration: true},
			}
			var locations []gen.Location
			if reqErr := lease.Request(ctx, "textDocument/references", &params, &locations); reqErr != nil {
				continue
			}
			if len(locations) == 0 {
				continue
			}
			rmLocs := make([]repomapPkg.Location, 0, len(locations))
			for _, loc := range locations {
				rmLocs = append(rmLocs, repomapPkg.Location{
					URI:  loc.URI,
					Line: int(loc.Range.Start.Line),
				})
			}
			g.EnrichFromLSP(file, rmLocs)
			enriched++
		}
	}
	if enriched > 0 {
		logger.Debug("LSP enrichment complete", "symbols_enriched", enriched)
	}
}
