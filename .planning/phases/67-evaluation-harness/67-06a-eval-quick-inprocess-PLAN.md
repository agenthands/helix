---
phase: 67
plan: 06a
type: tdd
wave: 4
depends_on: [67-01, 67-03, 67-04, 67-05]
autonomous: true
requirements: [EVAL-03]
files_modified:
  - internal/eval/runner/inprocess.go
  - internal/eval/runner/inprocess_test.go
  - internal/eval/runner/scripted_agent.go
  - internal/eval/runner/scripted_agent_test.go
  - eval/fixtures/quick-rename-001/task.md
  - eval/fixtures/quick-rename-001/expected_tools.yaml
  - eval/fixtures/quick-rename-001/budget.yaml
  - eval/fixtures/quick-rename-001/scripted_agent.yaml
  - eval/fixtures/quick-rename-001/verify.sh
  - eval/fixtures/quick-rename-001/repo/main.go
  - eval/fixtures/quick-rename-001/repo/go.mod
  - eval/fixtures/quick-delete-001/task.md
  - eval/fixtures/quick-delete-001/expected_tools.yaml
  - eval/fixtures/quick-delete-001/budget.yaml
  - eval/fixtures/quick-delete-001/scripted_agent.yaml
  - eval/fixtures/quick-delete-001/verify.sh
  - eval/fixtures/quick-delete-001/repo/main.go
  - eval/fixtures/quick-delete-001/repo/go.mod
  - cmd/helix-eval/main.go
  - Makefile
tags: [eval, in-process, eval-quick, plumbing]

must_haves:
  truths:
    - "make eval-quick runs entirely in-process: no claude subprocess, no daemon subprocess"
    - "eval-quick uses an in-process daemon via daemon.New(cfg, logger).Run with profile overlays in-memory"
    - "eval-quick uses a scripted agent (hard-coded MCP tool-call sequence) instead of claude CLI"
    - "eval-quick wall time < 30s for the COMBINED 10-fixture run (this plan ships 2 reference fixtures; Plan 06b adds 8 more under the same budget)"
    - "eval-quick output explicitly states 'harness validation only — agent behavior NOT measured' (Pitfall 6 mitigation)"
    - "eval-quick refuses to mark EvalResult.success=true unless invoked with --quick flag"
  artifacts:
    - path: "internal/eval/runner/inprocess.go"
      provides: "In-process daemon + scripted-agent path"
      exports: ["RunQuick"]
    - path: "internal/eval/runner/scripted_agent.go"
      provides: "Hard-coded MCP tool-call sequence executor"
      exports: ["ScriptedAgent", "LoadScript"]
    - path: "eval/fixtures/quick-rename-001 + quick-delete-001"
      provides: "2 reference fixtures used by RunQuick + scripted-agent unit tests; Plan 06b expands to 10 total"
  key_links:
    - from: "internal/eval/runner/inprocess.go"
      to: "internal/daemon/daemon.go"
      via: "daemon.New(cfg, logger).Run(ctx) called directly (no subprocess)"
      pattern: "daemon\\.New"
    - from: "internal/eval/runner/scripted_agent.go"
      to: "in-process MCP client"
      via: "MCP tools/call dispatched directly to the in-process server"
      pattern: "ToolRegistry|tools/call|CallTool"
---

<objective>
Wave 4 (part 1 of 2): in-process `eval-quick` PLUMBING. EVAL-03 mandates a fast variant that skips both subprocesses (no claude, no daemon) for fast local CI feedback. Uses a SCRIPTED agent (hard-coded MCP call sequence per fixture) — NOT real-agent behavior. Heavily flagged in EVAL.md (Pitfall 6 mitigation).

Purpose: Engineers need <30s feedback that the harness wiring is intact before paying for real eval runs. eval-quick proves: (a) profile filter works, (b) tool registry is reachable, (c) trace tap captures events, (d) scorer applies rules. It does NOT prove agent behavior.

This plan delivers the **plumbing** (RunQuick, scripted-agent dispatch, bufconn daemon boot, harness-validation banner, success-flag gate, <30s wall budget) plus **2 reference fixtures** (rename + delete) used by the unit/integration tests in this plan. Plan **67-06b** then expands to the full D-05 ~10-task fixture set covering all 5 EVAL-05 families × Go/TS/Python.

D-05 (LOCKED): "make eval-quick: ~10 in-process tasks, target <30s wall-time, in CI on every PR." This plan stays under the <30s budget with 2 fixtures; 06b must preserve the budget when scaling to 10.

