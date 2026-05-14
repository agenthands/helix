// Package daemon: Phase 63 P63-02 Task 3 — compaction-engine wiring.
//
// compact_wiring.go owns:
//
//   - compactBundle — daemon-side container for the per-workspace
//     compactor map, the shared CompactionGate factory, and the
//     OnCoalescerFlush forwarder that the live Service hooks into.
//   - newCompactBundle constructed alongside buildLiveBundle /
//     buildRankBundle.
//   - compactBundle.Run blocks on ctx.Done() and joins per-compactor
//     goroutines via context cancellation. Mirrors rank_wiring.go.

package daemon

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/compact"
	graphpkg "github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/workspace"
)

// compactBundle owns the daemon-side compaction registry. nil-safe at
// every accessor.
type compactBundle struct {
	cfg     compact.Config
	store   *semanticstore.Store
	logger  *slog.Logger
	metrics compact.MetricsSink
	kernel  *kernel.Kernel

	// Accessor wiring for the gate. Each wraps the corresponding
	// production source (live bundle's coalescer / store / lspenrich
	// queue / rank bundle's schedulers / kernel) into the read-only
	// interface compact.GateDeps consumes.
	live       *liveBundle
	lspQueue   *lspenrich.LaneQueue
	rankBundle *rankBundle

	// bleveMetaFn (Phase 69-03 / STATUS-02) resolves the per-workspace
	// bleve engine as a compact.BleveMeta writer. Bound by Plan 69-05
	// from semantic_wiring.go (closure over semanticBundle.engines).
	// Nil-safe: when unbound, ensureCompactor threads a nil BleveMeta
	// into compact.Deps, which is itself a no-op on the writer side.
	bleveMetaFnMu sync.Mutex
	bleveMetaFn   func(ws workspace.WorkspaceKey) compact.BleveMeta

	runCtxMu sync.Mutex
	runCtx   context.Context

	mu   sync.Mutex
	subs map[string]*compact.Compactor // repoID → compactor
}

// newCompactBundle constructs the bundle. Returns nil when storeAdapter
// is nil (semantic disabled). Mirrors newRankBundle's nil-safety
// contract.
func newCompactBundle(
	maintCfg semantic.MaintenanceConfig,
	liveCfg semantic.LiveUpdatesConfig,
	store *semanticstore.Store,
	live *liveBundle,
	lspQueue *lspenrich.LaneQueue,
	rb *rankBundle,
	k *kernel.Kernel,
	metrics *obs.Metrics,
	logger *slog.Logger,
) *compactBundle {
	if store == nil {
		return nil
	}

	// Resolve durations. semantic_index.live_updates.compact_after_idle_ms
	// is the canonical timer; lsp_compaction_max_wait_ms is the gate's
	// LSP-pending ceiling.
	compactAfterIdle := time.Duration(liveCfg.CompactAfterIdleMS) * time.Millisecond
	if compactAfterIdle <= 0 {
		compactAfterIdle = 5 * time.Second
	}
	lspMaxWait := time.Duration(liveCfg.LSPCompactionMaxWaitMS) * time.Millisecond
	if lspMaxWait <= 0 {
		lspMaxWait = 30 * time.Second
	}
	maxOverlayRows := liveCfg.MaxOverlayFiles * 4
	if maxOverlayRows <= 0 {
		maxOverlayRows = 4000
	}

	vacuumInterval := 168 * time.Hour
	if maintCfg.VacuumInterval != "" {
		if d, err := time.ParseDuration(maintCfg.VacuumInterval); err == nil && d > 0 {
			vacuumInterval = d
		}
	}

	cfg := compact.Config{
		CompactAfterIdle:     compactAfterIdle,
		LSPCompactionMaxWait: lspMaxWait,
		SnapshotRetention:    5,
		MaxOverlayRows:       maxOverlayRows,
		VacuumEnabled:        maintCfg.VacuumEnabled,
		VacuumInterval:       vacuumInterval,
	}

	logger.Info("compact bundle constructed",
		"compact_after_idle_ms", liveCfg.CompactAfterIdleMS,
		"lsp_compaction_max_wait_ms", liveCfg.LSPCompactionMaxWaitMS,
		"max_overlay_rows", maxOverlayRows,
		"vacuum_enabled", maintCfg.VacuumEnabled,
		"vacuum_interval", vacuumInterval,
	)

	return &compactBundle{
		cfg:        cfg,
		store:      store,
		logger:     logger,
		metrics:    metrics,
		kernel:     k,
		live:       live,
		lspQueue:   lspQueue,
		rankBundle: rb,
		subs:       map[string]*compact.Compactor{},
	}
}

// Run blocks until ctx is cancelled. Mirrors rankBundle.Run.
func (b *compactBundle) Run(ctx context.Context) error {
	if b == nil {
		<-ctx.Done()
		return ctx.Err()
	}
	b.runCtxMu.Lock()
	b.runCtx = ctx
	b.runCtxMu.Unlock()
	<-ctx.Done()
	return ctx.Err()
}

