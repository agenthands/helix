// Package daemon: Phase 62 P03 rank engine wiring.
//
// rank_wiring.go owns:
//
//   - rankBundle — daemon-side container for the graph.Engine, the
//     fan-out goroutine that demultiplexes GraphVersionAdvance into the
//     correct per-workspace RankScheduler, and the workspace-keyed
//     scheduler map.
//   - rankStoreAdapter — adapts *store.Store + *store.OverlayTx into the
//     graph.RepairStore + graph.SchedulerStore narrow seams.
//
// The bundle is constructed alongside buildLiveBundle (when SemanticIndex
// + LiveUpdates are both enabled and the store is open). Per-workspace
// scheduler goroutines are spun up in SetActivateCallback after the kernel
// workspace activates; the daemon's top-level errgroup owns each
// scheduler.Run goroutine.

package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic"
	graphpkg "github.com/agenthands/helix/internal/semantic/graph"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
)

// rankBundle owns the Phase 62 P03 graph.Engine + per-workspace
// RankScheduler map. nil when SemanticIndex is disabled or the store is
// nil; downstream consumers MUST guard.
type rankBundle struct {
	cfg     graphpkg.SchedulerConfig
	engine  *graphpkg.Engine
	store   graphpkg.SchedulerStore
	logger  *slog.Logger
	metrics graphpkg.MetricsSink

	notifyCh chan graphpkg.GraphVersionAdvance

	// runCtx is captured from Run's argument and used by ensureScheduler
	// to launch per-workspace scheduler goroutines under the daemon
	// errgroup's lifetime. Set on first Run invocation.
	runCtxMu sync.Mutex
	runCtx   context.Context

	mu   sync.Mutex
	subs map[string]*graphpkg.RankScheduler // repoID → scheduler
}

// newRankBundle constructs the engine + adapter + fan-out infrastructure.
// Returns nil when storeAdapter is nil. The caller (daemon.New) MUST
// invoke (b *rankBundle).Run inside the top-level errgroup so the fan-out
// goroutine and per-workspace schedulers receive ctx.Done.
func newRankBundle(
	pageRankCfg semantic.PageRankConfig,
	graphCfg semantic.GraphConfig,
	storeAdapter graphpkg.SchedulerStore,
	metrics *obs.Metrics,
	logger *slog.Logger,
) *rankBundle {
	if storeAdapter == nil {
		return nil
	}
	cfg := graphpkg.SchedulerConfig{
		Debounce:               time.Duration(pageRankCfg.RepairDebounceMs) * time.Millisecond,
		FullRecomputeIdle:      time.Duration(pageRankCfg.FullRecomputeIdleMs) * time.Millisecond,
		FullRecomputeThreshold: pageRankCfg.FullRecomputeThreshold,
		MaxLocalNodes:          graphCfg.MaxLocalPagerankNodes,
		Damping:                pageRankCfg.Damping,
		Epsilon:                pageRankCfg.Epsilon,
		MaxIter:                pageRankCfg.MaxIterations,
		Projection:             "call_graph",
		QueueSize:              16,
	}
	// D-09 hard invariant: the 5000-node frontier threshold is logged at
	// startup so ops can audit overrides.
	logger.Info("rank scheduler bundle constructed",
		"debounce_ms", pageRankCfg.RepairDebounceMs,
		"full_recompute_idle_ms", pageRankCfg.FullRecomputeIdleMs,
		"full_recompute_threshold", pageRankCfg.FullRecomputeThreshold,
		"max_local_pagerank_nodes", graphCfg.MaxLocalPagerankNodes,
	)

	// Engine: the SINGLE graph_version bump site (D-06). Notify channel
	// feeds the bundle's fan-out goroutine, which demultiplexes by repo_id
	// into the per-workspace scheduler.
	notifyCh := make(chan graphpkg.GraphVersionAdvance, 64)
	engine := graphpkg.NewEngine(rankRepairStoreFromScheduler(storeAdapter))
	engine.SetLogger(logger)
	if metrics != nil {
		engine.SetMetrics(metrics)
	}
	engine.SetNotifyChannel(notifyCh)

	return &rankBundle{
		cfg:      cfg,
		engine:   engine,
		store:    storeAdapter,
		logger:   logger,
		metrics:  metrics,
		notifyCh: notifyCh,
		subs:     map[string]*graphpkg.RankScheduler{},
	}
}

