---
phase: 77-bench-runtime-first-e2e-smoke
fixed_at: 2026-06-17T00:00:00Z
review_path: .planning/phases/77-bench-runtime-first-e2e-smoke/77-REVIEW.md
iteration: 1
findings_in_scope: 11
fixed: 11
skipped: 0
status: all_fixed
---

# Phase 77: Code Review Fix Report

**Fixed at:** 2026-06-17
**Source review:** .planning/phases/77-bench-runtime-first-e2e-smoke/77-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 11 (0 critical, 6 warning, 5 info — fix_scope=all)
- Fixed: 11
- Skipped: 0

All fixes were applied in an isolated git worktree, committed atomically, and
verified with `go build ./...`, `go vet`, `go test`, and the
`HELIX_BIN`-backed integration tests (TestDaemonTap + TestCrossCell both PASS).

## Fixed Issues

### WR-01: `verify.sh` context cancellation recorded as a benchmark FAILURE
**Files modified:** `bench/runtime/cell.go`
**Commit:** 07a2d11f
**Applied fix:** Changed `runVerify` to return `(int, error)`. After `cmd.Run()`
it checks `ctx.Err()`; on cancellation/timeout it returns an infra error
(`verify cancelled/timed out: %w`) instead of fabricating an exit code. Real
non-zero exits still go through `errors.As(err, &exitErr)` and return the exit
code with a nil error. The call site routes the new error through `preserve(...)`
rather than feeding a fabricated code into `trace.Merge`.
**Requires human verification:** this is a logic/outcome-classification change
(cancellation vs. genuine verify failure); please confirm the cancellation path
behaves as intended in your environment.

### WR-02 / WR-03 / IN-04: drive-leg reader/dispatch redesign
**Files modified:** `bench/runtime/drive.go`
**Commit:** f9360578
**Applied fix:** Implemented as one coherent reader/dispatch design (per fix
guidance), not three conflicting patches:
- **WR-03:** introduced `respDispatcher{ch, pending map[int]jsonrpcResp}`. Its
  `wait(ctx, id)` consults the pending buffer first, and stashes any non-matching
  id into `pending` instead of discarding it, so an out-of-order response (id=4
  before id=3) is retained rather than lost. Dispatch order is no longer a silent
  invariant. Single-consumer (drive goroutine only), so no lock.
- **WR-02:** added a `done` channel closed by the drive `defer`. `readResponses`
  now `select`s `respCh <- parsed` vs `<-done`, so once the drive loop returns
  and stops draining, an unexpected extra id-bearing line causes the reader to
  return rather than parking forever on a full channel (goroutine-leak fix).
- **IN-04:** the `initialize` reply (id=1) is intentionally drained/buffered via
  the id-keyed `pending` map; documented that it is not awaited explicitly and
  that the dispatch order is no longer load-bearing (relies on WR-03 buffering).

### WR-04: stray dotfile/dot-dir in dataset dir fails the entire run
**Files modified:** `cmd/helix-bench/main.go`
**Commit:** 88d8fe67
**Applied fix:** Added `strings` import; `discoverTasks` now skips entries with a
leading dot (`!strings.HasPrefix(e.Name(), ".")`) in addition to non-dirs, so a
benign `.git`/`.DS_Store`/editor scratch dir no longer hard-fails `ExpandMatrix`.

### WR-05: `Kill` process-group signal but daemon not in its own group
**Files modified:** `internal/eval/sandbox/sandbox.go` (SHARED Phase-67 eval
sandbox, exercised by both eval and bench)
**Commit:** cfe89dac
**Applied fix:** Minimal change at the daemon spawn — set
`cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` in `StartDaemon` so the
`Kill` 5s-timeout fallback's `syscall.Kill(-pid, SIGKILL)` targets exactly the
daemon's own process group (and its LS/`go test` descendants) instead of
returning ESRCH. Confirmed BOTH suites stay green:
`go test ./internal/eval/...` (incl. the 82s runner integration tests and the
sandbox tests) and `go test ./bench/...` all pass.

### WR-06: trace `start` captured after daemon spawn, understating duration
**Files modified:** `bench/runtime/cell.go`
**Commit:** 4cb6964c
**Applied fix:** Moved `start := time.Now()` to BEFORE `subprocess.StartDaemon`
(which blocks up to 10s polling for the socket), with a comment documenting that
`start`/`DurationMs` now spans the full harness window including daemon boot, so
daemon-side warm-up tool_calls no longer sort before `StartedAt`.

### IN-01: misleading "SIGKILL flushes slog buffers" comment
**Files modified:** `bench/runtime/cell.go`
**Commit:** 5f87f8cc
**Applied fix:** Reworded the package-doc step and the step-(5) comment to state
the real invariant: the kill terminates (it cannot flush userspace buffers); the
tap is correct only because the daemon writes tool_call log lines
synchronously/unbuffered at the app layer, and introducing daemon log buffering
would break the tap.

### IN-02: dispatch ignores context cancellation while cells are queued
**Files modified:** `bench/runtime/matrix.go`
**Commit:** b8458a63
**Applied fix:** After acquiring the semaphore in `dispatch`, added an
`if ctx.Err() != nil` check that records a cancelled `CellOutcome{Cell, Err}` and
returns instead of proceeding into sandbox creation / daemon spawn.

### IN-03: test fixture uses non-existent `relative_path` arg for `read_file`
**Files modified:** `cmd/helix-bench/run_cmd_test.go`
**Commit:** 0d3eabbe
**Applied fix:** Changed the synthetic `scripted_agent.yaml` fixture arg from
`relative_path: sum.go` to `path: sum.go` to match the real `ReadFileArgs`
schema (`internal/kernel/fileops/tools.go`).

### IN-05: `RunIndex` hard-wired to 0; path layout does not use it
**Files modified:** `bench/runtime/cell.go`
**Commits:** a2f43d33 (initial guard), 30a71607 (corrected to doc-only)
**Applied fix (fix-OR-document → document):** Chose the documentation option
(no Phase-79 run-index path machinery, per scope guidance). The initial commit
added a hard `RunIndex==0` guard, but that broke the existing `TestCrossCell`,
which legitimately passes `RunIndex=0..2` with DISTINCT `OutDir`s per cell (no
collision). The corrective commit replaced the guard with a clear comment:
`RunIndex` is metadata-only this phase (it flows into result.v2's `run_index`),
the durable path has no run-index segment, repetitions sharing an `OutDir` would
overwrite, and callers must give repetitions distinct `OutDir`s until run-index
path segments land in Phase 79. Final state leaves all tests green.

## Verification Results

- `gofmt -l` of all touched files: empty (clean). (Note: pre-existing gofmt
  drift exists in several UNTOUCHED `internal/eval/...` files — left as-is to
  keep the fix scope narrow.)
- `go build ./...`: clean.
- `go vet ./bench/... ./cmd/helix-bench/... ./internal/eval/...`: clean.
- `go test ./bench/... ./cmd/helix-bench/...`: all PASS.
- `go test ./internal/eval/...`: all PASS (runner integration 82s, sandbox OK) —
  confirms the shared-sandbox WR-05 change is safe for eval.
- Integration (built helix on PATH via `HELIX_BIN`):
  `go test -count=1 ./bench/runtime/...` → **TestDaemonTap PASS, TestCrossCell
  PASS** (both ran, neither skipped).

---

_Fixed: 2026-06-17_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
