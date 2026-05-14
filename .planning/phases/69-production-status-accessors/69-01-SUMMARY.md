---
phase: 69
plan: 01
subsystem: semantic-store
tags: [semantic-graph, status, cluster, read-accessor, race-clean, d-09]
dependency_graph:
  requires: []
  provides:
    - "*Store.ClusterStatusForGraphVersion read accessor"
    - "ClusterStatusRow type (GraphVersion + ActualGraphVersion + IsCurrent discriminators)"
    - "seedClusterRows test helper for Plan 69-06 reuse"
  affects:
    - "Plan 69-05 (semSchedulerAdapter.ClusterStatus) — consumes this accessor"
    - "Plan 69-04 (skill status handler) — intentionally breaks build until 69-05 lands"
tech_stack:
  added: []
  patterns:
    - "Read accessor on *Store (lock-free per D-09)"
    - "Two-step SQL with prior-version fallback (cold path)"
    - "DuckDB EXTRACT(EPOCH FROM ts) * 1000 → BIGINT unix-ms cast"
    - "sql.NullInt64 scan for DuckDB MAX over empty set (NULL not 0)"
key_files:
  created: []
  modified:
    - internal/semantic/store/effective_graph.go
    - internal/semantic/store/effective_graph_test.go
decisions:
  - "MemberCount derived from COUNT(*) over semantic_cluster_members, not from the overloaded semantic_clusters.score column"
  - "Stale-fallback returns highest STRICTLY-LOWER gv (no silent forward to future gvs)"
  - "Receiver pinned to *Store as compile-time D-09 guard against accidental OverlayTx coupling"
  - "Defensive guard: if MAX returns a prior gv but re-aggregation finds zero clusters, surface zero-value (genuinely empty) instead of misreporting stale"
metrics:
  duration: "~25min wall, single execute pass"
  completed: "2026-05-14"
requirements: [STATUS-01]
---

# Phase 69 Plan 01: Production Status Accessors — ClusterStatusForGraphVersion Summary

JWT-shaped read seam over `semantic_clusters` + `semantic_cluster_members` exposing cluster cardinality, member cardinality, and `computed_at` to the upcoming Plan 69-05 status adapter, with `IsCurrent` / `ActualGraphVersion` discriminators so the adapter can emit `current` / `stale` / `unknown` deterministically.

## Artifacts Shipped

| Artifact | Path | Hash |
| -------- | ---- | ---- |
| `*Store.ClusterStatusForGraphVersion` + `ClusterStatusRow` + `aggregateClusterStatus` helper | `internal/semantic/store/effective_graph.go` | `adaa98d4` (GREEN) |
| `TestClusterStatusForGraphVersion_Basic` + `_StaleFallback` + `_LockFree_RaceSafe` + `seedClusterRows` / `clusterSeed` helper | `internal/semantic/store/effective_graph_test.go` | `eaf1068a` (RED) |

## Test Counts

- **3 test functions**, **7 logical cases** (4 sub-tests in `_Basic`, 2 sub-tests in `_StaleFallback`, 1 standalone race test):
  - `_Basic/Test1_BasicPopulatedCurrent_gv5_two_clusters_3plus2_members` — current-gv populated path.
  - `_Basic/Test4_TrulyEmpty_no_rows_at_any_gv_returns_zero_value` — empty-store zero-value path.
  - `_Basic/Test5_WrongGv_only_future_gv10_rows_query_gv5_no_fallback_to_future` — strict `< graphVersion` fallback (no forward to future gvs).
  - `_Basic/Test7_MemberCount_from_COUNT_not_score_column` — proves `MemberCount=3` over actual member rows when `score=99.0` (overlay.go:835 overload deliberately ignored).
  - `_StaleFallback/Test2_StaleFallback_only_gv4_query_gv5` — basic prior-gv fallback.
  - `_StaleFallback/Test3_StaleFallback_picks_highest_prior_gv2_and_gv4_query_gv5` — highest-prior (not lowest) selection on multi-prior repo.
  - `_LockFree_RaceSafe` — 100 concurrent readers + background `BeginOverlayTx → Commit` writer, 5s per-reader timeout, no errors and no deadlock.
- **All 7 PASS under `-race -timeout 60s`** on the new tests, and the full `internal/semantic/store` package green under `-race -timeout 120s` (no regression).

## SQL Final Form

### Step 1 — Primary aggregation (extracted into `aggregateClusterStatus` helper)

```sql
SELECT
  COALESCE(MAX(EXTRACT(EPOCH FROM c.computed_at) * 1000), 0)::BIGINT AS computed_at_ms,
  COUNT(DISTINCT c.cluster_id)                                       AS cluster_count,
  (SELECT COUNT(*)
     FROM semantic_cluster_members m
    WHERE m.repo_id = ? AND m.graph_version = ?)                     AS member_count
FROM semantic_clusters AS c
WHERE c.repo_id       = ?
  AND c.graph_version = ?
```

Bind order: `(repoID, gv, repoID, gv)` — member subquery first, outer WHERE second. `cluster_count == 0` is the in-band signal for "no rows at this gv"; the helper returns `(zero, false, nil)` so the caller can branch into Step 2.

### Step 2 — Highest prior `graph_version` for stale fallback

```sql
SELECT MAX(graph_version)
  FROM semantic_clusters
 WHERE repo_id       = ?
   AND graph_version < ?
```

Scanned into `sql.NullInt64`. Null result → genuinely empty → return `ClusterStatusRow{GraphVersion: graphVersion}` zero-value with `nil` error. Valid result → re-run Step 1 against the resolved `priorGV` and return `IsCurrent=false, ActualGraphVersion=priorGV`.

## DuckDB Quirks Encountered

