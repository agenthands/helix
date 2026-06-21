---
phase: 75-schema-fairness-contract-tree-skeleton
plan: 03
subsystem: testing
tags: [json-schema, draft-2020-12, santhosh-tekuri-jsonschema, bench, result-contract, fairness, tdd]

# Dependency graph
requires:
  - phase: 75-01
    provides: Wave 0 bench/ skeleton groundwork (six-dir layout incl. bench/schema/)
  - phase: 75-02
    provides: bench/schema/ directory present (six-dir skeleton landed)
provides:
  - "bench/schema/result.v2.schema.json — versioned result-JSON contract (Draft 2020-12), schema_version const 'v2' required, additive-only=minor / breaking=v3 policy"
  - "FAIR-03 schema substrate: tokens_input_cached_read + tokens_input_cache_write columns (D-12) and fairness.overrides[] of {mode, waiver_reason, approved_by} (D-10)"
  - "bench/schema/testdata/result.v2.golden.json — canonical validating fixture downstream phases copy from"
  - "Self-enforcing golden-validate test (4 tests) keeping schema and example from drifting"
affects: [76, 77, 78, 79, 80, 81, 82, 89, bench-runners, evaluators, reports, fairness]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Golden-fixture-validates-against-schema test (offline Draft 2020-12, santhosh-tekuri/jsonschema v6)"
    - "Instances decoded via jsonschema.UnmarshalJSON (NOT encoding/json) to preserve json.Number (Pitfall 2)"
    - "Minimal required set (schema_version only) + open additionalProperties = additive-only minor versioning (D-03)"

key-files:
  created:
    - bench/schema/result.v2.schema.json
    - bench/schema/result.v2_test.go
    - bench/schema/testdata/result.v2.golden.json
  modified: []

key-decisions:
  - "schema_version is the ONLY top-level required field; everything else optional so additive fields stay minor (D-03/D-04/Pitfall 5)"
  - "additionalProperties left OPEN at top level (verified by TestResultV2AdditiveFieldStaysValid) — additive policy recorded in a schema $comment"
  - "FAIR-03 delivered as SCHEMA SUBSTRATE ONLY: cached-input columns + fairness.overrides[] as fields; variance detector + cost_quality.md warning deferred to Phase 82/89"
  - "Token fields typed as integer (>=0); fairness.overrides[].* typed as string"

patterns-established:
  - "Versioned result contract: result.vN.schema.json + testdata/result.vN.golden.json + result.vN_test.go co-located in bench/schema/"
  - "RED-before-GREEN TDD gating with atomic per-gate commits (test(...) then feat(...))"

requirements-completed: [BENCH-03, FAIR-03]

# Metrics
duration: ~12min
completed: 2026-06-15
---

# Phase 75 Plan 03: result.v2 result-schema contract + self-enforcing golden validation Summary

**Versioned `result.v2.json` contract (Draft 2020-12) with a single required `schema_version: "v2"`, FAIR-03 cached-input columns + `fairness.overrides[]` substrate, an open-additionalProperties additive-only=minor policy, and a 4-test golden-validate suite that keeps schema and example from drifting.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-06-15 (post 75-02)
- **Completed:** 2026-06-15
- **Tasks:** 1 (TDD: RED + GREEN gates; no REFACTOR needed)
- **Files created:** 3

## Accomplishments
- Authored `bench/schema/result.v2.schema.json` — the keystone result contract 14 downstream phases (76-89) write into and validate against. Draft 2020-12, `$schema` + `$id`, `schema_version` pinned to const `"v2"` as the only required field.
- Delivered the FAIR-03 SCHEMA SUBSTRATE: separate `tokens_input_cached_read` / `tokens_input_cache_write` integer columns (D-12) and a `fairness.overrides[]` array of `{mode, waiver_reason, approved_by}` (D-10). (Variance detector + cost_quality.md warning explicitly deferred to Phase 82/89.)
- Authored the canonical golden fixture `bench/schema/testdata/result.v2.golden.json` exercising every defined field, stamped `"schema_version": "v2"`.
- Self-enforcing test suite (4 tests): golden validates, schema is valid Draft 2020-12 against the offline meta-schema, missing `schema_version` fails, additive optional field stays valid.
- Recorded the additive-only=minor / breaking=v3 versioning policy in a schema `$comment` (D-03/D-04).

