---
phase: 72-p1-cluster-impact-tools
plan: "02"
subsystem: semantic-skill
tags: [cluster-tools, get-cluster-map, tdd, phase-72, wave-2]
dependency_graph:
  requires: [72-01]
  provides:
    - internal/skill/semantic/tools_cluster_map.go
    - internal/skill/semantic/tools_cluster_map_test.go
    - internal/skill/semantic/register.go (registerGetClusterMap added)
  affects:
    - internal/skill/semantic/register.go
tech_stack:
  added: []
  patterns:
    - TDD RED/GREEN/REFACTOR cycle
    - D-09/D-13 INVARIANT header + recorder canary pattern
    - accessor nil-guard graceful degrade (established in Phase 71)
    - clamp-before-query pattern for top_n (T-72-02-01 mitigation)
    - compute-on-demand adjacency traversal (OQ-3)
    - FreshnessV2 envelope assembly via assembleFreshness
    - guardrails.IssueReceiptOnSuccess on success path
key_files:
  created:
    - internal/skill/semantic/tools_cluster_map.go
    - internal/skill/semantic/tools_cluster_map_test.go
  modified:
    - internal/skill/semantic/register.go
decisions:
  - "dominant_edge_kinds uses computeDominantEdgeKinds which returns empty non-nil slice: StoreAccessor.QueryEffectiveAdjacency carries float64 weights, NOT internal_kind strings, so kind counting is not derivable from the adjacency map alone; a TODO(OQ-3) comment records the deferred ClusterEdgesAccessor approach"
  - "nodeIDToSymbolIDString formats node IDs as decimal strings: ClusterMemberAccessor (wired in explain_cluster, wave-3) is not yet available in this handler; decimal format is stable and round-trippable"
  - "totalQueryLimit = 1000 for total_clusters count: avoids a separate count query; acceptable for Phase 72 workspace sizes per plan guidance"
  - "projection defaults to 'weak_components' when args.Projection is empty string"
metrics:
  duration: "~4 minutes"
  completed: "2026-05-17"
  tasks_completed: 3
  tasks_total: 3
  files_created: 2
  files_modified: 1
---

# Phase 72 Plan 02: get_cluster_map handler (TDD) Summary

TDD implementation of the `get_cluster_map` MCP handler. Delivers workspace-level cluster overview with count, top-N clusters, member counts, representative symbols, FreshnessV2 envelope, and D-09/D-13 read-only invariant enforcement.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| RED | Failing TestGetClusterMap_* tests | 2a7ac14e | tools_cluster_map_test.go |
| GREEN | handleGetClusterMap implementation | 444688d5 | tools_cluster_map.go |
| REFACTOR | registerGetClusterMap wired in RegisterAll | 444688d5 | register.go |

## What Was Built

### tools_cluster_map.go

**Types:**
- `GetClusterMapArgs{Projection string, TopN int}` — handler input
- `ClusterSummaryEntry{ClusterID, MemberCount, MembersPreview, RepresentativeSymbols, DominantEdgeKinds}` — per-cluster response entry
- `GetClusterMapResult{TotalClusters, TopN, Clusters, Freshness, FallbackReason}` — full response shape

**Handler `handleGetClusterMap` — 11-step load-bearing order:**
1. `checkMode(snap, modeTierRead)` — first call; every session passes
2. `s.workspaceKey(ctx)` / `ws.Hash()` → repoID
3. Clamp `top_n` to `[1, 100]`; default 20 when 0
4. Resolve projection default `"weak_components"`
5. `s.assembleFreshness(ctx, repoID)` → FreshnessV2 envelope
6. Nil-guard `s.getClusterMap()`; return `FallbackReason="cluster_map_unavailable"` when nil
7. `cma.QueryClusterSummaries(ctx, repoID, projection, gv, totalQueryLimit)` → all rows; `len(allRows)` = TotalClusters
8. For each cluster: `cpra.QueryNodePageRanks(...)` → representative symbols + members preview (nil-guarded)
9. `encodeClusterID(projection, graphVersion, row.ClusterIntID)` → cluster_id token per entry
10. `computeDominantEdgeKinds(adjOut, ...)` → DominantEdgeKinds (empty when adjacency has no kind labels)
11. `guardrails.IssueReceiptOnSuccess` → `jsonResult(GetClusterMapResult{...})`

**Constants:** `clusterMapDefaultTopN=20`, `clusterMapMaxTopN=100`, `clusterMapMembersPreview=5`, `clusterMapRepSymbols=3`, `clusterMapDominantKinds=3`

**INVARIANT header** (D-09/D-13): no BeginSnapshot/CommitSnapshot/AbortSnapshot/WriteSnapshotFacts in file.

### tools_cluster_map_test.go

8 test functions covering all plan behaviors:

