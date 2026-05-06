---
phase: 62-graph-engine-ranking-type-resolution
plan: 04
subsystem: semantic-graph
tags: [graph, clustering, weak-components, union-find, determinism, sha256, golden-digest, overlay-tx, duckdb]

# Dependency graph
requires:
  - phase: 62-02
    provides: store.OverlayTx surface (BeginOverlayTx, Commit, Rollback) under per-workspace mutex (D-04)
  - phase: 62-03
    provides: graph.NodeID + util.sortedNodeIDs determinism helper template
  - phase: 60
    provides: per-workspace overlay mutex contract (D-04) reused for T-62-04-T4 mitigation
  - phase: 57
    provides: semantic_clusters + semantic_cluster_members schema (migration v1, SPEC §9.10)

provides:
  - cluster.WeakComponents: deterministic sorted-key union-find weak-component algorithm (GRAPH-06)
  - cluster.Cluster type (ID + sorted Members)
  - cluster.RunClusterDetection: full orchestration (CurrentGraphVersion → QueryEffectiveGraph → WeakComponents → Delete → UpsertClusters → UpsertClusterMembers → Commit) under existing per-workspace mutex
  - store.ClusterSummary + store.ClusterMemberRow boundary types (cluster ↔ store seam)
  - store.OverlayTx.UpsertClusters, UpsertClusterMembers, DeleteClustersForGraphVersion (writes to existing v1 schema)
  - Three pinned sha256 hex digests guaranteeing byte-equal cluster output across runs

affects: [phase-64-cluster-mcp-tools, phase-63-compaction]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Sorted-key union-find with smaller-ID-becomes-root tiebreak for deterministic component naming"
    - "Hex-digest golden + -count=10 multiplier for byte-equality determinism contracts (GRAPH-01/GRAPH-06)"
    - "Boundary types in the store package (ClusterSummary / ClusterMemberRow) to keep cluster→store imports acyclic"
    - "Algorithm-only delivery — MCP tools deferred to v1.10.x; algorithm + persistence ship now so downstream readers can light up later without re-touching this surface"

key-files:
  created:
    - internal/semantic/cluster/doc.go (28 LOC) — package contract incl. T1/T4 mitigations + v1.10.x deferral note
    - internal/semantic/cluster/weak.go (128 LOC) — WeakComponents + Cluster
    - internal/semantic/cluster/weak_test.go (270 LOC) — 8 tests incl. hex-digest goldens + -count=10 determinism
    - internal/semantic/cluster/persist.go (141 LOC) — RunClusterDetection orchestrator + bridge helpers
    - internal/semantic/cluster/persist_test.go (261 LOC) — 3 fake-tx tests covering call ordering, rollback, gv threading
    - internal/semantic/cluster/testdata/golden_three_components.txt — sha256 744cb4ae…
    - internal/semantic/cluster/testdata/golden_single_component.txt — sha256 f112d312…
    - internal/semantic/cluster/testdata/golden_isolated_nodes.txt — sha256 3281f71a…
  modified:
    - internal/semantic/store/overlay.go — +ClusterSummary, +ClusterMemberRow, +UpsertClusters, +UpsertClusterMembers, +DeleteClustersForGraphVersion
    - internal/semantic/store/overlay_test.go — +5 cluster persistence tests

key-decisions:
  - "Smaller-ID-becomes-root tiebreak. Cluster ID is therefore the smallest member NodeID, deterministic across runs, no separate mapping table required."
  - "Boundary types live in the store package, NOT cluster. cluster.RunClusterDetection converts []Cluster → []store.ClusterSummary at the call site via private toSummaries/toMemberRows helpers, so cluster→store calls stay one-directional."
  - "Algorithm name carries the projection identifier. The v1 schema stores algorithm TEXT NOT NULL but no explicit projection column; recording projection as algorithm makes re-runs at the same projection idempotent (same PK → UPDATE)."
  - "Member weight is fixed at 1.0. Weak-component membership is unconditional; the schema's weight DOUBLE NOT NULL column is satisfied with 1.0 to keep the SQL contract honored without inventing a meaningless weight gradient."
  - "snapshot_id = 0 for cluster rows. Mirrors the Phase 62 P02 score-row choice — clusters are overlay-side state keyed on (repo_id, graph_version); the base-snapshot relationship is re-established by Phase 63 compaction."
  - "DeleteClustersForGraphVersion scopes by (repo_id, graph_version) only, not by projection. The v1 schema's PK does not include projection, so deletes clear all algorithms for the version. Acceptable for v1 since only one algorithm (weak_components) writes today."