// ensureCompactor lazily constructs the per-workspace Compactor on
// first activation. Mirrors rankBundle.ensureScheduler.
func (b *compactBundle) ensureCompactor(_ context.Context, repoID string, ws workspace.WorkspaceKey) *compact.Compactor {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if c, ok := b.subs[repoID]; ok {
		return c
	}
	b.runCtxMu.Lock()
	runCtx := b.runCtx
	b.runCtxMu.Unlock()
	if runCtx == nil {
		runCtx = context.Background()
	}

	// Build the gate's accessor wiring. Each producer was extended in
	// Task 1 with the read-only accessor the gate consumes.
	gate := compact.NewCompactionGate(repoID, compact.GateDeps{
		Coalescer:  &coalescerAccessor{live: b.live, ws: ws},
		OverlayTx:  b.store,
		OverlayRow: b.store,
		LSPQueue:   &lspQueueAccessor{q: b.lspQueue},
		Scheduler:  &schedulerAccessor{rb: b.rankBundle},
		KernelEdit: b.kernel,
	}, compact.GateConfig{
		CompactAfterIdle:     b.cfg.CompactAfterIdle,
		LSPCompactionMaxWait: b.cfg.LSPCompactionMaxWait,
	}, nil)

	// Resolve the per-workspace BleveMeta writer at construction time.
	// The engine handle for a workspace is stable for the compactor's
	// lifetime (same pattern as b.live, b.store), so a single resolution
	// here matches how every other dep is threaded. nil until Plan 69-05
	// binds bleveMetaFn — nil-safe on the compactor side.
	b.bleveMetaFnMu.Lock()
	resolveBleveMeta := b.bleveMetaFn
	b.bleveMetaFnMu.Unlock()
	var bleveMeta compact.BleveMeta
	if resolveBleveMeta != nil {
		bleveMeta = resolveBleveMeta(ws)
	}

	c := compact.NewCompactor(ws, repoID, b.cfg, compact.Deps{
		Gate:       gate,
		Store:      b.store,
		OverlayOps: b.store,
		Metrics:    b.metrics,
		Logger:     b.logger,
		BleveMeta:  bleveMeta,
	})
	b.subs[repoID] = c
	go func() {
		if err := c.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
			b.logger.Warn("compactor exited", "repo_id", repoID, "err", err)
		}
	}()
	b.logger.Info("compactor started", "repo_id", repoID)
	return c
}

// OnCoalescerFlush forwards a post-flush signal into the per-workspace
// compactor's OnFlush (which resets its AfterFunc timer). nil-safe.
func (b *compactBundle) OnCoalescerFlush(ws workspace.WorkspaceKey) {
	if b == nil {
		return
	}
	c := b.ensureCompactor(context.Background(), ws.RepoRoot, ws)
	if c != nil {
		c.OnFlush()
	}
}

// SetBleveMetaFn binds the per-workspace BleveMeta resolver — the
// closure that maps a workspace key to its bleve engine handle.
// Called from daemon post-init (Plan 69-05) once the semantic engines
// map is populated. nil-safe (clears the binding); idempotent — calling
// SetBleveMetaFn after compactors have already been constructed does
// NOT retroactively rewire them (each compactor captured its BleveMeta
// at NewCompactor time). For ordering, post-init wiring MUST call
// SetBleveMetaFn before the first workspace activates a compactor.
func (b *compactBundle) SetBleveMetaFn(fn func(workspace.WorkspaceKey) compact.BleveMeta) {
	if b == nil {
		return
	}
	b.bleveMetaFnMu.Lock()
	b.bleveMetaFn = fn
	b.bleveMetaFnMu.Unlock()
}

// coalescerAccessor adapts a liveBundle's per-workspace coalescer to
// compact.CoalescerAccessor. Returns the zero time when the coalescer
// hasn't been instantiated for ws yet.
type coalescerAccessor struct {
	live *liveBundle
	ws   workspace.WorkspaceKey
}

// LastFlushAt looks up the per-ws coalescer through the live service's
// internal map. We cannot reach the coalescer pointer directly without
// expanding service.Service's surface; instead, we approximate by
// reading the metadata stamp from the live service if exposed, else
// return zero (which the gate treats as "ready" — see gate.go).
func (a *coalescerAccessor) LastFlushAt() time.Time {
	if a == nil || a.live == nil {
		return time.Time{}
	}
	return a.live.LastFlushAt(a.ws)
}

// lspQueueAccessor adapts *lspenrich.LaneQueue.
type lspQueueAccessor struct{ q *lspenrich.LaneQueue }

func (a *lspQueueAccessor) Depth() int {
	if a == nil || a.q == nil {
		return 0
	}
	return a.q.DepthAll()
}

func (a *lspQueueAccessor) LastEnqueueAt() time.Time {
	if a == nil || a.q == nil {
		return time.Time{}
	}
	return a.q.LastEnqueueAt()
}

// schedulerAccessor adapts the per-workspace rank scheduler. Looks up
// the scheduler by repoID; nil-safe for workspaces that haven't started
// a scheduler yet.
type schedulerAccessor struct {
	rb *rankBundle
}

func (a *schedulerAccessor) IsQuiescent(ws workspace.WorkspaceKey) bool {
	if a == nil || a.rb == nil {
		return true
	}
	a.rb.mu.Lock()
	s, ok := a.rb.subs[ws.RepoRoot]
	a.rb.mu.Unlock()
	if !ok || s == nil {
		return true
	}
	return s.IsQuiescent()
}

// Compile-time guards.
var (
	_ compact.CoalescerAccessor  = (*coalescerAccessor)(nil)
	_ compact.LSPQueueAccessor   = (*lspQueueAccessor)(nil)
	_ compact.SchedulerAccessor  = (*schedulerAccessor)(nil)
	_ compact.OverlayTxAccessor  = (*semanticstore.Store)(nil)
	_ compact.OverlayRowAccessor = (*semanticstore.Store)(nil)
	_ compact.KernelEditAccessor = (*kernel.Kernel)(nil)
	_ compact.SnapshotStore      = (*semanticstore.Store)(nil)
	_ compact.OverlayOps         = (*semanticstore.Store)(nil)
)

// guard against unused-warning when compact.GraphScheduler is the only
// thing referenced from graphpkg in this file.
var _ = graphpkg.NewRankScheduler
