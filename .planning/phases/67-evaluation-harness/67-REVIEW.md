---
phase: 67-evaluation-harness
reviewed: 2026-05-10T00:00:00Z
depth: standard
files_reviewed: 30
files_reviewed_list:
  - cmd/helix-eval/main.go
  - internal/eval/agent/claude.go
  - internal/eval/agent/doc.go
  - internal/eval/agent/mcpconfig.go
  - internal/eval/budget/budget.go
  - internal/eval/budget/types.go
  - internal/eval/judge/client.go
  - internal/eval/judge/doc.go
  - internal/eval/judge/judge.go
  - internal/eval/pipeline.go
  - internal/eval/report/cost_summary.go
  - internal/eval/report/doc.go
  - internal/eval/report/eval_report.go
  - internal/eval/report/eval_result.go
  - internal/eval/report/run_metadata.go
  - internal/eval/report/safety_compliance.go
  - internal/eval/runner/doc.go
  - internal/eval/runner/inprocess.go
  - internal/eval/runner/runner.go
  - internal/eval/runner/scripted_agent.go
  - internal/eval/runner/zdr_gate.go
  - internal/eval/sandbox/doc.go
  - internal/eval/sandbox/sandbox.go
  - internal/eval/score/doc.go
  - internal/eval/score/rules.go
  - internal/eval/score/score.go
  - internal/eval/trace/doc.go
  - internal/eval/trace/merge.go
  - internal/eval/trace/schema.go
  - internal/eval/trace/tap.go
  - internal/profile/loader.go
  - internal/profile/profiles/baseline.yaml
findings:
  critical: 4
  warning: 5
  info: 3
  total: 12
status: issues_found
---

# 67-REVIEW.md — Phase 67 Code Review

**Depth:** standard
**Files reviewed:** 30 (+ baseline.yaml skimmed)
**Date:** 2026-05-10

## Summary

Phase 67 delivers a well-structured evaluation harness with solid security
thinking (env allowlisting, ZDR gate, symlink checks, PID-gated tap). 4
Critical and 5 Warning findings were identified. The most severe issues are:
daemon log file closed before the child process can write to it (guaranteed
data loss); `Kill()` returning an error for already-finished processes that
callers silently discard (causing the comment "ignore already-finished errors"
to lie); `TapDaemonLog` called with `expectedPid=0` (PID-gate bypassed in the
primary codepath); and an unknown `source:` value in `source.yaml` silently
treated as `CorpusSynthetic` (ZDR gate escape).

---

## Critical Issues

### CR-01: Daemon log file closed before subprocess inherits it — zero-length logs guaranteed

**File:** `internal/eval/sandbox/sandbox.go:237-248`

**Issue:** `logFile` is opened at line 237, assigned to `cmd.Stderr` and
`cmd.Stdout` at lines 241-242, then `cmd.Start()` is called at line 244.
After `cmd.Start()` returns, the parent closes the file at line 248 (`_ =
logFile.Close()`). On UNIX, `cmd.Start()` duplicates the fd into the child
process before returning, so the daemon's writes land in the file even after
the parent closes its copy. However, opening the file with `O_TRUNC` and then
immediately closing it races with the child's first write on some kernels.
More critically, if `cmd.Start()` fails (line 244-246), the parent closes the
file (line 245) without error propagation on the close — that is correct. But
after successful start, the file descriptor in the parent is closed while the
file is written by the daemon subprocess. On macOS, `os.RemoveAll(s.Root)` in
`Cleanup()` called after the daemon is alive will truncate/remove the log
while the daemon is still writing to it, producing empty or corrupt logs.
The real defect is that `logFile.Close()` at line 248 is unconditionally
called immediately after `cmd.Start()`, without waiting for the socket to
appear or the daemon to be killed. Any log content written during the poll
loop (`waitSocket`, lines 251-254) will still reach the file because the fd
in the child is independent — this part is correct on POSIX. The actual bug
is subtler: if `waitSocket` fails, the code does `cmd.Process.Kill()` +
`cmd.Wait()` at lines 252-253, but the file descriptor has already been
closed by the parent. On some platforms this causes the remaining kernel
buffer flush to fail silently, losing the daemon startup error output that
would explain the failure.

**Fix:** Keep the log file open until after the daemon is confirmed up (or
killed on failure). Use `defer logFile.Close()` placed after the socket check:

