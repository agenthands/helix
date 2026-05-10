---
phase: 67
plan: 05
type: tdd
wave: 3
depends_on: [67-01, 67-02, 67-03, 67-04]
autonomous: true
requirements: [EVAL-01, EVAL-02, EVAL-04, EVAL-06]
files_modified:
  - internal/eval/runner/runner.go
  - internal/eval/runner/runner_test.go
  - internal/eval/runner/zdr_gate.go
  - internal/eval/runner/zdr_gate_test.go
  - internal/eval/report/eval_result.go
  - internal/eval/report/eval_result_test.go
  - internal/eval/report/eval_report.go
  - internal/eval/report/eval_report_test.go
  - internal/eval/report/cost_summary.go
  - internal/eval/report/cost_summary_test.go
  - internal/eval/report/safety_compliance.go
  - internal/eval/report/safety_compliance_test.go
  - internal/eval/report/run_metadata.go
  - internal/eval/report/run_metadata_test.go
  - internal/eval/pipeline.go
  - cmd/helix-eval/main.go
tags: [eval, runner, reports, zdr]

must_haves:
  truths:
    - "Runner orchestrates the per-(task,mode) DAG by filling phasegraph EvalPhases Run bodies (DAG-02 contract)"
    - "Runner refuses to start on non-synthetic, non-OSS-Helix corpora unless HELIX_EVAL_ZDR_VERIFIED=1 (EVAL-06)"
    - "EvalResult per-(task,mode) captures success, patch_applies, tests_pass, diagnostics_clean, duration_ms, tokens, edit_count, guardrail_compliance (EVAL-01)"
    - "5 reports emitted to eval/reports/<run-id>/: eval_report.json, eval_report.md, cost_summary.json, tool_behavior.json, safety_compliance.json (EVAL-04)"
    - "run_metadata.json captures claude --version output (D-03)"
    - "eval_report.md includes mode-comparison table + tool-call distribution + guardrail compliance + 3 failure examples"
    - "patch.diff per (task, mode) is written via git diff under sandbox repo"
  artifacts:
    - path: "internal/eval/runner/runner.go"
      provides: "Per-(task,mode) DAG runner; wires phasegraph EvalPhases Run bodies"
      exports: ["Runner", "NewRunner", "(*Runner).RunTask", "(*Runner).RunMatrix", "EvalResult"]
    - path: "internal/eval/runner/zdr_gate.go"
      provides: "EVAL-06 corpus-source enforcement"
      exports: ["AssertCorpusAllowed"]
    - path: "internal/eval/report/eval_report.go"
      provides: "Aggregate JSON + Markdown reporter"
      exports: ["WriteEvalReport"]
    - path: "internal/eval/report/cost_summary.go"
      provides: "Tokens-only cost summary (D-07)"
      exports: ["WriteCostSummary"]
    - path: "internal/eval/report/safety_compliance.go"
      provides: "Guardrail counts from Phase 66 telemetry"
      exports: ["WriteSafetyCompliance"]
    - path: "internal/eval/report/run_metadata.go"
      provides: "claude --version + helix version + run env"
      exports: ["CaptureRunMetadata", "WriteRunMetadata"]
  key_links:
    - from: "internal/eval/pipeline.go"
      to: "internal/phasegraph/pipelines/eval.go"
      via: "Real Run bodies replace noopRun"
      pattern: "PhaseSpec\\{ID: PhasePrepareWorkspace.*Run:"
    - from: "internal/eval/runner/runner.go"
      to: "internal/eval/sandbox/sandbox.go"
      via: "per-(task,mode) tmpdir lifecycle"
      pattern: "sandbox\\.NewSandbox|StartDaemon"
---

<objective>
Wave 3: tie sandbox + agent + trace + scorer together via the phasegraph EvalPhases DAG (DAG-02 contract — fill noopRun bodies). Add the ZDR corpus-source gate (EVAL-06). Emit all 5 reports + per-task patch.diff + result.json (EVAL-01, EVAL-04).

