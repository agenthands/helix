---
phase: 88-multi-swe-bench-terminal-bench-2-0-adapters
plan: 01
subsystem: testing
tags: [multi-swe-bench, evaluator-adapter, subprocess-shellout, config-producer, per-language-slicing, aggregator, hermetic-fixtures]

# Dependency graph
requires:
  - phase: 87-swe-bench-verified-adapter
    provides: "bench/evaluators/swebench/ template (harness/ingest/report/procgroup), isValidPredictionsPath, strict env allowlist, runShim seam, resolved-gate ingestion"
  - phase: 85-aider-adapter
    provides: "ResultInput.Language open key + aggregator reduceLanguageRows/rowLanguage per-language slicing substrate"
provides:
  - "bench/evaluators/multiswebench/ leaf adapter: config.json producer (the one config-driven divergence) + fixed --config harness argv + final_report/per-instance -> result.v2 ingestion"
  - "directory->Cell.Language mapping (Pitfall 1: Multi-SWE has no per-instance language field) flowed through the existing aggregator"
  - "SC#1 hermetic proof: 7-language fixture yields 7 ByLanguage rows through existing reduceLanguageRows with zero new aggregator production code"
affects: [88-02-terminalbench, 88-03-longwall, 88-04-multi-swe-bench-mini-dataset, aggregator, bench-cell-orchestration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Config-file-driven evaluator adapter (vs flag-driven SWE-bench): validate every path field BEFORE json.MarshalIndent, then trivial fixed --config argv"
    - "Language stamped from the cell argument, never a JSON field (Multi-SWE partitions by dataset-file directory)"

key-files:
  created:
    - "bench/evaluators/multiswebench/config.go"
    - "bench/evaluators/multiswebench/harness.go"
    - "bench/evaluators/multiswebench/ingest.go"
    - "bench/evaluators/multiswebench/report.go"
    - "bench/evaluators/multiswebench/procgroup_unix.go"
    - "bench/evaluators/multiswebench/procgroup_windows.go"
    - "bench/evaluators/multiswebench/testdata/config.golden.json"
    - "bench/evaluators/multiswebench/testdata/final_report.json"
    - "bench/evaluators/multiswebench/testdata/report.go.json"
    - "bench/evaluators/multiswebench/testdata/report.java.json"
  modified:
    - "bench/aggregator/aggregate_test.go"

key-decisions:
  - "config.go does NOT import bench/runtime: copied the writeCacheAtomic temp+rename primitive locally so the adapter stays a leaf package (writeDurable is unexported)"
  - "Config struct-field declaration order IS the JSON key order (json.MarshalIndent, no map iteration) -> byte-stable golden"
  - "Ingest signature carries an explicit lang argument (Pitfall 1): Multi-SWE-bench has no per-instance language JSON field; the cell orchestrator owns the directory->language mapping"
  - "Task 3 is test-only by design: reduceLanguageRows/rowLanguage already slice the language open key, so 7-language slicing needs zero new aggregator production code"

patterns-established:
  - "Config-file producer divergence: path-validate every field with isValidPredictionsPath BEFORE marshal, atomic temp+rename, fixed --config argv"
  - "resolved-set/per-instance resolved bool is the sole authoritative task_success gate (distinct &taskSuccess pointer), never a test-row count; missing instance -> ingest error, never fabricated success"

requirements-completed: [ADAPTER-MULTI-01]

# Metrics
duration: 8min
completed: 2026-06-21
---

# Phase 88 Plan 01: Multi-SWE-bench Adapter Summary

**Config-file-driven Multi-SWE-bench evaluator adapter cloning the Phase 87 swebench template — a path-validating byte-stable config.json producer, a fixed `--config` harness argv under a strict env allowlist, resolved-gate `final_report`/per-instance ingestion with directory-derived `Cell.Language`, and SC#1 proof that 7-language slicing flows through the existing aggregator with zero new production code.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-06-21T06:59Z
- **Completed:** 2026-06-21T07:04Z
- **Tasks:** 3
- **Files modified:** 11 created, 1 modified

## Accomplishments
- `bench/evaluators/multiswebench/config.go` — `Config` (18 verbatim upstream fields in declaration order) + `WriteConfig` that path-validates every field (Workdir/OutputDir/RepoDir/LogDir + each PatchFiles[]/DatasetFiles[]) with the verbatim `isValidPredictionsPath` BEFORE marshal, then writes via a locally-copied atomic temp+rename (no `bench/runtime` import). Byte-stable golden contract committed.
- `bench/evaluators/multiswebench/harness.go` — the one structural divergence: a trivial fixed argv `["-m","multi_swe_bench.harness.run_evaluation","--config",<path>]` (config-driven, not flag-driven), with the verbatim Detect/runShim/strict-env-allowlist/`multiswebench-runs` workdir cloned from the template.
- `bench/evaluators/multiswebench/report.go` + `ingest.go` — size-capped strict parsers for `final_report.json` (resolved_ids/unresolved_ids/total) and the per-instance report; ingestion gates `TaskSuccess` on the authoritative `resolved` bool (distinct pointer), carries `ContainerID`/`ExitCode *int` (a literal 0 preserved), stamps `Language` from the cell argument (Pitfall 1), and errors on a missing instance (never fabricated success).
- `bench/aggregator/aggregate_test.go` — `TestAggregateByLanguageMultiSWE` proves a 7-language fixture {go,java,ts,js,rust,c,cpp} yields exactly 7 `ByLanguage` rows through the existing `reduceLanguageRows` (SC#1, zero new aggregator production code).

## Task Commits

Each TDD task was committed RED→GREEN:

1. **Task 1 RED: config tests + golden fixture** - `cbbbf7fc` (test)
2. **Task 1 GREEN: config.json producer** - `9aa4118e` (feat)
3. **Task 2 RED: harness/report/ingest tests + fixtures** - `873bde62` (test)
4. **Task 2 GREEN: harness argv + parsers + ingestion** - `c2bdc2ee` (feat)
5. **Task 3: 7-language slicing test (test-only)** - `4dec1b6b` (test)

_Note: Task 3 is a single test commit by design — it proves existing aggregator behavior over a new fixture with zero production code (SC#1)._

## Files Created/Modified
- `bench/evaluators/multiswebench/config.go` - Config struct + path-validating atomic WriteConfig producer
- `bench/evaluators/multiswebench/harness.go` - fixed `--config` argv, Detect skip-gate, strict env allowlist, multiswebench-runs workdir, runShim seam
- `bench/evaluators/multiswebench/report.go` - FinalReport + per-instance InstanceEval/InstanceReport; size-capped strict parsers
- `bench/evaluators/multiswebench/ingest.go` - resolved-gate -> TaskSuccess, Language from cell arg, ExitCode *int, missing-instance error
- `bench/evaluators/multiswebench/procgroup_{unix,windows}.go` - verbatim process-group seam (package renamed)
- `bench/evaluators/multiswebench/testdata/{config.golden.json,final_report.json,report.go.json,report.java.json}` - hermetic fixtures (the SOLE proof)
- `bench/aggregator/aggregate_test.go` - extended with TestAggregateByLanguageMultiSWE (7 ByLanguage rows)

## Decisions Made
- Kept the adapter a leaf package by copying the atomic temp+rename primitive into `config.go` rather than importing `bench/runtime` (its `writeDurable` is unexported) — matches the plan's `read_first` directive.
- The golden `config.golden.json` is the producer contract; per the plan its exact upstream field spellings (A1 [ASSUMED]) are pinned here for the hermetic golden and confirmed against the live README at Plan 04's human-verify checkpoint.
- `Ingest` takes a per-instance `InstanceReport` plus an explicit `lang` argument (the cell-language axis), not the `FinalReport`, mirroring the SWE-bench `Ingest(InstanceReport, ...)` shape; `FinalReport` parsing is exercised independently in `report_test.go`.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Task 3's test passed immediately on first run. This is expected and correct per the plan: the task is explicitly test-only ("zero new aggregator production code") because the existing `reduceLanguageRows`/`rowLanguage` already slice the `language` open key. Verified (TDD fail-fast investigation) the test genuinely exercises the aggregator over a new 7-language fixture and asserts 7 rows — it is not a vacuous or pre-existing-coverage duplicate.

## User Setup Required
None - no external service configuration required. The live Multi-SWE Mini-set run is Docker/multi_swe_bench-gated; `Detect()` resolves python but the live test `t.Skip`s cleanly and is never the sole proof (the hermetic fixtures are).

## Next Phase Readiness
- The Multi-SWE adapter half of ADAPTER-MULTI-01 is complete and hermetically proven.
- Plan 02 (terminalbench) clones the same template; Plan 03 (longwall) wires the cell orchestrator that calls `WriteConfig` + `Ingest` and supplies the per-language `Cell.Language`; Plan 04 confirms the config golden field names against the live upstream README and ships the Mini dataset.
- No blockers.

## Known Stubs
None - every producer/parser/ingest path is wired and exercised by a hermetic fixture test.

## Self-Check: PASSED

---
*Phase: 88-multi-swe-bench-terminal-bench-2-0-adapters*
*Completed: 2026-06-21*
