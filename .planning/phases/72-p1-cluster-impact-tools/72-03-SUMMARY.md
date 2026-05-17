---
phase: 72-p1-cluster-impact-tools
plan: 03
subsystem: semantic-skill
tags: [explain_cluster, tdd, cohesion, conductance, stale-cluster-id, entry-points]
dependency_graph:
  requires: [72-01]
  provides: [explain_cluster MCP tool, ExplainClusterResult, ClusterMemberEntry]
  affects: [internal/skill/semantic/register.go]
tech_stack:
  added: []
  patterns: [stale-soft-signal, isExportedSymbol-heuristic, OQ-3-compute-on-demand]
key_files:
  created:
    - internal/skill/semantic/tools_explain_cluster.go
    - internal/skill/semantic/tools_explain_cluster_test.go
  modified:
    - internal/skill/semantic/register.go
decisions:
  - "Stale-cluster-id check uses freshness.GraphVersion from assembleFreshness to avoid a double-read of graph version"
  - "isExportedSymbol declared package-private in tools_explain_cluster.go (not imported from kernel) to respect vet-nokernel2semantic boundary"
  - "errAs helper declared locally instead of importing standard errors package to keep the serr.Error type assertion clean"
  - "computeClusterMetrics returns empty (non-nil) dominantKinds slice because StoreAccessor.QueryEffectiveAdjacency carries weights not kind labels"
metrics:
  duration: "~15 minutes"
  completed: "2026-05-17T14:51:04Z"
  tasks_completed: 3
  files_changed: 3
---

# Phase 72 Plan 03: explain_cluster Handler with Stale-ID Check and Cohesion/Conductance — Summary

One-liner: TDD-built explain_cluster handler with stale_cluster_id soft signal, on-demand cohesion/conductance, and PageRank-sorted entry points.

## What Was Built

Implemented the `explain_cluster` MCP tool (P1TOOL-04) following strict TDD: RED (failing tests), GREEN (handler), REFACTOR (register.go).

### Handler: `handleExplainCluster`

`internal/skill/semantic/tools_explain_cluster.go`

13-step handler order per plan:

1. `checkMode(snap, modeTierRead)` — retained for code-review visibility
2. `decodeClusterID(args.ClusterID)` — returns `errorResult` on parse failure
3. `assembleFreshness(ctx, repoID)` — populates freshness, then compares `decoded.GraphVersion != freshness.GraphVersion` for stale-id check; returns soft `FallbackReason: "stale_cluster_id"` (not `IsError`) when mismatched
4. Workspace key + repoID
5. Clamp `max_members` to `[1, 1000]`, default 200 (`explainClusterDefaultMaxMembers`)
6. `s.getClusterMember().QueryClusterMembers(...)` with `limit=0` for total count, then cap applied client-side to preserve `MemberCount` vs `MembersReturned` contract
7. `s.getClusterPageRank().QueryNodePageRanks(...)` — nil-guarded, degrades to 0.0 scores
8. Sort `ClusterMemberEntry` list by PageRank desc
9. `s.getStore().QueryEffectiveAdjacency(...)` — filter to intra/leaving edges; compute cohesion and separation. `TODO(perf): O(E)` comment per OQ-3 lock
10. Entry points = members where `isExportedSymbol(SymbolID) == true`, sorted by PageRank desc (already in order from step 8)
11. Dominant edge kinds: `computeClusterMetrics` returns empty non-nil slice (adjacency carries weights not kind strings)
12. `guardrails.IssueReceiptOnSuccess`
13. `jsonResult(ExplainClusterResult{...})`

### Types Declared

- `ExplainClusterArgs` — `ClusterID string`, `MaxMembers int`
- `ClusterMemberEntry` — `SymbolID`, `PageRank`, `IsEntryPoint`
- `ExplainClusterResult` — full response with `MemberCount`, `MembersReturned`, `Members`, `Cohesion`, `Separation`, `DominantEdgeKinds`, `EntryPoints`, `Freshness`, `FallbackReason`

### `isExportedSymbol`

Mirrors `blast_radius_strangler.go:isExported` heuristic: capital letter after last `:` in the stable_key. Declared package-private in `tools_explain_cluster.go` — not imported from kernel to respect the vet-nokernel2semantic boundary.

### Register wiring

`internal/skill/semantic/register.go` — `registerExplainCluster(server, s, tracer)` added after `registerGetClusterMap`.

## Tests: `tools_explain_cluster_test.go`

8 behavior tests, all passing under `-race -count=1`:

