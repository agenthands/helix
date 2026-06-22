---
phase: 87-swe-bench-verified-adapter-utboost-rescorer-multi-oracle-ver
plan: 03
subsystem: testing
tags: [swe-bench, verified-correctness, utboost-rescorer, differential, multi-oracle-gate, fail-closed, hermetic-fixture, bench, go]

# Dependency graph
requires:
  - phase: 87-02
    provides: "swebench InstanceEval/TestsStatus/TestList report structs + ParseInstanceReport; Ingest sets task_success<-resolved, leaves VerifiedCorrectness nil (this plan is its sole producer)"
  - phase: 87-01
    provides: "runtime.SwebenchRawResolvedKey / SwebenchRescoredVerifiedKey pinned open-key-name consts in bench/runtime/result.go"
  - phase: 86
    provides: "bench/evaluators/completion_gate.Grade/GateResult/ApplyToMetrics — the EXACT gate cloned here (fail-closed abstain, all-three-required, additive producer)"
  - phase: 79
    provides: "evaluators.Metrics nullable-pointer contract (TaskSuccess/VerifiedCorrectness distinct *bool); regression_checker null+MetricError-on-empty-denominator discipline"
provides:
  - "bench/evaluators/swebench.Grade / GateResult / ApplyToMetrics — 3-condition test-execution verified_correctness gate (canonical ∧ augmented ∧ no-regress), fail-closed abstain, independent of task_success"
  - "bench/evaluators/swebench.DiffOverlap — gold-vs-agent diff-overlap signal (pure text over diff --git headers; empty gold -> nil+MetricError)"
  - "bench/evaluators/swebench.Rescore / RescoreResult / ApplyToRow — raw-vs-rescored distinct-pointer verdicts + result-row producer stamping the two pinned open keys"
  - "runtime.ResultInput.SwebenchRawResolved / SwebenchRescoredVerified (*bool) + resultDoc projection (additive-open, omitempty, json tags == pinned const values)"
  - "bench/evaluators/swebench/VERIFIED.md + extended verify-verified-md Makefile gate (now gates BOTH VERIFIED.md files)"
  - "committed fixtures: report.utboost.json (all-aug-pass), report.buggy_canonical.json + report.buggy_utboost.json (SC#2 divergence)"
