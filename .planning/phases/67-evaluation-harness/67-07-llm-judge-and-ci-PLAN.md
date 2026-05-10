---
phase: 67
plan: 07
type: tdd
wave: 5
depends_on: [67-03, 67-05, 67-06a, 67-06b]
autonomous: true
requirements: [EVAL-07, EVAL-04]
files_modified:
  - internal/eval/judge/judge.go
  - internal/eval/judge/judge_test.go
  - internal/eval/judge/client.go
  - internal/eval/judge/client_test.go
  - internal/eval/judge/prompts/rubric.md
  - internal/eval/judge/prompts/tool_behavior.tmpl
  - internal/eval/judge/prompts/few_shot.md
  - cmd/helix-eval/main.go
  - .github/workflows/ci.yml
  - Makefile
  - eval/EVAL.md
tags: [eval, judge, llm, informational, ci]

must_haves:
  truths:
    - "Judge runs on the merged trace ONLY (no raw patch diff, no task.md content) — bias mitigation per RESEARCH §'Judge Bias Mitigations'"
    - "Judge writes tool_behavior_judge.json with embedded INFORMATIONAL boilerplate (Pitfall 8 mitigation)"
    - "Judge failure / timeout / API error NEVER affects helix-eval exit code (EVAL-07 hard constraint)"
    - "--no-judge flag (and missing ANTHROPIC_API_KEY) skips judge silently"
    - "--judge-model flag accepts sonnet (default) or opus"
    - "PR-gating CI workflow runs make eval-quick; full make eval is excluded from PR-gating (project memory rule: benchmarks local-only)"
    - "A linter/grep gate fails the build if tool_behavior_judge.json is referenced from any file under .github/workflows/"
  artifacts:
    - path: "internal/eval/judge/judge.go"
      provides: "Per-(task,mode) trace → judge JSON output"
      exports: ["Judge", "Run", "WriteJudgeReport"]
    - path: "internal/eval/judge/client.go"
      provides: "Anthropic API client (hand-rolled net/http; no SDK dep this phase)"
      exports: ["NewClient", "(*Client).Score"]
    - path: ".github/workflows/ci.yml"
      provides: "PR gate: make eval-quick only; never make eval"
    - path: "internal/eval/judge/prompts/rubric.md"
      provides: "Static rubric per EVAL-05 task family"
  key_links:
    - from: "internal/eval/judge/judge.go"
      to: "internal/eval/judge/client.go"
      via: "POST https://api.anthropic.com/v1/messages with merged trace summary"
      pattern: "api\\.anthropic\\.com"
    - from: "cmd/helix-eval/main.go"
      to: "internal/eval/judge/judge.go"
      via: "post-aggregate optional pass"
      pattern: "judge\\.Run"
---

<objective>
Wave 5: LLM judge (informational) + CI integration. Per EVAL-07, judge MUST be informational and NEVER gate merges. Per project memory rule, `make eval` is local-only and never PR-gated.

Purpose: Adds qualitative depth to tool-behavior scoring (catches novel patterns the heuristic DSL misses) without compromising the merge-gate signal. CI integration is the final wave because it depends on every prior wave being green.

Output: hand-rolled Anthropic HTTP client (assumption A2 fallback per RESEARCH); judge module; INFORMATIONAL boilerplate in output JSON; CI workflow that runs eval-quick on PRs and explicitly excludes eval; lint gate that fails if a future PR wires judge into CI.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/phases/67-evaluation-harness/67-RESEARCH.md
@.planning/REQUIREMENTS.md

<interfaces>
Anthropic Messages API (verified RESEARCH §"LLM Judge Design"):
POST https://api.anthropic.com/v1/messages
Headers: x-api-key, anthropic-version: 2023-06-01, content-type: application/json
Body: {"model": "claude-sonnet-4-6", "max_tokens": N, "messages": [...], "system": "..."}
Response: {"content": [{"type":"text","text":"..."}], "usage":{...}, "stop_reason":...}

