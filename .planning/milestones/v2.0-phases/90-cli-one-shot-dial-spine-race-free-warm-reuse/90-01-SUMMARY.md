---
phase: 90-cli-one-shot-dial-spine-race-free-warm-reuse
plan: 01
subsystem: infra
tags: [cli, forwarder, daemon, flock, race, ipc, synctest, tdd]

# Dependency graph
requires: []
provides:
  - "Race-free ConnectOrStartDaemon: N parallel cold callers spawn exactly one daemon (CLI-03)"
  - "Per-socket gofrs/flock startup lock with double-checked tryConnect (TOCTOU fix)"
  - "startupGuard algorithm with injectable seams (newLocker/connect/spawn/waitUp) for deterministic testing"
  - "lockfilePath(<socket>.lock) derivation reused by later dial-spine work"
  - "github.com/gofrs/flock v0.13.0 dependency (provenance-approved)"
affects:
  - "90-02 / 90-03 / 90-04 (one-shot verb dial + E2E real-PID oracle ride on this guard)"
  - "bench per-cell socket isolation (per-socket lock means distinct sockets do not serialize)"

# Tech tracking
tech-stack:
  added:
    - "github.com/gofrs/flock v0.13.0 (portable advisory file lock: Unix flock + Windows LockFileEx, no CGO)"
  patterns:
    - "Seam-injection for synctest: extract the concurrency algorithm into an unexported function taking interface/func seams so testing/synctest can drive it under a virtual clock without real exec/sleep/OS-lock"
    - "Double-checked locking around a process-spawn window (warm-reuse fast path + post-lock re-probe)"

key-files:
  created:
    - internal/forwarder/dial_lock.go
    - internal/forwarder/dial_race_test.go
  modified:
    - internal/forwarder/dial.go
    - go.mod
    - go.sum

key-decisions:
  - "Wrapped the lock inside ConnectOrStartDaemon (least churn) so activate.go / RunForwarder inherit the fix without signature changes"
  - "Injected an in-process mutex locker for synctest because a real OS flock would durably-block synctest's virtual clock (RESEARCH Pitfall 4)"
  - "Promoted gofrs/flock to a direct require manually (go mod tidy fails on an unrelated pre-existing transitive module, google/s2a-go)"

patterns-established:
  - "synctest seam pattern: startupGuard(ctx, socketPath, seams{...}) is the testable unit; production wires real tryConnect/startDaemon/waitForDaemon + a flock-backed locker"
  - "Per-socket lockfile derivation (<socketPath>.lock) — a regular file, never bind()ed, not AF_UNIX-length-limited"

requirements-completed: [CLI-03]

# Metrics
duration: 5min
completed: 2026-06-21
status: complete
---

# Phase 90 Plan 01: CLI One-Shot Dial Spine — Race-Free Daemon Spawn Summary

**Per-socket gofrs/flock startup lock with double-checked tryConnect in `ConnectOrStartDaemon`, proven by a synctest fan-out test to spawn exactly one daemon under N concurrent cold callers (CLI-03).**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-06-21T11:58:58Z
- **Completed:** 2026-06-21
- **Tasks:** 2
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments

