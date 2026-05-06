// Phase 62 P03: per-workspace RankScheduler.
//
// One goroutine per workspace, errgroup-owned. Subscribes to graph_version
// advances via the typed Notify(GraphVersionAdvance) channel from
// ApplyRepair (Engine.SetNotifyChannel wires the producer side; the
// scheduler's Notify forwards into its in-channel non-blockingly).
//
// Lifecycle mirrors internal/semantic/live/coalescer/coalescer.go:
// closed-channel select on ctx.Done()/in, debounce + long-idle timers,
// no per-event goroutine spawn. Drops on full input channel and emits
// SemanticGraphRepairInc("error") so operators can alert on saturation
// (D-08 best-effort contract).

package graph

import (
	"context"
	"log/slog"
	"sync"
	"time"

	gengraph "github.com/agenthands/helix/internal/graph"
)

// SchedulerConfig wires the koanf semantic_index.pagerank.* keys into the
// scheduler. Zero values get overlay defaults from the production wiring
// path; tests pass tight values for fast assertions.
type SchedulerConfig struct {
	// Debounce is the quiet-period before an incremental repair fires.
	// Each Notify resets this timer.
	Debounce time.Duration
	// FullRecomputeIdle is the longer idle window that — when stale rows
	// exceed FullRecomputeThreshold — triggers a full recompute. Reset by
	// successful incremental + full recomputes ONLY (Pitfall 4).
	FullRecomputeIdle time.Duration
	// FullRecomputeThreshold is the stale/total fraction above which the
	// long-idle timer's fire schedules a full recompute.
	FullRecomputeThreshold float64
	// MaxLocalNodes caps the 1-hop frontier (D-09). Above this, the
	// scheduler marks every score row stale + schedules full recompute.
	MaxLocalNodes int
	// PageRank tunables flowed from semantic_index.pagerank.*.
	Damping float64
	Epsilon float64
	MaxIter int
	// Projection is the score-row projection key (v1 ships only
	// "call_graph"; multi-projection support is deferred per CONTEXT.md).
	Projection string
	// QueueSize controls the input channel buffer. Defaults to 8 when
	// zero. Drops on full + emits SemanticGraphRepairInc("error").
	QueueSize int
}

// RankScheduler is one goroutine per workspace.
type RankScheduler struct {
	repoID  string
	cfg     SchedulerConfig
	in      chan GraphVersionAdvance
	store   SchedulerStore
	logger  *slog.Logger
	metrics MetricsSink

	// pending holds the union of ChangedNodes across all advances seen
	// since the last incremental repair. Reset under timerMu when the
	// debounce fires and the incremental repair starts.
	timerMu       sync.Mutex
	pendingChanged []NodeID
	debounceTimer *time.Timer
	longIdleTimer *time.Timer
	lastSeenGV    uint64
}

// NewRankScheduler constructs a scheduler bound to the given store. Run
// must be invoked in a goroutine for the scheduler to make progress.
func NewRankScheduler(
	repoID string,
	cfg SchedulerConfig,
	store SchedulerStore,
	logger *slog.Logger,
	metrics MetricsSink,
) *RankScheduler {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 8
	}
	if cfg.Projection == "" {
		cfg.Projection = "call_graph"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &RankScheduler{
		repoID:  repoID,
		cfg:     cfg,
		in:      make(chan GraphVersionAdvance, cfg.QueueSize),
		store:   store,
		logger:  logger,
		metrics: metrics,
	}
}

// Notify enqueues a graph_version advance. Non-blocking: drops on full
// buffer and emits SemanticGraphRepairInc("error") so operators can alert
// on saturation. The advance carries a pre-sorted ChangedNodes slice so
// the scheduler does not have to re-sort.
//
// W3 LOCKED: payload is graph.GraphVersionAdvance, NOT a bare uint64 —
// this lets the scheduler read adv.ChangedNodes directly without a
// stale-row fallback (62-03 P03 Acceptance: TestRankScheduler_DebounceCoalesces
// implicitly proves the no-fallback contract).
func (s *RankScheduler) Notify(adv GraphVersionAdvance) {
	if s == nil {
		return
	}
	select {
	case s.in <- adv:
	default:
		if s.metrics != nil {
			s.metrics.SemanticGraphRepairInc("error")
		}
		s.logger.Warn("rank scheduler: notify channel full; dropping advance",
			"repo_id", s.repoID, "version", adv.Version)
	}
}

