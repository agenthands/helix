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
//     joining on the latest committed snapshot for repoID via
//     semantic_snapshots.
//   - semantic_live_overlay_edges: has repo_id, src_node_id, dst_node_id,
//     edge_kind, weight, status ('live' | 'deleted'). NO `tombstone`
//     boolean — the closed-enum status='live' is the non-tombstoned state
//     (overlay's MarkEdgesDeleted flips 'live' → 'deleted').
//   - semantic_graph_scores: keyed by (repo_id, graph_version, node_id,
//     score_name). The interface's `projection` parameter maps to
//     score_name for scores and to edge_kind for edges.
//   - semantic_symbols: has snapshot_id, symbol_id, name, file_id,
//     start_line. NO `docstring` or `path` columns — path lives in
//     semantic_files (joined via snapshot_id+file_id); docstring is not
//     materialized in Schema 5 yet (P64-07 corpus will read it from
//     source files at index time using LineStart as anchor).
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
	"strconv"

	"github.com/agenthands/helix/internal/semantic/graph"
)

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
// projection) effective graph: snapshot edges (from the latest committed
// snapshot) ⊕ live overlay edges, minus any overlay row tombstoned via
// status='deleted'.
//
// The `projection` parameter is the score-row projection key (e.g.,
// "call_graph") and maps onto the `edge_kind` column for both
// semantic_edges and semantic_live_overlay_edges.
//
// Reads under no workspace lock (scheduler_store.go:48-56). Consumes the
// Phase 60 D-04 CAS contract via the underlying DuckDB MVCC isolation.
//
// Returns empty (non-nil) maps when no committed snapshot exists for
// repoID — the scheduler treats that as a clean empty graph and short-
// circuits its incremental path.
func (s *Store) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
) {
	if s == nil || s.db == nil {
		return nil, nil, errors.New("QueryEffectiveAdjacency: nil store")
	}
	out = make(map[graph.NodeID]map[graph.NodeID]float64)
	in = make(map[graph.NodeID]map[graph.NodeID]float64)

	// Snapshot edges live in semantic_edges keyed by (snapshot_id, edge_id);
	// repoID lookup goes through semantic_snapshots. Overlay edges live in
	// semantic_live_overlay_edges keyed by repo_id directly. The UNION ALL
	// across both sources, filtered by edge_kind and the overlay's
	// status='live' tombstone discriminator, is the effective view.
	const q = `
		SELECT e.src_node_id, e.dst_node_id, e.weight
		  FROM semantic_edges AS e
		  JOIN semantic_snapshots AS s ON s.snapshot_id = e.snapshot_id
		 WHERE s.repo_id   = ?
		   AND s.status    = 'committed'
		   AND e.edge_kind = ?
		   AND s.snapshot_id = (
		     SELECT MAX(snapshot_id) FROM semantic_snapshots
		      WHERE repo_id = ? AND status = 'committed'
		   )
		UNION ALL
		SELECT o.src_node_id, o.dst_node_id, o.weight
		  FROM semantic_live_overlay_edges AS o
		 WHERE o.repo_id   = ?
		   AND o.edge_kind = ?
		   AND o.status    = 'live'
	`
	rows, err := s.db.QueryContext(ctx, q, repoID, projection, repoID, repoID, projection)
	if err != nil {
		return nil, nil, fmt.Errorf("QueryEffectiveAdjacency(%q, %q): %w", repoID, projection, err)
	}
	defer rows.Close()

	for rows.Next() {
		var src, dst uint64
		var w float64
		if err := rows.Scan(&src, &dst, &w); err != nil {
			return nil, nil, fmt.Errorf("QueryEffectiveAdjacency scan: %w", err)
		}
		if out[graph.NodeID(src)] == nil {
			out[graph.NodeID(src)] = make(map[graph.NodeID]float64)
		}
		if in[graph.NodeID(dst)] == nil {
			in[graph.NodeID(dst)] = make(map[graph.NodeID]float64)
		}
		out[graph.NodeID(src)][graph.NodeID(dst)] = w
		in[graph.NodeID(dst)][graph.NodeID(src)] = w
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("QueryEffectiveAdjacency rows.Err: %w", err)
	}
	return out, in, nil
}

