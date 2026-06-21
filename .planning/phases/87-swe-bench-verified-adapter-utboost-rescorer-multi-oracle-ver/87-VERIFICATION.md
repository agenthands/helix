---
phase: 87-swe-bench-verified-adapter-utboost-rescorer-multi-oracle-ver
verified: 2026-06-21T00:00:00Z
status: passed
score: 4/4 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  note: initial verification (no prior VERIFICATION.md)
---

# Phase 87: SWE-bench Verified Adapter + UTBoost Rescorer + Multi-Oracle `verified_correctness` Verification Report

**Phase Goal:** SWE-bench Verified adapter via subprocess-shellout to upstream `python -m swebench.harness.run_evaluation`; raw + UTBoost-augmented rescored scores side-by-side; 3-condition `verified_correctness` (canonical AND augmented AND no-regress) computed INDEPENDENTLY of `task_success`.
**Verified:** 2026-06-21
**Status:** passed
**Re-verification:** No — initial verification

## Gate Results (run by the verifier, not SUMMARY claims)

| Gate | Command | Result |
| ---- | ------- | ------ |
| Build | `go build ./...` | exit 0 |
| Bench tests | `go test -count=1 ./bench/...` | all `ok` (incl. swebench, aggregator, swebench-utboost) |
| **SC#2 critical** | `go test ./bench/evaluators/swebench/ -run 'TestVerified_VacuousPass\|TestVerified_BuggyPatch'` | **both PASS** (3 subtests) |
| Vet | `make vet` | exit 0 (incl. 6 custom vettools) |
| VERIFIED.md gate | `make verify-verified-md` | exit 0 — all required sections present |

## Goal Achievement

