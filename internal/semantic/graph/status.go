package graph

// ScoreStatus is the D-07 closed enum returned by computeScoreStatus at
// read time. Phase 62 P02 ships exactly four values; downstream readers
// MUST treat any other string as a contract violation.
type ScoreStatus string

const (
	// ScoreStatusExact: a row exists for the (repo, projection, node) and
	// its graph_version equals the workspace's current graph_version.
	ScoreStatusExact ScoreStatus = "exact"

	// ScoreStatusApproximate: a row exists at the current graph_version
	// but carries the approximate flag — set by the full-recompute
	// scheduler when a mid-run preempt forces it (P03 D-10).
	ScoreStatusApproximate ScoreStatus = "approximate"

	// ScoreStatusStale: a row exists but its graph_version is older than
	// the workspace's current graph_version.
	ScoreStatusStale ScoreStatus = "stale"

	// ScoreStatusMissing: no row exists for the (repo, projection, node).
	ScoreStatusMissing ScoreStatus = "missing"
)

// computeScoreStatus is the D-07 read-time decision:
//
//   - !hasRow → missing (regardless of other inputs)
//   - approximate → approximate (overrides version equality)
//   - rowGV < currentGV → stale
//   - rowGV == currentGV → exact
//   - rowGV > currentGV is impossible by D-06 monotonicity; the function
//     returns exact in that pathological case to avoid a panic.
func computeScoreStatus(currentGV, rowGV uint64, hasRow, approximate bool) ScoreStatus {
	if !hasRow {
		return ScoreStatusMissing
	}
	if approximate {
		return ScoreStatusApproximate
	}
	if rowGV < currentGV {
		return ScoreStatusStale
	}
	return ScoreStatusExact
}
