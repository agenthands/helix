---
author: qa
responsible: architect
phase: 139
status: complete
verdict: passed
---

# VERIFICATION 139 — In-body + return-bridge emission, read surface, Go multi-hop proof

**Verdict: `passed`.** Verified independently against code REALITY (not the engineer's
SUMMARY). Build+vet+tests green FRESH (`-count=1`); M1 append order correct; M4 map-free;
edge contracts exact; distinctness/honesty/multi-hop guards proven non-vacuous by an
independent source-level mutation (RED → byte-identical restore); M3 comments+test honest;
deps/schema unchanged.

## 1. Fresh build + vet + test (uncached)

| Gate | Command | Result |
|------|---------|--------|
| Build | `go build ./cmd/helix` | clean (`BUILD_OK`) |
| Vet | `make vet` | exit 0 — all 8 vettools ran clean (`go vet ./...` + 7 custom: noduckdb, nokernel2semantic, nosemantic2kernel, compact-uses-store, ablation-leakage, bench-rag-leakage, tools-quarantine) |
| Tests | `go test -count=1 ./internal/daemon/... ./internal/skill/semantic/... ./internal/semantic/...` | **47 packages ok, 3 no-test**, exit 0 |

Key packages fresh: `internal/daemon 5.007s`, `internal/skill/semantic 14.671s`,
`internal/semantic 0.016s`. No FAIL anywhere. (`-count=1` disables the test cache — genuinely re-run.)

## 2. M1 append order (read `semantic_wiring.go`, factsFromExtracted)

Read-evidence (independently re-confirmed, lines 2531–2552):

```
2534: out.Edges = append(out.Edges, dataFlowEdges(fpNodes, nodeToParams, nameToNode, nameCount)...)
2535-2540: // v2.13 comment — appended STRICTLY AFTER dataFlowEdges (M1)
2541: out.Edges = append(out.Edges, inBodyDataFlowEdges(fpNodes, nodeToParams, nameToNode, nameCount)...)
2542: out.Edges = append(out.Edges, returnBridgeEdges(fpNodes, nodeToParams)...)
2550: if fc.typeResolve {
2551:     resolveTypeEdges(&out, repoID, extracted, nameToNode, nameCount, fileIDToPath, fc.typeResolveCfg)
2552: }
```

Both new passes append at **2541–2542, STRICTLY AFTER** `dataFlowEdges` (2534) and **BEFORE**
the `resolveTypeEdges`/RESOLVES_TO block (2550–2552, still last). Existing EdgeIDs unshifted. **PASS.**

## 3. M4 map-free output (read `semantic_similarity_edges.go`)

Both funcs build the SAME sorted `withFlow` slice (`sort.Slice(...nodeID < ...)`, 559 / 631) as
`dataFlowEdges` and iterate SLICES only in the emission path. Loop headers quoted:

`inBodyDataFlowEdges` (549–608):
```
563: for _, n := range withFlow {              // sorted slice, ascending nodeID
564:     for _, ib := range n.flow.InBodyFlows {   // dedup-sorted slice
```
Map access inside is indexed LOOKUP only: `nameCount[ib.Producer]` (566), `nameToNode[ib.Producer]`
(569), `nameCount[ib.Consumer]` (574), `nameToNode[ib.Consumer]` (577), `nodeToParams[consNode]` (582).

`returnBridgeEdges` (621–667):
```
635: for _, n := range withFlow {              // sorted slice, ascending nodeID
636:     fnParams := nodeToParams[n.nodeID]        // LOOKUP only — never range the map
637:     for _, pf := range n.flow.Params {        // slice
```

No `for … range <someMap>` in either output path. Emission order = slice position → deterministic
EdgeID stamp. **PASS.**

## 4. Edge-contract correctness (read both funcs)

**in-body** (`inBodyDataFlowEdges`, EdgeFact at 595–605): `SrcNodeID=prodNode` (producer FUNCTION),
`DstNodeID=dstParam` (consumer PARAM), `SrcKind="function"`, `DstKind="parameter"`,
`EdgeKind="DATA_FLOWS"`, `Source="def_use_inbody"`, `Confidence:0.50`, `Weight:0.5`,
`Reason="return of {producer} -> {consumer} param{j}"`. Directed dedup on `[2]uint64{prodNode,dstParam}`
(590). Honesty gates: `nameCount[Producer]!=1` (566), `nameCount[Consumer]!=1` (574), `prodNode==0` (570),
`consNode==0` (578), `dstParam==0` + self-guard `dstParam==prodNode` (587). **Exact.**