Output: `RunQuick` path; scripted agent; 2 reference fixtures under `eval/fixtures/`; Makefile target wired; explicit Pitfall-6 guard.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/phases/67-evaluation-harness/67-CONTEXT.md
@.planning/phases/67-evaluation-harness/67-RESEARCH.md
@internal/daemon/daemon.go
@cmd/helix/main.go

<interfaces>
The daemon exposes `daemon.New(cfg, logger)` returning a Daemon with `Run(ctx)` that blocks until ctx cancellation. The MCP server inside is `*mcp.SerenaMCPServer`; tools are registered atomically before Run begins.

For in-process use, two seam options:
1. Run the daemon as-is on a `bufconn.Listen()` Unix-socket replacement (gRPC over bufconn); scripted agent dials via the in-memory listener.
2. Reach into the daemon's ToolRegistry directly and invoke tool handlers through a synthetic `mcp.Session`-like wrapper.

RECOMMENDED (per RESEARCH §"In-Process eval-quick"): bufconn for realism. The scripted-agent issues real MCP `tools/call` requests so the TelemetryMiddleware emits the same trace lines as a real run. Verify the cleanest seam during implementation; do NOT modify the daemon's exported surface in this plan.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Scripted-agent + 2 reference fixtures (rename, delete)</name>
  <files>
    internal/eval/runner/scripted_agent.go,
    internal/eval/runner/scripted_agent_test.go,
    eval/fixtures/quick-rename-001/task.md,
    eval/fixtures/quick-rename-001/expected_tools.yaml,
    eval/fixtures/quick-rename-001/budget.yaml,
    eval/fixtures/quick-rename-001/scripted_agent.yaml,
    eval/fixtures/quick-rename-001/verify.sh,
    eval/fixtures/quick-rename-001/repo/main.go,
    eval/fixtures/quick-rename-001/repo/go.mod,
    eval/fixtures/quick-delete-001/task.md,
    eval/fixtures/quick-delete-001/expected_tools.yaml,
    eval/fixtures/quick-delete-001/budget.yaml,
    eval/fixtures/quick-delete-001/scripted_agent.yaml,
    eval/fixtures/quick-delete-001/verify.sh,
    eval/fixtures/quick-delete-001/repo/main.go,
    eval/fixtures/quick-delete-001/repo/go.mod
  </files>
  <behavior>
    - Test 1 (RED): TestLoadScript — `scripted_agent.yaml` with steps `[{tool:find_references,args:{...}},{tool:rename_symbol,args:{...}}]` parses with KnownFields(true).
    - Test 2 (RED): TestScriptedAgentDispatch — given a fake MCPCaller (in-test) returning canned responses, ScriptedAgent.Run executes each step in order and returns a list of step results.
    - Test 3 (RED): TestScriptedAgentRefusesUnknownStepKey — yaml with `tools` instead of `tool` key fails LoadScript.
    - Test 4 (RED): TestScriptedAgentRespectsBudgetMaxToolCalls — script with 6 steps + budget.MaxToolCalls=3 stops after step 3 and reports BreachReason{Axis:"tool_calls"}.
    - Test 5 (fixtures): TestQuickRenameFixtureLoadable — `eval/fixtures/quick-rename-001` loads via score.LoadRules + LoadScript without error.
    - Test 6 (fixtures): TestQuickDeleteFixtureLoadable — same for quick-delete-001.
  </behavior>
  <action>
    `internal/eval/runner/scripted_agent.go`:
    - `type ScriptedStep struct { Tool string; Args map[string]any; ExpectError bool }`
    - `type Script struct { Steps []ScriptedStep }`
    - `func LoadScript(path string) (Script, error)` with `yaml.Decoder.KnownFields(true)`.
    - `type MCPCaller interface { CallTool(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) }`
    - `type ScriptedAgent struct { caller MCPCaller }`
    - `(a *ScriptedAgent) Run(ctx, s, wd *budget.Watchdog) ([]StepResult, *budget.BreachReason, error)` — dispatch each step; record AtTime; on each call, `wd.RecordToolCall()`; on breach, abort and return BreachReason.

    Each StepResult is converted by the runner into synthetic CC-side events (Source="cc", KindAssistantMsg + KindToolResult) so the merger sees a CC stream. The DAEMON-side events come from the real in-process TelemetryMiddleware — that's the actual signal under test.

    Fixture `eval/fixtures/quick-rename-001/`:
    - task.md: "Rename Foo to Bar in main.go"
    - repo/main.go: `package main\n\ntype Foo struct{}\nfunc (f *Foo) Hello() {}\nfunc main() { _ = (&Foo{}).Hello }`
    - repo/go.mod: `module example.com/quick-rename-001` + `go 1.22`
    - scripted_agent.yaml: `steps: [{tool: find_references, args: {symbol: Foo}}, {tool: rename_symbol, args: {old_name: Foo, new_name: Bar}}]`
    - expected_tools.yaml: `expect_sequence: [{id: rename-after-references, score: 1, pattern: [{tool: find_references}, {tool: rename_symbol}]}]`
    - budget.yaml: `max_seconds: 10\nmax_tool_calls: 10\nmax_input_tokens: 200000\nmax_output_tokens: 32000`
    - verify.sh: `set -e; cd repo; grep -q "type Bar struct" main.go; ! grep -q "type Foo struct" main.go; go vet ./...`

    Fixture `eval/fixtures/quick-delete-001/`: analogous with safe_delete_symbol and a receipts rule.

    Both fixtures use Go (first-class LSP tier; fastest warm-up — keeps under 30s budget). Plan 06b adds 8 more fixtures spanning the remaining EVAL-05 families and TS/Python coverage.
  </action>
  <verify>
    <automated>go test ./internal/eval/runner -run "TestLoadScript|TestScriptedAgent|TestQuickRenameFixtureLoadable|TestQuickDeleteFixtureLoadable" -count=1 -race && cd eval/fixtures/quick-rename-001/repo && go vet ./... && cd - >/dev/null && cd eval/fixtures/quick-delete-001/repo && go vet ./...</automated>
  </verify>
  <done>
    Six tests green; both fixtures vet-clean; verify.sh files are chmod +x.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: In-process RunQuick — daemon over bufconn + Pitfall-6 success-flag guard + <30s budget headroom for 10-fixture set</name>
  <files>
    internal/eval/runner/inprocess.go,
    internal/eval/runner/inprocess_test.go,
    cmd/helix-eval/main.go,
    Makefile
  </files>
  <behavior>
    - Test 1 (RED): TestRunQuickStartsInProcessDaemon — `RunQuick(ctx, cfg)` boots `daemon.New(...).Run(ctx)` on a bufconn listener; daemon shuts down on ctx-cancel.
    - Test 2 (RED): TestRunQuickRunsAllFourModes — runs the 2 reference fixtures across the 4 modes (baseline, native, semantic, semantic_guarded) by passing in-memory profile/config overlays per mode (no filesystem writes for HOME).
    - Test 3 (RED): TestRunQuickWallTimeUnder30Seconds — assert duration <30s on CI for the 2-reference-fixture set with comfortable headroom (target <10s) so Plan 06b can scale to 10 fixtures within the same 30s D-05 budget. Use `testing.Short()` skip-shortcut on under-resourced runners.
    - Test 4 (RED): TestRunQuickEmitsHarnessValidationBanner — `helix-eval run --quick` writes a banner line "eval-quick: HARNESS VALIDATION ONLY — agent behavior NOT measured" to stdout AND prepends the same line to `eval_report.md`.
    - Test 5 (RED): TestRunQuickSuccessFlagGuard — without `--quick`, scripted-agent execution path is unreachable (sentinel error). With `--quick`, EvalResult.success can be true. (Pitfall 6 mitigation per CONTEXT.md.)
    - Test 6 (RED): TestRunQuickEmitsFiveReports — same 5 EVAL-04 reports as the full path, written under `eval/reports/<run-id>/`.
    - Test 7 (RED): TestRunQuickReusesDaemonAcrossFixtures — assert the in-process daemon is booted ONCE per RunQuick call and reused across all (fixture × mode) pairs (per-mode reuse with config overlay swap, NOT per-fixture re-boot). This is the lever that keeps 10 fixtures × 4 modes under 30s.
  </behavior>
  <action>
    `internal/eval/runner/inprocess.go`:
    - `func RunQuick(ctx context.Context, opts QuickOpts) (Summary, error)`.
    - QuickOpts: `FixturesDir string; Modes []string; OutDir string; RunID string`.
    - Boot a daemon with a bufconn listener; build an in-memory MCP client that talks to the bufconn endpoint (mirror existing forwarder dial code with the listener swapped). Confirm the cleanest seam by reading `internal/daemon/daemon.go` once; do NOT modify the daemon surface.
    - **Daemon lifecycle (D-05 budget headroom):** Boot the daemon ONCE per mode at the start of the matrix; reuse it across all fixtures for that mode. Switching modes requires a profile overlay swap (Plan 05 mode-difference matrix); if profile swap is not hot-reloadable in-memory, accept one daemon-reboot per mode (4 boots total) but NEVER per fixture. This is what allows 10 fixtures × 4 modes to fit in <30s.
    - For each (fixture, mode):
      1. In-memory profile/config overlay per mode (per Plan 05 mode-difference matrix).
      2. `sandbox.NewSandbox` for tmpdir/repo isolation (sandbox is reused; no daemon-subprocess spawn — RunQuick uses the in-process daemon only).
      3. `LoadScript` + `ScriptedAgent.Run`.
      4. Tap daemon trace via the in-process TelemetryMiddleware (subscribe to its event sink — confirm seam during impl; if no subscribe API exists, capture daemon log output via an in-memory writer).
      5. Trace.Merge → score → result.json.
    - Set `result.IsQuick = true` so reporters mark it.

    Pitfall-6 success-flag guard: `EvalResult.success` setter is gated. In runner.RunQuick, the gate is opened. In runner.RunMatrix (subprocess full path from Plan 05), the gate is also opened. Anywhere else (e.g., a forgotten test setup), the default is false. Implement as a struct field `quickPathAllowed bool` set explicitly by the two entrypoints.

    `cmd/helix-eval/main.go` `run` subcommand:
    - When `--quick` flag is set, call `RunQuick` instead of `runner.RunMatrix`.
    - Always emit the harness-validation banner to stdout.
    - Exit 0 if all results pass per the rules; non-zero otherwise.

    `Makefile`: confirm `eval-quick` target invokes `go run ./cmd/helix-eval run --quick --corpus eval/fixtures --out eval/reports`. Add a comment block at the top of the target documenting Pitfall-6 explicitly AND the D-05 <30s budget for the full 10-fixture set (Plan 06b).
  </action>
  <verify>
    <automated>go test ./internal/eval/runner -run "TestRunQuick" -count=1 -race && go vet ./internal/eval/runner ./cmd/helix-eval && time make eval-quick</automated>
  </verify>
  <done>
    Seven tests green; `make eval-quick` runs end-to-end under 30s with comfortable headroom on the 2-fixture set; harness-validation banner emitted; success-flag gate enforced; 5 reports emitted under reports/; daemon-per-mode reuse verified.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Scripted agent script → in-process daemon | Trusted (in-repo, PR-reviewed); no LLM in the loop. |