// CountStaleScoreRows returns (stale, total) score-row counts for (repo,
// projection). Drives the RankScheduler's full-recompute decision (D-08
// FullRecomputeThreshold).
//
// LOCK-FREE per scheduler_store.go:48-56. The query reads
// semantic_graph_scores via s.db (a fresh statement-scoped connection) —
// MUST NOT call LockOverlayWorkspace (would deadlock against the caller
// who already released the lock).
//
// `projection` maps to the score_name column; an empty store or one
// without rows for (repo, projection) returns (0, 0, nil).
func (s *Store) CountStaleScoreRows(ctx context.Context, repoID, projection string) (
	stale, total int, err error,
) {
	if s == nil || s.db == nil {
		return 0, 0, errors.New("CountStaleScoreRows: nil store")
	}
	const q = `
		SELECT
		  COALESCE(SUM(CASE WHEN status = 'stale' THEN 1 ELSE 0 END), 0) AS stale,
		  COUNT(*)                                                       AS total
		FROM semantic_graph_scores
		WHERE repo_id    = ?
		  AND score_name = ?
	`
	if err := s.db.QueryRowContext(ctx, q, repoID, projection).Scan(&stale, &total); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("CountStaleScoreRows(%q, %q): %w", repoID, projection, err)
	}
	return stale, total, nil
}

// MarkAllScoreRowsStale flips every score row for (repo, projection) to
// status='stale'. Idempotent (re-running on already-stale rows is a no-op
// at the DuckDB tx level — the UPDATE simply rewrites the same value).
// Called on frontier overflow (D-09) so the next reader observes
// ScoreStatusStale.
//
// `projection` maps to the score_name column. Other projections under the
// same repoID are untouched.
func (s *Store) MarkAllScoreRowsStale(ctx context.Context, repoID, projection string) error {
	if s == nil || s.db == nil {
		return errors.New("MarkAllScoreRowsStale: nil store")
	}
	const q = `
		UPDATE semantic_graph_scores
		   SET status = 'stale'
		 WHERE repo_id    = ?
		   AND score_name = ?
	`
	if _, err := s.db.ExecContext(ctx, q, repoID, projection); err != nil {
		return fmt.Errorf("MarkAllScoreRowsStale(%q, %q): %w", repoID, projection, err)
	}
	return nil
}

// LatestCommittedSnapshot returns the highest committed snapshot_id for
// repoID. Returns (0, nil) when no committed snapshot exists.
//
// Pending / aborted snapshots (status != 'committed') are excluded —
// callers (P64-04 status surface, P64-07 bleve recovery) want the
// id of a snapshot whose facts are visible to readers, not one mid-build.
func (s *Store) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("LatestCommittedSnapshot: nil store")
	}
	const q = `
		SELECT COALESCE(MAX(snapshot_id), 0)
		  FROM semantic_snapshots
		 WHERE repo_id = ?
		   AND status  = 'committed'
	`
	var id uint64
	if err := s.db.QueryRowContext(ctx, q, repoID).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("LatestCommittedSnapshot(%q): %w", repoID, err)
	}
	return id, nil
}

// RankedFileRow is one row returned by QueryRankedFiles. Score is the
// per-file aggregated PageRank score (MAX over the file's symbols);
// GraphVersion is the (repo_id, projection)-stamped graph_version under
// which the scores were computed (Phase 62 D-07 contract).
type RankedFileRow struct {
	Path         string
	Score        float64
	GraphVersion uint64
}

// QueryRankedFiles returns the per-file aggregated PageRank scores for
// (repoID, projection) at the latest committed snapshot. Aggregation:
// MAX over the file's symbols (the symbol with the highest centrality is
// the file's representative — Phase 62 D-07 score-name contract).
//
// Sort: score DESC, then path ASC (Phase 62 CR-03 stable-key tiebreak).
//
// limit <= 0 means "no limit". Empty result is (nil, nil).
//
// Reads under no workspace lock (scheduler_store.go:48-56). Returns
// (nil, nil) when no committed snapshot exists for repoID.
//
// Phase 65 65-10 Task 1.
func (s *Store) QueryRankedFiles(
	ctx context.Context, repoID, projection string, limit int,
) ([]RankedFileRow, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("QueryRankedFiles: nil store")
	}
	if projection == "" {
		return nil, errors.New("QueryRankedFiles: empty projection")
	}
	latest, err := s.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("QueryRankedFiles: %w", err)
	}
	if latest == 0 {
		return nil, nil
	}
	const baseQ = `
		SELECT f.path, MAX(g.score) AS score, MAX(g.graph_version) AS gv
		  FROM semantic_graph_scores AS g
		  JOIN semantic_symbols AS sym
		    ON sym.symbol_id   = g.node_id
		   AND sym.snapshot_id = ?
		  JOIN semantic_files AS f
		    ON f.file_id     = sym.file_id
		   AND f.snapshot_id = sym.snapshot_id
		 WHERE g.repo_id    = ?
		   AND g.score_name = ?
		   AND g.status IN ('exact', 'approximate')
		 GROUP BY f.path
		 ORDER BY score DESC, f.path ASC
	`
	q := baseQ
	args := []any{latest, repoID, projection}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("QueryRankedFiles(%q, %q): %w", repoID, projection, err)
	}
	defer rows.Close()

	var out []RankedFileRow
	for rows.Next() {
		var (
			path  string
			score float64
			gv    uint64
		)
		if err := rows.Scan(&path, &score, &gv); err != nil {
			return nil, fmt.Errorf("QueryRankedFiles scan: %w", err)
		}
		out = append(out, RankedFileRow{Path: path, Score: score, GraphVersion: gv})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("QueryRankedFiles rows.Err: %w", err)
	}
	return out, nil
}

