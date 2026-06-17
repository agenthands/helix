---
phase: 77-bench-runtime-first-e2e-smoke
plan: 02
subsystem: bench-runtime
tags: [bench, runtime, result-v2, cctap, trace-merge, schema-validation, tdd]
requires:
  - bench/schema/result.v2.schema.json (validate-on-write target)
  - bench/runners.DefaultContract (fairness block source)
  - internal/eval/trace.CCTapResult / Event / Usage / Merge (2-leg trace target)
  - internal/eval/runner.StepResult (synth input shape)
  - github.com/santhosh-tekuri/jsonschema/v6 (Draft2020 validation, pre-existing dep)
provides:
  - bench/runtime.BuildResult (result.v2.json builder, pure transform)
  - bench/runtime.Validate (validate-on-write gate, T-77-04)
  - bench/runtime.ResultInput (cell orchestrator provenance struct)
  - bench/runtime.SynthCCTap (CCTapResult synthesis from scripted StepResults)
  - bench/schema.ResultV2SchemaBytes / ResultV2SchemaID (embedded schema, cwd-free)
affects:
  - Plan 03 (cell orchestrator calls BuildResult + Validate + SynthCCTap)
  - Phase 79 (rich metrics land as additive optional props; outcome/trace_ref keys frozen here)
  - Phase 80 (fairness.overrides forward-compat already covered by builder)
tech-stack:
  added: []
  patterns:
    - pure-transform builder + validate-on-write gate (no IO in BuildResult/SynthCCTap)
    - go:embed of the committed schema (cwd-independent validation)
    - typed result doc with open provenance props (additive-only=minor contract)
    - non-nil empty slice so JSON marshals []  not null (fairness.overrides)
    - synthesize agent-tap leg from scripted tool-call log (no daemon KindToolCall, no double-count)
    - per-step AtTime drives synth event timestamps (approximate-faithful merge ordering)
key-files:
  created:
    - bench/runtime/result.go
    - bench/runtime/result_test.go
    - bench/runtime/cctap.go
    - bench/runtime/cctap_test.go
    - bench/schema/schema.go
  modified: []
decisions:
  - "D-04 result.v2 open provenance keys frozen as snake_case: outcome, trace_ref, model_id (Open Question 3) so Phase 79 consumes without rename"
  - "schema exposed to the builder via go:embed in a new bench/schema/schema.go (ResultV2SchemaBytes) rather than a cwd-relative read or runtime.Caller path — single source of truth, no test-vs-prod cwd skew"
  - "result doc is a typed struct (not map[string]any): rich metrics absent by construction since they are not fields, satisfying the metric-sparse assertion structurally"
  - "fairness.overrides is a non-nil empty slice so it marshals as [] not null (schema/D-04); ModelID projected into the open model_id prop"
  - "D-02 SynthCCTap emits NO Source:daemon KindToolCall events; Merge counts ToolCallSummary only from the real daemon leg (merge.go:97-99), so the CC leg adds 2nd-leg continuity without double-counting"
  - "D-02/Pitfall 6 synthesized Usage is the zero value (scripted has no model); tokens never fabricated"
  - "D-02/Pitfall 5 synth event timestamps come from each StepResult.AtTime, not a batch time.Now()"
metrics:
  duration_seconds: 540
  completed: 2026-06-17
  tasks: 2
  files: 5
---

# Phase 77 Plan 02: result.v2 Builder + CC-Tap Synthesis Summary

Two pure-transform glue pieces the cell orchestrator (Plan 03) needs, both built TDD (RED→GREEN): the `result.v2.json` builder (none existed — Pitfall 2) that emits a schema-valid, provenance-complete / metric-sparse doc with fairness from `DefaultContract` and validate-on-write against the embedded schema (T-77-04), and the CC-tap synthesizer that turns the scripted agent's `[]runner.StepResult` into a `trace.CCTapResult` so `trace.Merge` produces a real 2-leg trace (`ToolCallSummary.Total>=1`, CC leg present) without double-counting (D-02).

## What Was Built