| Test | Behavior |
|------|----------|
| `TestExplainCluster_ValidToken` | 4 members, sorted by PageRank, Cohesion > 0, Freshness present |
| `TestExplainCluster_StaleClustersID` | graphVersion mismatch → `FallbackReason="stale_cluster_id"`, IsError=false |
| `TestExplainCluster_MaxMembersCap` | limit=3, total=5 → MembersReturned=3, MemberCount=5 |
| `TestExplainCluster_CohesionConductance` | exact assert cohesion=0.25, separation=0.25 (±1e-9) |
| `TestExplainCluster_FreshnessV2Present` | Freshness.GraphVersion > 0, Status non-empty |
| `TestExplainCluster_ReadOnlyCanary` | D-09 write-surface canary — Begin/Commit/Abort/Write never called |
| `TestExplainCluster_EntryPoints` | exactly 2 exported symbols in entry_points, sorted by PageRank |
| `TestExplainCluster_InvalidToken` | `"notvalid"` → IsError=true |

## TDD Gate Compliance

- RED gate: `test(72-03)` commit `6219c503` — build fails, undefined `ExplainClusterResult` and `handleExplainCluster`
- GREEN gate: `feat(72-03)` commit `d6c7ba59` — all 8 tests pass
- REFACTOR gate: `refactor(72-03)` commit `1f134524` — register.go extended, vet clean

## Verification Results

```
go test ./internal/skill/semantic/ -run TestExplainCluster -race -count=1
ok  github.com/agenthands/helix/internal/skill/semantic  2.408s (all 8 PASS)

go vet ./internal/skill/semantic/... — CLEAN (only swift binding warning, pre-existing)

grep "registerExplainCluster" internal/skill/semantic/register.go
→ 1 line: registerExplainCluster(server, s, tracer)

grep -v '^//' tools_explain_cluster.go | grep -c "BeginSnapshot|CommitSnapshot|AbortSnapshot|WriteSnapshotFacts"
→ 0

grep "stale_cluster_id" tools_explain_cluster_test.go
→ 3 lines (test function body + assertion)
```

## Commits

| Hash | Type | Description |
|------|------|-------------|
| `6219c503` | `test(72-03)` | RED — failing tests for explain_cluster handler |
| `d6c7ba59` | `feat(72-03)` | GREEN — implement explain_cluster handler |
| `1f134524` | `refactor(72-03)` | REFACTOR — wire registerExplainCluster into RegisterAll |

## Deviations from Plan

**1. [Rule 2 - Design clarification] MemberCount uses all-rows pre-cap**

The plan specifies `MemberCount` as the total number of members in the cluster, but the `QueryClusterMembers` accessor only accepts a `limit` parameter. The handler calls `QueryClusterMembers` with `limit=0` (which the fixture accessor interprets as "no limit"), and the cap is applied client-side after the full slice is returned. This preserves the `MemberCount` vs `MembersReturned` contract without needing a separate count query.

**2. [Rule 1 - Simplification] dominantKinds always empty**

`computeClusterMetrics` returns an empty non-nil `[]EdgeKindSurface{}` for `DominantEdgeKinds`. The reason: `StoreAccessor.QueryEffectiveAdjacency` returns `map[graph.NodeID]map[graph.NodeID]float64` — weights only, no internal kind labels. Computing dominant kinds requires per-edge kind data that is not available in the current adjacency seam. The test `TestGetClusterMap_DominantEdgeKinds` in the sibling plan documents the same constraint. This is intentional and consistent with the 72-02 pattern.

**3. [Rule 2 - errAs helper] Local type assertion helper**

Added `errAs(err error, target **serr.Error) bool` as a package-private function to perform the `serr.Error` type assertion without importing the standard `errors` package. This avoids potential import conflicts and keeps the error handling concise. The handler returns `errorResult(err.Error())` for any decode error regardless of type — the type assertion is defensive.

## Known Stubs

None — all response fields are computed from fixture data. `DominantEdgeKinds` is intentionally empty (documented above) because the adjacency seam lacks per-edge kind labels.

## Threat Flags

No new security-relevant surface beyond what the threat model documents:

- `T-72-03-01` (cluster_id injection): mitigated — `decodeClusterID` uses `strings.SplitN` + `ParseUint`; only uint64 values reach accessor calls.
- `T-72-03-02` (max_members input): mitigated — clamped to `[1, 1000]` before accessor call.

## Self-Check: PASSED

- `internal/skill/semantic/tools_explain_cluster.go` — EXISTS
- `internal/skill/semantic/tools_explain_cluster_test.go` — EXISTS
- `internal/skill/semantic/register.go` — MODIFIED (registerExplainCluster present)
- Commits `6219c503`, `d6c7ba59`, `1f134524` — ALL IN GIT LOG