We hand-roll instead of taking the SDK dep (RESEARCH Assumption A2 fallback): keeps go.mod clean and avoids any CGO pull-in risk per Pitfall 9.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Hand-rolled Anthropic HTTP client + judge prompts/rubric</name>
  <files>
    internal/eval/judge/client.go,
    internal/eval/judge/client_test.go,
    internal/eval/judge/prompts/rubric.md,
    internal/eval/judge/prompts/tool_behavior.tmpl,
    internal/eval/judge/prompts/few_shot.md
  </files>
  <behavior>
    - Test 1 (RED): TestClientPostsCorrectShape — using `httptest.Server`, assert request has `x-api-key`, `anthropic-version`, body.model="claude-sonnet-4-6", body.max_tokens set, system prompt and user message present.
    - Test 2 (RED): TestClientReturnsTextContent — server returns canned response; client extracts text correctly.
    - Test 3 (RED): TestClientHonorsContextDeadline — server hangs; ctx with 100ms timeout returns ctx.Err.
    - Test 4 (RED): TestClientHandles429 — server returns 429; client retries with exponential backoff up to 3 attempts; final return is the success response.
    - Test 5 (RED): TestClientHandles5xxAsRetryable — server returns 503 once then 200; client succeeds.
    - Test 6 (RED): TestClientHandles4xxAsTerminal — server returns 401; client returns error immediately, no retry.
    - Test 7 (RED): TestClientNeverLogsAPIKey — assertion that a captured logger output never contains the API key value (defensive).
    - Test 8: TestRubricFamiliesPresent — `prompts/rubric.md` contains the five EVAL-05 family headers (rename, delete, public_api, large_edit, security).
  </behavior>
  <action>
    `internal/eval/judge/client.go`:
    - `type Client struct { baseURL, apiKey, model string; httpc *http.Client; logger *slog.Logger }`
    - `func NewClient(opts Options) *Client`. Default `baseURL = "https://api.anthropic.com/v1/messages"`, `model = "claude-sonnet-4-6"`. Test seam via opts.BaseURL.
    - `(c *Client) Score(ctx context.Context, system, user string, maxTokens int) (string, error)` — POST JSON; retry on 429 + 5xx (exponential backoff 1s/2s/4s, max 3 retries); never log apiKey.
    - Headers: `x-api-key`, `anthropic-version: 2023-06-01`.

    `internal/eval/judge/prompts/rubric.md`: copy the rubric table from RESEARCH §"Rubric Per EVAL-05 Family" verbatim (5 rows: rename, delete, public_api, large_edit, security). Add a header noting the rubric is intentionally static for cross-run comparability.

    `internal/eval/judge/prompts/tool_behavior.tmpl`: Go text/template per RESEARCH §"Prompt Structure". Variables: `.TaskKind, .TaskDescription, .Events (with .T, .Tool, .ArgsSummary, .Outcome fields), .Outcome, .TestsPass, .Rubric`. Output instruction: strict JSON `{"scores":{...},"reasoning":"...","flags":[...]}`.

    `internal/eval/judge/prompts/few_shot.md`: 2 worked examples (one rename family +1, one delete family -1). Optional in template; loaded only when present.

    All prompt files are read at runtime via `embed.FS` in client.go's package (`go:embed prompts/*`). Tests use the embedded FS directly to assert content.
  </action>
  <verify>
    <automated>go test ./internal/eval/judge -run "TestClient|TestRubric" -count=1 -race && grep -q "rename" internal/eval/judge/prompts/rubric.md && grep -q "delete" internal/eval/judge/prompts/rubric.md && grep -q "public_api" internal/eval/judge/prompts/rubric.md && grep -q "large_edit" internal/eval/judge/prompts/rubric.md && grep -q "security" internal/eval/judge/prompts/rubric.md</automated>
  </verify>
  <done>
    Eight tests green. Anthropic API client hand-rolled, retry-aware, secret-safe. Rubric covers all five EVAL-05 families.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Judge orchestration — trace-only input, INFORMATIONAL boilerplate, never gates exit code</name>
  <files>
    internal/eval/judge/judge.go,
    internal/eval/judge/judge_test.go,
    cmd/helix-eval/main.go
  </files>
  <behavior>
    - Test 1 (RED): TestJudgeBuildsPromptFromTraceOnly — given a MergedTrace + EvalResult, the rendered prompt does NOT contain raw patch diff bytes nor raw task.md content (bias mitigation m4 per RESEARCH).
    - Test 2 (RED): TestJudgeAggregatesPerTaskMode — given 3 (task, mode) pairs, `judge.Run(...)` produces one entry per pair in tool_behavior_judge.json with task_id, mode, scores{right_tool, evidence, blast_radius, recovery}, reasoning, flags.
    - Test 3 (RED): TestJudgeOutputBoilerplate — emitted JSON has top-level `__readme: "INFORMATIONAL — DO NOT USE FOR CI GATING"` (Pitfall 8 mitigation per CONTEXT.md and RESEARCH).
    - Test 4 (RED): TestJudgeAPIErrorDoesNotPropagate — when API client returns 500-class error, `judge.Run` writes a stub `tool_behavior_judge.json` containing `{"__readme": "...", "judge_failed": true, "error_summary": "..."}` and returns nil error. The runner must NOT fail.
    - Test 5 (RED): TestJudgeMissingAPIKeySkipsCleanly — `ANTHROPIC_API_KEY=""` → `judge.Run` writes the stub with `judge_skipped: true, reason: "no api key"`; no API call made.
    - Test 6 (RED): TestNoJudgeFlagSkips — `--no-judge` flag → no API call, stub written with `judge_skipped: true, reason: "explicit --no-judge"`.
    - Test 7 (RED): TestJudgeModelFlagAcceptsSonnetOpus — `--judge-model=opus` → client uses `claude-opus-4-5` (or current Opus tag); unknown values rejected.
    - Test 8 (integration): TestEvalReportMarkdownIncludesJudgeSection — when judge output exists, eval_report.md has an "Informational: LLM judge" section quoting the boilerplate; when missing, the section is replaced with "(judge not run)".
    - Test 9 (CRITICAL): TestRunMatrixExitCodeIgnoresJudge — set up runner so all task results have success=true; have judge fail; assert helix-eval exits 0.
  </behavior>
  <action>
    `internal/eval/judge/judge.go`:
    ```go
    type Input struct {
        TaskID, Mode string
        TaskKind string // from expected_tools.yaml
        TaskDescription string // from task.md (DESCRIPTION ONLY, not full content — pass first line / title)
        Trace trace.MergedTrace
        Result report.EvalResult
    }
    type Scores struct{ RightTool, Evidence, BlastRadius, Recovery int }
    type Entry struct{ TaskID, Mode string; Scores Scores; Reasoning string; Flags []string }
    type Output struct {
        Readme string `json:"__readme"`
        JudgeModel string `json:"judge_model"`
        JudgedAt time.Time `json:"judged_at"`
        Tasks []Entry `json:"tasks"`
        JudgeSkipped bool `json:"judge_skipped,omitempty"`
        JudgeFailed bool `json:"judge_failed,omitempty"`
        Reason string `json:"reason,omitempty"`
        ErrorSummary string `json:"error_summary,omitempty"`
    }
    func Run(ctx context.Context, client *Client, inputs []Input, model string) Output
    func WriteJudgeReport(path string, o Output) error
    ```
    - The boilerplate string is exactly: `"INFORMATIONAL — DO NOT USE FOR CI GATING"` (matches CONTEXT.md verbatim).
    - `Run` swallows all errors and folds them into the Output struct; signature returns `Output` only (no error). The runner CANNOT propagate a judge failure into its exit code. This is enforced by signature, not just convention.
    - `Run` builds the prompt by rendering `tool_behavior.tmpl` with: TaskKind, TaskDescription (TITLE ONLY — first line of task.md), event list (timestamp, tool name, args_summary, outcome), Result.Outcome, Result.TestsPass, Rubric (read from embedded prompts/rubric.md). Patch diff and full task.md content are EXPLICITLY excluded.
    - Each call uses `context.WithTimeout(ctx, 60*time.Second)` so a slow API can never stall the run.

    `cmd/helix-eval/main.go`:
    - Add flags `--no-judge` (bool, default false) and `--judge-model` (string, default "sonnet"; accepts "sonnet"|"opus"; map to actual model strings).
    - After RunMatrix completes, call `judge.Run` (unless flagged off or API key missing); always write `tool_behavior_judge.json` (the file ALWAYS exists — even as a stub — so the report can reason about its presence).
    - Exit code computation: ONLY consider per-task results, NEVER the judge output. Add a comment block at the exit-code site explicitly citing EVAL-07.

    `internal/eval/report/eval_report.go`: extend the markdown writer to read `tool_behavior_judge.json` if present and render the "Informational: LLM judge" section (TestEvalReportMarkdownIncludesJudgeSection covers this).
  </action>
  <verify>
    <automated>go test ./internal/eval/judge -count=1 -race && go test ./internal/eval/report -run "TestEvalReportMarkdownIncludesJudgeSection" -count=1 && go test ./cmd/helix-eval -run "TestRunMatrixExitCodeIgnoresJudge" -count=1</automated>
  </verify>
  <done>
    Nine judge-related tests green. Judge errors structurally cannot propagate (Run returns Output only). __readme boilerplate enforced. Trace-only input asserted (no patch, no task.md body).
  </done>
