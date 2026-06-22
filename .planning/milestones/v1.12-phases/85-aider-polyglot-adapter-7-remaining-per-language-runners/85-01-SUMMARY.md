---
phase: 85-aider-polyglot-adapter-7-remaining-per-language-runners
plan: 01
subsystem: testing
tags: [bench, result.v2, json-schema, aggregator, language-axis, provenance, tdd]

# Dependency graph
requires:
  - phase: 83-cost-and-ablation-provenance
    provides: EmbedderID additive-open-provenance-key precedent (result.v2 omitempty open key)
  - phase: 82-stats-and-cost-aggregator
    provides: two-level reduction + (task,mode) grouping + successCount null discipline
provides:
  - "Optional `language` provenance field on result.v2 (populated from Cell.Language, omitempty)"
  - "result.v2.schema.json documents `language` as optional additive-minor (schema_version stays v2)"
  - "Aggregator Report.ByLanguage []LanguageRow per-language pass-rate slice (SC#1 substrate)"
  - "rowLanguage open-key reader mirroring rowModelID"
affects: [aider-polyglot-adapter, per-language-runners, phase-89-reports, SC#1-per-language-pass-rate]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Additive-open-provenance-key: new result.v2 fields land OPTIONAL + omitempty; additionalProperties stays OPEN; schema_version unchanged (no v3 bump)"
    - "Aggregator open-key read: rowLanguage reads Doc[\"language\"] verbatim like rowModelID"
    - "Purely-additive aggregator reduction: ByLanguage never touches the RNG nor alters the (mode x benchmark) leaderboard/cost output"

key-files:
  created: []
  modified:
    - bench/runtime/result.go
    - bench/runtime/cell.go
    - bench/schema/result.v2.schema.json
    - bench/aggregator/aggregate.go
    - bench/aggregator/report.go
    - bench/runtime/result_test.go
    - bench/aggregator/aggregate_test.go

key-decisions:
  - "Mirrored the EmbedderID open-additive-key precedent verbatim for `language` (omitempty, open additionalProperties, no schema major bump)"
  - "Per-language pass-rate is a flat pooled rate (#pass / #non-nil task_success), NOT a BCa-bootstrapped CI — SC#1 needs the slice to exist and be correct; the bootstrapped per-language CI is downstream Phase 89"
  - "Absent-`language`-key rows bucket under Language=='' so pre-language artifacts still aggregate without breaking the leaderboard"
  - "rowLanguage never derives language from task_id (task_id parsing is the documented downstream fallback only) — the persisted Cell.Language is the only source"

patterns-established:
  - "Additive-minor result.v2 evolution: optional + omitempty field, open additionalProperties, schema_version pinned, backward-compat Validate() test on a pre-field fixture"
  - "Per-axis aggregator slice: group all loaded rows by an open doc key, pool via successCount null discipline, expose as an additive Report field"

requirements-completed: [ADAPTER-AIDER-01]

# Metrics
duration: ~25min
completed: 2026-06-21
---

# Phase 85 Plan 01: Additive `language` Axis for the Bench Result Pipeline Summary

**Threaded an optional `language` provenance field through result.v2 (result.go + schema + cell, mirroring the EmbedderID open-key precedent) and added a purely-additive per-language pass-rate slice (`Report.ByLanguage`) to the aggregator, so SC#1's "Python pass-rate" is sliceable while pre-language artifacts still validate and aggregate unchanged.**

## Performance

- **Duration:** ~25 min
- **Completed:** 2026-06-21
- **Tasks:** 2 (both TDD: RED → GREEN)
- **Files modified:** 7

## Accomplishments
- `language` is an additive-minor open provenance key on result.v2: `omitempty`, `additionalProperties` stays OPEN, `schema_version` stays `"v2"` (no v3 bump). Populated from `Cell.Language` (cell.go:189) at the BuildResult call site.
- A result.v2 with no `language` field (every pre-Phase-85 artifact) still passes `Validate()` and still aggregates — backward-compat pinned by a dedicated hand-rolled-fixture test.
- The aggregator exposes a correct per-language pass-rate slice (`Report.ByLanguage`), grouping every loaded row by `rowLanguage` and pooling pass-rate via the existing `successCount` null discipline. python=1.0 / rust=0.0 in the fixture; absent-key rows bucket under `""`.
- Zero change to the existing (mode×benchmark) leaderboard/cost output — the golden end-to-end and determinism tests pass unchanged.

## Task Commits

Each task was committed atomically (TDD: RED then GREEN):

1. **Task 1 (RED): failing tests for the `language` field** - `75a8e47f` (test)
2. **Task 1 (GREEN): thread `language` through result.v2 + schema + cell** - `90274bae` (feat)
3. **Task 2 (RED): failing tests for the aggregator per-language reduction** - `96d5d797` (test)
4. **Task 2 (GREEN): rowLanguage + reduceLanguageRows + Report.ByLanguage** - `9bdc12bb` (feat)

_RED→GREEN gate satisfied: both `test(85-01)` and `feat(85-01)` commits exist._

## Files Created/Modified
- `bench/runtime/result.go` - `Language` on `ResultInput` + `resultDoc` (`json:"language,omitempty"`) + the `BuildResult` assembly, mirroring `EmbedderID`.
- `bench/runtime/cell.go` - threads `cfg.Language` into the `BuildResult(ResultInput{...})` call site.
- `bench/schema/result.v2.schema.json` - optional `language` property doc entry (additive-minor; not in `required`; `additionalProperties` left open).
- `bench/aggregator/aggregate.go` - `rowLanguage` open-key reader (mirrors `rowModelID`); `reduceLanguageRows` pooling per-language pass-rate; wired additively into `Aggregate`.
- `bench/aggregator/report.go` - `LanguageRow{Language, PassRate, N}` type + `Report.ByLanguage` field.
- `bench/runtime/result_test.go` - 3 cases: language emitted when set, omitted when empty, pre-language fixture still validates.
- `bench/aggregator/aggregate_test.go` - `rowLanguage` open-key read; python=1.0/rust=0.0 slice; absent-key `""` bucket doesn't break the leaderboard. Added `writeLangRow` helper + `encoding/json` import.

## Decisions Made
- Mirrored the `EmbedderID` open-additive-key precedent verbatim rather than inventing a new pattern — it is the proven additive-open-key template and keeps the schema policy honest.
- Per-language pass-rate is a flat pooled rate, not a BCa-bootstrapped CI: SC#1 needs the slice to exist and be correct; the bootstrapped per-language CI is explicitly downstream (Phase 89).
- `rowLanguage` reads only the persisted `language` doc key (never derives from `task_id`), keeping the grouping source single and honest.

## Deviations from Plan

None - plan executed exactly as written. The plan's two TDD tasks were implemented exactly to spec (additive open key in Task 1; additive `ByLanguage` reduction in Task 2), no Rule 1-4 deviations were required.

## Issues Encountered
None. The schema is `go:embed`-compiled, so the updated `result.v2.schema.json` is picked up by `Validate()` automatically; no regeneration step was needed.

## Threat Surface

Per the plan's threat register, all three threats are satisfied by the implementation:
- **T-85-01-01 (Tampering / path-key):** `language` is used only as a grouping label in the aggregator (never joined into a filesystem path); `Cell.Language` is already validated by `validateCellKey` before persistence.
- **T-85-01-02 (Info Disclosure):** the language axis is public benchmark metadata; no PII/secret.
- **T-85-01-03 (Tampering / schema tightening):** the field is OPTIONAL + `omitempty`, `required` unchanged, `additionalProperties` left OPEN; `TestLanguageBackwardCompatValidate` pins that a pre-language artifact still validates.

No new security surface beyond the plan's threat model was introduced.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The `language` axis is now persisted on result.v2 and sliceable in the aggregator (`Report.ByLanguage`) — the shared substrate the Aider adapter (Plan 07) and the 7 per-language runner plans (03-05) consume.
- Rendering the per-language slice into a report file is intentionally deferred to Phase 89.
- Backward compatibility is guaranteed: existing result.v2 artifacts and the existing leaderboard/cost output are unchanged.

## Self-Check: PASSED

All 7 modified source files and the SUMMARY exist on disk; all 4 task commits (`75a8e47f`, `90274bae`, `96d5d797`, `9bdc12bb`) are present in git history.

---
*Phase: 85-aider-polyglot-adapter-7-remaining-per-language-runners*
*Completed: 2026-06-21*