patterns-established:
  - "Sort-before-iterate at every node-keyed map ranged in this package (sortedNodes, sortedSrcs, sortedDsts, rootList) per Pitfall 1 of 62-RESEARCH.md"
  - "Capture-fake test pattern: a thin capturingFakeStore wraps the inner fake to expose the post-orchestration tx for sequencing assertions"
  - "Defer-rollback with committed flag — works correctly when the underlying tx (here OverlayTx) tolerates Rollback after Commit via its own once-guard"

requirements-completed: [GRAPH-06]

# Metrics
duration: ~50min
completed: 2026-05-06
---

# Phase 62 Plan 04: Weak-Component Clustering + Persistence Summary

**Deterministic sorted-key weak-component algorithm (GRAPH-06) shipped under `internal/semantic/cluster/` with three pinned sha256 goldens and full overlay-tx persistence via three new OverlayTx helpers — algorithm only, MCP tools deferred to v1.10.x.**

## Performance

- **Duration:** ~50 min
- **Started:** 2026-05-06T14:23:00Z
- **Completed:** 2026-05-06T15:12:18Z
- **Tasks:** 5/5
- **Files created:** 8 (5 .go + 3 testdata)
- **Files modified:** 2 (overlay.go + overlay_test.go)

## Accomplishments

- Deterministic weak-component algorithm with sorted union-find: same input → byte-identical Cluster slice across 10× same-process runs (TestWeakComponents_DeterministicAcrossRuns) and `-count=10` outer multiplier
- Three pinned sha256 hex digests (three_components, single_component, isolated_nodes) freeze the byte-equality contract against future drift
- Cluster identity stable: smallest-NodeID-becomes-root, no separate cluster-name table needed
- store.OverlayTx surface extended with three persistence helpers (UpsertClusters / UpsertClusterMembers / DeleteClustersForGraphVersion) writing to the existing Phase 57 v1 schema
- RunClusterDetection orchestrator with single-tx Delete→Upsert→Upsert→Commit sequencing under the existing Phase 60 D-04 per-workspace mutex (T-62-04-T4 satisfied without a second mutex map)
- Boundary types (ClusterSummary, ClusterMemberRow) keep cluster→store imports acyclic; cluster does NOT import store types into its own surface
- Zero MCP tool registrations in cluster package — `get_cluster_map` / `explain_cluster` explicitly deferred to v1.10.x

## Task Commits

Each task committed atomically:

1. **Task 1: RED — failing weak-component tests + placeholder goldens** — `08515fd1` (test)
2. **Task 2a: GREEN — implement weak.go (sorted union-find + deterministic naming)** — `f357ed2e` (feat)
3. **Task 2b: pin three sha256 hex digests after engine GREEN** — `4ce353c4` (test)
4. **Task 3: extend store.OverlayTx with cluster persistence helpers** — `e4b033fa` (feat)
5. **Task 4: implement RunClusterDetection orchestrator + persist_test.go** — `27c87fe9` (feat)
6. **Task 5: project-wide green** — no commit (clean out of the gate)

_TDD gate evidence:_ commit `08515fd1` (RED, type=`test`) precedes `f357ed2e` (GREEN, type=`feat`); the goldens were intentionally pinned in a follow-up `test()` commit (`4ce353c4`) AFTER the engine compiled because hex digests cannot be authored without a working algorithm.

## Files Created/Modified

### Created
- `internal/semantic/cluster/doc.go` — package contract (T1/T4 mitigations, v1.10.x deferral note for cluster MCP tools)
- `internal/semantic/cluster/weak.go` — `WeakComponents` algorithm + `Cluster` type
- `internal/semantic/cluster/weak_test.go` — 8 tests covering three/single/isolated/directed components, hex digests, determinism (10×), empty input, sorted members
- `internal/semantic/cluster/persist.go` — `RunClusterDetection` + `ClusterStore` / `ClusterTx` interfaces + `toSummaries` / `toMemberRows` boundary converters
- `internal/semantic/cluster/persist_test.go` — 3 tests covering RoundTrip (call ordering), RollsBackOnError, UsesCurrentGraphVersion
- `internal/semantic/cluster/testdata/golden_three_components.txt` — `744cb4aeaac7f475c189e76d65b901d16f805b2d473a3cea1fde89464baa226e`
- `internal/semantic/cluster/testdata/golden_single_component.txt` — `f112d312c50c75d708f4331e9b46b7b5ebac7bc87aa9e1f840ce40a910d700be`
- `internal/semantic/cluster/testdata/golden_isolated_nodes.txt` — `3281f71af5b23b75d48a6463d7b5ab19bf1af166ee07abf1a46a5c1cf3806ffb`

### Modified
- `internal/semantic/store/overlay.go` — +`ClusterSummary` + `ClusterMemberRow` types; +3 OverlayTx methods (134 LOC added)
- `internal/semantic/store/overlay_test.go` — +5 cluster persistence tests (223 LOC added)

