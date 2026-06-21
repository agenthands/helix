---
phase: 94-retire-the-agent-facing-mcp-surface-delete
fixed_at: 2026-06-22T00:00:00Z
review_path: .planning/phases/94-retire-the-agent-facing-mcp-surface-delete/94-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
deferred: 2
status: all_fixed
---

# Phase 94: Code Review Fix Report

**Source review:** 94-REVIEW.md
**Scope:** Critical + Warning (the 2 Info findings were intentionally left out of scope)
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (CR-01, WR-01, WR-02)
- Fixed: 3
- Skipped: 0
- Deferred (out of scope by request): 2 (IN-01, IN-02)

## Fixed Issues

### CR-01 (BLOCKER): `validateGRPCAddr` allows wildcard (`:port`) bind — full MCP tool surface exposed on all interfaces

**Files modified:** `internal/daemon/grpc_tcp.go`, `internal/daemon/grpc_tcp_test.go`
**Commit:** `6361b1f4`

**Applied fix:** Removed `""` from the loopback allowlist in `validateGRPCAddr`.
An empty host (`:9099` / `:0`) now hits an explicit `case ""` branch that
REFUSES it with a REMOTE-01-pointing error, because `net.Listen("tcp", ":9099")`
binds ALL interfaces (0.0.0.0 + ::) and would expose the full
`ForwarderService.StreamMCP` MCP tool surface (read/edit/refactor tools behind
all middlewares) plaintext + unauthenticated on the LAN. Only explicit loopback
hosts (`127.0.0.1`, `::1`, `localhost`) pass. Added an `!ip.IsUnspecified()`
guard on the `net.ParseIP` branch as defense-in-depth against `0.0.0.0` / `::`
should the explicit branch ever be reordered. The empty-addr (disabled) no-op is
preserved, so the default (`--grpc-addr` unset) still binds nothing.

Added table-driven test cases: `:9099` REFUSED, `:0` REFUSED, `[::]:9099`
REFUSED (alongside the pre-existing `0.0.0.0:9099`, `192.168.1.5:9099` REFUSED
and `127.0.0.1:9099`, `[::1]:9099`, `127.0.0.1:0` allowed). Empirical gate
confirmed green: `:9099` and `0.0.0.0:9099` REFUSED, `127.0.0.1:9099` accepted.

**Admin-addr sibling note (per constraints):** `validateAdminAddr`
(`internal/daemon/telemetry.go:108`) has the SAME empty-host gap. It was
deliberately NOT changed here to keep this fix scoped to the security-sensitive
gRPC bind. The admin path only exposes instrumentation endpoints (`/healthz`,
`/readyz`, `/metrics`, opt-in `pprof`), so the exposure is materially less
severe. The two `validate*Addr` helpers were left as separate functions rather
than collapsed into a shared corrected helper, again to keep the change scoped;
the admin gap is documented in the `validateGRPCAddr` doc comment and is a
recommended follow-up (out of Phase 94 scope).

### WR-01: gRPC TCP bind failure is FATAL to the whole daemon (kills the working unix socket)

**Files modified:** `internal/daemon/daemon.go`
**Commit:** `9531777c`

**Applied fix:** Mirrored `listenAdmin`'s resilience pattern (the chosen
approach, consistent with how the daemon already handles its other optional
listener). The `g.Go` closure for `listenGRPCTCP` now logs and swallows any
non-`context.Canceled` error and returns `nil`, so a transient bind error
(EADDRINUSE / TIME_WAIT collision on restart) on the OPTIONAL TCP listener no
longer cancels `gctx` and tears down the kernel + working unix-socket transport.
Because validation is now fatal-early at config-load (WR-02), a non-nil return
from `listenGRPCTCP` at runtime can only be a transport/bind failure worth
degrading on — never a misconfiguration worth refusing to start for.

### WR-02: `--grpc-addr` is never validated at config-load time — only inside the listener goroutine

**Files modified:** `internal/cli/root.go`, `internal/daemon/grpc_tcp.go`
**Commit:** `9531777c`

**Applied fix:** Added an exported `daemon.ValidateGRPCAddr` thin wrapper over
the unexported `validateGRPCAddr`, and called it in `runDaemon` immediately after
`config.Load` (on the RESOLVED config value, i.e. CLI override + project/user
config) and before `daemon.New`. A misconfigured insecure bind is now refused
with a clear early error (`invalid daemon.grpc_addr: ...`) before any subsystem
starts, instead of an opaque late "daemon exited". The listener still
re-validates internally (defense in depth). This is what makes WR-01's
"validation fatal-early / transient bind non-fatal" split natural and gives the
operator an actionable message.

## Deferred Issues (out of requested scope)

### IN-01: `grpcTCPListenerAddr` global documented "TEST-ONLY" but written on every production bind

**File:** `internal/daemon/grpc_tcp.go:28-32, 105-107`
**Reason:** Info-tier; reviewer marked "No action required" (correctly cleared on
exit, not a leak/race, mirrors `adminListenerAddr`). Left per scope (Critical +
Warning only).

### IN-02: `drive.go` retains the now-unused `helixBin` parameter

**File:** `bench/runtime/drive.go:67-68`
**Reason:** Info-tier; intentional ("retained for signature stability"),
documented. Reviewer marked "None needed now". Left per scope.

## Verification

- `gofmt -l` on all touched files: clean
- `go vet ./...`: clean
- `go build ./...` and `go build -tags integration ./...`: OK
- `go test ./internal/daemon/... ./internal/forwarder/... ./internal/cli/... -count=1`: green
- Empirical gate: `TestValidateGRPCAddr` confirms `:9099`, `:0`, `[::]:9099`,
  `0.0.0.0:9099` REFUSED; `127.0.0.1:9099`, `[::1]:9099`, `127.0.0.1:0` accepted
- Invariants: `git diff go.mod go.sum` empty; `git diff api/proto/` empty
- Default behavior preserved: empty `--grpc-addr` remains unix-socket only

---

_Fixed: 2026-06-22_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
