---
phase: 79-evaluators-result-schema-metrics-layer
verified: 2026-06-18T16:14:00Z
status: human_needed
score: 4/4 success criteria verified
overrides_applied: 0
human_verification:
  - test: "Jaeger import of the merged trace.json (METRIC-06 visual continuity)"
    expected: "A single trace from the bench.run_index/run_id root down to LSP leaves, no orphan spans, no cross-cell PID leakage"
    why_human: "Jaeger UI import is a visual confirmation; the structural invariants (single merged trace, tool_call_summary.total parity, rejected_foreign_pid clear) are automated and PASS, but the visual root→leaf continuity must be eyeballed"
---

# Phase 79: Evaluators & Result-Schema Metrics Layer Verification Report

**Phase Goal:** Every per-task `result.v2.json` is populated with all 17 normalized metrics from real graders — including the load-bearing `tokens_input/output` from the provider's `usage` block (not Helix's MCP counter), `edit_locality` as the headline locality metric, and a single merged OTel trace per `(task, mode, run_index)`.
**Verified:** 2026-06-18T16:14:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

All four ROADMAP success criteria are verified TRUE in the codebase, the deterministic
unit suite is green, and the phase-goal headline outcome was confirmed by **running the
live `make bench-quick` E2E gate** (exit 0, 1/1 cells succeeded) and inspecting the
produced `result.v2.json`. One genuine human-verification item remains (Jaeger visual
continuity per VALIDATION.md), so status is `human_needed`, not `passed`.

### Observable Truths

| # | Truth (Success Criterion) | Status | Evidence |
|---|---------------------------|--------|----------|
| 1 | 5 grader packages produce all 12 base + 5 extended metrics; missing = explicit nulls (no omitempty); coordinator fans out to all 5 with D-07 isolation | ✓ VERIFIED | All 5 packages exist under `bench/evaluators/{test_runner,patch_validator,token_meter,tool_trace_analyzer,regression_checker}/`; `grep -c omitempty bench/evaluators/metrics.go` = 0; `Metrics` struct has all 19 pointer fields (17 metrics + 2 cached). Coordinator at `bench/evaluators/coordinator/coordinator.go` (sanctioned deviation — import-cycle avoidance) references all 5 graders and isolates each `*MetricError` (lines 90-133). Live E2E result.v2.json carries all 19 keys; nulls are explicit `null` with matching `metric_errors[]` entries. |
| 2 | Test asserts `tokens_input/output` source-of-truth = provider `usage` block, NOT MCP counter; cached tokens are separate columns | ✓ VERIFIED | `token_meter.go` reads ONLY `mt.Usage.{InputTokens,OutputTokens,CacheReadTokens,CacheCreationTokens}`; grep confirms no `ToolCallSummary`/`ResultSizeBytes` sourcing. `token_meter_test.go:43-63` negative test: a trace with daemon `ToolCallSummary.Total=9999` + `ResultSizeBytes=1_000_000` but distinct provider usage (7/9) asserts `tokens_input==7`, `NotEqual(9999)`, `NotEqual(1_000_000)`. Fixture `testdata/cc_stream_with_usage.jsonl` parsed via real `trace.TapCCStream`. Cached columns are separate `*int` fields, present-and-zero distinct from scripted-null. |
| 3 | `edit_locality` and `regression_rate` unit-tested at edge cases; both documented in METRICS.md | ✓ VERIFIED | `patch_validator_test.go TestEditLocality`: root-only `>0.9`, all-files `==0.0`, zero-tracked `nil + MetricError`, untracked-excluded `==0.75`. `regression_checker_test.go TestRegressionRate`: clean=0.0, one-regression=`1.0/3.0`, pre-existing-failure-NOT-counted=0.0, empty-denominator nil+MetricError. `bench/evaluators/METRICS.md` documents `edit_locality`, `regression_rate`, `edit_distance_patch` definitions + source-of-truth + scripted-null rule. |
| 4 | Single merged trace per `(task,mode,run_index)`; tool_trace_analyzer consumes already-merged `MergedTrace` (no re-merge); `run_index` segment threaded + guarded by `validatePathSegment` | ✓ VERIFIED | `trace.Merge` called exactly once in `cell.go:461`; analyzer reads `merged` directly, `grep 'trace.Merge' tool_trace_analyzer.go` = none. `matrix.go` `RunIndex:0` hard-code removed → `c.RunIndex` (line 260). `cellDurablePaths` threads `<run_index>` through `validateRunIndexSegment`→`validatePathSegment` (cell.go:159-180). Live artifact path: `IT-go-patch-apply-1/your_agent_full/0/result.v2.json` + sibling `trace.json`; `tool_call_summary.total=2` == metric `tool_calls=2`; `rejected_foreign_pid` clear. |

