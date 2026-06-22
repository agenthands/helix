---
phase: 94-retire-the-agent-facing-mcp-surface-delete
reviewed: 2026-06-21T22:36:00Z
depth: standard
files_reviewed: 6
files_reviewed_list:
  - internal/daemon/grpc_tcp.go
  - internal/forwarder/session.go
  - internal/forwarder/dial.go
  - internal/daemon/daemon.go
  - internal/cli/root.go
  - bench/runtime/drive.go
findings:
  critical: 1
  warning: 2
  info: 2
  total: 5
status: issues_found
---

# Phase 94: Code Review Report

**Reviewed:** 2026-06-21T22:36:00Z
**Depth:** standard
**Files Reviewed:** 6
**Status:** issues_found

## Summary

Phase 94 retires the two agent-facing MCP heads (stdio forwarder + HTTP `/mcp`)
and replaces them with (a) a gated, loopback-only gRPC TCP bind on the daemon and
(b) an internal in-process `forwarder.Session` driver for bench/eval. The head
deletion is clean: no live `--http-addr`/`HTTPAddr`/`Streamable`/`/mcp` serving
code or config remains in `internal/cli` or `internal/daemon` (the
`http_session_middleware*.go` files are deleted), and `RunForwarder`/`runForwarderFn`
are gone. The new `Session` driver does NOT reintroduce any agent-facing MCP
server path — it is an in-process Go API that dials the retained gRPC
`StreamMCP` wire, so SC2 holds. Teardown ordering (session.Close → CloseSend →
conn.Close) is correct and matches the documented `oneshot.go` invariant. Session
IDs use `crypto/rand` (no insecure RNG). The `--http-addr` exec-arg removal from
`startDaemon` is complete and does not break auto-start (unix-socket path intact).
`drive.go` preserves timing-faithful per-step `AtTime` dispatch instants.

The prompt's premise that `helix --mode=stdio` "exits 0 for an unknown mode" is
**not accurate against the built binary**: `runRoot` returns an error,
`cmd.Execute()` surfaces it, and `main.go` calls `os.Exit(cli.ExitCodeForError(err))`
which yields exit code 1 (verified by building and running). No finding needed
there — the behavior is already correct.

The one serious issue is the loopback gate: `validateGRPCAddr` accepts an empty
host (`:9099`), which `net.Listen("tcp", addr)` binds to **all interfaces**,
defeating the loopback-only guarantee for the most sensitive surface in the repo
(the full MCP tool registry behind all 6 middlewares — read/edit/refactor tools),
plaintext and unauthenticated.

## Critical Issues

### CR-01: `validateGRPCAddr` allows wildcard (`:port`) bind — full MCP tool surface exposed on all interfaces

**File:** `internal/daemon/grpc_tcp.go:65-81` (gate); `internal/daemon/grpc_tcp.go:99` (bind)

**Issue:** The loopback gate special-cases an empty host as valid:

```go
switch host {
case "", "localhost", "127.0.0.1", "::1":
    return nil
}
```

