---
phase: 90-cli-one-shot-dial-spine-race-free-warm-reuse
reviewed: 2026-06-21T00:00:00Z
depth: standard
files_reviewed: 11
files_reviewed_list:
  - internal/cli/cli_e2e_test.go
  - internal/cli/root.go
  - internal/cli/root_test.go
  - internal/cli/verb.go
  - internal/cli/verb_test.go
  - internal/forwarder/dial.go
  - internal/forwarder/dial_lock.go
  - internal/forwarder/dial_race_test.go
  - internal/forwarder/grpc_client_transport.go
  - internal/forwarder/grpc_client_transport_test.go
  - internal/forwarder/oneshot.go
findings:
  critical: 0
  warning: 6
  info: 5
  total: 11
status: issues_found
---

# Phase 90: Code Review Report

**Reviewed:** 2026-06-21
**Depth:** standard
**Files Reviewed:** 11
**Status:** issues_found

## Summary

Reviewed the v2.0 one-shot CLI dial spine: the `helix call <verb>` cobra
wiring (`internal/cli/verb.go`, `root.go`), the race-free daemon-spawn guard
(`internal/forwarder/dial.go`, `dial_lock.go`), the client-side gRPC↔MCP-SDK
transport (`grpc_client_transport.go`), the one-shot helper
(`oneshot.go`), and the test/E2E suite.