### Task 1 — result.v2.json builder + validate-on-write (D-04)
- `bench/runtime/result.go`: `ResultInput` struct (provenance the cell hands in), `BuildResult(ResultInput) ([]byte, error)` pure transform, and `Validate([]byte) error` validate-on-write gate.
- The doc is a **typed struct** (`resultDoc`) whose fields are exactly the named schema props (`schema_version`, `task_id`, `mode`, `benchmark`, `run_index`, `tokens_input`, `tokens_output`, `fairness`) plus the three open provenance props (`outcome`, `trace_ref`, `model_id`). Rich metrics (`edit_locality`, `regression_rate`, pass@k) are **not fields**, so they are absent by construction — the metric-sparse assertion holds structurally, not by runtime filtering.
- `schema_version` defaults to `"v2"`; `fairness.overrides` is sourced from `runners.DefaultContract` and is a non-nil empty slice so it marshals as `[]`, never `null`. `model_id` carries `DefaultContract.ModelID`.
- `Validate` reuses the `bench/schema/result.v2_test.go` jsonschema/v6 Draft2020 compile pattern verbatim (UnmarshalJSON preserving json.Number, AddResource, Compile, Validate). It rejects a malformed doc (non-string / non-"v2" `schema_version`, or missing `schema_version`).
- `bench/schema/schema.go` (new): `go:embed result.v2.schema.json` exposed as `ResultV2SchemaBytes` + `ResultV2SchemaID`, so the builder validates against the committed contract without a cwd-relative read (plan's "module-relative path or go:embed; do NOT assume cwd").

### Task 2 — synthesize CCTapResult from scripted StepResults (D-02)
- `bench/runtime/cctap.go`: `SynthCCTap(steps []runner.StepResult) trace.CCTapResult`.
- Emits `SessionInit -> per step (AssistantMsg carrying ToolUse{Name: step.Tool} + ToolResult{IsError: step.Err != nil}) -> Result`, all `Source:"cc"`, mirroring `TapCCStream`'s shape so the leg looks identical to a real claude run.
- Event timestamps come from each `step.AtTime` (Pitfall 5), not a batch `time.Now()`; `Usage` is the zero value (Pitfall 6 — scripted has no model, never fabricated); `FinalSessionID` is a stable synthetic marker.
- Emits **no** `Source:"daemon"` `KindToolCall` events — `Merge` counts `ToolCallSummary` only from the daemon leg (merge.go:97-99), so the CC leg adds 2nd-leg continuity without double-counting. Verified by the integration test asserting `ToolCallSummary.Total == 1` with one fabricated daemon call alongside the synth.

## How to Verify

- `go test ./bench/runtime/ -run ResultV2Valid -count=1` — builder + validate (Task 1).
- `go test ./bench/runtime/ -run SynthCCTap -count=1` — synth shape + timestamps + 2-leg Merge (Task 2).
- `go test ./bench/... -count=1` — whole bench tree green.
- `go build ./...`, `go vet ./bench/...` clean; `gofmt -l bench/runtime bench/schema` empty.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Schema not reachable via go:embed across directories**
- **Found during:** Task 1 (Validate implementation).
- **Issue:** `bench/schema/result.v2.schema.json` lives in a sibling dir; `go:embed` cannot reference parent/sibling paths from `bench/runtime`, and the schema package exposed no Go API or embedded bytes (only the test in `package schema_test`). A cwd-relative read would have violated the plan's "do NOT assume cwd" constraint.
- **Fix:** Added `bench/schema/schema.go` (`package schema`) embedding the schema as exported `ResultV2SchemaBytes` + `ResultV2SchemaID`. This keeps the schema file the single source of truth and is reusable by any future validator (e.g. the cell orchestrator, Phase 79).
- **Files modified:** `bench/schema/schema.go` (new).
- **Commit:** dc074657 (committed with the Task 1 RED test, since the embed is the prerequisite the test compiles against).

No other deviations — the two tasks executed as written.

## TDD Gate Compliance

Both tasks followed RED→GREEN with the mandated commit sequence:
- Task 1: `test(77-02)` dc074657 (RED, compile-fail) → `feat(77-02)` 2621ff2d (GREEN).
- Task 2: `test(77-02)` d08e45c6 (RED, compile-fail) → `feat(77-02)` 755a8e3e (GREEN).

No unexpected RED pass occurred (each RED failed to compile because the impl symbols did not yet exist). No REFACTOR commit was needed.

## Known Stubs

None. Both files are fully wired pure transforms with passing tests. The `model_id` / `outcome` / `trace_ref` keys are populated from caller-supplied + contract data, not hardcoded placeholders. (Scripted `tokens_input`/`tokens_output` of 0 are intentional and correct per D-02/Pitfall 6 — not a stub.)

## Self-Check: PASSED

- bench/runtime/result.go — FOUND
- bench/runtime/result_test.go — FOUND
- bench/runtime/cctap.go — FOUND
- bench/runtime/cctap_test.go — FOUND
- bench/schema/schema.go — FOUND
- Commit dc074657 (test, RED) — FOUND
- Commit 2621ff2d (feat, GREEN) — FOUND
- Commit d08e45c6 (test, RED) — FOUND
- Commit 755a8e3e (feat, GREEN) — FOUND
