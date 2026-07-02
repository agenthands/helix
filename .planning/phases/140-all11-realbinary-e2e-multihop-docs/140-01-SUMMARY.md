---
author: engineer
responsible: engineer
phase: 140
plan: 140-01
status: complete
---

# SUMMARY 140-01 — All-11 real-binary E2E + function-seeded multi-hop + docs

## Outcome

A HELIX_BIN-gated real-binary E2E oracle (`internal/cli/cli_dataflow_inbody_e2e_test.go`,
build tag `!windows`, pkg `cli_test`) CONFIRMS through a REAL `helix` binary driven as a
subprocess against a live daemon that:

1. **In-body-origin `data_flows` surfaces via `helix explain-symbol-deep` for ALL 11
   languages** — go, typescript, java, csharp, python, c, cpp, rust, kotlin, php, ruby —
   with per-language differential anti-vacuity (positive edge + negative zero). **11/11
   PASS, 0 skips.**
2. **A function-seeded producer→transform→sink multi-hop is reachable via the REAL
   `helix trace-data-flow`** for Go (FLOW-06c), and the **broken-hop variant is
   unreachable** (revert-and-fail RED confirmed).
3. **Determinism** (FLOW-06d): re-index → identical in-body edge set (Go subtest).
4. The DATA_FLOWS docs (`docs/edge-types.md`) updated for in-body origins, both new
   `Source` markers, the function-seed interaction, and the honest deferred surfaces.

**Path B chosen by the co-driver** (relayed via Main): the milestone's D-BREADTH "all 11
proven E2E" deliverable is realized by wiring Rust/Kotlin/PHP/Ruby into the daemon's
full-index walk (`langFromExt`). No engine / emission / verb-behavior change; the DATA_FLOWS
edge contract and the `trace_data_flow` verb are untouched. `go.mod`/`go.sum` unchanged.

## Per-language E2E ledger (FLOW-06a / FLOW-06b) — 11/11 GREEN

| Lang | E2E result | Notes |
|---|---|---|
| go | **PASS** | in-body edge producer.function → sink.param; +determinism guard green |
| typescript | **PASS** | |
| java | **PASS** | class-wrapped (methods resolvable) |
| csharp | **PASS** | class-wrapped |
| python | **PASS** | |
| c | **PASS** | top-level free functions captured by the C extractor |
| cpp | **PASS** | class-wrapped (fixture-shape fix — see surprises) |
| kotlin | **PASS** | class-wrapped; wired via `langFromExt` (.kt/.kts, Path B) |
| php | **PASS** | wired via `langFromExt` (.php, Path B) |
| ruby | **PASS** | wired via `langFromExt` (.rb, Path B) |
| rust | **PASS** | wired via `langFromExt` (.rs, Path B) |

**11/11 E2E green; ZERO skips.** The honest breadth ledger (FLOW-06b) closes empty.

## The daemon-gate discovery + Path A→B decision trail

The first real-binary run was 7/11 (rust/kotlin/php/ruby returned `resolution="not_found"`).
Root cause: the daemon's full-index walk gate `langFromExt`
(`internal/daemon/semantic_wiring.go`) mapped `.rs/.kt/.php/.rb → ""` — a **deliberate,
tested product deferral** ("stay '' until v2.13 wires them"), LOCKED by
`TestLangFromExt_CFamily`. All 11 providers ARE registered at daemon bootstrap
(`daemon.go:362-372`); `langFromExt` was the SOLE gate.

This was **invisible until Phase 140**: Phase 138's all-11 unit matrix runs `AnalyzeFlow` on
bare parse trees (bypassing the daemon walk), and Phase 139 was Go-only in-process. I
confirmed (a scratch `factsFromExtracted` probe, since removed) that all 4 gated languages
emit the exact `def_use_inbody` edge `producer.function → sink.param0` — the block was purely
the extension gate, not engine or emission.

The fork (Path A: record 4 skips, diff = test+docs only; Path B: wire the 4, achieve 11/11,
a shipped-product behavior change v2.12 deferred) was escalated to the co-driver, who chose
**Path B**. Type-resolver risk is VOID: `newIndexedResolver` returns nil for these 4
(`type_resolver_wiring.go`), and a nil resolver is skipped → wiring them adds ZERO
`has_type`/`uses_type` edges; extraction covers symbols / references / similarity /
`data_flows` only.

