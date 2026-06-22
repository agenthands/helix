---
phase: 80-five-of-six-ablation-runners-fairness-enforcement
plan: 02
subsystem: testing
tags: [bench, result-schema, json-schema, ablation, tdd]

# Dependency graph
requires:
  - phase: 77
    provides: result.v2 builder (BuildResult/ResultInput/resultDoc) + validate-on-write gate + go:embed schema
  - phase: 79
    provides: result.v2 metrics object + nullable token columns (additive-only=minor precedent)
provides:
  - "ResultInput.AblationStatus string field on the result.v2 builder"
  - "resultDoc.AblationStatus json tag ablation_status,omitempty (honest modes omit it)"
  - "BuildResult wiring AblationStatus: in.AblationStatus"
  - "ablation_status documented as OPTIONAL top-level property in result.v2.schema.json (additive, no major bump)"
  - "greppable deferral-marker value guarantee_pending_phase_81 documented in schema"
affects: [80-03, 81, 82]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Additive-only=minor: new optional field rides as open additionalProperty; schema_version stays v2 (no v3)"
    - "omitempty on the open provenance string so honest modes emit nothing; only the partial arm carries the marker"

key-files:
  created: []
  modified:
    - bench/runtime/result.go
    - bench/runtime/result_test.go
    - bench/schema/result.v2.schema.json

key-decisions:
  - "ablation_status is additive-only (open top-level additionalProperties); schema_version stays v2, no v3, no additionalProperties:false introduced (D-03)"
  - "resultDoc.AblationStatus uses omitempty so honest modes omit the key entirely; only the no_semantic arm carries guarantee_pending_phase_81 (value SET by Plan 03 cell wiring)"
  - "Field documented in schema for the Phase 82 aggregator to distinguish a partial no_semantic row from a clean measurement; provenance/display-only (no injection sink)"

patterns-established:
  - "Open provenance prop (ride-along additionalProperty) extended again — mirrors outcome/trace_ref/model_id but with omitempty"

requirements-completed: [ABLATE-01]

# Metrics
duration: ~6min
completed: 2026-06-19
---

# Phase 80 Plan 02: Additive ablation_status field Summary

**Added the additive optional `ablation_status` open provenance field to the result.v2 builder (ResultInput → resultDoc `ablation_status,omitempty` → BuildResult) and documented it as an optional top-level schema property — honest modes omit it, the no_semantic arm will carry `guarantee_pending_phase_81` (D-03 machine-checkable deferral marker).**

## Performance

- **Duration:** ~6 min
- **Completed:** 2026-06-19
- **Tasks:** 1 (TDD RED→GREEN)
- **Files modified:** 3

## Accomplishments
- `ResultInput.AblationStatus string` added with the D-03 doc comment.
- `resultDoc.AblationStatus string` with json tag `ablation_status,omitempty` so honest modes emit nothing.
- `BuildResult` doc literal wires `AblationStatus: in.AblationStatus`.
- `ablation_status` documented as an OPTIONAL top-level `{"type":"string"}` property in `result.v2.schema.json`, naming the greppable `guarantee_pending_phase_81` enum value and noting it is additive-only (no schema major bump).
- Two new builder tests prove the round-trip and the omitempty behavior, both schema-valid.

## Task Commits

Each task was committed atomically (TDD RED→GREEN):

1. **Task 1 (RED): failing ablation_status tests** - `496a81b1` (test)
2. **Task 1 (GREEN): additive ablation_status field + schema doc** - `3aec6043` (feat)

_No REFACTOR commit — the GREEN implementation was minimal and clean._

## Files Created/Modified
- `bench/runtime/result.go` - Added `AblationStatus` to `ResultInput` and `resultDoc` (json `ablation_status,omitempty`); wired `AblationStatus: in.AblationStatus` in the `BuildResult` doc literal.
- `bench/runtime/result_test.go` - Added `TestAblationStatusProjected` (non-empty value round-trips under `ablation_status`, schema-valid) and `TestAblationStatusOmittedWhenEmpty` (honest mode omits the key, schema-valid).
- `bench/schema/result.v2.schema.json` - Documented `ablation_status` as an optional top-level string property; described `guarantee_pending_phase_81`, noted additive-only / no major bump.

## Decisions Made
- Field carried as an open provenance ride-along (like `outcome`/`trace_ref`/`model_id`) but WITH `omitempty`, so honest modes emit nothing — only the partial no_semantic arm carries the marker. Value SET by Plan 03 cell wiring; this plan delivers field + schema doc only.
- No `schema_version` bump and no `additionalProperties: false` introduced anywhere — preserves the D-03/D-04 additive-only=minor contract.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- After adding the field, the `BuildResult` doc literal needed gofmt realignment of the existing struct keys (mechanical; `gofmt -w` applied, no logic change). Not a deviation.

## TDD Gate Compliance
- RED gate: `test(80-02)` commit `496a81b1` (tests fail to compile — `AblationStatus` field absent).
- GREEN gate: `feat(80-02)` commit `3aec6043` (field added; both tests pass).
- Gate sequence satisfied (RED precedes GREEN).

## Threat Flags
None — `ablation_status` is a fixed internal harness constant (provenance/display-only, no exec/shell/query sink). No new external surface; matches the plan's T-80-02 disposition.

## Verification
- `go build ./cmd/helix` — OK
- `go vet ./...` — OK (no findings)
- `go test ./bench/... -count=1` — all pass (including `bench/runtime`, `bench/schema`)
- `go test ./bench/runtime/ -run TestAblationStatus -count=1` — both new tests pass
- `grep -q ablation_status bench/schema/result.v2.schema.json` — present

## Next Phase Readiness
- Field + schema doc ready for Plan 03 to SET `ablation_status: guarantee_pending_phase_81` on the no_semantic cell.
- Phase 81 lands the kernel `disable_semantic_subsystem` guarantee; Phase 82 aggregator reads `ablation_status` to distinguish partial rows.

## Self-Check: PASSED

- bench/runtime/result.go — FOUND
- bench/runtime/result_test.go — FOUND
- bench/schema/result.v2.schema.json — FOUND
- 80-02-SUMMARY.md — FOUND
- commit 496a81b1 (RED test) — FOUND
- commit 3aec6043 (GREEN feat) — FOUND

---
*Phase: 80-five-of-six-ablation-runners-fairness-enforcement*
*Completed: 2026-06-19*
