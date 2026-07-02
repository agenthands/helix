---
author: engineer
responsible: engineer
phase: 139
plan: 139-01
status: complete
---

# SUMMARY 139-01 — In-body + return-bridge emission, read surface, Go multi-hop proof

## Outcome

The two Phase-138 flow-summary signals are now EMITTED as DATA_FLOWS edges:
`inBodyDataFlowEdges` (producer.function → consumer.param, `def_use_inbody`) and
`returnBridgeEdges` (param → enclosing-function, `def_use_return`). Both wired
STRICTLY AFTER `dataFlowEdges` (M1), map-free in the output path (M4), honest
(endpoint-0/nameCount>1 gated out), distinct (own `Source` markers), non-regressing
(the 6-lang `def_use` SET byte-unchanged), CLI-readable (survive `shapeEdges`,
traverse the reachability BFS), and multi-hop-reachable in-process on real Go. The
three now-false M3 invariants are corrected. Zero new Go deps; no schema migration.

## What changed (files + functions)

### `internal/daemon/semantic_similarity_edges.go` (+133)
- **`inBodyDataFlowEdges(nodes, nodeToParams, nameToNode, nameCount)` (NEW):**
  mirrors `dataFlowEdges` — same sorted `withFlow` list (ascending nodeID), gated on
  `n.flow != nil && len(n.flow.InBodyFlows) > 0`. Resolves producer FUNCTION node
  (`nameCount==1`, `!=0`), consumer FUNCTION node (`nameCount==1`, `!=0`), consumer
  PARAM node via `nodeToParams[consNode][ib.ArgPos]`; self-guard (`dstParam != prodNode`);
  directed dedup on `[2]uint64{prodNode, dstParam}`. Emits the in-body contract:
  `SrcKind="function"`, `DstKind="parameter"`, `Source="def_use_inbody"`,
  `Confidence:0.50`, `Weight:0.5`, `Reason:"return of {producer} -> {consumer} param{j}"`.
- **`returnBridgeEdges(nodes, nodeToParams)` (NEW):** same sorted `withFlow` list gated
  on `len(n.flow.Params) > 0`; `fnParams := nodeToParams[n.nodeID]` (LOOKUP only, never
  ranged); for each `pf` with `pf.Returns`, `srcParam := fnParams[pf.Index]`; self-guard
  (`srcParam != n.nodeID`); directed dedup on `[2]uint64{srcParam, n.nodeID}`. Emits:
  `SrcKind="parameter"`, `DstKind="function"`, `Source="def_use_return"`,
  `Confidence:0.55`, `Weight:0.5`, `Reason:"param{i} -> return"`.

### `internal/daemon/semantic_wiring.go` (+20 / M1 wiring + M3 comment)
- Wired both passes in `factsFromExtracted` immediately after the `dataFlowEdges`
  append (line 2534); `resolveTypeEdges` (RESOLVES_TO) still runs LAST (line 2551).
- Corrected the M3 `semP1DataFlowReachabilityAdapter` seed-semantics doc-comment.

### `internal/skill/semantic/accessors.go` (+16 / M3)
- Corrected the `DataFlowReachabilityAccessor` doc-comment: DATA_FLOWS is no longer
  purely param→param; a function seed MAY now reach in-body targets.

### `internal/skill/semantic/tools_trace_data_flow.go` (+7 / M3)
- Softened the `TraceDataFlowArgs.Seed` comment: a function seed now mechanically
  reaches in-body targets; full verb function-seed support is a fast-follow (Phase 140).

### `internal/daemon/dataflow_reachability_test.go` (+92 / M3 repurpose + FLOW-05f)
- **Repurposed** `TestDataFlowReachability_FunctionSeedEmpty` →
  `TestDataFlowReachability_FunctionSeedReachesInBody`: with a function→param
  `def_use_inbody` edge present, a FUNCTION seed reaches the in-body target at hop 1
  (asserts the NEW truth; no longer encodes the now-false emptiness).
- **Added** `TestDataFlowReachability_FunctionAnchoredNoError` (FLOW-05f smoke): the
  param-seeded BFS traverses a mixed function-anchored chain
  (srcFn→tParam→tFn→sinkParam) without error.