**return-bridge** (`returnBridgeEdges`, EdgeFact at 653–663): `SrcNodeID=srcParam` (PARAM),
`DstNodeID=n.nodeID` (enclosing FUNCTION), `SrcKind="parameter"`, `DstKind="function"`,
`EdgeKind="DATA_FLOWS"`, `Source="def_use_return"`, `Confidence:0.55`, `Weight:0.5`,
`Reason="param{i} -> return"`. Gate `!pf.Returns` (638), bounds `pf.Index` (641), `srcParam==0` +
self-guard `srcParam==n.nodeID` (645); directed dedup on `[2]uint64{srcParam,n.nodeID}` (648). **Exact.**

## 5. FLOW-05 tests (`-count=1 -v`) — all PASS, all non-vacuous

```
--- PASS: TestFactsFromExtracted_InBodyEmission           (05a)
--- PASS: TestFactsFromExtracted_ReturnBridge             (05b)
--- PASS: TestFactsFromExtracted_InBodyAntiMisBind        (05c)
--- PASS: TestFactsFromExtracted_InBodyExternalConsumerNoEdge (05c)
--- PASS: TestFactsFromExtracted_Distinctness            (05d)
--- PASS: TestFactsFromExtracted_DefUseSetNonRegression  (05e)
--- PASS: TestFactsFromExtracted_MultiHop                (05g)
--- PASS: TestFactsFromExtracted_MultiHop_BrokenHop      (05g)
--- PASS: TestDataFlowReachability_MultiHop
--- PASS: TestDataFlowReachability_FunctionSeedReachesInBody (M3)
--- PASS: TestDataFlowReachability_FunctionAnchoredNoError   (05f)
--- PASS: TestShapeEdges_SurfacesInBodyAndReturnBridge    (05f)  [skill/semantic]
```

Non-vacuity confirmed by reading assertions (not just PASS):
- **05a**: asserts exact `producer.fn → sink.v` edge with `function/parameter`, conf/weight `0.50/0.5`,
  Reason `"return of producer -> sink param0"` (dataflow_edges_test.go:233–250).
- **05b**: exact `x → transform.fn` `parameter/function`, `0.55/0.5`, Reason `"param0 -> return"` (271–286).
- **05c**: `nameCount==2` duplicate producer ⇒ `len(def_use_inbody)==0` (301–303); external consumer ⇒ 0 (316–318).
- **05d distinctness**: in-body-only body ⇒ ≥1 def_use_inbody AND **0** def_use param→param (333–338);
  param-only body ⇒ ≥1 def_use AND **0** def_use_inbody (344–349).
- **05e non-regression**: Go PURE-param src→mid→sink; asserts the `Source=="def_use"` content-tuple SET
  is EXACTLY the v2.9 two-edge set `{src.x→mid.m, mid.m→sink.v}` (parameter/parameter, 0.55/0.5,
  "arg0 -> callee param0"), size-checked (387–394). The fixture is a pure-param forward, so D-COMPOSE never
  fires — a valid non-regression base.
- **05g multi-hop**: real Go `caller(){a:=src();b:=transform(a);sink(b)}`; `reachable()` (real BFS over the
  emitted DATA_FLOWS adjacency) proves `sink.v` reachable from `src`'s FUNCTION node (417–419). Broken-hop
  variant (transform drops its return) asserts sink.v UNREACHABLE (442–444).
- **M3 repurposed test**: asserts function seed `sk:producer` reaches in-body target `sk:v` at **hop 1**
  (the CORRECTED truth), seed at hop 0 (172–181) — NOT the now-false emptiness.
- **05f read-surface**: both DATA_FLOWS rows survive `shapeEdges`, total==2, each maps to
  `EdgeKindDataFlows` (55–70); `_FunctionAnchoredNoError` walks a mixed function-anchored chain
  srcFn→tParam→tFn→sinkParam without error and reaches `sk:v` (212–222).

Helpers are real, not stubs: `reachable` is a genuine directed BFS (157–178); `paramNodeByName`/
`functionNodeByName` are real symbol lookups (182–199); `collectDataFlowsBySource` filters by exact
Source marker (203–210).

## 6. Independent revert-and-fail (my own mutation, distinct from the engineer's)

The engineer mutated the in-body `Source` marker for distinctness. I chose a **different** guard: I
inverted the return-bridge emission gate at the SOURCE (`semantic_similarity_edges.go:638`,
`if !pf.Returns` → `if pf.Returns`) while keeping the real fixtures intact — this severs the middle
bridge hop in the emission logic itself.

RED (`go test -count=1 -run 'MultiHop$|ReturnBridge$' ./internal/daemon/`):
```
--- FAIL: TestFactsFromExtracted_ReturnBridge  dataflow_edges_test.go:263: want >=1 def_use_return edge; got none. all DATA_FLOWS: []
--- FAIL: TestFactsFromExtracted_MultiHop       dataflow_edges_test.go:418: sink.v(…) not reachable from src.fn(…) over multi-hop DATA_FLOWS;
    edges: [{…SrcKind:function DstKind:parameter Source:def_use_inbody Reason:return of src -> transform param0}
            {…SrcKind:function DstKind:parameter Source:def_use_inbody Reason:return of transform -> sink param0}]
```
The edge dump proves the two in-body hops remained but the `def_use_return` bridge vanished, so BFS
could not connect `src.fn → sink.v`. Both guards are non-vacuous.

