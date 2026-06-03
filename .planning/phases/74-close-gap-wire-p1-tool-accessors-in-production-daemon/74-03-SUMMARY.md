---
phase: 74-close-gap-wire-p1-tool-accessors-in-production-daemon
plan: "03"
subsystem: daemon
tags: [semantic, accessor, adapter, store, daemon, P1, D-01a, FOLD, SymbolEdges, ClusterMembership]

requires:
  - phase: 74-02
    provides: Five D-01 store-backed adapter structs (semP1SymbolByNameAdapter, semP1ExtractorRunAdapter, semP1ClusterMapAdapter, semP1ClusterMemberAdapter, semP1ClusterPageRankAdapter) in semantic_wiring.go
  - phase: 74-01
    provides: WiredAccessorsForTest seam in export_p1_test.go
  - phase: 69-production-status-accessors
    provides: semStoreAdapter nil-guard + delegate pattern (direct analog)

provides:
  - semP1SymbolEdgesAdapter: satisfies SymbolEdgesAccessor with CallersOf/IncomingEdgesOf/OutgoingEdgesOf backed by QuerySymbolEdgesIncoming/Outgoing store helpers
  - semP1ClusterMembershipAdapter: satisfies ClusterMembershipAccessor with ClusterIDOf backed by CurrentGraphVersion + QueryNodeIDByStableKey + QueryClusterIDOfNode
  - Store helpers: QuerySymbolEdgesIncoming, QuerySymbolEdgesOutgoing, QueryClusterIDOfNode (all three in effective_graph.go)
  - SymbolEdgeRaw exported type for cross-package use
  - Compile-time interface guards for both new adapters
  - Closes BLOCKER-1 D-01a FOLD: all two-hop adapters wired

affects:
  - 74-04 (setters block extension — symbolEdgesAccessor + clusterMembershipAccessor called there)
  - 74-05 (bootstrap test validates both new accessors are non-nil after wiring)

tech-stack:
  added: []
  patterns:
    - "Two-hop adapter: stable_key → node_id via QueryNodeIDByStableKey, then SQL on semantic_edges/semantic_cluster_members"
    - "resolveSymbolEdgesNodeID helper: shared first-hop inside semP1SymbolEdgesAdapter (avoids repeating LatestCommittedSnapshot + QueryNodeIDByStableKey in all three methods)"
    - "assembleEdgeRows helper: shared stable_key resolution for all three SymbolEdges directions"
    - "callsOnly bool parameter: controls edge_kind='CALLS' filter in QuerySymbolEdgesIncoming (single method covers both CallersOf and IncomingEdgesOf)"
    - "Pitfall 1 enforced: JOIN via semantic_symbols.symbol_id = semantic_edges.src_node_id (not node_id)"
    - "Pitfall 2 enforced: CallersOf passes callsOnly=true; IncomingEdgesOf passes false"
    - "Pitfall 3 enforced: ClusterIDOf calls CurrentGraphVersion internally"

key-files:
  created: []
  modified:
    - internal/daemon/semantic_wiring.go
    - internal/semantic/store/effective_graph.go

key-decisions:
  - "SymbolEdgeRaw exported (was symbolEdgeRaw unexported) — required for daemon-package adapters to iterate results from effective_graph.go helper methods"
  - "resolveSymbolEdgesNodeID + assembleEdgeRows helpers extracted inside semP1SymbolEdgesAdapter to avoid three copies of the same two-hop boilerplate"
  - "callsOnly bool in QuerySymbolEdgesIncoming merges CallersOf and IncomingEdgesOf into one store method"
  - "All SQL uses positional parameters — no string interpolation (Threats T-74-03-01, T-74-03-02)"
  - "Both adapters placed in semantic_wiring.go per D-04 single-file invariant"

requirements-completed: [AUDIT-R-01]

duration: 15min
completed: 2026-06-03
---

# Phase 74 Plan 03: D-01a FOLD Two-Hop Accessor Adapters Summary

**semP1SymbolEdgesAdapter (SymbolEdgesAccessor) and semP1ClusterMembershipAdapter (ClusterMembershipAccessor) added to semantic_wiring.go with supporting Store helper methods in effective_graph.go, closing BLOCKER-1 for the D-01a FOLD accessors.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-06-03T13:40:00Z
- **Completed:** 2026-06-03T13:51:49Z
- **Tasks:** 2 (committed together — same two files modified)
- **Files modified:** 2

## Accomplishments

- Added three Store helpers to `effective_graph.go`:
  - `QuerySymbolEdgesIncoming(ctx, snapshotID, dstNodeID, callsOnly)` — incoming edges with optional CALLS filter; satisfies Pitfall 2
  - `QuerySymbolEdgesOutgoing(ctx, snapshotID, srcNodeID)` — outgoing edges, no edge_kind filter
  - `QueryClusterIDOfNode(ctx, repoID, graphVersion, nodeID)` — cluster lookup via semantic_cluster_members JOIN semantic_clusters
