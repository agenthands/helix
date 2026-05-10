---
phase: 67
plan: 05
subsystem: eval
tags: [eval, runner, zdr, reports, phasegraph, dag-02]
dependency_graph:
  requires: [67-01, 67-02, 67-03, 67-04]
  provides: [runner, zdr-gate, aggregate-reports, helix-eval-run]
  affects: [internal/eval/pipeline.go, cmd/helix-eval/main.go]
tech_stack:
  added: []
  patterns: [phasegraph-run-bodies, tdd-red-green, direct-string-rendering, json-0600-files]
key_files:
  created:
    - internal/eval/runner/zdr_gate.go
    - internal/eval/runner/zdr_gate_test.go
    - internal/eval/runner/runner.go
    - internal/eval/runner/runner_test.go
    - internal/eval/report/eval_result.go
    - internal/eval/report/eval_result_test.go
    - internal/eval/report/run_metadata.go
    - internal/eval/report/run_metadata_test.go
    - internal/eval/report/cost_summary.go
    - internal/eval/report/cost_summary_test.go
    - internal/eval/report/safety_compliance.go
    - internal/eval/report/safety_compliance_test.go
    - internal/eval/report/eval_report.go
    - internal/eval/report/eval_report_test.go
    - cmd/helix-eval/run_cmd_test.go
  modified:
    - internal/eval/pipeline.go
    - cmd/helix-eval/main.go
    - cmd/helix-eval/main_test.go
    - .gitignore
decisions:
  - "EvalPhases Run bodies use a shared *RunState passed via BuildEvalPhases closure capture; pipeline.go exposes both Phases (shape-only, backward-compat) and BuildEvalPhases (real bodies)"
  - "Runner.RunTask uses direct subprocess orchestration without the phasegraph at the runner layer; BuildEvalPhases is available for callers needing the full phase-graph API"
  - "renderMarkdown uses direct bytes.Buffer string building rather than text/template to avoid complex map-key iteration issues in Go templates"
  - "T-67-03 enforced via hard-coded switch in writeModeConfig; unknown modes return error before any file write"
  - "T-67-Pitfall-1 enforced in CaptureRunMetadata: EnvKeys captures key names only, never values"
  - "Wave-0 TestHelixEvalRunNotImplemented placeholder test updated to TestHelixEvalRunEmptyCorpusSucceeds"
metrics:
  duration: "11m 57s"
  completed: "2026-05-10T12:16:13Z"
  tasks_completed: 3
  files_changed: 18
---

# Phase 67 Plan 05: Runner and Reporters Summary

**One-liner:** Per-(task,mode) phasegraph runner with EVAL-06 ZDR corpus gate, all 5 EVAL-04 aggregate reporters, and wired `helix-eval run` end-to-end.

## What Was Built

### Task 1 — ZDR Corpus Gate + EvalResult Schema (commit 8db2e2c1)

**ZDR gate** (`internal/eval/runner/zdr_gate.go`): `AssertCorpusAllowed` enforces the EVAL-06 Zero Data Retention policy:
- `synthetic` (default, no source.yaml) → always allowed
- `helix-oss` (corpusDir resolves under the helix module root via go.mod scan) → allowed with WARN log
- `external` (source.yaml declares `source: external`) → blocked unless `HELIX_EVAL_ZDR_VERIFIED=1` (T-67-06 mitigation)

**EvalResult** (`internal/eval/report/eval_result.go`): Full EVAL-01 schema with 13 fields. `ContextPrecision` and `ContextRecall` are `*float64` (nullable) — serialize as JSON `null` until ground-truth labels exist (post-phase-67 TODO).

**WriteResult**: writes 0600 JSON artifact to `eval/reports/<run-id>/tasks/<task>/<mode>/result.json`.

Tests: 4 ZDR gate tests + 2 result schema/write tests = **6 green**.

### Task 2 — Runner + EvalPhases Run Bodies (commit 2bfdd530)

**Runner** (`internal/eval/runner/runner.go`):
- `NewRunner(Config)` — corpus dir, out dir, helix bin, run-id, max-parallel (default 1 per D-05)
- `RunTask(ctx, sandbox, TaskSpec)` — runs all 10 DAG-02 phases inline, returns `*EvalResult`
- `RunMatrix(ctx, modes)` — walks corpus dirs × modes with semaphore-bounded concurrency

**pipeline.go** — `BuildEvalPhases(state *RunState)` returns 10 `PhaseSpec`s with real Run closures capturing `*RunState`:

| Phase | Run body |
|-------|----------|
| `prepare_workspace` | `Sandbox.Prepare` + `CloneRepo` + `git init baseline` |
| `configure_mode` | `writeModeConfig` (hard-coded enum, T-67-03) |
| `run_agent` | context-cancellation → budget breach; real subprocess in Plan 06 |
| `collect_trace` | `TapDaemonLog` + `TapCCStream` + `trace.Merge` → `trace.json` |
| `apply_patch_check` | `git diff HEAD` → `patch.diff`; `PatchApplies` set |
| `run_tests` | `verify.sh` → `verify.log`; `TestsPass` set |
| `run_diagnostics` | v1 placeholder: `DiagnosticsClean = TestsPass` (TODO post-67) |
| `score_tool_behavior` | `score.LoadRules` + `score.Apply` from `expected_tools.yaml` |
| `score_guardrails` | populate `GuardrailCompliance` from `Merged.Guardrails` |
| `aggregate_report` | resolve outcome + write `result.json` |

Tests: 6 runner tests = **6 green**, race-clean.

### Task 3 — Aggregate Reporters + helix-eval run (commit 08e7e812)

**5 EVAL-04 reporters:**

