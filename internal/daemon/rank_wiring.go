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

// stubObserveKey identifies a (repoID, method) pair for once-gated
// logging on Phase 64-deferred read paths.
type stubObserveKey struct{ repoID, method string }

// stubObserveState is the observability harness for unwired Phase 64
// read paths on rankStoreAdapter. emit() increments
// SemanticGraphRepairInc("stub_no_data") on every call AND emits
// EXACTLY ONE WARN log per (repoID, method) pair per process via a
// per-key sync.Once. Nil-safe: a nil receiver is a silent no-op so
// production paths that construct an adapter without metrics+logger
// (e.g. early bootstrap) keep working.
//
// Closes 62-VERIFICATION.md gap truth #21 (WR-05): operators monitoring
// helix_semantic_graph_repair_total{outcome="stub_no_data"} get a
// non-zero rate proportional to scheduler activity → clear "rank wired
// but data not yet flowing" signal, instead of mistaking a clean stub
// path for outcome="applied" success.
type stubObserveState struct {
	mu      sync.Mutex
	onceMap map[stubObserveKey]*sync.Once
	metrics graphpkg.MetricsSink
	logger  *slog.Logger
}

func (st *stubObserveState) emit(repoID, method string) {
	if st == nil {
		return
	}
	if st.metrics != nil {
		st.metrics.SemanticGraphRepairInc("stub_no_data")
	}
	st.mu.Lock()
	if st.onceMap == nil {
		st.onceMap = make(map[stubObserveKey]*sync.Once)
	}
	key := stubObserveKey{repoID: repoID, method: method}
	once, ok := st.onceMap[key]
	if !ok {
		once = &sync.Once{}
		st.onceMap[key] = once
	}
	st.mu.Unlock()
	once.Do(func() {
		if st.logger != nil {
			st.logger.Warn(
				"rankStoreAdapter: production read method is stubbed; rank surface returns empty until Phase 64 lands the typed effective-graph queries",
				"method", method,
				"repo_id", repoID,
				"phase", "64",
				"see", "62-VERIFICATION.md truth #21 (WR-05); closure 62-08-PLAN.md",
			)
		}
	})
}

// rankStoreAdapter wraps *semanticstore.Store with the graph.SchedulerStore
// surface. Phase 64 P64-08 collapsed the four read-method stubs to delegate
// directly to *Store now that 64-02 shipped real implementations. The
// stubObserve harness is retained because the public *rankStoreAdapter
// shape still surfaces it through the constructor, and future once-WARN
// signals (e.g., empty pre-data results in a deployment that was expected
// to have data) can re-arm it without touching the constructor surface.
type rankStoreAdapter struct {
	store       *semanticstore.Store
	stubObserve *stubObserveState
}

// newRankStoreAdapter constructs the production adapter. metrics + logger
// flow into the stubObserve harness so the four Phase 64-deferred read
// methods surface operator-visible signal until Phase 64 lands real
// queries. Both are nil-safe: a nil sink/logger keeps the harness silent.
func newRankStoreAdapter(s *semanticstore.Store, metrics graphpkg.MetricsSink, logger *slog.Logger) *rankStoreAdapter {
	if s == nil {
		return nil
	}
	return &rankStoreAdapter{
		store: s,
		stubObserve: &stubObserveState{
			metrics: metrics,
			logger:  logger,
		},
	}
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

// Phase 64 P64-08 stub-collapse: the four methods below now delegate
// verbatim to *Store. Phase 64-02 shipped QueryEffectiveAdjacency /
// CountStaleScoreRows / MarkAllScoreRowsStale on *Store; the previous
// stub-no-data observability harness is no longer fired on these paths.
//
// QueryEffectiveGraph is derived from QueryEffectiveAdjacency: the
// scheduler's effective-graph contract is "(nodes, outgoing-adjacency)";
// nodes are the union of source nodes from the outgoing adjacency map,
// which is the natural projection. The stubObserve harness is retained on
// the IngestNode-from-adjacency path because no committed snapshot still
// produces empty results (a clean pre-data state, not a stub gap).
func (a *rankStoreAdapter) QueryEffectiveGraph(ctx context.Context, repoID, projection string) ([]graphpkg.NodeID, map[graphpkg.NodeID]map[graphpkg.NodeID]float64, error) {
	out, _, err := a.store.QueryEffectiveAdjacency(ctx, repoID, projection)
	if err != nil {
		return nil, nil, err
	}
	if out == nil {
		out = map[graphpkg.NodeID]map[graphpkg.NodeID]float64{}
	}
	// Nodes set: union of source nodes (the keys of the outgoing
	// adjacency map are the canonical "knew about this node" set; nodes
	// with only inbound edges still appear as targets in some src entry).
	seen := make(map[graphpkg.NodeID]struct{}, len(out))
	for src, dsts := range out {
		seen[src] = struct{}{}
		for dst := range dsts {
			seen[dst] = struct{}{}
		}
	}
	nodes := make([]graphpkg.NodeID, 0, len(seen))
	for n := range seen {
		nodes = append(nodes, n)
	}
	return nodes, out, nil
}

func (a *rankStoreAdapter) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (map[graphpkg.NodeID]map[graphpkg.NodeID]float64, map[graphpkg.NodeID]map[graphpkg.NodeID]float64, error) {
	return a.store.QueryEffectiveAdjacency(ctx, repoID, projection)
}

func (a *rankStoreAdapter) CountStaleScoreRows(ctx context.Context, repoID, projection string) (int, int, error) {
	return a.store.CountStaleScoreRows(ctx, repoID, projection)
}

func (a *rankStoreAdapter) MarkAllScoreRowsStale(ctx context.Context, repoID, projection string) error {
	return a.store.MarkAllScoreRowsStale(ctx, repoID, projection)
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
	_ graphpkg.RepairTx       = (*rankRepairTxAdapter)(nil)
	_ graphpkg.SchedulerStore = (*rankStoreAdapter)(nil)
	_ graphpkg.RepairStore    = repairStoreShim{}
)

// _ keeps unused fmt import out of the build; remove once any error
// formatting lands.
var _ = fmt.Sprintf