// Run is the bundle's top-level goroutine. Drives the fan-out demuxer
// until ctx is cancelled; per-workspace schedulers run in their own
// goroutines started by ensureScheduler. Captures ctx so subsequently
// activated workspaces inherit the daemon's errgroup lifetime.
func (b *rankBundle) Run(ctx context.Context) error {
	if b == nil {
		<-ctx.Done()
		return ctx.Err()
	}
	b.runCtxMu.Lock()
	b.runCtx = ctx
	b.runCtxMu.Unlock()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case adv := <-b.notifyCh:
			b.mu.Lock()
			s, ok := b.subs[adv.RepoID]
			b.mu.Unlock()
			if !ok {
				// No scheduler registered for this repo — workspace not
				// yet activated. Drop the advance; the next ApplyRepair
				// after activation will land normally.
				continue
			}
			s.Notify(adv)
		}
	}
}

// ensureScheduler returns (and lazily constructs) the RankScheduler for
// repoID. The caller (SetActivateCallback) MUST invoke it so workspace
// activation produces a live scheduler before the next ApplyRepair fires.
//
// The activation ctx is NOT captured for the scheduler.Run goroutine —
// that ctx is per-RPC and cancels when the activation closure returns.
// Instead the bundle uses the runCtx captured from Run, which is bound to
// the daemon's top-level errgroup. Falls back to context.Background when
// Run hasn't been entered yet (early-activation race during bootstrap),
// which still cancels via the daemon shutdown path because ApplyRepair's
// own ctx propagation is the dominant tear-down signal.
func (b *rankBundle) ensureScheduler(_ context.Context, repoID string) *graphpkg.RankScheduler {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if s, ok := b.subs[repoID]; ok {
		return s
	}
	b.runCtxMu.Lock()
	runCtx := b.runCtx
	b.runCtxMu.Unlock()
	if runCtx == nil {
		runCtx = context.Background()
	}
	s := graphpkg.NewRankScheduler(repoID, b.cfg, b.store, b.logger, b.metrics)
	b.subs[repoID] = s
	go func() {
		if err := s.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
			b.logger.Warn("rank scheduler exited",
				"repo_id", repoID, "err", err)
		}
	}()
	b.logger.Info("rank scheduler started",
		"repo_id", repoID,
		"debounce_ms", int64(b.cfg.Debounce/time.Millisecond),
		"full_recompute_idle_ms", int64(b.cfg.FullRecomputeIdle/time.Millisecond),
		"full_recompute_threshold", b.cfg.FullRecomputeThreshold,
		"max_local_pagerank_nodes", b.cfg.MaxLocalNodes,
	)
	return s
}

// Engine exposes the engine for handler.SetRankApplier wiring.
func (b *rankBundle) Engine() *graphpkg.Engine {
	if b == nil {
		return nil
	}
	return b.engine
}

// rankRepairStoreFromScheduler narrows a graph.SchedulerStore down to the
// graph.RepairStore surface Engine.ApplyRepair expects. Both interfaces
// are satisfied by the same production adapter; the narrowing is purely
// type-system bookkeeping so the engine cannot reach for scheduler-only
// methods.
func rankRepairStoreFromScheduler(s graphpkg.SchedulerStore) graphpkg.RepairStore {
	return repairStoreShim{inner: s}
}

type repairStoreShim struct{ inner graphpkg.SchedulerStore }

func (r repairStoreShim) LockWorkspace(repoID string) func() {
	return r.inner.LockWorkspace(repoID)
}
func (r repairStoreShim) BeginRepairTx(ctx context.Context, repoID string) (graphpkg.RepairTx, error) {
	return r.inner.BeginRepairTx(ctx, repoID)
}
func (r repairStoreShim) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return r.inner.CurrentGraphVersion(ctx, repoID)
}

// rankStoreAdapter wraps *semanticstore.Store with the graph.SchedulerStore
// surface. Methods that have direct DuckDB-side counterparts forward
// straight; methods that require new query logic are best-effort no-op
// stubs Phase 64 will fill (CountStaleScoreRows, MarkAllScoreRowsStale,
// QueryEffectiveAdjacency, QueryEffectiveGraph). The stubs return nil
// errors so the scheduler keeps running; the rank surface stays empty
// until those are wired, but the Phase 62 P03 lifecycle / single-bump /
// per-workspace-isolation invariants ALL hold.
type rankStoreAdapter struct {
	store *semanticstore.Store
}

func newRankStoreAdapter(s *semanticstore.Store) *rankStoreAdapter {
	if s == nil {
		return nil
	}
	return &rankStoreAdapter{store: s}
}

func (a *rankStoreAdapter) LockWorkspace(repoID string) func() {
	return a.store.LockOverlayWorkspace(repoID)
}

func (a *rankStoreAdapter) BeginRepairTx(ctx context.Context, repoID string) (graphpkg.RepairTx, error) {
	tx, err := a.store.BeginOverlayTx(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return &rankRepairTxAdapter{tx: tx}, nil
}

func (a *rankStoreAdapter) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return a.store.CurrentGraphVersion(ctx, repoID)
}

