---
phase: 72-p1-cluster-impact-tools
plan: "04"
subsystem: semantic-skill
tags: [mcp-tool, review-tier, impact-graph, tdd, confidence-cap]
dependency_graph:
  requires: [72-01, 71-01, 71-03, 71-05]
  provides: [get_change_impact_graph]
  affects: [internal/skill/semantic/register.go, internal/skill/semantic/readonly_gate_test.go]
tech_stack:
  added: []
  patterns: [review-plus-mode-enforcement, capEdgeConfidences-local-helper, OQ-1-confidence-cap, surfaceToInternalKinds-filter]
key_files:
  created:
    - internal/skill/semantic/tools_change_impact.go
    - internal/skill/semantic/tools_change_impact_test.go
  modified:
    - internal/skill/semantic/register.go
    - internal/skill/semantic/readonly_gate_test.go
decisions:
  - "capEdgeConfidences is a local 5-LOC helper in tools_change_impact.go (NOT imported from kernel per vet-nokernel2semantic boundary)"
  - "OQ-1 predicate: any impact.Confidence < 0.8 triggers confidence_cap{value:0.6, reason:'type_resolver_tier_3'}"
  - "ReachedDepth = clampedMaxDepth (Pitfall 4: ExpandFrom does not expose per-impact depth)"
  - "NodesCount + EdgesCount count pre-cap totals including edge-kind filter"
  - "gatedHandlerFiles in readonly_gate_test.go extended to include all 3 Phase 72 handler files"
metrics:
  duration: "~15 minutes"
  completed: "2026-05-17"
  tasks_completed: 3
  files_changed: 4
---

# Phase 72 Plan 04: get_change_impact_graph Handler — Summary

## One-liner

`get_change_impact_graph` review+ handler: BFS subgraph (nodes + edges) from seed symbol with node/edge caps, EdgeKinds filter, and OQ-1 confidence cap (type_resolver_tier_3).

## What Was Built

### tools_change_impact.go (new)

Handler `handleGetChangeImpactGraph` implementing the 14-step load-bearing order:

1. `checkMode(snap, modeTierReview)` — FIRST call; rejects mode < "review" with PermissionDenied
2. Workspace key + repoID
3. `resolveSeed(ctx, ws, args.Seed)` — short-circuit with FallbackReason="symbol_not_found" on not_found
4. Depth clamp: [1, 5], default 2
5. ImpactLookupAccessor nil-guard → FallbackReason="impact_lookup_unavailable"
6. `lookup.ExpandFrom(ctx, ws, sym, clampedDepth)` → `[]integ.Impact`
7. EdgeKinds filter via `surfaceToInternalKinds(EdgeKindSurface(k))` (same package, Pitfall 5 safe)
8. Pre-cap totals: NodesCount = len(impacts), EdgesCount = sum of filtered edges
9. Node cap (200) + edge cap (500); Truncated=true when exceeded
10. OQ-1 confidence cap: if any `impact.Confidence < 0.8` → `capEdgeConfidences(rawEdges, 0.6)` + set `ConfidenceCap{Value:0.6, Reason:"type_resolver_tier_3"}`
11. Assemble ImpactNode + ImpactEdge slices; `MapInternalKind(e.Kind)` for EdgeKind field
12. `assembleFreshness(ctx, repoID)` for FreshnessV2 envelope
13. `guardrails.IssueReceiptOnSuccess`
14. `jsonResult(GetChangeImpactGraphResult{...})`

Types declared: `GetChangeImpactGraphArgs`, `ImpactNode`, `ImpactEdge`, `ConfidenceCap`, `GetChangeImpactGraphResult`.

Local helper `capEdgeConfidences(edges []ImpactEdge, cap float64)` — 5 LOC, mutates in-place. NOT imported from `internal/kernel/symbols` (vet-nokernel2semantic boundary enforced).

INVARIANT (D-09 / D-13) header present. `var getChangeImpactGraphHelp` declared for tool registry.

### tools_change_impact_test.go (new)

8 test functions covering all plan behaviors:

- `TestGetChangeImpactGraph_HappyPath` — 3 impacts × 2 edges → Nodes=3, Edges=6, Truncated=false, valid EdgeKind
- `TestGetChangeImpactGraph_ReviewTierEnforced` — mode="read" → IsError=true; mode="review" → success
- `TestGetChangeImpactGraph_ReadOnlyCanary` — D-09 recorder canary; no write methods called
- `TestGetChangeImpactGraph_DepthClamping` — MaxDepth=0→2, MaxDepth=10→5, negative→2
- `TestGetChangeImpactGraph_NodeCap` — 250 impacts → Truncated=true, NodesCount=250, len(Nodes)=200
- `TestGetChangeImpactGraph_EdgeKindsFilter` — "calls" filter removes IMPORTS edges
- `TestGetChangeImpactGraph_ConfidenceCap` — impact.Confidence=0.5 → ConfidenceCap set, all edge Confidence ≤ 0.6
- `TestGetChangeImpactGraph_SeedNotFound` — not_found → FallbackReason="symbol_not_found", IsError=false

### register.go (modified)

Added `registerGetChangeImpactGraph(server, s, tracer)` call after `registerExplainCluster`.

### readonly_gate_test.go (modified)

Extended `gatedHandlerFiles` to include all 3 Phase 72 handler files:
- `tools_cluster_map.go` (Phase 72 addition)
- `tools_explain_cluster.go` (Phase 72 addition)
- `tools_change_impact.go` (Phase 72 addition)

## TDD Gate Compliance

- RED: `test(72-04): add failing tests for get_change_impact_graph (RED gate)` — commit `85052edf` — tests failed with build errors (handleGetChangeImpactGraph undefined)
- GREEN: `feat(72-04): implement handleGetChangeImpactGraph (GREEN)` — commit `819364e9` — all 8 tests pass
- REFACTOR: `refactor(72-04): wire registerGetChangeImpactGraph into RegisterAll` — commit `136c7dae`

## Verification Results

```
go test ./internal/skill/semantic/ -run TestGetChangeImpactGraph -race -count=1  → PASS (8 tests)
go test ./internal/skill/semantic/ -race -count=1                                → PASS (full suite, ~5.8s)
go vet ./internal/skill/semantic/                                                 → clean
grep "registerGetChangeImpactGraph" register.go                                  → 1 line
grep -v '^//' tools_change_impact.go | grep -c "BeginSnapshot|..."               → 0
grep "modeTierReview" tools_change_impact.go                                     → 1 line
```

## Deviations from Plan

None. Plan executed exactly as written.

- `capEdgeConfidences` is local to `tools_change_impact.go` (no import of `internal/kernel/symbols`)
- `gatedHandlerFiles` was extended with all 3 Phase 72 handler files (Pitfall 6 avoided)
- `ReachedDepth = clampedMaxDepth` with Pitfall 4 comment (ExpandFrom does not expose per-impact depth)
- `NodesCount` counts all impacts before node-cap; `EdgesCount` counts filtered edges before edge-cap

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes. Handler is read-only (review+ mode enforced as first call). No new threat surface beyond the plan's STRIDE register T-72-04-01/02/03.

## Self-Check: PASSED

- `internal/skill/semantic/tools_change_impact.go` — FOUND
- `internal/skill/semantic/tools_change_impact_test.go` — FOUND
- `internal/skill/semantic/register.go` (registerGetChangeImpactGraph call) — FOUND
- Commit `85052edf` (RED) — FOUND
- Commit `819364e9` (GREEN) — FOUND
- Commit `136c7dae` (REFACTOR) — FOUND