affects: [87-04, 87-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "3-condition test-execution gate as a verbatim structural clone of completion_gate.Grade: swap 3 completion oracles for 3 tests_status conditions, KEEP fail-closed abstain branch (&false, oracles not consulted) and independent-local binding (no pointer aliasing)"
    - "verified_correctness computed INDEPENDENTLY of task_success (distinct pointers): task_success<-canonical.Resolved (Plan 02), verified<-Grade(canonical,augmented).VerifiedCorrectness — the VERIFIED-01 divergence proven hermetically by TestVerified_BuggyPatch"
    - "fail-closed abstain: nil augmented report -> explicit non-nil &false (never nil, never a spurious true) — a false-positive correctness verdict is the security-relevant benchmark failure"
    - "diff-overlap as pure text: parse diff --git a/ b/ headers -> touched-file sets, |gold ∩ agent|/|gold|; empty/headerless gold -> nil+MetricError (clone regression_checker null discipline); zero overlap is a legitimate 0.0"
    - "result-row producer (ApplyToRow) stamps two pinned open doc keys via runtime.ResultInput *bool fields whose json tags == the pinned const VALUES; *bool omitempty PRESERVES a load-bearing false (the SC#2 rescored verdict) where a value-type omitempty would drop it (Pitfall 2)"
    - "producer↔reader wiring proven end-to-end: TestRescore_ApplyToRow_RoundTrip stamps a row, BuildResult+Validate, then drives Plan 04's rowSwebenchScores reader -> raw/rescored present=true (no hand-injected key, shared pinned consts the drift guard)"
    - "verify-verified-md Makefile gate refactored to a shell check() function gating BOTH VERIFIED.md files fail-closed (non-zero on a missing required header)"

key-files:
  created:
    - bench/evaluators/swebench/verified.go
    - bench/evaluators/swebench/verified_test.go
    - bench/evaluators/swebench/differential.go
    - bench/evaluators/swebench/differential_test.go
    - bench/evaluators/swebench/rescore.go
    - bench/evaluators/swebench/rescore_test.go
    - bench/evaluators/swebench/VERIFIED.md
    - bench/evaluators/swebench/testdata/report.utboost.json
    - bench/evaluators/swebench/testdata/report.buggy_canonical.json
    - bench/evaluators/swebench/testdata/report.buggy_utboost.json
    - bench/aggregator/swebench_roundtrip_test.go
  modified:
    - bench/runtime/result.go
    - Makefile

decisions:
  - "verified.go emits NO MetricError today (each condition is a boolean over len(.failure)==0), so the completion_gate graderName const was dropped from the clone; GateResult.Errs preserves the future could-not-score path"
  - "result.go struct json tags MUST be literal strings (Go cannot reference a const in a struct tag); the literal == the pinned const VALUE, and TestRescore_ApplyToRow_RoundTrip is the drift guard (it reads via runtime.SwebenchRawResolvedKey, so a tag/const divergence fails the round-trip)"
  - "the round-trip test lives in package aggregator (not swebench) so it can drive the unexported rowSwebenchScores reader directly against the real stamped+validated row"
  - "swebench VERIFIED.md required headers chosen as Oracles/Conditions/Abstain/Differential/Proof (Threshold/Tokenizer/EditSimilarity are completion-gate-specific and N/A for a boolean test-execution gate)"

metrics:
  duration_minutes: 22
  completed: 2026-06-21
  tasks_completed: 4
  files_created: 11
  files_modified: 2
  commits: 8
---

# Phase 87 Plan 03: 3-Condition verified_correctness Gate + UTBoost Rescorer + Differential Summary

The load-bearing VERIFIED-01/02 honesty slice: a 3-condition test-execution
`verified_correctness` gate (clone of `completion_gate.Grade`) composing
canonical-pass ∧ augmented-pass ∧ no-regress directly off the harness reports,
computed INDEPENDENTLY of `task_success`; the UTBoost rescorer surfacing raw vs
rescored verdicts side-by-side and stamping them onto the result row's two pinned
open keys (the LIVE-run wiring the Plan 04 aggregator reads); a gold-vs-agent
diff-overlap signal; and a `bench/evaluators/swebench/VERIFIED.md` gated by the
extended `verify-verified-md` target. The ★ hermetic `TestVerified_BuggyPatch`
proves a known-buggy patch lands `task_success=true` AND
`verified_correctness=false`.

## RED → GREEN per feature

### Feature 1 — 3-condition verified gate (SC#2 load-bearing)
- **RED** (`90e...` test commit): `TestVerified_BuggyPatch` + `TestVerified_Gate`
  + `TestVerified_ApplyToMetrics` against undefined `Grade`/`GateResult`; committed
  the three fixtures (`report.utboost.json`, `report.buggy_canonical.json`,
  `report.buggy_utboost.json`).
- **GREEN**: `verified.go` clones `completion_gate.Grade` — swaps the 3 completion
  oracles for `canonicalPass`/`augmentedPass`/`noRegress` read off `tests_status`,
  KEEPS the fail-closed abstain branch (`nil augmented -> &false`, oracles not
  consulted) and the independent-local binding. `ApplyToMetrics` mutates ONLY
  `VerifiedCorrectness`.

### Feature 2 — gold-vs-agent diff-overlap (SC#4)
- **RED**: `TestDifferential` (full/partial/zero overlap; empty + headerless gold
  -> nil+MetricError) against undefined `DiffOverlap`.
- **GREEN**: `differential.go` parses `diff --git a/ b/` headers via stdlib
  bufio -> touched-file sets, `|gold ∩ agent| / |gold|`; empty/headerless gold ->
  `(nil, *MetricError{Grader:"swebench_differential", Metric:"diff_overlap"})`.

### Feature 3 — UTBoost rescorer + ApplyToRow producer (VERIFIED-02)
- **RED**: `TestRescore` + `TestRescore_ApplyToRow` (swebench pkg) and
  `TestRescore_ApplyToRow_RoundTrip` (aggregator pkg) against undefined `Rescore`
  and missing `ResultInput.Swebench*` fields.
- **GREEN**: added `SwebenchRawResolved`/`SwebenchRescoredVerified` (*bool,
  omitempty, tags == pinned const values) to `ResultInput`+`resultDoc`+`BuildResult`;
  `rescore.go` `Rescore` returns raw (`canonical.Resolved`) and rescored
  (`Grade(...).VerifiedCorrectness`) as DISTINCT pointers; `ApplyToRow` stamps both
  onto the row. The round-trip drives the real Plan 04 `rowSwebenchScores` reader.

### Feature 4 — VERIFIED.md + extended Makefile gate
- `bench/evaluators/swebench/VERIFIED.md` documents the 3-condition contract (Oracles
  / Conditions / Abstain / Differential / Proof) + the SC#2 divergence.
- `verify-verified-md` refactored to a `check()` shell function gating BOTH
  VERIFIED.md files; verified fail-closed (a sed-mangled header -> exit 2).

## ★ SC#2 proof
`TestVerified_BuggyPatch`: `report.buggy_canonical.json` (`resolved=true`,
canonical `FAIL_TO_PASS.failure=[]`) + `report.buggy_utboost.json` (augmented
`FAIL_TO_PASS.failure=["test_utboost_aug_overflow"]`) →
`task_success == true` AND `verified_correctness == false`, with the two verdicts
distinct pointers. Hermetic — no Docker, no `HELIX_BIN`, no subprocess.

## Deviations from Plan

None — plan executed as written. The verified gate's `MetricError` graderName const
was omitted because the gate emits no MetricError today (each condition is a pure
boolean); `GateResult.Errs` still preserves the reserved could-not-score path. This
is a faithful narrowing of the clone, not a behavioral deviation.

## Verification
- `go build ./...` — clean
- `go vet ./bench/...` + `make vet` (all custom singlecheckers) — clean
- `go test ./bench/...` — all pass; smoke tests SKIP cleanly (no `HELIX_BIN`)
- ★ `TestVerified_BuggyPatch` PASS · ★ `TestRescore_ApplyToRow_RoundTrip` PASS
- `make verify-verified-md` — both VERIFIED.md files gated, fail-closed verified
- Phase 86 leaderboard/cost goldens unchanged (additive open keys, no render path)

## Self-Check: PASSED
- All 11 created files + 2 modified files present on disk.
- All 8 commits (4 test/RED, 3 feat/GREEN, 1 docs) present in `git log`.