</task>

<task type="auto">
  <name>Task 3: CI workflow + grep gate forbidding judge usage in CI</name>
  <files>
    .github/workflows/ci.yml,
    Makefile,
    eval/EVAL.md
  </files>
  <action>
    Update `.github/workflows/ci.yml` (read existing; do NOT clobber unrelated jobs):
    1. Locate the existing `test` / `unit` job. Add a step at the end (or a sibling job) named `eval-quick` that runs:
       ```
       - name: eval-quick (harness validation)
         run: make eval-quick
         timeout-minutes: 2
       ```
       This is the PR gate.
    2. EXPLICITLY do NOT add `make eval` anywhere in `.github/workflows/`.
    3. Add a grep gate step (in any job, e.g., a `lint` job; or a tiny dedicated job):
       ```
       - name: forbid judge in CI
         run: |
           if grep -r "tool_behavior_judge" .github/workflows/ ; then
             echo "ERROR: tool_behavior_judge.json must NEVER be referenced in CI workflows (EVAL-07)."
             exit 1
           fi
           # Allow this file itself (the grep gate's own self-reference would match):
           # We grep with --exclude on this file via the pipeline above.
       ```
       Refine: the grep must exclude the workflow file itself (otherwise the gate self-trips). Use `grep -r --exclude="$(basename "$GITHUB_WORKFLOW")"` or a known workflow name.

    `Makefile`: ensure the `eval-quick` target's comment block explicitly states it's the only PR-gating eval target. Add a `make eval-no-network` target convenience (alias of eval-quick) for clarity.

    `eval/EVAL.md`: append an "Operator notes" section listing the four mitigations for EVAL-07 (separate JSON file, INFORMATIONAL __readme, judge errors swallowed in Run signature, CI grep gate). Add a quick-troubleshooting paragraph: "If a CI failure mentions `tool_behavior_judge`, the grep gate caught a regression; revert the workflow change."

    Also add the operator-checklist for ZDR per Pitfall 7 mitigation if not already present from Plan 01:
    ```
    ## ZDR Operator Checklist (per quarter)
    - [ ] Anthropic DPA still active for the API key in use
    - [ ] HELIX_EVAL_ZDR_VERIFIED=1 ONLY when point above is true
    - [ ] No external corpora committed to eval/corpus/ without explicit code review
    ```
  </action>
  <verify>
    <automated>grep -E "make eval-quick|eval-quick:" .github/workflows/ci.yml && ! grep -E "^\\s*run: make eval$" .github/workflows/ci.yml && grep -q "tool_behavior_judge" .github/workflows/ci.yml && grep -q "ZDR Operator Checklist" eval/EVAL.md && grep -q "INFORMATIONAL" eval/EVAL.md</automated>
  </verify>
  <done>
    CI workflow runs `make eval-quick` on PRs; `make eval` is NEVER invoked from CI; grep gate fails if a future PR wires judge into a workflow; EVAL.md has operator checklist + EVAL-07 mitigations summary.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Eval runner ↔ Anthropic API | Untrusted network; TLS via stdlib http.DefaultTransport defaults. |
