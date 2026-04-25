---
phase: 56-bug-ls-notification-dispatch-and-jdtls-readiness
plan: 01
subsystem: lspool
tags: [lspool, lifecycle, jsonrpc, notifications, d-02]

requires:
  - phase: 03 (LanguageQuirks → QuirkAdapter)
    provides: ProcessHandle / Conn lifecycle that this plan splits apart
provides:
  - ProcessHandle.Start that no longer auto-launches the JSON-RPC dispatch loop
  - ProcessHandle.StartListen(ctx) as the only call site for `go p.conn.Listen(ctx)`
  - Lifecycle test (TestProcessHandle_StartListenSeparate) enforcing the contract
affects:
  - 56-02 (Worker.Start wiring)
  - 56-03 (jdtls language/status readiness)
  - 56-04 (full-suite gate)

tech-stack:
  added: []
  patterns:
    - "RED stub-then-fix: introduce method as no-op-ish stub in RED commit so test compiles, replace eager call site in GREEN commit"

key-files:
  created:
    - internal/kernel/lspool/process_test.go
  modified:
    - internal/kernel/lspool/process.go

key-decisions:
  - "Add StartListen as a stub in the RED commit so process_test.go compiles; the GREEN commit removes the eager Listen launch from Start. This keeps each commit individually green-on-vet while preserving a true RED → GREEN test trajectory."
  - "Test uses POSIX `cat` as a benign LSP-frame echo subprocess; explicit Windows skip + LookPath safety."

patterns-established:
  - "Lifecycle contract documented in StartListen doc-comment and enforced by TestProcessHandle_StartListenSeparate (regression bait for future refactors that re-add eager Listen)."

requirements-completed: [LSDISP-02]

duration: ~12min
completed: 2026-04-25
---

# Phase 56 Plan 01: ProcessHandle.Start / StartListen split Summary

**ProcessHandle.Start no longer auto-launches the JSON-RPC dispatch loop; new StartListen(ctx) is the sole call site for `go p.conn.Listen(ctx)`, closing the race window where notifications received before `OnNotification` assignment were silently dropped.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-04-25 (worktree session)
- **Completed:** 2026-04-25
- **Tasks:** 3 (Task 1 RED, Task 2 GREEN, Task 3 vet/test gate)
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments

- TestProcessHandle_StartListenSeparate exercises both states of the lifecycle contract: pre-StartListen frames sit unprocessed (`dispatched == 0` after 75ms with a frame in flight), post-StartListen frames dispatch within 1s.
- ProcessHandle.Start now only spawns the process, creates the `*jsonrpc.Conn`, and launches `drainStderr` + `reap` + `watchContext`. The `go p.conn.Listen(ctx)` line is gone (Phase 56 D-02 comment in its place).
- ProcessHandle.StartListen is now the only call site for `go p.conn.Listen(ctx)` in the file (`grep -c 'go p\.conn\.Listen'` returns exactly 1).
- Targeted gate green: `go test ./internal/kernel/lspool/... ./internal/kernel/jsonrpc/... -count=1` passes; `go vet ./...` exits 0.

## Task Commits

1. **Task 1: Scaffold process_test.go (RED)** — `5def465c` (test)
2. **Task 2: Restructure Start + finalise StartListen contract (GREEN)** — `19a979a6` (feat)
3. **Task 3: vet + targeted test gate** — no commit (verification-only task; plan declares no files for this task)

_Note: TDD plan; no refactor commit was needed — Task 2's GREEN code is already minimal._

## Files Created/Modified

- `internal/kernel/lspool/process_test.go` (created, 78 lines) — `TestProcessHandle_StartListenSeparate` + `sendNotification` helper. Direct same-package access to unexported `ph.stdin` field for frame injection.
- `internal/kernel/lspool/process.go` (modified, ~15 lines net) — Removed eager `go p.conn.Listen(ctx)` from `Start` (was at line 88 pre-fix), added doc-comment to `Start` documenting the new contract, added `StartListen(ctx context.Context)` method (~line 105-115 post-fix) right after the `Conn()` accessor.

### Constructor signature observed