### `internal/daemon/dataflow_edges_test.go` (+256 / FLOW-05a–e,g)
- Helpers `functionNodeByName`, `collectDataFlowsBySource`.
- 8 tests: `_InBodyEmission` (05a), `_ReturnBridge` (05b), `_InBodyAntiMisBind` +
  `_InBodyExternalConsumerNoEdge` (05c), `_Distinctness` (05d),
  `_DefUseSetNonRegression` (05e, Go pure-param src→mid→sink content-tuple SET), `_MultiHop`
  + `_MultiHop_BrokenHop` (05g).

### `internal/skill/semantic/explain_symbol_relatededge_test.go` (+33 / FLOW-05f)
- `TestShapeEdges_SurfacesInBodyAndReturnBridge`: both DATA_FLOWS rows survive
  `shapeEdges` and map to `EdgeKindDataFlows` (readable from `explain-symbol-deep`).

## M1 append-order evidence (verify-by-reading, `semantic_wiring.go`)

```
2534:	out.Edges = append(out.Edges, dataFlowEdges(fpNodes, nodeToParams, nameToNode, nameCount)...)
2535:	// v2.13 in-body-origin + return-bridge DATA_FLOWS (Phase 139). Appended
2536:	// STRICTLY AFTER dataFlowEdges (M1 hard constraint): EdgeID is a dense ...
2541:	out.Edges = append(out.Edges, inBodyDataFlowEdges(fpNodes, nodeToParams, nameToNode, nameCount)...)
2542:	out.Edges = append(out.Edges, returnBridgeEdges(fpNodes, nodeToParams)...)
...
2550:	if fc.typeResolve {
2551:		resolveTypeEdges(&out, repoID, extracted, nameToNode, nameCount, fileIDToPath, fc.typeResolveCfg)
2552:	}
```

The two new appends are STRICTLY AFTER the `dataFlowEdges` append (2534) and BEFORE
the `resolveTypeEdges`/RESOLVES_TO block (2551, still last) — existing EdgeIDs unshifted.

## M4 map-free evidence (loop headers quoted)

`inBodyDataFlowEdges` output path iterates SLICES only:
```
563:	for _, n := range withFlow {          // sorted slice (ascending nodeID)
564:		for _, ib := range n.flow.InBodyFlows {   // dedup-sorted slice
```
`nameToNode`/`nameCount`/`nodeToParams` are indexed for LOOKUP, never ranged.

`returnBridgeEdges` output path iterates SLICES only:
```
635:	for _, n := range withFlow {          // sorted slice (ascending nodeID)
636:		fnParams := nodeToParams[n.nodeID] // LOOKUP only — never range the map
637:		for _, pf := range n.flow.Params {        // slice
```
No `for k := range <map>` anywhere in either output path — emission order is
deterministic (EdgeID is a slice-position stamp).

## Distinctness + multi-hop proof

- **Distinctness (FLOW-05d, `_Distinctness`):** in-body-only body (`caller(){a:=producer();sink(a)}`)
  ⇒ ≥1 `def_use_inbody` edge + ZERO `def_use` param→param; param-only body
  (`f(x){sink(x)}`) ⇒ ≥1 `def_use` + ZERO `def_use_inbody`. GREEN.
- **def_use SET non-regression (FLOW-05e, `_DefUseSetNonRegression`):** on the pure-param
  Go forward src→mid→sink (no in-body call-return, D-COMPOSE never fires), the
  `Source=="def_use"` content-tuple SET is EXACTLY the v2.9 two-edge set
  {src.x→mid.m, mid.m→sink.v}, each `parameter/parameter`, `Confidence 0.55`, `Weight 0.5`,
  `Reason "arg0 -> callee param0"`. Scoped to Go (a stable lang per the 138 ledger).
- **Multi-hop (FLOW-05g, `_MultiHop`):** real Go `caller(){a:=src();b:=transform(a);sink(b)}`
  with `src()int`, `transform(x int)int{return x}`, `sink(v int)`. `sink.v` is reachable
  from `src`'s FUNCTION node via in-body(src→transform.x) → return-bridge(transform.x→transform)
  → in-body(transform→sink.v). GREEN.

## Revert-and-fail evidence (anti-vacuity — mandatory)