| Judge prompt ↔ task content | Task content excluded from prompt (only title); patch excluded entirely; bias mitigation m4. |
| Judge output ↔ CI exit code | Structurally separated: `judge.Run` returns Output only (no error). |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-67-05 | T (Tampering) | judge prompt-injection from task content | mitigate | Judge sees ONLY merged trace events + task TITLE (first line of task.md). Patch diff and full task.md body are explicitly excluded; TestJudgeBuildsPromptFromTraceOnly asserts. Rubric is in-repo and never extends from task content. |
| T-67-Pitfall-8 | (judge gates merges) | runner exit code | mitigate | (a) `judge.Run` signature returns Output ONLY, no error — structurally cannot propagate. (b) `__readme` boilerplate. (c) CI grep gate fails any future workflow that references `tool_behavior_judge`. (d) EVAL.md operator notes. Five-layer defense. |
| T-67-Pitfall-1 | I (Info Disclosure) | API key in logs | mitigate | TestClientNeverLogsAPIKey asserts; client takes apiKey via opts struct, never via env-read inside the package, never logged. |
| T-67-09 | (network reliability) | Anthropic API outage causes flaky CI | mitigate | Judge runs ONLY post-aggregate, never on the eval-quick path (which is the PR gate). Pitfall-8 mitigation also covers this: a complete API outage produces a stub file and exit 0. |