`NewProcessHandle(command string, args []string, workDir string, env []string, logger *slog.Logger) *ProcessHandle`

(Plan example used `NewProcessHandle(cmd, logger)` shorthand; actual signature takes 5 args. Test wires `cat` with `NewProcessHandle("cat", nil, "", nil, testLogger())`.)

### Exact line numbers

- Pre-fix `Start`: lines 56-97 (`go p.conn.Listen(ctx)` was line 88).
- Post-fix `Start`: lines 58-103 (Phase 56 D-02 comment in place of the deleted line, ~lines 86-89).
- New `StartListen` method: lines 109-115.
- `grep -c 'go p\.conn\.Listen' internal/kernel/lspool/process.go` → `1` (only inside StartListen body).
- `grep -c 'Phase 56' internal/kernel/lspool/process.go` → `3` (Start doc-comment, in-Start D-02 marker, StartListen doc-comment).

## Decisions Made

- **Stub-then-fix RED:** The plan acceptance gate for Task 1 requires both `go vet` clean AND a runtime test failure. The only way to satisfy both was to introduce `StartListen` as a stub in the RED commit (so the test file compiles) while leaving `Start`'s eager Listen launch intact (so the runtime assertion fails). Task 2 then removes the eager launch and the same test passes. Each individual commit is internally consistent and the RED → GREEN trajectory is preserved at the assertion-failure level (not just compile-failure).
- **Cat as test fixture:** Per plan suggestion, `cat` echoes stdin to stdout, letting us inject LSP-framed notifications via `ph.stdin` and observe them on the read side. Explicit Windows skip + `exec.LookPath("cat")` guard.

## Deviations from Plan

None - plan executed exactly as written, with two clarifications worth recording for downstream waves:

1. The plan's example `NewProcessHandle(cmd, logger)` shorthand does not match the real 5-arg signature. The plan's instruction to "verify the actual `NewProcessHandle` signature" was followed; no plan change required.
2. Task 3 declares no files (`<files></files>`) so no commit was created for the verification-only step. This matches the plan's intent (Task 3 is a gate, not a code change).

## Issues Encountered

- **Worktree path confusion (executor-side, not a plan issue):** Initial Bash invocations defaulted to the parent repo path rather than the worktree path, causing the first round of edits to land in the wrong checkout. Reverted those changes immediately and redid the work using full worktree-absolute paths. No commits or content were lost; the git history of the worktree branch is clean.

## Pre-flight gate result

`grep -rn 'Worker{}\|\.Start(ctx' internal/kernel/lspool/*_test.go` returned no hits in the worktree base (6d2a0085). No pre-existing end-to-end Worker.Start lifecycle test exists in `internal/kernel/lspool`, so Wave 1 cannot regress one. Acceptance gate cleared with no escalation.

## Next Plan Readiness

- Plan 56-02 (Worker.Start wiring) can now call `process.StartListen(ctx)` after assigning `Conn().OnNotification`. The contract is documented in `StartListen`'s doc-comment and enforced by `TestProcessHandle_StartListenSeparate` (regression bait for any future refactor that tries to re-add eager Listen inside Start).
- Production paths through `Worker.Start` are intentionally broken between Wave 1 and Wave 2: any worker created during this window will hang on `initialize` because Listen is never started. Per plan Task 2 instructions, full `go test ./...` MUST NOT be run between waves; only the targeted lspool/jsonrpc gate (which is green).

## Self-Check: PASSED

- `internal/kernel/lspool/process_test.go` — FOUND
- `internal/kernel/lspool/process.go` — modified (FOUND)
- Commit `5def465c` — FOUND in `git log`
- Commit `19a979a6` — FOUND in `git log`
- `grep -c 'go p\.conn\.Listen' internal/kernel/lspool/process.go` returns 1 — VERIFIED
- `TestProcessHandle_StartListenSeparate` passes — VERIFIED
- `go vet ./...` exits 0 — VERIFIED
- `go test ./internal/kernel/lspool/... ./internal/kernel/jsonrpc/... -count=1` exits 0 — VERIFIED

---
*Phase: 56-bug-ls-notification-dispatch-and-jdtls-readiness*
*Completed: 2026-04-25*
