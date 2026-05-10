---
phase: 67
plan: 02
type: tdd
wave: 1
depends_on: [67-01]
autonomous: true
requirements: [EVAL-02]
files_modified:
  - internal/eval/sandbox/sandbox.go
  - internal/eval/sandbox/sandbox_test.go
  - internal/eval/agent/claude.go
  - internal/eval/agent/claude_test.go
  - internal/eval/agent/mcpconfig.go
  - internal/eval/agent/mcpconfig_test.go
  - internal/eval/budget/budget.go
  - internal/eval/budget/budget_test.go
tags: [eval, sandbox, isolation, subprocess]

must_haves:
  truths:
    - "Each (task, mode) pair gets its own tmpdir with HOME, daemon socket, repo clone, mcp-config.json"
    - "Daemon subprocess starts with --socket=<per-mode-tmpdir>/daemon.sock and HOME=<per-mode-tmpdir>/home"
    - "claude subprocess invocation uses --bare --strict-mcp-config to prevent dev-config leakage"
    - "Environment is scrubbed to a documented allowlist (PATH, HOME, ANTHROPIC_API_KEY, HELIX_LOG_LEVEL)"
    - "Two concurrent modes A and B do not collide on socket or HOME"
    - "D-08 budget caps (max_input_tokens, max_output_tokens, max_seconds, max_tool_calls) are enforceable"
    - "Budget breach surfaces as failed-with-cause: budget_<axis>"
  artifacts:
    - path: "internal/eval/sandbox/sandbox.go"
      provides: "Per-(task,mode) tmpdir, HOME, socket layout + lifecycle"
      exports: ["NewSandbox", "(*Sandbox).ModeDir", "(*Sandbox).RepoFor", "(*Sandbox).SocketFor", "(*Sandbox).HomeFor", "(*Sandbox).McpConfigPath", "(*Sandbox).StartDaemon", "(*Sandbox).Cleanup"]
    - path: "internal/eval/agent/claude.go"
      provides: "claude CLI subprocess wrapper"
      exports: ["NewAgent", "(*Agent).Run"]
    - path: "internal/eval/agent/mcpconfig.go"
      provides: "Per-mode --mcp-config JSON writer"
      exports: ["WriteMCPConfig"]
    - path: "internal/eval/budget/budget.go"
      provides: "D-08 four-axis budget enforcement (watchdog + YAML loader). Consumes the BreachReason type declared in Plan 01 internal/eval/budget/types.go — does NOT redeclare it."
      exports: ["Budget", "Watchdog", "Default", "LoadFromFile"]
  key_links:
    - from: "internal/eval/sandbox/sandbox.go"
      to: "internal/forwarder/dial.go"
      via: "waitForDaemon socket-poll pattern"
      pattern: "waitSocket|waitForDaemon"
    - from: "internal/eval/agent/claude.go"
      to: "internal/eval/sandbox/sandbox.go"
      via: "Sandbox.HomeFor + Sandbox.SocketFor inputs to env + MCP config"
      pattern: "sandbox\\.(Home|Socket|Repo)For"
    - from: "internal/eval/budget/budget.go"
      to: "context.WithTimeout"
      via: "max_seconds enforcement"
      pattern: "context\\.WithTimeout"
---

<objective>
Wave 1 part 1: subprocess sandbox + agent runner + budget watchdog. Each (task, mode) gets a clean filesystem-, socket-, and config-namespace; the agent is `claude --bare --strict-mcp-config` per RESEARCH §"Subprocess Isolation Design"; budgets are enforced four-axis per D-08.

Purpose: EVAL-02 mandates "out-of-process subprocess with isolated config dir". This plan delivers the isolation primitive AND the agent invocation, plus D-08 watchdog. Without this, no real-agent eval is possible.

Output: `internal/eval/sandbox`, `internal/eval/agent`, `internal/eval/budget` with concrete implementations and TDD tests.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/phases/67-evaluation-harness/67-CONTEXT.md
@.planning/phases/67-evaluation-harness/67-RESEARCH.md

@internal/forwarder/dial.go
@internal/config/loader.go
@internal/cli/root.go

<interfaces>
<!-- Patterns to lift directly. Do not invent new helpers. -->

