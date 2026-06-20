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

// queryContext is the single semantic-store READ chokepoint for multi-row
// queries on the database handle (s.db). Phase 81 ABLATE-06: every read that
// funnels through here increments helix_semantic_store_reads_total via
// SemanticStoreReadsInc, so the no_semantic ablation arm's "zero reads"
// assertion (D-05) is faithful. Writes/maintenance (Exec, schema-version,
// migrations, BeginOverlayTx epoch bump) deliberately do NOT route through
// this helper — the counter is reads-only by construction (T-81-01-02).
//
// Routed read sites: QueryEffectiveAdjacency, CountStaleScoreRows,
// IterateCommittedSymbols, the effective_graph cluster/impact reads, and the
// overlay pure-read seams (CurrentGraphVersion, CurrentOverlayEpoch,
// OverlayChangedPathsSince). See the SUMMARY for the exact file:line set.
func (s *Store) queryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if s.metrics != nil {
		s.metrics.SemanticStoreReadsInc()
	}
	return s.db.QueryContext(ctx, query, args...)
}

// queryRowContext is the single-row sibling of queryContext (same read
// chokepoint contract). Phase 81 ABLATE-06.
func (s *Store) queryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if s.metrics != nil {
		s.metrics.SemanticStoreReadsInc()
	}
	return s.db.QueryRowContext(ctx, query, args...)
}

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
	rows, err := s.queryContext(ctx, q, repoID, projection, repoID, repoID, projection)
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
	if err := s.queryRowContext(ctx, q, repoID, projection).Scan(&stale, &total); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("CountStaleScoreRows(%q, %q): %w", repoID, projection, err)
	}
	return stale, total, nil
}

// ClusterStatusRow is the read-side payload returned by
// ClusterStatusForGraphVersion. It captures everything the Plan 69-05
// adapter needs to derive `"current"` / `"stale"` / `"unknown"` status:
//
//   - GraphVersion       — the graph_version the caller asked about.
//   - ActualGraphVersion — the graph_version whose rows we actually
//     returned (== GraphVersion when IsCurrent, else the highest prior
//     graph_version with rows for the repo).
//   - IsCurrent          — true when rows for the requested graph_version
//     were found at the primary lookup; false when the accessor fell
//     back to a prior graph_version OR returned zero-value (no rows).
//   - ComputedAt         — unix milliseconds derived from
//     MAX(computed_at) of the returned rows; 0 when zero-value.
//   - MemberCount        — COUNT(*) over semantic_cluster_members at
//     ActualGraphVersion. Deliberately NOT read from
//     semantic_clusters.score (which UpsertClusters overloads with the
//     planning-time MemberCount — see overlay.go:835 + 69-PATTERNS).
//   - ClusterCount       — COUNT(DISTINCT cluster_id) at
//     ActualGraphVersion.
//
// Zero value (IsCurrent=false, ActualGraphVersion=0, ComputedAt=0,
// MemberCount=0, ClusterCount=0) with GraphVersion = requested
// graph_version signals "no cluster rows for this repo at any
// graph_version". The adapter maps this to `"unknown"`.
type ClusterStatusRow struct {
	GraphVersion       uint64
	ActualGraphVersion uint64
	IsCurrent          bool
	ComputedAt         int64
	MemberCount        int
	ClusterCount       int
}

