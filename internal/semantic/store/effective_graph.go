// Phase 64 P64-02: effective-graph queries on *Store.
//
// These five methods implement the foundation surface that Phase 63 deferred
// to Phase 64. Consumers:
//
//   - rankStoreAdapter (internal/daemon/rank_wiring.go) — delegates the
//     SchedulerStore interface methods QueryEffectiveAdjacency,
//     CountStaleScoreRows, MarkAllScoreRowsStale to the production *Store.
//   - tools_status.go (P64-04) — calls LatestCommittedSnapshot.
//   - retrieval/recovery.go (P64-07) — calls IterateCommittedSymbols.
//
// SCHEMA REALITY (vs PLAN's pseudocode):
//
//   - semantic_edges: keyed by (snapshot_id, edge_id). Columns:
//     src_node_id, dst_node_id, edge_kind, weight. NO repo_id; restrict by
//     joining on the latest committed snapshot for repoID.
//   - semantic_live_overlay_edges: has repo_id, src_node_id, dst_node_id,
//     edge_kind, weight, status ('live' | 'deleted'). NO `tombstone`
//     boolean — the closed-enum status='live' is the non-tombstoned state.
//   - semantic_graph_scores: keyed by (repo_id, graph_version, node_id,
//     score_name). The interface's `projection` parameter maps to
//     score_name for scores and to edge_kind for edges.
//   - semantic_symbols: has snapshot_id, symbol_id, name, file_id,
//     start_line. NO `docstring` or `path` columns — path lives in
//     semantic_files (joined via snapshot_id+file_id); docstring is not
//     materialized in Schema 5 yet (P64-07 corpus will read it from
//     source files at index time). SymbolRow.Docstring is therefore
//     emitted as the empty string here and supplemented by the bleve
//     corpus on the consumer side.
//
// LOCK CONTRACT (scheduler_store.go:48-56): all five methods read with no
// workspace lock. CountStaleScoreRows / QueryEffectiveAdjacency MUST NOT
// re-acquire LockWorkspace — RankScheduler invokes them after releasing
// the lock; re-acquiring would self-deadlock. Methods open read queries on
// s.db directly, never on a per-tx Tx.

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/agenthands/helix/internal/semantic/graph"
)

// errNotImplemented is the sentinel returned by the stubbed implementations
// during the Task 1 RED gate (PLAN 64-02). Task 2 GREEN replaces every
// stub body with the production query and removes this sentinel.
var errNotImplemented = errors.New("effective_graph: not implemented (Task 1 RED stub)")

// SymbolRow is the iteration payload for IterateCommittedSymbols.
//
// Field set is the minimum required by Phase 64 P64-07 retrieval/corpus.go
// MapSymbolToDoc — the bleve corpus consumer maps these fields onto its
// SymbolDoc indexed-fields shape (see 64-PATTERNS.md row
// "internal/semantic/retrieval/corpus.go").
//
// Docstring is currently always empty string (Schema 5 has no docstring
// column on semantic_symbols; the corpus loader will read it from the
// file's source bytes at index time using LineStart as anchor). The field
// is kept on SymbolRow so the seam stays stable when a future migration
// materializes docstrings.
type SymbolRow struct {
	SymbolID  string
	Name      string
	Path      string
	Docstring string
	FileID    string
	LineStart int
}

// QueryEffectiveAdjacency returns (out, in) adjacency for the (repo,
// projection) effective graph: snapshot edges ⊕ live overlay edges, minus
// any overlay row tombstoned via status='deleted'.
//
// Reads under no workspace lock (scheduler_store.go:48-56). Consumes the
// Phase 60 D-04 CAS contract via the underlying DuckDB MVCC isolation.
func (s *Store) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
) {
	return nil, nil, errNotImplemented
}

// CountStaleScoreRows returns (stale, total) score-row counts for (repo,
// projection). Drives the RankScheduler's full-recompute decision (D-08
// FullRecomputeThreshold).
//
// LOCK-FREE per scheduler_store.go:48-56.
func (s *Store) CountStaleScoreRows(ctx context.Context, repoID, projection string) (
	stale, total int, err error,
) {
	return 0, 0, errNotImplemented
}

// MarkAllScoreRowsStale flips every score row for (repo, projection) to
// status='stale'. Idempotent. Called on frontier overflow (D-09) so the
// next reader observes ScoreStatusStale.
func (s *Store) MarkAllScoreRowsStale(ctx context.Context, repoID, projection string) error {
	return errNotImplemented
}

// LatestCommittedSnapshot returns the highest committed snapshot_id for
// repoID. Returns (0, nil) when no committed snapshot exists.
func (s *Store) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	return 0, errNotImplemented
}

// IterateCommittedSymbols walks every semantic_symbols row at snapshotID
// in stable symbol_id ASC order, invoking fn(row). If fn returns false,
// iteration aborts cleanly without error. Honors ctx cancellation via the
// underlying QueryContext.
//
// Consumed by P64-07's bleve recovery probe to rebuild the FTS segment
// from a committed snapshot deterministically across daemon restarts.
func (s *Store) IterateCommittedSymbols(ctx context.Context, snapshotID uint64, fn func(SymbolRow) bool) error {
	return errNotImplemented
}

// Compile-time: keep sql import used even before GREEN lands the queries.
var _ = sql.ErrNoRows

// Phase 64 RED-stub guard: fmt.Errorf is used for parameterized error wraps
// in the GREEN implementation (Task 2). Reference here keeps the import
// alive so `goimports` / `go vet` do not strip it during the two-step TDD
// gate's intermediate state.
var _ = fmt.Errorf
