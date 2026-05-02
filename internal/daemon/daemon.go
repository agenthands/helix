package daemon

import (
	"context"
	"encoding/json"
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

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/degrade"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/kernel/diag"
	"github.com/agenthands/helix/internal/kernel/edit"
	"github.com/agenthands/helix/internal/kernel/fileops"
	"github.com/agenthands/helix/internal/kernel/health"
	"github.com/agenthands/helix/internal/kernel/help"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/kernel/symbols"
	"github.com/agenthands/helix/internal/langregistry"
	helixMCP "github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/profile"
	repomapPkg "github.com/agenthands/helix/internal/repomap"
	"github.com/agenthands/helix/internal/skill"
	repomapSkill "github.com/agenthands/helix/internal/skill/repomap"
	"github.com/agenthands/helix/internal/treesitter"
	"github.com/agenthands/helix/internal/workspace"
	gen "github.com/agenthands/helix/protocol/gen"
)

// SkillToolExecutor is implemented by skills that support direct tool execution
// (memory, workflow). Kernel skill adapters skip this and register via RegisterTools.
type SkillToolExecutor interface {
	ExecuteTool(name string, args map[string]interface{}) (string, error)
}

// daemonSessionProvider is a minimal SessionProvider for the profile skill.
type daemonSessionProvider struct {
	session *helixMCP.SessionInfo
}

