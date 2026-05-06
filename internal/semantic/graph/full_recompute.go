// Phase 62 P03: full-recompute path with preemption → approximate marker.
//
// RunFullRecompute reads the full effective graph for one (repo, projection)
// pair, runs PageRank end-to-end, deletes every prior score row for the
// projection, and writes the fresh generation under the graph_version that
// was current when the recompute started. If a competing ApplyRepair
// advances graph_version mid-run, the rows are stamped status=approximate
// (D-10 hard invariant) and the function returns preempted=true so the
// scheduler can immediately start a fresh incremental repair.

package graph

import (
	"context"
	"fmt"
	"time"

	gengraph "github.com/agenthands/helix/internal/graph"
)

// FullRecomputeOptions carries the PageRank tunables for one full
// recompute. The scheduler pulls these from the SchedulerConfig (which in
// turn flows from the koanf config layer), so this struct exists primarily
// to keep RunFullRecompute callable in unit tests without a full
// SchedulerConfig instantiation.
type FullRecomputeOptions struct {
	Damping float64
	Epsilon float64
	MaxIter int
}

// RunFullRecompute is the D-10 path. Returns (preempted, err).
//
//   - Reads startGV via store.CurrentGraphVersion before opening the tx
//     so the preemption check at end-of-run has a clean baseline.
//   - Loads the full (nodes, edges) effective view for (repo, projection)
//     and runs internal/graph.PageRank against it.
//   - Opens a RepairTx, deletes every prior score row for the projection,
//     and writes the fresh generation. If endGV > startGV at write time
//     the rows are marked approximate, preempted=true, and the scheduler
//     restarts incremental repair on return.
//   - Always commits or rolls back the tx exactly once.
//
// The function does NOT bump graph_version itself — D-06 reserves that
// site for ApplyRepair. A full recompute writes new rows under the
// existing version (or the caller-observed updated version if mid-run
// preempted).
func RunFullRecompute(
	ctx context.Context,
	repoID, projection string,
	store SchedulerStore,
	opts FullRecomputeOptions,
) (preempted bool, err error) {

	if store == nil {
		return false, fmt.Errorf("RunFullRecompute: nil store")
	}
	if repoID == "" {
		return false, fmt.Errorf("RunFullRecompute: empty repoID")
	}
	if projection == "" {
		return false, fmt.Errorf("RunFullRecompute: empty projection")
	}

	startGV, err := store.CurrentGraphVersion(ctx, repoID)
	if err != nil {
		return false, fmt.Errorf("RunFullRecompute(%q): read currentGV: %w", repoID, err)
	}

	nodes, edges, err := store.QueryEffectiveGraph(ctx, repoID, projection)
	if err != nil {
		return false, fmt.Errorf("RunFullRecompute(%q): query graph: %w", repoID, err)
	}

	// PageRank is pure and stdlib-only (D-01 invariant); no goroutines, no
	// timer interaction, no I/O.
	scores := gengraph.PageRank(nodes, edges, gengraph.Options{
		Damping: opts.Damping,
		Epsilon: opts.Epsilon,
		MaxIter: opts.MaxIter,
	})

	release := store.LockWorkspace(repoID)
	defer release()

	tx, err := store.BeginRepairTx(ctx, repoID)
	if err != nil {
		return false, fmt.Errorf("RunFullRecompute(%q): begin tx: %w", repoID, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := tx.DeleteScoresForProjection(ctx, projection); err != nil {
		return false, fmt.Errorf("RunFullRecompute(%q): delete prior rows: %w", repoID, err)
	}

	// Build the fresh row set in deterministic node order.
	rows := make([]ScoreRow, 0, len(nodes))
	for _, n := range sortedNodeIDsSlice(nodes) {
		rows = append(rows, ScoreRow{
			NodeID:       n,
			Score:        scores[n],
			GraphVersion: startGV,
			Status:       string(ScoreStatusExact),
		})
	}

	// Preemption check: if a competing ApplyRepair advanced graph_version
	// while we were running PageRank, mark every row approximate (D-10).
	endGV, err := store.CurrentGraphVersion(ctx, repoID)
	if err != nil {
		return false, fmt.Errorf("RunFullRecompute(%q): re-read currentGV: %w", repoID, err)
	}
	if endGV > startGV {
		preempted = true
		for i := range rows {
			rows[i].Status = string(ScoreStatusApproximate)
			rows[i].GraphVersion = endGV
		}
	}

	if err := tx.UpsertGraphScores(ctx, projection, rows); err != nil {
		return preempted, fmt.Errorf("RunFullRecompute(%q): write scores: %w", repoID, err)
	}

	// Re-check after the write — the test fakes advance endGV inside the
	// UpsertGraphScores hook to model "preemption observed mid-write". A
	// production tx holds the per-workspace mutex so this branch is
	// effectively dead in real workloads, but the second check keeps the
	// unit-test contract honest without coupling tests to wall time.
	endGV2, err := store.CurrentGraphVersion(ctx, repoID)
	if err != nil {
		return preempted, fmt.Errorf("RunFullRecompute(%q): post-write currentGV: %w", repoID, err)
	}
	if endGV2 > startGV && !preempted {
		preempted = true
		// Delete the previously-written exact rows under startGV before
		// writing the approximate generation under endGV2 — readers must
		// see exactly one row per (repo, projection, node), and that row
		// MUST carry the approximate marker (D-10 atomic invariant).
		if err := tx.DeleteScoresForProjection(ctx, projection); err != nil {
			return preempted, fmt.Errorf("RunFullRecompute(%q): delete exact rows on preempt: %w", repoID, err)
		}
		approxRows := make([]ScoreRow, len(rows))
		for i, r := range rows {
			r.Status = string(ScoreStatusApproximate)
			r.GraphVersion = endGV2
			approxRows[i] = r
		}
		// Re-write with approximate marker.
		if err := tx.UpsertGraphScores(ctx, projection, approxRows); err != nil {
			return preempted, fmt.Errorf("RunFullRecompute(%q): rewrite approximate: %w", repoID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return preempted, fmt.Errorf("RunFullRecompute(%q): commit: %w", repoID, err)
	}
	committed = true

	_ = time.Now() // hold the time import for future metric integration
	return preempted, nil
}

// sortedNodeIDsSlice returns a sorted copy of `in` so iteration order is
// deterministic at the score-write site (D-03 sort-before-iterate).
func sortedNodeIDsSlice(in []NodeID) []NodeID {
	out := append([]NodeID(nil), in...)
	// Insertion-order copies preserve any caller-supplied ordering, but we
	// must NOT trust it — sort defensively. The map-driven sortedNodeIDs
	// helper is the right fit when we have a map; here the source is a
	// slice already, so we reuse the standard library sort in place via
	// the helper composition below.
	tmp := map[NodeID]struct{}{}
	for _, n := range in {
		tmp[n] = struct{}{}
	}
	sorted := sortedNodeIDs(tmp)
	if len(sorted) == len(out) {
		return sorted
	}
	return sorted // dedup-safe path
}
