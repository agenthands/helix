---
author: engineer
responsible: engineer
phase: 138
plan: 138-01
status: complete
---

# SUMMARY 138-01 — Origin-space widening + kind-union extension + all-11 unit verification

## Outcome

The `dataflow` leaf engine now tracks **in-body origins** (the return value of an
in-body call, `y := producer(); sink(y)`) alongside v2.9 param origins, records
`Producer → Consumer.ArgPos` in-body flows deterministically, and does so across
**all 11 grammars**. **No emission** (Phase 139). Leaf boundary intact (stdlib +
tree-sitter only); `go.mod`/`go.sum` unchanged (zero new deps).

## What changed (files + functions)

### `internal/semantic/dataflow/summary.go` (+164 / −36)

- **Types (FLOW-04a):**
  - `Origin{Param int; Callee string}` — comparable discriminant. `Callee==""` ⇒
    param origin (`Param` = 0-based index); `Callee!=""` ⇒ callReturn origin.
  - `InBodyFlow{Producer, Consumer string; ArgPos int}`.
  - `Summary.InBodyFlows []InBodyFlow` added (nil when none — anti-vacuity).
  - `taint` widened everywhere to `map[string]map[Origin]bool`; `collectOrigins`
    now returns `map[Origin]bool`. All these fns are internal to `summary.go`
    (verified: no external caller), so the widening is fully contained.
- **`AnalyzeFlow`:** relaxed the `len(paramNames)==0` early bail — now **always
  walks the body**. Threads `var inBody []InBodyFlow` via `&inBody`. Returns nil
  **only** when `len(out.Params)==0 && len(out.InBodyFlows)==0` (the v2.9
  anti-vacuity gate generalized, per the CONTEXT "load-bearing correction").
- **`processAssignment` (D-COMPOSE, FLOW-04b + m2):** before the generic path,
  tries `unwrapToCall(rhs)`; on success sets `taint[lhs] = {Origin{Callee:name}}`
  and returns (LHS = callReturn of the OUTERMOST call). Else falls through to the
  widened generic ident-origin propagation (covers pure ident forwarding `b := a`).
- **`processCall` (FLOW-04c):** per arg, splits origins — param origin ⇒ v2.9
  `CallArgs` path **unchanged**; callReturn origin ⇒ append
  `InBodyFlow{Producer:o.Callee, Consumer:callee, ArgPos:i}` to `*inBody`.
- **`processReturn` (FLOW-04d):** param origin ⇒ `Returns=true` (unchanged);
  callReturn-reaches-return ⇒ **NOT recorded** (locked out — no v2.13 edge).
- **`unwrapToCall` (NEW):** strict single-child whitelist
  `{expression_list, parenthesized_expression}` → call node, else nil. Multi-child
  expression_list or any non-whitelisted wrapper (selector/member/index/binary)
  refuses.
- **`dedupSortInBody` (NEW, FLOW-04f):** sort by (Producer, Consumer, ArgPos),
  dedup adjacent — mirrors `dedupSortCallArgs`. No Go-map iteration in the output
  path (determinism).
- **Unions (FLOW-04i / B1), `summary.go:78-108`:**
  - `paramListKinds` += `method_parameters` (Ruby).
  - `assignKinds`: replaced dead `init_declaration` with `init_declarator`
    (C/C++); added `assignment` (Python/Ruby) and `property_declaration` (Kotlin).
  - `argListKinds` += `value_arguments` (Kotlin).

### `internal/semantic/dataflow/dataflow_test.go` (+153)

Added: `TestAnalyzeFlow_InBody_Single`, `_CoexistWithParam`, `_TwoHopChain`
(FLOW-04h); `_NestedWrapOutermost`, `_SelectorRHS_NoOrigin`, `_MultiCallRHS_NoOrigin`
(m2 RED guards); `_DeadLocal_AntiVacuity`, `_PureParam_NoInBody` (FLOW-04g);
`_Determinism` (FLOW-04f, `reflect.DeepEqual` on re-analyze). Helpers `inBodyEqual`,
`hasInBody`.

### `internal/semantic/dataflow/dataflow_matrix_test.go` (NEW, +122)