// ClusterStatusForGraphVersion returns the cluster summary state for
// (repoID, graphVersion). When rows exist at the requested graph_version,
// returns IsCurrent=true with ActualGraphVersion = graphVersion. When no
// rows exist at the requested gv but rows exist at a strictly-lower gv
// for the same repo, returns IsCurrent=false with ActualGraphVersion set
// to the highest such prior gv and counts/timestamp drawn from THAT prior
// gv (Approach A — closes 69-CONTEXT D1 "stale" semantics).
//
// When NO cluster rows exist at any graph_version for repoID, returns
// the zero value of ClusterStatusRow with GraphVersion = graphVersion and
// nil error. The Plan 69-05 adapter discriminates the three states:
//
//   - ClusterCount > 0 && IsCurrent  → "current"
//   - ClusterCount > 0 && !IsCurrent → "stale" (graph_version-lag)
//   - ClusterCount == 0              → "unknown" (no-cluster-rows)
//
// Lock-free per D-09: receiver is *Store (NOT *OverlayTx); both queries
// are pure SELECTs on s.db — no Begin/Commit/Abort/Write on the read
// path. Two queries on the fallback path is acceptable since fallback is
// the cold path; current-gv hits return in one round trip.
//
// MemberCount derivation: COUNT(*) over semantic_cluster_members at the
// resolved graph_version. MUST NOT be read from semantic_clusters.score
// (which UpsertClusters overloads at overlay.go:835 with the planning-time
// cardinality — that value can desync from the persisted member rows when
// UpsertClusterMembers is partially applied or the cluster's membership
// edits across runs).
func (s *Store) ClusterStatusForGraphVersion(ctx context.Context, repoID string, graphVersion uint64) (ClusterStatusRow, error) {
	if s == nil || s.db == nil {
		return ClusterStatusRow{}, errors.New("ClusterStatusForGraphVersion: nil store")
	}

	// Step 1 — primary aggregation at the requested graph_version.
	row, found, err := s.aggregateClusterStatus(ctx, repoID, graphVersion)
	if err != nil {
		return ClusterStatusRow{}, fmt.Errorf("ClusterStatusForGraphVersion(%q, %d): %w", repoID, graphVersion, err)
	}
	if found {
		return ClusterStatusRow{
			GraphVersion:       graphVersion,
			ActualGraphVersion: graphVersion,
			IsCurrent:          true,
			ComputedAt:         row.ComputedAt,
			MemberCount:        row.MemberCount,
			ClusterCount:       row.ClusterCount,
		}, nil
	}

	// Step 2 — fallback: highest prior graph_version (strict <) with
	// rows for this repo. DuckDB MAX over an empty set returns NULL, so
	// scan into sql.NullInt64 and treat null as "no prior gv exists".
	const priorQ = `
		SELECT MAX(graph_version)
		  FROM semantic_clusters
		 WHERE repo_id       = ?
		   AND graph_version < ?
	`
	var prior sql.NullInt64
	if err := s.queryRowContext(ctx, priorQ, repoID, graphVersion).Scan(&prior); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ClusterStatusRow{GraphVersion: graphVersion}, nil
		}
		return ClusterStatusRow{}, fmt.Errorf("ClusterStatusForGraphVersion(%q, %d) prior lookup: %w", repoID, graphVersion, err)
	}
	if !prior.Valid {
		// No rows at requested gv AND no rows at any strictly-lower gv
		// for this repo → genuinely empty. The adapter sees ClusterCount=0
		// and emits "unknown".
		return ClusterStatusRow{GraphVersion: graphVersion}, nil
	}
	priorGV := uint64(prior.Int64)

	priorRow, priorFound, err := s.aggregateClusterStatus(ctx, repoID, priorGV)
	if err != nil {
		return ClusterStatusRow{}, fmt.Errorf("ClusterStatusForGraphVersion(%q, %d) fallback at gv=%d: %w", repoID, graphVersion, priorGV, err)
	}
	if !priorFound {
		// Defensive: MAX(graph_version) said there's a row at priorGV but
		// the aggregation returned zero clusters. Cannot happen with the
		// current schema (semantic_clusters MAX over its own table) but
		// guard anyway — surface as no-data rather than misreport stale.
		return ClusterStatusRow{GraphVersion: graphVersion}, nil
	}
	return ClusterStatusRow{
		GraphVersion:       graphVersion,
		ActualGraphVersion: priorGV,
		IsCurrent:          false,
		ComputedAt:         priorRow.ComputedAt,
		MemberCount:        priorRow.MemberCount,
		ClusterCount:       priorRow.ClusterCount,
	}, nil
}