- Exported `SymbolEdgeRaw` (was `symbolEdgeRaw`) — required for daemon-package adapter iteration
- Added `semP1SymbolEdgesAdapter` to `semantic_wiring.go`:
  - `CallersOf`: callsOnly=true → CALLS-filtered incoming (Pitfall 2 satisfied)
  - `IncomingEdgesOf`: callsOnly=false → all incoming edge kinds
  - `OutgoingEdgesOf`: src_node_id filter via QuerySymbolEdgesOutgoing
  - Shared `resolveSymbolEdgesNodeID` + `assembleEdgeRows` helpers eliminate duplication
- Added `semP1ClusterMembershipAdapter` to `semantic_wiring.go`:
  - `ClusterIDOf`: CurrentGraphVersion + QueryNodeIDByStableKey + QueryClusterIDOfNode (Pitfall 3 satisfied — graph_version resolved internally)
- Factory constructors: `symbolEdgesAccessor()` and `clusterMembershipAccessor()` on semanticBundle
- Compile-time interface guards for both adapters appended to the var block

## Task Commits

1. **Tasks 1+2: Add semP1SymbolEdgesAdapter + semP1ClusterMembershipAdapter (D-01a FOLD)** — `8de80475` (feat)

## Files Created/Modified

- `internal/daemon/semantic_wiring.go` — two new P1 adapter structs, two factory constructors, compile-time guards added after existing P1 section
- `internal/semantic/store/effective_graph.go` — SymbolEdgeRaw type exported; QuerySymbolEdgesIncoming, QuerySymbolEdgesOutgoing, QueryClusterIDOfNode methods appended

## Decisions Made

- SymbolEdgeRaw exported: cross-package visibility is required for daemon-level iteration; the type is simple and safe to export
- Single `callsOnly bool` parameter in QuerySymbolEdgesIncoming consolidates CallersOf/IncomingEdgesOf into one query path
- All adapters placed in semantic_wiring.go per D-04 single-file invariant (no separate files)
- Store helpers placed in effective_graph.go (same file as QueryNodeIDByStableKey, the building block they consume)

## Deviations from Plan

**1. [Rule 1 - Bug] Exported SymbolEdgeRaw type (was symbolEdgeRaw unexported)**
- **Found during:** Task 1 build — `undefined: semanticstore.SymbolEdgeRaw` compile error
- **Issue:** Plan referenced the type as if it could be used across packages, but it was unexported. Daemon-package adapters cannot access unexported types from the store package.
- **Fix:** Renamed `symbolEdgeRaw` → `SymbolEdgeRaw` (exported). No callers outside effective_graph.go existed at the time; no breakage.
- **Files modified:** `internal/semantic/store/effective_graph.go`
- **Commit:** `8de80475`

## Issues Encountered

One build error on first attempt (unexported type), fixed inline per Rule 1. Build and all vet gates passed on second attempt.

## Threat Model Coverage

| Threat ID | Mitigation Status |
|-----------|-------------------|
| T-74-03-01 | COVERED: All WHERE parameters in QuerySymbolEdgesIncoming/Outgoing are positional; edge_kind='CALLS' is a string literal, not user input |
| T-74-03-02 | COVERED: repoID, graphVersion, nodeID all positional in QueryClusterIDOfNode; LIMIT 1 prevents row explosion |
| T-74-03-03 | COVERED: JOIN on semantic_symbols.symbol_id = semantic_edges.src_node_id documented in comment (Pitfall 1); confirmed by build |
| T-74-03-SC | ACCEPTED: no new external packages added |

## Self-Check

- [x] `internal/daemon/semantic_wiring.go` modified and committed at 8de80475
- [x] `internal/semantic/store/effective_graph.go` modified and committed at 8de80475
- [x] `go build ./internal/daemon/... ./internal/semantic/store/...` exits 0
- [x] `go vet -tags vet-noduckdb ./internal/daemon/... ./internal/semantic/store/...` exits 0
- [x] `go vet -tags vet-nokernel2semantic ./internal/daemon/... ./internal/semantic/store/...` exits 0
- [x] `go test -race -count=1 ./internal/daemon/... ./internal/skill/semantic/...` exits 0
- [x] `grep -c "semP1SymbolEdgesAdapter" internal/daemon/semantic_wiring.go` = 10 (>= 2)
- [x] `grep -c "symbolEdgesAccessor" internal/daemon/semantic_wiring.go` = 1 (>= 1)
- [x] `grep -c "semP1ClusterMembershipAdapter" internal/daemon/semantic_wiring.go` = 6 (>= 2)
- [x] `grep -c "clusterMembershipAccessor" internal/daemon/semantic_wiring.go` = 1 (>= 1)
- [x] CallersOf passes callsOnly=true confirming CALLS filter (A-11: callers direction)

## Self-Check: PASSED

## Known Stubs

None — both adapters are fully implemented with real SQL against the live schema.

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries. Store helpers are read-only SELECT queries on existing schema tables.

## Next Phase Readiness

- `symbolEdgesAccessor()` and `clusterMembershipAccessor()` factory constructors ready to be called by Plan 74-04 setters block extension
- `SetSymbolEdges` and `SetClusterMembership` calls can now be added to the `if b.skill != nil` block
- Plan 74-05 bootstrap test can assert `wired.SymbolEdges == true` and `wired.ClusterMembership == true` after wiring

---
*Phase: 74-close-gap-wire-p1-tool-accessors-in-production-daemon*
*Completed: 2026-06-03*
