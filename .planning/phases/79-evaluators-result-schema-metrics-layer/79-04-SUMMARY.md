---
phase: 79
plan: 04
subsystem: bench-evaluators-runtime
tags: [metrics, coordinator, result-schema, run-index, pre-patch-snapshot, tdd, d07]
requires:
  - "bench/evaluators.Metrics + MetricError (79-01)"
  - "the 5 graders: test_runner, patch_validator, token_meter, tool_trace_analyzer, regression_checker (79-02/79-03)"
  - "bench/runtime.{RunCell, BuildResult, Validate, validatePathSegment} (Phase 77)"
provides:
  - "bench/evaluators/coordinator.Grade(ctx, GradeInput) (Metrics, []MetricError) — D-07 per-metric failure isolation over all 5 graders"
  - "bench/runtime ResultInput.Metrics + .MetricErrors -> result.v2.json metrics + metric_errors (validate-on-write)"
  - "bench/runtime cellDurablePaths: <out>/<task>/<mode>/<run_index>/ durable layout (V5-guarded run_index segment)"
  - "bench/runtime prePatchSnapshot: pre-patch RunTests outcome feeding regression_checker"
  - "bench/evaluators/METRICS.md — edit_locality + regression_rate + edit_distance_patch definitions"
affects:
  - "Phase 79 pre-verify E2E gate (make bench-quick) consumes the metric-complete result.v2.json this plan assembles"
tech-stack:
  added: []
  patterns:
    - "coordinator as a fan-out pure-transform + exec delegation; never errors, never drops the row (D-07)"
    - "metrics object has NO omitempty: every nullable field always emitted (nil -> JSON null, D-07)"
    - "run_index path segment formatted via strconv.Itoa + guarded by validateRunIndexSegment->validatePathSegment (T-79-04-01)"
    - "pre-patch RunTests snapshot BEFORE driveScript (D-05/Pitfall 6)"
key-files:
  created:
    - bench/evaluators/coordinator/coordinator.go
    - bench/evaluators/coordinator/coordinator_test.go
    - bench/evaluators/METRICS.md
  modified:
    - bench/runtime/result.go
    - bench/runtime/result_test.go
    - bench/runtime/cell.go
    - bench/runtime/matrix.go
decisions:
  - "Coordinator lives in package coordinator (bench/evaluators/coordinator), NOT package evaluators, to avoid the grader->evaluators import cycle (Rule 3 blocking-issue resolution)"
  - "run_index path segment uses a numeric-segment wrapper validateRunIndexSegment (rejects negatives, then delegates to validatePathSegment) since strconv.Itoa(-1)='-1' would otherwise pass the string guard"
  - "Cell gains a RunIndex field so runOneCell threads the cell's actual run index (removes the matrix.go RunIndex:0 hard-code)"
  - "usagePresent = (Agent=='claude' AND merged.Usage has input/output tokens); the scripted corpus is usage-absent so its token metrics are explicit null (D-01)"
metrics:
  duration: "~10m"
  completed: "2026-06-18"
  tasks: 4
  files: 7
---

# Phase 79 Plan 04: Coordinator + result.v2 Wiring + run_index/Pre-Patch Runtime + METRICS.md Summary

The phase integration plan: a `coordinator.Grade` fan-out with D-07 per-metric
failure isolation, the `result.v2.json` builder extended to carry the full
nullable 17-metric (+2 cached-token) record and `metric_errors`, the `RunCell`
durable path threaded with a guarded `<run_index>` segment plus a pre-patch
`RunTests` snapshot, and `bench/evaluators/METRICS.md`. Together they assemble
the metric-complete `result.v2.json` the phase goal demands.

## What Was Built

### Task 1 — Coordinator (`bench/evaluators/coordinator`)
- `GradeInput{TestOutcome, PrePatchOutcome, RepoDir, Merged, UsagePresent, Agent, CompileErrorsBefore}`
- `Grade(ctx, in) (evaluators.Metrics, []evaluators.MetricError)` — fans out to all
  5 graders, assigns successful values, and on any grader's `*MetricError` leaves
  that metric nil + appends the annotation. Never returns a Go error, never
  short-circuits; the full `Metrics` is always returned (D-07).