// clusterAggResult is the internal counts payload of aggregateClusterStatus.
type clusterAggResult struct {
	ComputedAt   int64
	ClusterCount int
	MemberCount  int
}

// aggregateClusterStatus runs the single read-only aggregation query for
// (repoID, gv) and returns (result, found, err). `found` is true iff at
// least one row in semantic_clusters matched — equivalent to ClusterCount > 0.
//
// SQL strategy:
//   - MAX(computed_at) cast to unix milliseconds via DuckDB
//     EXTRACT(EPOCH FROM ts) * 1000, COALESCE'd to 0 over the empty set.
//   - COUNT(DISTINCT cluster_id) over semantic_clusters for the cluster count.
//   - Correlated subquery on semantic_cluster_members for the authoritative
//     member count (NOT semantic_clusters.score — see ClusterStatusRow
//     doc-comment).
func (s *Store) aggregateClusterStatus(ctx context.Context, repoID string, gv uint64) (clusterAggResult, bool, error) {
	const q = `
		SELECT
		  COALESCE(MAX(EXTRACT(EPOCH FROM c.computed_at) * 1000), 0)::BIGINT AS computed_at_ms,
		  COUNT(DISTINCT c.cluster_id)                                       AS cluster_count,
		  (SELECT COUNT(*)
		     FROM semantic_cluster_members m
		    WHERE m.repo_id = ? AND m.graph_version = ?)                     AS member_count
		FROM semantic_clusters AS c
		WHERE c.repo_id       = ?
		  AND c.graph_version = ?
	`
	var (
		computedAtMs int64
		clusterCount int
		memberCount  int
	)
	if err := s.queryRowContext(ctx, q, repoID, gv, repoID, gv).Scan(&computedAtMs, &clusterCount, &memberCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return clusterAggResult{}, false, nil
		}
		return clusterAggResult{}, false, err
	}
	if clusterCount == 0 {
		return clusterAggResult{}, false, nil
	}
	return clusterAggResult{
		ComputedAt:   computedAtMs,
		ClusterCount: clusterCount,
		MemberCount:  memberCount,
	}, true, nil
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
	if err := s.queryRowContext(ctx, q, repoID).Scan(&id); err != nil {
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
	rows, err := s.queryContext(ctx, q, args...)
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
	if err := s.queryRowContext(ctx, q, snapshotID, symID).Scan(&path); err != nil {
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
	err = s.queryRowContext(ctx, q,
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

// QuerySymbolByName returns up to 6 stable_keys for symbols matching
// (path, name) at the latest committed snapshot for repoID. Results are
// ordered deterministically (stable_key ASC). The LIMIT 6 cap allows the
// caller (Phase 71-01 seed resolver) to detect "> 5 matches" and apply the
// D1 ambiguity truncation policy (cap 5).
//
// Returns (nil, nil) on clean miss (unknown name, or no committed snapshot
// exists). Empty repoID or path is treated as a miss, not an error.
//
// Phase 71-01 Task 1. Mirrors QuerySymbolByLocation's read-only lock-free
// shape; reads via s.db with no overlay tx.
func (s *Store) QuerySymbolByName(
	ctx context.Context, repoID, path, name string,
) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("QuerySymbolByName: nil store")
	}
	latest, err := s.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("QuerySymbolByName(%q,%q): %w", path, name, err)
	}
	if latest == 0 {
		return nil, nil
	}
	const q = `
		SELECT sym.stable_key
		  FROM semantic_symbols AS sym
		  JOIN semantic_files AS f
		    ON f.snapshot_id = sym.snapshot_id AND f.file_id = sym.file_id
		 WHERE sym.snapshot_id = ?
		   AND f.path          = ?
		   AND sym.name        = ?
		 ORDER BY sym.stable_key ASC
		 LIMIT 6
	`
	rows, err := s.queryContext(ctx, q, latest, path, name)
	if err != nil {
		return nil, fmt.Errorf("QuerySymbolByName(%q,%q): %w", path, name, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var sk string
		if err := rows.Scan(&sk); err != nil {
			return nil, fmt.Errorf("QuerySymbolByName(%q,%q) scan: %w", path, name, err)
		}
		out = append(out, sk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("QuerySymbolByName(%q,%q) rows.Err: %w", path, name, err)
	}
	return out, nil
}

// LatestExtractorRunID returns a monotonic opaque identifier for the most
// recent extractor pass against repoID. Phase 71-01 Task 1.
//
// Resolution policy (per RESEARCH A4): no dedicated extractor_run_id
// column or semantic_extractor_runs table exists in Schema 5. The id is
// derived from LatestCommittedSnapshot as `snap-<snapshot_id>`, which
// satisfies the contract that the id advances monotonically every time
// the extractor commits new state. A dedicated column can replace this
// derivation later without changing the accessor signature.
//
// Returns ("", nil) when no committed snapshot exists for repoID
// (matches the empty-repo / cold-start case).
func (s *Store) LatestExtractorRunID(ctx context.Context, repoID string) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("LatestExtractorRunID: nil store")
	}
	latest, err := s.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		return "", fmt.Errorf("LatestExtractorRunID(%q): %w", repoID, err)
	}
	if latest == 0 {
		return "", nil
	}
	return fmt.Sprintf("snap-%d", latest), nil
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
	if err := s.queryRowContext(ctx, q, latest, stableKey).Scan(&symID); err != nil {
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
	if err := s.queryRowContext(ctx, q, latest, nodeID).Scan(&sk); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("QueryStableKeyByNodeID(%q, %d): %w", repoID, nodeID, err)
	}
	return sk, true, nil
}