| eval-quick output → CI gate | TRUSTED but explicitly NOT a measurement of agent behavior. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-67-Pitfall-6 | (Misadvertised coverage) | eval-quick claims agent-behavior validation | mitigate | (a) Banner line on stdout + prepended to eval_report.md; (b) `success` flag gated behind `quickPathAllowed bool` set only by RunQuick/RunMatrix; (c) EVAL.md (Plan 01) loud disclaimer; (d) Makefile comment block. Four-layer defense. |
| T-67-04 | S (Spoofing) | in-process daemon trace mixed with another daemon's trace | accept | RunQuick boots a fresh daemon per call into a per-(task,mode) sandbox; no socket file involved (bufconn). PID-gate from Plan 03 still applies if a real daemon log is also tapped, but in-process mode bypasses log files entirely (events are captured via in-memory hooks). |

Block on: HIGH severity. T-67-Pitfall-6 is HIGH (could let real-agent regression slip through); four-layer mitigation. T-67-04 in this context is LOW (no foreign processes share the in-memory event stream).
</threat_model>

<verification>
- `make eval-quick` runs in <30s on CI (with headroom for Plan 06b's 8 additional fixtures).
- Banner line appears in stdout AND in eval_report.md header.
- Quick path produces all 5 EVAL-04 reports.
- Without `--quick`, scripted-agent path is unreachable (sentinel error).
- `go vet ./internal/eval/...` clean.
- Daemon reused across fixtures within a mode (TestRunQuickReusesDaemonAcrossFixtures).
</verification>

<success_criteria>
- [ ] 2 reference fixtures under `eval/fixtures/quick-{rename,delete}-001` with all required files.
- [ ] `internal/eval/runner/scripted_agent.go` + 6 tests green.
- [ ] `internal/eval/runner/inprocess.go` + 7 tests green.
- [ ] `helix-eval run --quick` end-to-end works.
- [ ] `make eval-quick` < 30s wall (with headroom for 10-fixture expansion in 06b).
- [ ] Pitfall-6 mitigation has 4 layers (banner, gate, EVAL.md, Makefile comment).
</success_criteria>

<dependencies>
- Plans: 67-01 (skeleton, EVAL.md), 67-03 (trace), 67-04 (scorer + corpus types), 67-05 (runner + reporters).
- Plan 67-06b depends on this plan (uses RunQuick + ScriptedAgent + LoadScript to add 8 more fixtures).
- External: none (no claude CLI; no Anthropic API).
</dependencies>

<output>
After completion, create `.planning/phases/67-evaluation-harness/67-06a-SUMMARY.md` recording: in-process seam chosen (bufconn vs ToolRegistry direct), wall-clock measurement on CI for 2-fixture set, daemon-reuse strategy, exact text of the harness-validation banner.
</output>
