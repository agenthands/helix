---
phase: 90-cli-one-shot-dial-spine-race-free-warm-reuse
fixed_at: 2026-06-21
review_path: 90-REVIEW.md
iteration: 1
findings_in_scope: 8
fixed: 8
skipped: 0
deferred: 3
status: all_fixed
---

# Phase 90: Code Review Fix Report

**Fixed at:** 2026-06-21
**Source review:** 90-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope (all WARNINGs + trivial-safe INFOs): 8
- Fixed: 8 (WR-01..WR-06, IN-01, IN-05)
- Deferred (out of scope / explicitly future-phase): 3 (IN-02, IN-03, IN-04)
- Skipped (failed/rolled back): 0

## Fixed Issues

### WR-01 / WR-02: Cold-start gRPC connection leaks
**Files modified:** `internal/forwarder/dial.go`
**Commit:** b3e5fda7
**Applied fix:** The `connect` (double-check) and `waitUp` seams discarded the
live `*grpc.ClientConn` that `tryConnect` / `waitForDaemon` return on success,
leaking resolver+keepalive goroutines and an FD on every lock-loser, TOCTOU
peer, and daemon spawn. Both seam closures now `Close()` the probe conn; the
single real conn is re-dialed by `ConnectOrStartDaemon` after the guard returns.
The warm fast path (pre-lock probe) is untouched.
Note: `waitForDaemon` returns `(client, conn, err)` — the conn is the SECOND
value (unlike `tryConnect`, conn-first); the fix accounts for that ordering.

### WR-03: One-shot teardown never CloseSends
**Files modified:** `internal/forwarder/oneshot.go`
**Commit:** d6b13e49
**Applied fix:** Teardown now performs `session.Close()` (flush) then
`stream.CloseSend()` (half-close) before `conn.Close()` (runs last), so the
daemon's `stream.Recv()` observes a clean `io.EOF` and records
`outcome="ended"` rather than the non-EOF RST abort that produced
`outcome="error"` per CLI call. Mirrors the long-lived forwarder
(`forwarder.go:95`). Behavioral fix — verified at runtime by the WR-06 oracle.

### WR-04: TryLock error swallowed as contention
**Files modified:** `internal/forwarder/dial_lock.go`
**Commit:** ca26f285
**Applied fix:** A genuine `TryLock` error (lockfile dir gone, EACCES, flock
I/O error) is now surfaced (`fmt.Errorf("startup lock tryLock: %w", err)`)
instead of being misclassified as ordinary contention. The contention path
(`locked==false, err==nil`) still blocks on `Lock()`, now also wrapped.

### WR-05: Auto-started daemon stderr discarded
**Files modified:** `internal/forwarder/dial.go`
**Commit:** 60783c65
**Applied fix:** The spawned daemon's stdout/stderr are now redirected to
`<socket>.daemon.log` (in the existing per-uid 0700 socket dir, inheriting its
permissions) so cold-start failures are diagnosable instead of surfacing only
as an opaque 10s `waitForDaemon` timeout. Best-effort: falls back to discarding
output if the log file cannot be opened, so it never fails the spawn.

### WR-06: E2E oracle never asserted the clean-shutdown contract
**Files modified:** `internal/cli/cli_e2e_test.go`, `internal/eval/sandbox/sandbox.go`
**Commit:** fab67c63
**Applied fix:** Added `TestCLI_E2E_OneShotCleanShutdown`. It enables the daemon
admin listener via a new additive `sandbox.WithAdminAddr` DaemonOption (plus a
`DaemonHandle.AdminAddr()` accessor), scrapes
`helix_session_lifecycle_total{transport="stdio"}` before/after one real
`helix call` subprocess, and asserts `outcome="ended"` increments while
`outcome="error"` does not — locking in WR-03. Gated on `HELIX_BIN` like the
other E2E sub-tests. Confirmed RAN and PASSED (not skipped).

### IN-01: Leftover `var _ = time.Second` import-guard hack
**Files modified:** `internal/forwarder/dial_race_test.go`
**Commit:** 205622fd
**Applied fix:** Removed the scaffolding line and the now-unused `time` import.

### IN-05: Dead per-verb `--socket` lookup
**Files modified:** `internal/cli/verb.go`
**Commit:** cd2d856e
**Applied fix:** Dropped the verb-local `Flags().Lookup("socket")` branch that
could never match (the flag lives on the root command); `resolveVerbSocket` now
reads the inherited root `--socket` flag directly.

## Deferred Issues

### IN-02: tryConnect ignores the os.Stat error class
**Reason:** Cosmetic error-precision nit; the dial fails regardless and the
message is only marginally less precise. Advisory, out of the fix scope.

### IN-03: payload-copy asymmetry between client/server drain loops
**Reason:** The finding itself states no change is required in phase-90 files —
the client side is correct; the server-side gap is out of this phase's change
set. Tracked for separate follow-up.

### IN-04: single-entry verbSpecs map iterated for a one-element loop
**Reason:** The finding explicitly defers the deterministic-iteration fix to
Phase 91 (when the full verb set lands). No action in phase 90.

## Verification

- Zero-proto invariant: `git diff --exit-code api/proto/` clean (PROTO_UNCHANGED_OK).
- `go vet ./...`: exit 0 (only pre-existing unrelated swift CGO `TOKEN_COUNT`
  redefinition warnings, not errors).
- `go build ./...`: exit 0.
- `go test ./internal/forwarder/... ./internal/cli/...`: PASS.
- `go test ./internal/eval/sandbox/...`: PASS.
- Gated E2E (`go build -o ./helix ./cmd/helix && HELIX_BIN="$(pwd)/helix" go
  test ./internal/cli/... -run 'CLI_E2E|CLI_WarmReuseSLO|CLI_ParallelColdSingleDaemon|OneShot|Shutdown|Clean' -count=1 -v`):
  all RAN and PASSED, including the new `TestCLI_E2E_OneShotCleanShutdown`.

---

_Fixed: 2026-06-21_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
