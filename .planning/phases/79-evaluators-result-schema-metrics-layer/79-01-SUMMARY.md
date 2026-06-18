---
phase: 79
plan: 01
subsystem: bench-evaluators
tags: [metrics, schema, contract, tdd, nullable, fair-03]
requires: []
provides:
  - "evaluators.Metrics (Go typed nullable metric contract, 19 pointer fields)"
  - "evaluators.MetricError (Go per-metric grader-failure annotation)"
  - "result.v2.schema.json metrics object (17 metrics + 2 cached columns, nullable)"
  - "result.v2.schema.json metric_errors[] array"
affects:
  - "bench/evaluators/* (Wave-2 graders build against Metrics)"
  - "bench/runtime/* (Wave-3 wiring marshals Metrics, validates against schema)"
tech-stack:
  added: []
  patterns:
    - "nullable *T fields, no omitempty -> nil marshals to explicit JSON null (eval_result.go ContextPrecision/ContextRecall analog)"
    - "snake_case json tag <-> schema property-name parity"
    - "additive-only=minor schema policy (Phase 75 D-03/D-04): relax not tighten, additionalProperties open, no v3"
key-files:
  created:
    - bench/evaluators/metrics.go
    - bench/evaluators/metrics_test.go
  modified:
    - bench/schema/result.v2.schema.json
    - bench/schema/result.v2_test.go
decisions:
  - "Metrics has 19 pointer fields (17 METRIC-01/02 metrics + 2 FAIR-03 cached-token columns), authoritative per plan <artifacts_this_phase_produces>; the 17-field PATTERNS snippet was a subset"
  - "Top-level tokens_input/tokens_output RELAXED to [\"integer\",\"null\"] (Open Q5, lowest-risk additive move); metrics object is the canonical home"
  - "no omitempty anywhere in metrics.go; doc comment avoids the literal token 'omitempty' so the acceptance grep returns 0"
metrics:
  duration: ~9min
  tasks: 2
  files: 4
  completed: "2026-06-18"
---

# Phase 79 Plan 01: Evaluators Metrics & Result-Schema Contract Summary

Typed nullable `Metrics`/`MetricError` Go contract plus the additive typing of all 17 metrics (+ 2 cached-token columns) under a nullable `metrics` object and a `metric_errors[]` array in `result.v2.schema.json` — the contract-first foundation every Wave-2 grader and Wave-3 runtime wiring consumes.

## What This Plan Delivered

Made METRIC-01's "explicit nulls, not omissions" and D-06/D-07 (typed nullable fields + per-metric error annotation) machine-enforceable before any grader produces a value:

- **`bench/evaluators/metrics.go`** — `Metrics` struct with 19 pointer fields, every json tag snake_case, NO `omitempty` (a nil marshals to JSON `null`, never dropped); `MetricError` with `Metric`/`Grader`/`Reason`. Pure data contract, no helpers/business logic.
- **`bench/schema/result.v2.schema.json`** — added a nullable `metrics` object (each field typed `["<type>","null"]` with documented range constraints) + a `metric_errors[]` array; relaxed top-level `tokens_input`/`tokens_output` to nullable. Minor bump (no v3); `schema_version` stays the only required field; `additionalProperties` open.

## Tasks (TDD: RED -> GREEN)

### Task 1: Typed nullable Metrics + MetricError contract
- **RED** (`ac345f50` `test(79-01): add failing test for nullable Metrics contract`): wrote `metrics_test.go` with `TestMetricsNullableMarshal` (zero-value emits explicit nulls; set fields marshal to value while rest stay null; key set == 19 snake_case keys) and `TestMetricErrorShape`. Failed: `undefined: Metrics` / `undefined: MetricError`.
- **GREEN** (`08690257` `feat(79-01): implement nullable Metrics + MetricError contract`): created `metrics.go`. Tests pass; `grep -c omitempty` == 0; `go vet` clean.
- **REFACTOR:** none (reworded the doc comment to drop the literal word "omitempty" so the acceptance grep returns 0 — same commit as GREEN, not a behavior change).

### Task 2: Type all 17 metrics + metric_errors in result.v2.schema.json (additive-only)
- **RED** (`d56eed9f` `test(79-01): add failing schema test for metrics + metric_errors`): added `TestResultV2Metrics` (fully-populated metrics validates; null values validate; metric_errors[] validates; out-of-range `edit_locality` 1.5 rejected; negative `tokens_input` rejected; metric-sparse golden still validates). Failed: the two range-rejection subtests (open schema accepted everything).
- **GREEN** (`be82ff47` `feat(79-01): type all 17 metrics + metric_errors in result.v2 schema (minor bump)`): added the `metrics` object + `metric_errors[]` array, relaxed top-level token fields. Full `./bench/schema/` suite green (new + 5 pre-existing tests).

## Metrics field -> json key mapping (consume verbatim, Wave-2/3)

| Go field | json key | type |
|---|---|---|
| TaskSuccess | task_success | *bool |
| VerifiedCorrectness | verified_correctness | *bool |
| TokensInput | tokens_input | *int |
| TokensOutput | tokens_output | *int |
| TokensInputCachedRead | tokens_input_cached_read | *int |
| TokensInputCacheWrite | tokens_input_cache_write | *int |
| ToolCalls | tool_calls | *int |
| WallTimeSeconds | wall_time_seconds | *float64 |
| FilesRead | files_read | *int |
| BytesRead | bytes_read | *int |
| FilesModified | files_modified | *int |
| EditLocality | edit_locality | *float64 (0..1) |
| RegressionRate | regression_rate | *float64 (>=0) |
| LSPDiagnosticsUsed | lsp_diagnostics_used | *int |
| SemanticToolCalls | semantic_tool_calls | *int |
| EditDistancePatch | edit_distance_patch | *int |
| RetryCount | retry_count | *int |
| CompileErrorsBefore | compile_errors_before | *int |
| CompileErrorsAfter | compile_errors_after | *int |

`MetricError`: `Metric` -> `metric`, `Grader` -> `grader`, `Reason` -> `reason`.

## Verification

- `go build ./cmd/helix` — clean.
- `go vet ./bench/...` — clean.
- `go test ./bench/...` — all packages green (evaluators, languages, languages/go, runners, runtime, schema).
- `grep -c omitempty bench/evaluators/metrics.go` == 0.
- `required` array == `["schema_version"]` only; `edit_locality` carries `"maximum": 1`; no `result.v3.schema.json`.
- Pre-existing golden `result.v2.golden.json` (metric-sparse) still validates (additive-only proof).

## Deviations from Plan

None — plan executed exactly as written. The plan's `<artifacts_this_phase_produces>` (19 fields) and acceptance criterion ("19 pointer fields") were authoritative over the 17-field illustrative snippet in 79-PATTERNS.md; followed the 19-field list.

## TDD Gate Compliance

Both tasks followed RED -> GREEN. `git log` order: `ac345f50` test -> `08690257` feat (Task 1); `d56eed9f` test -> `be82ff47` feat (Task 2). No test passed unexpectedly during RED.

## Self-Check: PASSED

- Files: all 4 present (metrics.go, metrics_test.go, result.v2.schema.json, result.v2_test.go).
- Commits: ac345f50, 08690257, d56eed9f, be82ff47 all in git log.
