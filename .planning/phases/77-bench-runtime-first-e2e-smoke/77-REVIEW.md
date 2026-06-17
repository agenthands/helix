---
phase: 77-bench-runtime-first-e2e-smoke
reviewed: 2026-06-17T00:00:00Z
depth: standard
files_reviewed: 23
files_reviewed_list:
  - bench/runtime/cell.go
  - bench/runtime/drive.go
  - bench/runtime/matrix.go
  - bench/runtime/result.go
  - bench/runtime/cctap.go
  - bench/runtime/sandbox/sandbox.go
  - bench/runtime/subprocess/daemon.go
  - bench/runtime/subprocess/claude.go
  - bench/runners/mode_resolver.go
  - bench/schema/schema.go
  - cmd/helix-bench/main.go
  - bench/runtime/cell_test.go
  - bench/runtime/cross_cell_test.go
  - bench/runtime/daemon_tap_integration_test.go
  - bench/runtime/matrix_test.go
  - bench/runtime/result_test.go
  - bench/runtime/cctap_test.go
  - bench/runners/mode_resolver_test.go
  - cmd/helix-bench/run_cmd_test.go
  - bench/datasets/toolbench-go/sum-doubler/sum.go
  - bench/datasets/toolbench-go/sum-doubler/sum_test.go
  - Makefile
  - bench/BENCH.md
findings:
  critical: 0
  warning: 6
  info: 5
  total: 11
status: issues_found
---

# Phase 77: Code Review Report

**Reviewed:** 2026-06-17
**Depth:** standard
**Files Reviewed:** 23
**Status:** issues_found

## Summary

Phase 77 wires a Go bench orchestrator that thin-wraps `internal/eval`. The
reuse-heavy structure is correct for the stated scope, path-traversal validation
is applied consistently at every join boundary (V5/T-77-* controls are real and
tested), the result.v2 schema validate-on-write gate is sound, and the
preserve-on-failure scratch contract is honored on the failure path. No security
vulnerabilities (injection, traversal, secret leak) were found in the in-scope
files — the subprocess argv is fixed/flag-based, env is allowlisted upstream, and
JSON-RPC frames are built with `%q`/`json.Marshal` rather than raw interpolation.

The defects that remain are correctness/robustness gaps concentrated in two
areas: (1) **outcome conflation** — context cancellation / timeout during
`verify.sh` (and during the kill→tap window) is silently recorded as a normal
verify *failure* rather than an infrastructure error, which corrupts the
pass/fail signal the whole bench produces; and (2) **drive-leg fragility** — the
forwarder response reader can leak a goroutine and silently drops out-of-order
responses, and an unconsumed `initialize` response makes the dispatch ordering
load-bearing in a way that is not defended. None rise to BLOCKER because the
hermetic single-cell CI path avoids the trigger conditions, but each degrades the
benchmark's correctness or robustness under the parallel / timeout conditions the
phase explicitly targets.

## Warnings

### WR-01: `verify.sh` context cancellation is recorded as a benchmark FAILURE, not an infra error

**File:** `bench/runtime/cell.go:294,361-375` (with `internal/eval/trace/merge.go:145`)
**Issue:** `runVerify` runs `verify.sh` under the cell `ctx` via
`exec.CommandContext`. When `ctx` is cancelled or its deadline fires (e.g. the
`TestCrossCell` 90s / `TestDaemonTap` 60s budgets, or any operator
Ctrl-C / matrix-level timeout), the context kills the process and `cmd.Run()`
returns a non-`ExitError` (or a SIGKILL-coded `ExitError`). `runVerify` maps that
to exit code `1` (or the signal-derived code), which flows into
`trace.Merge` → `Outcome="failed"`. The cell then reports `Success=false` as if
the agent *failed the task*, when in fact the harness was cancelled. This
corrupts the headline pass/fail number the bench exists to produce and is
indistinguishable, in the result.v2, from a genuine task failure.
**Fix:** Distinguish cancellation from a real non-zero verify exit. Return the
context error as an infrastructure error (preserve-on-failure) instead of an
outcome:
```go
func runVerify(ctx context.Context, scriptPath, repoDir string) (int, error) {
	if _, err := os.Stat(scriptPath); err != nil {
		return 0, nil // no verify.sh — auto-pass
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", scriptPath)
	cmd.Dir = repoDir
	err := cmd.Run()
	if ctx.Err() != nil {
		return 0, fmt.Errorf("verify cancelled/timed out: %w", ctx.Err())
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, nil
	}
	return 0, nil
}
```
and at the call site treat the new error via `preserve(...)` rather than feeding a
fabricated exit code into `Merge`.

### WR-02: Forwarder response reader can leak a goroutine on unexpected extra id-bearing lines