## Decisions Made

1. **Boundary types in store, not cluster.** `internal/semantic/cluster` cannot import `internal/semantic/store` if the store needs to import `cluster.Cluster` — that's a cycle. Resolution: define `store.ClusterSummary` + `store.ClusterMemberRow` in overlay.go and have `cluster.RunClusterDetection` convert `[]Cluster → []store.ClusterSummary` at the call site via `toSummaries` / `toMemberRows`. cluster→store imports stay one-directional.
2. **Algorithm name carries the projection identifier.** The v1 schema (Phase 57 migration v1, SPEC §9.10a) stores `algorithm TEXT NOT NULL` but no explicit `projection` column. Recording projection as algorithm makes re-runs at the same projection idempotent through the `(repo_id, graph_version, cluster_id)` PK. This collapses two concerns the v1 schema didn't separate, but the choice is reversible: a future schema migration can split them and re-write upserts.
3. **Member weight = 1.0.** Weak components have no per-member weight (membership is unconditional). The schema requires `weight DOUBLE NOT NULL`; 1.0 satisfies the constraint without inventing a meaningless gradient. Future cluster algorithms (label propagation, soft membership) can populate this column meaningfully.
4. **snapshot_id = 0 for cluster rows.** Mirrors the Phase 62 P02 score-row choice — clusters are overlay-side state keyed on (repo_id, graph_version). Phase 63 compaction will re-key them onto a snapshot once it ships.
5. **DeleteClustersForGraphVersion does NOT scope by projection.** The v1 PK is `(repo_id, graph_version, cluster_id)` — no projection column. Deletes therefore clear every algorithm's clusters for the version. For v1 this is acceptable (only `weak_components` writes today). When a second algorithm ships, this method will need a new schema column and a per-algorithm scope; the helper's `projection` parameter is already accepted so the call site is forward-compatible.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Schema column mismatch in plan's example SQL**
- **Found during:** Task 3 (Extend store.OverlayTx)
- **Issue:** The plan's pseudocode for `UpsertClusters` referenced `member_count`, `write_epoch`, and `updated_at` columns on `semantic_clusters`. The v1 schema (Phase 57 migration v1, see migrations.go lines 265-289) has none of these. Actual columns: `algorithm TEXT NOT NULL`, `label TEXT`, `summary TEXT`, `score DOUBLE`, `status TEXT NOT NULL`, `computed_at TIMESTAMP NOT NULL`. Same situation on `semantic_cluster_members` — no `write_epoch` / `updated_at`; PK does NOT include `projection`.
- **Fix:** Adjusted SQL to match real schema:
  - `algorithm` carries the projection identifier (idempotent re-run via PK)
  - `score` carries member_count (DOUBLE-cast)
  - `status` is hard-coded `'exact'` (clustering is computed precisely from the effective graph; D-07's stale/missing enum doesn't apply)
  - `computed_at = now()` substitutes for updated_at
  - `member.weight = 1.0`, `role = NULL`
  - Two delete statements: members first, then clusters (FK-safe ordering)
- **Files modified:** internal/semantic/store/overlay.go (UpsertClusters, UpsertClusterMembers, DeleteClustersForGraphVersion all written against the real schema)
- **Verification:** All 5 new persistence tests pass; round-trip + composite-PK tests confirm INSERT vs UPDATE branches behave correctly against the real DuckDB schema.
- **Committed in:** e4b033fa (Task 3 commit)

**2. [Rule 1 - Bug] Plan's per-row Upsert*Members signature didn't match call site needs**
- **Found during:** Task 4 (Implement persist.go)
- **Issue:** The plan's `interfaces` block declared `UpsertClusterMembers(ctx, projection, gv, clusters []Cluster)` — passing the original `Cluster` slice and re-iterating it inside the store package. That would force the store package to import the cluster package, creating a cycle (cluster already imports store).
- **Fix:** Changed `UpsertClusterMembers` to accept `[]store.ClusterMemberRow` (one row per member, flattened by `toMemberRows` in the cluster package). The store package never imports cluster types. The plan even acknowledged this risk in Task 3's "NOTE" paragraph and recommended the boundary-type approach; the implementation follows that guidance.
- **Files modified:** internal/semantic/store/overlay.go (UpsertClusterMembers signature), internal/semantic/cluster/persist.go (toMemberRows helper)
- **Verification:** `go list -deps ./internal/semantic/cluster/...` shows no cycle; all overlay_test + persist_test tests pass.
- **Committed in:** e4b033fa + 27c87fe9 (Tasks 3 & 4 — boundary types in 3, conversion helpers in 4)

---

