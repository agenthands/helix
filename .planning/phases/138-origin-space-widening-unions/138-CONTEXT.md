---
author: architect
responsible: architect
phase: 138
milestone: v2.13
status: planned
phase_type: implementation
hard_bar: true
security_relevant: false
design_fork: false
parent_artifacts:
  - .planning/milestones/v2.13-ROADMAP.md
  - .planning/milestones/v2.13-REQUIREMENTS.md
  - agent://RedTeamV213
---

# Phase 138 — Origin-space widening + kind-union extension + all-11 unit verification

## Goal (the contract Q this phase establishes)

The `dataflow` leaf engine tracks **in-body origins** — the return value of an in-body
call (`y := producer(); sink(y)`) — in addition to v2.9 param origins, records
producer→consumer in-body flows + per-param reaches-return, **deterministically**, and
does so across **all 11 grammars**. Proven with pure tree-sitter unit tests including a
cheap all-11 matrix. **No emission** (that is Phase 139).

**Hoare frame:** `{fn is a fingerprintable function/method body}` `AnalyzeFlow`
`{Summary holds, per param, the v2.9 {reaches-return, reaches-call} set UNCHANGED, PLUS
InBodyFlows = the exact set of callReturn(P)→consumer.argPos in-body dependences derivable
from case-1 syntactic def-use — no over-approximation; ambiguity ⇒ no origin}`.

## Scope (files)

- `internal/semantic/dataflow/summary.go` (leaf pkg; stdlib + tree-sitter only) — widen
  origin space, D-COMPOSE assignment unwrap, in-body flow recording, union extension.
- `internal/semantic/dataflow/dataflow_test.go` (+ a new all-11 matrix test file) — the
  new fixtures, m2 RED guards, anti-vacuity, determinism, all-11 matrix.
- `internal/semantic/extract/fact.go` / `fingerprint.go` — carry the new summary fields
  (already `*dataflow.Summary`; the widened struct flows through unchanged — verify, don't
  re-plumb).

## Decisions locked (co-driver + red-team fold; grounded at HEAD)

- **D-COMPOSE (FLOW-04b + m2):** when an assignment RHS **unwraps** — via the STRICT
  whitelist `{expression_list, parenthesized_expression}` ONLY — to a call `C(...)`, the
  LHS receives origin `callReturn(C)` (outermost callee, last-segment idiom). NEVER descend
  into `selector_expression` / `member_expression` / index / binary / a multi-call
  expression list. `y := producer().field` ⇒ NO in-body origin; `a, b := f(), g()` ⇒ NO
  origin; `y := wrap(producer())` ⇒ `callReturn(wrap)` ONLY.
- **D-ANCHOR:** an in-body origin's source identity is the **producer function node** (a
  return value has no distinct node). Recorded as the producer callee *name*; resolved to a
  node at emission (Phase 139).
- **D-BRIDGE (FLOW-04d):** reclaim the currently-dead `ParamFlow.Returns` computation as
  the per-param reaches-return signal (emitted as the return-bridge in Phase 139). The
  callReturn-reaches-return case is **NOT recorded** — it maps to no emitted edge in v2.13
  (the co-driver locked exactly two markers) and an unconsumed field is dead scaffolding.
- **D-CASE1:** exact syntactic data dependence only; no over-approximation.
- **D-INVARIANTS:** zero new Go deps; leaf boundary intact; the v2.9 param→param
  (`CallArgs`/`Returns`) semantics for the 6 already-working languages are **byte-unchanged**;
  determinism = no Go-map iteration in the output path (dedup+sorted records).

## Load-bearing correction found at grounding (pin it)

`AnalyzeFlow` returns nil early when `len(paramNames)==0` (`summary.go:98-100`). A
**param-less** caller — `func c(){ a := producer(); sink(a) }` — has in-body flows but no
param flows; it MUST NOT bail. The early return must relax: always walk the body; return
nil only when BOTH the param-target set AND `InBodyFlows` are empty. (This is the correct
generalization of the v2.9 anti-vacuity gate, not a loosening of it.)

## Anti-vacuity (the invariant that makes this not theater — FLOW-04g)

- `f(){ y := producer(); return }` with `y` unused ⇒ **no** in-body flow (dead local).
- `f(x){ sink(x) }` (pure param) ⇒ **no** in-body flow (only a v2.9 param flow).
Revert-and-fail RED: if the analyzer reports an in-body flow for a dead local or a pure
param, the guard goes RED.

## The breadth risk this phase retires (B1 / M5 — why this is first)

The tree-sitter kind unions (`summary.go:54-83`) cover only 6/11 languages today. Python,
Ruby, Kotlin, C, C++ do NOT record the in-body flow (Ruby records **nothing** since v2.9 —
its params parse as `method_parameters ∉ paramListKinds`). This phase extends the unions
(FLOW-04i) and proves breadth with a **cheap all-11 pure-unit matrix** — so breadth risk
surfaces here, at the cheapest phase, not at the expensive Phase 140.

## Acceptance

1. **FLOW-04h fixtures (Go, real provider):** single in-body (`y:=producer();sink(y)` →
   `InBodyFlow{producer→sink.arg0}`); in-body + param coexisting (`y:=producer(x);sink(y)`
   → InBodyFlow AND v2.9 `x→producer.arg0`); 2-hop chain (`a:=producer();b:=transform(a);
   sink(b)` → two InBodyFlows).
2. **m2 RED guards:** nested-wrap positive (`callReturn(wrap)` only); selector-RHS negative;
   multi-call-RHS negative.
3. **Anti-vacuity (FLOW-04g):** dead local ⇒ none; pure param ⇒ none. Revert-and-fail RED.
4. **All-11 matrix (M5):** `AnalyzeFlow` on `local := producer(); sink(local)` in all 11
   grammars records the InBodyFlow — OR a language is xfailed with a **recorded reason**
   (the honest breadth ledger starts here). Ruby's `method_parameters` now recognized
   (fixes the latent v2.9 zero-output).
5. **Determinism:** re-extract ⇒ byte-identical summary.
6. Zero new deps; leaf boundary intact; `make vet` clean; `go test
   ./internal/semantic/dataflow/... ./internal/semantic/extract/...` green.

## Out of scope

- Any edge emission / callee-node resolution / read surface (Phase 139).
- Multi-hop reachability proof, distinctness guard (Phase 139).
- Real-binary E2E (Phase 140).
- callReturn-reaches-return recording / any function→function edge (locked out: two markers).