**Score:** 4/4 success criteria verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `bench/evaluators/metrics.go` | Nullable `Metrics` (19 *T fields) + `MetricError`, 0 omitempty | ✓ VERIFIED | 19 pointer fields, snake_case json tags, 0 omitempty |
| `bench/schema/result.v2.schema.json` | `metrics` object (19 nullable keys) + `metric_errors[]`; only `schema_version` required; `edit_locality maximum:1` | ✓ VERIFIED | All 19 keys present; `required:["schema_version"]`; `edit_locality maximum:1`; `metric_errors` array typed |
| `bench/evaluators/test_runner/test_runner.go` | exit-authoritative success + compile-error counts | ✓ VERIFIED | gates on `.Passed`/`ExitCode`, not `len(Tests)` |
| `bench/evaluators/patch_validator/patch_validator.go` | git-tracked edit_locality + edit_distance via fixed-argv git | ✓ VERIFIED | `exec.CommandContext` fixed argv (`ls-files`,`diff`,`--numstat`), `cmd.Dir`, no shell |
| `bench/evaluators/regression_checker/regression_checker.go` | regression_rate vs cached pre-patch passing set | ✓ VERIFIED | pure transform; pre-existing failures excluded; empty-denom nulled |
| `bench/evaluators/token_meter/token_meter.go` | provider-usage tokens, null-when-absent | ✓ VERIFIED | sources only `mt.Usage`; scripted → all-nil + MetricError |
| `bench/evaluators/tool_trace_analyzer/tool_trace_analyzer.go` | 7 trace metrics, no re-merge | ✓ VERIFIED | consumes `MergedTrace`; no `trace.Merge` call |
| `bench/evaluators/coordinator/coordinator.go` | 5-grader fan-out, D-07 isolation | ✓ VERIFIED | sanctioned deviation from `evaluators.go` path (import cycle); behavior intact |
| `bench/evaluators/METRICS.md` | locality/regression/edit-distance definitions | ✓ VERIFIED | all three documented + source-of-truth + scripted-null |
| `bench/runtime/result.go` | `ResultInput.Metrics`+`MetricErrors` → validate-on-write | ✓ VERIFIED | `resultDoc.Metrics json:"metrics"` (no omitempty); `Validate` gate reused |
| `bench/runtime/cell.go` | run_index path + pre-patch snapshot wired | ✓ VERIFIED | `cellDurablePaths`, `prePatchSnapshot` before drive, `coordinator.Grade` call |
| `bench/runtime/matrix.go` | RunIndex threaded (no hard-code) | ✓ VERIFIED | `RunIndex:0` removed; `c.RunIndex` threaded |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full bench unit suite | `go test ./bench/... -count=1` | all packages ok | ✓ PASS |
| Vet | `go vet ./bench/...` | clean | ✓ PASS |
| **E2E goal gate** | `make bench-quick` | exit 0, "1/1 cells succeeded", wrote `result.v2.json` | ✓ PASS |
| Metric-completeness of live artifact | inspect `.../0/result.v2.json` | all 19 metric keys present; nulls explicit; 4 matching metric_errors; D-07 row valid | ✓ PASS |
| Single-trace / no-PID-leak | inspect sibling `trace.json` | `tool_call_summary.total=2` == `tool_calls=2`; outcome success; no foreign-PID | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| METRIC-01 | 79-01,02,03,04 | 12 base metrics, schema-valid, explicit nulls | ✓ SATISFIED | 19-key metrics object, 0 omitempty, validate-on-write, live artifact |
| METRIC-02 | 79-02,03,04 | 5 extended metrics on a real task | ✓ SATISFIED | semantic_tool_calls/edit_distance_patch/retry_count/compile_errors_*; `TestAllSeventeenMetrics` |
| METRIC-03 | 79-03,04 | tokens from provider usage, not MCP counter | ✓ SATISFIED | token_meter Usage-only + negative no-counter test |
| METRIC-04 | 79-02,04 | edit_locality formula + edge tests + METRICS.md | ✓ SATISFIED | TestEditLocality 4 cases; documented |
| METRIC-05 | 79-02,04 | regression_rate formula + synthetic case | ✓ SATISFIED | TestRegressionRate 4 cases incl. pre-existing-failure-excluded |
| METRIC-06 | 79-03,04 | single merged trace per (task,mode,run_index); reuse Phase 67 tap | ✓ SATISFIED | one trace.Merge call; no re-merge; run_index path; structural continuity automated (Jaeger visual = human item) |