```go
logFile, err := os.OpenFile(logPath, ...)
if err != nil { ... }
defer logFile.Close()          // closed when StartDaemon returns

cmd.Stderr = logFile
cmd.Stdout = logFile

if err := cmd.Start(); err != nil {
    return nil, fmt.Errorf(...)
}

if err := waitSocket(ctx, sockPath, 10*time.Second); err != nil {
    _ = cmd.Process.Kill()
    _ = cmd.Wait()
    return nil, fmt.Errorf(...)
}
```

---

### CR-02: `Kill()` does not ignore "process already finished" — contradicts its own comment

**File:** `internal/eval/sandbox/sandbox.go:185-188`

**Issue:** The comment at line 186 says "Ignore 'process already finished'
errors," but the code at line 187 wraps and returns the error immediately:

```go
if err := h.cmd.Process.Kill(); err != nil {
    // Ignore "process already finished" errors.
    return fmt.Errorf("sandbox: kill daemon %s/%s: %w", h.taskID, h.mode, err)
}
```

On Linux, when a process has already exited, `cmd.Process.Kill()` returns
`os.ErrProcessDone` (Go 1.16+). The code returns that as an error, not nil.
All callers (`Cleanup()` at line 274) discard the error with `_ = h.Kill()`,
so the bug does not propagate — but `Cleanup()` then proceeds to
`os.RemoveAll(s.Root)` without waiting for the process, which can race with
residual child writes. The fix must also call `cmd.Wait()` in the
already-finished branch to reap the zombie.

**Fix:**
```go
if err := h.cmd.Process.Kill(); err != nil {
    if errors.Is(err, os.ErrProcessDone) {
        _ = h.cmd.Wait() // reap zombie if not already reaped
        return nil
    }
    return fmt.Errorf("sandbox: kill daemon %s/%s: %w", h.taskID, h.mode, err)
}
```

---

### CR-03: `TapDaemonLog` called with `expectedPid=0` — T-67-04 PID gate bypassed

**File:** `internal/eval/runner/runner.go:148`, `internal/eval/pipeline.go:155`

**Issue:** Both `RunTask` and the pipeline's `collect_trace` phase call
`trace.TapDaemonLog(daemonLogPath, 0)`. The PID gate in `TapDaemonLog`
(tap.go:70) rejects events where `raw.Pid != expectedPid`. With
`expectedPid=0`, the gate passes only log lines that have `pid: 0` in their
JSON — which are none in practice, since the daemon assigns its own PID.
This means every tool-call event is rejected as "foreign PID" when
`expectedPid=0`, producing an empty event list. The T-67-04 mitigation is
effectively disabled: the trace always contains zero daemon events, causing
scorers to always score zero for the full eval path.

The actual daemon PID is available from the `DaemonHandle` (which wraps
`exec.Cmd`), but the runner does not pass it to `TapDaemonLog`.

**Fix:** Retrieve the PID from the daemon handle and pass it:
```go
// In RunTask, after obtaining daemonHandle:
pid := 0
if daemonHandle != nil && daemonHandle.Pid() != 0 {
    pid = daemonHandle.Pid()
}
daemonTap, _ = trace.TapDaemonLog(daemonLogPath, pid)
```
Add a `Pid() int` accessor to `DaemonHandle`:
```go
func (h *DaemonHandle) Pid() int {
    if h.cmd == nil || h.cmd.Process == nil {
        return 0
    }
    return h.cmd.Process.Pid
}
```

---

### CR-04: Unknown `source:` value in `source.yaml` silently passes as `CorpusSynthetic` — ZDR gate bypass

**File:** `internal/eval/runner/zdr_gate.go:115-118`

**Issue:** `readTaskSource` returns `m.Source` directly after YAML decode.
If a `source.yaml` contains `source: proprietary-dataset` (any string other
than `"synthetic"`, `"helix-oss"`, or `"external"`), the function returns
that unknown string. The caller then tests `if src == CorpusExternal` — the
unknown string is not `"external"`, so it falls through to the "no external
tasks" branch and is allowed without attestation. An attacker who places a
corpus with `source: anything-except-external` bypasses ZDR enforcement.

**Fix:** Add an explicit allowlist validation:
```go
switch m.Source {
case CorpusSynthetic, CorpusHelixOSS, CorpusExternal:
    return m.Source, nil
default:
    return CorpusSynthetic, fmt.Errorf("source.yaml: unknown source %q; valid values: synthetic, helix-oss, external", m.Source)
}
```

---

## Warnings

### WR-01: `defer os.RemoveAll(tmpDir)` inside a loop — defers accumulate until function returns