### Observable Truths (Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 (ADAPTER-SWE-01) | SWE-bench Verified adapter via subprocess to upstream harness; predictions.jsonl producer; harness-JSON → result.v2 ingestion preserving container_id + exit_code | ✓ VERIFIED | `harness.go` fixed-argv `RunArgs` (allowlisted dataset/run_id/instance_id/path, no shell, strict env); `predictions.go WritePredictions` sorted+deterministic; `report.go` bounded untrusted parsers; `ingest.go Ingest` maps task_success←canonical.Resolved + carries container_id/*int exit_code into `result.v2`. Live 5-task smoke honestly `t.Skip`s on Docker/swebench absence (`harness_test.go:287,295`); hermetic argv/ingest/parse tests carry the proof. |
| 2 (VERIFIED-01) — **HEADLINE** | verified_correctness computed INDEPENDENTLY of task_success; buggy patch → task_success=true AND verified_correctness=false; 3-condition gate fail-closed; **CR-01 vacuous-pass FIXED** | ✓ VERIFIED | `verified.go Grade`: `passed()`=`len(Failure)==0 && len(Success)>0` (empty bucket NOT a pass), separate `noRegressBucket()`, fail-closed on `!PatchSuccessfullyApplied` and nil augmented. `TestVerified_BuggyPatch` PASS (task_success=true ∧ verified_correctness=false, distinct pointers). `TestVerified_VacuousPass` PASS (patch-not-applied→false; patch-applied-no-tests→false). Fixtures honest (`report.buggy_*`, `report.notapplied_*`, `report.notests_*`). |
| 3 (VERIFIED-02) | raw upstream + UTBoost-rescored score side-by-side (additive column, goldens byte-identical); reproducible per --run-id | ✓ VERIFIED | `rescore.go Rescore`/`ApplyToRow` stamps PINNED `runtime.SwebenchRawResolvedKey`/`SwebenchRescoredVerifiedKey`; `aggregate.go rowSwebenchScores`/`reduceSwebenchScores` reads SAME consts; `report.go` additive `RawScore`/`RescoredScore`. `TestRescore_ApplyToRow_RoundTrip` proves real producer→BuildResult→reader (no hand-injection). `TestSwebenchColumnsDivergence`/`AbsentIsEmDash`/`GoldenStable` PASS. |
| 4 | run-all-tests override; differential.go consumes gold patch alongside agent patch → diff-overlap | ✓ VERIFIED | `differential.go DiffOverlap` = \|gold∩agent\|/\|gold\| from `diff --git` headers; empty gold → nil+MetricError (never fabricated 0/1). `TestDifferential` PASS. Run-all-tests override sourced from UTBoost augmented FAIL_TO_PASS (condition (b)), documented in `differential.go` header + `VERIFIED.md`. |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `bench/evaluators/swebench/harness.go` | fixed-argv subprocess seam | ✓ VERIFIED | `RunArgs` total-validated, env allowlist, WR-03 `cmd.Dir=workDir`, WR-04 fail-closed PATH + DOCKER_* forwarding |
| `bench/evaluators/swebench/predictions.go` | sorted deterministic predictions.jsonl | ✓ VERIFIED | `WritePredictions` copies+sorts by instance_id, byte-identical reruns |
| `bench/evaluators/swebench/report.go` | bounded untrusted run/instance parsers | ✓ VERIFIED | 64 MiB size-cap before unmarshal; null-body rejection |
| `bench/evaluators/swebench/ingest.go` | task_success←resolved, carries container_id/exit_code | ✓ VERIFIED | distinct pointer; VerifiedCorrectness left nil (gate owns it) |
| `bench/evaluators/swebench/verified.go` | 3-condition gate, CR-01-safe | ✓ VERIFIED | non-empty-success gate buckets + patch-applied guard |
| `bench/evaluators/swebench/rescore.go` | raw vs rescored producer (ApplyToRow) | ✓ VERIFIED | stamps both pinned open keys; *bool preserves load-bearing false |
| `bench/evaluators/swebench/differential.go` | gold-vs-agent diff-overlap | ✓ VERIFIED | nil+MetricError on undefined denominator |
| `bench/evaluators/swebench/VERIFIED.md` | 3-condition acceptance doc | ✓ VERIFIED | Oracles/Conditions/Abstain/Differential/Proof headers; gated by `make verify-verified-md` |
| `bench/runtime/result.go` | additive container_id/exit_code + pinned consts | ✓ VERIFIED | omitempty fields + `Swebench*Key` consts; BuildResult propagates all 4 |
| `bench/schema/result.v2.schema.json` | optional container_id/exit_code | ✓ VERIFIED | additive-only, schema_version stays "v2", additionalProperties OPEN |
| `bench/aggregator/{aggregate,report}.go` | rowSwebenchScores + RawScore/RescoredScore | ✓ VERIFIED | reads pinned consts; zero RNG; goldens byte-stable |
| `bench/datasets/swebench-utboost/{pin,fetch}.go` | pin + bounded fetch | ✓ VERIFIED | WR-01 PinnedContentDigests + WR-02 readCacheCapped |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| `rescore.ApplyToRow` | `runtime.ResultInput` open keys | pinned `Swebench*Key`-tagged *bool fields | ✓ WIRED |
| `ResultInput` | `BuildResult`/`resultDoc` | `result.go:282-283` propagation | ✓ WIRED |
| `aggregate.rowSwebenchScores` | `Row.Doc` keys | reads SAME `runtime.Swebench*Key` consts | ✓ WIRED |
| producer ↔ reader | end-to-end | `TestRescore_ApplyToRow_RoundTrip` (no hand-injection) | ✓ WIRED |
| `ingest` | `report.canonical.json resolved` | `task_success ← canonical.Resolved` | ✓ WIRED |

### Code Review Resolution (87-REVIEW → 87-REVIEW-FIX)

