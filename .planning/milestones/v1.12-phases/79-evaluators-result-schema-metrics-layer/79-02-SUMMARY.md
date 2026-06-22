---
phase: 79
plan: 02
subsystem: bench-evaluators
tags: [metrics, evaluators, tdd, git, regression, edit-locality]
requires:
  - "bench/evaluators.Metrics + MetricError (Plan 79-01)"
  - "bench/languages.TestOutcome / TestResult (prior phase)"
provides:
  - "bench/evaluators/test_runner.Grade — task_success/verified_correctness/compile_errors_before/after"
  - "bench/evaluators/patch_validator.EditLocality + EditDistancePatch — files_modified/edit_locality/edit_distance_patch"
  - "bench/evaluators/regression_checker.RegressionRate — regression_rate"
affects:
  - "Plan 79-04 coordinator wiring (RunCell): threads pre-patch outcomes + repoDir into these graders"
tech-stack:
  added: []
  patterns:
    - "exec.CommandContext + fixed argv + cmd.Dir=repoDir (no shell) for all git invocations"
    - "exit-code-authoritative success gate (TestOutcome.Passed), never len(Tests) (Pitfall 4)"
    - "per-metric *evaluators.MetricError isolation (D-07)"
key-files:
  created:
    - bench/evaluators/test_runner/test_runner.go
    - bench/evaluators/test_runner/test_runner_test.go
    - bench/evaluators/patch_validator/patch_validator.go
    - bench/evaluators/patch_validator/patch_validator_test.go
    - bench/evaluators/regression_checker/regression_checker.go
    - bench/evaluators/regression_checker/regression_checker_test.go
  modified: []
decisions:
  - "edit_distance_patch defined as sum(added+deleted) from `git diff --numstat` (binary files → 0)"
  - "regression numerator counts cached-passing members now failing OR absent post-patch"
metrics:
  duration: "~25m"
  completed: "2026-06-18"
  tasks: 3
  files: 6
---

# Phase 79 Plan 02: Exec/Filesystem Graders Summary

Three thin TDD graders over existing artifacts — `test_runner` (exit-authoritative success + compile-error counts), `patch_validator` (git-tracked `edit_locality` + `edit_distance_patch`), and `regression_checker` (pre/post double-run `regression_rate`) — each returning typed `(value, *evaluators.MetricError)` for D-07 per-metric isolation.

## What Was Built

### test_runner (`bench/evaluators/test_runner`)
- `Grade(post languages.TestOutcome, pre *languages.TestOutcome) Result`
- `Result{TaskSuccess, VerifiedCorrectness, CompileErrorsBefore, CompileErrorsAfter *... ; Errs []evaluators.MetricError}`
- `task_success` / `verified_correctness` ← `post.Passed` (exit==0). `len(Tests)` is consulted ONLY in `compileErrorCount` to split compile-failure (non-zero exit + empty rows → 1) from test-failure (non-zero exit + rows → 0); it never gates success (Pitfall 4).
- `compile_errors_before` derived from the optional pre-patch outcome (nil when not supplied).

### patch_validator (`bench/evaluators/patch_validator`)
- `EditLocality(ctx, repoDir) (*float64, *int, *evaluators.MetricError)` — denominator = `git ls-files` (tracked only, D-04), numerator = `git diff --name-only` ∩ tracked ∩ under-repoDir; `edit_locality = 1 − modified/total`. Zero tracked → nil + MetricError. Untracked files excluded from denominator.
- `EditDistancePatch(ctx, repoDir) (*int, *evaluators.MetricError)` — `sum(added+deleted)` over `git diff --numstat`; binary/malformed lines contribute 0.
- All git via `exec.CommandContext` with fixed argv + `cmd.Dir = repoDir`, no shell (T-79-02-01). `underRepo` enforces the path-prefix invariant (T-79-02-02).

### regression_checker (`bench/evaluators/regression_checker`)
- `RegressionRate(prePatch, postPatch languages.TestOutcome) (*float64, *evaluators.MetricError)` — pure transform. Denominator = cached pre-patch passing set keyed on `(Package, Name)`; numerator = cached-passing members not passing post-patch (failing OR absent). Pre-existing failures excluded (Pitfall 6). Empty passing set → nil + MetricError.

## Exported Signatures (for Plan 04 wiring)

```go
// package test_runner
func Grade(post languages.TestOutcome, pre *languages.TestOutcome) Result
type Result struct {
    TaskSuccess         *bool
    VerifiedCorrectness *bool
    CompileErrorsBefore *int
    CompileErrorsAfter  *int
    Errs                []evaluators.MetricError
}

// package patch_validator
func EditLocality(ctx context.Context, repoDir string) (*float64, *int, *evaluators.MetricError)
func EditDistancePatch(ctx context.Context, repoDir string) (*int, *evaluators.MetricError)

// package regression_checker
func RegressionRate(prePatch, postPatch languages.TestOutcome) (*float64, *evaluators.MetricError)
```

`edit_distance_patch` definition for Plan 04's METRICS.md: **sum of added + deleted lines from `git diff --numstat`** across the working tree; binary files contribute 0.

## TDD Cycle (per task)

| Task | RED commit | GREEN commit | Run target |
|------|-----------|--------------|-----------|
| 1 test_runner | `0eed7c3d` | `0473f624` | `TestRunnerSuccessGate` |
| 2 patch_validator | `2c1aef4b` | `ee714800` | `TestEditLocality\|TestEditDistancePatch` |
| 3 regression_checker | `a4b055cc` | `0d58a041` | `TestRegressionRate` |

Each RED commit (`test(...)`) precedes its GREEN commit (`feat(...)`) in git history. REFACTOR: patch_validator's `gitLines` error branch was simplified before its GREEN commit (no behavior change). No standalone refactor commits.

## TDD Gate Compliance
- RED gate (`test(...)`) and GREEN gate (`feat(...)`) commits present for all three tasks, RED before GREEN.

## Deviations from Plan

None of Rules 1–4 triggered. One discretionary implementation choice (allowed by the plan's "signature at executor's discretion"): the regression numerator counts a cached-passing test as regressed when it is failing OR absent post-patch (a disappeared test is no longer passing). This is a superset-safe reading of D-05 and does not affect any plan-specified fixture (all fixtures keep the same row set pre/post).

## Verification

- `go build ./cmd/helix` — exit 0
- `go vet ./...` — clean
- `go test ./bench/... -count=1` — all packages pass (test_runner, patch_validator, regression_checker, plus existing bench packages)
- `grep -rnE 'sh.*-c|bash.*-c' bench/evaluators/{test_runner,patch_validator,regression_checker}/` — no matches (no shell)
- success gate grep: `len(.*Tests` appears only in `patch_validator`-unrelated `compileErrorCount`, never in success-gating

## Self-Check: PASSED

All 6 source files and the SUMMARY exist on disk; all 6 task commits (3 RED + 3 GREEN) present in git history.