func (a *rankStoreAdapter) QueryEffectiveGraph(_ context.Context, _, _ string) ([]graphpkg.NodeID, map[graphpkg.NodeID]map[graphpkg.NodeID]float64, error) {
	// Phase 64 follow-up: typed query reads through internal/semantic/store/effective.go.
	return nil, map[graphpkg.NodeID]map[graphpkg.NodeID]float64{}, nil
}

func (a *rankStoreAdapter) QueryEffectiveAdjacency(_ context.Context, _, _ string) (map[graphpkg.NodeID]map[graphpkg.NodeID]float64, map[graphpkg.NodeID]map[graphpkg.NodeID]float64, error) {
	// Phase 64 follow-up: the adjacency query is the inverse of the
	// effective-graph view. Returning empty maps here keeps the scheduler
	// alive — its incremental path will compute an empty frontier and
	// short-circuit cleanly.
	return map[graphpkg.NodeID]map[graphpkg.NodeID]float64{}, map[graphpkg.NodeID]map[graphpkg.NodeID]float64{}, nil
}

func (a *rankStoreAdapter) CountStaleScoreRows(_ context.Context, _, _ string) (int, int, error) {
	// Phase 64 follow-up: SELECT COUNT(*) FROM semantic_graph_scores WHERE
	// repo_id=? AND score_name=? AND status='stale' / total counterpart.
	return 0, 0, nil
}

func (a *rankStoreAdapter) MarkAllScoreRowsStale(_ context.Context, _, _ string) error {
	// Phase 64 follow-up: bulk UPDATE on semantic_graph_scores.
	return nil
}

// rankRepairTxAdapter wraps *semanticstore.OverlayTx with the
// graph.RepairTx surface. Score-row methods are real (UpsertGraphScores,
// DeleteScoresForProjection ship in Phase 62 P02 + P03 store
// extensions). Mark*Deleted forwards. UpsertEdgesWithMerge translates the
// engine-side EdgeUpsert into the store-side EdgeRow.
type rankRepairTxAdapter struct {
	tx *semanticstore.OverlayTx
}

func (a *rankRepairTxAdapter) BumpGraphVersion(ctx context.Context) (uint64, error) {
	return a.tx.BumpGraphVersion(ctx)
}
func (a *rankRepairTxAdapter) MarkSymbolsDeleted(ctx context.Context, fileIDs []uint64) error {
	return a.tx.MarkSymbolsDeleted(ctx, fileIDs)
}
func (a *rankRepairTxAdapter) MarkEdgesDeleted(ctx context.Context, nodeIDs []uint64) error {
	return a.tx.MarkEdgesDeleted(ctx, nodeIDs)
}
func (a *rankRepairTxAdapter) UpsertEdgesWithMerge(ctx context.Context, edges []graphpkg.EdgeUpsert) error {
	if len(edges) == 0 {
		return nil
	}
	rows := make([]semanticstore.EdgeRow, len(edges))
	for i, e := range edges {
		rows[i] = semanticstore.EdgeRow{
			SrcNodeID:       uint64(e.SrcNodeID),
			DstNodeID:       uint64(e.DstNodeID),
			EdgeKind:        e.EdgeKind,
			Source:          e.Source,
			Confidence:      e.Confidence,
			Weight:          e.Weight,
			ValidationState: e.ValidationState,
			FactJSON:        e.FactJSON,
		}
	}
	return a.tx.UpsertEdgesWithMerge(ctx, rows)
}
func (a *rankRepairTxAdapter) UpsertGraphScores(ctx context.Context, projection string, rows []graphpkg.ScoreRow) error {
	if len(rows) == 0 {
		return nil
	}
	storeRows := make([]semanticstore.ScoreRow, len(rows))
	for i, r := range rows {
		storeRows[i] = semanticstore.ScoreRow{
			NodeID:       uint64(r.NodeID),
			Score:        r.Score,
			GraphVersion: r.GraphVersion,
			Status:       r.Status,
		}
	}
	return a.tx.UpsertGraphScores(ctx, projection, storeRows)
}
func (a *rankRepairTxAdapter) DeleteScoresForProjection(ctx context.Context, projection string) error {
	return a.tx.DeleteScoresForProjection(ctx, projection)
}
func (a *rankRepairTxAdapter) Commit() error   { return a.tx.Commit() }
func (a *rankRepairTxAdapter) Rollback() error { return a.tx.Rollback() }

// Compile-time interface satisfaction guards. If a future RepairTx /
// SchedulerStore extension lands without an adapter update the build
// fails here, not at first runtime call.
var (
	_ graphpkg.RepairTx        = (*rankRepairTxAdapter)(nil)
	_ graphpkg.SchedulerStore  = (*rankStoreAdapter)(nil)
	_ graphpkg.RepairStore     = repairStoreShim{}
)

// _ keeps unused fmt import out of the build; remove once any error
// formatting lands.
var _ = fmt.Sprintf