Restore: reverted 638 to `if !pf.Returns`. **Byte-identical** —
`sha256sum` = `2f7b65e47c44a710231c9abe986c4729397debcd99df6c1a287cfa1ba5d7eb91` before AND after
(matched). Re-ran `_MultiHop`+`_ReturnBridge`: GREEN. `git diff --stat` scope unchanged (7 files,
no residual byte). No `MUTATION` string in the file.

## 7. M3 honesty (3 comments + repurposed test)

- **`accessors.go:285–293`** (`DataFlowReachabilityAccessor`): now reads *"DATA_FLOWS is no longer purely
  param→param — it also carries function→param (def_use_inbody) and param→function (def_use_return) edges,
  so a FUNCTION seed MAY now reach in-body targets."* No longer asserts purely-param-anchored. ✓
- **`semantic_wiring.go:2810–2823`** (`semP1DataFlowReachabilityAdapter`): *"Post-v2.13 this is no longer
  the whole story… a FUNCTION seed MAY mechanically reach in-body targets… full function-seed verb support
  stays a fast-follow."* ✓
- **`tools_trace_data_flow.go:30–34`** (`TraceDataFlowArgs.Seed`): *"Post-v2.13, DATA_FLOWS also carries
  function→param (def_use_inbody) edges, so a FUNCTION seed now mechanically reaches in-body targets; full
  function-seed support in the verb contract is a fast-follow (Phase 140)."* ✓ — the previously-false
  "a FUNCTION seed is not useful… param-anchored" claim is gone.
- **Repurposed test**: `TestDataFlowReachability_FunctionSeedEmpty` fully removed (0 refs anywhere) →
  `TestDataFlowReachability_FunctionSeedReachesInBody` asserts the CORRECTED truth (function seed reaches
  in-body target at hop 1). It asserts NO falsehood. ✓

All four are comment/test-name/one-repurposed-body corrections; no edge or verb behavior changed.

## 8. Deps + schema

- `git diff --stat go.mod go.sum` → **empty** (zero new deps).
- `git diff --stat internal/semantic/store/` → **empty** (no schema migration; `EdgeFact` unchanged).

## Scope confirmation

`git diff --stat` of the seven Phase-139 files matches the SUMMARY's claimed diff stat exactly:
`dataflow_edges_test.go +256`, `dataflow_reachability_test.go 92±`, `semantic_similarity_edges.go +133`,
`semantic_wiring.go 20±`, `accessors.go 16±`, `explain_symbol_relatededge_test.go +33`,
`tools_trace_data_flow.go 7±` (total +528/-29).

## Verdict rationale

Every `passed` gate met: fresh build+vet+tests green; M1 order correct (read-confirmed 2534→2541/2542→2550);
M4 map-free (loop headers slice-only, map lookups indexed); edge contracts exact for both passes;
distinctness/honesty/multi-hop guards proven non-vacuous by an independent source mutation with
byte-identical restore; M3 comments + repurposed test are honest (no falsehood asserted); deps/schema
untouched. → **`passed`**. No gaps for the architect.

## Records

```yaml
gate_result:
  record_author: qa
  accountable_owner: qa
  id: GV213-139
  kind: phase_verification
  gates_subject: phase-139-inbody-returnbridge-emission-go-proof
  evaluator: qa
  verdict: pass
  status: accepted
  method:
    - ran the emission/distinctness/non-regression/multi-hop suite fresh (all pass)
    - read inBodyDataFlowEdges + returnBridgeEdges against the edge contracts
    - confirmed M1 append order (semantic_wiring.go:2541-2542) + M4 map-free output
    - confirmed the M3 test asserts the corrected truth (not a falsehood)
  evidence_refs:
    - EV213-139
```

```yaml
evidence:
  record_author: qa
  accountable_owner: qa
  evidence_producer: qa
  commissioned_by: qa
  issuance:
    kind: peer_authored
  id: EV213-139
  subject_ref: GV213-139
  kind: test
  warrant:
    kind: test_pass
    scope:
      - internal/daemon/dataflow_edges_test.go
      - internal/daemon/dataflow_reachability_test.go
      - internal/daemon/semantic_similarity_edges.go
      - internal/daemon/semantic_wiring.go
    rationale: >-
      go test -count=1 on the daemon emission suite -> InBodyEmission, ReturnBridge,
      Distinctness, DefUseSetNonRegression, MultiHop (+ BrokenHop guard),
      FunctionSeedReachesInBody all PASS; emission funcs + M1/M4/M3 confirmed by reading.
  freshness:
    status: fresh
    checked_against: HEAD
  supports:
    - GV213-139
```