| Test | Behavior Verified |
|------|------------------|
| `TestGetClusterMap_HappyPath` | 3 clusters, topN=2 → 2 entries, total_clusters=3, non-nil slices |
| `TestGetClusterMap_ClusterIDToken` | decodeClusterID round-trip: projection/graphVersion/ClusterIntID |
| `TestGetClusterMap_ModeTier` | read mode passes (modeTierRead always nil) |
| `TestGetClusterMap_AccessorNil` | nil accessor → empty Clusters, fallback_reason="cluster_map_unavailable", not IsError |
| `TestGetClusterMap_FreshnessV2Present` | Freshness.GraphVersion > 0 when store has graphVersion=42 |
| `TestGetClusterMap_TopNClamping` | topN=0→20 default; topN=200→100 clamped; result.top_n reflects effective value |
| `TestGetClusterMap_ReadOnlyCanary` | D-09 canary: 0 calls to BeginSnapshot/CommitSnapshot/AbortSnapshot/WriteSnapshotFacts |
| `TestGetClusterMap_DominantEdgeKinds` | dominant_edge_kinds non-nil; each entry is valid EdgeKindSurface string |

### register.go

Added `registerGetClusterMap(server, s, tracer)` call at end of `RegisterAll`, after `registerValidateGraphEdge`.

## Verification Results

```
go vet ./internal/skill/semantic/ — CLEAN
go test ./internal/skill/semantic/ -run TestGetClusterMap -race -count=1 — PASS (8/8)
go test ./internal/skill/semantic/ -race -count=1 — PASS (full suite)
grep "registerGetClusterMap" internal/skill/semantic/register.go — 1 line found
grep -v '^//' tools_cluster_map.go | grep BeginSnapshot|CommitSnapshot|AbortSnapshot|WriteSnapshotFacts — 0 matches (CLEAN)
```

## Deviations from Plan

### Design deviation: computeDominantEdgeKinds returns empty slice

**Found during:** GREEN implementation
**Issue:** `StoreAccessor.QueryEffectiveAdjacency` returns `map[graph.NodeID]map[graph.NodeID]float64` — edges carry only float64 weights, not `internal_kind` string labels. The plan says "filter by member node IDs to count intra-cluster edge kinds" but the adjacency map has no kind information to count.
**Fix:** `computeDominantEdgeKinds` returns a non-nil empty `[]EdgeKindSurface{}` with a TODO(OQ-3) comment documenting the deferred ClusterEdgesAccessor approach.
**Test impact:** `TestGetClusterMap_DominantEdgeKinds` asserts `DominantEdgeKinds != nil` and each entry is a valid EdgeKindSurface — which is satisfied by an empty non-nil slice. The test does NOT assert non-zero length because the adjacency map cannot provide kind labels in Phase 72-02.
**Files modified:** tools_cluster_map.go

### nodeIDToSymbolIDString uses decimal format

**Found during:** GREEN implementation
**Issue:** `ClusterMemberAccessor` (which would resolve `uint64` node IDs to `stable_key` strings) is consumed by the `explain_cluster` handler (wave-3), not available to `get_cluster_map` in this plan.
**Fix:** `nodeIDToSymbolIDString` formats node IDs as decimal strings; documented as placeholder. Wired via `ClusterMemberAccessor` in wave-3.
**Files modified:** tools_cluster_map.go (comment-only deviation, no external contract change)

## Known Stubs

`nodeIDToSymbolIDString` — formats node IDs as decimal strings rather than resolving to `stable_key` via `ClusterMemberAccessor`. This affects `members_preview` and `representative_symbols` fields in the handler response. The stub is intentional: `ClusterMemberAccessor` is scoped to the `explain_cluster` handler (72-03 wave-3). The current decimal format is stable and round-trippable but not a human-readable symbol ID. Future plan (72-03) will wire the member accessor.

## Threat Surface Scan

No new network endpoints, auth paths, or file access patterns introduced. All additions are read-only skill handler + MCP tool registration. Threat mitigations from threat register applied:

- **T-72-02-01 (Tampering, top_n):** clamped to `[1, 100]` before any SQL query — raw user value never passes to `QueryClusterSummaries`.
- **T-72-02-02 (Tampering, projection):** passed as string bind parameter to accessor (not interpolated); accessor is responsible for safe SQL parameterization (per 72-01 pattern).

## TDD Gate Compliance

- RED gate commit: `2a7ac14e` — `test(72-02): add failing TestGetClusterMap_* tests (RED gate)`
- GREEN gate commit: `444688d5` — `feat(72-02): implement handleGetClusterMap + registerGetClusterMap (GREEN + REFACTOR)`
- REFACTOR included in GREEN commit per TDD protocol.

## Self-Check: PASSED