The design is sound and the race algorithm (TryLock → Lock-wait → double-check
→ spawn) is correct and well-tested under synctest. No Critical defects found:
no injection, no secrets, no data-loss path, no crash-on-nil. However there are
two genuine **resource-leak** defects on the daemon cold-start path
(`*grpc.ClientConn` handles discarded without `Close()`), a **clean-shutdown
gap** in the one-shot teardown that the code's own comment claims to fix but
does not (no `CloseSend`, so the daemon records `outcome="error"` per CLI
call — the exact failure mode `oneshot.go`'s doc comment says it prevents), and
an **error-swallowing** `TryLock` that misclassifies a real lock error as
"lock contended". The remaining findings are robustness/idiom issues.

No structural findings block was provided for this phase.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: Cold-start leaks the double-check probe's gRPC connection

**File:** `internal/forwarder/dial.go:53`
**Issue:** The `connect` seam passed into `startupGuard` discards the
`*grpc.ClientConn` that `tryConnect` returns on success:

```go
connect: func(sp string) error { _, _, e := tryConnect(ctx, sp, tp); return e },
```

When the post-lock double-check probe (`startupGuard` line 93) succeeds — the
common TOCTOU-win / lock-loser-reuse case — `tryConnect` has already built a
live `grpc.NewClient` conn (dial.go:84) and returns it as the discarded first
value. `startupGuard` then returns nil, and `ConnectOrStartDaemon` builds a
*third* fresh conn at dial.go:62 for the real handles. The double-check conn is
never `Close()`d, leaking its resolver/keepalive goroutines and the underlying
FD for the life of the process. On the hot warm-reuse path the pre-lock probe
(dial.go:38) returns early so this leak only fires on the cold/contended path,
but every lock-loser and every TOCTOU peer leaks one conn per call.

**Fix:** Have the `connect` seam close the conn it opens, or make `tryConnect`
not leave a conn open on the probe-only path:
```go
connect: func(sp string) error {
	c, _, e := tryConnect(ctx, sp, tp)
	if c != nil {
		_ = c.Close()
	}
	return e
},
```

### WR-02: Cold-start leaks the waitForDaemon connection

**File:** `internal/forwarder/dial.go:55` (and `waitForDaemon` at dial.go:136-156)
**Issue:** Same class as WR-01. The `waitUp` seam discards the conn:

```go
waitUp: func(sp string) error { _, _, e := waitForDaemon(ctx, sp, 10*time.Second, tp); return e },
```

`waitForDaemon` returns a fully-built `*grpc.ClientConn` on success
(dial.go:150) which the closure throws away. After `startupGuard` returns,
`ConnectOrStartDaemon` opens yet another conn at dial.go:62. So every actual
daemon spawn leaks one conn. `waitForDaemon` returning client+conn it expects
the caller to use is itself a footgun given the only two callers both discard
them.

**Fix:** Close the conn in the `waitUp` closure (mirror WR-01), or simplify
`waitForDaemon` to return only an error (it polls for readiness; the live conn
is always rebuilt by the caller anyway):
```go
waitUp: func(sp string) error {
	c, _, e := waitForDaemon(ctx, sp, 10*time.Second, tp)
	if c != nil {
		_ = c.Close()
	}
	return e
},
```

### WR-03: One-shot teardown never CloseSends — daemon records outcome="error" per call

**File:** `internal/forwarder/oneshot.go:43,62-63` (vs `internal/forwarder/forwarder.go:95`)
**Issue:** `CallTool`'s doc comment (oneshot.go:25-28) explicitly states the
ordered teardown exists so "the daemon records the session with an `error`
outcome" does NOT happen. But the implementation does not achieve a clean
half-close. The teardown is only:
```go
defer conn.Close()      // line 43 (runs last)
...
defer session.Close()   // line 63 (runs first)
```
`session.Close()` → `ioConn.Close()` → closes the local io.Pipe ends
(`clientReader`/`serverWriter`); it does **not** call `stream.CloseSend()` on
the gRPC bidi stream. The recv pump goroutine
(`grpc_client_transport.go:64-81`) only unblocks when `conn.Close()` *aborts*
the stream, which the daemon's `stream.Recv()` (daemon.go:1444/1490) observes
as a non-EOF RST error → `session.Wait()` returns non-nil →
`SessionLifecycleInc("error", "stdio")` (daemon.go:1465). This is exactly the
metric inflation the comment claims to prevent. The long-lived forwarder gets
this right by calling `stream.CloseSend()` (forwarder.go:95) before the conn
drops; the one-shot path omits the equivalent.

**Fix:** Issue a graceful half-close before dropping the conn. Expose
`CloseSend()` on `GRPCClientStream` (the concrete `BidiStreamingClient` already
has it) and call it after `session.Close()` flushes, e.g.:
```go
defer func() {
	session.Close()
	_ = stream.CloseSend() // signal clean EOF so daemon records "ended", not "error"
}()
```
Then verify the daemon emits `helix_session_lifecycle{outcome="ended"}` for a
one-shot call (the E2E oracle does not currently assert this — see WR-06).

### WR-04: TryLock error is silently swallowed and reclassified as contention

**File:** `internal/forwarder/dial_lock.go:81-88`
**Issue:** `startupGuard` discards the error from `TryLock`:
```go
locked, _ := locker.TryLock()
if !locked {
	if err := locker.Lock(); err != nil {
		return err
	}
}
```
A genuine `TryLock` failure (e.g. the lockfile's directory is gone, EACCES, or
an I/O error from `gofrs/flock`) returns `locked=false, err!=nil`. The code
ignores `err` and falls into the blocking `Lock()`, treating an environmental
failure as ordinary lock contention. If `Lock()` also fails the original cause
is lost; if `Lock()` somehow blocks, the caller hangs instead of surfacing the
real error. This defeats the diagnostics for the one scenario where the startup
guard misbehaves.

**Fix:** Propagate the TryLock error:
```go
locked, err := locker.TryLock()
if err != nil {
	return fmt.Errorf("startup lock tryLock: %w", err)
}
if !locked {
	if err := locker.Lock(); err != nil {
		return fmt.Errorf("startup lock: %w", err)
	}
}
```

### WR-05: startDaemon redirects child stdout/stderr to the parent's fds (nil), defeating detach

**File:** `internal/forwarder/dial.go:122-126,132`
**Issue:** `cmd.Stdout = nil` / `cmd.Stderr = nil` does NOT discard the
auto-started daemon's output. Per `os/exec`, a nil Stdout/Stderr means the
child inherits... actually it means the child's fd is connected to
`/dev/null`-equivalent only if unset — but here the parent is a short-lived CLI
process (`helix call ...`) that exits immediately after `cmd.Process.Release()`.
With nil fds, `exec` opens `os.DevNull` for the child, which is the intended
behavior — so output is discarded correctly. The real risk is ordering:
`cmd.Process.Release()` (line 132) is called, but if the daemon writes a startup
error to stderr it is lost, making a failed cold-start (e.g. bad config,
unwritable socket dir) invisible — the only signal is the 10s `waitForDaemon`
timeout with no captured cause. For a daemon that "silently dies" this is a
debuggability gap on the exact failure path the `--http-addr=` comment
(dial.go:116-121) was added to fix.

**Fix:** Redirect the spawned daemon's stderr to a rotating log file (or the
per-uid state dir) rather than discarding it, so cold-start failures are
diagnosable:
```go
logFile, _ := os.OpenFile(socketPath+".daemon.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
cmd.Stderr = logFile
```
At minimum, document that all auto-start daemon diagnostics are discarded.