func (p *daemonSessionProvider) CurrentSession() *helixMCP.SessionInfo {
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
	mcpServer      *helixMCP.SerenaMCPServer
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
		BinDir:      filepath.Join(homeDir, ".helix", "bin"),
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

	// 6a. Refuse to start under CGO_ENABLED=0 (DEF-51-02 / Phase 51.1 / D-02).
	//     The CGO=0 binary is a goreleaser archive-count placeholder; tree-sitter
	//     parsing, RepoMap tag extraction, and replace_symbol_body all depend on
	//     CGO bindings. Surface the unavailability loudly rather than starting in
	//     a permanently degraded state. Under CGO=1, treesitter.Available is a
	//     compile-time const true and the Go compiler eliminates this branch.
	if !treesitter.Available {
		return nil, fmt.Errorf("tree-sitter is unavailable in this build " +
			"(CGO_ENABLED=0): rebuild with CGO_ENABLED=1 or download the " +
			"CGO=1 release binary (see CONTRIBUTING.md > Releasing)")
	}

	// 6. Create diagnostic store and body extractor.
	// NOTE: step numbering preserved from original New() for git-blame continuity.
	diagStore := diag.NewDiagnosticStore()
	grammarRegistry := treesitter.NewGrammarRegistry()
	bodyExtractor := edit.NewBodyExtractor(grammarRegistry)

	// 7. Create MCP server.
	mcpServer := helixMCP.NewSerenaMCPServer(workspaces, logger, observability.Tracer())

	// 8. Resolve profile per D-08.
	globalDir := filepath.Join(homeDir, ".helix")
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
	health.RegisterTools(mcpServer, k)
	help.RegisterTools(mcpServer, k)

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
		session: &helixMCP.SessionInfo{
			Profile:      cfg.Profile,
			Mode:         initialMode,
			AllowedTools: initialAllowedTools,
		},
	}
	if ps := profile.GetProfileSkill(); ps != nil {
		ps.SetSessionProvider(sessionProvider)
	}

	// 12a. Wire shared GrammarRegistry into repomap skill (BUG-04, D-01/D-02/D-03).
	if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
		rs.SetRegistry(grammarRegistry)
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

	// 12c. Wire repomap skill fallback extraction for non-tree-sitter languages (RMAP-02).
	if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
		rs.SetFallbackDeps(&repomapSkill.FallbackDeps{
			Extractor: repomapPkg.NewFallbackExtractor(),
			AcquireFn: func(ctx context.Context, lang string) (repomapPkg.SymbolRequester, func(), error) {
				wsKey := workspace.WorkspaceKey{RepoRoot: activeWSKey.RepoRoot, Language: lang}
				sessionID := fmt.Sprintf("fallback-%s", lang)
				lease, err := k.Pool().AcquireLease(ctx, sessionID, wsKey, false)
				if err != nil {
					return nil, nil, err
				}
				return lease, func() { k.Pool().ReleaseLease(sessionID) }, nil
			},
		})
	}

	// 12d. Wire repomap skill metrics sink (Phase 53 D-15). *obs.Metrics
	// satisfies repomap.MetricsSink ad-hoc via the helper methods declared in
	// internal/obs/metrics.go (compile-time-checked in wiring_test.go).
	// Two seams: the skill dispatcher emits per-extractor latency
	// (helix_repomap_extract_duration_seconds), and the underlying TagCache
	// emits hit/miss lookups (helix_repomap_lookups_total).
	if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
		rs.SetMetricsSink(observability.Metrics())
		if cache := rs.Cache(); cache != nil {
			cache.SetMetricsSink(observability.Metrics())
		}
	}

	// 14. Install middleware: TelemetryMiddleware (METRIC-02, absorbs Phase 8
	// logging) + ProfileFilterMiddleware (PRF-03). Ordering is independent
	// because telemetry emits on tools/call and profile filter only touches
	// tools/list.
	getSessionFn := func(ctx context.Context) *helixMCP.SessionInfo {
		return sessionProvider.CurrentSession()
	}
	degradeCfg := cfg.Degradation
	budgetFn := helixMCP.BudgetFunc(func(toolName string) time.Duration {
		return degrade.BudgetFor(toolName, degradeCfg)
	})
	helixMCP.InstallMiddleware(mcpServer.SDK(), observability, profileStore, getSessionFn, budgetFn, mcpServer.Registry(), logger)

	// 14b. Install suggestion middleware: enriches error responses with "did you mean"
	// parameter corrections (SERR-01, SERR-02, SERR-03). Schema map built from all
	// registered tools (steps 10-11 complete). Must run AFTER InstallMiddleware so
	// SuggestionMiddleware sits between the tool handler and TelemetryMiddleware --
	// errors are enriched before telemetry classifies the outcome.
	suggestionSchemaMap := helixMCP.BuildToolSchemaMap(mcpServer.CollectToolSchemas())
	helixMCP.InstallSuggestionMiddleware(mcpServer.SDK(), suggestionSchemaMap, logger)

	// 14c. Install lazy init middleware (LAZY-01, LAZY-02). Must be installed LAST
	// so it runs FIRST in the LIFO middleware chain (before TelemetryMiddleware deadline).
	lazyActivateFn := func(ctx context.Context, repoPath string) error {
		rt, err := k.ActivateWorkspace(ctx, repoPath)
		if err != nil {
			return err
		}
		activeWSKey = workspace.WorkspaceKey{RepoRoot: repoPath}
		if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
			rs.SetWorkspaceRoot(repoPath)
		}
		if langs := rt.Languages(); len(langs) > 0 {
			activeWSLang = langs[0]
		}
		if sess := sessionProvider.CurrentSession(); sess != nil {
			sess.SetLanguage(activeWSLang)
		}
		logger.Info("kernel workspace activated (lazy init)", "root", repoPath, "languages", rt.Languages())
		return nil
	}
	isActiveFn := func() bool { return activeWSKey.RepoRoot != "" }
	helixMCP.InstallLazyInitMiddleware(mcpServer.SDK(), lazyActivateFn, isActiveFn, "", logger)

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
func (d *Daemon) MCPServer() *helixMCP.SerenaMCPServer { return d.mcpServer }

// KernelInstance returns the kernel for lifecycle management in tests.
func (d *Daemon) KernelInstance() *kernel.Kernel { return d.kernel }

// ObsProvider returns the observability provider for test assertions (e.g.,
// ShutdownTracing flush verification in trace_shutdown_test.go).
func (d *Daemon) ObsProvider() *obs.Provider { return d.obs }