1. **`EXTRACT(EPOCH FROM ts) * 1000` returns DOUBLE**, not BIGINT. Without the explicit `::BIGINT` cast, the Go scan into `int64` errors with a type-mismatch. The `COALESCE(..., 0)` wrap is also required because over an empty rowset the `MAX(...)` term is NULL — without coalescing the cast fails (`NULL::BIGINT` is fine but mixed-arithmetic against a NULL leaks NaN/NULL semantics into the result row).
2. **`MAX(graph_version)` over an empty filter returns NULL, not 0** — Step 2 MUST scan into `sql.NullInt64`. The first draft used a plain `var int64` and silently treated "no prior gv" as `priorGV=0`, which then re-ran Step 1 at gv=0 and would defensively surface zero-value anyway, but the explicit null path is the correctness contract.
3. **No `sql.ErrNoRows` from `QueryRowContext` on aggregation queries** — DuckDB always returns exactly one row for `SELECT COUNT(...), MAX(...) FROM t WHERE ...` (the row is `(0, NULL)` for an empty match). The `errors.Is(err, sql.ErrNoRows)` branch in the helper is defensive scaffolding; it never fires for this query shape.

## Deviations from Plan

**None significant.** Two minor refactors during execution:

1. **Extracted `aggregateClusterStatus` private helper** — the plan described running the Step-1 aggregation inline twice (once at requested gv, once at fallback gv). The helper de-duplicates the SQL and bind-order plumbing into a single seven-line function returning `(result, found, err)`. Improves readability without changing the surface contract. Tracked as a minor implementation-detail refinement, not a Rule-flagged deviation.
2. **`tx.Abort()` → `tx.Rollback()` in `seedClusterRows`** — the plan mentioned `Abort` as the error-path teardown name; the actual `*OverlayTx` method is `Rollback()` (overlay.go:326). Caught at compile-time during RED verification; trivial typo-class fix, no Rule-1 bug because the helper is test-only and never failed in CI. Documented here for the planner's reference.
3. **`clusterSeed.actualMembers` field added** — to make Test 7 (MemberCount derivation) able to seed `ClusterSummary.MemberCount=99` while writing only 3 actual `semantic_cluster_members` rows, the helper takes an optional `actualMembers` override. When zero, defaults to `memberCount` (so Tests 1–6 stay one-field clean). This is a strictly-additive helper enhancement; downstream Plan 69-06 reuse remains usable with the same default-zero ergonomics.

## `seedClusterRows` Shape (As Shipped)

```go
type clusterSeed struct {
    id            uint64
    memberCount   int // → semantic_clusters.score via UpsertClusters
    actualMembers int // → number of semantic_cluster_members rows; 0 → memberCount
}

func seedClusterRows(t *testing.T, ctx context.Context, s *Store, repoID string, gv uint64, seeds []clusterSeed)
```

Plan 69-06's integration test can call `seedClusterRows(t, ctx, s, "r-it", 5, []clusterSeed{{id:1, memberCount:3}, {id:2, memberCount:2}})` — same package, same signature. No future migration of the helper is anticipated.

## D-09 Audit (Read-Path Purity)

```bash
$ awk '/^func \(s \*Store\) ClusterStatusForGraphVersion/,/^}/' internal/semantic/store/effective_graph.go \
    | grep -nE 'Begin(Snapshot|OverlayTx)|CommitSnapshot|AbortSnapshot|WriteSnapshotFacts'
(zero matches)
$ awk '/^func \(s \*Store\) aggregateClusterStatus/,/^}/' internal/semantic/store/effective_graph.go \
    | grep -nE 'Begin(Snapshot|OverlayTx)|CommitSnapshot|AbortSnapshot|WriteSnapshotFacts'
(zero matches)
```

Both function bodies are pure `s.db.QueryRowContext` calls; receiver pinned to `*Store` (NOT `*OverlayTx`) provides the compile-time guard. Race-safety test exercises 100 concurrent readers under live `BeginOverlayTx` writer churn with zero deadlock and zero error.

## Threat Surface Scan

No new network endpoints, auth paths, or trust-boundary schema changes. The accessor returns counts + computed_at + graph_version only — no `c.label`, no `c.summary`, no `m.node_id` (T-69-01 mitigation per the plan's STRIDE register holds). No threat flags raised.

## Handoff to Plan 69-05

- Import `internal/semantic/store.ClusterStatusRow` directly; the struct is exported from `package store`.
- Adapter mapping invariant (re-stated from the plan):
  - `row.ClusterCount > 0 && row.IsCurrent`  → emit `"current"`.
  - `row.ClusterCount > 0 && !row.IsCurrent` → emit `"stale"`, set `Reason="graph_version-lag"`, surface `row.ActualGraphVersion` to the caller.
  - `row.ClusterCount == 0`                  → emit `"unknown"`, set `Reason="no-cluster-rows"`.
- `row.ComputedAt` is unix-milliseconds (matches the daemon's status-payload convention). Convert with `time.UnixMilli(row.ComputedAt)` when re-serializing into a human-readable timestamp.
- No locking concerns for the adapter — call from any goroutine, any context, without grabbing the workspace lock.

## Self-Check: PASSED

- `internal/semantic/store/effective_graph.go` — FOUND
- `internal/semantic/store/effective_graph_test.go` — FOUND
- Commit `eaf1068a` (RED `test(69-01)`) — FOUND in `git log`
- Commit `adaa98d4` (GREEN `feat(69-01)`) — FOUND in `git log`
- `go vet ./internal/semantic/store/...` — clean
- `go test ./internal/semantic/store/... -race -count=1` — PASS
- D-09 grep audit on both new functions — zero matches
- TDD gate sequence in git log: `test(69-01)` → `feat(69-01)` — PRESENT