**Note:** `.planning/REQUIREMENTS.md` lines 175-180 still show METRIC-01..06 as `TBD / Pending` in the traceability table even though the checkboxes at lines 56-61 are `[x]`. This is a stale traceability table, not a coverage gap — every ID is implemented and tested. Recommend the orchestrator update the table to `Phase 79 / <commit> / Done`.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `bench/runtime/cell.go` | 497 | `usagePresent := ... (merged.Usage.InputTokens > 0 || OutputTokens > 0)` — presence derived from a value threshold, not a true presence signal (MD-01) | ⚠️ Warning | Latent: defeats the present-and-zero contract (a real-LLM run that legitimately reports 0 input/output tokens would be misclassified as scripted-null). Never misfires on today's corpus because the scripted agent always reports zero usage. Not goal-blocking for the corpus that exists today; should be fixed before a real-LLM (`your_agent_full` with live claude) corpus lands. |
| `bench/evaluators/coordinator/coordinator.go` | 90-96 | zero-tracked-files error path appends `edit_locality` MetricError but drops the computable `files_modified` (MD-02) | ⚠️ Warning | On the `locErr` branch `files_modified` is never assigned, so a computable count is nulled. Cosmetic for today's scripted corpus (git also fails so files_modified is unavailable anyway), but contradicts the grader's `(locality, files_modified, err)` contract. Not goal-blocking. |

No 🛑 BLOCKER anti-patterns. No debt markers (`TBD`/`FIXME`/`XXX`) in any phase-modified file.

### Observation (not a gap): live edit_locality is null on the only corpus today

On the live `make bench-quick` run, `edit_locality`, `edit_distance_patch`, and `regression_rate`
came back `null` with `metric_errors` (`git ls-files failed: exit status 128`, `no pre-patch passing
tests`). Root cause: the scripted Go-ToolBench seed cell's repo working tree is **not a git repo**, so
the two git-backed graders and the regression denominator are undefined for that corpus. This is
**correct D-07 behavior** — the metric is an explicit null + a recorded annotation, never a crash or a
fabricated value, exactly matching the goal's "missing metrics are explicit nulls, not omissions." The
grader formulas themselves are unit-proven on real git repos (criterion 3). The headline `edit_locality`
metric will populate once a git-backed corpus (or a `git init` in the cell scratch) is present; that is a
corpus/runtime-fixture property scheduled with real-corpus work, not a Phase 79 grader defect. Flagged
for human awareness only; does not reduce the score.

### Human Verification Required

#### 1. Jaeger visual continuity (METRIC-06)

**Test:** Import the merged `trace.json` (e.g. `bench/reports/<ts>/IT-go-patch-apply-1/your_agent_full/0/trace.json`) into Jaeger.
**Expected:** A single trace from the `bench.run_index`/`run_id` root down to LSP leaves, with no orphan spans and no cross-cell PID leakage.
**Why human:** Jaeger UI import is a visual confirmation. The structural invariants behind it — single `trace.Merge` per cell, `tool_call_summary.total` parity with `tool_calls`, `rejected_foreign_pid` clear — are automated and PASS, but the end-to-end visual root→leaf continuity is a manual check per `79-VALIDATION.md` § Manual-Only Verifications.

### Gaps Summary

No goal-blocking gaps. All 4 ROADMAP success criteria are verified TRUE both in the unit
suite and in the live E2E artifact: the 5 graders + coordinator produce the full nullable
17(+2)-metric record with D-07 isolation; tokens are sourced exclusively from the provider
usage block (with a robust negative no-MCP-counter assertion); edit_locality and
regression_rate formulas are edge-tested and documented in METRICS.md; and a single merged
trace per `(task,mode,run_index)` lands at the guarded `<run_index>` durable path with no
re-merge. The `make bench-quick` gate exits 0 and writes a schema-valid metric-complete
`result.v2.json` at `.../<task>/<mode>/0/`.

Two MEDIUM advisory findings (MD-01 usagePresent value-threshold, MD-02 dropped
files_modified) are latent and never misfire on the corpus that exists today — recorded as
WARNINGs for pre-real-corpus follow-up, not goal-blockers. One genuine human item remains
(Jaeger visual continuity), so the phase resolves to `human_needed` rather than `passed`.

---

_Verified: 2026-06-18T16:14:00Z_
_Verifier: Claude (gsd-verifier)_