// registerSkillTools registers all tools from a ToolProvider with the MCP server.
// Skills with ExecuteTool (memory, workflow) get live handlers; others are catalog-only.
func registerSkillTools(server *helixMCP.SerenaMCPServer, tp skill.ToolProvider, logger *slog.Logger) {
	executor, hasExecutor := tp.(SkillToolExecutor)
	for _, td := range tp.Tools() {
		if hasExecutor {
			server.AddSkillTool(td.Name, td.Description, td.BriefDescription, td.HelpText, executor)
		} else {
			// Catalog-only registration (e.g., profile skill with custom Execute* methods).
			server.Registry().Register(&helixMCP.ToolDef{
				Name:             td.Name,
				Description:      td.Description,
				BriefDescription: td.BriefDescription,
				HelpText:         td.HelpText,
			})
		}
	}
	logger.Info("skill tools registered", "skill", tp.Name(), "tools", len(tp.Tools()), "has_executor", hasExecutor)
}

// Run starts the daemon and blocks until shutdown (DMN-01, DMN-12).
// Signal handlers are registered FIRST per Pitfall 3.
func (d *Daemon) Run(ctx context.Context) error {
	// Mark this process (and every child it spawns — LS workers, hypothetical
	// `helix upgrade` exec) as running inside the daemon's process tree by
	// setting HELIX_RUNNING_AS_DAEMON=1. Read by internal/upgrade/daemon_detect.go
	// (RunningInDaemon) so `helix upgrade` invoked from inside the daemon
	// short-circuits with a "restart the daemon manually" hint instead of
	// tearing out the running binary from under the daemon (D-08, threat
	// T-52-04-08). os.Setenv mutates the calling process's environment in
	// place and is inherited by exec.Command children via os.Environ().
	_ = os.Setenv("HELIX_RUNNING_AS_DAEMON", "1")

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
		kernel:    d.kernel,
		logger:    d.logger,
		// Phase 53 D-17: direct call to *obs.Metrics for stdio session
		// lifecycle emission. observability.Metrics() is never nil per the
		// Noop-default invariant — no nil guard needed inside the handler.
		metrics: d.obs.Metrics(),
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
	// Phase 53 D-09 + Q-1 Option 2: wrap the SDK HTTP handler with the
	// session-lifecycle middleware to emit (started|ended|error, http).
	// Best-effort `ended` semantic is documented in USAGE.md by Plan 06.
	mux.Handle("/mcp", httpSessionMiddleware(d.mcpServer.HTTPHandler(), d.obs.Metrics()))

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

// sessionRunner is the test seam that runs the MCP session for a forwarder
// stream. The production implementation (defaultSessionRunner) wraps the
// stream in a GRPCTransport, calls mcpServer.SDK().Connect, and waits for
// the session to end. Tests inject a stub that bypasses the MCP runtime
// while preserving the (started → ended | error) lifecycle classification
// (Phase 53 D-08).
type sessionRunner func(ctx context.Context, stream serenav1.ForwarderService_StreamMCPServer, firstMsg *serenav1.MCPMessage) error

// forwarderServiceHandler implements the gRPC ForwarderService.
type forwarderServiceHandler struct {
	serenav1.UnimplementedForwarderServiceServer
	mcpServer *helixMCP.SerenaMCPServer
	kernel    *kernel.Kernel
	logger    *slog.Logger

	// metrics is the observability hook for session lifecycle events
	// (Phase 53 D-17). Direct call instead of sink-interface because
	// daemon.go already imports internal/obs and the field is a
	// production-time pointer that never mutates.
	metrics *obs.Metrics

	// serveSession is the test seam for the MCP-runtime portion of
	// StreamMCP. nil in production (defaultSessionRunner is used);
	// non-nil in tests to stub Connect/Wait without spinning up a real
	// MCP server.
	serveSession sessionRunner
}

// StreamMCP handles a bidirectional MCP stream from a forwarder.
//
// Lifecycle emission contract (Phase 53 D-08):
//   - (started, stdio) emitted ONCE after firstMsg is received (we know we
//     have a session). NOT emitted if Recv() fails before firstMsg, because
//     no session_id is known yet — emitting would inflate the started
//     counter without a matching (ended, stdio) / (error, stdio).
//   - (error, stdio) on Connect() failure or non-nil session.Wait() return.
//   - (ended, stdio) on clean session.Wait() return.
func (h *forwarderServiceHandler) StreamMCP(stream serenav1.ForwarderService_StreamMCPServer) error {
	// Read the first message to get the session ID
	firstMsg, err := stream.Recv()
	if err != nil {
		// No session_id known yet — do NOT emit a lifecycle event.
		return fmt.Errorf("receiving first message: %w", err)
	}

	sessionID := firstMsg.SessionId
	h.logger.Info("new forwarder stream", "session_id", sessionID)
	// Phase 53 D-08: emit started AFTER firstMsg is received.
	h.metrics.SessionLifecycleInc("started", "stdio")

	runner := h.serveSession
	if runner == nil {
		runner = defaultSessionRunner(h.mcpServer)
	}
	err = runner(stream.Context(), stream, firstMsg)
	h.logger.Info("forwarder stream ended", "session_id", sessionID)
	if err != nil {
		// Phase 53 D-08: error covers both Connect-failure and
		// session.Wait-failure paths (the seam collapses them — both
		// surface as a non-nil err here).
		h.metrics.SessionLifecycleInc("error", "stdio")
	} else {
		h.metrics.SessionLifecycleInc("ended", "stdio")
	}
	return err
}

// defaultSessionRunner returns the production sessionRunner that bridges
// the gRPC stream to the MCP SDK. Extracted so tests can substitute a
// stub via the forwarderServiceHandler.serveSession field without
// reaching into the MCP runtime.
func defaultSessionRunner(mcpServer *helixMCP.SerenaMCPServer) sessionRunner {
	return func(ctx context.Context, stream serenav1.ForwarderService_StreamMCPServer, firstMsg *serenav1.MCPMessage) error {
		sessionID := firstMsg.SessionId
		// Create a GRPCTransport that bridges this stream to the MCP SDK.
		// Pass firstMsg so it gets replayed into the transport pipe.
		transport := helixMCP.NewGRPCTransport(stream, sessionID, firstMsg)

		// Connect the MCP server to this transport
		session, err := mcpServer.SDK().Connect(ctx, transport, nil)
		if err != nil {
			return fmt.Errorf("connecting MCP session: %w", err)
		}

		// Wait for the session to end
		return session.Wait()
	}
}

// GetStatus returns workspace health for the CLI status command.
func (h *forwarderServiceHandler) GetStatus(ctx context.Context, req *serenav1.StatusRequest) (*serenav1.StatusResponse, error) {
	report := h.kernel.HealthStatus()
	health.FilterReport(report, req.Verbose)

	payload, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("marshaling health report: %w", err)
	}
	return &serenav1.StatusResponse{Payload: payload}, nil
}