`TestAnalyzeFlow_AllElevenMatrix`: table over all 11 providers, parses the
idiomatic `local := producer(); sink(local)` fixture (class-wrapped for
Java/C#/Kotlin), locates the func-decl node by `funcDeclKind`, runs `AnalyzeFlow`,
asserts `InBodyFlows` contains `{producer, sink, 0}`. Skip path with a recorded
reason string is wired (`skipReason`) but **unused** — all 11 resolve.

### Carry seam (FLOW-04e) — VERIFIED, not re-plumbed

`SymbolFact.FlowSummary *dataflow.Summary` (`fact.go:127`) +
`FingerprintBody` (`fingerprint.go:54`) carry the widened struct through the
pointer unchanged. Confirmed by the extract test suite staying green (13 pkgs);
no edit to fact.go / fingerprint.go was needed.

## Per-language matrix result (M5 / FLOW-04i)

| Lang | funcDeclKind | assign kind | arg-list kind | Result |
|---|---|---|---|---|
| go | function_declaration | short_var_declaration | argument_list | **PASS** |
| typescript | function_declaration | variable_declarator | arguments | **PASS** |
| java | method_declaration | variable_declarator | argument_list | **PASS** |
| csharp | method_declaration | variable_declaration/declarator | argument_list | **PASS** |
| kotlin | function_declaration | property_declaration (NEW) | value_arguments (NEW) | **PASS** |
| php | function_definition | assignment_expression | arguments | **PASS** |
| python | function_definition | assignment (NEW) | argument_list | **PASS** |
| ruby | method | assignment (NEW) + method_parameters (NEW) | argument_list | **PASS** |
| rust | function_item | let_declaration | arguments | **PASS** |
| c | function_definition | init_declarator (NEW) | argument_list | **PASS** |
| cpp | function_definition | init_declarator (NEW) | argument_list | **PASS** |

**11/11 GREEN — zero skips.** The honest breadth ledger starts empty: no language
required xfail. Ruby (which emitted **nothing** since v2.9 — its params parse as
`method_parameters ∉ paramListKinds`, so `collectParams` was empty and the engine
bailed) now resolves both its in-body flow and its latent v2.9 param→param flows.

## FLOW-05e disclosure ledger — which languages' v2.9 def_use SET is genuinely NEW (Phase 139 depends on this)

The union widening retroactively repairs latent v2.9 param→param gaps. I measured
the HEAD baseline empirically (stashed my change, restored the pre-138 unions, ran
a param-flow probe: `direct` = `f(x){ sink(x) }`, `transitive` = `f(x){ y=x; sink(y) }`),
then re-measured post-138. `true` = the v2.9 `CallArgs` (def_use param→param) flow
is recorded.

| Lang | HEAD direct | HEAD transitive | post-138 direct | post-138 transitive | def_use SET status |
|---|---|---|---|---|---|
| ruby | **false** | **false** | true | true | **WHOLESALE NEW** (params were `method_parameters ∉ paramListKinds` → engine bailed → ZERO edges since v2.9) |
| kotlin | **false** | **false** | true | true | **WHOLESALE NEW** (call args were `value_arguments ∉ argListKinds` → `processCall` found no args → ZERO edges) |
| python | true | **false** | true | true | **PARTIALLY NEW** (direct flows already emitted at HEAD; assignment-transitive flows NEW via `assignment ∈ assignKinds`) |
| c | true | **false** | true | true | **PARTIALLY NEW** (direct already emitted; init-transitive NEW via `init_declarator` replacing the dead `init_declaration`) |
| cpp | true | **false** | true | true | **PARTIALLY NEW** (same as C) |
| go / typescript / java / csharp / php | true | true | true | true | **UNCHANGED** (byte-identical def_use SET — the 6 already-working langs, minus the 5 above) |

**Phase 139 FLOW-05e guidance:** assert def_use SET unchanged ONLY for
**go/typescript/java/csharp/php** (the genuinely-stable set). Ruby and Kotlin emit
def_use param→param edges *wholesale-new*; Python/C/C++ emit *additional*
assignment/init-transitive def_use edges (their direct edges were already present).
All of this is a folded latent-v2.9-bug-fix (disclosed, not silent) and does NOT
mutate the existing 6-language edge content.

## Revert-and-fail evidence (acceptance #4 — anti-vacuity is not theater)

Temporarily replaced the strict `unwrapToCall` whitelist with the naive
`findFirst(n, callKinds)` anti-pattern that m2 explicitly forbids (descends into
selector/member/multi-call RHS). Result:

```
--- FAIL: TestAnalyzeFlow_InBody_SelectorRHS_NoOrigin
    selector RHS must yield no in-body origin; got [{Producer:producer Consumer:sink ArgPos:0}]
--- FAIL: TestAnalyzeFlow_InBody_MultiCallRHS_NoOrigin
    multi-call RHS must yield no in-body origin; got [{Producer:f Consumer:sink ArgPos:0}]
```

Both m2 guards went **RED** (the broken engine falsely bound `producer→sink` for a
selector RHS and `f→sink` for a multi-call RHS). Restored the strict whitelist →
all green again. This proves the guards actually constrain the engine.

## AST-shape surprises found (empirically dumped against the pinned grammars)

1. **Kotlin `property_declaration` has NO field names.** Its named children are
   positional: child[0]=`variable_declaration` (holds the LHS ident), child[1]=
   `call_expression` (the RHS). The existing positional fallback in
   `splitAssignment` (first named child → `identText` descends to `local`; last
   named child = the call) already resolves it — **no bespoke Kotlin split branch
   was needed** (the PLAN allowed for one; reality was cleaner). Adding
   `property_declaration`→`assignKinds` + `value_arguments`→`argListKinds` sufficed.
   *(Phase 139/140: property_declaration rides the positional path, not a bespoke split.)*
2. **Go's `short_var_declaration` RHS is an `expression_list`** wrapping the call
   (`local := producer()` → right field = `expression_list[call_expression]`). This
   is exactly why `unwrapToCall`'s expression_list unwrap is load-bearing for Go,
   and why the multi-call guard (`a, b := f(), g()` → multi-child expression_list)
   is a real Go shape, not a hypothetical.
3. **Rust `let_declaration` / C·C++ `init_declarator`** use `pattern`/`value` and
   `declarator`/`value` fields respectively (no `name`/`left`) — the positional
   fallback + the `value` handling in `splitAssignment` covers them once the kind
   is in `assignKinds`.

## Skips / justification

None. All 11 languages green; the breadth ledger carried into Phase 140 (FLOW-06b)
is currently empty.

## Deliberate behavioral refinement (disclosed, not silent)

D-COMPOSE's early return means `y := producer(x); sink(y)` now binds `y` to
`callReturn(producer)` **instead of** param(x). Consequence: the v2.9
**over-approximated** transitive edge `x → sink.arg0` (param flowing *through* a
call return) is no longer produced; the direct, correct `x → producer.arg0` edge
is still recorded (verified by `TestAnalyzeFlow_InBody_CoexistWithParam`). This is
mandated by D-COMPOSE ("LHS origin = callReturn(C), NOT the inner args' origins")
and D-CASE1 ("no over-approximation; ambiguity ⇒ no origin"). It is invisible to
persisted state: `FlowSummary` is IN-MEMORY-ONLY (`fact.go`: "NEVER changes an
existing extractor golden"), and no committed test asserted the old transitive
behavior — all extract goldens pass byte-unchanged. The v2.9 `CallArgs`/`Returns`
records for direct param→target flows (the 6 already-working languages' emitted
edge set) are byte-preserved (8 pre-existing tests green).

Note on the CONTEXT vs REQUIREMENTS wording: FLOW-04d's requirements line mentions
"also record when a callReturn(P) origin reaches the return," but the CONTEXT
(D-BRIDGE) and the PLAN (step 5) both **lock this out** as dead scaffolding (no
v2.13 edge maps to it). I followed the authoritative CONTEXT/PLAN: callReturn-
reaches-return is NOT recorded.

## Acceptance checklist

1. `go build ./cmd/helix` — clean. `go vet ./internal/semantic/dataflow/...
   ./internal/semantic/extract/...` — clean.
2. `go test ./internal/semantic/dataflow/... ./internal/semantic/extract/...` —
   **13 packages ok** (1 no-test: testutil).
3. All-11 matrix — **11/11 PASS, 0 skip**.
4. Revert-and-fail — confirmed RED on both m2 guards, restored green (above).
5. `git diff --stat go.mod go.sum` — **empty** (zero new deps).
6. This SUMMARY written.
7. Working tree left dirty (not committed).

## Diff stat

```
internal/semantic/dataflow/dataflow_test.go        | 153 ++++++++++++++++++++
internal/semantic/dataflow/summary.go              | 164 +++++++++++++++------
internal/semantic/dataflow/dataflow_matrix_test.go | 122 +++++++++++++++ (new)
```

## Out of scope (Phase 139/140)

Edge emission, callee-node resolution, read surface, reachability, distinctness,
real-binary E2E, callReturn-reaches-return recording.
