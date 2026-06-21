# VERIFIED.md — SWE-bench 3-Condition `verified_correctness` Gate (VERIFIED-01/02)

This document is the acceptance artifact for the SWE-bench **test-execution**
`verified_correctness` gate implemented in `bench/evaluators/swebench/`. It
records the exact contract the gate enforces so the documented semantics stay
honest. The `verify-verified-md` Makefile target fails closed (non-zero exit) if
any of the required section headers below is missing.

This gate is the **test-bearing** producer of `verified_correctness`: where the
sibling `bench/evaluators/completion_gate/` gate derives the metric from three
CrossCodeEval *completion* oracles, THIS gate derives it from three SWE-bench
*test-execution* oracles read DIRECTLY off the harness reports. It is an
**additive** producer of the existing `evaluators.Metrics.VerifiedCorrectness
*bool` (metrics.go:21) — no `result.v2.schema.json` field is added.

`verified_correctness` is computed **INDEPENDENTLY of `task_success`**.
`task_success` is the upstream harness's own `report.resolved` bool (set by the
Plan 02 `Ingest`); `verified_correctness` is the 3-condition gate verdict over the
canonical + UTBoost-augmented reports. They are **distinct pointers** — never
aliased, never copied one from the other.

## Oracles

`verified_correctness = true` requires **ALL THREE** test-execution conditions to
hold (all-three-required; one failing condition drives the composite to an
explicit `false`). The three conditions are read directly off the harness
`tests_status` buckets:

| Condition | Source | Pass condition |
|-----------|--------|----------------|
| (a) canonical-pass | canonical `report.tests_status.FAIL_TO_PASS` | `.failure` is empty (all canonical issue tests pass) |
| (b) augmented-pass | UTBoost-augmented `report.tests_status.FAIL_TO_PASS` | augmented report present AND `.failure` is empty |
| (c) no-regress | canonical `report.tests_status.PASS_TO_PASS` + `PASS_TO_FAIL` | both `.failure` lists empty (no pre-existing test regressed) |

The composite rule, verbatim from `swebench.Grade`:

```
verified_correctness = canonicalPass
    AND augmentedPass
    AND noRegress
```

A single passing condition never masquerades as full verification: the
`TestVerified_Gate` matrix asserts `verified_correctness == false` when any one
condition fails (canonical FAIL_TO_PASS failure, augmented FAIL_TO_PASS failure,
PASS_TO_PASS regress, or PASS_TO_FAIL regress).

## Conditions

The gate is a structural CLONE of `completion_gate.Grade`: the three completion
oracles (exact-match / edit-similarity / identifier-match) are swapped for the
three test-execution conditions above, and each per-condition outcome is surfaced
as a DISTINCT `*bool` (`CanonicalPass` / `AugmentedPass` / `NoRegress`) so the
composite verdict is fully auditable. The composite is bound to an independent
local (`okv`) so no pointer aliases another (clone of `gate.go:129-136`).

Unlike the completion gate there is **no configurable threshold knob**: a
test-execution bucket either has failures or it does not, so each condition is a
boolean over `len(.failure) == 0`. The augmented (UTBoost) dataset's broader
`FAIL_TO_PASS` set IS the run-all-tests override — it exercises the patch against
more than just the PR-modified tests, which is exactly what condition (b)
consumes.

## Abstain

When the UTBoost-augmented report is **missing** (nil), the gate fails **closed**:
`Grade(canonical, nil)` returns `verified_correctness = &false` — a real, non-nil
`*bool` pointing to `false` — WITHOUT consulting the conditions (the per-condition
pointers stay nil).

> A missing augmented oracle is an abstain → `verified_correctness=&false` — never
> a false-positive `true`, never a nil drop.

This is the security-relevant invariant (VERIFIED-01 / T-87-03-01): a
false-positive `verified_correctness=true` is the worst failure a benchmark can
produce, so the gate prefers an explicit `false` to any risk of a spurious `true`,
and it never nil-drops the metric on this path. The clone is verbatim of the
`completion_gate.Grade` abstain branch (`gate.go:104-108`).

## Differential

`swebench.DiffOverlap(goldPatch, agentPatch)` reports the gold-vs-agent
diff-overlap signal (SC#4): `|gold_files ∩ agent_files| / |gold_files|` over the
`diff --git a/<f> b/<f>` headers of the two unified diffs — a PURE text transform
that touches NO working tree. The gold patch (the upstream instance row's
reference fix) is the denominator; an **empty / headerless gold** is an undefined
denominator and is reported as `(nil, *MetricError)` — never a fabricated `0` or
`1` (clone of `regression_checker`'s null+MetricError discipline). A zero overlap
where the agent simply edited elsewhere is a legitimate `0.0`, NOT an error.

The complementary run-all-tests signal lives in condition (b) above: the
augmented dataset's broader test sets are the run-all override, and
`DiffOverlap` answers the orthogonal "did the agent edit where the gold fix
edited" question.

## Proof

The hermetic fixture tests in `bench/evaluators/swebench/verified_test.go`,
`differential_test.go`, and `rescore_test.go` (plus the
`bench/aggregator/swebench_roundtrip_test.go` round-trip) are the SOLE
authoritative proof of this gate — **no Docker, no `HELIX_BIN`, no subprocess**.
The committed fixtures `report.buggy_canonical.json` (resolved=true, canonical
`FAIL_TO_PASS` all pass) + `report.buggy_utboost.json` (augmented `FAIL_TO_PASS`
has a failure) drive the ★ load-bearing `TestVerified_BuggyPatch`:

> a known-buggy patch that passes ONLY the canonical tests lands
> `task_success=true` AND `verified_correctness=false`.

This is the reason the phase exists — a false-positive correctness verdict is the
security-relevant benchmark failure, proven hermetically and never via the live
Docker smoke (which SKIPs cleanly when Docker / swebench are absent). The
`TestRescore_ApplyToRow_RoundTrip` test additionally proves a stamped row
round-trips through the Plan 04 aggregator's `rowSwebenchScores` reader with
`raw`/`rescored` present, closing the producer↔reader wiring gap via the shared
`bench/runtime` pinned key consts. Run them with
`go test ./bench/evaluators/swebench/... ./bench/aggregator/...`.
