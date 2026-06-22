---
phase: 87-swe-bench-verified-adapter-utboost-rescorer-multi-oracle-ver
plan: 02
subsystem: testing
tags: [swe-bench, predictions, harness-report, ingestion, result-schema, bench, go, hermetic-fixture]

# Dependency graph
requires:
  - phase: 87-01
    provides: "result.v2 additive open keys container_id (string) + exit_code (*int) on ResultInput; bench/evaluators/swebench harness.go subprocess wrapper"
  - phase: 79
    provides: "bench/evaluators.Metrics nullable-pointer contract (TaskSuccess/VerifiedCorrectness distinct *bool); test_runner pure-transform-over-outcome + distinct-pointer divergence discipline"
provides:
  - "bench/evaluators/swebench.WritePredictions — pure, sorted, deterministic predictions.jsonl producer"
  - "bench/evaluators/swebench.ParseRunReport / ParseInstanceReport — strict, size-bounded harness report parsers (RunReport / InstanceReport / InstanceEval / TestsStatus / TestList structs)"
  - "bench/evaluators/swebench.Ingest — harness-JSON -> runtime.ResultInput (task_success <- report.resolved; container_id/exit_code carried; VerifiedCorrectness left nil)"
  - "committed fixtures: predictions.golden.jsonl, run_report.json, report.canonical.json, instance.json"