Purpose: This is where the harness becomes runnable end-to-end. Without this plan there's no way to invoke the matrix or generate evidence.

Output: Runner + ZDR gate + 5 reporter writers + filled-in EvalPhases Run bodies. `helix-eval run --corpus eval/corpus --mode baseline ...` works (subject to Plan 02's daemon spawn working — Plan 02 ships first).
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/phases/67-evaluation-harness/67-RESEARCH.md
@internal/phasegraph/pipelines/eval.go
@internal/phasegraph/

<interfaces>
<!-- phasegraph contract from internal/phasegraph/pipelines/eval.go -->
EvalPhases is a []PhaseSpec with 10 phases: prepare_workspace → configure_mode → run_agent → collect_trace → apply_patch_check → run_tests → run_diagnostics → score_tool_behavior → score_guardrails → aggregate_report.

PhaseSpec.Run signature is (per `internal/phasegraph/`): `func(ctx context.Context, state State) error` (verify exact signature in plan; this plan does NOT modify the phasegraph contract — only fills bodies).

<!-- EVAL-01 EvalResult fields per CONTEXT.md / REQUIREMENTS.md -->
- success bool
- patch_applies bool
- tests_pass bool
- diagnostics_clean bool
- duration_ms int64
- tokens {input, output} int
- edit_count int
- guardrail_compliance {warned, blocked, receipts_issued} ints
- context_precision float (0..1)
- context_recall float (0..1)

context precision/recall are placeholders until later phases add ground-truth labels; emit them as `null` for v1 with a TODO(post-phase-67) comment. EVAL-01 does NOT mandate a non-null value.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: ZDR corpus gate + EvalResult schema + per-task result.json writer</name>
  <files>
    internal/eval/runner/zdr_gate.go,
    internal/eval/runner/zdr_gate_test.go,
    internal/eval/report/eval_result.go,
    internal/eval/report/eval_result_test.go
  </files>
  <behavior>
    - Test 1 (RED): TestZDRGate_SyntheticOK — corpus with all task IDs flagged synthetic (defaults to synthetic for tasks under `eval/corpus/` shipped in this repo) passes without env-var.
    - Test 2 (RED): TestZDRGate_HelixOSSAllowed — when corpus path resolves to the helix repo root or a subdir, allow without env-var (per EVAL-06 OSS Helix permitted secondary).
    - Test 3 (RED): TestZDRGate_NonSyntheticBlocked — corpus with a task explicitly marked `source: external` (added via `source.yaml` per-task) is REJECTED unless `HELIX_EVAL_ZDR_VERIFIED=1`.
    - Test 4 (RED): TestZDRGate_NonSyntheticAllowedWithEnvVar — same task, env-var set → allowed; emits a WARN log line.
    - Test 5 (RED): TestEvalResultJSONShape — marshaling EvalResult emits exactly the EVAL-01 fields with the right names and types. context_precision/context_recall are `*float64` (nullable) and serialize as `null` when not set.
    - Test 6 (RED): TestWriteResultJSON — `WriteResult(path, r)` writes 0600 file under `eval/reports/<run-id>/tasks/<task>/<mode>/result.json`.
  </behavior>
  <action>
    `internal/eval/runner/zdr_gate.go`:
    ```go
    type CorpusSource string
    const (
        CorpusSynthetic CorpusSource = "synthetic"
        CorpusHelixOSS CorpusSource = "helix-oss"
        CorpusExternal CorpusSource = "external"
    )
    type CorpusManifest struct{ Source CorpusSource `yaml:"source"` }
    func AssertCorpusAllowed(corpusDir string) error
    ```
    Algorithm:
    1. Check whether `corpusDir` resolves under the helix repo root (read `go list -m -f '{{.Dir}}'` once or use `filepath.Abs` against a known marker file `.helix-repo-marker` if present). If under repo → allow + log WARN "running against helix-OSS corpus".
    2. For each task dir, look for optional `source.yaml`. Default if missing: `synthetic`.
    3. If any task has `source: external` AND `os.Getenv("HELIX_EVAL_ZDR_VERIFIED") != "1"`, return a wrapped error listing the offending task IDs.
    4. If `HELIX_EVAL_ZDR_VERIFIED=1` is set without external sources, log INFO acknowledging the env-var was set.

    `internal/eval/report/eval_result.go`:
    ```go
    type EvalResult struct {
        TaskID string `json:"task_id"`
        Mode string `json:"mode"`
        Success bool `json:"success"`
        PatchApplies bool `json:"patch_applies"`
        TestsPass bool `json:"tests_pass"`
        DiagnosticsClean bool `json:"diagnostics_clean"`
        DurationMs int64 `json:"duration_ms"`
        Tokens struct{ Input, Output int } `json:"tokens"`
        EditCount int `json:"edit_count"`
        GuardrailCompliance trace.GuardrailCounts `json:"guardrail_compliance"`
        ContextPrecision *float64 `json:"context_precision"` // null until ground-truth labels exist
        ContextRecall *float64 `json:"context_recall"`
        Outcome string `json:"outcome"`
        FailureReason string `json:"failure_reason,omitempty"`
    }
    func WriteResult(path string, r EvalResult) error
    ```
  </action>
  <verify>
    <automated>go test ./internal/eval/runner -run "TestZDRGate" -count=1 -race && go test ./internal/eval/report -run "TestEvalResult|TestWriteResult" -count=1 -race</automated>
  </verify>
  <done>
    Six tests green. ZDR gate enforces EVAL-06 with three modes (synthetic/OSS/external). EvalResult JSON shape matches EVAL-01.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Runner — fill phasegraph EvalPhases Run bodies; orchestrate per-(task, mode); write patch.diff + verify.log</name>
  <files>
    internal/eval/runner/runner.go,
    internal/eval/runner/runner_test.go,
    internal/eval/pipeline.go
  </files>
  <behavior>
    - Test 1 (RED): TestRunnerSinglePathHappy — given a fake claude binary that emits a deterministic stream-json payload (one tool_use → one tool_result → result), runner runs all 10 phases and produces a result.json with success=true, patch_applies=true (verified by an asserting fake `verify.sh`).
    - Test 2 (RED): TestRunnerVerifyFailMarksFailed — fake `verify.sh` exits 1 → result.json has success=false, tests_pass=false.
    - Test 3 (RED): TestRunnerBudgetBreachSurfaces — fake claude sleeps past budget.MaxSeconds → outcome `failed-with-cause: budget_seconds`, result.json reflects it.
    - Test 4 (RED): TestRunnerWritesPatchDiff — runner runs `git diff` on the sandbox repo (after init+add+commit baseline) and writes `patch.diff`; file is non-empty when agent modifies a file.
    - Test 5 (RED): TestRunnerWritesVerifyLog — verify.log captures stdout+stderr of verify.sh and exit code on the last line.
    - Test 6 (RED): TestRunnerMatrix — `RunMatrix(corpus, modes=[baseline, native])` runs every (task, mode) pair and emits 2N result.json files. Concurrency limit configurable; defaults to 1 to match D-05 wall-time targets.
  </behavior>
  <action>
    Update `internal/eval/pipeline.go` to replace the re-export with real PhaseSpecs that wrap `pipelines.EvalPhases` IDs and Provides but with real Run functions. Each Run function reads/writes from a typed `*State` struct shared across phases:
    ```go
    type State struct {
        Sandbox *sandbox.Sandbox
        Task corpus.Task
        Mode string
        DaemonHandle *sandbox.DaemonHandle
        AgentResult *agent.Result
        DaemonTap trace.DaemonTapResult
        CCTap trace.CCTapResult
        Merged trace.MergedTrace
        VerifyExit int
        Patch []byte
        Diagnostics interface{} // post-run get_diagnostics; omit if not implemented this phase
        ToolBehaviorScore score.Score
        Result report.EvalResult
        OutDir string
    }
    ```

    Phase Run bodies:
    1. `prepare_workspace` — `state.Sandbox.Prepare(taskID, mode)`; `state.Sandbox.CloneRepo(corpus task repo, taskID, mode)`; `git -C repo init && git add -A && git commit -m baseline`.
    2. `configure_mode` — write `helix_config.yml` per RESEARCH §"Mode Differences" matrix:
       - baseline: profile=baseline, semantic_index.enabled=false, guardrails.enforcement=off
       - native: profile=full, semantic_index.enabled=false, guardrails.enforcement=off
       - semantic: profile=full, semantic_index.enabled=true, guardrails.enforcement=off
       - semantic_guarded: profile=full, semantic_index.enabled=true, guardrails.enforcement=warn
    3. `run_agent` — `state.Sandbox.StartDaemon(...)`; `agent.NewAgent(sandbox, helixBin).Run(ctx, task, mode)`; record AgentResult.
    4. `collect_trace` — `trace.TapDaemonLog(daemonLogPath, daemonPid)`; `trace.TapCCStream(ccStdoutPath)`; `trace.Merge(...)`; write `trace_daemon.jsonl`, `trace_cc.json`, `trace.json`.
    5. `apply_patch_check` — `git -C repo diff --binary HEAD > patch.diff`; check `git apply --check` against a clean clone (or skip if patch is empty); set `state.Result.PatchApplies` accordingly.
    6. `run_tests` — `bash verify.sh` with cwd=repo; capture stdout+stderr to `verify.log`; record exit code; set `state.Result.TestsPass = (exit == 0)`.
    7. `run_diagnostics` — for v1, set `state.Result.DiagnosticsClean = true` if verify passed; defer real LSP-diagnostics integration to a later phase. Document the TODO inline.
    8. `score_tool_behavior` — `score.Apply(state.Merged, rules)` using rules from `corpus/<task>/expected_tools.yaml`; record state.ToolBehaviorScore. (Aggregate write to `tool_behavior.json` happens in aggregate_report.)
    9. `score_guardrails` — populate state.Result.GuardrailCompliance from state.Merged.Guardrails.
    10. `aggregate_report` — write per-(task, mode) `result.json`. (The cross-(task, mode) aggregation happens after the matrix is done — see Task 3.)

    `Runner.RunTask(ctx, task, mode)` builds a fresh State, runs `phasegraph.RunPhaseGraph(EvalPhases)` against it, returns the State.
    `Runner.RunMatrix(ctx, corpusDir, modes []string)` walks corpus, calls RunTask for every (task, mode), aggregates list of EvalResult.

    Concurrency knob `Runner.MaxParallel int` (default 1). Set to 1 for D-05 wall-time predictability; bump to GOMAXPROCS later if needed.
  </action>
  <verify>
    <automated>go test ./internal/eval/runner -count=1 -race && go vet ./internal/eval/runner ./internal/eval/pipeline</automated>
  </verify>
  <done>
    Six runner tests green; race-clean. EvalPhases Run bodies are no longer noopRun (DAG-02 contract honored). Per-(task, mode) artifacts written: trace_daemon.jsonl, trace_cc.json, trace.json, patch.diff, verify.log, result.json.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Aggregate reporters — eval_report.{json,md} + cost_summary.json + tool_behavior.json + safety_compliance.json + run_metadata.json</name>
  <files>
    internal/eval/report/eval_report.go,
    internal/eval/report/eval_report_test.go,
    internal/eval/report/cost_summary.go,
    internal/eval/report/cost_summary_test.go,
    internal/eval/report/safety_compliance.go,
    internal/eval/report/safety_compliance_test.go,
    internal/eval/report/run_metadata.go,
    internal/eval/report/run_metadata_test.go,
    cmd/helix-eval/main.go
  </files>
  <behavior>
    - Test 1 (RED): TestRunMetadataCapturesClaudeVersion — `CaptureRunMetadata()` invokes `claude --version` (skip with t.Skip if not on PATH); records the version string verbatim. Also records helix version (`runtime/debug.ReadBuildInfo`), GOOS/GOARCH, env keys present (not values), modes invoked, run-id.
    - Test 2 (RED): TestWriteRunMetadata — produces `run_metadata.json` 0600 with all required fields.
    - Test 3 (RED): TestWriteCostSummary — given list of EvalResults, writes per-mode and overall input/output token totals. NO dollar conversion (D-07).
    - Test 4 (RED): TestWriteSafetyCompliance — given list of EvalResults, writes counts of guardrail_warned, guardrail_blocked, receipts_issued per mode.
    - Test 5 (RED): TestWriteToolBehavior — given map[(task,mode)]score.Score, writes per-task per-mode delta breakdown. (Note: WriteToolBehavior lives in score package OR here — pick here for cohesion.)
    - Test 6 (RED): TestWriteEvalReportJSON — given matrix of EvalResults, writes `eval_report.json` with: run-id, modes, claude_version, per-mode aggregates {success_rate, mean_tokens, mean_duration, tool_call_distribution, guardrail_counts}, per-task per-mode results.
    - Test 7 (RED): TestWriteEvalReportMarkdown — writes `eval_report.md` with:
      - Header (run-id, claude --version, helix version, date)
      - Mode-comparison table (success/cost/latency rows)
      - Tool-call distribution per mode (top 10 tools)
      - Guardrail compliance section
      - 3 failure examples (selected by sorting failures by mode then task; quote the FailureReason from MergedTrace).
      - "Informational: LLM judge" placeholder section (filled by Plan 06).
    - Test 8 (RED): TestEvalReportRendersWithoutLLMJudge — when `tool_behavior_judge.json` is absent, the markdown report still renders without error and notes "(judge not run)".
    - Test 9 (RED): TestRunSubcommandWiresThemAll — `helix-eval run --corpus eval/corpus --mode baseline --out <tmp>` produces all 6 files (eval_report.json, eval_report.md, cost_summary.json, tool_behavior.json, safety_compliance.json, run_metadata.json) under `<tmp>/<run-id>/`.
  </behavior>
  <action>
    `run_metadata.go`:
    ```go
    type RunMetadata struct {
        RunID string
        StartedAt, EndedAt time.Time
        ClaudeVersion string // verbatim output of `claude --version`; "" if not on PATH
        HelixVersion string
        GOOS, GOARCH string
        Modes []string
        EnvKeys []string // KEYS only; never values (T-67-Pitfall-1 mitigation)
        CorpusDir string
    }
    func CaptureRunMetadata(modes []string, corpusDir string) RunMetadata
    func WriteRunMetadata(path string, m RunMetadata) error
    ```
    `cost_summary.go`:
    ```go
    type CostSummary struct {
        SchemaVersion string
        ByMode map[string]ModeCostAgg
        Overall struct{ InputTokens, OutputTokens int }
    }
    type ModeCostAgg struct{ TaskCount int; InputTokens, OutputTokens int; MeanInputTokens, MeanOutputTokens float64 }
    func WriteCostSummary(path string, results []EvalResult) error
    ```
    `safety_compliance.go`:
    ```go
    type SafetyCompliance struct {
        SchemaVersion string
        ByMode map[string]ModeSafetyAgg
    }
    type ModeSafetyAgg struct{ Warned, Blocked, ReceiptsIssued int }
    func WriteSafetyCompliance(path string, results []EvalResult) error
    ```
    `eval_report.go`:
    - `WriteEvalReport(jsonPath, mdPath string, results []EvalResult, scores map[string]score.Score, meta RunMetadata) error`.
    - JSON shape: `{schema_version: "1", metadata: ..., by_mode: {...}, by_task: {...}, results: [...]}`.
    - Markdown sections per Test 7 above. Use `text/template` with a checked-in template literal.
    - Include the canonical "INFORMATIONAL: LLM judge — DO NOT GATE CI" boilerplate when `tool_behavior_judge.json` is included; hide the section when absent.

    `cmd/helix-eval/main.go` `run` subcommand body:
    1. Parse flags (corpus, modes, run-id, out, --quick, --judge-model, --no-judge).
    2. If `--quick`: route to in-process path (Plan 06; this plan only stubs the routing).
    3. Else: `runner.AssertCorpusAllowed(corpusDir)` → `runner.NewRunner(...)` → `runner.RunMatrix(...)` → write all reports.
    4. Print summary (counts per outcome) to stdout.
    5. Exit code: 0 if all results have success=true; 1 otherwise. (Note: judge failure does NOT affect exit code per EVAL-07.)
  </action>
  <verify>
    <automated>go test ./internal/eval/report ./cmd/helix-eval -count=1 -race && go vet ./internal/eval/report ./cmd/helix-eval</automated>
  </verify>
  <done>
    Nine reporter tests green. All 5 EVAL-04 reports + run_metadata.json emitted under `eval/reports/<run-id>/`. `helix-eval run` wires it end-to-end. Judge-absence does not break the markdown report.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Corpus directory → ZDR gate | Untrusted source designation; gate enforces synthetic-default + env-var attestation for external. |
| run_metadata env capture | Trusted shape but values must NEVER be recorded (only keys). |
| Reports → reviewer | Reports go on the local filesystem; the LLM judge boundary is enforced by Plan 06's separate JSON file. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-67-06 | I (Info Disclosure) | non-synthetic corpus + commercial-LLM call | mitigate | `AssertCorpusAllowed` rejects external sources without `HELIX_EVAL_ZDR_VERIFIED=1`. Tests cover all three corpus-source modes (TestZDRGate_*). EVAL.md operator-checklist documents quarterly re-verification. |
| T-67-Pitfall-1 | I (Info Disclosure) | env-var values in run_metadata | mitigate | run_metadata captures KEYS only, never values. TestRunMetadataCapturesClaudeVersion asserts the EnvKeys field is `[]string` of names. |
| T-67-Pitfall-8 | (judge gates merges) | aggregate report wiring | mitigate | Markdown report has separate "Informational: LLM judge" section with the explicit boilerplate; `helix-eval run` exit code does NOT consider judge output (Plan 06 will assert). EVAL-07 verbatim. |
| T-67-03 | T (Tampering) | mode-name → config path mapping | mitigate | `configure_mode` Run body uses a hard-coded switch over the four mode names; unknown mode returns error before any file write. |

Block on: HIGH severity. T-67-06 is HIGH; mitigated. T-67-03 is HIGH; mitigated by hard-coded enum. Pitfalls 1 and 8 are MEDIUM and mitigated.
</threat_model>

<verification>
- 5 reports + run_metadata.json emitted on a fixture run.
- Mode-comparison table renders correctly even with 0 LLM judge data.
- ZDR gate exit-codes correctly across all three corpus-source cases.
- DAG-02 contract honored: pipelines.EvalPhases noopRun bodies are now real implementations.
</verification>

<success_criteria>
- [ ] `internal/eval/runner/zdr_gate.go` — 4 ZDR tests green.
- [ ] `internal/eval/report/eval_result.go` — 2 schema tests green.
- [ ] `internal/eval/runner/runner.go` — 6 runner tests green; phasegraph integration verified.
- [ ] `internal/eval/report/{eval_report,cost_summary,safety_compliance,run_metadata}.go` — 9 reporter tests green.
- [ ] `helix-eval run --corpus eval/corpus --mode baseline --out <tmp>` produces all expected files.
- [ ] `go vet ./...` and `go test ./internal/eval/...` clean.
</success_criteria>

<dependencies>
- Plans: 67-01 (skeleton + Makefile), 67-02 (sandbox + agent + budget), 67-03 (trace), 67-04 (scorer + corpus).
- External: helix daemon binary; `claude` CLI on PATH for full RunMatrix tests (tests using fake claude in-process where possible).
</dependencies>

<output>
After completion, create `.planning/phases/67-evaluation-harness/67-05-SUMMARY.md` recording: final EvalResult schema, ZDR-gate corpus-source enum (synthetic/helix-oss/external), Run-body implementation patterns chosen for each of the 10 phasegraph phases.
</output>
