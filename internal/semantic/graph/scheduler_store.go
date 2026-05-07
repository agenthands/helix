// Phase 62 P03: SchedulerStore narrow seam.
//
// SchedulerStore is the read+write surface the rank scheduler and the
// full-recompute path call into. Production = a thin adapter over
// *store.Store wired in internal/daemon/live_wiring.go; unit tests inject
// recording fakes (scheduler_test.go, full_recompute_test.go).
//
// The interface is intentionally narrow: every method has a direct
// counterpart in *store.Store + *store.OverlayTx. No business logic lives
// here — that belongs in scheduler.go / full_recompute.go.

package graph

import "context"

// SchedulerStore is the per-package narrow seam. Production binds to a
// thin adapter wrapping *store.Store; tests inject recording fakes.
type SchedulerStore interface {
	// LockWorkspace acquires the per-workspace overlay mutex (T4 invariant:
	// share with current_epoch + ApplyRepair). The release function MUST be
	// invoked exactly once after the corresponding tx terminates.
	LockWorkspace(repoID string) (release func())

	// BeginRepairTx opens a per-workspace overlay tx the scheduler can use
	// for score-row writes (UpsertGraphScores / DeleteScoresForProjection)
	// and edge-merge writes (UpsertEdgesWithMerge). Caller MUST hold the
	// workspace lock before invoking — production wiring enforces this via
	// the LockWorkspace contract above.
	BeginRepairTx(ctx context.Context, repoID string) (RepairTx, error)

	// CurrentGraphVersion reads the current graph_version for repoID.
	CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error)

	// QueryEffectiveGraph returns the (nodes, outgoing-adjacency) view of
	// the snapshot ⊕ overlay − tombstones graph for (repo, projection).
	// Caller may iterate the adjacency directly because PageRank is sort-
	// before-iterate (D-03) at the engine layer.
	QueryEffectiveGraph(ctx context.Context, repoID, projection string) ([]NodeID, map[NodeID]map[NodeID]float64, error)

	// QueryEffectiveAdjacency returns both directions of adjacency for the
	// 1-hop frontier algorithm (D-09). The caller passes both maps to
	// ComputeFrontier verbatim.
	QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (out, in map[NodeID]map[NodeID]float64, err error)

	// CountStaleScoreRows returns (stale, total) for (repo, projection).
	// The scheduler uses the ratio to drive its full-recompute decision
	// (D-08 FullRecomputeThreshold).
	//
	// LOCK CONTRACT (CR-01 closure, 62-07): Implementations
	// MUST NOT acquire the workspace lock returned by LockWorkspace.
	// RankScheduler invokes this method AFTER tx.Commit() AND AFTER
	// releasing the workspace lock; any implementation that re-acquires
	// LockWorkspace would self-deadlock against the caller's prior
	// release. Open a fresh read tx without the workspace lock, or use
	// an unlocked counter view.
	CountStaleScoreRows(ctx context.Context, repoID, projection string) (stale, total int, err error)

	// MarkAllScoreRowsStale flips every score row for (repo, projection)
	// to status='stale'. Called on frontier-overflow (D-09) so the next
	// reader sees the correct ScoreStatusStale.
	MarkAllScoreRowsStale(ctx context.Context, repoID, projection string) error
}
