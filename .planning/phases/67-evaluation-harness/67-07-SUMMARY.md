---
phase: 67
plan: "07"
subsystem: eval
tags: [eval, judge, llm, informational, ci, anthropic]
one_liner: "Hand-rolled Anthropic HTTP client + informational LLM judge with structural EVAL-07 enforcement; eval-quick CI gate added"
dependency_graph:
  requires: [67-03, 67-05, 67-06a, 67-06b]
  provides: [judge-client, judge-orchestration, ci-eval-gate]
  affects: [cmd/helix-eval, internal/eval/judge, internal/eval/report, .github/workflows]
tech_stack:
  added:
    - "Hand-rolled Anthropic Messages API client (net/http; no SDK dep per Pitfall 9)"
    - "embed.FS for prompts/rubric.md, tool_behavior.tmpl, few_shot.md"
    - "text/template for scoring prompt rendering"
  patterns:
    - "EVAL-07 structural guarantee: judge.Run returns Output only (no error)"
    - "TDD RED/GREEN cycle per task"
    - "Retry-with-exponential-backoff (429/5xx, 3 attempts, 1s/2s/4s default)"
key_files:
  created:
    - internal/eval/judge/client.go
    - internal/eval/judge/client_test.go
    - internal/eval/judge/judge.go
    - internal/eval/judge/judge_test.go
    - internal/eval/judge/prompts/rubric.md
    - internal/eval/judge/prompts/tool_behavior.tmpl
    - internal/eval/judge/prompts/few_shot.md
  modified:
    - internal/eval/report/eval_report.go
    - internal/eval/report/eval_report_test.go
    - cmd/helix-eval/main.go
    - cmd/helix-eval/run_cmd_test.go
    - .github/workflows/go-test.yml
    - Makefile
    - eval/EVAL.md
decisions:
  - "judge.Run signature returns Output only (no error) — structurally prevents judge failure from affecting helix-eval exit code (EVAL-07)"
  - "Default judge model: claude-sonnet-4-6; --judge-model flag accepts 'sonnet' (→ claude-sonnet-4-6) or 'opus' (→ claude-opus-4-5)"
  - "Retry policy: 3 attempts, exponential backoff starting at 1s (1s/2s/4s); 429 and 5xx retryable, 4xx terminal"
  - "Prompt includes ONLY trace event tool names, args_summary, outcome, task kind, task TITLE — patch diff and full task.md body excluded (bias mitigation m4, T-67-05)"
  - "__readme boilerplate: exactly 'INFORMATIONAL — DO NOT USE FOR CI GATING'"
  - "CI grep gate excludes itself (--exclude=go-test.yml) to avoid self-trip"
  - "eval-no-network is an alias for eval-quick for explicit no-network semantics"
metrics:
  duration: "~8 minutes"
  completed: "2026-05-10"
  tasks_completed: 3
  tasks_total: 3
  files_created: 7
  files_modified: 7
---

# Phase 67 Plan 07: LLM Judge and CI Integration Summary

Hand-rolled Anthropic HTTP client + informational LLM judge with structural EVAL-07 enforcement; eval-quick CI gate added.

## What Was Built

### Task 1: Hand-rolled Anthropic HTTP client + judge prompts/rubric

`internal/eval/judge/client.go` implements a minimal Anthropic Messages API client:
- `NewClient(opts Options)` with `BaseURL` test seam, configurable `InitialWait`
- `Score(ctx, system, user string, maxTokens int) (string, error)` — POST JSON; retry on 429/5xx; API key never logged
- Headers: `x-api-key`, `anthropic-version: 2023-06-01`, `content-type: application/json`
- Default model: `claude-sonnet-4-6`
- Retry policy: 3 attempts, exponential backoff 1s/2s/4s; 4xx errors are terminal (no retry)
- All prompt files embedded via `go:embed prompts/*`

Prompt files:
- `prompts/rubric.md`: static rubric for 5 EVAL-05 families (rename/delete/public_api/large_edit/security)
- `prompts/tool_behavior.tmpl`: Go text/template — trace events, task kind, task title, rubric; instructs strict JSON response
- `prompts/few_shot.md`: 2 calibration examples (rename +1, delete -1)

8 tests green, including `TestClientNeverLogsAPIKey` (key not in log output) and `TestRubricFamiliesPresent`.

### Task 2: Judge orchestration — trace-only input, INFORMATIONAL boilerplate, never gates exit code

`internal/eval/judge/judge.go`:
- `Run(ctx, client, inputs, model) Output` — structurally cannot return error (EVAL-07)
- `RunWithOptions(ctx, client, inputs, model, RunOptions{NoJudge}) Output` — `--no-judge` flag skip path
- Missing `ANTHROPIC_API_KEY` → `judge_skipped: true, reason: "no api key"`
- `--no-judge=true` → `judge_skipped: true, reason: "explicit --no-judge"`
- API errors fold into `judge_failed: true, error_summary: "..."` (never propagated)
- Prompt rendering via `BuildPrompt(Input)` — includes ONLY trace events, task kind, task title (NOT patch diff or full task.md body — bias mitigation m4, T-67-05)
- 60s per-item context timeout prevents stall
- `__readme` boilerplate: `"INFORMATIONAL — DO NOT USE FOR CI GATING"` (asserted by `TestJudgeOutputBoilerplate`)
- `WriteJudgeReport(path, Output)` always writes the file (even stubs)