| File | Reporter | Key invariant |
|------|----------|---------------|
| `run_metadata.go` | `CaptureRunMetadata` + `WriteRunMetadata` | EnvKeys = names only (T-67-Pitfall-1) |
| `cost_summary.go` | `WriteCostSummary` | No dollar conversion (D-07 explicit) |
| `safety_compliance.go` | `WriteSafetyCompliance` | Per-mode guardrail count aggregation |
| `eval_report.go` | `WriteEvalReport` (JSON + MD) + `WriteToolBehavior` | MD has explicit EVAL-07 LLM judge boilerplate |
| _(via eval_result.go)_ | `WriteResult` per (task,mode) | Ships with Task 1 |

**eval_report.md sections** (all rendered):
1. Header (run-id, claude version, helix version, date)
2. Mode Comparison table (success/latency/token rows)
3. Tool-Call Distribution (top 10 per mode)
4. Guardrail Compliance table
5. Failure Examples (up to 3, sorted by mode then task)
6. INFORMATIONAL: LLM Judge (with EVAL-07 boilerplate + "(judge not run)" when absent)

**`helix-eval run` wired**:
1. Parse flags
2. `runner.AssertCorpusAllowed` (ZDR gate)
3. `runner.RunMatrix` → per-(task,mode) result.json files
4. Write all 6 aggregate reports under `<out>/<run-id>/`
5. Print summary; exit 0 if all succeeded, 1 if any failed (judge never affects exit code, EVAL-07)

Tests: 9 reporter tests + 4 cmd tests = **13 green**, race-clean.

## EvalResult Schema (final, EVAL-01)

```go
type EvalResult struct {
    TaskID           string                `json:"task_id"`
    Mode             string                `json:"mode"`
    Success          bool                  `json:"success"`
    PatchApplies     bool                  `json:"patch_applies"`
    TestsPass        bool                  `json:"tests_pass"`
    DiagnosticsClean bool                  `json:"diagnostics_clean"`
    DurationMs       int64                 `json:"duration_ms"`
    Tokens           struct{ Input, Output int } `json:"tokens"`
    EditCount        int                   `json:"edit_count"`
    GuardrailCompliance trace.GuardrailCounts `json:"guardrail_compliance"`
    ContextPrecision *float64              `json:"context_precision"` // null until Phase 68+
    ContextRecall    *float64              `json:"context_recall"`    // null until Phase 68+
    Outcome          string                `json:"outcome"`
    FailureReason    string                `json:"failure_reason,omitempty"`
}
```

## ZDR Gate Corpus-Source Enum

```go
const (
    CorpusSynthetic CorpusSource = "synthetic"  // default; no source.yaml
    CorpusHelixOSS  CorpusSource = "helix-oss"  // resolves under go.mod helix root
    CorpusExternal  CorpusSource = "external"   // requires HELIX_EVAL_ZDR_VERIFIED=1
)
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Wave-0 placeholder test TestHelixEvalRunNotImplemented updated**
- **Found during:** Task 3 — implementing the run command body
- **Issue:** The pre-existing test expected `"not yet implemented"` error; now the command is implemented
- **Fix:** Replaced with `TestHelixEvalRunEmptyCorpusSucceeds` that verifies all 6 report files are written for an empty corpus
- **Files modified:** `cmd/helix-eval/main_test.go`
- **Commit:** 08e7e812

**2. [Rule 2 - Missing] Gitignore entries for runtime artifacts**
- **Found during:** Task 3 commit — `helix-eval` binary and `eval/reports/` created during test execution appeared as untracked
- **Fix:** Added `/helix-eval`, `/eval/reports/`, `/cmd/helix-eval/eval/` to `.gitignore`
- **Files modified:** `.gitignore`
- **Commit:** 08e7e812

**3. [Rule 2 - Design] renderMarkdown uses direct string building instead of text/template**
- **Found during:** Task 3 implementation — `text/template` with `map[string]modeAggregate` iteration produced compilation errors (struct with slice field cannot be used as map key)
- **Fix:** Replaced template approach with `bytes.Buffer` direct string building, which is simpler and equally readable
- **Files modified:** `internal/eval/report/eval_report.go`
- **Commit:** 08e7e812

## Threat Model Coverage

| Threat | Mitigation | Verified by |
|--------|-----------|-------------|
| T-67-06 (ZDR leakage) | `AssertCorpusAllowed` blocks `external` without env-var | TestZDRGate_NonSyntheticBlocked |
| T-67-Pitfall-1 (env values) | `captureEnvKeys` records names only | TestRunMetadataCapturesClaudeVersion |
| T-67-Pitfall-8 (judge gates) | Separate "INFORMATIONAL" section + "(judge not run)" | TestEvalReportRendersWithoutLLMJudge |
| T-67-03 (mode tamper) | `writeModeConfig` hard-coded switch; unknown mode → error | Test coverage in TestRunnerMatrix |

## Self-Check

Checking files exist and commits are recorded:

- FOUND: internal/eval/runner/zdr_gate.go
- FOUND: internal/eval/runner/runner.go
- FOUND: internal/eval/report/eval_result.go
- FOUND: internal/eval/report/eval_report.go
- FOUND: internal/eval/report/cost_summary.go
- FOUND: internal/eval/report/safety_compliance.go
- FOUND: internal/eval/report/run_metadata.go
- FOUND: internal/eval/pipeline.go
- FOUND: cmd/helix-eval/main.go
- FOUND: .planning/phases/67-evaluation-harness/67-05-SUMMARY.md
- COMMIT FOUND: 8db2e2c1
- COMMIT FOUND: 2bfdd530
- COMMIT FOUND: 08e7e812

## Self-Check: PASSED