// QuerySymbolPath resolves the file path of (snapshotID, symbolID) at the
// given committed snapshot. Returns ("", false, nil) when no symbol matches.
//
// Phase 65 65-10 Task 2: backs *integSemanticLookup.RankFromSeeds's
// resolveSymbolPath helper for the decimal-symbol_id format pinned by
// Task 0. The argument is parsed as a base-10 uint64; non-numeric strings
// resolve as a clean miss (returns "", false, nil) — they're a Task 0
// format mismatch, not a SQL error.
//
// Reads under no workspace lock (scheduler_store.go:48-56).
func (s *Store) QuerySymbolPath(ctx context.Context, snapshotID uint64, symbolID string) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, errors.New("QuerySymbolPath: nil store")
	}
	symID, err := strconv.ParseUint(symbolID, 10, 64)
	if err != nil {
		// Format mismatch with Task 0's TEXTRANK_SYMBOLID_FORMAT contract;
		// treated as a clean miss, not a SQL error.
		return "", false, nil
	}
	const q = `
		SELECT f.path FROM semantic_symbols AS sym
		  JOIN semantic_files AS f
		    ON f.snapshot_id = sym.snapshot_id AND f.file_id = sym.file_id
		 WHERE sym.snapshot_id = ? AND sym.symbol_id = ?
		 LIMIT 1
	`
	var path string
	if err := s.db.QueryRowContext(ctx, q, snapshotID, symID).Scan(&path); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("QuerySymbolPath(snap=%d, sym=%q): %w", snapshotID, symbolID, err)
	}
	return path, true, nil
}

// QuerySymbolByLocation returns the stable_key of the symbol whose source
// range contains (line, col) in the file at path, at the latest committed
// snapshot for repoID. Returns ("", false, nil) on miss (or when no
// committed snapshot exists). Inner-scope preference: when multiple
// symbols overlap (line, col), the smallest span (end_byte - start_byte)
// wins.
//
// line and col are 1-based to match LSP's external surface (mirrors
// SemanticLookup.SymbolID's contract).
//
// Phase 65 65-11 Task 1.
func (s *Store) QuerySymbolByLocation(
	ctx context.Context, repoID, path string, line, col uint32,
) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, errors.New("QuerySymbolByLocation: nil store")
	}
	latest, err := s.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		return "", false, fmt.Errorf("QuerySymbolByLocation: %w", err)
	}
	if latest == 0 {
		return "", false, nil
	}
	const q = `
		SELECT sym.stable_key
		  FROM semantic_symbols AS sym
		  JOIN semantic_files AS f
		    ON f.snapshot_id = sym.snapshot_id AND f.file_id = sym.file_id
		 WHERE sym.snapshot_id = ?
		   AND f.path          = ?
		   AND (
		         (sym.start_line < ?) OR
		         (sym.start_line = ? AND sym.start_col <= ?)
		       )
		   AND (
		         (sym.end_line > ?) OR
		         (sym.end_line = ? AND sym.end_col >= ?)
		       )
		 ORDER BY (sym.end_byte - sym.start_byte) ASC
		 LIMIT 1
	`
	var stableKey string
	err = s.db.QueryRowContext(ctx, q,
		latest, path,
		line, line, col,
		line, line, col,
	).Scan(&stableKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("QuerySymbolByLocation(%q, %q, %d:%d): %w",
			repoID, path, line, col, err)
	}
	return stableKey, true, nil
}