From `internal/forwarder/dial.go` (extracted):
```go
// startDaemon spawns a helix daemon subprocess; waitForDaemon polls the socket.
// Pattern: cmd := exec.CommandContext(ctx, helixBin, "--serve", "--socket", sockPath, ...)
//          cmd.Env = []string{...} // explicit allowlist
//          cmd.Start(); waitForDaemon(ctx, sockPath, 10*time.Second)
```

From `internal/cli/root.go` (extracted):
- `--serve` enters daemon mode; `--socket <path>` overrides UDS path; `--config <path>` overrides config-file lookup; `--profile <name>` selects profile; `--json` enables JSON-formatted structured logs.
- Default behavior with NO subcommand is stdio forwarder mode.

From `internal/config/loader.go` (extracted):
- `DefaultSocketPath()` is `$TMPDIR/helix-<uid>/daemon.sock` — UID-keyed, NOT mode-keyed. Sandbox MUST override.
- `Load(global, project, cliOverrides)` resolves global config via `os.UserHomeDir()`. Overriding HOME redirects to a clean dir. NO new code in `internal/config` is needed.

CC `--mcp-config` JSON shape (RESEARCH §"CC --mcp-config Schema", verified 2026-05-10):
```json
{"mcpServers":{"helix":{"type":"stdio","command":"/abs/path/to/helix","args":["--socket","<sockpath>"],"env":{"HOME":"<homepath>"}}}}
```
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Sandbox primitive — per-(task,mode) tmpdir + daemon spawn + socket-wait + cleanup</name>
  <files>
    internal/eval/sandbox/sandbox.go,
    internal/eval/sandbox/sandbox_test.go
  </files>
  <behavior>
    - Test 1 (RED): TestSandboxLayout — `NewSandbox(runID)` creates `$TMPDIR/helix-eval-<runID>/`; `ModeDir(taskID, mode)` returns `<runID>/<task>/<mode>/`; subdirs `home/.helix/`, `repo/` exist; mode 0700 on dirs.
    - Test 2 (RED): TestSandboxIsolation — two modes for same task have disjoint sockets, disjoint HOME, disjoint repo paths.
    - Test 3 (RED): TestSandboxCloneRepo — `CloneRepo(corpusRepoPath, taskID, mode)` copies `corpus/<task>/repo/` recursively into the per-(task,mode) repo dir.
    - Test 4 (RED): TestSandboxStartDaemon — uses a fake `helixBin` that simply opens a UDS at the requested socket path then sleeps; `StartDaemon` must block until the socket exists; returned handle has `Kill()` that cleanly reaps.
    - Test 5 (RED): TestSandboxCleanup — `Cleanup()` removes the runID tmpdir; tolerant if a daemon process is still running (kills first).
    - Test 6 (RED): TestSandboxConcurrent — two `StartDaemon(ctx, "baseline")` calls in parallel for two different `(task,mode)` keys do NOT collide (different sockets).
    - Test 7 (RED): TestSandboxRefusesSymlinkedTmpdir — if `os.MkdirTemp` follows a symlink to a non-0700 dir, fail.
  </behavior>
  <action>
    Implement `internal/eval/sandbox/sandbox.go` with:
    - `type Sandbox struct { Root string; helixBin string }`
    - `NewSandbox(runID string, helixBin string) (*Sandbox, error)` — `os.MkdirTemp("", "helix-eval-"+runID+"-")` with mode 0700; reject if the resolved path is a symlink (Pitfall: T-67-01 mitigation per threat model).
    - `(s *Sandbox) ModeDir(taskID, mode string) string` — `<root>/<task>/<mode>/`.
    - `(s *Sandbox) HomeFor(taskID, mode string) string` — `<modeDir>/home`.
    - `(s *Sandbox) RepoFor(taskID, mode string) string` — `<modeDir>/repo`.
    - `(s *Sandbox) SocketFor(taskID, mode string) string` — `<modeDir>/daemon.sock`.
    - `(s *Sandbox) McpConfigPath(taskID, mode string) string` — `<modeDir>/mcp-config.json`.
    - `(s *Sandbox) Prepare(taskID, mode string) error` — creates the directory tree (HOME/.helix, repo, etc.) with mode 0700.
    - `(s *Sandbox) CloneRepo(srcRepo, taskID, mode string) error` — recursive copy preserving file modes; do NOT follow symlinks out of srcRepo.
    - `(s *Sandbox) StartDaemon(ctx context.Context, taskID, mode, profileName, cfgPath string) (*DaemonHandle, error)` — `exec.CommandContext` per the dial.go pattern shown in interfaces. `cmd.Env` is the documented allowlist: `PATH`, `HOME` (set to HomeFor), `HELIX_LOG_LEVEL=info`. Stderr goes to `<modeDir>/daemon.log`. Polls socket up to 10s.
    - `(h *DaemonHandle) Kill() error` — `Process.Kill` then `Wait` with 5s deadline; returns wrapped error with mode/task context.
    - `(s *Sandbox) Cleanup() error` — kill any held daemon handles, then `os.RemoveAll(s.Root)`.

    `waitSocket(ctx, sockPath, deadline)` is a private helper — copy the loop pattern from `internal/forwarder/dial.go` (poll `os.Stat` 50ms ticker until socket exists or ctx/deadline expires).

    All tests RED first, then GREEN. No skipped tests. Tests use `t.TempDir()` and a stub helixBin that's a small Go program built into a temp file (or use `os.Args[0]` with `TestHelperProcess` style — pick whichever is idiomatic in this repo; mirror existing forwarder tests).
  </action>
  <verify>
    <automated>go test ./internal/eval/sandbox -run TestSandbox -count=1 -race</automated>
  </verify>
  <done>
    Seven sandbox tests pass; race-clean. `go vet ./internal/eval/sandbox/...` clean.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Agent — claude CLI subprocess wrapper + mcp-config writer + clean-env discipline</name>
  <files>
    internal/eval/agent/claude.go,
    internal/eval/agent/claude_test.go,
    internal/eval/agent/mcpconfig.go,
    internal/eval/agent/mcpconfig_test.go
  </files>
  <behavior>
    - Test 1 (RED): TestWriteMCPConfig — given absHelixBin, sockPath, homePath: emits valid JSON with `mcpServers.helix.type=="stdio"`, `command==absHelixBin`, `args==["--socket",sockPath]`, `env=={"HOME":homePath}`. File mode 0600.
    - Test 2 (RED): TestMCPConfigRejectsRelativeBin — `WriteMCPConfig(relPath, ...)` returns error (Pitfall 1 mitigation: never depend on PATH).
    - Test 3 (RED): TestAgentBuildsArgv — `(*Agent).buildArgv(task, mode)` produces argv containing exactly: `--print --bare --strict-mcp-config --mcp-config <path> --output-format stream-json --verbose --include-partial-messages --max-turns <N> <prompt>`. Order is fixed and asserted.
    - Test 4 (RED): TestAgentCleanEnv — `(*Agent).cleanEnv()` returns ONLY entries for keys in the allowlist (`PATH`, `HOME`, `ANTHROPIC_API_KEY`, `HELIX_LOG_LEVEL`). A key like `OPENAI_API_KEY` in the host env MUST NOT appear in the result.
    - Test 5 (RED): TestAgentSkipsIfClaudeMissing — if `exec.LookPath("claude")` errors, `(*Agent).Run` returns a sentinel `ErrClaudeNotFound`; tests using a fake `claude` script use a stub binary on a temp PATH.
    - Test 6 (RED): TestAgentCapturesStdoutStderr — fake `claude` script writes "hello" to stdout and "warn" to stderr; `Run` returns paths to the captured files; files are mode 0600.
    - Test 7 (RED): TestAgentRespectsContextDeadline — `Run` with a 100ms context cancel kills the subprocess and returns ctx.Err().
  </behavior>
  <action>
    `internal/eval/agent/mcpconfig.go`:
    - `type MCPServerConfig struct { Type string `json:"type"`; Command string `json:"command"`; Args []string `json:"args"`; Env map[string]string `json:"env"` }`
    - `type MCPConfig struct { MCPServers map[string]MCPServerConfig `json:"mcpServers"` }`
    - `WriteMCPConfig(path string, helixAbsBin, sockPath, homePath string) error` — assert `filepath.IsAbs(helixAbsBin)`; build struct; `os.WriteFile(path, json, 0600)`.

    `internal/eval/agent/claude.go`:
    - `type Agent struct { sandbox *sandbox.Sandbox; helixBin string }`
    - `func NewAgent(sb *sandbox.Sandbox, helixBin string) *Agent`
    - `(*Agent) cleanEnv() []string` — allowlist: `PATH, HOME (sandbox.HomeFor), ANTHROPIC_API_KEY, HELIX_LOG_LEVEL`. HOME is set per-call; others copied from `os.Getenv` if set.
    - `(*Agent) buildArgv(task Task, mode string) []string` — exact order per test 3.
    - `(*Agent) Run(ctx context.Context, task Task, mode string) (*Result, error)` — writes mcp-config; spawns `claude` via `exec.CommandContext`; sets `cmd.Dir = sandbox.RepoFor`; `cmd.Env = cleanEnv()`; redirects stdout/stderr to `<modeDir>/claude.stdout` / `claude.stderr` with mode 0600; waits; returns Result{StdoutPath, StderrPath, ExitCode, Duration}.
    - `Task` is a struct with `ID, Prompt, Budget` fields; `Budget` carries `MaxToolCalls int` (used for `--max-turns`).
    - `var ErrClaudeNotFound = errors.New("claude CLI not found on PATH")`.

    Use a `t.Helper`-based fake claude binary: a small Go program written to a temp file at test setup (or a shell script written from the Go test). Mirror whatever pattern `internal/forwarder/forwarder_test.go` uses for fake daemons.
  </action>
  <verify>
    <automated>go test ./internal/eval/agent -count=1 -race</automated>
  </verify>
  <done>
    Seven agent tests pass; race-clean. `--bare --strict-mcp-config` enforced. Allowlist documented inline. `go vet ./internal/eval/agent/...` clean.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: Budget watchdog — D-08 four-axis enforcement (max_input_tokens / max_output_tokens / max_seconds / max_tool_calls)</name>
  <files>
    internal/eval/budget/budget.go,
    internal/eval/budget/budget_test.go
  </files>
  <behavior>
    - Test 1 (RED): TestBudgetDefaults — defaults are 200000 / 32000 / 300s / 100 calls per CONTEXT.md D-08.
    - Test 2 (RED): TestBudgetParseYAML — `LoadFromFile("budget.yaml")` reads a per-task override; missing keys fall back to defaults; unknown keys with `KnownFields(true)` cause error.
    - Test 3 (RED): TestWatchdogSecondsBreach — `Watchdog` started with `MaxSeconds=1` cancels its context after ~1s and reports `BreachReason{Axis:"seconds", Value:1}`.
    - Test 4 (RED): TestWatchdogToolCallsBreach — `Watchdog.RecordToolCall()` past `MaxToolCalls` triggers context-cancel and `BreachReason{Axis:"tool_calls"}`.
    - Test 5 (RED): TestWatchdogTokensBreach — `Watchdog.RecordTokens(input, output)` past `MaxInputTokens` or `MaxOutputTokens` triggers cancel + breach reason.
    - Test 6 (RED): TestWatchdogReasonStringConstruction — assert that on each axis breach the watchdog populates a `*BreachReason` (type from Plan 01) with the correct `Axis` value; `BreachReason.String()` test itself lives in Plan 01's types_test.go.
  </behavior>
  <action>
    `internal/eval/budget/budget.go`:
    - `type Budget struct { MaxInputTokens, MaxOutputTokens, MaxSeconds, MaxToolCalls int }`
    - `var Default = Budget{MaxInputTokens:200000, MaxOutputTokens:32000, MaxSeconds:300, MaxToolCalls:100}`
    - `func LoadFromFile(path string) (Budget, error)` — yaml.v3 with `dec.KnownFields(true)`; merges over `Default`.
    - **`BreachReason` IS NOT redeclared here** — it ships in Plan 01 (`internal/eval/budget/types.go`) as a Wave-0 cross-plan type contract so Plan 02 and Plan 03 can run parallel-disjoint in Wave 1. This plan IMPORTS `budget.BreachReason` from the same package (same-package reference, no cross-package import) and constructs values like `&BreachReason{Axis:"seconds", Limit:int64(b.MaxSeconds), Observed:int64(elapsed.Seconds())}`.
    - `type Watchdog struct { ... }`
    - `func NewWatchdog(ctx context.Context, b Budget) (*Watchdog, context.CancelFunc)` — wraps `context.WithTimeout(ctx, MaxSeconds * time.Second)` plus per-axis counters.
    - `(*Watchdog) RecordToolCall()` — atomically increments; if past `MaxToolCalls`, sets `breach` and calls inner cancel.
    - `(*Watchdog) RecordTokens(input, output int)` — same for token axes.
    - `(*Watchdog) Breach() *BreachReason` — returns non-nil after a breach; nil otherwise.
    - `(*Watchdog) Done() <-chan struct{}` — proxies inner ctx.Done.

    The seconds watchdog runs as a goroutine that selects on the inner ctx.Done; on timer-fire it sets `breach = &BreachReason{Axis:"seconds", Limit:int64(b.MaxSeconds), Observed:int64(elapsed.Seconds())}`. Race-safe via `sync/atomic` or a mutex.

    Tests use `time.Sleep(1100*time.Millisecond)` for the seconds test (tolerate 100ms slack) and synthetic increments for the count tests; no real subprocess work.
  </action>
  <verify>
    <automated>go test ./internal/eval/budget -count=1 -race</automated>
  </verify>
  <done>
    Six budget tests pass; race-clean. `BreachReason.String()` matches D-08 wording verbatim. `go vet ./internal/eval/budget/...` clean.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Eval runner ↔ daemon-under-test | Trusted (we spawn it; clean env) |