// QuerySymbolLocationByStableKey returns the (path, start_line, start_col)
// of the symbol identified by stableKey at the latest committed snapshot for
// repoID. Returns ("", 0, 0, false, nil) on miss (or when no committed
// snapshot exists). Phase 65 65-12 Task 1 — backs the production
// integSemanticLookup.LocateSymbol method, which the kernel-side
// analyze_blast_radius Pass-2 LSP probe consumes to derive the (line, col)
// input for FindReferences from a stable SymbolID.
//
// Returned line/col are the symbol's start coordinates, 1-based, matching
// the LSP external surface convention used by SymbolID(). The smallest
// inner-scope tiebreak is NOT applied here — by stable_key, every symbol is
// uniquely identified.
//
// Reads under no workspace lock (scheduler_store.go:48-56).
func (s *Store) QuerySymbolLocationByStableKey(
	ctx context.Context, repoID, stableKey string,
) (path string, line, col uint32, ok bool, err error) {
	if s == nil || s.db == nil {
		return "", 0, 0, false, errors.New("QuerySymbolLocationByStableKey: nil store")
	}
	latest, e := s.LatestCommittedSnapshot(ctx, repoID)
	if e != nil {
		return "", 0, 0, false, fmt.Errorf("QuerySymbolLocationByStableKey: %w", e)
	}
	if latest == 0 {
		return "", 0, 0, false, nil
	}
	const q = `
		SELECT f.path, sym.start_line, sym.start_col
		  FROM semantic_symbols AS sym
		  JOIN semantic_files AS f
		    ON f.snapshot_id = sym.snapshot_id
		   AND f.file_id     = sym.file_id
		 WHERE sym.snapshot_id = ?
		   AND sym.stable_key  = ?
		 LIMIT 1
	`
	var startLine, startCol int
	err = s.queryRowContext(ctx, q, latest, stableKey).Scan(&path, &startLine, &startCol)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", 0, 0, false, nil
		}
		return "", 0, 0, false, fmt.Errorf("QuerySymbolLocationByStableKey(%q, %q): %w",
			repoID, stableKey, err)
	}
	return path, uint32(startLine), uint32(startCol), true, nil
}

// ----- Phase 72-01 additions: cluster & impact read methods -----

