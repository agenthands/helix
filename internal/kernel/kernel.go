package kernel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/postfix/serena/internal/kernel/lspool"
	"github.com/postfix/serena/internal/langregistry"
	"github.com/postfix/serena/internal/workspace"
)

// KernelConfig holds configuration for the kernel.
type KernelConfig struct {
	Pool lspool.PoolConfig
}

// Kernel coordinates workspaces, the LS worker pool, and tool dispatch.
// It is the central entry point for code intelligence operations.
type Kernel struct {
	workspaces     map[string]*WorkspaceRuntime // keyed by workspace key hash
	pool           *lspool.Pool
	registry       *workspace.Registry    // Phase 1 workspace registry
	langReg        *langregistry.Registry // language registry for extension-based detection
	config         KernelConfig
	logger         *slog.Logger
	tracer         trace.Tracer // Phase 12: plumbed via constructor, noop-safe
	sessionMetrics SessionMetricsSink
	mu             sync.RWMutex
}

// SetSessionMetricsSink wires the workspace lifecycle sink. Daemon calls
// this in plan 53-03 with *obs.Metrics so activate emits land on the
// owned Prometheus registry. Idempotent and nil-safe.
func (k *Kernel) SetSessionMetricsSink(s SessionMetricsSink) {
	if s == nil {
		s = NoopSessionSink{}
	}
	k.mu.Lock()
	k.sessionMetrics = s
	k.mu.Unlock()
}

// NewKernel creates a new kernel with the given workspace registry, language registry, and configuration.
// The installer provides three-tier LS resolution (PATH/download/error).
// metrics is the lspool.MetricsSink receiving worker lifecycle events; pass
// lspool.NoopSink{} (or nil) to disable.
func NewKernel(registry *workspace.Registry, langReg *langregistry.Registry, installer *langregistry.Installer, cfg KernelConfig, pressure lspool.MemoryPressure, logger *slog.Logger, metrics lspool.MetricsSink, tracer trace.Tracer) *Kernel {
	if tracer == nil {
		tracer = tracenoop.NewTracerProvider().Tracer("kernel-fallback")
	}
	pool := lspool.NewPool(cfg.Pool, langReg, installer, pressure, logger, metrics)
	return &Kernel{
		workspaces:     make(map[string]*WorkspaceRuntime),
		pool:           pool,
		registry:       registry,
		langReg:        langReg,
		config:         cfg,
		logger:         logger.With("component", "kernel"),
		tracer:         tracer,
		sessionMetrics: NoopSessionSink{},
	}
}

// Tracer returns the kernel's trace.Tracer. Never nil — the constructor
// falls back to a noop tracer when none is provided.
func (k *Kernel) Tracer() trace.Tracer { return k.tracer }

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
	// Phase 53 WR-02 idempotency invariant: SessionLifecycleInc(phase="activate")
	// is emitted EXACTLY ONCE per workspace per daemon lifetime. Both
	// lazyActivateFn and SetActivateCallback in the daemon invoke this method;
	// the early return below ensures the metric is not double-counted. Any
	// future refactor that re-detects languages on cache hits MUST preserve
	// this invariant — see TestActivateWorkspace_EmitsActivateOnce in
	// internal/kernel/kernel_test.go.
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

	// Phase 53 D-04: emit one SessionLifecycleInc per detected language so
	// per-language activation rate falls out of PromQL. If detection found
	// no languages we still emit once with an empty label so dashboards can
	// surface "unknown language" workspaces.
	if len(langs) == 0 {
		k.sessionMetrics.SessionLifecycleInc("", PhaseActivate)
	} else {
		for _, lang := range langs {
			k.sessionMetrics.SessionLifecycleInc(lang, PhaseActivate)
		}
	}

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

// ActiveLanguages returns a snapshot of the languages across every
// currently-active WorkspaceRuntime. Used by the daemon's signal-first
// shutdown sweep to emit one SessionLifecycleInc(lang, "shutdown") per
// still-active language before kernel teardown (Phase 53 D-04).
//
// When a workspace was activated but no language was detected, an empty
// string is included so dashboards can still surface the shutdown of an
// "unknown language" workspace (mirrors ActivateWorkspace's empty-label
// emission).
func (k *Kernel) ActiveLanguages() []string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	var out []string
	for _, rt := range k.workspaces {
		langs := rt.Languages()
		if len(langs) == 0 {
			out = append(out, "")
			continue
		}
		out = append(out, langs...)
	}
	return out
}

// LanguagesForRoot returns the languages detected for the workspace
// rooted at the given repository path, or nil if no workspace runtime
// matches. Used by the daemon's gRPC DeactivateWorkspace handler to emit
// SessionLifecycleInc(lang, "deactivate") per detected language.
//
// NOTE: a nil return is ambiguous between "workspace not tracked" and
// "workspace tracked but zero languages detected". Callers that need to
// distinguish these (Phase 53 IN-05: deactivate emission policy) should
// use HasWorkspace separately to disambiguate.
func (k *Kernel) LanguagesForRoot(repoRoot string) []string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	for _, rt := range k.workspaces {
		if rt.Key().RepoRoot == repoRoot {
			return rt.Languages()
		}
	}
	return nil
}

// HasWorkspace reports whether the kernel currently tracks a workspace
// runtime rooted at the given repository path. Used by the daemon's
// gRPC DeactivateWorkspace handler to disambiguate "unknown workspace"
// (skip emit) from "known workspace with zero detected languages"
// (emit with language="" to mirror ActivateWorkspace and the shutdown
// sweep — Phase 53 IN-05 policy alignment).
func (k *Kernel) HasWorkspace(repoRoot string) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()
	for _, rt := range k.workspaces {
		if rt.Key().RepoRoot == repoRoot {
			return true
		}
	}
	return false
}
