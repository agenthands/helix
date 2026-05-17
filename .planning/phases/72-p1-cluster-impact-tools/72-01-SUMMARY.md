---
phase: 72-p1-cluster-impact-tools
plan: "01"
subsystem: semantic-skill
tags: [cluster-tools, accessors, store-reads, codec, phase-72, wave-1]
dependency_graph:
  requires: [71-p1-single-symbol-read-tools]
  provides:
    - internal/skill/semantic/cluster_id.go
    - internal/skill/semantic/accessors.go (Phase 72-01 section)
    - internal/skill/semantic/skill.go (Phase 72 fields + setters + getters)
    - internal/semantic/store/effective_graph.go (QueryClusterSummaries, QueryClusterMembers, QueryNodePageRanks)
  affects:
    - internal/daemon/semantic_wiring.go (wiring site ready; adapter deferred)
tech_stack:
  added: []
  patterns:
    - narrow-interface accessor convention (Pattern F)
    - lock-guarded setter/getter pattern (Phase 71 mirror)
    - positional IN clause for SQL injection mitigation (T-72-01-02)
    - store-internal result types bridged to skill-layer row types
key_files:
  created:
    - internal/skill/semantic/cluster_id.go
    - internal/skill/semantic/cluster_id_test.go
  modified:
    - internal/skill/semantic/accessors.go
    - internal/skill/semantic/skill.go
    - internal/semantic/store/effective_graph.go
    - internal/semantic/store/effective_graph_test.go
decisions:
  - "cluster_id codec: decodedClusterID kept minimal (encode/decode only); checkClusterIDFresh deferred to wave-2 explain_cluster handler per plan D1 note to avoid circular type dependency on ExplainClusterResult"
  - "ImpactLookupAccessor wraps only ExpandFrom+Status (OQ-2: narrow interface, not full integ.SemanticLookup)"
  - "ClusterSummaryResult and ClusterMemberResult are store-internal types; skill-layer ClusterSummaryRow and ClusterMemberRow are declared separately in accessors.go"
  - "QueryNodePageRanks uses positional IN clause (T-72-01-02 mitigation)"
  - "Daemon wiring stubs deferred: Phase 71 setters are also unwired in production; adding nil wiring for Phase 72 would diverge from established pattern"
metrics:
  duration: "~25 minutes"
  completed: "2026-05-17"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 4
---

# Phase 72 Plan 01: cluster_id Codec + Accessors + Store Read Methods Summary

Wave-1 foundation for Phase 72 P1 cluster & impact tools. Delivers the opaque cluster_id codec, four narrow accessor interfaces, four new skill fields with lock-guarded setters/getters, and three new Store read methods with race-clean tests.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | cluster_id.go codec + accessors.go + skill.go wave-1 seam | 75cc746a (impl) / 1bbf448e (test) | cluster_id.go, cluster_id_test.go, accessors.go, skill.go |
| 2 | Store.QueryClusterSummaries + QueryClusterMembers + QueryNodePageRanks + tests | 1d58962e (impl) / ce6ebc7c (test) | effective_graph.go, effective_graph_test.go |

## What Was Built

### Task 1: cluster_id codec + accessor interfaces + skill fields

**cluster_id.go** — opaque composite token codec binding a cluster integer id to `(projection, graph_version)`:
- `encodeClusterID(projection, graphVersion, clusterIntID uint64) string` — `fmt.Sprintf("%s:%d:%d", ...)`
- `decodeClusterID(token string) (decodedClusterID, error)` — `strings.SplitN` with `serr.InvalidArgs` on malformed tokens
- `decodedClusterID` struct with `Projection string`, `GraphVersion uint64`, `ClusterIntID uint64`
- Note: `checkClusterIDFresh` deferred to wave-2 `tools_explain_cluster.go` per plan D1 note (avoids circular type dependency on `ExplainClusterResult`)

**accessors.go** — Phase 72-01 section with four narrow interfaces + two row types:
- `ClusterSummaryRow {ClusterIntID uint64; MemberCount int}` — skill-layer type
- `ClusterMemberRow {NodeID uint64; SymbolID string}` — skill-layer type
- `ClusterMapAccessor` — `QueryClusterSummaries(ctx, repoID, projection, graphVersion, topN) ([]ClusterSummaryRow, error)`
- `ClusterMemberAccessor` — `QueryClusterMembers(ctx, repoID, projection, graphVersion, clusterIntID, limit) ([]ClusterMemberRow, error)`
- `ClusterPageRankAccessor` — `QueryNodePageRanks(ctx, repoID, projection, graphVersion, nodeIDs) (map[uint64]float64, error)`
- `ImpactLookupAccessor` — `ExpandFrom(...)` + `Status(...)` (OQ-2: narrow, not full `integ.SemanticLookup`)