`internal/eval/report/eval_report.go`:
- `WriteEvalReportWithJudge(jsonPath, mdPath, judgeReportPath, ...)` reads judge file when present
- Markdown `eval_report.md` renders "Informational: LLM judge" section with table or "(judge not run)" fallback
- `renderMarkdownWithJudge` replaces the static placeholder from prior plans

`cmd/helix-eval/main.go`:
- Judge pass wired after `RunMatrix` in non-quick path
- Exit code computed exclusively from per-task results — EVAL-07 comment block at exit-code site
- `--no-judge` default changed to `false` (judge skips automatically on missing key anyway)

9 judge tests green + `TestEvalReportMarkdownIncludesJudgeSection` + `TestRunMatrixExitCodeIgnoresJudge`.

### Task 3: CI workflow + grep gate + EVAL.md updates

`.github/workflows/go-test.yml`:
- Added `eval-quick (harness validation)` step: `run: make eval-quick`, `timeout-minutes: 2`
- Added `forbid judge in CI` step: `grep -r "tool_behavior_judge" .github/workflows/ --exclude="go-test.yml"` — fails with EVAL-07 message if found
- `make eval` is NOT present in any workflow file (project memory rule: benchmarks local-only)

`Makefile`:
- Added `eval-no-network` alias (identical to `eval-quick`; clarity alias for no-external-network)
- `eval-quick` comment block updated to state it's the only PR-gating eval target

`eval/EVAL.md`:
- Added "EVAL-07: LLM Judge Informational Status" section with four-layer mitigation summary
- Quick troubleshooting paragraph for CI failures mentioning `tool_behavior_judge`
- Added "ZDR Operator Checklist (per quarter)" with DPA/key verification items

## Judge Configuration Reference

| Parameter | Value |
|-----------|-------|
| Default model (flag: `sonnet`) | `claude-sonnet-4-6` |
| Opus model (flag: `opus`) | `claude-opus-4-5` |
| Retry attempts | 3 |
| Initial retry wait | 1s (default); test seam via `InitialWait` option |
| Retry wait schedule | 1s, 2s, 4s (exponential) |
| Per-item timeout | 60s |
| Retryable status codes | 429, 500-599 |
| Terminal status codes | 400-428, 430-499 |
| Exact `__readme` boilerplate | `"INFORMATIONAL — DO NOT USE FOR CI GATING"` |

## CI Workflow Reference

| File | Line region | What it does |
|------|-------------|-------------|
| `.github/workflows/go-test.yml` | After `go test` step | `make eval-quick` as PR gate |
| `.github/workflows/go-test.yml` | After eval-quick step | `forbid judge in CI` grep gate |

## Deviations from Plan

### Auto-adjustments

**1. [Rule 2 - Missing critical] WriteEvalReportWithJudge added alongside WriteEvalReport**
- **Found during:** Task 2 implementation
- **Issue:** Plan called for extending `eval_report.go` markdown writer; existing `WriteEvalReport` called from `main.go` and tests; changing its signature would break callers
- **Fix:** Added new `WriteEvalReportWithJudge` function that accepts `judgeReportPath` parameter; `main.go` updated to call it; `WriteEvalReport` preserved unchanged for existing callers
- **Files modified:** `internal/eval/report/eval_report.go`, `cmd/helix-eval/main.go`

**2. [Rule 1 - Bug] httptest server hang in TestClientHonorsContextDeadline**
- **Found during:** Task 1 GREEN
- **Issue:** Test took 10s instead of <1s due to httptest.Server waiting for hung connection to drain
- **Fix:** Used `done` channel to signal handler to unblock on Close(), reducing test time to ~0.1s
- **Files modified:** `internal/eval/judge/client_test.go`

**3. [Rule 2 - Missing] TestWriteJudgeReport added to judge_test.go**
- **Found during:** Task 2 implementation  
- **Issue:** Plan specified 9 judge tests; one (file I/O verification) was not included in RED tests but is needed for WriteJudgeReport coverage
- **Fix:** Added `TestWriteJudgeReport` to verify JSON file is written with `__readme` boilerplate

## Known Stubs

None — all plan objectives implemented and wired.

## Threat Surface Scan

No new network endpoints, auth paths, file access patterns, or schema changes at trust boundaries beyond what the threat model documented. The Anthropic API client (eval runner ↔ Anthropic API boundary) is covered by T-67-Pitfall-1 (API key logging) and T-67-09 (API outage) — both mitigated.

## Self-Check: PASSED

Files verified present:
- internal/eval/judge/client.go ✓
- internal/eval/judge/judge.go ✓
- internal/eval/judge/prompts/rubric.md ✓
- internal/eval/judge/prompts/tool_behavior.tmpl ✓
- internal/eval/judge/prompts/few_shot.md ✓
- .github/workflows/go-test.yml (updated) ✓
- eval/EVAL.md (updated) ✓

Commits verified:
- 61efe9af test(67-07): RED client tests ✓
- 2cc06296 feat(67-07): client.go + prompts ✓
- 03850faa test(67-07): RED judge tests ✓
- 063b5180 feat(67-07): judge.go + report integration ✓
- 3065e32a feat(67-07): CI + grep gate + EVAL.md ✓