## Path B changes (this phase's engine-adjacent edits)

1. **`langFromExt` (`semantic_wiring.go`)** — added `.rs→"rust"`, `.kt/.kts→"kotlin"`,
   `.php→"php"`, `.rb→"ruby"`; doc-comment corrected (v2.13 wires them; type resolvers
   remain nil-stubs → no has_type/uses_type). NOT an engine/emission/verb change — it is the
   daemon file-walk extension map (the D-BREADTH wiring the milestone mandates).
2. **`TestLangFromExt_CFamily` (`semantic_cfamily_wiring_test.go`)** — the guard that locked
   `.rs/.kt/.php/.rb → ""` updated to assert the new mappings (`rust`/`kotlin`/`php`/`ruby`);
   doc-comment corrected. `TestLangFromExt_RegistryResolves` unchanged (still C-family
   scoped) — all 11 providers already resolve in the real daemon registry.

## Function-seeded multi-hop (FLOW-06c, M2) — PASS through the real binary

`TestCLI_E2E_InBodyMultiHop` (Go): fixture `caller(){ a:=src(); b:=transform(a); sink(b) }`
with `transform(x){return x}`. Through the REAL `helix trace-data-flow --seed-json` seeded
at the **producer FUNCTION `src`** (mechanically accepted — no kind gate, the BFS is
kind-agnostic node-ID adjacency per 139's confirmed surprise), **`sink`'s param `v` is
reachable** over in-body(src→transform.x) → return-bridge(transform.x→transform) →
in-body(transform→sink.v). The reachability assertion is an exact `symbol_id` match (the
trace reachable set's `SymbolID` is the same stable-key namespace as
`explain-symbol-deep`'s seed `symbol_id`, via `QueryStableKeyByNodeID`).

**Broken-hop (revert-and-fail RED):** the same workspace is overwritten with
`transform(x){return 0}` (the param goes dead → no `ParamFlow` → no return-bridge edge),
re-indexed, and re-traced: `sink.v` is **NO LONGER reachable** from `src`. The severed hop
breaks reachability, exactly as required. GREEN.

## Determinism (FLOW-06d) — PASS

The Go subtest of `TestCLI_E2E_InBodyDataFlow` re-runs `helix index-semantic-graph
--mode-arg=full` and re-`explain-symbol-deep`s the consumer param; the in-body `data_flows`
edge set (count + from/to/edge_kind/internal_kind tuples, order-independent via
`sameEdgeSet`) is identical across re-index.

## Docs (FLOW-06e) — `docs/edge-types.md`

- **Families table:** the `data_flows` row now lists all three producers
  (`dataFlowEdges` v2.9, `inBodyDataFlowEdges` v2.13, `returnBridgeEdges` v2.13).
- **`data_flows` edge section:** rewritten to document the three `Source` markers
  (`def_use` param→param; `def_use_inbody` producer.function→consumer.param;
  `def_use_return` param→enclosing-function), that in-body origins are now modeled while
  binding stays symbol-node identity, and the honest **deferred surfaces** (variable-level
  precision, field/heap flow, source/sink taint). Language-coverage ledger states **all 11
  proven E2E** (v2.13 wired the last four; their type resolvers stay nil-stubs → no
  has_type/uses_type).
- **Reader section:** `trace-data-flow` note updated — the BFS is kind-agnostic over all
  three markers, so a **function seed** now mechanically reaches in-body targets (full
  function-seed verb contract is a fast-follow).

## Test-design notes (grounded)

- **Reuses the v2.12 harness verbatim:** `newE2EFixture`, `switchModeReview` (MCP
  `switch_mode`), `indexFull`, `explainSymbol`, `explainDeepResult`/`explainEdge`,
  `containsToken` — no harness reinvention. Added `dataFlowEdgesOf` (mirrors `hasTypeEdges`),
  `inBodyEdgesInto` (endpoint-shape filter: To==seed ∧ From carries "producer"),
  `newInBodyDataFlowFixture` (mirrors `newCTypeE2EFixture`; guards `os.Remove(main.go)` so
  the Go fixture — itself `main.go` — is not deleted), `traceDataFlowResult`/`traceDataFlow`
  (parse struct mirroring the verb's JSON; the verb is UNCHANGED), `sameEdgeSet`.
- **Seed-to-surface:** a `def_use_inbody` edge is producer.function → consumer.param;
  seeding the CONSUMER param surfaces it as an INCOMING edge (explain applies no kind
  filter). The in-body edge is distinguished from v2.9 param→param (From would be a param)
  and the return-bridge (To would be a function) by the endpoint shape, since `Source` is
  not on the surface envelope.
- **Differential negative:** a `pure(q)` method fed a literal `pure(0)` → `q` resolves
  `exact` but has ZERO in-body inflow → zero `data_flows` edges. Positive asserted FIRST.
- **skipReason mechanism retained** (field + subtest guard) though no case sets it now — a
  future genuinely-unresolvable language stays a recorded skip, not a silent omission.

## Fixture-shape surprises

1. **C++ captures only in-class method names, not top-level free functions.** The cpp
   extractor's `@definition.function` rule matches a `field_identifier` declarator (in-class
   method) but NOT a plain `identifier` (free function). Confirmed by the existing golden
   `cpp/testdata/function_basic/before.cpp` = `int hello(){ return 0; }` → **zero symbols**
   (expected.json has only a `types` row). So the cpp fixture is **class-wrapped** (like
   Java/C#/Kotlin). C, by contrast, captures free functions and needs no wrap. Pre-existing
   cpp-extractor property, not a v2.13 regression.
2. **`nameToNode`/`nameCount` key on the BARE name** (`semantic_wiring.go:2466-2468`), so
   class-wrapped methods (`producer`, `sink`) resolve by their bare name with no
   qualified-name handling needed — class-wrapping does not break producer/consumer
   resolution.
3. **The daemon `langFromExt` extension gate, not the engine, was the all-11 breadth
   blocker** — the engine union work (Phase 138) and emission (Phase 139) were complete;
   the last mile was one switch arm per language in the daemon walk (the M5-style "invisible
   until the expensive phase" risk, but for the daemon gate rather than the engine union).

## Acceptance checklist

1. `go build ./...` — clean. `make vet` — clean (exit 0, all 8 vettools).
2. `HELIX_BIN=/tmp/helix-v213-e2e go test -count=1 -v -run TestCLI_E2E_InBody ./internal/cli/`
   — **TestCLI_E2E_InBodyDataFlow 11/11 PASS (0 skips)** + **TestCLI_E2E_InBodyMultiHop PASS**.
   Real PASS, not skip.
3. Function-seeded multi-hop PASS through the binary (FLOW-06c); broken-hop RED confirmed.
4. Determinism guard green (FLOW-06d).
5. `docs/edge-types.md` updated (both Source markers + function-seed note + all-11 coverage
   ledger).
6. **Regression check** (daemon walk now indexes 4 new langs): `go test -count=1
   ./internal/daemon/... ./internal/skill/semantic/...` — **both packages ok** (no
   pre-existing test broke from enabling extraction).
7. `git diff --stat go.mod go.sum` — **empty** (zero new deps).
8. Scratch diagnostic (`internal/daemon/zz_scratch_inbody_test.go`) REMOVED — did not ship.
9. This SUMMARY written. Working tree left dirty (NOT committed).

## Diff stat (Phase 140's own contribution)

```
internal/cli/cli_dataflow_inbody_e2e_test.go     | ~540 (new)   E2E oracle
docs/edge-types.md                               |   59 (+47/-12) DATA_FLOWS docs
internal/daemon/semantic_wiring.go               |   langFromExt: +8 arms + doc (Path B)
internal/daemon/semantic_cfamily_wiring_test.go  |   13 (+8/-5)  guard test flipped to wired
```

(The full working tree also carries the 138 files — summary.go, dataflow_test.go,
dataflow_matrix_test.go — and the 139 files — semantic_similarity_edges.go, the 139 portion
of semantic_wiring.go, accessors.go, tools_trace_data_flow.go, dataflow_edges_test.go,
dataflow_reachability_test.go, explain_symbol_relatededge_test.go — as the dirty baseline
this phase built on. Phase-140 own edits: the new cli test, docs, the langFromExt arms +
doc in semantic_wiring.go, and the langFromExt guard test.)

## Out of scope / follow-ups

- Full function-seed support in the `trace_data_flow` verb's documented contract
  (fast-follow); variable-level nodes, field/heap flow, source/sink taint (milestone
  out-of-scope). Type resolvers for Rust/Kotlin/PHP/Ruby remain nil-stubs (no
  has_type/uses_type) — a separate deferred track.
