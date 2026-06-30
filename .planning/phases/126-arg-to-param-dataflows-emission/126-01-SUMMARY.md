---
author: architect
responsible: architect
phase: 126
milestone: v2.9
status: complete
plan: 126-01
parent_artifacts:
  - .planning/phases/126-arg-to-param-dataflows-emission/126-01-PLAN.md
  - .planning/phases/125-intraprocedural-flow-summary-engine/125-01-SUMMARY.md
---

# SUMMARY 126-01 — Arg→param DATA_FLOWS emission + read surface + reachability

## Outcome: DELIVERED (the whole feature)

`DATA_FLOWS` now has a real producer: directed caller.param → callee.param
interprocedural flow edges, emitted from Phase 125's FlowSummaries. Phase 127
(propagation) collapsed into this phase — reachability is a plain graph walk
over the emitted edges (consecutive hops share the callee-param node), proven
end-to-end. This is the v2.9 taint substrate payoff.

## What shipped

- **`dataFlowEdges(nodes, nodeToParams, nameToNode, nameCount)`** in
  `semantic_similarity_edges.go` — for each fingerprintedNode carrying a flow, for
  each param's CallArgs, resolves caller.param → callee.param and emits a
  directed `DATA_FLOWS` edge (Source `def_use`, conf 0.55, SrcKind/DstKind
  `parameter`). Deterministic (ascending NodeID); directed dedup on
  `(SrcNodeID, DstNodeID)`.
- **Callee-param index (emit-order adjacency, D1b)** in `factsFromExtracted` — a
  batch-wide `nodeToParams map[uint64][]uint64` built per-file: a function's
  params = the maximal consecutive `KindParameter` run following it in
  `single.Symbols`. This is the red-team's B1 fix — NOT the flat DEFINES
  container heuristic.
- **Anti-mis-bind multiplicity (D5)** — the name-index loop now also builds
  `nameCount`; a callee name resolves only when `nameCount == 1`.
- **Wired** at the sibling-emission point (`semantic_wiring.go`); EdgeID stamping
  covers the PK (D9, no v2.8 collision re-trigger).

## Red-team folds honored (all 6 majors + 2 blockers)

- **B1 (DEFINES is flat):** callee params via emit-order adjacency, verified on
  real Go extraction by the e2e.
- **B2 (reference-node namespace):** SrcNodeID is always a param symbol node.
- **M1 (no tree-sitter CALLS edge):** substrate = FlowSummary call entries +
  batch name index.
- **M2 (anti-mis-bind):** `nameCount != 1` ⇒ no edge.
- **M3 (non-positional):** positional + receiver-offset/variadic/kwargs handled
  in the FlowSummary (Phase 125); emission binds by ArgPos with graceful no-edge
  on ambiguity.
- **M4/M6 (cap/dedup):** directed `(Src,Dst)` dedup, no similarity cap.
- **M5 (Phase 127 collapse):** reachability folded into 126's acceptance.

## Tests (all green)

- `TestFactsFromExtracted_E2E_DataFlows` — drives the REAL Go provider through
  the full pipeline (`src(x){mid(x)}` / `mid(m){sink(m)}` / `sink(v){}`): emits
  ≥2 DATA_FLOWS edges, all param-anchored (distinctness FLOW-03b), and `sink.v`
  is reachable from `src.x` (forward) but NOT vice-versa (directed).
- `TestFactsFromExtracted_E2E_DataFlows_BrokenHop` — revert-and-fail: a dead
  `mid` (no forward) breaks reachability.
- `TestDataFlowEdges_AntiMisBind` — duplicate callee name ⇒ no edge (D5).
- `TestDataFlowEdges_DedupDirected` — two call sites same pair ⇒ one edge (D7/D8).
- `TestDataFlowEdges_ExternalCalleeNoEdge` — unresolved callee ⇒ no edge (D2).

## Verification

- `go build ./...` — clean.
- `go test ./internal/semantic/dataflow/... ./internal/semantic/extract/...
  ./internal/daemon/... ./internal/skill/semantic/...` — 15 pkgs ok, 0 fail.
- **`make vet` (go vet + 8 vettools) — clean.**
- **Zero new Go deps** — `go.mod`/`go.sum` byte-unchanged.
- **No schema migration** — `EdgeFact` unchanged; binding = node identity.

## Honest scope (the taint substrate is real but bounded)

This is **syntactic param→param pass-through reachability** — the classical
function-summary taint-composition primitive. It precisely answers "if a value
enters F's param i, can it reach G's param j?" by walking edges. It is NOT full
taint analysis: no in-body origins (`src()`), no field/heap flow, no source/sink
modeling, no sanitization (those need variable-level nodes — deferred).

## Deferred (recorded)

- Dedicated `trace-data-flow` verb / `get-change-impact-graph` multi-projection
  rework (milestone-sized; the `reachable` test helper is the primitive it'll
  consume).
- Variable-level nodes / in-body origins; queryable arg/param-position column.