### WR-06: E2E oracle asserts only daemon count, never the clean-shutdown contract (WR-03)

**File:** `internal/cli/cli_e2e_test.go:318-385`
**Issue:** `TestCLI_ParallelColdSingleDaemon` proves "exactly one daemon", and
`TestCLI_E2E_OneShot` proves result parity, but nothing in the suite asserts
the session-lifecycle outcome of a one-shot call. Because WR-03 makes every
`helix call` abort its stream, the daemon's `helix_session_lifecycle` counter
climbs with `outcome="error"` and no test catches it. The oracle's own header
(cli_e2e_test.go:25-28 region of `oneshot.go`) treats clean shutdown as a
contract, yet the contract is untested. This is a test-coverage gap that lets a
real regression (WR-03) ship green.

**Fix:** After a one-shot call, scrape the daemon's admin/metrics endpoint (or
read the lifecycle counter via the existing obs surface) and assert
`outcome="ended"` incremented and `outcome="error"` did not. Gate it the same
way as the other HELIX_BIN sub-tests.

## Info

### IN-01: `var _ = time.Second` import-guard hack left in committed test

**File:** `internal/forwarder/dial_race_test.go:218-219`
**Issue:** `// guard against unused import when iterating` + `var _ = time.Second`
is scaffolding left from development. The `time` import is not otherwise used in
this file; the guard should be removed along with the import.
**Fix:** Delete lines 218-219 and the `"time"` import (line 9) if unused.

### IN-02: tryConnect ignores the os.Stat error class

**File:** `internal/forwarder/dial.go:73-75`
**Issue:** Only `os.IsNotExist(err)` is special-cased; a stat error for any
other reason (EACCES, ENOTDIR on a malformed path) falls through to
`net.DialTimeout`, masking the real cause behind a generic "socket not
responding". Minor, since the dial will also fail, but the error is less
precise than it could be.
**Fix:** Return a wrapped stat error for non-NotExist cases.

### IN-03: defensive payload copy asymmetry between client and server drain loops

**File:** `internal/forwarder/grpc_client_transport.go:108-111` vs `internal/mcp/grpc_transport.go:106-109`
**Issue:** The client drain copies the line before Send
(`append([]byte(nil), line...)`), the server drain does not. The client copy is
the correct one (the slice aliases the reusable `buf`), so the server side
(`internal/mcp/grpc_transport.go`) is the latent bug — but that file is out of
this phase's change set. Noting for cross-reference: the two "mirror" transports
have diverged on a memory-safety detail. The client side here is correct.
**Fix:** None required in phase-90 files; track the server-side copy gap
separately (it relies on gRPC marshalling the payload synchronously before the
buffer is reused).

### IN-04: representativeVerb is a single-entry map iterated for a one-element loop

**File:** `internal/cli/verb.go:68-86,102-105`
**Issue:** `verbSpecs` is a one-entry map and `newVerbCommand` ranges over it.
Map iteration order is irrelevant for one element, but once Phase 91 populates
the full set, ranging a map gives nondeterministic subcommand registration
order (cobra sorts for help, so cosmetic only). Flagged now so the Phase 91
generator sorts keys for deterministic command construction.
**Fix:** When Phase 91 lands, iterate sorted keys: `for _, verb := range
slices.Sorted(maps.Keys(verbSpecs))`.

### IN-05: resolveVerbSocket checks the verb's own --socket flag that is never defined

**File:** `internal/cli/verb.go:204-209`
**Issue:** `cmd.Flags().Lookup("socket")` on the verb subcommand will only find
a flag if the verb defines one, which `newVerbSubcommand` does not — `--socket`
lives on the root command. The first branch (lines 205-209) is therefore dead
for the verb path; only the `cmd.Root().Flags()` branch (lines 210-214) can
match. Harmless (the root branch covers it), but the local-flag branch is
misleading dead code suggesting per-verb socket override exists when it does
not.
**Fix:** Drop the local-flag lookup, or wire an inherited persistent `--socket`
flag so the lookup is meaningful.

---

_Reviewed: 2026-06-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
