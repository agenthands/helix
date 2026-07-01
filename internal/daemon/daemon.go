package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/degrade"
	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/guardrails/rules"
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
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/compact"
	"github.com/agenthands/helix/internal/semantic/extract"
	cextract "github.com/agenthands/helix/internal/semantic/extract/c"
	cppextract "github.com/agenthands/helix/internal/semantic/extract/cpp"
	csharpextract "github.com/agenthands/helix/internal/semantic/extract/csharp"
	goextract "github.com/agenthands/helix/internal/semantic/extract/golang"
	javaextract "github.com/agenthands/helix/internal/semantic/extract/java"
	kotlinextract "github.com/agenthands/helix/internal/semantic/extract/kotlin"
	phpextract "github.com/agenthands/helix/internal/semantic/extract/php"
	pyextract "github.com/agenthands/helix/internal/semantic/extract/python"
	rubyextract "github.com/agenthands/helix/internal/semantic/extract/ruby"
	rustextract "github.com/agenthands/helix/internal/semantic/extract/rust"
	tsextract "github.com/agenthands/helix/internal/semantic/extract/typescript"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/semantic/scheduler"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/semantic/types"
	"github.com/agenthands/helix/internal/skill"
	repomapSkill "github.com/agenthands/helix/internal/skill/repomap"
	semanticpkg "github.com/agenthands/helix/internal/skill/semantic"
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
	// semanticStore is the DuckDB-backed semantic fact store (Phase 57+).
	// nil when cfg.SemanticIndex.Enabled is false; also nil on the
	// windows/arm64 target where Open returns serr.ErrUnsupported and
	// step 6b soft-degrades (D-14 / DEF-51-04). Downstream consumers
	// (P64+) MUST nil-check.
	semanticStore *semanticstore.Store
	// semanticExtractRegistry is the daemon-owned catalogue of per-language
	// extraction providers (Phase 59 P02). nil when cfg.SemanticIndex.Enabled
	// is false. Holds the daemon-singleton *treesitter.GrammarRegistry
	// (BUG-04 / EXTRACT-05 invariant); per-language providers (Phase 59 P04)
	// are registered into this registry at daemon bootstrap step 6c, never
	// via init() (D-02). Downstream consumers (scheduler, future MCP tools)
	// MUST nil-check.
	semanticExtractRegistry *extract.Registry
	// grammarRegistry is the daemon-singleton *treesitter.GrammarRegistry
	// (BUG-04 / EXTRACT-05 invariant). Exposed via the (test-only) accessor
	// in daemon_test_export_test.go so the EXTRACT-05 regression test can
	// assert pointer-equality across all consumers (extract registry, body
	// extractor, repomap skill).
	grammarRegistry *treesitter.GrammarRegistry
	// semanticScheduler orchestrates initial-walk and incremental extraction
	// (Phase 59 P03). nil when cfg.SemanticIndex.Enabled is false. The
	// activate callback fires ScheduleInitialExtraction(workspace_activation)
	// non-blockingly per D-04.
	semanticScheduler *scheduler.Scheduler
	// live holds the Phase 60 live-update + Phase 61 enrichment-manager
	// bundle. nil when SemanticIndex.LiveUpdates.Enabled is false. Run
	// pulls live.Run(gctx) into the top-level errgroup; the gRPC
	// DeactivateWorkspace handler invokes live.OnWorkspaceDeactivate so
	// cached enrichment leases are released promptly per B2.
	live *liveBundle

	// rank holds the Phase 62 P03 rank engine + per-workspace
	// RankScheduler map. nil when the semantic store is not open or the
	// live handler is missing. Run pulls rank.Run(gctx) into the top-
	// level errgroup; per-workspace schedulers are spun up lazily in
	// SetActivateCallback via rank.ensureScheduler.
	rank *rankBundle

	// compact holds the Phase 63 P63-02 per-workspace compaction
	// worker registry. nil when the semantic store is not open. Run
	// pulls compact.Run(gctx) into the top-level errgroup; per-workspace
	// compactors are spun up lazily in SetActivateCallback via
	// compact.ensureCompactor.
	compact *compactBundle

	// semantic holds the Phase 64 P64-08 semantic-skill production wiring
	// (bleve handles + IndexRunner + 7 narrow accessor adapters). nil
	// when the semantic store is not open. Run pulls semantic.Run(gctx)
	// into the top-level errgroup; per-workspace bleve engines + recovery
	// probes are spun up lazily in SetActivateCallback via
	// semantic.ensureRetrieval.
	semantic *semanticBundle

	// typeResolver is the Phase 62 P05 type-resolver dispatcher — the
	// 7-language registry (go / typescript / javascript / python / java /
	// php / ruby) per D-11. nil when the semantic store is unavailable.
	// Wired at bootstrap step 6g. Phase 64 will attach a consumer via
	// SetSemanticGraph; until then the dispatcher is held on the daemon
	// for later attachment.
	typeResolver types.Resolver

	// semanticGraphRanker is the optional ranker handle paired with the
	// type-resolver dispatcher per RESEARCH Open Question 4. Typed `any`
	// to avoid importing the rank engine's concrete type into this struct;
	// Phase 64 will narrow it.
	semanticGraphRanker any

	// effSemanticDisabled is the Phase 81 ABLATE-06 composition-root gate
	// (resolved once at New, daemon.go:294). Persisted on the struct so the
	// Plan 07 (CR-01) gate test can assert the background read pipelines are
	// inert under the gate (build-but-block: the store/bundle stay built).
	effSemanticDisabled bool

	// testServeSession is a TEST-ONLY seam (Phase 94 RETIRE-04). When non-nil it
	// is wired into the forwarderServiceHandler built by listenSocket /
	// listenGRPCTCP as the StreamMCP session runner, so a daemon unit/lifecycle
	// test can drive a real gRPC round-trip without a full MCP server. nil in
	// production — the handler falls back to defaultSessionRunner.
	testServeSession sessionRunner
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
	// TODO(v1.11): migrate to phasegraph.RunPhaseGraph(BootstrapPhases) (DAG-04).
	// The numbered imperative steps below are the future PhaseSpec set.
	// See internal/phasegraph/pipelines/ for the consumer-side shape declarations.
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

	// 4b. Resolve the active profile early (hoisted from former step 8) so
	// the kernel can be constructed with the effective subsystem-disable
	// flags. Phase 76 ABLATE-05/07: the composition root owns the kernel-
	// config concern (D-02/D-03); the effective flag is (CLI override OR
	// resolved-profile field) — a one-way force-disable because both default
	// OFF and the flags are opt-in disables (RESEARCH Pattern 2).
	globalDir := filepath.Join(homeDir, ".helix")
	profileStore, activeProfile, err := config.ResolveProfile(cfg, globalDir)
	if err != nil {
		return nil, fmt.Errorf("resolving profile: %w", err)
	}
	logger.Info("profile resolved",
		"profile", cfg.Profile,
		"default_mode", activeProfile.DefaultMode,
	)
	effDisableLSP := cfg.DisableLSPSubsystem || activeProfile.DisableLSPSubsystem
	effDisableSE := cfg.DisableStructuredEditSubsystem || activeProfile.DisableStructuredEditSubsystem
	// Phase 81 ABLATE-06 (D-02): resolve effSemanticDisabled ONCE here, OR'ing
	// the config field with the active profile field (precedence already
	// collapsed upstream, D-03). Threaded into every back-channel semantic read
	// consumer below to force integ.NoopLookup{} + a DISABLED ConfigGate
	// (build-but-block, D-04 — the bundle/store is STILL built).
	effSemanticDisabled := resolveSemanticDisabled(cfg, activeProfile)
	if effDisableLSP || effDisableSE || effSemanticDisabled {
		logger.Info("subsystem ablation flags resolved",
			"disable_lsp_subsystem", effDisableLSP,
			"disable_structured_edit_subsystem", effDisableSE,
			"disable_semantic_subsystem", effSemanticDisabled,
		)
	}

	// 5. Create kernel (fail-fast). obs.Metrics is wired as the lspool sink;
	// the compile-time check lives in internal/daemon/wiring_test.go.
	k := kernel.NewKernel(workspaces, langReg, installer, kernel.KernelConfig{
		Pool:                           poolCfg,
		DisableLSPSubsystem:            effDisableLSP,
		DisableStructuredEditSubsystem: effDisableSE,
	}, pressure, logger, observability.Metrics(), observability.Tracer())

	// 6b. Open semantic fact store when enabled (Phase 57, STORE-01..06).
	//     Fail-fast core subsystem; on Tier-2 corruption auto-quarantines to
	//     `<path>.corrupt.<ts>` and rebuilds fresh (SPEC §29.1, STORE-01).
	//     Tier-3 (rebuild failed) returns an error so the daemon refuses to
	//     start. When semantic_index.enabled=false, semanticStore is nil and
	//     downstream consumers (P64+ tools) MUST guard.
	//
	//     D-14: on the windows/arm64 target the semantic store is platform-
	//     stubbed (duckdb-go-bindings has no lib/windows-arm64 — DEF-51-04);
	//     Open returns serr.ErrUnsupported. Soft-degrade by leaving
	//     semanticStore nil so the daemon proceeds rather than refusing to
	//     start; downstream consumers already guard on nil per STORE-01.
	var semanticStore *semanticstore.Store
	if cfg.SemanticIndex.Enabled {
		s, err := semanticstore.Open(context.Background(), cfg.SemanticIndex,
			logger, observability.Metrics())
		if err != nil {
			if errors.Is(err, serr.ErrUnsupported) {
				logger.Warn("semantic store unavailable on this target; semantic_index disabled",
					"err", err)
				semanticStore = nil
			} else {
				return nil, fmt.Errorf("opening semantic store: %w", err)
			}
		} else {
			semanticStore = s
		}
	}

	// 6. Create diagnostic store and body extractor.
	// NOTE: step numbering preserved from original New() for git-blame continuity.
	diagStore := diag.NewDiagnosticStore()
	grammarRegistry := treesitter.NewGrammarRegistry()
	bodyExtractor := edit.NewBodyExtractor(grammarRegistry)

	// 6c. Construct semantic extractor registry (Phase 59 P05) with the three
	// per-language providers (Phase 59 P04). Per CONTEXT.md D-02 hard
	// invariant: NO init() registration, NO blank imports — the providers
	// are constructed here so each one receives the daemon-singleton
	// *treesitter.GrammarRegistry (BUG-04 / EXTRACT-05; static enforcement
	// in internal/semantic/extract/registry_grep_test.go).
	var semanticExtractRegistry *extract.Registry
	var semanticScheduler *scheduler.Scheduler
	if cfg.SemanticIndex.Enabled {
		semanticExtractRegistry = extract.NewExtractorRegistry(
			grammarRegistry,
			goextract.NewProvider(grammarRegistry),
			tsextract.NewProvider(grammarRegistry),
			pyextract.NewProvider(grammarRegistry),
			javaextract.NewProvider(grammarRegistry),
			csharpextract.NewProvider(grammarRegistry),
			rustextract.NewProvider(grammarRegistry),
			cextract.NewProvider(grammarRegistry),
			cppextract.NewProvider(grammarRegistry),
			kotlinextract.NewProvider(grammarRegistry),
			phpextract.NewProvider(grammarRegistry),
			rubyextract.NewProvider(grammarRegistry),
		)
		logger.Info("semantic extract registry constructed",
			"providers", len(semanticExtractRegistry.Languages()),
		)

		// 6d. Construct the extraction scheduler (Phase 59 P03). The scheduler
		// owns workspace-keyed state, idempotent admission of initial-walk
		// jobs, and the RequireReady gate. Activation (step 15) fires
		// ScheduleInitialExtraction non-blockingly per D-04.
		semanticScheduler = scheduler.NewScheduler(semanticExtractRegistry)
		logger.Info("semantic extraction scheduler constructed")
	}

	// 6e. Wire Phase 60 live-update pipeline. Returns nil when
	// SemanticIndex.LiveUpdates.Enabled is false OR a required dep is
	// missing — SetActivateCallback handles nil cleanly.
	//
	// On non-nil return: kernel.SetEditNotifier is installed and
	// scheduler.SetIncrementalHandler is wired. Per-workspace lifecycle
	// hooks (Start/Stop) fire from SetActivateCallback below.
	// Phase 76 D-09 null-object injection: under effDisableLSP the live-update
	// bundle is NOT built. buildLiveBundle is what installs the kernel
	// EditNotifier (live_wiring.go SetEditNotifier) + the LSP-enrichment
	// manager; skipping it leaves EditNotifier() nil so all 4 fileops OnEdit
	// hooks no-op via the existing nil-check contract (notifier.go) and no
	// LSP-enrichment worker is started. The default arm is unchanged.
	var live *liveBundle
	if !effDisableLSP {
		live = buildLiveBundle(
			cfg.SemanticIndex.LiveUpdates,
			cfg.SemanticIndex.LSPEnrichment,
			semanticStore,
			semanticScheduler,
			k,
			observability.Metrics(),
			logger,
		)
	} else {
		logger.Info("no_lsp: skipping live-update bundle (EditNotifier stays nil, OnEdit hooks no-op)")
	}
	if live != nil {
		logger.Info("live-update pipeline wired",
			"watcher_enabled", cfg.SemanticIndex.LiveUpdates.WatcherEnabled,
			"manifest_scan_enabled", cfg.SemanticIndex.LiveUpdates.ManifestScanEnabled,
			"manifest_scan_interval", cfg.SemanticIndex.LiveUpdates.ManifestScanInterval,
		)
	}

	// 6f. Phase 62 P03: rank engine + per-workspace RankScheduler bundle.
	// Constructed only when the semantic store is open AND the live bundle
	// is wired (the post-commit ApplyRepair hook lives on the live handler;
	// no live → no graph_version advances). Per-workspace schedulers spin
	// up lazily in SetActivateCallback so the daemon doesn't have to know
	// the workspace set ahead of time. The bundle's Run goroutine demuxes
	// GraphVersionAdvance into the right scheduler.Notify; kernel-first
	// shutdown is preserved because the bundle returns on ctx.Done.
	var rank *rankBundle
	if semanticStore != nil && live != nil && live.handler != nil {
		// Phase 68 Plan 04: wire FileFactStore + ExtractRegistry +
		// metrics sink so the Tier-1 / Tier-2 populator in
		// internal/semantic/live/handler/difffacts.go has the real
		// dependencies it needs. nil-safe setters (Phase 60 pattern).
		// *semanticstore.Store satisfies handler.FileFactStore via the
		// Phase 68 Plan 01 accessor (GetLatestFileFact); *extract.Registry
		// satisfies handler.ExtractRegistry via Provider(lang).
		// Phase 81 Plan 07 (CR-01): SetFileFactStore is the read-DRIVER
		// behind GetLatestFileFact / LatestCommittedSnapshot — gate it on
		// effSemanticDisabled so the no_semantic arm drives ZERO background
		// reads against the COUNTED DuckDB chokepoint. The store stays OPEN
		// (store-Open guard untouched, D-04 build-but-block); we un-wire the
		// read-driver, not the store.
		if !backgroundSemanticReadsDisabled(effSemanticDisabled) {
			live.handler.SetFileFactStore(semanticStore)
		}
		if semanticExtractRegistry != nil {
			live.handler.SetExtractRegistry(semanticExtractRegistry)
		}
		live.handler.FileFactDiffMetrics = observability.Metrics()
		rankAdapter := newRankStoreAdapter(semanticStore, observability.Metrics(), logger)
		rank = newRankBundle(
			cfg.SemanticIndex.PageRank,
			cfg.SemanticIndex.Graph,
			rankAdapter,
			observability.Metrics(),
			logger,
		)
		if rank != nil {
			// Wire post-commit hook: the handler's local FileFactDiff →
			// graph.ComputeGraphRepair → engine.ApplyRepair flow now has a
			// real target. Phase 62 P02 wired SetRankApplier nil-safe; this
			// is the production binding.
			live.handler.SetRankApplier(rank.engine)
			// SetVersionNotifier alias is exposed by Engine for plan-
			// compatibility (P03 acceptance criterion); the production
			// channel-based notify path is wired in newRankBundle (see
			// rank_wiring.go) and stays the primary fan-out. This call
			// surfaces the keyword in daemon wiring without changing the
			// active code path.
			_ = rank.engine.SetVersionNotifier
			logger.Info("rank engine wired to live handler post-commit hook")
		}
	}

	// 6f.1 Phase 63 P63-02: compaction worker bundle. Constructed
	// alongside rank — needs the live bundle (for the coalescer
	// accessor + post-flush hook), the lspenrich queue (for
	// BlockedLSPPending), the rank bundle (for BlockedRankRepairing),
	// and the kernel (for BlockedEditTxActive).
	var compactBndl *compactBundle
	if semanticStore != nil {
		var lspQ *lspenrich.LaneQueue
		if live != nil {
			lspQ = live.lspQueue
		}
		compactBndl = newCompactBundle(
			cfg.SemanticIndex.Maintenance,
			cfg.SemanticIndex.LiveUpdates,
			semanticStore,
			live,
			lspQ,
			rank,
			k,
			observability.Metrics(),
			logger,
		)
		// Wire the post-flush hook on the live service so every
		// per-workspace coalescer signals back into the compactor.
		if compactBndl != nil && live != nil && live.service != nil {
			live.service.SetOnFlushHook(compactBndl.OnCoalescerFlush)
			logger.Info("compactor post-flush hook wired to live service")
		}
	}

	// 6f.2 Phase 64 P64-08: semantic skill production wiring. Constructed
	// alongside compactBndl — needs the store, the rank bundle (for the
	// scheduler accessor), the lspenrich queue (for the queue accessor),
	// the live bundle (for the live + flush accessors), and the compactor
	// bundle (for the compactor accessor). The session-lookup closure is
	// captured once below (after the InstallMiddleware site reaches
	// scope) and passed in via setter so all accessors share a single
	// source of truth (closes checker W2 at the production layer).
	//
	// Wiring order:
	//   1. Construct sBndl with a placeholder getSession (nil); register
	//      tools & runner at this point.
	//   2. Once getSessionFn is built (after step 14 below), call
	//      sBndl.SetSessionFn(getSessionFn) so the session adapter is
	//      live-wired.
	//
	// The bundle returns nil when the store is nil (semantic disabled);
	// downstream wiring is a no-op in that case.
	// Hoisted from former step 10 so newSemanticBundle can capture the
	// wsKeyFn closure (Phase 65 65-02: closes D-09 carryover #2 by
	// threading the daemon's active-workspace registry into
	// semSessionAdapter; RESEARCH §Pattern 3 a).
	var activeWSKey workspace.WorkspaceKey
	var activeWSLang string
	wsKeyFn := func() workspace.WorkspaceKey { return activeWSKey }
	workspaceRootFn := func() string { return activeWSKey.RepoRoot }

	var sBndl *semanticBundle
	if semanticStore != nil {
		var lspQ *lspenrich.LaneQueue
		if live != nil {
			lspQ = live.lspQueue
		}
		sBndl = newSemanticBundle(
			loadSemanticConfig(),
			semanticStore,
			rank,
			lspQ,
			live,
			compactBndl,
			semanticExtractRegistry,
			logger,
			observability.Metrics(),
			nil, // getSession wired below in step 14b.5 after getSessionFn is constructed
			wsKeyFn,
			effSemanticDisabled, // Phase 81 ABLATE-06: gate the Set*Accessor block (Pitfall 3 / A5)
		)
		// Phase 69-05 / STATUS-02: bind the compactBundle bleveMetaFn so
		// every per-workspace compactor receives the matching bleve
		// engine as its BleveMeta writer at ensureCompactor time. MUST
		// happen BEFORE the first SetActivateCallback fires (the
		// callback invokes compactBndl.ensureCompactor which captures
		// the resolved BleveMeta at construction; bindings after that
		// point do NOT retroactively rewire). compactBndl is nil-safe
		// when semantic is disabled.
		if compactBndl != nil && sBndl != nil {
			compactBndl.SetBleveMetaFn(func(ws workspace.WorkspaceKey) compact.BleveMeta {
				sBndl.mu.Lock()
				defer sBndl.mu.Unlock()
				eng, ok := sBndl.engines[ws.RepoRoot]
				if !ok || eng == nil {
					return nil
				}
				return eng
			})
		}
	}

	// 6g. Phase 62 P05: type-resolver dispatcher.
	//
	// Constructed when the semantic store is open. The dispatcher carries
	// 7 entries (go / typescript / javascript / python / java / php /
	// ruby) per D-11; the JS alias is wired explicitly so it shows up in
	// the bootstrap log alongside the others. Per RESEARCH Open Question
	// 4 the dispatcher is paired with the rank engine via a single
	// SetSemanticGraph setter; in Phase 62 the setter is optional —
	// Phase 64 will attach the MCP-tool consumer.
	//
	// The per-language constructors and the EffectiveReader adapter live
	// in type_resolver_wiring.go to keep this bootstrap step compact;
	// the dispatcher constructor is invoked here directly so the
	// daemon's registration site is one grep away.
	var typeResolver types.Resolver
	if semanticStore != nil {
		reader := newTypeStoreAdapter(semanticStore)
		tsResolver := tsTypeResolver(reader)
		typeDispatcher := types.NewDispatcher(map[string]types.Resolver{
			"go":         goTypeResolver(reader),
			"typescript": tsResolver,
			"javascript": tsResolver, // shared with TS (D-11)
			"python":     pyTypeResolver(reader),
			"java":       javaTypeResolver(reader),
			"php":        phpTypeStub(),
			"ruby":       rubyTypeStub(),
			"c_sharp":    csharpTypeResolver(reader),
			"rust":       rustTypeStub(reader),
			"c":          cTypeResolver(reader),
			"cpp":        cppTypeResolver(reader),
			"kotlin":     kotlinTypeStub(reader),
		})
		typeResolver = typeDispatcher
		logger.Info("type resolver registered",
			"languages", []string{"go", "typescript", "javascript", "python", "java", "php", "ruby", "c_sharp", "rust", "c", "cpp", "kotlin"},
			"max_chain_depth", cfg.SemanticIndex.TypeResolution.MaxChainDepth,
			"max_fixpoint_iterations", cfg.SemanticIndex.TypeResolution.MaxFixpointIterations,
			"comment_parsers_enabled", cfg.SemanticIndex.Types.CommentParsersEnabled,
		)
	}

	// 7. Create MCP server.
	mcpServer := helixMCP.NewSerenaMCPServer(workspaces, logger, observability.Tracer())

	// 8. Profile already resolved at step 4b (hoisted in Phase 76 so the
	// kernel can be built with effective subsystem-disable flags).
	// profileStore / activeProfile / globalDir are in scope from there.

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
	// (activeWSKey / activeWSLang / wsKeyFn / workspaceRootFn are declared
	// earlier — step 6f.2 — so newSemanticBundle can capture wsKeyFn.)

	// Phase 65 65-06: thread the daemon-wired SemanticLookup + ConfigGate
	// into symbols.RegisterTools so registerAnalyzeBlastRadius can route
	// through integ.ChooseSource(cfgGate, lookup, nil) for the cfg-disabled →
	// SourceTreeSitter contract (D-04 + Pitfall §3) BEFORE any semantic call.
	//
	// The lookupFn is a closure rather than a fixed lookup so the daemon
	// retains the option to swap the production adapter in later (mirrors
	// the SetSemanticLookup post-init wiring used by RepoMapSkill in 65-05).
	// nil sBndl is normalized to NoopLookup{} so ChooseSource always sees a
	// valid SemanticLookup interface value.
	// Phase 81 ABLATE-06 gate point 1 (D-04 / Pitfall 4): under
	// effSemanticDisabled the closure returns integ.NoopLookup{} as its FIRST
	// line (build-but-block — the bundle stays built) and the cfgGate reports
	// DISABLED so ChooseSource yields source=tree_sitter, not fallback.
	symbolsLookupFn := gatedSymbolsLookupFn(sBndl, effSemanticDisabled)
	symbolsCfgGate := gatedCfgGate(cfg, effSemanticDisabled)
	symbols.RegisterTools(mcpServer, k, wsKeyFn, symbolsLookupFn, symbolsCfgGate)

	// Phase 65 65-07 INTEG-04 / INTEG-05: capture the wired SemanticLookup
	// once for both the get_health source-field stamp and the semantic_index
	// block. healthLookup is normalized to integ.NoopLookup{} when sBndl is
	// nil (semantic disabled) so integ.ChooseSource always sees a valid
	// SemanticLookup interface value, and the SemanticIndexAccessor adapter
	// always has a non-nil delegate.
	// Phase 81 ABLATE-06 gate point 2 (health): healthLookup inherits the gated
	// symbolsLookupFn (Noop under the gate); healthCfgGate is disabled under the
	// gate to match (Pitfall 4 — source=tree_sitter on the get_health stamp).
	healthLookup := symbolsLookupFn()
	healthSemIndex := &daemonSemIndexAccessor{lookup: healthLookup}
	healthCfgGate := gatedCfgGate(cfg, effSemanticDisabled)
	edit.RegisterTools(mcpServer, k, bodyExtractor, diagStore, wsKeyFn)
	// Phase 60 D-03: fileops.RegisterTools now threads *kernel.Kernel +
	// wsKeyFn so the create_file / replace_in_file / fuzzy_edit register*
	// closures can fire the EditNotifier.OnEdit hook on the success path.
	fileops.RegisterTools(mcpServer, k, workspaceRootFn, wsKeyFn, observability.Tracer())

	// Diag lease provider. Phase 76 D-10 audit: this leaseFn is NOT
	// profile-gated (it is wired directly here, not behind a tool the
	// bench-no-lsp profile can exclude), so under effDisableLSP it must be
	// neutralized to a stub that returns serr.Unsupported WITHOUT touching
	// k.Pool() — otherwise a diagnostics tool call could acquire a live
	// worker and emit an lspool.lsp.* span on the no_lsp arm.
	var leaseFn diag.LeaseProvider
	if effDisableLSP {
		leaseFn = func(ctx context.Context, uri string) (*lspool.WorkerLease, error) {
			return nil, serr.New(serr.Unsupported, "subsystem_disabled: LSP subsystem disabled; diagnostics unavailable")
		}
	} else {
		leaseFn = func(ctx context.Context, uri string) (*lspool.WorkerLease, error) {
			key := activeWSKey
			if key.Language == "" && activeWSLang != "" {
				key.Language = activeWSLang
			}
			return k.Pool().AcquireLease(ctx, "diag-"+uri, key, false)
		}
	}
	diag.RegisterTools(mcpServer, diagStore, workspaceRootFn, leaseFn, observability.Tracer())
	// WR-2 / IN-04 (Phase 65 65-11 Task 2): the probe carries the bundle
	// pointer so Probe() can stamp lastErrReason on SELECT 1 failure and
	// clear it on success. sBndl is nil when semantic is disabled — the
	// bundle setter is nil-safe.
	health.RegisterTools(mcpServer, k, semanticStoreProbe{s: semanticStore, bundle: sBndl}, healthSemIndex, healthCfgGate, healthLookup, wsKeyFn)
	help.RegisterTools(mcpServer, k)

	// 11. Register skill-provided tools with MCP SDK.
	for _, tp := range skill.ToolProviders() {
		// Skip kernel skill adapters (already registered via RegisterTools above).
		switch tp.Name() {
		case "symbol-retrieval", "symbol-editing", "file-ops", "diagnostics", "symbols":
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
	// WR-01 fail-closed: validate the initial operational mode resolves to a
	// registered mode BEFORE resolving its tool whitelist. A profile is always
	// active here (ResolveProfile falls back to the "full" default profile), so
	// an unresolvable mode means a misconfigured profile.DefaultMode or a bad
	// config-file `mode:` value. Left unchecked, resolveAllowedToolsForMode would
	// return nil, and ProfileEnforcementMiddleware treats a nil whitelist as
	// "all tools allowed" — silently disabling tools/call enforcement for the
	// whole session (a fail-OPEN authz default). Refuse to start instead, in
	// line with the daemon's fail-fast-for-core-subsystems policy. The legitimate
	// "no profile configured → nil → all tools" default is unaffected: that path
	// never reaches here because a profile is always resolved.
	if _, ok := profileStore.Mode(initialMode); !ok {
		return nil, fmt.Errorf(
			"initial operational mode %q does not resolve to a registered mode "+
				"(profile=%s, available modes=%v); refusing to start to avoid "+
				"fail-open tools/call enforcement — check profile.DefaultMode or the "+
				"config-file `mode:` value",
			initialMode, activeProfile.Name, profileStore.ModeNames(),
		)
	}
	// Resolve initial AllowedTools from profile + initial mode so that tools/list
	// is filtered from session start (not only after the first switch_mode call).
	initialAllowedTools := resolveAllowedToolsForMode(profileStore, activeProfile, initialMode)
	sessionProvider := &daemonSessionProvider{
		session: &helixMCP.SessionInfo{
			// IN-03: report the RESOLVED profile, not the raw cfg.Profile.
			// ResolveProfile falls back to the "full" default when cfg.Profile
			// does not resolve, so AllowedTools/initialMode derive from
			// activeProfile; reporting cfg.Profile here would name a profile that
			// does not match the basis of any PermissionDenied refusal or the
			// telemetry profile label, misleading an operator debugging a refusal.
			Profile:      activeProfile.Name,
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
	// Phase 76 D-09: SKIP entirely under effDisableLSP so the repomap engine
	// falls back to tree-sitter only — enrichRepoMapFromLSP is the seam that
	// would lease an LS worker and emit lspool.lsp.* spans.
	if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
		if effDisableLSP {
			// Explicit null-object: clear any previously-set enrich fn so the
			// wiring is idempotent (the skill is a process-global singleton).
			rs.SetEnrichFn(nil)
		} else {
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
	}

	// 12c. Wire repomap skill fallback extraction for non-tree-sitter languages (RMAP-02).
	// Phase 76 D-10 audit: the AcquireFn here calls k.Pool().AcquireLease and
	// is NOT profile-gated, so under effDisableLSP the whole SetFallbackDeps
	// wiring is skipped — leaving the skill without a pool-leasing fallback so
	// no live worker (and no lspool.lsp.* span) is reachable on the no_lsp arm.
	if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
		if effDisableLSP {
			// Explicit null-object: clear any previously-set pool-leasing
			// fallback deps so the wiring is idempotent on the singleton.
			rs.SetFallbackDeps(nil)
		} else {
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

	// 12e. Wire repomap skill SemanticLookup + ConfigGate (Phase 65 65-05,
	// INTEG-01 / INTEG-02 / INTEG-05). When sBndl is non-nil (semantic
	// enabled and the bundle constructed), pass the production
	// integSemanticLookup adapter through the integLookupAccessor accessor
	// declared in semantic_wiring.go. When sBndl is nil (semantic disabled),
	// SetSemanticLookup is skipped — the skill's lookup() accessor normalizes
	// nil to integ.NoopLookup{} so the priority-ladder gate at ChooseSource
	// always sees a valid SemanticLookup.
	//
	// The ConfigGate is wired ALWAYS (even when sBndl is nil) so the v1.9
	// steady-state path renders source="tree_sitter" instead of
	// source="fallback" + reason="index_disabled" when the feature is off
	// (D-04 / Pitfall §3).
	//
	// Phase 81 ABLATE-06 gate point 3 (repomap, get_repo_map / get_context):
	// the ConfigGate is gated DISABLED under effSemanticDisabled (Pitfall 4 →
	// source=tree_sitter), and the SemanticLookup is EXPLICITLY nulled
	// (idempotent null-object, Pitfall 5 — mirror SetEnrichFn(nil) above) rather
	// than skipping the setter, so a stale real lookup cannot persist on the
	// process-global RepoMapSkill singleton.
	if rs := repomapSkill.GetRepoMapSkill(); rs != nil {
		rs.SetConfigGate(gatedCfgGate(cfg, effSemanticDisabled))
		if effSemanticDisabled {
			rs.SetSemanticLookup(nil)
		} else if sBndl != nil {
			rs.SetSemanticLookup(sBndl.integLookupAccessor())
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

	// 14b.5. Phase 66 (GUARD-01): install Guardrail middleware AFTER Suggestion
	// and BEFORE LazyInit so LIFO execution = LazyInit → Guardrail → Suggestion
	// → ProfileFilter → Telemetry → handler. LazyInit-last invariant preserved
	// (see internal/mcp/lazy_init.go:106-112).
	{
		var guardrailMetricsSink guardrails.MetricsSink
		if m := observability.Metrics(); m != nil {
			guardrailMetricsSink = metricsReceiptSink{m: m}
		}
		guardrailStore := guardrails.NewStore(guardrails.StoreOptions{
			Metrics: guardrailMetricsSink,
			Now:     time.Now,
		})
		// F-08: wire the JSONL emitter so guardrails.Store.Issue emits a
		// tap-compatible "receipt issued" line per receipt, consumed by
		// internal/eval/trace/tap.go in the eval harness.
		guardrails.SetJSONLLogger(logger)
		// Wire the issue sink so read-tool issuance (Plan 05) lights up at runtime.
		// CR-03 fix: thread the active workspace key into the issuance closure so
		// receipts are stored under the correct workspace identity. The closure
		// captures activeWSKey by reference; lazyActivateFn updates it before the
		// first tool call (LazyInit middleware runs first in the LIFO chain).
		guardrails.SetReceiptIssueSink(func(ctx context.Context, class guardrails.ReceiptClass, scope guardrails.ReceiptScope, tool string) (guardrails.ReceiptID, error) {
			return guardrailStore.Issue(activeWSKey, class, scope, guardrails.IssueFields{IssuingTool: tool})
		})
		// Phase 81 ABLATE-06: the guardrail middleware is a FOURTH
		// integLookupAccessor hand-out (the SessionContext.Lookup consumed by
		// rule predicates G-001..G-005). Gate it under effSemanticDisabled so the
		// no_semantic arm sees integ.NoopLookup{} here too (T-81-04-01: no
		// back-channel read consumer may leak a semantic read onto the gated arm).
		var semanticLookup integ.SemanticLookup = integ.NoopLookup{}
		if sBndl != nil && !effSemanticDisabled {
			semanticLookup = sBndl.integLookupAccessor()
		}
		guardrailDeps := newGuardrailDeps(
			semanticLookup,
			newProfileStoreResolver(profileStore),
			cfg.SemanticIndex.Guardrails,
			guardrailStore,
			rules.DefaultEvaluator{},
			noopOutlineProvider{},
			logger,
			observability.Metrics(),
			wsKeyFn,
		)
		helixMCP.InstallGuardrailMiddleware(mcpServer.SDK(), guardrailDeps, getSessionFn, logger)
		// Register store shutdown with the kernel-first shutdown ordering.
		// guardrailStore.Close() is idempotent; safe to call multiple times.
		defer guardrailStore.Close()
		// TODO(phase-66.x): wire guardrailDeps.OnGraphVersionAdvance to the
		// graph_version publish/subscribe seam from Phase 62. For v1.10 the
		// receipt store relies on TTL-only expiry; graph_version invalidation
		// is available via OnGraphVersionAdvance but not yet auto-triggered.
		logger.Info("guardrail middleware installed (Phase 66 GUARD-01)")
		// WR-03: surface the production-stub gap to operators. The G-001
		// rule predicate cannot match a symbol against an outline when the
		// outline provider is a no-op, so G-001 effectively allows every
		// fuzzy_edit / replace_in_file in production until a real adapter
		// lands. The other four rules (G-002..G-005) are unaffected.
		logger.Warn("guardrail G-001 outline provider is a no-op stub — G-001 is structurally Allow in production until a tree-sitter outline adapter is wired (TODO phase-66.x)")
	}

	// 14b.6. Phase 91 (SEC-01): install the tools/call profile/mode enforcement
	// middleware AFTER Guardrail (14b.5) and BEFORE LazyInit (14c) so LIFO
	// execution = LazyInit → ProfileEnforce → Guardrail → Suggestion →
	// ProfileFilter → Telemetry → handler. LazyInit-first invariant preserved
	// (see internal/mcp/lazy_init.go:106-112); ProfileEnforce runs before
	// Guardrail so an out-of-profile call is refused before receipt evaluation.
	// Reuses the SAME getSessionFn closure as Telemetry/ProfileFilter/Guardrail
	// (single source of truth for session state). Authz is server-side because
	// the daemon session is authoritative and a CLI-only check is bypassable
	// (T-91-05, T-91-06, T-91-07).
	helixMCP.InstallProfileEnforcementMiddleware(mcpServer.SDK(), getSessionFn, logger)
	logger.Info("profile/mode tools/call enforcement middleware installed (Phase 91 SEC-01)")

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

		// F-05 parity: lazy-activate MUST also schedule initial extraction so
		// semantic queries against lazy-activated workspaces are not empty
		// (mirrors the explicit-activate path in SetActivateCallback below).
		// Reason string distinguishes the lazy path from the explicit one so
		// operators can tell which trigger fired extraction.
		//
		// Phase 81 Plan 07 (CR-01): ScheduleInitialExtraction is a semantic
		// read-DRIVER (the initial walk reads the COUNTED store) — gate it on
		// effSemanticDisabled here too so the lazy-activate path (not just the
		// explicit SetActivateCallback below) drives ZERO back-channel reads on
		// the no_semantic arm. Build-but-block preserved (the store stays open).
		if semanticScheduler != nil && !backgroundSemanticReadsDisabled(effSemanticDisabled) {
			semanticScheduler.ScheduleInitialExtraction(
				semantic.WorkspaceID(repoPath),
				scheduler.InitialExtraction{
					Reason: "workspace_activation_lazy",
					Mode:   scheduler.ModeAuto,
				},
			)
		}
		return nil
	}
	isActiveFn := func() bool { return activeWSKey.RepoRoot != "" }
	helixMCP.InstallLazyInitMiddleware(mcpServer.SDK(), lazyActivateFn, isActiveFn, "", logger)

	// 14d. Phase 64 P64-08: wire the semantic skill's SessionAccessor with
	// the SAME getSessionFn closure passed to InstallMiddleware (single
	// source of truth — closes checker W2 at the production layer). Then
	// register the four MCP tools (index/refresh/status/context) so they
	// surface in tools/list and route to the skill's typed-args handlers.
	if sBndl != nil {
		sBndl.SetSessionFn(getSessionFn)
		if sBndl.skill != nil {
			tracer := observability.Tracer()
			semanticpkg.RegisterAll(mcpServer, sBndl.skill, tracer)
			logger.Info("semantic MCP tools registered",
				"tools", []string{
					"index_semantic_graph",
					"refresh_semantic_graph",
					"get_semantic_graph_status",
					"get_semantic_context",
				},
			)
		}
	}

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
		// Phase 81 Plan 07 (CR-01): the five SetActivateCallback semantic
		// read-DRIVERS below reach the COUNTED DuckDB chokepoint
		// (ScheduleInitialExtraction → extraction reads; startWorkspace →
		// incremental reads; ensureScheduler → QueryEffectiveAdjacency /
		// CountStaleScoreRows; ensureCompactor / ensureRetrieval → store
		// reads). Under build-but-block (D-04) the store + bundle are NON-nil,
		// so the existing nil-checks do NOT stop them on a store-ON no_semantic
		// arm. Gate them ALL on effSemanticDisabled so the no_semantic arm
		// drives ZERO back-channel reads. The kernel workspace activation +
		// repomap root + session language above are NOT semantic-store reads
		// and stay ungated.
		bgReadsDisabled := backgroundSemanticReadsDisabled(effSemanticDisabled)
		// Phase 59 P03: kick the initial-walk extraction non-blockingly
		// (D-04). Idempotent — repeat activations of the same workspace
		// return the already-in-flight JobID.
		if semanticScheduler != nil && !bgReadsDisabled {
			semanticScheduler.ScheduleInitialExtraction(
				semantic.WorkspaceID(repoPath),
				scheduler.InitialExtraction{
					Reason: "workspace_activation",
					Mode:   scheduler.ModeAuto,
				},
			)
		}
		// Phase 60 D-05/D-06: per-workspace live-update lifecycle. nil
		// bundle means LiveUpdates is disabled — startWorkspace is a
		// no-op in that case.
		if !bgReadsDisabled {
			live.startWorkspace(ctx, activeWSKey, logger)
		}

		// Phase 62 P03: lazy-construct the per-workspace RankScheduler
		// the first time a workspace activates. The scheduler.Run
		// goroutine is launched inside ensureScheduler with the
		// activation ctx; cancellation propagates via the daemon's
		// top-level errgroup-attached ctx (rank.Run owns the ctx the
		// scheduler closures capture). nil-safe: when rank == nil the
		// helper short-circuits.
		if rank != nil && !bgReadsDisabled {
			rank.ensureScheduler(ctx, repoPath)
		}
		// Phase 63 P63-02: lazy-construct the per-workspace compactor
		// alongside the rank scheduler. nil-safe: the helper short-
		// circuits when compactBndl is nil (semantic disabled).
		if compactBndl != nil && !bgReadsDisabled {
			compactBndl.ensureCompactor(ctx, repoPath, activeWSKey)
		}
		// Phase 64 P64-08: lazy-construct the per-workspace bleve
		// retrieval engine + recovery probe. nil-safe: short-circuits
		// when sBndl is nil (semantic disabled). The probe runs in its
		// own goroutine inside ensureRetrieval — non-blocking.
		if sBndl != nil && !bgReadsDisabled {
			sBndl.ensureRetrieval(ctx, activeWSKey)
		}
		logger.Info("kernel workspace activated",
			"root", repoPath,
			"languages", rt.Languages(),
		)
		return nil
	})

	return &Daemon{
		config:                  cfg,
		logger:                  logger,
		obs:                     observability,
		workspaces:              workspaces,
		mcpServer:               mcpServer,
		kernel:                  k,
		langRegistry:            langReg,
		profileStore:            profileStore,
		activeProfile:           activeProfile,
		diagStore:               diagStore,
		bodyExtractor:           bodyExtractor,
		semanticStore:           semanticStore,
		semanticExtractRegistry: semanticExtractRegistry,
		grammarRegistry:         grammarRegistry,
		semanticScheduler:       semanticScheduler,
		live:                    live,
		rank:                    rank,
		compact:                 compactBndl,
		semantic:                sBndl,
		typeResolver:            typeResolver,
		effSemanticDisabled:     effSemanticDisabled,
	}, nil
}

// SemanticStore returns the semantic fact store, or nil when
// cfg.SemanticIndex.Enabled is false. Downstream consumers (Phase 64+
// MCP tools) MUST nil-check.
func (d *Daemon) SemanticStore() *semanticstore.Store { return d.semanticStore }

// semanticStoreProbe adapts *semanticstore.Store to the
// internal/kernel/health.SemanticStoreProbe interface (SC-1). Defined
// here — not in health/ — so the kernel package stays free of
// internal/semantic imports.
//
// Phase 65 65-11 Task 2 (WR-2 / IN-04): the probe holds an optional
// bundle pointer so the Probe-time SELECT 1 failure path can stamp the
// bundle's lastErrReason via SetLastErrorReason. The bundle pointer is
// optional (nil-safe via the bundle's setter) — when semantic is
// disabled (sBndl == nil), Probe() still returns the closed-enum
// ErrUnsupported and kernel/health classifies it via the existing
// classifySemanticProbeError code path.
type semanticStoreProbe struct {
	s      *semanticstore.Store
	bundle *semanticBundle
}

// Available reports whether the underlying store is functional. Nil-safe.
func (p semanticStoreProbe) Available() bool {
	return p.s != nil && p.s.Available()
}

// Probe runs a SELECT 1 against the underlying *sql.DB. The store's
// DB() accessor is nil-safe.
//
// IN-NEW-02: ComputeSemanticStoreStatus gates Probe behind Available(),
// and Available() returns true only when p.s != nil AND p.s.DB() != nil
// (see duckdb.go Store.DB contract). The nil-DB branch is therefore
// unreachable and has been removed; the surviving p.s == nil branch is
// wrapped with serr.ErrUnsupported so callers (and the closed-enum
// classifier in kernel/health) can distinguish a "feature disabled"
// state from a transient DB failure.
//
// Phase 65 65-11 Task 2 (WR-2 / IN-04): on a SELECT 1 failure (the
// "live store handle present but the underlying connection is sick"
// path), the probe stamps the bundle's lastErrReason via
// SetLastErrorReason(integ.FallbackReasonIndexError) so a subsequent
// get_health.semantic_index.last_error reports the closed-enum reason.
// On success, the stamp is cleared (setter accepts an empty reason).
func (p semanticStoreProbe) Probe(ctx context.Context) error {
	if p.s == nil {
		// WR-2: pre-store-construction path; the bundle does not exist
		// here either (sBndl is nil when semantic is disabled), so we
		// can't stamp. Operator-visible via the existing log line.
		return fmt.Errorf("semantic store unavailable: %w", serr.ErrUnsupported)
	}
	db := p.s.DB()
	var one int
	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		// WR-2 / IN-04: stamp FallbackReasonIndexError on Probe-time
		// SQL failure so get_health.semantic_index.last_error surfaces
		// the closed-enum reason (NOT the raw SQL error text — that
		// would leak DB internals through WR-NEW-01's envelope shield).
		p.bundle.SetLastErrorReason(integ.FallbackReasonIndexError)
		return err
	}
	// WR-2: clear the stamp on probe success so a previously-stamped
	// transient error stops surfacing once the store recovers.
	p.bundle.SetLastErrorReason("")
	return nil
}

// daemonSemIndexAccessor adapts an integ.SemanticLookup to the
// internal/kernel/health.SemanticIndexAccessor interface (Phase 65 65-07
// INTEG-04). Defined here — not in health/ — so the kernel package stays
// free of internal/semantic concretions; the kernel only sees the integ
// value-types boundary.
//
// The lookup field is normalized to integ.NoopLookup{} at construction in
// daemon.go (step 10 / health.RegisterTools wiring) so Status always has
// a non-nil delegate. NoopLookup.Status returns
// (SemanticStatus{}, integ.ErrIndexErrored), which kernel/health's
// ComputeSemanticIndexBlock surfaces as a closed-enum
// LastError="index_error" (WR-NEW-01).
//
// M-readtier: the underlying integSemanticLookup.Status method body is
// guarded by the grep canary at internal/daemon/integ_lookup_test.go;
// this adapter delegates without adding any write-method tokens.
type daemonSemIndexAccessor struct {
	lookup integ.SemanticLookup
}

// Status delegates to the underlying SemanticLookup. Read-only by
// contract (M-readtier).
func (a *daemonSemIndexAccessor) Status(ctx context.Context, ws workspace.WorkspaceKey) (integ.SemanticStatus, error) {
	if a == nil || a.lookup == nil {
		return integ.SemanticStatus{}, integ.ErrIndexErrored
	}
	return a.lookup.Status(ctx, ws)
}

// Compile-time guard: daemonSemIndexAccessor must satisfy
// health.SemanticIndexAccessor.
var _ health.SemanticIndexAccessor = (*daemonSemIndexAccessor)(nil)

// MCPServer returns the MCP server for test wiring (e.g., SDK().Connect).
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

	// Phase 61 P03: LSP enrichment manager runs in the errgroup. When
	// LSPEnrichment is disabled, live.Run blocks until ctx done so the
	// errgroup goroutine doesn't return early and tear down its siblings.
	if d.live != nil {
		g.Go(func() error {
			return d.live.Run(gctx)
		})
	}

	// Phase 62 P03: rank engine fan-out goroutine. Demuxes
	// graph.GraphVersionAdvance events from Engine.notifyVersion into the
	// correct per-workspace RankScheduler.Notify. Per-workspace
	// scheduler.Run goroutines are launched lazily by
	// rank.ensureScheduler under the activation ctx; cancellation cascades
	// via the closures' captured gctx.
	if d.rank != nil {
		g.Go(func() error {
			return d.rank.Run(gctx)
		})
	}

	// Phase 63 P63-02: compaction-bundle Run blocks on ctx.Done(); the
	// per-workspace compactor goroutines spun up by ensureCompactor
	// share the same gctx via the bundle's runCtx field.
	if d.compact != nil {
		g.Go(func() error {
			return d.compact.Run(gctx)
		})
	}

	// Phase 64 P64-08: semantic-bundle Run blocks on ctx.Done() and
	// closes every bleve handle + the IndexRunner on shutdown. Per-
	// workspace bleve engines are constructed lazily by ensureRetrieval
	// inside the SetActivateCallback closure.
	if d.semantic != nil {
		g.Go(func() error {
			return d.semantic.Run(gctx)
		})
	}

	// Unix socket listener for forwarder connections via gRPC (DMN-03)
	g.Go(func() error {
		return d.listenSocket(gctx)
	})

	// Optional loopback gRPC TCP listener for split-host CLI↔daemon use
	// (Phase 94 RETIRE-04). Gated on GRPCAddr != "" like listenAdmin;
	// default empty = unix-socket only. Serves the SAME ForwarderService.
	//
	// WR-01: a transient bind error (e.g. EADDRINUSE / TIME_WAIT collision on
	// restart) on this OPTIONAL listener must NOT tear down the daemon and kill
	// the working unix-socket transport. Mirror listenAdmin's resilience: log
	// and continue. The non-loopback VALIDATION error is fatal-early at the CLI
	// composition root (runDaemon → daemon.ValidateGRPCAddr, WR-02), so by the
	// time we reach here a non-nil return can only be a transport/bind failure
	// worth degrading on, never a misconfiguration worth refusing to start for.
	if d.config.Daemon.GRPCAddr != "" {
		g.Go(func() error {
			if err := d.listenGRPCTCP(gctx); err != nil && !errors.Is(err, context.Canceled) {
				d.logger.Error("grpc tcp listener failed; continuing on unix socket",
					"error", err,
					"addr", d.config.Daemon.GRPCAddr)
			}
			return nil // do not tear down the working unix transport
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
		"grpc_addr", d.config.Daemon.GRPCAddr,
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

	// Create and register gRPC server with OTel tracing propagation (D-12 /
	// Phase 58 D-06). obs.ServerStatsHandler centralises the TracerProvider
	// + WithPropagators(TraceContext{}) option set so the client and server
	// sites cannot drift — see internal/obs/grpc.go.
	d.grpcServer = grpc.NewServer(
		grpc.StatsHandler(obs.ServerStatsHandler(d.obs.TracerProvider())),
	)
	serenav1.RegisterForwarderServiceServer(d.grpcServer, d.newForwarderServiceHandler())

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

	// live is the daemon's live-update + Phase 61 enrichment-manager
	// bundle. nil when LiveUpdates is disabled. DeactivateWorkspace
	// invokes live.OnWorkspaceDeactivate(wsKey) so cached enrichment
	// leases for the workspace are released promptly (B2 fix-path-A).
	live *liveBundle

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
	//
	// Phase 61 P03 (B2): release the enrichment manager's cached per-
	// (wsKey, lang) leases for this workspace so the next workspace cycle
	// re-acquires fresh leases (CONTEXT lines 492-494). Daemon Stop
	// remains the safety-net fallback that releases any leases left when
	// individual deactivation events were not delivered.
	if h.live != nil {
		h.live.OnWorkspaceDeactivate(workspace.WorkspaceKey{RepoRoot: wsPath})
	}
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