- Lives in `package coordinator` (a sub-package) rather than `package evaluators`
  because all 5 grader subpackages import the parent `evaluators` package for the
  `Metrics`/`MetricError` contract — a coordinator in `package evaluators` that
  imported the graders would form an import cycle.

### Task 2a — result.v2 wiring (`bench/runtime/result.go`)
- `ResultInput` gains `Metrics evaluators.Metrics` + `MetricErrors []evaluators.MetricError`.
- `resultDoc` gains `Metrics` (`json:"metrics"`, **no** omitempty — always emitted,
  nil fields serialize to JSON `null`) + `MetricErrors` (`json:"metric_errors,omitempty"`).
- `BuildResult` assembles them; the existing verbatim jsonschema/v6 `Validate` gate
  now also gates the metrics object (reused unchanged).

### Task 2b — run_index path + pre-patch snapshot (`bench/runtime/cell.go`, `matrix.go`)
- `cellDurablePaths(out, task, mode, runIndex)` → `<out>/<task>/<mode>/<run_index>/{result.v2.json,trace.json}`.
- `validateRunIndexSegment` formats run_index via `strconv.Itoa` and rejects
  negatives, then delegates to the shared `validatePathSegment` (V5/T-79-04-01).
- `prePatchSnapshot(ctx, runner, repoDir)` runs Setup+RunTests BEFORE `driveScript`
  and threads the outcome into `coordinator.Grade` as `GradeInput.PrePatchOutcome`
  (D-05/Pitfall 6); a nil runner yields no snapshot (verify.sh fallback).
- `RunCell` calls `coordinator.Grade` after the post-patch outcome + merge, writing
  the returned `Metrics`+`MetricErrors` into `ResultInput`. `Analyze` consumes
  `merged` (res.Merged) directly — no re-merge (METRIC-06).
- `matrix.go`: `Cell` gains a `RunIndex` field; `runOneCell` threads `c.RunIndex`
  (the `RunIndex: 0` hard-code is removed).

### Task 3 — METRICS.md (`bench/evaluators/METRICS.md`)
- Documents all 19 metric fields with snake_case names matching the Go struct +
  schema, the grader-to-metric map, the D-07 isolation + scripted-null invariants,
  and the deep definitions for `edit_locality` (git-tracked denominator + edge
  cases), `regression_rate` (pre/post double-run + cached passing set + pre-existing
  failures excluded), and `edit_distance_patch` (`git diff --numstat` added+deleted).

## TDD Cycle (RED → GREEN)

| Task | RED commit(s) | GREEN commit | Run target |
|------|---------------|--------------|------------|
| 1 coordinator | `cab85858`, `43510c90` (relocate) | `29f606f4` | `TestPerMetricIsolation\|TestAllSeventeenMetrics` |
| 2a result.v2 | `da8cfc46` | `44f72d2d` | `TestResultMetricsRoundTrip` |
| 2b run_index + pre-patch | `2778e343` | `566514f7` | `TestRunIndexPathSegment\|TestRunCellPrePatchOrder` |
| 3 METRICS.md (auto) | — | `e82a70e9` | doc-existence + `go test ./bench/...` |

RED precedes GREEN for every TDD task in git history.

### Note on Task 1's two RED commits
The first RED commit (`cab85858`) placed the failing test in `package evaluators`.
On the GREEN attempt the grader→evaluators import cycle surfaced (all 5 graders
import the parent package), making a `package evaluators` coordinator unbuildable.
Resolved by relocating the (still-failing) test to `bench/evaluators/coordinator`
(`package coordinator`) in `43510c90` — still a RED commit (build-fail: undefined
`Grade`/`GradeInput`), preceding the GREEN `29f606f4`.

## TDD Gate Compliance
- RED (`test(...)`) and GREEN (`feat(...)`) commits present for Tasks 1, 2a, 2b,
  RED before GREEN. Task 3 is `type: auto` (doc + gate), exempt from RED.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Coordinator package placement (import cycle)**