Chose the FLOW-05d distinctness mutation. Replaced the in-body `Source:"def_use_inbody"`
marker with `Source:"def_use"` in `inBodyDataFlowEdges`. `TestFactsFromExtracted_Distinctness`
went RED on BOTH assertions:
```
--- FAIL: TestFactsFromExtracted_Distinctness (0.00s)
    dataflow_edges_test.go:334: in-body-only: want >=1 def_use_inbody edge; got 0
    dataflow_edges_test.go:337: in-body-only: want 0 def_use param->param edges; got [{...Source:def_use Reason:return of producer -> sink param0}]
```
Restored the marker byte-identical (`git diff` shows zero `MUTATION` strings); full
affected suite GREEN again.

(Note: an earlier attempt to break the return-bridge via the `pf.Returns` gate did NOT
fire the broken-hop guard — because the broken fixture's `transform` returns a constant,
so its param is dead and `AnalyzeFlow` returns nil, never reaching the gate. That is the
dead-param anti-vacuity path working correctly upstream; the distinctness mutation is the
honest revert-and-fail here.)

## M3 sites corrected (FLOW-05h)

1. `internal/skill/semantic/accessors.go` `DataFlowReachabilityAccessor` doc-comment.
2. `internal/daemon/semantic_wiring.go` `semP1DataFlowReachabilityAdapter` seed-semantics comment.
3. `internal/skill/semantic/tools_trace_data_flow.go` `TraceDataFlowArgs.Seed` comment.
4. `internal/daemon/dataflow_reachability_test.go` — `TestDataFlowReachability_FunctionSeedEmpty`
   renamed/repurposed to `_FunctionSeedReachesInBody` (asserts the corrected truth). No
   test asserts the now-false emptiness.

All four are comment/test-name corrections plus one repurposed test body; NO edge or
verb behavior changed.

## Acceptance checklist

1. `go build ./cmd/helix` — clean. `make vet` — clean (all 8 vettools).
2. `go test -count=1 ./internal/daemon/... ./internal/skill/semantic/... ./internal/semantic/...`
   — **47 packages ok, 3 no tests**. 11 new/repurposed FLOW-05 test functions all GREEN.
3. M1 append-order + M4 map-free confirmed by reading (evidence above).
4. Revert-and-fail RED (distinctness mutation) + byte-identical restore — reported.
5. M3: 3 comments corrected + reachability test repurposed — no falsehood asserted.
6. `git diff --stat go.mod go.sum` — **empty** (zero new deps). No store/schema change
   (`git diff --stat internal/semantic/store/` empty; `EdgeFact` unchanged).
7. Working tree left dirty (not committed).

## Surprises

- **The reachability BFS is fully kind-agnostic** — `QueryAllDataFlowEdges` filters on
  `edge_kind = 'DATA_FLOWS'` (not `Source`), so all three markers (`def_use`,
  `def_use_inbody`, `def_use_return`) feed one adjacency map. A function seed reaches
  in-body targets with ZERO adapter change (M2's "no kind gate" observation, confirmed).
- **The return-bridge dead-param interaction:** severing a hop by making `transform`
  return a constant removes the param's `ParamFlow` entirely (engine anti-vacuity),
  so the return-bridge never even evaluates `pf.Returns` — the multi-hop breaks upstream
  of emission. Honest and correct, but it means the broken-hop RED comes from the missing
  edge, not a false-positive emission; the distinctness mutation is the cleaner
  emission-logic break for the anti-vacuity proof.

## Diff stat

```
internal/daemon/dataflow_edges_test.go             | 256 +++
internal/daemon/dataflow_reachability_test.go      |  92 +-
internal/daemon/semantic_similarity_edges.go       | 133 +++
internal/daemon/semantic_wiring.go                 |  20 +-
internal/skill/semantic/accessors.go               |  16 +-
internal/skill/semantic/explain_symbol_relatededge_test.go |  33 +
internal/skill/semantic/tools_trace_data_flow.go   |   7 +-
```
(plus the Phase-138 dirty files summary.go / dataflow_test.go carried in the tree.)

## Out of scope (Phase 140)

Real-binary E2E across 11 languages; full function-seed verb contract; DATA_FLOWS docs.