**File:** `bench/runtime/drive.go:91-92,182-195,242-260`
**Issue:** `respCh` is buffered to exactly `len(script.Steps)+2` (init + activate
+ one per step). `readResponses` runs for the lifetime of forwarder stdout and
pushes *every* line that parses with a numeric `id`. If the daemon/forwarder ever
emits more id-bearing lines than expected (a duplicate response, a server→client
request that happens to carry a numeric id, or a retried frame), the buffer
fills and `respCh <- parsed` (drive.go:188) blocks permanently: by then the drive
loop has already returned, no one is draining, and the deferred
`cmd.Process.Kill()` closes the pipe but cannot unblock a goroutine parked on a
*channel send*. The goroutine leaks for the life of the process.
**Fix:** Make the send non-blocking (drop on overflow) or have the reader select
on a done channel:
```go
select {
case respCh <- parsed:
default: // channel full; drive already drained — drop rather than leak
}
```
(plus close-or-signal the reader from the drive `defer`).

### WR-03: `waitForResp` permanently drops out-of-order step responses, turning ordering into an unguarded invariant

**File:** `bench/runtime/drive.go:242-261`
**Issue:** `waitForResp` consumes from `respCh` and *discards* any response whose
id does not match the one currently awaited (the `// Out-of-order ... ignore`
branch). Because steps are awaited strictly in dispatch order (id 3, then 4, …),
if the daemon ever returns step responses out of order — id=4 arriving before
id=3 — the wait for id=3 silently eats id=4, then the subsequent wait for id=4
finds nothing and times out after `driveDeadline` (10s). The step is recorded
with a spurious timeout error and its real (already-delivered) response is lost.
Today a single daemon over one socket responds in order so this does not fire,
but nothing enforces it, and the failure mode is a silent 10s-per-step stall plus
a corrupted CC leg.
**Fix:** Buffer responses by id in a small map so a response for a not-yet-awaited
id is retained rather than dropped:
```go
pending := map[int]jsonrpcResp{}
// in waitForResp: check pending[id] first; otherwise read, stashing
// non-matching ids into pending instead of discarding them.
```

### WR-04: A stray dotfile / dot-dir in the dataset dir fails the ENTIRE run, not just that entry

**File:** `cmd/helix-bench/main.go:245-262` (with `bench/runtime/matrix.go:83-91`)
**Issue:** `discoverTasks` returns every subdirectory name under
`<datasets>/<benchmark>/`, including names beginning with `.` (e.g. a `.git`,
`.DS_Store` dir, or an editor scratch dir). Those names are then handed to
`ExpandMatrix`, whose `validateMatrixID` rejects any leading-dot id — failing the
whole expansion with a hard error before a single legitimate task runs. A benign
filesystem artifact in the dataset tree thus takes down the entire bench run.
**Fix:** Filter non-task entries during discovery rather than failing the run:
```go
for _, e := range entries {
	if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
		tasks = append(tasks, e.Name())
	}
}
```
(and skip non-dirs, which it already does). Optionally also skip dirs lacking a
`task.json`/`scripted_agent.yaml`.

### WR-05: `Kill` uses a process-group signal (`syscall.Kill(-pid, …)`) but the daemon is never put in its own process group

**File:** `internal/eval/sandbox/sandbox.go:216,231-272` (reached by every cell via `bench/runtime/subprocess/daemon.go` → `cell.go:284`)
**Issue:** The `Kill` 5s-timeout fallback escalates with
`syscall.Kill(-pid, syscall.SIGKILL)` to reap daemon-spawned children
(language servers, `go test`). But `StartDaemon` never sets
`cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`, so the daemon does NOT
lead its own process group — it shares the bench harness's group. `-pid` then
references a process group keyed by the daemon's pid, which is not a group leader,
so the call returns ESRCH and reaps nothing; the child LS/processes the comment
claims to clean up survive. (In the pathological case where `pid` collides with a
real pgid it could even signal unintended processes, though that is unlikely.)
Net: the documented child-reaping guarantee is not delivered, so a hung
`go test` / language server can leak per cell — directly relevant under
`--parallel`. This lives in the reused eval sandbox but is exercised on the bench
critical path.
**Fix:** Put the daemon in its own group at spawn so the negative-pid kill targets
exactly its descendants:
```go
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
```

### WR-06: `start` clock for the merged trace is captured after daemon spawn, understating duration and risking pre-`start` daemon events