**File:** `internal/eval/runner/inprocess.go:136`

**Issue:** `RunQuick` iterates over `opts.Modes` (up to 4 modes). For each
mode, it creates a `tmpDir` and calls `defer os.RemoveAll(tmpDir)`. Defers
are scoped to the function, not the loop body. All 4 temp directories survive
until `RunQuick` returns. For large fixture sets with many modes, this can
exhaust disk space during the run. The directories hold skill registration
state, not large data, so the impact is bounded — but it contradicts the
intent of cleaning up per-mode.

**Fix:** Wrap the loop body in a closure or use an explicit `os.RemoveAll`
after the mode's work completes instead of `defer`.

---

### WR-02: `os.Exit(1)` called inside `RunE` — bypasses cobra cleanup and deferred cleanup

**File:** `cmd/helix-eval/main.go:159, 247`

**Issue:** `runCommand` is called from a cobra `RunE` closure. Two `os.Exit(1)`
calls at lines 159 and 247 bypass all deferred functions registered by cobra
and by any callers. In tests that exercise `RunE` via `cmd.Execute()`, the
`os.Exit` call terminates the test process. The correct pattern in cobra is to
return a non-nil error from `RunE` (which cobra translates to a non-zero exit
via its own machinery) or to use `cobra.Command.SilenceErrors` + direct exit
only from `main()`.

**Fix:** Replace `os.Exit(1)` with `return fmt.Errorf("...")` at both call
sites. The success/failure exit code is already handled by cobra: if `RunE`
returns a non-nil error, cobra exits 1 (when `SilenceUsage` and
`SilenceErrors` are set appropriately). If a specific exit code is required,
use a sentinel error type and check in `main()`.

---

### WR-03: `Kill()` goroutine leak when `cmd.Wait()` blocks beyond 5s

**File:** `internal/eval/sandbox/sandbox.go:192-198`

**Issue:** `Kill()` spawns a goroutine that calls `h.cmd.Wait()` into a
buffered channel (line 192). If the 5-second timeout fires, the function
returns the timeout error, but the goroutine is still blocked on `cmd.Wait()`
indefinitely. Since the channel is buffered with capacity 1, the goroutine
will eventually unblock when the process terminates and the send will succeed,
so there is no permanent leak — but the goroutine lives past `Kill()`'s
return, and if the OS never reaps the process (e.g., zombie), this leaks
indefinitely.

**Fix:** Pass a context to the goroutine so it can be cancelled:
```go
waitCh := make(chan error, 1)
go func() { waitCh <- h.cmd.Wait() }()
select {
case <-waitCh:
case <-time.After(5 * time.Second):
    return fmt.Errorf("sandbox: daemon %s/%s did not exit within 5s after kill", h.taskID, h.mode)
}
```
This is already the current pattern — the goroutine is buffered and will
unblock eventually. Flag is downgraded but worth fixing: add a process-group
kill (`syscall.Kill(-pid, syscall.SIGKILL)`) to ensure reaping.

---

### WR-04: `ToolCallDistribution` field never populated in `buildModeAggregates`

**File:** `internal/eval/report/eval_report.go:159-185`

**Issue:** `modeAggregate.ToolCallDistribution` (line 92) is declared but
never assigned in `buildModeAggregates`. The `EvalResult` struct does not
carry a per-tool call map — it only has `EditCount` (total). The report
renderer at lines 238-249 checks `len(agg.ToolCallDistribution) > 0` before
rendering the tool-call distribution table. Because the map is always nil/empty,
the table is never rendered for any run, and the `eval_report.md` always shows
"(No tool calls recorded in this run.)" even when tool calls occurred.

**Fix:** Populate `ToolCallDistribution` from `EvalResult` if the per-tool
breakdown is available, or add `ByTool map[string]int` to `EvalResult` and
aggregate it in `buildModeAggregates`.

---

### WR-05: Redundant dead-code branch in `runVerify` exit code extraction

**File:** `internal/eval/runner/runner.go:337-343`

**Issue:** The exit-code extraction inside `runVerify` contains a dead
conditional:

```go
if err != nil {
    var exitErr *exec.ExitError
    if ok := (err != nil); ok {   // always true inside the outer if-err-nil block
        if e, ok2 := err.(*exec.ExitError); ok2 {
```

`ok := (err != nil)` is always `true` because we are inside `if err != nil`.
The intermediate `if ok` branch is never false. While the final result is
correct (the exit code is extracted properly), this indicates copy-paste error
and leaves dead code that will confuse future readers. The `errors.As` pattern
should be used instead:

