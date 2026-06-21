# Phase 90: CLI One-Shot Dial Spine + Race-Free Warm Reuse - Research

**Researched:** 2026-06-21
**Domain:** Go CLI ↔ daemon IPC (cobra + gRPC bidi stream + MCP SDK client), cross-process startup locking, real-subprocess E2E testing, latency SLO measurement
**Confidence:** HIGH (all findings grounded in this repo's source at named file:line; the only external dependency is `gofrs/flock`, verified via `go list`)

## Summary

Phase 90 builds the "dial spine": a `helix <verb>` cobra command that issues a single MCP `tools/call` through the warm daemon over the existing gRPC `StreamMCP` bidi stream, auto-starting the daemon cold and reusing it warm — without changing `api/proto/` and without opening a persistent stdio MCP session. Almost every primitive this phase needs already exists in-tree and is being reused as-is per the locked constraints: `forwarder.ConnectOrStartDaemon` (auto-start + connect, `internal/forwarder/dial.go:24`), `client.StreamMCP(ctx)` (the bidi stream, `internal/forwarder/forwarder.go:51`), `mcp.GRPCTransport` (the daemon-side gRPC↔MCP-SDK bridge, `internal/mcp/grpc_transport.go`), and `internal/eval/sandbox` + `bench/runtime/subprocess` (the real-subprocess daemon harness). The MCP SDK client (`mcp.NewClient` / `session.CallTool`, used throughout `test/integration/`) is the right way to issue one-shot calls — it performs the `initialize` handshake and one `tools/call` and tears down, all over a client-side transport that wraps the gRPC stream.

The two genuinely new pieces of engineering are (1) a **cross-process startup lock** to fix the daemon-spawn race in `dial.go` (today `ConnectOrStartDaemon` has try-connect → spawn → poll with no lock, so N parallel cold callers each spawn a daemon), and (2) a **CLI-side client transport** mirroring the daemon's `GRPCTransport` so the MCP SDK client can run over `StreamMCP`. The lock should be an OS advisory file lock (`flock`) on a per-socket lockfile next to the socket — the idiomatic Go choice is `github.com/gofrs/flock` (cross-platform, `TryLock`/`Lock`), and the daemon's own `net.Listen("unix", …)` already provides a natural second line of defense (the second daemon's listen fails, `internal/daemon/daemon.go:1318`).

**Primary recommendation:** Add a thin `internal/cli` "verb dispatch" command (the spine) that calls a new `internal/forwarder` (or `internal/clirpc`) one-shot helper: `ConnectOrStartDaemon` → wrap `client.StreamMCP(ctx)` in a new client-side `GRPCClientTransport` → `mcp.NewClient(...).Connect()` → `session.CallTool(name, args)` → render → close. Fix the race inside `ConnectOrStartDaemon` (or a new `connectOrStartDaemonLocked`) with a `flock` guard around the spawn-and-wait window. Stand up the E2E oracle as a `//go:build !windows`, `HELIX_BIN`-gated test reusing `evalsandbox.Sandbox`/`StartDaemon`, exactly like `bench/runtime/daemon_tap_integration_test.go`.

## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Zero-proto invariant:** the one-shot `tools/call` rides the existing gRPC `StreamMCP` wire; `git diff api/proto/` must stay empty. `[CITED: 90-CONTEXT.md:43-45]`
- **Fix the cross-process daemon-spawn race** (`dial.go` has no lock today) and **set a 2nd-call latency SLO.** `[CITED: 90-CONTEXT.md:43-45]`
- **Reuse v1.0 forwarder primitives as-is:** `ConnectOrStartDaemon`, `StreamMCP`, `GRPCTransport`. `[CITED: 90-CONTEXT.md:46-47]`
- **Reuse `bench/runtime/subprocess` + `internal/eval/sandbox` patterns** for the E2E oracle. `[CITED: 90-CONTEXT.md:48-49]`

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions. `[CITED: 90-CONTEXT.md:37-40]`

### Deferred Ideas (OUT OF SCOPE)
None for this phase. Milestone-level out-of-scope (do not touch in Phase 90): excising the MCP SDK, removing the gRPC IPC layer, re-implementing middlewares, dual MCP+CLI head, named sessions, default-on JSON, a generic `helix run <tool>` mega-verb. `[CITED: REQUIREMENTS.md:86-98]`

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CLI-01 | `helix <verb>` invokes a single tool against the warm daemon via one-shot `tools/call` over existing gRPC `StreamMCP`, reusing `forwarder.ConnectOrStartDaemon`; `git diff api/proto/` empty. | Reuse `ConnectOrStartDaemon` (`dial.go:24`) + `client.StreamMCP` (`forwarder.go:51`); add a CLI-side `GRPCClientTransport` mirroring `internal/mcp/grpc_transport.go`; drive with `mcp.NewClient(...).CallTool` (pattern: `test/integration/harness.go:176-200`). No proto edit required — `StreamMCP` already carries arbitrary JSON-RPC payloads (`ipc.proto:11-12`). |
| CLI-02 | Auto-start cold, reuse warm; 2nd-call p50 below a phase-set SLO. | `ConnectOrStartDaemon` already does try-connect → `startDaemon` → `waitForDaemon` (`dial.go:26-39`). SLO measured by timing a warm second `session.CallTool` round-trip (see Validation Architecture → Sampling). |
| CLI-03 | Race-free auto-start under parallel first calls — cross-process startup lock prevents duplicate spawns. | `dial.go` has NO lock today (race confirmed below). Add `gofrs/flock` guard around the spawn+wait window; daemon's `net.Listen("unix")` (`daemon.go:1318`) is the backstop. Fan-out / synctest stress test asserts exactly one daemon PID. |
| CLI-04 | No-arg `helix` prints grouped usage/help and no longer launches a stdio MCP server. | Today `runRoot` falls through to `runForwarder` on no args (`root.go:98-118`, `112-114`). Phase 90 must change the no-arg path to print help (`cmd.Help()` / `RunE` returning usage) and exit 0. (Full forwarder-head deletion is Phase 94 — here, only the no-arg behavior changes.) |
| TEST-01 | CLI-over-daemon E2E oracle runs a representative verb as a subprocess against a real daemon, green under `go test`. | Reuse `evalsandbox.Sandbox` + `StartDaemon` (`internal/eval/sandbox/sandbox.go:315`) and the `HELIX_BIN`-gated, `//go:build !windows` pattern from `bench/runtime/daemon_tap_integration_test.go:24-64`. |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Verb parsing / arg → flag binding | CLI (cobra, `internal/cli`) | — | The "spine" lives in `internal/cli`; later phases (91) code-generate verbs on top of it. |
| Daemon auto-start + connect | CLI-invoked forwarder dial (`internal/forwarder`) | OS process mgmt | `ConnectOrStartDaemon` already owns this; reused as-is. |
| Cross-process startup lock | CLI-invoked forwarder dial (`internal/forwarder`) | OS (flock + unix-socket bind) | The race is in the dial path; the lock belongs next to the spawn it guards. |
| One-shot `tools/call` framing | CLI client (MCP SDK client + new `GRPCClientTransport`) | gRPC `StreamMCP` wire | The SDK client performs `initialize`+`tools/call`; the new transport bridges it onto the existing bidi stream. |
| Tool dispatch + middleware (profile/telemetry/lazy-init) | Daemon (unchanged) | MCP SDK inside daemon | Locked: SDK + 5 middlewares stay inside the daemon behind the wire. |
| E2E oracle (subprocess harness) | Test tier (`internal/eval/sandbox`, new `_test.go`) | OS process mgmt | Reuses the v1.12 sandbox; no production code. |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | v1.10.2 (in `go.mod`) | CLI command tree, grouped help (`AddGroup`), flag binding | Already the project's CLI framework (`internal/cli/root.go`). `[VERIFIED: go.mod]` |
| `github.com/modelcontextprotocol/go-sdk/mcp` | v1.5.0 (in `go.mod`) | MCP client (`NewClient`/`Connect`/`CallTool`) and `IOTransport` | Already used for daemon server + integration test clients (`test/integration/harness.go`). One-shot client reuses the exact `session.CallTool` API. `[VERIFIED: go.mod]` |
| `google.golang.org/grpc` | v1.80.0 (in `go.mod`) | `StreamMCP` bidi stream transport, `grpc.NewClient` | Existing IPC wire; reused unchanged. `[VERIFIED: go.mod]` |
| `github.com/agenthands/helix/api/proto/serena/v1` | in-tree | `ForwarderServiceClient`, `MCPMessage`, `StreamMCP` | The wire — MUST NOT change (zero-proto invariant). `[VERIFIED: ipc.proto]` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/gofrs/flock` | v0.13.0 | Cross-platform advisory file lock (`TryLock`/`Lock`/`Unlock`) for the startup mutex | The recommended cross-process startup lock; pure-Go, Windows+Unix, no CGO. `[VERIFIED: go list -m github.com/gofrs/flock@latest → v0.13.0]` |
| `testing/synctest` | stdlib (Go 1.25 GA) | Deterministic virtual-clock concurrency test for the fan-out race | Already used at `internal/kernel/lspool/pool_synctest_test.go:5`. Module is `go 1.25.1`, toolchain `go1.26.0` — synctest is GA, no `GOEXPERIMENT`. `[VERIFIED: go.mod go 1.25.1; go version go1.26.0]` |
| `github.com/agenthands/helix/internal/eval/sandbox` | in-tree | Per-cell isolated daemon subprocess (`StartDaemon`, `DaemonHandle.Pid()/Kill()`) | The E2E oracle's daemon lifecycle. `[VERIFIED: internal/eval/sandbox/sandbox.go:315]` |
| `github.com/stretchr/testify` | in `go.mod` | `require`/`assert` for the E2E + race tests | Project test convention. `[VERIFIED: existing tests]` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `gofrs/flock` (lockfile) | Unix-socket-bind-as-lock only (no flock) | The socket bind IS already a backstop (`net.Listen("unix")` fails for the 2nd daemon, `daemon.go:1318`), but it does not stop N callers from *spawning* N daemons — only N-1 of them fail to listen and exit. That is a connect-storm + wasted forks. A flock around the spawn window prevents the duplicate spawns entirely. Recommend flock as primary, socket-bind as defense-in-depth. `[VERIFIED: dial.go:32-39 + daemon.go:1318]` |
| `gofrs/flock` | `syscall.Flock` direct (Unix) | Hand-rolling loses Windows support (Windows uses `LockFileEx`); the milestone keeps a Windows local-dial smoke (RETIRE-02 SC#2). `gofrs/flock` abstracts both. `[CITED: ROADMAP.md:115]` |
| New `GRPCClientTransport` | Reuse daemon's `GRPCTransport` | The existing `GRPCTransport` (`internal/mcp/grpc_transport.go`) is *server-side* (Recv from forwarder → MCP SDK server). The CLI is the *client* — it needs the mirror image (MCP SDK client writes → `stream.Send`; `stream.Recv` → MCP SDK client reads). Same io.Pipe-bridge shape, opposite ends. Small new file, no proto change. `[VERIFIED: grpc_transport.go:18-24]` |
| MCP SDK client (`session.CallTool`) | Hand-frame JSON-RPC `tools/call` over `stream.Send` (like `trace_continuity_test.go:132-136`) | Hand-framing skips the MCP `initialize` handshake the daemon's SDK server expects; the SDK server may reject or not route un-initialized `tools/call`. The SDK client does the handshake correctly. Hand-framing is fine for a *trace* test (it only needs a span emitted) but wrong for a real result round-trip. Recommend SDK client. `[VERIFIED: trace_continuity_test.go vs harness.go:176-200]` |

**Installation:**
```bash
go get github.com/gofrs/flock@v0.13.0
```
(All other dependencies are already in `go.mod`.)

**Version verification:**
```bash
go list -m github.com/gofrs/flock@latest   # → github.com/gofrs/flock v0.13.0 (verified 2026-06-21)
```

## Package Legitimacy Audit

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/gofrs/flock` | Go modules (proxy.golang.org) | mature (gofrs org, multi-year) | widely used (k8s-adjacent ecosystem) | github.com/gofrs/flock | OK | Approved — verify maintainer + repo at install (`checkpoint:human-verify` recommended since discovered partly from training knowledge) |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

`gofrs/flock` is tagged `[ASSUMED]`-adjacent on provenance (the *name* came from training knowledge; existence + version confirmed via `go list -m`). Because the GSD package-legitimacy seam was unavailable in this session, the planner SHOULD add one `checkpoint:human-verify` task confirming `github.com/gofrs/flock v0.13.0` is the intended maintainer/repo before the `go get`. All other packages are already vendored project dependencies (no new install).

## Architecture Patterns

### System Architecture Diagram

```
helix <verb> --flag=val           (cobra parses verb + flags → tool name + args map)
      │
      ▼
internal/cli  (verb dispatch "spine")
      │  build CallToolParams{Name, Arguments}
      ▼
internal/forwarder.ConnectOrStartDaemonLocked(ctx, socketPath)   ← RACE FIX HERE
      │
      ├─ tryConnect(socket) ──────────► warm daemon? ─yes─► reuse conn  ─┐
      │        │ no                                                       │
      │        ▼                                                          │
      │   flock(lockfile).TryLock ── held by peer? ─► wait+retry connect ─┤
      │        │ acquired                                                 │
      │        ▼                                                          │
      │   startDaemon (exec self --serve --socket=…)                      │
      │        │                                                          │
      │        ▼                                                          │
      │   waitForDaemon (poll socket up to 10s) ───────────────────────► conn
      │        │                                                          │
      │   flock.Unlock                                                    │
      ▼                                                                   ▼
client.StreamMCP(ctx)  ── gRPC bidi stream ───────────────► daemon listenSocket (daemon.go:1318)
      │                                                            │ StreamMCP handler (daemon.go:1442)
      ▼                                                            ▼
GRPCClientTransport (NEW, mirror of mcp.GRPCTransport)      GRPCTransport → MCP SDK server
      │  initialize + 1× tools/call                               │  TelemetryMW→ProfileFilterMW→
      ▼                                                            │  SuggestMW→LazyInitMW→handler
mcp.NewClient(...).Connect → session.CallTool(name,args)          ▼
      │  ◄──────────── CallToolResult ◄───── tool result ◄──── kernel tool exec
      ▼
render result to stdout (terse rendering = Phase 92; here: raw/JSON is fine) → exit 0
      │
      ▼
session.Close() + conn.Close()   (one-shot: no persistent session)
```

### Recommended Project Structure
```
internal/cli/
├── root.go              # MODIFY: no-arg → help (CLI-04); register the verb dispatch
├── verb.go              # NEW: the one-shot verb dispatch "spine" command(s)
└── verb_test.go         # NEW: unit test no-arg help + flag→args mapping (no daemon)
internal/forwarder/
├── dial.go              # MODIFY: add cross-process startup lock around spawn+wait (CLI-03)
├── dial_lock.go         # NEW (optional): flock helper + lockfile path derivation
├── oneshot.go           # NEW: CallTool one-shot helper (connect→stream→client→CallTool→close)
└── grpc_client_transport.go  # NEW: client-side mirror of mcp.GRPCTransport (CLI-01)
test/integration/  (or internal/cli/)
└── cli_e2e_test.go      # NEW: HELIX_BIN-gated, !windows, real-subprocess oracle (TEST-01)
internal/forwarder/
└── dial_race_test.go    # NEW: synctest/fan-out — N parallel cold callers → 1 daemon PID (CLI-03)
```
> Note: a new `GRPCClientTransport` placed in `internal/forwarder` (not `internal/mcp`) keeps the MCP-SDK client wiring on the CLI side and avoids a forwarder→mcp import cycle question; the existing server-side `GRPCTransport` stays in `internal/mcp`. Confirm import direction during planning.

### Pattern 1: One-shot MCP CallTool over a custom transport
**What:** Use the MCP SDK client exactly as the integration harness does, but over the gRPC-backed client transport instead of an in-memory pipe.
**When to use:** Every `helix <verb>` invocation.
**Example:**
```go
// Source: test/integration/harness.go:176-200 (in-memory variant); Phase 90 swaps the transport.
client := mcp.NewClient(&mcp.Implementation{Name: "helix-cli", Version: cli.CurrentVersion()}, nil)
session, err := client.Connect(ctx, grpcClientTransport, nil) // grpcClientTransport wraps client.StreamMCP(ctx)
if err != nil { return err }
defer session.Close()
res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: toolName, Arguments: argsMap})
if err != nil { return err }
// render res.Content / res.IsError; Phase 92 owns terse formatting.
```

### Pattern 2: Client-side gRPC↔MCP-SDK transport (mirror of the daemon's)
**What:** Bridge the MCP SDK client's `IOTransport` reader/writer to `stream.Send`/`stream.Recv`.
**When to use:** Once, as the new `GRPCClientTransport.Connect`.
**Example:**
```go
// Source: mirror of internal/mcp/grpc_transport.go:44-130 (server-side), inverted for the client.
// SDK client WRITES JSON-RPC → serverWriter → goroutine → stream.Send(MCPMessage{Payload,SessionId})
// stream.Recv() → clientWriter → SDK client READS. Same io.Pipe pair shape; opposite direction.
```

### Pattern 3: Cross-process startup lock around the spawn window
**What:** Acquire a `flock` on `<socket>.lock` before spawning; release after the socket is up. Late arrivals that fail `TryLock` loop back to `tryConnect` (the winner's daemon is now up).
**When to use:** Inside `ConnectOrStartDaemon`, only on the cold path.
**Example:**
```go
// Source: internal/forwarder/dial.go:26-39 is the unlocked window today. Add:
fl := flock.New(socketPath + ".lock")
locked, _ := fl.TryLock()
if !locked {
    // peer is spawning; wait for ITS daemon rather than racing a second spawn.
    fl.Lock()          // block until peer releases (its socket is up)
    defer fl.Unlock()
    if c, conn, err := tryConnect(ctx, socketPath, tp); err == nil { return c, conn, nil }
    // fall through to spawn only if peer's daemon vanished.
}
defer fl.Unlock()
// double-check after acquiring: a peer may have finished between our first tryConnect and the lock.
if c, conn, err := tryConnect(ctx, socketPath, tp); err == nil { return c, conn, nil }
// ... existing startDaemon + waitForDaemon ...
```

### Anti-Patterns to Avoid
- **Hand-framing `tools/call` without `initialize`:** The daemon's MCP SDK server expects the protocol handshake; `trace_continuity_test.go:132` only works because it asserts on spans, not results. Use the SDK client. `[VERIFIED]`
- **Spawning the daemon without a lock and relying solely on the unix-socket bind:** N callers fork N daemons; N-1 fail `net.Listen` and exit — wasteful forks + connect storm, not "exactly one daemon" cleanly. `[VERIFIED: dial.go has no lock; daemon.go:1318]`
- **Changing `api/proto/serena/v1`:** breaks the zero-proto invariant (a hard acceptance gate, `git diff api/proto/` must be empty). `[CITED: REQUIREMENTS.md:26]`
- **Leaving the no-arg path falling into the forwarder:** `root.go:112-114` currently enters stdio forwarder mode on no args — CLI-04 explicitly forbids this. `[VERIFIED: root.go:98-118]`
- **Real `time.Sleep` in the race test:** use `testing/synctest` (already in-repo) for deterministic fan-out, or a `sync.WaitGroup` barrier with real subprocesses if asserting actual PIDs. `[VERIFIED: pool_synctest_test.go:5]`

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-process mutual exclusion for daemon spawn | A pidfile + read-check-write dance | `github.com/gofrs/flock` | Pidfiles race (TOCTOU between read and write); OS advisory locks are atomic and auto-release on process death. |
| Daemon connect+autostart | New dial logic | `forwarder.ConnectOrStartDaemon` (`dial.go:24`) | Already does try-connect → detached spawn → poll, with keepalive + otel handler. Reuse, just add the lock. |
| gRPC↔MCP byte bridge | A bespoke JSON-RPC framing loop | Mirror `mcp.GRPCTransport` (io.Pipe + IOTransport) (`grpc_transport.go`) | The newline-delimited JSON framing + pipe lifecycle is already correct on the server side; invert it. |
| MCP `initialize` + `tools/call` protocol | Manual JSON-RPC messages | `mcp.NewClient().Connect().CallTool()` | The SDK owns the handshake, id correlation, and result typing (`CallToolResult`). |
| Per-cell isolated daemon subprocess for the oracle | A new `exec.Command` harness | `evalsandbox.Sandbox.StartDaemon` + `DaemonHandle` (`sandbox.go:315`) | Owns socket-path-length safety (macOS 104-byte limit), env allowlist, single-`Wait` reaping (CR-01), graceful `Stop`/`Kill`, process-group isolation. |
| Helix-binary resolution in tests | Hardcoded paths | `resolveHelixBin()` pattern (`daemon_tap_integration_test.go:24`) | `HELIX_BIN` env → PATH fallback → SKIP; the established bench convention. |

**Key insight:** This phase is ~80% wiring of existing, battle-tested primitives. The only net-new logic is the startup lock and the client-side transport mirror — both small. Resist the urge to build a parallel dial/transport stack.

## Runtime State Inventory

> Not a rename/refactor phase. The one behavior *change* (CLI-04) is covered explicitly:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — verified: the CLI is stateless; the warm daemon holds all state (sockets/caches), unchanged this phase. | none |
| Live service config | The daemon socket path `os.TempDir()/helix-<uid>/daemon.sock` (`config/loader.go:64-76`) is the only host-registered artifact; unchanged. | none |
| OS-registered state | Daemon process group (`Setpgid`, `dial_unix.go:13`) — reused as-is. | none |
| Secrets/env vars | `HELIX_RUNNING_AS_DAEMON` (set in `daemon.go:1211`), `HELIX_BIN`/`HELIX_LOG_LEVEL` (tests). No new vars required; lockfile path derives from socket path. | none |
| Build artifacts | New `gofrs/flock` entry in `go.mod`/`go.sum` after `go get`. | run `go mod tidy` |
| **Behavior change (CLI-04)** | `root.go:112-114` routes no-arg → `runForwarder` (stdio MCP). Phase 90 reroutes no-arg → grouped help, exit 0, no stdio session. | code edit in `root.go` `runRoot` |

## Common Pitfalls

### Pitfall 1: The spawn race is wider than just "two daemons"
**What goes wrong:** Even with a lock, a caller can finish its first `tryConnect` (sees no socket), then block on the lock while the winner spawns. After acquiring the lock it must **re-check `tryConnect`** before spawning again, or it spawns a redundant daemon the instant the winner releases.
**Why it happens:** TOCTOU between the pre-lock connect probe and lock acquisition.
**How to avoid:** Double-checked locking — `tryConnect` again immediately after `flock` acquires (see Pattern 3). Also keep the unix-socket bind backstop: `ensureSocket` (`socket.go:14`) + `net.Listen` returns "daemon already running" / `EADDRINUSE` for any straggler that slips through.
**Warning signs:** Race test sometimes sees 2 PIDs; daemon log shows `daemon already running at <socket>`.

### Pitfall 2: Lockfile vs socket path on macOS (104-byte limit)
**What goes wrong:** The lockfile must NOT inflate the socket path; the socket itself is already near the 104-byte AF_UNIX limit on macOS (the sandbox deliberately roots under `/tmp` for this, `sandbox.go:33-37`).
**Why it happens:** `<socket>.lock` adds 5 bytes — fine for the default `os.TempDir()/helix-<uid>/daemon.sock`, but a long `--socket` override + `.lock` could exceed limits for the *socket* (the lock is a regular file, not bound, so its own length is unbounded).
**How to avoid:** Lockfile is a normal file (`flock` opens it, never `bind`s) — its length is not AF_UNIX-limited. Keep the socket path itself short; the lock can be `<socket>.lock` safely.
**Warning signs:** `bind: invalid argument` on the socket (not the lock) under long `--socket` overrides.

### Pitfall 3: One-shot client must close cleanly or the daemon counts an `error` session
**What goes wrong:** The daemon's `StreamMCP` handler classifies session end as `started`/`ended`/`error` (`daemon.go:1442-1470`). If the CLI drops the stream uncleanly, the daemon increments the `error` counter, polluting telemetry.
**Why it happens:** Forgetting `session.Close()` / not calling `stream.CloseSend()`; killing the conn mid-flight.
**How to avoid:** `defer session.Close()` then `defer conn.Close()`; let the SDK client's normal teardown flush. Confirm the daemon logs `forwarder stream ended` (not error) in the E2E oracle.
**Warning signs:** `helix_session_lifecycle{outcome="error"}` climbing in admin metrics per CLI call.

### Pitfall 4: synctest cannot drive real subprocesses
**What goes wrong:** `testing/synctest` virtualizes the Go scheduler/clock *within one process*. It cannot fork N real `helix` daemons.
**Why it happens:** Conflating "N goroutines racing the lock" (synctest-able) with "N OS processes racing to spawn a daemon" (needs real subprocesses).
**How to avoid:** Two complementary tests — (a) a **synctest** unit test of the in-process lock/double-check logic (N goroutines, virtual clock, assert one `startDaemon` call via a spawn-counter seam); (b) a **real-subprocess** integration test (N parallel `exec` of a tiny `helix`-dial harness or N goroutines each calling `ConnectOrStartDaemon` against the SAME socket, then `pgrep`/count daemon PIDs == 1). The success criterion ("N parallel cold `helix` invocations → exactly one daemon process") is the real-process test; synctest pins the algorithm.
**Warning signs:** synctest panics on `exec.Command`/real I/O ("not idle" deadlock detection firing on the blocked syscall).

### Pitfall 5: 2nd-call SLO must measure WARM, not cold+LS-index
**What goes wrong:** Timing the second call while the LS is still indexing the workspace inflates p50 and makes the SLO meaningless or flaky.
**Why it happens:** Cold daemon + cold LS; the first call may return before LS readiness.
**How to avoid:** SLO is for a daemon that is already warm AND (for LS-backed verbs) indexed — gate the measurement on LS readiness like `WaitForLS` (`harness.go:237`), OR pick a representative verb that does not require LS warmth (e.g. a memory/health/fileops verb) so the SLO isolates *dial + IPC + dispatch* latency, which is what CLI-02 is really about. Record the chosen verb + warmth precondition in the SLO note.
**Warning signs:** Wildly variable second-call timings; SLO passes locally, fails in CI cold-start.

### Pitfall 6: Windows local-dial parity
**What goes wrong:** A flock or socket assumption that is Unix-only breaks the milestone's Windows local-dial smoke (RETIRE-02 SC#2, `ROADMAP.md:115`).
**Why it happens:** Unix socket + `syscall.Flock` are POSIX; Windows uses named pipes + `LockFileEx`.
**How to avoid:** `gofrs/flock` is cross-platform. The race-fix logic must not hard-depend on `syscall.Flock`. Mark Unix-only *tests* with `//go:build !windows` (the existing convention, `daemon_tap_integration_test.go:1`), but keep the *production* lock portable.
**Warning signs:** `go build` failure on the Windows CI runner; `syscall.Flock` undefined on windows.

## Code Examples

Verified patterns already in this repo:

### Auto-start + connect (reuse verbatim, then wrap with lock)
```go
// Source: internal/cli/activate.go:53 — a working CLI command that already uses ConnectOrStartDaemon.
client, conn, err := forwarder.ConnectOrStartDaemon(cmd.Context(), socketPath, logger, noop.NewTracerProvider())
if err != nil { return fmt.Errorf("connecting to daemon: %w", err) }
defer conn.Close()
```

### MCP client CallTool (reuse the API over the new transport)
```go
// Source: test/integration/harness.go:197-206
result, err := session.CallTool(ctx, &mcp.CallToolParams{
    Name:      "search_symbols",
    Arguments: map[string]any{"query": query},
})
if err != nil { /* transport/protocol error */ }
if result.IsError { /* tool returned an error result */ }
```

### Daemon-side gRPC↔SDK bridge (the mirror to invert for the client)
```go
// Source: internal/mcp/grpc_transport.go:44-129 — server side.
// gRPC stream.Recv() → clientWriter → IOTransport.Reader → MCP SDK (server)
// MCP SDK (server) → IOTransport.Writer → serverReader → gRPC stream.Send()
// Client side is the same pipes wired to the SDK *client* and to client.StreamMCP(ctx).
```

### Real-subprocess daemon for the oracle
```go
// Source: internal/eval/sandbox/sandbox.go:315 + bench/runtime/daemon_tap_integration_test.go:24-64
helixBin := resolveHelixBin()           // HELIX_BIN → PATH → ""(skip)
if helixBin == "" { t.Skip("…") }
sb, _ := sandbox.NewSandbox(runID, helixBin)
defer sb.Cleanup()
sb.Prepare(task, mode)
h, _ := sb.StartDaemon(ctx, task, mode, "", "")   // waits for socket; "" profile/cfg
defer h.Kill()
pid := h.Pid()                                     // assert exactly one daemon
```

### synctest deterministic concurrency (the race-algorithm test)
```go
// Source: internal/kernel/lspool/pool_synctest_test.go:31
synctest.Test(t, func(t *testing.T) {
    // N goroutines call the lock-guarded spawn fn against a fake spawner that
    // counts startDaemon invocations; assert count == 1 with a virtual clock.
})
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `GOEXPERIMENT=synctest` | `testing/synctest` GA in stdlib | Go 1.25 | No experiment flag needed; this module is `go 1.25.1`. `[VERIFIED: go.mod]` |
| no-arg `helix` → stdio MCP forwarder | no-arg `helix` → grouped help (this phase); stdio head deleted (Phase 94) | v2.0 (Phase 90/94) | CLI-04 flips the default; full deletion deferred to keep strangler-fig parity. `[CITED: ROADMAP.md:28-32]` |

**Deprecated/outdated:** Nothing being removed in Phase 90. The MCP forwarder head and HTTP transport remain live until Phase 94 (strangler-fig). `[CITED: STATE.md:56]`

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `gofrs/flock v0.13.0` is the correct, intended cross-platform flock library (name from training knowledge; version verified via `go list`). | Standard Stack / Legitimacy | Low — if a different lock lib is preferred, swap is local to `dial.go`/`dial_lock.go`. Planner should add one `checkpoint:human-verify` for the `go get`. |
| A2 | The MCP SDK server inside the daemon requires a normal `initialize` handshake before `tools/call` (hence the SDK client, not hand-framing). | Architecture Patterns | Low-Med — if the daemon tolerates bare `tools/call`, hand-framing would also work; SDK client is strictly safer regardless. |
| A3 | A representative verb exists that round-trips without LS warmth for a clean SLO (e.g. a memory/health/fileops tool). | Pitfall 5 / Validation | Low — if all chosen verbs need LS, gate the SLO on `WaitForLS`; the SLO definition adapts. |
| A4 | Placing `GRPCClientTransport` in `internal/forwarder` avoids an import cycle with `internal/mcp`. | Project Structure | Low — resolved trivially by package placement during planning; worst case it lives in a new `internal/clirpc`. |
| A5 | The no-arg CLI-04 change can be confined to `runRoot` in `root.go` without touching the forwarder package (Phase 94 owns deletion). | Phase Requirements (CLI-04) | Low — `runRoot` is the single dispatch point (`root.go:98-118`). |

## Open Questions

1. **What is the numeric 2nd-call SLO?**
   - What we know: CLI-02 requires a *phase-set* warm second-call p50; the dial path is unix-socket + gRPC + one `tools/call`.
   - What's unclear: the exact ms target (depends on host + chosen verb).
   - Recommendation: Measure the warm round-trip in the E2E oracle for a no-LS verb, set p50 SLO at a comfortable multiple of the observed median (e.g. ≤ 50–100ms p50 is plausible for loopback unix-socket IPC), and record it in the phase SUMMARY. Make the test assert the recorded number, not a hardcoded guess.

2. **Lock granularity: per-socket or global?**
   - What we know: the socket path is per-uid by default but `--socket` can override (bench uses per-cell sockets).
   - What's unclear: whether the lock should key on the socket path (allows distinct daemons for distinct sockets to start concurrently) or be global.
   - Recommendation: **Per-socket lockfile** (`<socketPath>.lock`). This is correct for both the default single-daemon case and the bench's per-cell sockets (parallel cells with distinct sockets must not serialize). `[VERIFIED: sandbox.go SocketFor per (task,mode)]`

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test | ✓ | go1.26.0 (module go 1.25.1) | — |
| `testing/synctest` | race-algorithm test | ✓ (GA) | stdlib 1.25+ | real-subprocess test alone |
| `gofrs/flock` | startup lock | ✗ (not yet a dep) | v0.13.0 (resolvable) | `go get` adds it; or `syscall.Flock` (Unix-only, loses Windows) |
| `helix` binary | E2E oracle | built on demand | — | `HELIX_BIN` env / `go build ./cmd/helix`; test SKIPs if absent |
| unix domain sockets | dial path | ✓ (Linux/macOS) | — | Windows named pipe (Phase 94 concern; tests `//go:build !windows`) |

**Missing dependencies with no fallback:** none (all blocking deps are present or trivially added).
**Missing dependencies with fallback:** `gofrs/flock` (add via `go get`, or fall back to `syscall.Flock` at the cost of Windows portability — not recommended given the Windows smoke).

## Validation Architecture

> `workflow.nyquist_validation: true` in `.planning/config.json` — section REQUIRED. `[VERIFIED: .planning/config.json]`

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `github.com/stretchr/testify` (require/assert) + `testing/synctest` |
| Config file | none (standard `go test`) |
| Quick run command | `go test ./internal/cli/... ./internal/forwarder/...` |
| Full suite command | `HELIX_BIN="$(pwd)/helix" go test ./...` (E2E oracle SKIPs without `HELIX_BIN`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CLI-01 | verb round-trips a `tools/call`, same result as MCP path; proto unchanged | integration (E2E) + a `git diff api/proto/` CI gate | `HELIX_BIN=… go test ./internal/cli/ -run CLI_OneShot -x` + `git diff --exit-code api/proto/` | ❌ Wave 0 |
| CLI-02 | warm 2nd-call p50 ≤ SLO | integration (timed) | `HELIX_BIN=… go test ./internal/cli/ -run CLI_WarmReuseSLO -x` | ❌ Wave 0 |
| CLI-03 | N parallel cold → exactly one daemon | (a) synctest unit + (b) real-subprocess integration | `go test ./internal/forwarder/ -run Race_OneDaemon` + `HELIX_BIN=… go test ./internal/cli/ -run CLI_ParallelColdSingleDaemon` | ❌ Wave 0 |
| CLI-04 | no-arg → grouped help, exit 0, no stdio session | unit (cobra, no daemon) | `go test ./internal/cli/ -run NoArgHelp -x` | ❌ Wave 0 |
| TEST-01 | representative verb as subprocess vs live daemon, green | integration (E2E oracle) | `HELIX_BIN=… go test ./internal/cli/ -run CLI_E2E -x` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/cli/... ./internal/forwarder/...` (fast, no daemon; covers CLI-04 + synctest race-algorithm + flag→args mapping).
- **Per wave merge:** `HELIX_BIN="$(pwd)/helix" go test ./internal/cli/... ./internal/forwarder/...` (lights up the E2E oracle + warm-SLO + real-subprocess single-daemon).
- **Phase gate:** `go vet ./... && HELIX_BIN="$(pwd)/helix" go test ./...` green, AND `git diff --exit-code api/proto/` empty, before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/cli/verb_test.go` — no-arg help (CLI-04) + flag→args mapping (unit, no daemon).
- [ ] `internal/forwarder/dial_race_test.go` — synctest: N goroutines, one `startDaemon` call (CLI-03 algorithm).
- [ ] `internal/cli/cli_e2e_test.go` — `HELIX_BIN`-gated, `//go:build !windows`: real daemon via `evalsandbox`, representative verb round-trip (TEST-01, CLI-01), warm 2nd-call timing (CLI-02), N-parallel-cold → one PID (CLI-03 real).
- [ ] CI step: `git diff --exit-code api/proto/` (zero-proto gate, CLI-01).
- [ ] Framework install: none — `testing`, `testify`, `synctest` all available. `go get github.com/gofrs/flock@v0.13.0` (one-time).

## Security Domain

> `security_enforcement` not set to false → section included. This phase is local IPC over a per-uid unix socket; the load-bearing security work (profile/mode enforcement on `tools/call`) is **explicitly Phase 91 (SEC-01/02)**, NOT this phase. `[CITED: STATE.md:54]`

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | local unix socket, per-uid dir `helix-<uid>` mode 0700 (`socket.go:33`, `loader.go:64`) — OS user is the principal. |
| V3 Session Management | partial | MCP session lifecycle owned by daemon (`daemon.go:1442`); one-shot CLI closes cleanly (Pitfall 3). |
| V4 Access Control | **deferred to Phase 91** | `tools/call` profile/mode enforcement is SEC-01/02; Phase 90 must NOT regress by exposing destructive verbs unguarded — but the verbs themselves don't land until 91, so 90's dial spine drives the still-filtered MCP path. `[CITED: STATE.md:54]` |
| V5 Input Validation | yes | tool args validated by the MCP SDK / tool schemas inside the daemon (unchanged). |
| V6 Cryptography | no | `crypto/rand` session IDs already handled (`forwarder.go:141`); no new crypto. |

### Known Threat Patterns for local CLI↔daemon IPC
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Symlink attack on socket/lock path in shared tmp | Tampering | per-uid dir 0700 (`createSocketDir`, `socket.go:33`); sandbox rejects symlinked roots (`sandbox.go:72`). Lockfile inherits the same 0700 dir. |
| Stale socket from crashed daemon → connect failure / spoof | Spoofing/DoS | `ensureSocket` dials before reuse and removes stale sockets (`socket.go:14-28`); flock auto-releases on holder death. |
| Duplicate-daemon connect storm | DoS | the CLI-03 startup lock (this phase) + `net.Listen` bind backstop. |
| Non-loopback exposure | Info disclosure | out of scope — TCP bind is RETIRE-04 (Phase 94), gated/loopback-default. `[CITED: REQUIREMENTS.md:65]` |

## Sources

### Primary (HIGH confidence)
- `internal/forwarder/dial.go:24-123` — `ConnectOrStartDaemon`, `tryConnect`, `startDaemon`, `waitForDaemon` (the dial path + the unlocked spawn race).
- `internal/forwarder/forwarder.go:45-176` — `StreamMCP` usage, `isToolsCall`, session ID generation.
- `internal/forwarder/dial_unix.go:13` / `dial_windows.go:23` — process-group detach (platform).
- `internal/mcp/grpc_transport.go:18-130` — server-side gRPC↔MCP-SDK bridge (the mirror to invert).
- `internal/daemon/daemon.go:1316-1492` — `listenSocket`, `StreamMCP` handler, `defaultSessionRunner`, `GRPCTransport` wiring, session lifecycle classification.
- `internal/daemon/socket.go:14-34` — `ensureSocket` / `createSocketDir` (stale-socket + 0700 dir).
- `internal/config/loader.go:64-76` — default socket path (`os.TempDir()/helix-<uid>/daemon.sock`).
- `internal/cli/root.go:44-206` — cobra wiring, `runRoot` (no-arg → forwarder today, CLI-04 target), `runForwarder`, `runDaemon`.
- `cmd/helix/main.go:14-22` — single entrypoint.
- `internal/cli/activate.go:32-73` — working CLI command using `ConnectOrStartDaemon` + a unary gRPC call (model for the verb command).
- `internal/eval/sandbox/sandbox.go:184-439` — `DaemonHandle` (Pid/Stop/Kill single-Wait), `StartDaemon`, `waitSocket`.
- `bench/runtime/subprocess/daemon.go:28-49` — bench-side `StartDaemon` wrapper (no-TCP-port invariant).
- `bench/runtime/daemon_tap_integration_test.go:24-75` — `resolveHelixBin` + `HELIX_BIN`-gated real-subprocess E2E pattern.
- `test/integration/harness.go:166-230` — MCP SDK client connect + `CallTool` + `WaitForLS`.
- `test/integration/trace_continuity_test.go:129-136` — raw `StreamMCP` send of a `tools/call` JSON-RPC payload (the hand-framing anti-pattern reference).
- `internal/kernel/lspool/pool_synctest_test.go:1-60` — `testing/synctest` usage pattern.
- `api/proto/serena/v1/ipc.proto:9-30` — `StreamMCP` + `MCPMessage` (the wire that must not change).
- `go.mod` — dependency versions (cobra v1.10.2, go-sdk v1.5.0, grpc v1.80.0, go 1.25.1).

### Secondary (MEDIUM confidence)
- `go list -m github.com/gofrs/flock@latest` → `v0.13.0` (run this session).
- `.planning/config.json` — `nyquist_validation: true`.

### Tertiary (LOW confidence)
- `gofrs/flock` as the *named* idiomatic Go cross-process lock (training knowledge; existence/version verified, maintainer identity flagged for human-verify — see A1).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all but one library already in `go.mod`; the one new lib verified via `go list`.
- Architecture: HIGH — every component (dial, transport, harness) read directly from this repo with file:line.
- Pitfalls: HIGH — derived from concrete in-repo invariants (session lifecycle classification, AF_UNIX limits, synctest constraints, no-arg routing).

**Research date:** 2026-06-21
**Valid until:** 2026-07-21 (stable — internal code references; re-check only if `dial.go`, `grpc_transport.go`, or the sandbox harness change before planning).