| Eval runner ↔ `claude` subprocess | Semi-trusted (binary we did not build); contained via `--bare`, `--strict-mcp-config`, scrubbed env |
| Per-mode tmpdir ↔ host filesystem | Trusted but local-attacker-vulnerable on shared hosts |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-67-01 | T (Tampering) | per-task `repo/` tmpdir | mitigate | `os.MkdirTemp` with 0700 perms; reject if path resolves through a symlink (TestSandboxRefusesSymlinkedTmpdir asserts). |
| T-67-02 | T (Tampering) | claude subprocess writes outside per-task `repo/` | mitigate | `cmd.Dir = sandbox.RepoFor(task, mode)`; trace post-processing (Plan 03) asserts patches stay under repo root. This plan ensures the cwd is set; Plan 03 asserts the path-prefix invariant. |
| T-67-03 | T (Tampering) | `--mcp-config` path injection | mitigate | `WriteMCPConfig` requires `filepath.IsAbs(helixBin)`; mcp-config path is constructed by sandbox (no user-controlled portion). Mode names are validated against an enum at the runner layer (Plan 06). |
| T-67-04 | S (Spoofing) | daemon Unix socket | mitigate | Per-mode tmpdir socket inside 0700 dir; sandbox refuses to start if socket exists at start (TestSandboxIsolation indirectly enforces). |
| T-67-Pitfall-1 | I (Info Disclosure) | env-var leakage from dev `~/` into eval run | mitigate | `(*Agent).cleanEnv()` strict allowlist (TestAgentCleanEnv asserts unknown env keys are dropped); `--bare --strict-mcp-config` disables auto-discovery of `~/.claude/`. |