// ActivateWorkspace ensures a workspace is active in the kernel (HOOK-01).
func (h *forwarderServiceHandler) ActivateWorkspace(ctx context.Context, req *serenav1.ActivateRequest) (*serenav1.ActivateResponse, error) {
	wsPath := req.WorkspacePath
	if wsPath == "" {
		return nil, fmt.Errorf("workspace_path is required")
	}

	// Resolve to absolute path for safety (T-36-06)
	absPath, err := filepath.Abs(wsPath)
	if err != nil {
		return nil, fmt.Errorf("resolving workspace path: %w", err)
	}

	// Verify it's a directory
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("workspace path %s: %w", absPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace path %s is not a directory", absPath)
	}

	_, err = h.kernel.ActivateWorkspace(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("activating workspace: %w", err)
	}

	h.logger.Info("workspace activated via gRPC", "path", absPath)
	return &serenav1.ActivateResponse{
		AlreadyActive: false, // Kernel handles idempotency internally
		Status:        "activated",
	}, nil
}

// DeactivateWorkspace cleans up session state for a workspace (HOOK-03).
func (h *forwarderServiceHandler) DeactivateWorkspace(ctx context.Context, req *serenav1.DeactivateRequest) (*serenav1.DeactivateResponse, error) {
	wsPath := req.WorkspacePath
	if wsPath == "" {
		return nil, fmt.Errorf("workspace_path is required")
	}

	h.logger.Info("workspace deactivation requested via gRPC", "path", wsPath)
	// Note: We don't actually shut down the workspace in the kernel --
	// other sessions may be using it. We just acknowledge the deactivation.
	// Session-scoped cleanup (counter files) is handled client-side.
	return &serenav1.DeactivateResponse{
		Status: "deactivated",
	}, nil
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