// ClusterSummaryResult is the store-internal read payload returned by
// QueryClusterSummaries. The skill-layer ClusterSummaryRow in accessors.go
// mirrors this type; the accessor adapter bridges the two.
type ClusterSummaryResult struct {
	ClusterIntID uint64
	MemberCount  int
}

// ClusterMemberResult is the store-internal read payload returned by
// QueryClusterMembers. The skill-layer ClusterMemberRow in accessors.go
// mirrors this type; the accessor adapter bridges the two.
type ClusterMemberResult struct {
	NodeID   uint64
	SymbolID string
}

// QueryClusterSummaries returns the top-N cluster summaries for
// (repoID, projection, graphVersion) ordered by score DESC. The `score`
// column is overloaded by overlay.go:835 to store the planning-time
// MemberCount, so CAST(score AS INTEGER) retrieves the member count.
//
// Returns (nil, nil) when the store is nil or no rows exist for the
// requested (repoID, graphVersion). The projection parameter is accepted
// for future multi-projection support but is not yet filtered in the SQL —
// semantic_clusters does not carry a projection column in Schema 5
// (projection=algorithm is the identifier used by the caller convention).
//
// Lock-free per scheduler_store.go:48-56 (pure SELECT on s.db).
// Threat T-72-01-02: topN is passed as a bound positional parameter.
func (s *Store) QueryClusterSummaries(
	ctx context.Context, repoID, projection string, graphVersion uint64, topN int,
) ([]ClusterSummaryResult, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	const q = `
		SELECT cluster_id, CAST(score AS INTEGER) AS member_count
		  FROM semantic_clusters
		 WHERE repo_id      = ?
		   AND graph_version = ?
		 ORDER BY score DESC
		 LIMIT ?
	`
	rows, err := s.queryContext(ctx, q, repoID, graphVersion, topN)
	if err != nil {
		return nil, fmt.Errorf("QueryClusterSummaries(%q, %q, gv=%d): %w", repoID, projection, graphVersion, err)
	}
	defer rows.Close()

	var out []ClusterSummaryResult
	for rows.Next() {
		var r ClusterSummaryResult
		if err := rows.Scan(&r.ClusterIntID, &r.MemberCount); err != nil {
			return nil, fmt.Errorf("QueryClusterSummaries scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("QueryClusterSummaries rows.Err: %w", err)
	}
	return out, nil
}

// QueryClusterMembers returns the member rows for the given
// (repoID, projection, graphVersion, clusterIntID) with a JOIN to
// semantic_symbols to surface stable_key as SymbolID.
//
// Returns (nil, nil) when the store is nil. Empty slice with nil error
// signals the cluster has no members or does not exist.
//
// The projection parameter is accepted for API consistency but not used in
// the SQL (Schema 5 semantic_cluster_members has no projection column).
// Lock-free per scheduler_store.go:48-56.
// Threat T-72-01-01: all parameters are bound positional args.
func (s *Store) QueryClusterMembers(
	ctx context.Context, repoID, projection string, graphVersion, clusterIntID uint64, limit int,
) ([]ClusterMemberResult, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	const q = `
		SELECT scm.node_id, ss.stable_key AS symbol_id
		  FROM semantic_cluster_members AS scm
		  JOIN semantic_symbols AS ss
		    ON ss.node_id      = scm.node_id
		   AND ss.snapshot_id  = (
		         SELECT MAX(snapshot_id)
		           FROM semantic_snapshots
		          WHERE repo_id = scm.repo_id
		            AND status  = 'committed'
		       )
		 WHERE scm.repo_id       = ?
		   AND scm.graph_version = ?
		   AND scm.cluster_id    = ?
		 ORDER BY scm.node_id ASC
		 LIMIT ?
	`
	rows, err := s.queryContext(ctx, q, repoID, graphVersion, clusterIntID, limit)
	if err != nil {
		return nil, fmt.Errorf("QueryClusterMembers(%q, %q, gv=%d, cluster=%d): %w",
			repoID, projection, graphVersion, clusterIntID, err)
	}
	defer rows.Close()

	var out []ClusterMemberResult
	for rows.Next() {
		var r ClusterMemberResult
		if err := rows.Scan(&r.NodeID, &r.SymbolID); err != nil {
			return nil, fmt.Errorf("QueryClusterMembers scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("QueryClusterMembers rows.Err: %w", err)
	}
	return out, nil
}

// QueryNodePageRanks returns a map[nodeID]score from semantic_graph_scores
// for the supplied nodeIDs where score_name matches the projection parameter
// and graph_version matches graphVersion.
//
// Returns (nil, nil) when the store is nil or nodeIDs is empty. Missing
// nodes (those without a score row) are omitted from the result map.
//
// Threat T-72-01-02: the IN clause is built with positional bound parameters,
// never via string concatenation, preventing SQL injection.
func (s *Store) QueryNodePageRanks(
	ctx context.Context, repoID, projection string, graphVersion uint64, nodeIDs []uint64,
) (map[uint64]float64, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if len(nodeIDs) == 0 {
		return map[uint64]float64{}, nil
	}

	// Build positional IN clause: one ? per nodeID.
	placeholders := make([]byte, 0, len(nodeIDs)*3)
	args := make([]any, 0, 3+len(nodeIDs))
	args = append(args, repoID, projection, graphVersion)
	for i, nid := range nodeIDs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, nid)
	}

	q := "SELECT node_id, score FROM semantic_graph_scores" +
		" WHERE repo_id = ? AND score_name = ? AND graph_version = ?" +
		" AND node_id IN (" + string(placeholders) + ")"

	rows, err := s.queryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("QueryNodePageRanks(%q, %q, gv=%d): %w", repoID, projection, graphVersion, err)
	}
	defer rows.Close()

	out := make(map[uint64]float64, len(nodeIDs))
	for rows.Next() {
		var nodeID uint64
		var score float64
		if err := rows.Scan(&nodeID, &score); err != nil {
			return nil, fmt.Errorf("QueryNodePageRanks scan: %w", err)
		}
		out[nodeID] = score
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("QueryNodePageRanks rows.Err: %w", err)
	}
	return out, nil
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
	rows, err := s.queryContext(ctx, q, snapshotID)
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

// ----- Phase 74 P1 two-hop accessor helpers (D-01a FOLD) -----
//
// These three methods are thin read-only helpers consumed by the two
// two-hop adapter structs in internal/daemon/semantic_wiring.go.
// All SQL uses positional parameters (Threats T-74-03-01, T-74-03-02).

// SymbolEdgeRaw is the intermediate row type used by
// QuerySymbolEdgesIncoming and QuerySymbolEdgesOutgoing.
// Exported so daemon-package adapters can iterate the results.
type SymbolEdgeRaw struct {
	SrcNodeID uint64
	DstNodeID uint64
	EdgeKind  string
}

// QuerySymbolEdgesIncoming returns edge rows whose dst_node_id = dstNodeID
// at the given snapshotID. When callsOnly is true, adds AND edge_kind='CALLS'
// (satisfying CallersOf direction — Pitfall 2: CallersOf must filter edge_kind).
//
// JOIN on semantic_symbols.symbol_id (NOT node_id — Pitfall 1: symbol_id is
// the same uint64 as src_node_id/dst_node_id in semantic_edges).
//
// Lock-free (pure SELECT on s.db).
func (s *Store) QuerySymbolEdgesIncoming(ctx context.Context, snapshotID, dstNodeID uint64, callsOnly bool) ([]SymbolEdgeRaw, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var rows *sql.Rows
	var err error
	if callsOnly {
		const q = `
			SELECT e.src_node_id, e.dst_node_id, e.edge_kind
			  FROM semantic_edges AS e
			 WHERE e.snapshot_id = ?
			   AND e.dst_node_id = ?
			   AND e.edge_kind   = 'CALLS'
		`
		rows, err = s.queryContext(ctx, q, snapshotID, dstNodeID)
	} else {
		const q = `
			SELECT e.src_node_id, e.dst_node_id, e.edge_kind
			  FROM semantic_edges AS e
			 WHERE e.snapshot_id = ?
			   AND e.dst_node_id = ?
		`
		rows, err = s.queryContext(ctx, q, snapshotID, dstNodeID)
	}
	if err != nil {
		return nil, fmt.Errorf("QuerySymbolEdgesIncoming(snap=%d, dst=%d, callsOnly=%v): %w", snapshotID, dstNodeID, callsOnly, err)
	}
	defer rows.Close()
	var out []SymbolEdgeRaw
	for rows.Next() {
		var r SymbolEdgeRaw
		if err := rows.Scan(&r.SrcNodeID, &r.DstNodeID, &r.EdgeKind); err != nil {
			return nil, fmt.Errorf("QuerySymbolEdgesIncoming scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("QuerySymbolEdgesIncoming rows.Err: %w", err)
	}
	return out, nil
}

// QuerySymbolEdgesOutgoing returns edge rows whose src_node_id = srcNodeID
// at the given snapshotID (no edge_kind filter — OutgoingEdgesOf returns all kinds).
//
// Lock-free (pure SELECT on s.db).
func (s *Store) QuerySymbolEdgesOutgoing(ctx context.Context, snapshotID, srcNodeID uint64) ([]SymbolEdgeRaw, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	const q = `
		SELECT e.src_node_id, e.dst_node_id, e.edge_kind
		  FROM semantic_edges AS e
		 WHERE e.snapshot_id = ?
		   AND e.src_node_id = ?
	`
	rows, err := s.queryContext(ctx, q, snapshotID, srcNodeID)
	if err != nil {
		return nil, fmt.Errorf("QuerySymbolEdgesOutgoing(snap=%d, src=%d): %w", snapshotID, srcNodeID, err)
	}
	defer rows.Close()
	var out []SymbolEdgeRaw
	for rows.Next() {
		var r SymbolEdgeRaw
		if err := rows.Scan(&r.SrcNodeID, &r.DstNodeID, &r.EdgeKind); err != nil {
			return nil, fmt.Errorf("QuerySymbolEdgesOutgoing scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("QuerySymbolEdgesOutgoing rows.Err: %w", err)
	}
	return out, nil
}

// QueryClusterIDOfNode returns the cluster_id and member count for the
// cluster that contains nodeID at the given (repoID, graphVersion).
// member count is derived from CAST(c.score AS INTEGER) (UpsertClusters
// overloads semantic_clusters.score with the member count per overlay.go).
//
// Returns (0, 0, false, nil) when no row is found — caller emits
// fallback_reason="cluster_boost_unavailable".
//
// All three WHERE parameters are positional (Threat T-74-03-02).
// Lock-free (pure SELECT on s.db).
func (s *Store) QueryClusterIDOfNode(ctx context.Context, repoID string, graphVersion, nodeID uint64) (clusterID uint64, memberCount int, found bool, err error) {
	if s == nil || s.db == nil {
		return 0, 0, false, nil
	}
	const q = `
		SELECT cm.cluster_id, CAST(c.score AS INTEGER) AS member_count
		  FROM semantic_cluster_members AS cm
		  JOIN semantic_clusters AS c
		    ON c.repo_id       = cm.repo_id
		   AND c.graph_version = cm.graph_version
		   AND c.cluster_id    = cm.cluster_id
		 WHERE cm.repo_id       = ?
		   AND cm.graph_version = ?
		   AND cm.node_id       = ?
		 LIMIT 1
	`
	var cid uint64
	var mc int
	if scanErr := s.queryRowContext(ctx, q, repoID, graphVersion, nodeID).Scan(&cid, &mc); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return 0, 0, false, nil
		}
		return 0, 0, false, fmt.Errorf("QueryClusterIDOfNode(%q, gv=%d, node=%d): %w", repoID, graphVersion, nodeID, scanErr)
	}
	return cid, mc, true, nil
}