affects: [87-03, 87-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Pure deterministic JSONL producer: copy-before-sort by key, json.Encoder SetEscapeHTML(false), one compact object per line, empty input emits zero bytes (Pitfall 4 reproducibility)"
    - "Strict size-bounded untrusted-JSON parse: maxReportBytes (64MiB) cap checked BEFORE encoding/json.Unmarshal; type-mismatch/null/empty/truncated -> error never panic; unknown extra key tolerated (T-87-02-01)"
    - "task_success <- canonical.Resolved via fresh local then &local (distinct pointer, never aliased with VerifiedCorrectness which Plan 03 owns) — the VERIFIED-01 independence split"
    - "Missing instance -> ingest error with zero-value ResultInput (TaskSuccess nil) so an ignored error cannot be mistaken for a fabricated pass (T-87-02-02)"

key-files:
  created:
    - bench/evaluators/swebench/predictions.go
    - bench/evaluators/swebench/predictions_test.go
    - bench/evaluators/swebench/report.go
    - bench/evaluators/swebench/report_test.go
    - bench/evaluators/swebench/ingest.go
    - bench/evaluators/swebench/ingest_test.go
    - bench/evaluators/swebench/testdata/predictions.golden.jsonl
    - bench/evaluators/swebench/testdata/run_report.json
    - bench/evaluators/swebench/testdata/report.canonical.json
    - bench/evaluators/swebench/testdata/instance.json
  modified: []

# Decisions
decisions:
  - "maxReportBytes = 64 MiB (not crosscodeeval's 256 MiB): harness reports are KB-scale documents, so a tighter cap is the right DoS bound; the io.LimitReader stream-boundary defense is documented for a live io.Reader caller"
  - "Ingest sets ONLY Metrics.TaskSuccess; VerifiedCorrectness stays nil so Plan 03's 3-condition gate is the unambiguous sole producer (no aliasing, no double-write)"
  - "RED test for the report parser over-asserted a shared-contract garbage case (non-bool resolved is tolerated by RunReport as an unknown key); fixed the test to assert per-parser strictness against each parser's OWN contract"

# Metrics
metrics:
  duration_minutes: 5
  completed_date: 2026-06-21
  tasks_completed: 3
  files_created: 10
  files_modified: 0
  commits: 8
---

# Phase 87 Plan 02: predictions.jsonl producer + harness report parser + ingestion Summary

Built the hermetic ingestion half of the SWE-bench adapter as three pure Go transforms over two verified upstream JSON contracts: a deterministic `predictions.jsonl` producer, strict size-bounded parsers for the harness run-report + per-instance `report.json`, and an `Ingest` transform mapping a parsed report into a `bench/runtime.ResultInput` with `task_success` sourced from `report.resolved` and the harness `container_id`/`exit_code` carried into the Plan 01 additive keys — all proven against committed fixtures, no subprocess, no Docker.

## What Was Built (RED → GREEN per feature)

### Feature 1 — predictions.jsonl producer
- **RED** (`1b51bd57`): `predictions_test.go` + `testdata/predictions.golden.jsonl` — golden byte-match over sorted compact JSONL, determinism across reversed input, empty-input-emits-zero-bytes. Compile failure (undefined `Prediction`/`WritePredictions`).
- **GREEN** (`498d9ab1`): `predictions.go` — `WritePredictions(w, rows)` copies-before-sort by `InstanceID`, `json.Encoder` with `SetEscapeHTML(false)`, one compact object per line. Empty/nil slice emits zero bytes.
- Golden re-generated independently and confirmed byte-identical on a second run.

### Feature 2 — harness report parsers
- **RED** (`aa02248a`): `report_test.go` + fixtures `run_report.json` / `report.canonical.json` / `instance.json` — run-report + per-instance parse over fixtures, unknown-key tolerance, garbage→error, oversized→error. Compile failure.
- **GREEN** (`f696b46e`): `report.go` — `RunReport` (mirrors `{model}.{run_id}.json` incl. Docker-only `unstopped_*`), `InstanceReport` map → `InstanceEval` (`resolved` + `tests_status` 4 buckets), `ParseRunReport`/`ParseInstanceReport` with a `maxReportBytes` (64 MiB) size cap checked before unmarshal (T-87-02-01). Garbage/type-mismatch/null/empty → error, never panic; unknown extra key tolerated.
- **REFACTOR/style** (`19b55d40`): gofmt struct-tag alignment on `report.go` (flagged by `make vet`/gofmt).

### Feature 3 — harness-JSON → result.v2 ingestion
- **RED** (`15f81779`): `ingest_test.go` — resolved=true→TaskSuccess true & verified nil; resolved=false→TaskSuccess false; exit_code ptr(0) preserved; nil exit carried as nil; missing instance→error (no fabricated success); distinct pointers. Compile failure.
- **GREEN** (`d476ac94`): `ingest.go` — `Ingest(report, instanceID, containerID, exitCode)` → `runtime.ResultInput{Benchmark:"swe-bench-verified", TaskID:instanceID, ContainerID, ExitCode, Metrics:{TaskSuccess:&local}}`. `task_success := eval.Resolved` via a fresh local then `&local` (distinct pointer). `VerifiedCorrectness` left nil. Missing instance → error + zero-value ResultInput.

## Verification (all green)
- `go build ./...` — clean.
- `go vet ./bench/...` — clean; `make vet` (all custom vettools) — clean.
- `go test ./bench/evaluators/swebench/... ./bench/runtime/...` — ok.
- `TestPredictions` (golden + determinism + empty), `TestReportParse` (fixtures + unknown-key + garbage + oversized), `TestIngest` (resolved→task_success, container_id/exit_code carried, exit_code 0 preserved, missing→error, distinct pointers) — all pass.
- Predictions golden confirmed byte-identical on independent re-generation.

## Threat mitigations applied
- **T-87-02-01** (DoS/Tampering, report parse): `maxReportBytes` cap before `encoding/json`; strict struct/map unmarshal; truncated/garbage/null/empty → error. Tested via the oversized-body and garbage-body cases.
- **T-87-02-02** (Tampering, task_success): `task_success ← canonical.Resolved` authoritative bool (Pitfall 1, never a `tests_status` row count); missing instance is an ingest error, never a fabricated success.
- **T-87-02-03** (Tampering, predictions reproducibility): rows sorted before emit; two runs over reversed input are byte-identical (Pitfall 4).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] RED report-parser test over-asserted a shared garbage case**
- **Found during:** Feature 2 GREEN
- **Issue:** The RED test fed `{"sympy__sympy-20590": {"resolved": "not a bool"}}` to BOTH parsers expecting an error from each. That body is genuinely malformed only for the *instance-report* shape (it maps onto `InstanceEval.Resolved`); for the *run-report* shape the whole instance-keyed object is an unknown key and is correctly tolerated, so `ParseRunReport` returns nil error by design.
- **Fix:** Split the assertion — `bothBad` cases assert error from both parsers; the non-bool-`resolved` case asserts error from `ParseInstanceReport` only, proving each parser is strict against its OWN contract rather than a conflated shared one. Also dropped `"null"` from the run-report set (a literal `null` unmarshals to a zero-value struct without error) and asserted it only against `ParseInstanceReport` (nil-map guard).
- **Files modified:** bench/evaluators/swebench/report_test.go
- **Commit:** f696b46e (folded into the Feature 2 GREEN commit)

## Known Stubs
None. All three transforms are fully wired against committed fixtures; `VerifiedCorrectness` left nil is an intentional independence split (Plan 03's 3-condition gate is the sole producer per the plan's anticipated VERIFIED-01 divergence), not a stub.

## Self-Check: PASSED
- Files: predictions.go, report.go, ingest.go + _test.go + 4 testdata fixtures — all FOUND.
- Commits 1b51bd57, 498d9ab1, aa02248a, f696b46e, 15f81779, d476ac94, 19b55d40 — all FOUND in git log.
- Full verification gate (build/vet/make-vet/test) — all green; predictions golden byte-stable on re-gen.