- Closed the cross-process daemon-spawn race: the previously unlocked `tryConnect → startDaemon → waitForDaemon` window in `dial.go` now serializes the cold path behind a per-socket `gofrs/flock` guard, so N parallel cold `helix` callers spawn exactly one daemon instead of forking a daemon storm.
- Implemented double-checked locking (RESEARCH Pitfall 1): after acquiring the lock (whether by winning `TryLock` or blocking on `Lock`), a re-probe via `tryConnect` returns the peer's daemon if one came up during lock acquisition — no redundant spawn.
- Lock-loser reuse: a caller that loses `TryLock` blocks on `Lock` until the winner releases (winner's socket is up), then reuses it.
- Per-socket isolation: the lockfile is `<socketPath>.lock`, so distinct sockets (bench per-cell) do not serialize.
- Pinned the in-process lock/double-check ALGORITHM with a deterministic `testing/synctest` test against a spawn-counter seam (no real exec — the real-PID oracle is deferred to plan 90-04 per RESEARCH Pitfall 4).
- Added `github.com/gofrs/flock v0.13.0` (human-approved provenance) and kept the lock portable (no raw syscall lock) so the later Windows local-dial smoke survives.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add gofrs/flock dependency (human-approved checkpoint)** - `fcc56e4d` (chore)
2. **Task 2 (RED): Failing synctest for race-free spawn** - `435ff0d9` (test)
3. **Task 2 (GREEN): flock-guarded race-free daemon spawn** - `7f20a874` (feat)

_Task 2 is a TDD task: RED (`435ff0d9`) then GREEN (`7f20a874`). No REFACTOR commit was needed — the GREEN implementation was already clean._

## Files Created/Modified

- `internal/forwarder/dial_lock.go` (created) — `daemonLocker` interface, `flockLocker` (gofrs/flock) adapter, `newFlockLocker` factory seam, `lockfilePath`, the `seams` struct, and `startupGuard` (the testable lock/double-check algorithm).
- `internal/forwarder/dial_race_test.go` (created) — synctest fan-out tests: exactly-one-spawn under N goroutines, lock-loser reuse, TOCTOU double-check, per-socket independence, and `lockfilePath` derivation; uses an in-process mutex locker seam.
- `internal/forwarder/dial.go` (modified) — `ConnectOrStartDaemon` keeps the warm-reuse fast path, routes the cold path through `startupGuard` with a flock-backed locker, then does a final `tryConnect` for the real `*grpc.ClientConn`. Exported signature unchanged.
- `go.mod` / `go.sum` (modified) — `github.com/gofrs/flock v0.13.0` added (Task 1) and promoted to a direct require (Task 2, since dial.go now imports it).

## Decisions Made

- **Wrap-inside over new-function:** kept the exported `ConnectOrStartDaemon` signature and wrapped the lock inside it (PATTERNS "Discretion — least churn") so all existing callers inherit the fix for free.
- **In-process locker seam for synctest:** a real `gofrs/flock` performs a real `flock(2)` syscall, which synctest cannot virtualize — it would durably-block the virtual clock and deadlock the test (RESEARCH Pitfall 4). The race test therefore injects a `sync.Mutex`-backed `daemonLocker` (one mutex per lockfile path, so distinct sockets stay independent), exercising the exact algorithm without an OS lock.
- **Manual direct-require promotion:** `go mod tidy` exits non-zero on a pre-existing, unrelated transitive resolution failure (`google/s2a-go`, a sigstore dependency), so it never rewrote flock's `// indirect` annotation. Promoted the require entry by hand; `go build ./...` confirms the module graph is sound.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected the TOCTOU test model**
- **Found during:** Task 2 (GREEN)
- **Issue:** The initial RED `TestRace_DoubleCheck_TOCTOU` modeled the connect seam to fail on its first call, assuming the guard does a pre-lock probe. It does not — the pre-lock warm-reuse probe lives in `ConnectOrStartDaemon` (outside the guard), so the guard's first `connect` call IS the post-lock double-check. The mis-modeled test asserted 0 spawns but the seam forced 1.
- **Fix:** Reworked the test's connect seam to represent the peer-already-up state (double-check succeeds on its only call) and added an assertion that the double-check probe actually ran (`calls >= 1`).
- **Files modified:** internal/forwarder/dial_race_test.go
- **Verification:** All 5 race/lockfile tests pass, including under `-race`.
- **Committed in:** `7f20a874` (Task 2 GREEN commit)

**2. [Rule 3 - Blocking] Reworded an in-code comment to satisfy the portability gate**
- **Found during:** Task 2 acceptance verification
- **Issue:** The acceptance gate `grep -c 'syscall.Flock' ... == 0` flagged a comment in dial.go that literally said "never syscall.Flock", a false positive (no actual call).
- **Fix:** Reworded the comment to "never the non-portable raw syscall lock" — the gate now reads 0 in both files while keeping the intent documented.
- **Files modified:** internal/forwarder/dial.go
- **Verification:** `grep -c 'syscall.Flock' internal/forwarder/dial.go internal/forwarder/dial_lock.go` is 0; `go build ./...` green.
- **Committed in:** `7f20a874` (Task 2 GREEN commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking)
**Impact on plan:** Both were corrections to the test/comment around the planned implementation; the production algorithm matches the plan exactly. No scope creep.

## Issues Encountered

- **Pre-existing `go mod tidy` failure (out of scope):** `go mod tidy` and `go mod verify` report problems with `github.com/google/s2a-go` (a transitive sigstore dependency: "module found but does not contain package" / "dir has been modified" in the local module cache). This is unrelated to flock or the forwarder and predates this plan. Per the executor scope boundary it was NOT fixed; `go build ./...` is green, proving the module graph used by the build is sound. Logged here for the verifier.

## Human Approvals

- **Task 1 supply-chain gate (threat T-90-SC):** `github.com/gofrs/flock@v0.13.0` provenance was human-approved before install — official `gofrs` org, `refs/tags/v0.13.0` released 2025-10-09, origin https://github.com/gofrs/flock, BSD-3-Clause, no CGO. go.sum pins the hash pair.

## Verification Results

- `go vet ./internal/forwarder/...` — clean.
- `go vet ./...` — clean (excluding the unrelated s2a-go cache note above).
- `go test ./internal/forwarder/ -run Race -count=1` — PASS (synctest: exactly 1 spawn under N=16 goroutines).
- `go test ./internal/forwarder/ -run Race -count=1 -race` — PASS (race detector clean).
- `go test ./internal/forwarder/...` (full package) — PASS.
- `go build ./...` — succeeds (Windows-portable: no raw syscall lock).
- `git diff --exit-code api/proto/` — empty (zero-proto invariant held).
- Acceptance greps: `flock.New` count 1 (>=1); `tryConnect` count 6 (>=3); `syscall.Flock` count 0; exported signature count 1; direct `gofrs/flock v0.13.0` require count 1; go.sum hash count 2.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- The race-free dial spine is in place; plans 90-02/03 (one-shot verb command + client-side transport bridge) and 90-04 (HELIX_BIN-gated real-PID E2E oracle) can build on `ConnectOrStartDaemon` unchanged.
- The synctest test pins the in-process algorithm; the real-subprocess exactly-one-PID assertion remains deferred to plan 90-04 by design (RESEARCH Pitfall 4).
- No blockers. The unrelated `go mod tidy`/`go mod verify` s2a-go noise should be watched if a future plan needs a clean `go mod tidy`.

---
*Phase: 90-cli-one-shot-dial-spine-race-free-warm-reuse*
*Completed: 2026-06-21*

## Self-Check: PASSED

- Files: FOUND internal/forwarder/dial.go, FOUND internal/forwarder/dial_lock.go, FOUND internal/forwarder/dial_race_test.go
- Commits: FOUND fcc56e4d, FOUND 435ff0d9, FOUND 7f20a874