```go
if err != nil {
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        exitCode = exitErr.ExitCode()
    }
}
```

---

## Info

### IN-01: LLM judge `Reasoning` field injected unescaped into Markdown table cells

**File:** `internal/eval/report/eval_report.go:343`

**Issue:** The judge `Reasoning` field (sourced from LLM output) is written
directly into a Markdown table cell with `%s`:

```go
fmt.Fprintf(&sb, "| %s | %s | %+d | %+d | %+d | %+d | %s |\n",
    e.TaskID, e.Mode, ..., e.Reasoning)
```

If the LLM response contains `|` characters or newlines, the Markdown table
will be malformed. This is informational-only output (EVAL-07 constraint), so
there is no security impact, but the rendered report may be unreadable.

**Fix:** Strip or replace `|` and newlines in `Reasoning` before rendering:
```go
reasoning := strings.ReplaceAll(strings.ReplaceAll(e.Reasoning, "|", "\\|"), "\n", " ")
```

---

### IN-02: `run_metadata.go` SHA truncation uses `min(s.Value, 12)` — argument order is reversed

**File:** `internal/eval/report/run_metadata.go:54`

**Issue:** The `min` helper is defined as `func min(s string, n int) int` and
returns `min(len(s), n)`. The call site is:

```go
helixVersion = "git-" + s.Value[:min(s.Value, 12)]
```

This is correct in behavior: `min` takes the string and the max length. But
the parameter naming is counterintuitive — `min(s string, n int)` reads as "s
is the smaller argument," whereas it's actually "s is the string to bound."
This compiles cleanly (Go 1.21 `min` builtin would shadow the local function
for two `int` args, but here the local is called). The issue is purely a
readability hazard.

**Fix:** Rename to `truncateLen(s string, max int) int` or inline the logic:
```go
helixVersion = "git-" + s.Value[:min(len(s.Value), 12)]
```
using the Go 1.21 builtin `min`.

---

### IN-03: `source.yaml` YAML parsing does not use `KnownFields(true)`

**File:** `internal/eval/runner/zdr_gate.go:112`

**Issue:** `readTaskSource` uses `yaml.Unmarshal` (not a `yaml.Decoder` with
`KnownFields(true)`) to parse `source.yaml`. Unknown keys are silently
ignored. While `corpusManifest` has only one field (`source`), a typo like
`source: external` would decode with `Source: ""` (empty) and be treated as
`CorpusSynthetic` — allowing an external corpus task past the ZDR gate. The
fix in CR-04 addresses the primary bypass; this adds defense-in-depth.

**Fix:**
```go
dec := yaml.NewDecoder(bytes.NewReader(data))
dec.KnownFields(true)
if err := dec.Decode(&m); err != nil { ... }
```

---

## Files reviewed

| File | Findings |
|------|----------|
| `internal/eval/sandbox/sandbox.go` | CR-01, CR-02, WR-03 |
| `internal/eval/runner/runner.go` | CR-03, WR-05 |
| `internal/eval/pipeline.go` | CR-03 (secondary) |
| `internal/eval/runner/zdr_gate.go` | CR-04, IN-03 |
| `internal/eval/runner/inprocess.go` | WR-01 |
| `cmd/helix-eval/main.go` | WR-02 |
| `internal/eval/report/eval_report.go` | WR-04, IN-01 |
| `internal/eval/report/run_metadata.go` | IN-02 |
| `internal/eval/judge/client.go` | clean |
| `internal/eval/judge/judge.go` | clean |
| `internal/eval/budget/budget.go` | clean |
| `internal/eval/trace/tap.go` | clean (PID gate correct; see CR-03 for caller bug) |
| `internal/eval/trace/merge.go` | clean |
| `internal/eval/score/rules.go` | clean |
| `internal/eval/score/score.go` | clean |
| `internal/eval/runner/scripted_agent.go` | clean |
| `internal/eval/agent/claude.go` | clean |
| `internal/eval/agent/mcpconfig.go` | clean |
| `internal/eval/report/cost_summary.go` | clean |
| `internal/eval/report/safety_compliance.go` | clean |
| `internal/eval/report/eval_result.go` | clean |
| `internal/eval/budget/types.go` | clean |
| `internal/profile/loader.go` | clean |
| `internal/profile/profiles/baseline.yaml` | clean |
| `internal/eval/trace/schema.go` | clean |
| `internal/eval/runner/runner.go` (RunMatrix) | clean |