- **Found during:** Task 1 GREEN.
- **Issue:** The plan frontmatter specified `bench/evaluators/evaluators.go`
  (`package evaluators`), but all 5 Wave-2 grader subpackages import the parent
  `evaluators` package for the `Metrics`/`MetricError` contract. A coordinator in
  `package evaluators` importing those graders forms an import cycle (unbuildable).
- **Fix:** Placed the coordinator in `bench/evaluators/coordinator`
  (`package coordinator`), which imports both the contract and the graders
  acyclically. Test relocated to the same package. The `must_haves` artifact path
  for the coordinator is therefore `bench/evaluators/coordinator/coordinator.go`
  rather than `bench/evaluators/evaluators.go`; all behavioral contracts
  (D-07 isolation, 5-grader fan-out, `GradeInput`/`Grade` signatures) are unchanged.
- **Files:** `bench/evaluators/coordinator/{coordinator.go,coordinator_test.go}`
- **Commits:** `43510c90`, `29f606f4`

No Rule 1/2/4 deviations.

## Acceptance Criteria Verification

- Task 1: `go test ./bench/evaluators/coordinator/ -count=1` passes (single + multi
  failure isolation, all-present). `grep -cE 'test_runner|patch_validator|token_meter|tool_trace_analyzer|regression_checker' coordinator.go` = 23 (all 5 referenced). RED before GREEN.
- Task 2a: `go test ./bench/runtime/ -run TestResultMetricsRoundTrip` passes; metrics
  object carries all 19 keys; a nil metric serializes to JSON `null`. `grep -n 'json:"metrics"' result.go` matches. RED before GREEN.
- Task 2b: `go test ./bench/runtime/ -count=1` passes. `grep -n 'RunIndex: 0' matrix.go`
  returns nothing. `validatePathSegment`/`validateRunIndexSegment` present in the cell.go path region. `RunTests` has two call sites (pre-patch via `prePatchSnapshot`, post-patch). RED before GREEN.
- Task 3: `bench/evaluators/METRICS.md` exists; `grep -q "edit_locality"` and
  `grep -q "regression_rate"` both succeed; `go vet ./...` clean; `go test ./bench/... -count=1` green.

## Durable-path shape (for the phase verifier)
`<OutDir>/<task>/<mode>/<run_index>/result.v2.json` and `.../trace.json`. The
`<run_index>` segment is formatted via `strconv.Itoa(cfg.RunIndex)` and guarded by
`validateRunIndexSegment` (rejects negatives + path-traversal) before the join.

## GradeInput contract (for the phase verifier)
```go
type GradeInput struct {
    TestOutcome         languages.TestOutcome // post-patch outcome (success oracle)
    PrePatchOutcome     languages.TestOutcome // pre-patch snapshot (regression denominator)
    RepoDir             string                // git-tracked denominator for patch_validator
    Merged              trace.MergedTrace     // already-merged; NEVER re-merged (METRIC-06)
    UsagePresent        bool                  // claude + provider usage block (D-01)
    Agent               string
    CompileErrorsBefore *int
}
func Grade(ctx context.Context, in GradeInput) (evaluators.Metrics, []evaluators.MetricError)
```

## make bench-quick E2E result
NOT run in this plan. `make bench-quick` is the **phase pre-verify E2E gate**
(per 79-VALIDATION.md / the plan `<verification>`), executed once by the
orchestrator before `/gsd-verify-work`, decoupled from this plan's deterministic
per-task commit gates. The per-task gates here (`go build ./cmd/helix`,
`go vet ./...`, `go test ./bench/...`) all pass.

## Verification
- `go build ./cmd/helix` — exit 0
- `go vet ./...` — clean
- `go test ./bench/... -count=1` — all packages green
- `grep -n 'RunIndex: 0' bench/runtime/matrix.go` — no match (hard-code removed)
- METRICS.md documents edit_locality + regression_rate + edit_distance_patch

## Self-Check: PASSED
All 7 key files exist on disk; all 8 task commits (4 RED + 3 GREEN + 1 docs) present in git history.