**skill.go** — Phase 72 additions:
- Four new struct fields: `clusterMap`, `clusterMember`, `clusterPageRank`, `impactLookup`
- Four `SetXxx` setters following Phase 71 lock-guarded pattern
- Four `getXxx` private getters

### Task 2: Store read methods

**effective_graph.go** — three new `*Store` read methods:
- `ClusterSummaryResult` + `ClusterMemberResult` store-internal row types
- `QueryClusterSummaries`: `SELECT cluster_id, CAST(score AS INTEGER) FROM semantic_clusters ORDER BY score DESC LIMIT ?`
- `QueryClusterMembers`: JOIN `semantic_symbols` via latest committed snapshot for `stable_key` resolution
- `QueryNodePageRanks`: positional IN clause (T-72-01-02 mitigation), returns `map[uint64]float64`
- All three: nil-store returns `(nil, nil)`; errors wrapped with `fmt.Errorf`

## Verification Results

```
go vet ./internal/skill/semantic/  — CLEAN
go vet ./internal/semantic/store/  — CLEAN
go test ./internal/skill/semantic/ -run TestClusterID -race -count=1  — PASS
go test ./internal/semantic/store/ -run TestQueryCluster -race -count=1  — PASS
go test ./internal/skill/semantic/ -race -count=1  — PASS (full suite)
go test ./internal/semantic/store/ -race -count=1  — PASS (full suite)
```

Interface counts:
- `grep "^type.*ClusterMapAccessor\|ClusterMemberAccessor\|ClusterPageRankAccessor\|ImpactLookupAccessor" accessors.go | wc -l` = 4
- `grep "^func.*SetClusterMap\|SetClusterMember\|SetClusterPageRank\|SetImpactLookup" skill.go | wc -l` = 4
- `grep "encodeClusterID\|decodeClusterID" cluster_id.go | wc -l` = 4

## Deviations from Plan

### Planned (no deviation)

None - plan executed as specified with one design clarification:

**Design clarification: checkClusterIDFresh placement**

The plan's action block notes: "Preferred: declare checkClusterIDFresh inline in tools_explain_cluster.go (wave 2) and keep cluster_id.go limited to encode/decode + decodedClusterID." This was followed exactly. `checkClusterIDFresh` is NOT in cluster_id.go because it would reference `ExplainClusterResult` (wave-2 type), creating a forward dependency.

**Deferred item: daemon wiring for Phase 72 setters**

The plan's optional task suggested adding wiring to `internal/daemon/semantic_wiring.go`. Investigation showed that Phase 71's setters (`SetSymbolByName`, `SetExtractorRun`, `SetClusterMembership`, `SetTypeChain`, `SetSymbolEdges`, `SetEdgeEvidence`) are also NOT wired in the current production daemon wiring — daemon wiring for P1 setters is deferred to a future adapter plan. Adding Phase 72 wiring stubs now would diverge from the established pattern without enabling any tests or E2E functionality. Deferred to `.planning/phases/72-p1-cluster-impact-tools/deferred-items.md`.

## Known Stubs

None. This plan creates foundation interfaces and read methods; no stubs flow to UI rendering.

## Threat Surface Scan

No new network endpoints, auth paths, or file access patterns introduced. All additions are internal read-only SQL methods and interface declarations. T-72-01-01 (token parsing) and T-72-01-02 (IN clause) mitigations are applied as required by the threat register.

## Self-Check: PASSED

- `internal/skill/semantic/cluster_id.go` — FOUND
- `internal/skill/semantic/cluster_id_test.go` — FOUND
- `internal/skill/semantic/accessors.go` — MODIFIED (Phase 72-01 section verified)
- `internal/skill/semantic/skill.go` — MODIFIED (Phase 72 fields + setters + getters verified)
- `internal/semantic/store/effective_graph.go` — MODIFIED (3 methods verified)
- `internal/semantic/store/effective_graph_test.go` — MODIFIED (9 new tests verified)
- Commits 1bbf448e, 75cc746a, ce6ebc7c, 1d58962e — all exist in git log