## Task Commits

TDD task — RED gate committed before GREEN gate:

1. **Task 1 (RED): failing golden-validate test + fixture** - `0e8decc6` (test)
2. **Task 1 (GREEN): author result.v2.schema.json** - `479e5c14` (feat)

REFACTOR gate: not needed — golden fixture already exercises every field, numeric types already consistent (integer throughout), descriptions tidy on first authoring; all four tests green without further change.

**Plan metadata:** committed separately (docs: complete plan).

_RED preceded GREEN in git history (`git log --oneline bench/schema/` shows `test(...)` below `feat(...)`)._

## Files Created/Modified
- `bench/schema/result.v2.schema.json` - Draft 2020-12 result contract; `schema_version` const "v2" (only required field); identity fields (task_id, mode, benchmark, run_index); token fields incl. FAIR-03 cached columns; `fairness.overrides[]`; open additionalProperties; `$comment` versioning policy.
- `bench/schema/result.v2_test.go` - package `schema_test`; 4 tests (`TestResultV2GoldenValidates`, `TestResultV2SchemaIsValidDraft2020`, `TestResultV2RequiresSchemaVersion`, `TestResultV2AdditiveFieldStaysValid`); compiles schema + meta-schema offline; decodes instances via `jsonschema.UnmarshalJSON`.
- `bench/schema/testdata/result.v2.golden.json` - canonical validating example; every defined field populated.

## Decisions Made
- Kept `required` to exactly `schema_version` (Pitfall 5) so future fields land optional and stay minor (D-03).
- Left `additionalProperties` open at top level rather than `false`, matching RESEARCH Open-Question 2 resolution; the additive policy lives in `$comment`.
- Scoped FAIR-03 to schema substrate only, per plan scope_notes — the >5% between-run variance detector (Phase 82, consumes STATS-01 N>=3) and the cost_quality.md warning (Phase 89) are NOT implemented or graded here.

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
- Full-suite `go test ./...` reports a FAILURE in `github.com/agenthands/helix/test/bench` (`TestBenchToolsManifestMatchesRegistry`: registry reports 53 tools, test expects 47; `TestToolDescriptionsGoldenFile` golden stale). **This is PRE-EXISTING and unrelated** — it fails identically at the commit before this plan (HEAD~2), `test/bench` does not reference `bench/schema`, and it was already logged under Plan 75-01 in `deferred-items.md`. Re-confirmed and appended a 75-03 note there. Not fixed (SCOPE BOUNDARY).
- `go vet ./...` exits 0. The target package `go test ./bench/schema/...` is fully green (all 4 tests pass).

## Known Stubs
None. The schema and golden fixture are complete and self-validating; no placeholder data or unwired fields.

## TDD Gate Compliance
- RED gate: `0e8decc6` (`test(75-03): ...`) — committed with all four tests failing because the schema did not yet exist.
- GREEN gate: `479e5c14` (`feat(75-03): ...`) — committed with all four tests passing.
- REFACTOR gate: intentionally omitted (no changes required to keep green).
- Gate ordering verified: `test(...)` precedes `feat(...)` in `git log --oneline bench/schema/`.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The stable `result.v2` contract is in place; downstream bench phases (runners, evaluators, reports) can now emit and validate `result.v2.json` and copy the golden fixture as a template.
- FAIR-03 variance gate (Phase 82) and cost_quality.md warning (Phase 89) remain to be built on top of the substrate fields delivered here.
- No blockers introduced. The pre-existing `test/bench` tool-manifest failure is independent of this plan and tracked for a dedicated golden-refresh task.

## Self-Check: PASSED

- All 3 created files present on disk.
- Both TDD gate commits present in git history (`0e8decc6` test, `479e5c14` feat).

---
*Phase: 75-schema-fairness-contract-tree-skeleton*
*Completed: 2026-06-15*