// Run drives the scheduler until ctx is cancelled. Closed-channel select
// over ctx.Done() and the input channel; AfterFunc-driven timers ensure
// the goroutine itself stays free of timing logic.
func (s *RankScheduler) Run(ctx context.Context) error {
	defer s.stopTimers()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case adv := <-s.in:
			s.handleAdvance(ctx, adv)
		}
	}
}

func (s *RankScheduler) stopTimers() {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if s.debounceTimer != nil {
		s.debounceTimer.Stop()
	}
	if s.longIdleTimer != nil {
		s.longIdleTimer.Stop()
	}
}

// handleAdvance accumulates changed-node payload + resets the debounce
// timer. The long-idle timer is started lazily on the first advance and
// is NOT reset here (Pitfall 4 invariant).
func (s *RankScheduler) handleAdvance(ctx context.Context, adv GraphVersionAdvance) {
	s.timerMu.Lock()
	s.lastSeenGV = adv.Version
	s.pendingChanged = mergeSorted(s.pendingChanged, adv.ChangedNodes)
	if s.debounceTimer != nil {
		s.debounceTimer.Stop()
	}
	s.debounceTimer = time.AfterFunc(s.cfg.Debounce, func() {
		s.runIncrementalRepair(ctx)
	})
	if s.longIdleTimer == nil {
		s.longIdleTimer = time.AfterFunc(s.cfg.FullRecomputeIdle, func() {
			s.maybeFullRecompute(ctx)
		})
	}
	s.timerMu.Unlock()
}