// QueryNodeIDByStableKey resolves stable_key → graph.NodeID (=symbol_id) at
// the latest committed snapshot for repoID. Returns (0, false, nil) on miss
// (or when no committed snapshot exists).
//
// Phase 65 65-11 Task 1: paired with QuerySymbolByLocation to drive
// integSemanticLookup.ExpandFrom's BFS — start nodes are stable_keys, the
// adjacency map is keyed on graph.NodeID.
func (s *Store) QueryNodeIDByStableKey(ctx context.Context, repoID, stableKey string) (uint64, bool, error) {
	if s == nil || s.db == nil {
		return 0, false, errors.New("QueryNodeIDByStableKey: nil store")
	}
	latest, err := s.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		return 0, false, fmt.Errorf("QueryNodeIDByStableKey: %w", err)
	}
	if latest == 0 {
		return 0, false, nil
	}
	const q = `
		SELECT symbol_id FROM semantic_symbols
		 WHERE snapshot_id = ? AND stable_key = ?
		 LIMIT 1
	`
	var symID uint64
	if err := s.db.QueryRowContext(ctx, q, latest, stableKey).Scan(&symID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("QueryNodeIDByStableKey(%q, %q): %w", repoID, stableKey, err)
	}
	return symID, true, nil
}

// QueryStableKeyByNodeID resolves graph.NodeID (=symbol_id) → stable_key at
// the latest committed snapshot for repoID. Returns ("", false, nil) on miss
// (or when no committed snapshot exists). Inverse of QueryNodeIDByStableKey.
//
// Phase 65 65-11 Task 1: ExpandFrom's BFS materializes []integ.Impact whose
// SymbolID field is a stable_key; the adjacency frontier is keyed on
// graph.NodeID, so this is the per-step translation.
func (s *Store) QueryStableKeyByNodeID(ctx context.Context, repoID string, nodeID uint64) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, errors.New("QueryStableKeyByNodeID: nil store")
	}
	latest, err := s.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		return "", false, fmt.Errorf("QueryStableKeyByNodeID: %w", err)
	}
	if latest == 0 {
		return "", false, nil
	}
	const q = `
		SELECT stable_key FROM semantic_symbols
		 WHERE snapshot_id = ? AND symbol_id = ?
		 LIMIT 1
	`
	var sk string
	if err := s.db.QueryRowContext(ctx, q, latest, nodeID).Scan(&sk); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("QueryStableKeyByNodeID(%q, %d): %w", repoID, nodeID, err)
	}
	return sk, true, nil
}

// IterateCommittedSymbols walks every semantic_symbols row at snapshotID
// in stable symbol_id ASC order, invoking fn(row). If fn returns false,
// iteration aborts cleanly without error. Honors ctx cancellation via the
// underlying QueryContext.
//
// Consumed by P64-07's bleve recovery probe to rebuild the FTS segment
// from a committed snapshot deterministically across daemon restarts.
//
// Path is JOINed in via semantic_files at the same snapshot_id so the
// returned SymbolRow is self-contained — the bleve corpus mapper does
// not need a second round-trip per symbol.
//
// Lock-free per scheduler_store.go:48-56 (no overlay tx required; reads
// committed state only).
func (s *Store) IterateCommittedSymbols(ctx context.Context, snapshotID uint64, fn func(SymbolRow) bool) error {
	if s == nil || s.db == nil {
		return errors.New("IterateCommittedSymbols: nil store")
	}
	if fn == nil {
		return errors.New("IterateCommittedSymbols: nil fn")
	}
	const q = `
		SELECT sym.symbol_id, sym.name, COALESCE(f.path, ''),
		       sym.file_id, sym.start_line
		  FROM semantic_symbols AS sym
		  LEFT JOIN semantic_files AS f
		    ON f.snapshot_id = sym.snapshot_id
		   AND f.file_id     = sym.file_id
		 WHERE sym.snapshot_id = ?
		 ORDER BY sym.symbol_id ASC
	`
	rows, err := s.db.QueryContext(ctx, q, snapshotID)
	if err != nil {
		return fmt.Errorf("IterateCommittedSymbols(snap=%d): %w", snapshotID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			symbolID  uint64
			name      string
			path      string
			fileID    uint64
			lineStart int
		)
		if err := rows.Scan(&symbolID, &name, &path, &fileID, &lineStart); err != nil {
			return fmt.Errorf("IterateCommittedSymbols scan: %w", err)
		}
		row := SymbolRow{
			SymbolID:  strconv.FormatUint(symbolID, 10),
			Name:      name,
			Path:      path,
			Docstring: "", // Schema 5: no docstring column; corpus loader supplies via source-file read.
			FileID:    strconv.FormatUint(fileID, 10),
			LineStart: lineStart,
		}
		if !fn(row) {
			return nil
		}
	}
	return rows.Err()
}