**Total deviations:** 2 auto-fixed (1 Rule 3 blocking, 1 Rule 1 bug)
**Impact on plan:** Both deviations were schema-reality-vs-pseudocode mismatches, not architectural changes. The plan's intent (per-cluster row, per-member row, idempotent under (repo_id, graph_version, cluster_id) PK, full-rewrite per detection run via Delete→Upsert) is preserved exactly; only the column names and the Go signatures were tightened to match the v1 schema. Plan Task 3 explicitly anticipated the boundary-type fix in its NOTE block, so this isn't scope creep.

## Issues Encountered

None — `go vet ./...` and `go test ./internal/... ./cmd/...` were green out of the gate at Task 5 (no Task 5 commit was needed per the plan's "ONLY if fixes were needed" clause).

## Verification Results

- `go vet ./...` → exit 0 (only pre-existing tree-sitter Swift binding C-warning; not new)
- `go test ./internal/... -count=1 -timeout=300s` → all packages pass; cluster + store both green
- `go test ./internal/semantic/cluster/... -count=10 -run TestWeakComponents` → 10× same-process determinism multiplier passes
- `grep -c "func WeakComponents" internal/semantic/cluster/weak.go` → 1
- `grep -c "type Cluster" internal/semantic/cluster/weak.go` → 1
- All three goldens are 64-char sha256 hex strings
- Direct imports of cluster package: `sort` + `internal/semantic/graph` + (in persist.go) `context`, `fmt`, `internal/semantic/store` — no kernel, no duckdb-go
- `go list -deps ./internal/semantic/cluster/...` shows no `internal/kernel[^/]` and no `marcboeker` (duckdb-go) entries
- `grep -rn "RegisterTools\|AddTool" internal/semantic/cluster/` → empty (no MCP tool surface in v1)
- `grep -rn "sync.Map" internal/semantic/cluster/` → empty (T-62-04-T4: no second mutex map)
- `find internal/semantic/cluster -name "*.go" -exec grep -l "tree-sitter\|treesitter" {} \;` → empty (Phase 62 reads facts from store, no tree-sitter access)

## TDD Gate Compliance

- **RED commit:** `08515fd1` (`test(62-04): add failing weak-component determinism + hex-digest + edge-case tests`) — failing tests before any production code
- **GREEN commit:** `f357ed2e` (`feat(62-04): implement deterministic weak-component clustering (GRAPH-06)`)
- **Follow-up test commit:** `4ce353c4` (`test(62-04): pin golden cluster hex digests after engine GREEN`) — pins the hex digests captured by running the freshly-implemented algorithm
- No REFACTOR commit — the GREEN implementation is already minimal (sort + union-find + group-by-root, ~80 LOC of logic).

The hex-digest goldens carry an extra delay between RED and full GREEN: the placeholder goldens fail at RED time with a clear "still PLACEHOLDER" message, the algorithm lands first to make the digests computable, then the digests are pinned in a separate `test()` commit. This is intentional and is the standard hex-digest TDD ratchet (P01 PageRank uses the same shape).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- **GRAPH-06 closed.** The phase 62 requirements list contains GRAPH-06 (weak-component clustering algorithm + persistence). This plan satisfies that requirement and only that requirement.
- **Algorithm-only delivery confirmed.** Cluster MCP tools (`get_cluster_map`, `explain_cluster`) explicitly deferred to v1.10.x per the v1 roadmap (62-CONTEXT.md "Out of scope" §, lines 64 + 800-804). When they ship, they will read from `semantic_clusters` + `semantic_cluster_members` — both populated by `RunClusterDetection` today.
- **Wiring to live consumer NOT done.** `RunClusterDetection` is callable but is not yet invoked by any scheduler / RankScheduler / live handler in the daemon. Wiring it onto a trigger (e.g., post-`ApplyRepair` cluster pass, or a scheduled batch) is left to a downstream plan or to the v1.10.x MCP-tools work that will define the consumer cadence. The store-side adapters in `internal/daemon/rank_wiring.go` already declare `QueryEffectiveGraph` (currently a stub returning nil/nil/nil), so the seam is in place but the implementation is not.
- **Plan 62-05 unblocked** if it depends on cluster persistence; nothing in this plan blocks subsequent waves.

## Self-Check: PASSED

- All 8 created files exist (5 .go + 3 testdata).
- All 5 task commits exist in git log: 08515fd1, f357ed2e, 4ce353c4, e4b033fa, 27c87fe9.
- `go vet ./...` and `go test ./internal/...` both green.
- Required acceptance grep counts (UpsertClusters / UpsertClusterMembers / DeleteClustersForGraphVersion in overlay.go = 1 each; func WeakComponents = 1; type Cluster = 1; goldens 64 chars) all verified.

---
*Phase: 62-graph-engine-ranking-type-resolution*
*Completed: 2026-05-06*