But the validated string is passed verbatim to `net.Listen("tcp", addr)` at
line 99. An address like `:9099` (or `:0`) splits to `host == ""`, passes the
gate, and then binds to **every** interface — including non-loopback ones. This
directly violates the RETIRE-04 security contract ("refuse non-loopback
addresses … no unauthenticated non-loopback exposure by default").

STRIDE impact is severe specifically for THIS listener: unlike the admin
listener (which mirrors the same gate bug but only exposes `/healthz`,
`/readyz`, `/metrics`, and opt-in `pprof`), `listenGRPCTCP` registers the SAME
`ForwarderService.StreamMCP` handler that dispatches into `mcpServer.SDK()` with
all 6 middlewares — i.e. the entire read/edit/refactor MCP tool catalogue. The
transport is plaintext (`insecure.NewCredentials()` on the dial side; no TLS,
no auth on the bind side). So `helix --serve --grpc-addr=:9099` silently exposes
**Spoofing/Tampering/Information-Disclosure/Elevation** of the full code-editing
tool surface to anyone who can route to the host on port 9099. This is the exact
"no unauthenticated non-loopback exposure by default" failure RETIRE-04 was
meant to prevent.

Note: the empty-host gap is a faithful copy of the pre-existing
`validateAdminAddr` bug (`telemetry.go:108`), but it is materially worse here
because the exposed surface is the full MCP runtime, not instrumentation
endpoints. It must be closed on the gRPC path regardless of the admin
precedent (and the admin path should be fixed too).

**Fix:** Reject the empty/wildcard host explicitly. `net.IP.IsLoopback()` is
already the source of truth — just stop treating `""` as loopback, and reject
wildcard IPs:

```go
func validateGRPCAddr(addr string) error {
	if addr == "" {
		return nil // disabled is valid
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid grpc addr %q: %w", addr, err)
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return nil
	case "":
		// ":9099" / ":0" → net.Listen binds ALL interfaces. Refuse.
		return fmt.Errorf("grpc addr must name a loopback host, got wildcard %q "+
			"(use 127.0.0.1:PORT or [::1]:PORT; non-loopback bind deferred to REMOTE-01)", addr)
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() && !ip.IsUnspecified() {
		return nil
	}
	return fmt.Errorf("grpc addr must be loopback, got %q (non-loopback gRPC bind is deferred to REMOTE-01)", host)
}
```

(`IsUnspecified()` also defends against `0.0.0.0`/`::` should the loopback
branch ever be reordered; `0.0.0.0` is already rejected today because it is not
loopback, but the explicit guard documents intent.)

## Warnings

### WR-01: gRPC TCP bind failure is FATAL to the whole daemon (kills the working unix socket)

**File:** `internal/daemon/daemon.go:1325-1329`; `internal/daemon/grpc_tcp.go:99-102`

**Issue:** `listenGRPCTCP` is launched into the top-level `errgroup`, and any
non-nil return (including a transient `net.Listen` failure such as
`EADDRINUSE` when port 9099 is momentarily occupied) propagates through
`g.Wait()`, cancels `gctx`, and tears down the entire daemon — kernel, the
unix-socket listener, semantic pipelines, everything. By contrast the admin
listener (`daemon.go:1333-1340`) deliberately swallows its error
("NEVER propagate — observability is instrumentation, not product"). A
split-host operator who fat-fingers the port, or hits a TIME_WAIT collision on
restart, loses the working local unix-socket transport too.

A non-loopback **validation** error being fatal is defensible (refuse to start
with a misconfigured insecure bind). A transient **bind** error killing the
already-functional unix path is a robustness regression.

**Fix:** Distinguish the validation error (fatal — fail fast) from the
listen/serve error (non-fatal — log and continue on the unix socket), or at
minimum document and intend the fatal-for-all behavior. Example:

```go
if d.config.Daemon.GRPCAddr != "" {
	// Validate up-front so a misconfigured insecure bind fails fast.
	if err := validateGRPCAddr(d.config.Daemon.GRPCAddr); err != nil {
		return fmt.Errorf("grpc tcp listener: %w", err)
	}
	g.Go(func() error {
		if err := d.listenGRPCTCP(gctx); err != nil && !errors.Is(err, context.Canceled) {
			d.logger.Error("grpc tcp listener failed; continuing on unix socket",
				"error", err, "addr", d.config.Daemon.GRPCAddr)
		}
		return nil // do not tear down the working unix transport
	})
}
```

### WR-02: gRPC TCP address is never validated at config-load time — only when the listener goroutine runs

**File:** `internal/cli/root.go:261-263`; `internal/daemon/grpc_tcp.go:96`

**Issue:** A non-loopback `--grpc-addr` / `daemon.grpc_addr` is accepted by the
CLI and by `config.Load`; the refusal happens only later inside the listener
goroutine in `listenGRPCTCP`. Combined with WR-01's fatal propagation, the
operator-visible failure is an opaque late "daemon exited" rather than an
immediate "that address is not allowed". The `--admin-addr` flag has the same
late-validation shape, so this is a consistency note rather than a novel defect,
but the security sensitivity of this surface (CR-01) raises the bar for
failing loud and early.

**Fix:** Validate at the composition root (e.g. in `runDaemon` after reading
`grpc-addr`, or in `config.Load`) so a bad address is rejected before any
subsystem starts. This also makes WR-01's "validation fatal / bind non-fatal"
split natural.

## Info

### IN-01: `grpcTCPListenerAddr` global is documented "TEST-ONLY" but is written on every production bind

**File:** `internal/daemon/grpc_tcp.go:28-32, 105-107`

**Issue:** The package-level `atomic.Pointer[string]` is described as a
"TEST-ONLY hook," but `listenGRPCTCP` stores the bound address into it
unconditionally — including in production when `--grpc-addr` is set. It is
correctly cleared on exit via `defer`, so it is neither a leak nor a race, and
it mirrors the existing `adminListenerAddr` pattern. No action required; flagged
only so the "TEST-ONLY" comment is not mistaken for "never runs in production."

**Fix:** None needed; optionally soften the comment to "test observation hook;
harmless in production."

### IN-02: `drive.go` retains the now-unused `helixBin` parameter

**File:** `bench/runtime/drive.go:67-68`

**Issue:** `helixBin` is no longer used (the driver dials the gRPC wire
in-process rather than spawning a forwarder binary) and is silenced with
`_ = helixBin`. This is intentional ("retained for signature stability") and
documented, so it is acceptable. Flagged only so a future cleanup pass knows the
parameter is dead and can be dropped together with its call sites when signature
churn is acceptable.

**Fix:** None needed now; drop the parameter when the caller signature can
change.

---

_Reviewed: 2026-06-21T22:36:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