**File:** `bench/runtime/cell.go:239,301-311`
**Issue:** `start := time.Now()` is taken *after* `StartDaemon` returns
(which itself blocks up to 10s polling for the socket), and is then passed as
`MergeInput.StartedAt`. Any daemon-side `tool_call` whose log timestamp predates
`start` (the daemon may emit activation/warm-up tool calls during the socket-wait
window before `start` is sampled) sorts before `StartedAt`, and the reported
`DurationMs` excludes the multi-second daemon boot. For a benchmark whose entire
purpose is to publish capability/latency numbers, anchoring the span after the
expensive setup is a measurement bug.
**Fix:** Capture `start` before `StartDaemon` (or capture two timestamps: a
`spawnedAt` for the harness and a `start` aligned to the first agent action), and
document which span `DurationMs` represents.

## Info

### IN-01: `runVerify` doc comment claims SIGKILL "flushes slog buffers" — SIGKILL cannot flush

**File:** `bench/runtime/cell.go:283` (and the package doc at `cell.go:13`)
**Issue:** The comment "Kill the daemon to flush slog buffers, THEN PID-gated tap"
is misleading: `DaemonHandle.Kill` ultimately relies on `Process.Kill()` (SIGKILL),
which terminates the process immediately with no opportunity to flush userspace
buffers. The tap only works because the daemon's slog writes to `daemon.log`
line-by-line (unbuffered at the app layer). The correctness of the tap therefore
hinges on an undocumented daemon-side property, not on the kill "flushing"
anything.
**Fix:** Reword to state the real invariant (the daemon must write tool_call log
lines synchronously/unbuffered; kill exists to terminate, not to flush) so a
future change to daemon log buffering is recognized as tap-breaking.

### IN-02: `dispatch` spawns one goroutine per cell up front and ignores context cancellation while queued

**File:** `bench/runtime/matrix.go:160-187`
**Issue:** All `len(cells)` goroutines are launched immediately and each blocks on
the semaphore. On `ctx` cancellation, already-queued goroutines still acquire the
sem and call `runOneCell`, which proceeds through sandbox creation / daemon spawn
before the cancelled `ctx` short-circuits anything downstream. For large matrices
this does wasted setup/teardown after the operator has cancelled.
**Fix:** Check `ctx.Err()` after acquiring the semaphore and record a cancelled
outcome instead of running:
```go
sem <- struct{}{}
defer func() { <-sem }()
if ctx.Err() != nil {
	oc := CellOutcome{Cell: c, Err: ctx.Err()}
	mu.Lock(); outcomes[i] = oc; mu.Unlock()
	return
}
```

### IN-03: Test fixture uses a non-existent `relative_path` arg for `read_file`

**File:** `cmd/helix-bench/run_cmd_test.go:28`
**Issue:** The synthetic `scripted_agent.yaml` fixture passes
`args: { relative_path: sum.go }` to `read_file`, but the real tool schema
(`internal/kernel/fileops/tools.go:22`) names the field `path`. The test tolerates
cell failure so it still passes, but the fixture encodes an incorrect tool
contract that could be copied into a real task and silently no-op.
**Fix:** Use `path: sum.go` to match `ReadFileArgs`.

### IN-04: `initialize` response (id=1) is sent but never awaited or drained explicitly

**File:** `bench/runtime/drive.go:88-119`
**Issue:** The `initialize` frame (id=1) is written but its response is never
waited on — `_ = idInit` only silences the unused const. It is implicitly drained
(and discarded) by the first `waitForResp(idActivate=2)` call via the
drop-non-matching path. This makes the dispatch ordering load-bearing and couples
the init handling to the activate wait; it works today only because the
forwarder happens to answer init before activate. Combined with WR-03 it is a
fragility, not a standalone bug.
**Fix:** Either explicitly `waitForResp(ctx, respCh, idInit)` before sending
activate, or document that init's reply is intentionally drained by the activate
wait and rely on the id-keyed buffering from WR-03.

### IN-05: `RunIndex` is hard-wired to 0 for every cell, so repeated (task,mode) cells would collide on the same out dir

**File:** `bench/runtime/matrix.go:219` and `bench/runtime/cell.go:164-165`
**Issue:** `runOneCell` always sets `RunIndex: 0`, and the durable artifact path
is `<OutDir>/<task>/<mode>/result.v2.json` with no run-index segment. The matrix
today produces one cell per (benchmark,mode,task) so there is no collision, but
the moment repetitions (pass@k / multi-run) are introduced — which the
`RunIndex` field plainly anticipates — every repetition will overwrite the same
two files. The field exists but is inert and the path layout does not use it.
**Fix:** When repetitions land (Phase 79), thread a real `RunIndex` and include it
in the durable path (`<task>/<mode>/<run_index>/`), or document explicitly that
single-rep is the only supported layout for Phase 77 and assert `RunIndex==0`.

---

_Reviewed: 2026-06-17_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