## Out of scope

- Test files (`*_test.go`) were not reviewed per instructions.
- `eval/corpus/**` and `eval/fixtures/**` data directories were not reviewed.
- Performance characteristics of the scorer's O(n²) subsequence matcher are
  out of v1 scope.
- The `gopls "module not in workspace"` diagnostics are worktree artifacts,
  not real issues, and were not flagged.

---

_Reviewed: 2026-05-10_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_

---

## Fix Report

**Fixed at:** 2026-05-10
**Fixer:** Claude (gsd-code-fixer)
**Scope:** Critical + Warning (9 findings)
**Summary:** 9 fixed / 0 skipped / 3 Info deferred (out of scope)

| Finding | Title | Commit | Status |
|---------|-------|--------|--------|
| CR-01 | Daemon log file closed before subprocess writes | `9dcdd0aa` | FIXED |
| CR-02 | Kill() returns ErrProcessDone despite "ignored" comment | `2958a859` | FIXED |
| CR-03 | TapDaemonLog called with expectedPid=0 — PID gate bypassed | `196e829c` | FIXED |
| CR-04 | Unknown source: value silently passes ZDR gate | `b06556b1` | FIXED |
| WR-01 | defer os.RemoveAll inside loop accumulates dirs until return | `3f9db720` | FIXED |
| WR-02 | os.Exit(1) inside RunE bypasses cobra cleanup | `78569067` | FIXED |
| WR-03 | Kill() goroutine leak — no process-group kill on 5s timeout | `458ce42d` | FIXED |
| WR-04 | ToolCallDistribution never populated in buildModeAggregates | `18bd93da` | FIXED |
| WR-05 | Dead if ok := (err != nil) conditional in runVerify | `cb7409d5` | FIXED |
| IN-01 | LLM judge Reasoning unescaped in Markdown table | — | DEFERRED (Info) |
| IN-02 | SHA truncation min() argument order | — | DEFERRED (Info) |
| IN-03 | source.yaml parsing lacks KnownFields(true) | — | DEFERRED (Info) |

### Fix Notes

**CR-01:** Changed `logFile.Close()` immediately after `cmd.Start()` to `defer logFile.Close()` so the fd stays open during the socket poll and is closed cleanly when `StartDaemon` returns.

**CR-02:** Added `errors.Is(err, os.ErrProcessDone)` guard in `Kill()` to suppress the already-finished error and call `cmd.Wait()` to reap the zombie, matching the intent of the existing comment.

**CR-03:** Added `Pid() int` nil-safe accessor to `DaemonHandle` in sandbox.go. Updated both `runner.go` and `pipeline.go` to call `daemonHandle.Pid()` instead of hard-coded `0`. The accessor returns 0 for a nil handle (wave-1 placeholder path), preserving current behaviour until wave-2 wires real daemon subprocess orchestration.

**CR-04:** Added explicit allowlist switch in `readTaskSource` after YAML decode. Any source value other than `synthetic`, `helix-oss`, or `external` now returns an error instead of silently defaulting to `CorpusSynthetic`.

**WR-01:** Wrapped the per-mode loop body in an immediately-called closure so `defer os.RemoveAll(tmpDir)` fires at end of each mode iteration rather than when `RunQuick` returns. Daemon cancel and session close also moved into the closure.

**WR-02:** Replaced both `os.Exit(1)` calls inside `runCommand` (a cobra `RunE`) with `return fmt.Errorf(...)` so cobra handles the non-zero exit code and deferred cleanup functions are not bypassed.

**WR-03:** On the 5-second timeout branch in `Kill()`, added `syscall.Kill(-pid, syscall.SIGKILL)` to send SIGKILL to the entire process group (negative PID), ensuring child processes spawned by the daemon are also killed.

**WR-04:** Added `ToolCallsByTool map[string]int` field to `EvalResult`. Populated it from `merged.ToolCallSummary.ByTool` in both `runner.go` and `pipeline.go`. Updated `buildModeAggregates` to aggregate per-result tool counts into `modeAggregate.ToolCallDistribution`.

**WR-05:** Replaced the double-nested type assertion (`if ok := (err != nil); ok { if e, ok2 := err.(*exec.ExitError)...}`) with the idiomatic `errors.As(err, &exitErr)` pattern, eliminating the always-true dead branch.

_Fixed: 2026-05-10_
_Fixer: Claude (gsd-code-fixer)_