Block on: HIGH severity. T-67-05 and Pitfall-8 are HIGH; both have multi-layer mitigations with assertion tests.
</threat_model>

<verification>
- `go test ./internal/eval/judge -race` green.
- TestRunMatrixExitCodeIgnoresJudge proves a failing judge cannot block a passing eval.
- CI workflow runs eval-quick only; grep gate enforces no judge ref in workflows.
- EVAL.md has the 5 EVAL-07-mitigation summary + ZDR operator checklist.
</verification>

<success_criteria>
- [ ] `internal/eval/judge/client.go` + 8 tests green.
- [ ] `internal/eval/judge/judge.go` + 9 tests green (Tests 1-9).
- [ ] `internal/eval/judge/prompts/{rubric.md, tool_behavior.tmpl, few_shot.md}` shipped.
- [ ] `.github/workflows/ci.yml` runs make eval-quick; never make eval.
- [ ] CI grep gate fails on `tool_behavior_judge` reference in any workflow file.
- [ ] EVAL.md updated with EVAL-07 mitigation summary + ZDR operator checklist.
- [ ] No new dependencies added to go.mod (hand-rolled HTTP per Pitfall 9 + RESEARCH §"Don't Hand-Roll").
</success_criteria>

<dependencies>
- Plans: 67-03 (trace.MergedTrace), 67-05 (runner + reports + EvalResult), 67-06a (eval-quick plumbing) + 67-06b (full 10-fixture set).
- External: ANTHROPIC_API_KEY for live judge runs (optional; missing key just skips silently).
</dependencies>

<output>
After completion, create `.planning/phases/67-evaluation-harness/67-07-SUMMARY.md` recording: judge model defaults, retry policy parameters, exact `__readme` boilerplate text, CI workflow file path + line number where eval-quick is wired.
</output>