Block on: HIGH severity. T-67-01..T-67-04 all HIGH (eval correctness depends on isolation). Mitigations enforced via tests in this plan.
</threat_model>

<verification>
- Sandbox isolates two modes: `TestSandboxIsolation` proves it.
- Daemon spawn pattern matches `internal/forwarder/dial.go` socket-wait loop.
- Agent uses `--bare --strict-mcp-config` (TestAgentBuildsArgv asserts argv shape).
- Env scrubbing is enforced (TestAgentCleanEnv asserts allowlist).
- Budget watchdog covers all four D-08 axes.
- `go vet ./internal/eval/...` and full `go test ./internal/eval/...` clean.
</verification>

<success_criteria>
- [ ] `internal/eval/sandbox/sandbox.go` ships with 7 GREEN tests.
- [ ] `internal/eval/agent/claude.go` ships with 7 GREEN tests.
- [ ] `internal/eval/budget/budget.go` ships with 6 GREEN tests.
- [ ] All tests race-clean (`-race`).
- [ ] No new external deps; uses Go stdlib + yaml.v3 (already in repo).
- [ ] T-67-01..T-67-04 mitigations have a corresponding test that would fail if the mitigation regressed.
</success_criteria>

<dependencies>
- Plans: 67-01 (eval/* package skeletons must exist).
- External: helix daemon binary buildable for daemon-spawn integration test (uses repo's own helix). `claude` CLI is NOT required for these tests (fake binary in tests).
</dependencies>

<output>
After completion, create `.planning/phases/67-evaluation-harness/67-02-SUMMARY.md` recording: sandbox layout final shape, env-allowlist final list, budget defaults shipped, any deviations from RESEARCH §"Subprocess Isolation Design".
</output>