// runIncrementalRepair fires after debounce. Computes the 1-hop frontier
// from the accumulated changed-node set; on overflow marks all stale +
// schedules full recompute on the existing long-idle timer; otherwise
// runs PageRank over the frontier subgraph and writes only those rows.
func (s *RankScheduler) runIncrementalRepair(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	s.timerMu.Lock()
	changed := s.pendingChanged
	s.pendingChanged = nil
	s.timerMu.Unlock()
	if len(changed) == 0 {
		return
	}

	out, in, err := s.store.QueryEffectiveAdjacency(ctx, s.repoID, s.cfg.Projection)
	if err != nil {
		s.metricInc("error")
		s.logger.Warn("rank scheduler: adjacency query failed",
			"repo_id", s.repoID, "err", err)
		return
	}

	frontier, overflow := ComputeFrontier(changed, out, in, s.cfg.MaxLocalNodes)
	if overflow {
		// D-09 hard invariant: above threshold → mark all stale + let the
		// long-idle timer schedule the full recompute. Do NOT reset the
		// long-idle timer here (Pitfall 4).
		if err := s.store.MarkAllScoreRowsStale(ctx, s.repoID, s.cfg.Projection); err != nil {
			s.metricInc("error")
			s.logger.Warn("rank scheduler: mark-all-stale failed",
				"repo_id", s.repoID, "err", err)
			return
		}
		s.metricInc("frontier_overflow")
		return
	}

	// Build the frontier subgraph: keep only edges where both endpoints
	// are in the frontier. Frontier nodes outside the subgraph still get
	// a row written so readers see "exact" or "approximate" status.
	frontierSet := map[NodeID]struct{}{}
	for _, n := range frontier {
		frontierSet[n] = struct{}{}
	}
	subEdges := map[NodeID]map[NodeID]float64{}
	for _, src := range frontier {
		for _, dst := range sortedNodeIDs(out[src]) {
			if _, ok := frontierSet[dst]; !ok {
				continue
			}
			if subEdges[src] == nil {
				subEdges[src] = map[NodeID]float64{}
			}
			subEdges[src][dst] = out[src][dst]
		}
	}

	scores := gengraph.PageRank(frontier, subEdges, gengraph.Options{
		Damping: s.cfg.Damping,
		Epsilon: s.cfg.Epsilon,
		MaxIter: s.cfg.MaxIter,
	})

	release := s.store.LockWorkspace(s.repoID)
	defer release()

	tx, err := s.store.BeginRepairTx(ctx, s.repoID)
	if err != nil {
		s.metricInc("error")
		s.logger.Warn("rank scheduler: begin tx failed",
			"repo_id", s.repoID, "err", err)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	rows := make([]ScoreRow, 0, len(frontier))
	for _, n := range frontier {
		rows = append(rows, ScoreRow{
			NodeID:       n,
			Score:        scores[n],
			GraphVersion: s.lastSeenGV,
			Status:       string(ScoreStatusExact),
		})
	}

	if err := tx.UpsertGraphScores(ctx, s.cfg.Projection, rows); err != nil {
		s.metricInc("error")
		s.logger.Warn("rank scheduler: upsert scores failed",
			"repo_id", s.repoID, "err", err)
		return
	}

	if err := tx.Commit(); err != nil {
		s.metricInc("error")
		s.logger.Warn("rank scheduler: commit failed",
			"repo_id", s.repoID, "err", err)
		return
	}
	committed = true
	s.metricInc("applied")

	// Reset the long-idle timer when an incremental repair lands AND the
	// post-write stale-fraction has cleared the threshold (Pitfall 4).
	stale, total, err := s.store.CountStaleScoreRows(ctx, s.repoID, s.cfg.Projection)
	if err == nil && total > 0 {
		ratio := float64(stale) / float64(total)
		if ratio < s.cfg.FullRecomputeThreshold {
			s.timerMu.Lock()
			if s.longIdleTimer != nil {
				s.longIdleTimer.Stop()
				s.longIdleTimer = time.AfterFunc(s.cfg.FullRecomputeIdle, func() {
					s.maybeFullRecompute(ctx)
				})
			}
			s.timerMu.Unlock()
		}
	}
}

// maybeFullRecompute fires when the long-idle timer expires. Checks the
// stale fraction and, if it crosses the threshold, runs the full
// recompute path. Always re-arms the long-idle timer afterwards.
func (s *RankScheduler) maybeFullRecompute(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	stale, total, err := s.store.CountStaleScoreRows(ctx, s.repoID, s.cfg.Projection)
	if err != nil {
		s.metricInc("error")
		s.logger.Warn("rank scheduler: count-stale failed",
			"repo_id", s.repoID, "err", err)
		return
	}
	shouldRun := total > 0 && float64(stale)/float64(total) >= s.cfg.FullRecomputeThreshold
	// Even when shouldRun is false, re-arm the long-idle timer so the
	// scheduler keeps polling at the same cadence.
	defer func() {
		s.timerMu.Lock()
		s.longIdleTimer = time.AfterFunc(s.cfg.FullRecomputeIdle, func() {
			s.maybeFullRecompute(ctx)
		})
		s.timerMu.Unlock()
	}()
	if !shouldRun {
		return
	}
	preempted, err := RunFullRecompute(ctx, s.repoID, s.cfg.Projection, s.store, FullRecomputeOptions{
		Damping: s.cfg.Damping,
		Epsilon: s.cfg.Epsilon,
		MaxIter: s.cfg.MaxIter,
	})
	if err != nil {
		s.metricInc("error")
		s.logger.Warn("rank scheduler: full recompute failed",
			"repo_id", s.repoID, "err", err)
		return
	}
	if preempted {
		s.metricInc("preempted")
		// Pre-empted full recompute: scheduler immediately starts a fresh
		// incremental repair to land the rows the new ApplyRepair carried.
		s.runIncrementalRepair(ctx)
		return
	}
	s.metricInc("applied")
}

func (s *RankScheduler) metricInc(outcome string) {
	if s == nil || s.metrics == nil {
		return
	}
	s.metrics.SemanticGraphRepairInc(outcome)
}

// mergeSorted returns a sorted-ascending dedup of `existing` ∪ `incoming`.
// Both slices must already be sorted ascending — incoming is from
// GraphVersionAdvance.ChangedNodes (engine.unionAndSort produces sorted
// output); existing accumulates between debounce fires and is kept sorted
// here.
func mergeSorted(existing, incoming []NodeID) []NodeID {
	if len(existing) == 0 {
		return append([]NodeID(nil), incoming...)
	}
	if len(incoming) == 0 {
		return existing
	}
	seen := make(map[NodeID]struct{}, len(existing)+len(incoming))
	for _, n := range existing {
		seen[n] = struct{}{}
	}
	for _, n := range incoming {
		seen[n] = struct{}{}
	}
	return sortedNodeIDs(seen)
}