| Finding | Severity | Status | Evidence |
|---------|----------|--------|----------|
| CR-01 vacuous-pass | CRITICAL | ✓ FIXED | `passed()` requires ≥1 success; `!PatchSuccessfullyApplied` fail-close; `TestVerified_VacuousPass` (2 subtests) PASS |
| WR-01 content-digest | WARNING | ✓ FIXED | `PinnedContentDigests` + `assertContentDigest` on network AND cache legs |
| WR-02 cache-cap | WARNING | ✓ FIXED | `readCacheCapped` os.Stat + LimitReader; `TestFetchCacheHitHonorsSizeCap` PASS |
| WR-03 cmd.Dir | WARNING | ✓ FIXED | `resolveWorkDir` + `cmd.Dir=workDir`; `TestResolveWorkDir` PASS |
| WR-04 env validation | WARNING | ✓ FIXED | `allowlistEnv` fail-close empty PATH + DOCKER_* forward; `TestAllowlistEnvFailsOnEmptyPath`/`ForwardsDocker` PASS |
| IN-01/IN-02/IN-03 | INFO | not in scope | IN-03 dependency satisfied (CR-01 fixed before any production caller; no external caller of Grade/ApplyToRow/Ingest exists today) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| SC#2 divergence (buggy patch) | `go test -run TestVerified_BuggyPatch` | task_success=true ∧ verified_correctness=false | ✓ PASS |
| SC#2 vacuous-pass closed (CR-01) | `go test -run TestVerified_VacuousPass` | both vacuous shapes → false | ✓ PASS |
| Producer↔reader round-trip | `go test -run TestRescore_ApplyToRow_RoundTrip` | present=true, raw=true, rescored=false | ✓ PASS |
| Raw-vs-rescored em-dash on absent | `go test -run TestSwebenchColumnsAbsentIsEmDash` | OK==false (no fabricated 0) | ✓ PASS |
| Goldens byte-stable | `go test -run TestSwebenchColumnsGoldenStable` | leaderboard/cost goldens unchanged | ✓ PASS |
| Live 5-task smoke | `go test -run TestLiveSmoke` | SKIP (Docker/swebench gated, honestly recorded) | ? SKIP (expected) |

### Probe Execution

No conventional `scripts/*/tests/probe-*.sh` declared for this phase; verification driven through `go test` + `make` targets (run above). N/A.

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|------------|--------|----------|
| ADAPTER-SWE-01 | 87-01, 87-02 | ✓ SATISFIED | harness subprocess + predictions producer + harness-JSON→result.v2 ingest; live smoke honestly Docker-gated |
| VERIFIED-01 | 87-03 | ✓ SATISFIED | 3-condition gate independent of task_success; buggy-patch divergence + CR-01 vacuous-pass both hermetic |
| VERIFIED-02 | 87-03, 87-04 | ✓ SATISFIED | raw-vs-rescored side-by-side column, additive, goldens byte-stable, reproducible per --run-id |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `bench/aggregator/report.go` | 78 | `"TBD"` token in comment | ℹ️ Info | FALSE POSITIVE — comment literally reads `(the REAL producer — NOT "TBD")`; no debt |
| `.planning/REQUIREMENTS.md` | 194 | traceability table row `ADAPTER-SWE-01 \| Phase 87 \| TBD \| Pending` | ℹ️ Info | Doc-bookkeeping drift: the checklist (line 87) marks ADAPTER-SWE-01 `[x]` complete and code is fully implemented; only the table row lags. Not a goal-achievement gap. |

No stubs, placeholders, empty-return-only handlers, or unresolved debt markers in phase-87 source. All phase-87 files committed.

### Human Verification Required

None. SC#1/SC#3 decision logic is hermetically proven and the live confirmation is honestly recorded as Docker-gated (the planner deferred the live 5-task smoke to a Docker+swebench+network host as an explicit `t.Skip`, not a failure). SC#2 is hermetically proven via `TestVerified_BuggyPatch` + `TestVerified_VacuousPass`. No `<verify><human-check>` blocks were deferred to end-of-phase requiring a human decision.

### Gaps Summary

No blocking gaps. The phase goal is achieved in the live codebase:

- The headline integrity claim (SC#2 / VERIFIED-01) is hermetically proven and the
  CRITICAL CR-01 vacuous-pass false-positive — the single worst benchmark failure the
  phase exists to prevent — is fixed and regression-locked: a zero-tests-run or
  patch-not-applied instance can NEVER yield `verified_correctness=true`.
- All 4 code-review warnings (WR-01..04) are fixed with hermetic regression coverage and
  no SC regression.
- Two informational doc nits (a false-positive "TBD" inside a comment, and a stale
  REQUIREMENTS.md traceability-table row for ADAPTER-SWE-01) do not affect goal
  achievement and are noted for bookkeeping cleanup.

---

_Verified: 2026-06-21_
_Verifier: Claude (gsd-verifier)_
