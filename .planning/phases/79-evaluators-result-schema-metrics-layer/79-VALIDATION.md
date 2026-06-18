---
phase: 79
slug: evaluators-result-schema-metrics-layer
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-18
---

# Phase 79 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Test seams, sampling, and Wave-0 gaps derived from `79-RESEARCH.md` §"Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (+ `jsonschema/v6 v6.0.2` for schema assertions) |
| **Config file** | none — `go test ./bench/...` |
| **Quick run command** | `go test ./bench/evaluators/... ./bench/schema/... -count=1` |
| **Full suite command** | `go vet ./... && go test ./...` |
| **Estimated runtime** | quick ~sub-second (pure transforms over fixtures); full suite per repo norm |

---

## Sampling Rate

- **After every task commit:** Run `go test ./bench/evaluators/... ./bench/schema/... -count=1`
- **After every plan wave:** Run `go vet ./... && go test ./bench/...`
- **Before `/gsd-verify-work`:** Full suite green + `make bench-quick` (≤90s hermetic Go-ToolBench E2E smoke) produces a metric-complete `result.v2.json`
- **Max feedback latency:** ~5 seconds (quick command is sub-second over fixtures)

---

## Per-Task Verification Map

> Task IDs are assigned once PLAN.md files exist. The requirement→test mapping below is from
> `79-RESEARCH.md`; the executor/`gsd-nyquist-auditor` fills the Task ID / Plan / Wave / Status
> columns as plans are created and run.

| Req | Behavior | Test Type | Automated Command | File Exists | Status |
|-----|----------|-----------|-------------------|-------------|--------|
| METRIC-01 | All 12 base metrics present (nullable, not omitted); schema validates | unit | `go test ./bench/schema/... -run TestResultV2Metrics` | ❌ W0 | ⬜ pending |
| METRIC-01/D-07 | One failed grader → that metric `null` + error annotation, row still emitted + schema-valid | unit | `go test ./bench/evaluators/... -run TestPerMetricIsolation` | ❌ W0 | ⬜ pending |
| METRIC-02 | All 5 extended metrics recorded on a real task | integration | `go test ./bench/evaluators/... -run TestAllSeventeenMetrics` | ❌ W0 | ⬜ pending |
| METRIC-03 | `tokens_input/output` sourced from provider `usage`, NOT MCP counter; cached columns separate | unit (fixture stream) | `go test ./bench/evaluators/token_meter/... -run TestProviderUsageSourceOfTruth` | ❌ W0 | ⬜ pending |
| METRIC-03/D-01 | scripted run → tokens `null`, never 0 | unit | `go test ./bench/evaluators/token_meter/... -run TestScriptedNullTokens` | ❌ W0 | ⬜ pending |
| METRIC-04 | `edit_locality` edge cases: root-only ≈ 1.0, all-files ≈ 0.0 (git-tracked denominator) | unit | `go test ./bench/evaluators/patch_validator/... -run TestEditLocality` | ❌ W0 | ⬜ pending |
| METRIC-05 | `regression_rate` on a synthetic case (pre/post double-run, cached passing set) | unit | `go test ./bench/evaluators/regression_checker/... -run TestRegressionRate` | ❌ W0 | ⬜ pending |
| METRIC-06 | Single merged trace per (task,mode,run_index); continuity root→LSP leaves; no orphan/foreign-PID spans | integration | `go test ./bench/evaluators/tool_trace_analyzer/... -run TestTraceMergeContinuity` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `bench/evaluators/metrics.go` — typed nullable `Metrics` struct + `MetricError` (shared test fixtures)
- [ ] `bench/evaluators/evaluators_test.go` — per-metric isolation (D-07) + all-17-present (METRIC-01/02)
- [ ] `bench/evaluators/token_meter/token_meter_test.go` — provider-usage source-of-truth + scripted-null (needs a captured CC stream-json fixture under `testdata/`)
- [ ] `bench/evaluators/patch_validator/patch_validator_test.go` — edit_locality edge cases (tiny git-repo fixtures / `git init` in temp dir)
- [ ] `bench/evaluators/regression_checker/regression_checker_test.go` — pre/post double-run synthetic regression
- [ ] `bench/evaluators/tool_trace_analyzer/tool_trace_analyzer_test.go` — trace metrics over a `MergedTrace` fixture (reuse `trace` test-helper shape)
- [ ] `bench/schema/result.v2_test.go` — extend: assert new `metrics`/`metric_errors` validate AND existing golden fixture still validates (additive-only proof)
- [ ] Framework install: none (stdlib `testing` already in use)

*Edge cases to assert per success criteria are enumerated in `79-RESEARCH.md` §"Validation Architecture > Edge cases to assert".*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| METRIC-06 Jaeger continuity (full root→LSP-leaf visual) | METRIC-06 | Jaeger UI import is a visual confirmation; the automated test asserts span continuity + no orphans/foreign-PID structurally | Import the merged `trace.json` into Jaeger; confirm a single trace from `bench.run_index` root to LSP leaves, no orphan spans |

*All other phase behaviors have automated verification (the source-of-truth METRIC-03 path uses a captured CC stream-json fixture — no live `claude` CLI needed in CI).*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
