---
author: architect
responsible: architect
phase: 126
milestone: v2.9
status: planned
phase_type: implementation
hard_bar: true
security_relevant: false
design_fork: false
parent_artifacts:
  - .planning/milestones/v2.9-REQUIREMENTS.md
  - .planning/phases/125-intraprocedural-flow-summary-engine/125-CONTEXT.md
---

# Phase 126 — Arg→param DATA_FLOWS emission + read surface + reachability

## Goal (the contract Q this phase establishes)

Emit real `DATA_FLOWS` edges from Phase 125's summaries, prove they are a genuine
param→param flow signal (distinct from structural/clone similarity), prove they are
reachable from a `helix` verb, and prove multi-hop source→sink reachability over the
emitted edges. This phase **is the whole feature** — Phase 127 (propagation) collapsed
into it (red-team M5): each edge already spans one call hop, so reachability is a plain
graph walk, not a propagation engine.

**Hoare frame:** `{Phase 125 FlowSummaries + batch name→node index exist}` `dataFlowEdges`
`{for every case-1 forwarding through a resolved unique in-repo callee, a directed
DATA_FLOWS edge caller.param→callee.param exists; no fabricated edges; reachable end-to-end}`.

## Scope

- Batch pass `dataFlowEdges(nodes, calleeParams, nameToNode)` in `semantic_similarity_edges.go`.
- Callee-param resolution via **emit-order adjacency** (a per-file `funcNode→[paramNode]`
  index built during the per-file loop, collected batch-wide).
- Wire into `factsFromExtracted` batch emission alongside the sibling passes.
- Read-surface confirmation + distinctness/forwarding/reachability guards.

## Decisions locked (from v2.9 REQUIREMENTS — all red-team-folded)

- **D1b — NOT DEFINES:** callee params resolved by emit-order adjacency (consecutive
  `KindParameter` run after the function in `single.Symbols`), NOT the flat DEFINES
  container heuristic. **Plan-time gate:** verify emit-order for all 11 providers;
  graceful degradation (no edge) where it doesn't hold.
- **D2 — no migration:** binding = node identity (Dst = callee param node).
- **D4 — substrate = reference.call-derived call entries + batch name index**, NOT
  CALLS edges (no tree-sitter CALLS edge exists at HEAD).
- **D5 — anti-mis-bind:** bare callee name with >1 candidate ⇒ no edge.
- **D6 — non-positional:** receiver offset, variadic fan-in, kwargs by name; ambiguity ⇒ no edge.
- **D7 — bound by total-edge budget + dedup**, NOT the similarity per-node cap.
- **D8 — directed dedup** on `(SrcNodeID, DstNodeID)`.
- **D9 — EdgeID stamping** covered automatically by the existing dense stamp loop.

## Centerpiece symbols (ground at PLAN time)

- `factsFromExtracted` (`semantic_wiring.go`) — the per-file loop (DEFINES at :2218, the
  batch name index at :2326, sibling emission at :2399-2401) — where the callee-param
  index is built and `dataFlowEdges` is wired.
- `fingerprintedNode` (`semantic_similarity_edges.go:127`) — carry the FlowSummary.
- `similarToEdges`/`structuralTwinEdges`/`semanticallyRelatedEdges` — the pattern to mirror.
- `MapInternalKind` (`edge_kind_surface.go:66`) — already maps DATA_FLOWS (read surface).
- The buildFn EdgeID stamp (`semantic_wiring.go:1608`) — confirms D9.

## Anti-vacuity / guards (each mutation-confirmed)

1. **Forwarding guard:** tainted-arg-passed fixture emits the edge; **non-forwarded param
   does NOT** (revert-and-fail RED). [FLOW-03c]
2. **Distinctness guard:** a `DATA_FLOWS` edge exists where no `STRUCTURAL_TWIN`/`SIMILAR_TO`
   does on a shared fixture (flow ≠ shape ≠ clone). [FLOW-03b]
3. **Anti-mis-bind guard:** two same-named callees ⇒ neither call mis-binds (no edge). [FLOW-02c]
4. **Reachability guard:** in-repo `src→f(x)→g(x)→sink` reachable end-to-end; **break a hop
   ⇒ reachability breaks** (revert-and-fail). [FLOW-03d]

## Acceptance

1. `DATA_FLOWS` edges emitted on a real Go fixture (0 producers → ≥1), case-1 param→param.
2. Callee-param resolution verified emit-order-based across all 11 providers (plan-time gate).
3. `explain-symbol-deep` surfaces the edges (test against `edges_outgoing`/`edges_incoming`).
4. All four guards green and mutation-confirmed RED when broken.
5. External/unresolved/ambiguous callees ⇒ no fabricated edge.
6. `go build ./...` clean; affected packages green; `make vet` clean; zero new deps; no migration.

## Out of scope

- The dedicated `trace-data-flow` verb (deferred; FLOW-03e helper optional only if cheap).
- Variable-level / in-body-origin flow (deferred — milestone out-of-scope).
- Queryable arg/param position column (deferred to the trace verb).
