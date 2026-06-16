package kernel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/langregistry"
	"github.com/agenthands/helix/internal/workspace"
)

// KernelConfig holds configuration for the kernel.
type KernelConfig struct {
	Pool lspool.PoolConfig
	// DisableLSPSubsystem requests that the LSP subsystem be disabled. When
	// set, the daemon wires no-op LSP seams (D-09) so back-channel LSP calls
	// cannot reach a live language server. Consumed by Plan 76-04.
	DisableLSPSubsystem bool // Phase 76 ABLATE-05
	// DisableStructuredEditSubsystem requests that structured-edit tools be
	// disabled. When set, the four structured-edit handlers return a typed
	// serr.Unsupported error and replace_in_file falls through to exact-match
	// only (no fuzzy cascade). Opt-in disable; default ENABLED (D-02).
	DisableStructuredEditSubsystem bool // Phase 76 ABLATE-07
}

// Kernel coordinates workspaces, the LS worker pool, and tool dispatch.
// It is the central entry point for code intelligence operations.
type Kernel struct {
	workspaces map[string]*WorkspaceRuntime // keyed by workspace key hash
	pool       *lspool.Pool
	registry   *workspace.Registry    // Phase 1 workspace registry
	langReg    *langregistry.Registry // language registry for extension-based detection
	config     KernelConfig
	logger     *slog.Logger
	tracer     trace.Tracer // Phase 12: plumbed via constructor, noop-safe
	mu         sync.RWMutex

	// Phase 60 D-03: optional fire-and-forget callback into the semantic
	// live service. Set by daemon bootstrap via SetEditNotifier; nil
	// until then. See notifier.go.
	editNotifier atomic.Value // editNotifierHolder
}

// NewKernel creates a new kernel with the given workspace registry, language registry, and configuration.
// The installer provides three-tier LS resolution (PATH/download/error).
// metrics is the lspool.MetricsSink receiving worker lifecycle events; pass
// lspool.NoopSink{} (or nil) to disable.
func NewKernel(registry *workspace.Registry, langReg *langregistry.Registry, installer *langregistry.Installer, cfg KernelConfig, pressure lspool.MemoryPressure, logger *slog.Logger, metrics lspool.MetricsSink, tracer trace.Tracer) *Kernel {
	if tracer == nil {
		tracer = tracenoop.NewTracerProvider().Tracer("kernel-fallback")
	}
	pool := lspool.NewPool(cfg.Pool, langReg, installer, pressure, logger, metrics, tracer)
	return &Kernel{
		workspaces: make(map[string]*WorkspaceRuntime),
		pool:       pool,
		registry:   registry,
		langReg:    langReg,
		config:     cfg,
		logger:     logger.With("component", "kernel"),
		tracer:     tracer,
	}
}

// Tracer returns the kernel's trace.Tracer. Never nil — the constructor
// falls back to a noop tracer when none is provided.
func (k *Kernel) Tracer() trace.Tracer { return k.tracer }

// StructuredEditDisabled reports whether the structured-edit subsystem is
// disabled (Phase 76 ABLATE-07). When true, structured-edit tool handlers
// return serr.Unsupported and replace_in_file does exact-match only.
func (k *Kernel) StructuredEditDisabled() bool { return k.config.DisableStructuredEditSubsystem }

// LSPSubsystemDisabled reports whether the LSP subsystem is disabled
// (Phase 76 ABLATE-05). Consumed by the daemon composition root (Plan 76-04)
// to wire no-op LSP seams.
func (k *Kernel) LSPSubsystemDisabled() bool { return k.config.DisableLSPSubsystem }

// ActivateWorkspace detects languages and creates a WorkspaceRuntime for a project root.
// Per WRK-02: auto-detects languages. Per WRK-03: supports multiple projects.
func (k *Kernel) ActivateWorkspace(ctx context.Context, rootPath string) (*WorkspaceRuntime, error) {
	// Create a base workspace key (language will be set per-language).
	baseKey := workspace.WorkspaceKey{
		RepoRoot: rootPath,
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	// Check if already activated.
	hash := baseKey.Hash()
	if rt, ok := k.workspaces[hash]; ok {
		return rt, nil
	}

	// Register with Phase 1 workspace registry.
	if _, err := k.registry.ActivateWorkspace(baseKey); err != nil {
		return nil, fmt.Errorf("activating workspace in registry: %w", err)
	}

	// Create runtime and detect languages.
	rt := NewWorkspaceRuntime(baseKey, k.pool, k.langReg)
	langs := rt.DetectLanguages(rootPath)

	k.workspaces[hash] = rt

	k.logger.Info("workspace activated",
		"root", rootPath,
		"languages", langs,
	)

	return rt, nil
}

// GetRuntime returns the workspace runtime for the given key.
func (k *Kernel) GetRuntime(wsKey workspace.WorkspaceKey) (*WorkspaceRuntime, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	hash := wsKey.Hash()
	rt, ok := k.workspaces[hash]
	if !ok {
		return nil, fmt.Errorf("workspace runtime not found: %s", wsKey)
	}
	return rt, nil
}

// Run starts the LS worker pool and blocks until ctx is cancelled.
func (k *Kernel) Run(ctx context.Context) error {
	k.logger.Info("kernel starting")
	return k.pool.Run(ctx)
}

// Shutdown stops all workers and releases all resources.
func (k *Kernel) Shutdown(ctx context.Context) error {
	k.logger.Info("kernel shutting down")
	k.mu.Lock()
	k.workspaces = make(map[string]*WorkspaceRuntime)
	k.mu.Unlock()
	return nil
}

// HealthStatus returns a health report enriched with workspace language data.
// The pool snapshot is taken under a single RLock; workspace data is merged
// afterward without holding the pool lock.
func (k *Kernel) HealthStatus() *lspool.HealthReport {
	report := k.pool.HealthSnapshot()

	// Enrich workspace entries with detected languages from WorkspaceRuntime.
	k.mu.RLock()
	for i := range report.Workspaces {
		ws := &report.Workspaces[i]
		for _, rt := range k.workspaces {
			if rt.Key().RepoRoot == ws.Root {
				ws.Languages = rt.Languages()
				break
			}
		}
	}
	k.mu.RUnlock()

	return &report
}

// Pool returns the LS worker pool (for direct access if needed).
func (k *Kernel) Pool() *lspool.Pool {
	return k.pool
}
