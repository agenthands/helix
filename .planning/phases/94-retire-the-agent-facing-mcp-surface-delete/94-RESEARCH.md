# Phase 94: Retire the Agent-Facing MCP Surface (DELETE) - Research

**Researched:** 2026-06-21
**Domain:** Go daemon/transport surgery — deleting two agent-facing MCP heads (stdio forwarder + Streamable-HTTP `/mcp`) while preserving the gRPC IPC dial path the CLI rides
**Confidence:** HIGH (codebase-anchored via SMTC/grep + Read; every delete/keep line cited with file:line)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
All implementation choices are at Claude's discretion — discuss was skipped. Bounded by carried v2.0 roadmap constraints:
- **DELETE only the two agent-facing heads:** stdio MCP forwarder head + Streamable-HTTP `/mcp` (`--mode http` MCP serving). Do NOT remove the daemon, the gRPC IPC, `StreamMCP`, `GRPCTransport`, the 5 middlewares, or any tool handler — those stay behind the wire (the CLI dials them).
- **RETIRE-03 is the gate:** dual-run parity must be green in the commit before deletion.
- **RETIRE-04 (optional gRPC TCP bind) rides this phase:** loopback/unix-socket default; non-loopback opt-in, gated per the v1.2 admin-addr pattern (do not bind a non-loopback address by default).
- Provide a short ADR / decision record for the remote/multi-client scope the removed HTTP transport previously covered.
- After deletion: no reachable stdio MCP server path; `/mcp` endpoint gone; `--mode http` no longer serves MCP; the CLI still dials the daemon (include the Windows local-dial smoke).
- Honor the docgen-drift lesson and zero-new-deps / proto-change discipline where applicable (proto stays; `StreamMCP` retained, so `api/proto/` likely unchanged — assert it).

### Claude's Discretion
- How to sequence plans/waves so parity proves first, deletion second.
- Whether to remove the deferred IN-01 `mergeJSONConfig` dead code here.
- Exact config key / flag name for the RETIRE-04 TCP bind.
- How to re-target or retire the HTTP-transport protocol oracle tests.

### Deferred Ideas (OUT OF SCOPE)
- Docs/identity rewrite + docgen regen against the frozen MCP-free surface is **Phase 95**, not this phase.
- Excising the MCP SDK from the daemon (out of milestone scope).
- Removing the gRPC IPC layer (it carries the `tools/call` frames — retained).
- Re-implementing the 5 middlewares natively.
- A compatibility MCP shim / dual MCP+CLI head.
- Real authn for non-loopback TCP (deferred to REMOTE-01).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| RETIRE-01 | Remove the stdio MCP forwarder head; agents no longer connect via stdio MCP (forwarder dial path retained only for CLI→daemon gRPC). Acceptance: no stdio MCP server code path remains reachable; CLI still dials the daemon. | The agent-facing stdio head is `RunForwarder` (`internal/forwarder/forwarder.go:22`), reached only from `runForwarder`→`runForwarderFn` via `--mode=stdio` (`internal/cli/root.go:189-192`, `:219-231`). Delete `RunForwarder` + the `--mode=stdio` route; KEEP `ConnectOrStartDaemon`/`tryConnect`/`CallTool`/`GRPCTransport` (the CLI dial path). `RunStdio` (`internal/mcp/server.go:191`) is already dead — remove it too. |
| RETIRE-02 | Remove the Streamable-HTTP MCP transport (`/mcp`, `--mode http`). Acceptance: HTTP MCP endpoint gone; `--mode http` no longer serves MCP. | The HTTP head is `listenHTTP` (`internal/daemon/daemon.go:1399-1431`) mounting `/mcp` via `httpSessionMiddleware(d.mcpServer.HTTPHandler(), …)` (`:1405`). Delete `listenHTTP`, its goroutine launch (`:1316-1321`), `HTTPHandler` (`internal/mcp/server.go:195-200`), `http_session_middleware.go`, and the `--mode http`/`--http-addr` flags + `DaemonConfig.HTTPAddr`. |
| RETIRE-03 | MCP-head removal happens only after CLI parity is proven via dual-run (strangler-fig). Acceptance: dual-run parity test green in the commit immediately BEFORE the deletion commit. | Extend the existing `test/oracle/contract/golden_test.go` / `internal/cli/cli_e2e_test.go` harness which ALREADY runs `helix <verb>` subprocess AND `forwarder.CallTool` (MCP path) against one daemon (`cli_e2e_test.go:241-278`). Land an explicit dual-run parity test in its own plan/commit that passes GREEN, THEN delete in a later commit. |
| RETIRE-04 | Retained gRPC IPC optionally binds a TCP address for split-host CLI↔daemon use (loopback/unix default; non-loopback opt-in, gated + documented per v1.2 admin-addr pattern). Acceptance: CLI can target a configured TCP daemon endpoint; default remains local unix socket. | Mirror `listenAdmin`/`validateAdminAddr` (`internal/daemon/telemetry.go:40-121`) and the `--admin-addr` flag wiring (`internal/cli/root.go:132-134,260-262`). Add an optional `daemon.grpc_addr` TCP listener gated loopback-only by default; teach `tryConnect` (`internal/forwarder/dial.go:91`) to dial `tcp://` when configured. |
</phase_requirements>

## Summary

Phase 94 is a precise **DELETE** of exactly two agent-facing transports, sitting atop an architecture that already cleanly separates the head (delete) from the wire (keep). The codebase makes this surgical because both heads were always thin shims over a shared engine:

1. **stdio forwarder head** = `forwarder.RunForwarder` (reads `os.Stdin`/writes `os.Stdout`, framing JSON-RPC over the gRPC `StreamMCP` stream), reachable only through `--mode=stdio` → `runForwarder`. Its sibling `forwarder.CallTool` (the one-shot CLI dial path, CLI-01) shares `ConnectOrStartDaemon`, `StreamMCP`, `generateSessionID`, and the proto — and is RETAINED. The shared `dial.go`/`grpc_client_transport.go` stay; only the stdin/stdout pump (`forwarder.go`) and the `--mode=stdio` route die.

2. **Streamable-HTTP `/mcp` head** = `daemon.listenHTTP` mounting `mcp.SerenaMCPServer.HTTPHandler()` (the SDK `StreamableHTTPHandler`) wrapped by `http_session_middleware.go`, launched conditionally on `DaemonConfig.HTTPAddr != ""`. Deleting this leaves `listenSocket` (the gRPC-over-unix listener the CLI dials, `daemon.go:1354`) and the `StreamMCP` handler (`daemon.go:1481`) fully intact.

Both heads dispatch into the SAME `mcpServer.SDK()` with all middlewares already installed (Telemetry, ProfileFilter, Suggestion, Guardrail, ProfileEnforcement, LazyInit — 6 receiving middlewares; "5 middlewares" in roadmap prose predates the SEC-01 ProfileEnforcement add). The middlewares attach to the SDK, NOT to either transport, so deletion does not touch them. **`RunStdio` (`internal/mcp/server.go:191`) is already dead code** — defined, zero callers (the stdio path goes through the forwarder + gRPC, never the SDK's StdioTransport). It should be removed in the same sweep.

**Primary recommendation:** Sequence as **two ordered plans across two waves**: (Plan A / Wave 1) land RETIRE-04's optional TCP bind AND the RETIRE-03 dual-run parity test, prove GREEN, commit. (Plan B / Wave 2) delete both heads + dead code, re-target the HTTP-transport oracle tests onto the gRPC `GRPCTransport` path, and assert `git diff api/proto/` is empty + a Windows local-dial smoke. Parity proof commit MUST precede the deletion commit (the strangler-fig invariant, CONTEXT line 14).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Agent stdio MCP session (DELETE) | Forwarder process (`internal/forwarder`) | — | `RunForwarder` is a per-process stdin/stdout pump; an IDE/agent connects here. No longer needed — agents use the CLI. |
| Agent HTTP MCP session (DELETE) | Daemon HTTP listener (`internal/daemon`) | MCP SDK (`internal/mcp`) | `/mcp` is the only network-transparent topology; replaced by the gated gRPC-TCP opt-in + an ADR. |
| CLI one-shot `tools/call` (KEEP) | CLI (`internal/cli`) → forwarder dial helpers (`internal/forwarder`) | gRPC `StreamMCP` / daemon | The retained agent surface. Shares dial code with the deleted stdio head — careful seam. |
| Tool dispatch + 5/6 middlewares (KEEP) | MCP SDK inside daemon (`internal/mcp`) | — | Middlewares attach to `mcpServer.SDK()`, transport-agnostic; both heads and the CLI path funnel through it. |
| gRPC IPC wire (KEEP) | `api/proto/serena/v1` + `daemon.listenSocket` | `daemon.StreamMCP` handler | Carries `tools/call`; proto MUST NOT change (zero-proto invariant). |
| Split-host CLI↔daemon (NEW, gated) | Daemon optional TCP gRPC listener | CLI dial (`tryConnect`) | RETIRE-04: loopback default, non-loopback opt-in gated per admin-addr. |

## Standard Stack

No new dependencies. This is a deletion + a small additive listener reusing the existing stack.

### Core (already present, retained)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `google.golang.org/grpc` | (go.mod pinned) | Unix + (new) TCP gRPC listener; CLI dial | Already the IPC wire; `tryConnect` uses `grpc.NewClient` |
| `github.com/modelcontextprotocol/go-sdk/mcp` | v1.5.0 (`go.mod:14`) | Internal SDK dispatch + middleware engine (retained) | Stays inside the daemon; only `StreamableHTTPHandler` usage is removed |
| `github.com/spf13/cobra` | v1.9.1 | Flag/route removal (`--mode`, `--http-addr`); new TCP flag | Existing CLI framework |
| `github.com/gofrs/flock` | (go.mod pinned) | Startup lock in shared `dial.go` (retained) | Untouched |

### Don't Add Anything
| Temptation | Don't | Use Instead |
|------------|-------|-------------|
| A new TLS/auth lib for the TCP bind | RETIRE-04 only GATES loopback; full auth is deferred to REMOTE-01 | `net.IP.IsLoopback()` like `validateAdminAddr` (`telemetry.go:117`) |
| A new TCP transport abstraction | The gRPC `tryConnect` already builds `grpc.NewClient("unix://"+path)`; just branch the scheme | Extend `tryConnect` to accept `tcp://host:port` |

## Package Legitimacy Audit

**Not applicable.** This phase installs zero external packages — it is pure deletion plus an additive listener reusing already-vendored deps. No npm/PyPI/crates lookups required. Zero-new-deps discipline is itself a verification gate (assert `git diff go.mod go.sum` is empty).

## Architecture Patterns

### System Architecture Diagram (before → after)

```
BEFORE (two agent-facing heads + CLI path):

  Agent/IDE --stdio--> [helix --mode=stdio]            \
                        forwarder.RunForwarder          \
                        (stdin/stdout <-> gRPC stream)   \
                                                          \
  Agent/IDE --HTTP---> [daemon :8080 /mcp]               -->  daemon gRPC StreamMCP  -->  mcpServer.SDK()
                        listenHTTP + HTTPHandler          /   (listenSocket, unix)        + 6 middlewares
                        + http_session_middleware        /         ^                       -> 53 tool handlers
                                                         /          |
  Agent  -----CLI----> [helix <verb>]  ----------------/-----------/
                        forwarder.CallTool (one-shot tools/call over gRPC)   <-- RETAINED

AFTER (CLI-only agent surface):

  Agent  -----CLI----> [helix <verb>]  --> forwarder.CallTool --> ConnectOrStartDaemon
                                                                    |
                                            (default) unix://daemon.sock  -- OR (gated opt-in) --> tcp://host:port
                                                                    |
                                                          daemon gRPC StreamMCP --> mcpServer.SDK()
                                                          (listenSocket [+ listenGRPCTCP]) + 6 middlewares
                                                                                       -> 53 tool handlers

  (deleted: RunForwarder, --mode=stdio route, listenHTTP, /mcp, HTTPHandler,
            http_session_middleware, --mode http, --http-addr, DaemonConfig.HTTPAddr, RunStdio)
```

A reader can trace: `helix find-references` → `runVerb` → `forwarder.CallTool` → `ConnectOrStartDaemon` (unix or, if opted-in, tcp) → `client.StreamMCP` → daemon `forwarderServiceHandler.StreamMCP` → `GRPCTransport` → `mcpServer.SDK().Connect` → middlewares → tool handler. None of that chain is deleted.

### Pattern 1: Head vs. shared-wire seam (the load-bearing distinction)
**What:** Both `RunForwarder` (delete) and `CallTool` (keep) live in `internal/forwarder` and both call `ConnectOrStartDaemon` + `StreamMCP`. The seam is: the *transport pump* (stdin/stdout in `forwarder.go`) is the head; the *dial helpers* (`dial.go`, `dial_lock.go`, `dial_unix.go`, `dial_windows.go`, `grpc_client_transport.go`, `oneshot.go`) are the wire.
**When to use:** Delete `forwarder.go` (`RunForwarder`, `generateSessionID` if unused elsewhere, `isToolsCall`, `sendWithSpan`). KEEP everything else in the package.
**Verify before deleting `generateSessionID`:** it is also called by `CallTool` (`oneshot.go:53`). It must MOVE to `oneshot.go` (or stay), not be deleted. Likewise `isToolsCall`/`sendWithSpan` are forwarder-only — safe to delete.

```go
// internal/cli/root.go:188-198 — the route to delete (the `case "stdio"` arm):
switch mode {
case "stdio":
    return runForwarderFn(cmd)   // <-- DELETE this arm; runForwarder + runForwarderFn become unused
case "auto":
    return cmd.Help()            // <-- KEEP (CLI-04)
default:
    return fmt.Errorf("unknown mode: %s", mode)
}
```

### Pattern 2: admin-addr loopback gate (the RETIRE-04 template)
**What:** `validateAdminAddr` (`telemetry.go:108-121`) refuses non-loopback hosts unless `net.ParseIP(host).IsLoopback()`; the listener is a no-op when the addr is empty (`telemetry.go:42-44`); the CLI flag only applies the override when non-empty (`root.go:260-262`).
**When to use:** Build `listenGRPCTCP` + `validateGRPCAddr` as direct analogs. Default empty (= unix-only). Non-loopback refused with an error pointing at REMOTE-01.

```go
// Source: internal/daemon/telemetry.go:108-121 (the pattern to mirror for grpc_addr)
func validateAdminAddr(addr string) error {
    host, _, err := net.SplitHostPort(addr)
    if err != nil { return fmt.Errorf("invalid admin addr %q: %w", addr, err) }
    switch host {
    case "", "localhost", "127.0.0.1", "::1":
        return nil
    }
    if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() { return nil }
    return fmt.Errorf("admin addr must be loopback, got %q (v1.3 will add auth for non-loopback)", host)
}
```

### Pattern 3: dual-run strangler-fig parity (RETIRE-03)
**What:** Run the SAME tool through the CLI verb subprocess AND through the in-process MCP-path reference, against ONE live daemon, and assert the results agree on the load-bearing fields. The harness already exists.
**When to use:** A dedicated parity test that runs FIRST (its own commit) and is green BEFORE the deletion commit.
**Existing harness to extend:** `internal/cli/cli_e2e_test.go:234-278` (`TestCLI_E2E_OneShot`) already captures `f.mcpSearch` (MCP path via `forwarder.CallTool`) and compares to `f.runCLIVerb` (subprocess) locus. `test/oracle/contract/golden_test.go:158-165` brings up a daemon AND calls `forwarder.CallTool` for activate. Generalize across a representative tool set.

```go
// Source: internal/cli/cli_e2e_test.go:241-278 (the dual-run shape already present)
f.mcpActivate(t, ctx)
reference := f.mcpSearch(t, ctx)              // MCP path (forwarder.CallTool over gRPC)
out, _ := f.runCLIVerb(ctx, verb, flag+pat)  // CLI path (real helix subprocess)
// assert CLI locus == MCP locus (same relpath:line)
```

### Anti-Patterns to Avoid
- **Deleting `dial.go`/`grpc_client_transport.go`/`oneshot.go`:** these ARE the retained CLI dial path. Only `forwarder.go` (the stdin/stdout pump) is the head.
- **Deleting `generateSessionID` outright:** `CallTool` depends on it (`oneshot.go:53`). Relocate, don't delete.
- **Deleting `StreamMCP`, `listenSocket`, `GRPCTransport`, or `forwarderServiceHandler`:** all retained; the CLI dials them.
- **Touching `api/proto/serena/v1/*`:** the `StreamMCP` RPC stays; `git diff api/proto/` MUST be empty. The proto package dir is named `serena` as a wire-format lineage artifact (CLAUDE.md / Phase 52-03) — do not rename.
- **Defining the parity test in the SAME commit as the deletion:** violates RETIRE-03's "green in the commit immediately before deletion."
- **Binding the new TCP listener non-loopback by default:** RETIRE-04 mandates loopback/unix default, opt-in only.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Loopback validation for the TCP bind | A custom host allowlist | `net.IP.IsLoopback()` + the `validateAdminAddr` switch (`telemetry.go:108`) | Already battle-tested; identical semantics; one source of truth for the loopback policy |
| Graceful listener shutdown for the TCP bind | A bespoke shutdown channel | Copy `listenSocket`'s `grpcServer.GracefulStop()` on `ctx.Done()` (`daemon.go:1390-1396`) | Existing pattern; errgroup-integrated |
| Dead-code detection after deletion | Manual reading | `go build ./... && go vet ./...` + `deadcode ./...` (golang.org/x/tools) | Compiler catches orphaned refs; vet/deadcode catch unused funcs |
| Proto-change detection | Eyeballing | `git diff --exit-code api/proto/` as a verify step | Mechanical, matches the zero-proto invariant gate |

**Key insight:** The architecture already did the hard separation work (heads are shims over `mcpServer.SDK()`). The risk is not building something — it is deleting one line too many across the shared `internal/forwarder` package. Lean on the compiler: after each deletion, `go build ./cmd/helix` must succeed, and the retained `TestCLI_E2E_OneShot` (minus its MCP-reference half if that half is reframed) must still pass.

## Runtime State Inventory

> This is a code/transport deletion, not a rename/data migration. The five categories are answered explicitly below.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — no datastore keys reference "stdio"/"http"/"/mcp" as identifiers. Daemon sockets and lock/log files are derived at runtime from `DefaultSocketPath()`, not persisted with transport names. | None |
| Live service config | **Client MCP registrations** written by older `helix setup` (pre-Phase-93). Phase 93-03 already flipped `setup` to skill+hooks install with MCP-only teardown across all 7 clients (STATE line 129). Any agent still configured with a stdio `helix --mode=stdio` MCP server entry will break post-deletion. | Phase 93's teardown handles the migration; verify no client config doc/sample still tells users to register `--mode=stdio`. (Doc rewrite is Phase 95.) |
| OS-registered state | None — no Task Scheduler / launchd / systemd unit embeds the transport mode. The daemon is auto-spawned by `startDaemon` (`dial.go:139`) with `--serve --socket=… --http-addr=` (HTTP already disabled on auto-start). | None — and note `startDaemon` passes `--http-addr=` (`dial.go:151`); that arg disappears when the flag is removed. Update `startDaemon`'s exec args. |
| Secrets/env vars | `HELIX_SOCKET` (read by the CLI for socket override; retained). `OTEL_EXPORTER_OTLP_ENDPOINT` (forwarder tracing; the forwarder process is deleted but the env var is read elsewhere too). No transport-mode env var. | Confirm `HELIX_SOCKET` path is retained; the new TCP opt-in may add a `HELIX_GRPC_ADDR`/config key (Claude's discretion). |
| Build artifacts | None persistent. `internal/forwarder` package stays (dial code retained), so no egg-info/binary equivalent goes stale. | `go build` regenerates; nothing to reinstall. |

**The canonical question — after every file is updated, what still references the deleted heads?** Answer: only (a) the `--mode`/`--http-addr` flag help text + `DaemonConfig.HTTPAddr` config field + `daemon.http_addr` default (`config/defaults.go:30`), (b) `startDaemon`'s `--http-addr=` exec arg (`dial.go:151`), (c) the HTTP-transport test harnesses (`test/harness/runner.go:67-84` `NewHTTPSession`, `test/integration/harness.go:89`) and their callers (`test/oracle/protocol/handshake_test.go`, `reconnect_test.go`, `test/integration/smoke_http_test.go`). All are listed in Common Pitfalls.

## Common Pitfalls

### Pitfall 1: Deleting shared dial code with the stdio head
**What goes wrong:** `RunForwarder` and `CallTool` (the retained CLI path) sit in the same package and share `ConnectOrStartDaemon`, `StreamMCP`, `generateSessionID`. A coarse "delete the forwarder" removes the CLI's dial path.
**Why it happens:** Package name `forwarder` suggests the whole package is the head.
**How to avoid:** Delete ONLY `internal/forwarder/forwarder.go` (the stdin/stdout pump) and the `--mode=stdio` route in `root.go`. Keep `dial*.go`, `grpc_client_transport*.go`, `oneshot.go`. Move/keep `generateSessionID` (used by `oneshot.go:53`).
**Warning signs:** `go build ./cmd/helix` fails with "undefined: ConnectOrStartDaemon" or `CallTool` errors.

### Pitfall 2: `RunStdio` looks live but is dead; `HTTPHandler` looks dead but is live in tests
**What goes wrong:** Assuming symmetry. `RunStdio` (`internal/mcp/server.go:191`) has ZERO callers (verified: grep shows only its own definition) — safe to delete. `HTTPHandler` (`server.go:195`) has THREE callers: `listenHTTP` (delete), `test/harness/runner.go:70`, `test/integration/harness.go:89` (must be re-targeted/removed first).
**Why it happens:** The stdio path goes through the forwarder+gRPC, never `RunStdio`; the HTTP path is exercised by integration tests beyond `listenHTTP`.
**How to avoid:** Delete `RunStdio` freely. Before deleting `HTTPHandler`, first re-target `NewHTTPSession` (`runner.go:67`) and `test/integration/harness.go:89` onto the gRPC `GRPCTransport` path (or remove the HTTP-only protocol oracle sub-tests).
**Warning signs:** Build failures in `test/harness`, `test/integration`, `test/oracle/protocol` after removing `HTTPHandler`.

### Pitfall 3: HTTP-transport oracle tests break the build
**What goes wrong:** `test/oracle/protocol/{handshake,reconnect}_test.go` call `runner.NewHTTPSession(t)`; `test/integration/smoke_http_test.go` hits the HTTP endpoint. All are `//go:build integration || llm || llmjudge`-tagged, so a plain `go test ./...` is GREEN even with them broken — a false-green trap (cf. MEMORY: helix-integration-tagged-suite-preexisting-failures).
**Why it happens:** The load-bearing gate is the untagged suite; tagged suites are not run by default.
**How to avoid:** Re-point `NewHTTPSession` to a `NewGRPCSession` over `GRPCTransport` (the in-memory or unix path), OR delete the HTTP-specific protocol sub-tests. Run `go build -tags integration ./...` AND `go vet -tags integration ./...` as an explicit verify step so tagged-file breakage surfaces. `TestHandshake_InMemory` (`handshake_test.go:17`) already uses an in-memory transport and is unaffected.
**Warning signs:** `go test ./...` green but `go build -tags integration ./...` red.

### Pitfall 4: `--http-addr` removal leaves dangling references
**What goes wrong:** `--http-addr` is read in `runDaemon` (`root.go:237,252-254`), set as a default (`config/defaults.go:30`), stored in `DaemonConfig.HTTPAddr` (`config/config.go:104-105`), logged (`daemon.go:1336`, `:1412`), and passed by `startDaemon` as `--http-addr=` (`dial.go:151`). Remove the flag without the rest → "unknown flag" on the auto-spawned daemon (`startDaemon` passes a now-undefined flag) → 10s cold-start timeout.
**Why it happens:** The flag is threaded through five sites.
**How to avoid:** Remove all of: the flag declaration (`root.go:127`), the `--mode http`/`serve||mode=="http"` arms (`root.go:184`), the `http-addr` override (`root.go:252-254`), `DaemonConfig.HTTPAddr` (`config.go:104-105`), `daemon.http_addr` default (`defaults.go:30`), the `http_addr` log fields (`daemon.go:1336`,`:1412`), and the `--http-addr=` arg in `startDaemon` (`dial.go:151`). The `--mode` flag itself may stay (only `stdio`/`http` arms removed) or be removed entirely — Claude's discretion, but `runRoot`'s `mode` switch must remain coherent.
**Warning signs:** Auto-started daemon dies immediately; `helix <verb>` cold call hangs ~10s then "daemon did not start". Check the `.daemon.log` beside the socket (`dial.go:134`).

### Pitfall 5: Windows local-dial regression
**What goes wrong:** The unix-socket dial is `//go:build !windows` in tests (`cli_e2e_test.go:1`). Removing heads must not break the Windows named-pipe/local dial path. `detachFromProcessGroup` is Windows-specific (`dial_windows.go`).
**How to avoid:** Keep `dial_windows.go` / `dial_unix.go`. Add a Windows local-dial smoke (success criterion 2). The TCP opt-in (RETIRE-04) should also help Windows where unix sockets are flaky.
**Warning signs:** CI Windows job red; `go build` on Windows fails.

### Pitfall 6: Parity test landing in the deletion commit (RETIRE-03 violation)
**What goes wrong:** Writing the dual-run parity test and the deletion in one commit means there is no commit where parity is green AND both heads still exist.
**How to avoid:** Two plans, two commits, ordered: parity commit (heads present, test green) THEN deletion commit. The plan dependency graph must enforce this — see Validation Architecture / wave structure.
**Warning signs:** A reviewer cannot point to a single commit that proves "CLI == MCP path with both heads alive."

## Code Examples

### Removing the stdio route (root.go) while keeping `auto`
```go
// Source: internal/cli/root.go:181-202 (current). After RETIRE-01/02:
//   - drop `serve || mode == "http"` → runDaemonFn (or keep --serve, drop http)
//   - drop the `case "stdio"` arm entirely (runForwarder/runForwarderFn deleted)
//   - keep `case "auto": return cmd.Help()` (CLI-04)
serve, _ := cmd.Flags().GetBool("serve")
if serve {
    return runDaemonFn(cmd)   // --serve still starts the daemon directly
}
return cmd.Help()             // bare `helix` prints grouped help (CLI-04)
```

### Deleting the HTTP listener launch (daemon.go)
```go
// Source: internal/daemon/daemon.go:1316-1321 (DELETE this whole block):
// if d.config.Daemon.HTTPAddr != "" {
//     g.Go(func() error { return d.listenHTTP(gctx) })
// }
// Then delete listenHTTP (1399-1431) and the http_session_middleware.go file.
// listenSocket (1354) — the gRPC-over-unix listener the CLI dials — STAYS.
```

### Adding the gated TCP gRPC bind (RETIRE-04, new, mirrors listenSocket + validateAdminAddr)
```go
// New: internal/daemon — launch alongside listenSocket, gated like listenAdmin.
// daemon.Run:  if d.config.Daemon.GRPCAddr != "" { g.Go(func() error { return d.listenGRPCTCP(gctx) }) }
func (d *Daemon) listenGRPCTCP(ctx context.Context) error {
    addr := d.config.Daemon.GRPCAddr
    if addr == "" { return nil }                       // disabled (default)
    if err := validateGRPCAddr(addr); err != nil { return err }  // loopback-only unless opted in
    ln, err := net.Listen("tcp", addr)
    if err != nil { return fmt.Errorf("listen grpc tcp %s: %w", addr, err) }
    // reuse the SAME grpc.Server registration as listenSocket (RegisterForwarderServiceServer)
    // and the SAME GracefulStop-on-ctx.Done pattern (daemon.go:1390-1396).
    ...
}
// CLI side: extend tryConnect (dial.go:91) to dial "tcp://host:port" when configured,
// defaulting to "unix://"+socketPath. grpc.NewClient already accepts both schemes.
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Agent connects via stdio MCP or HTTP `/mcp` | Agent uses `helix <verb>` CLI over one-shot gRPC `tools/call` | v2.0 Phases 90–93 (CLI proven, taught, set-up) | Phase 94 removes the now-redundant heads |
| `helix setup` registers an MCP server | `helix setup` installs skill + hooks, tears down MCP registration | Phase 93-03 (STATE line 129) | Migration already shipped; deletion is safe |
| MCP-only HTTP transport for remote/multi-client | Gated gRPC-TCP opt-in + ADR; full remote deferred to REMOTE-01/02 | Phase 94 (RETIRE-04) | One network-transparent topology replaced; multi-client fan-out deferred |

**Deprecated/outdated after this phase:**
- `RunForwarder` / `forwarder.go` stdin-stdout pump — removed.
- `listenHTTP` / `HTTPHandler` / `http_session_middleware.go` / `/mcp` — removed.
- `RunStdio` (`internal/mcp/server.go:191`) — already dead, removed.
- `--mode http`, `--http-addr`, `DaemonConfig.HTTPAddr`, `daemon.http_addr` default — removed.
- `httpSessionMiddleware` + its test file — removed (HTTP-only).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `RunStdio` (`server.go:191`) has zero callers and is safe to delete | RETIRE-01, Pitfall 2 | LOW — grep across the tree shows only its own definition; deleting it and rebuilding will catch any reflective/test caller |
| A2 | The "MCP path" for RETIRE-03 parity is acceptably represented by `forwarder.CallTool` over gRPC (the retained path) compared to the CLI subprocess, since both deleted heads funnel through the identical `mcpServer.SDK()` dispatch | RETIRE-03 | MEDIUM — if the gate intends to compare specifically against the stdio/HTTP heads BEFORE deletion, the parity plan must capture a reference through `RunForwarder`/`HTTPHandler` while they still exist (do this in the parity commit, where heads are alive). Recommend capturing the reference via the HTTP `NewHTTPSession` harness (still present pre-deletion) for maximum fidelity. |
| A3 | "5 middlewares" in roadmap prose maps to 6 installed receiving middlewares today (Telemetry, ProfileFilter, Suggestion, Guardrail, ProfileEnforcement, LazyInit) | Architecture map | LOW — all are installed on `mcpServer.SDK()` and untouched by transport deletion; count discrepancy is cosmetic |
| A4 | The IN-01 `mergeJSONConfig` dead code (`setup_clients.go:60`) is genuinely unused in production (only referenced by `setup_test.go`) and may be removed here | Deletion mechanics | MEDIUM — it has tests; confirm no live `setup` path calls it before removing. It is OUT of the strict RETIRE scope; removing it is a discretionary cleanup, gate behind its own task so it does not entangle the head deletion. |
| A5 | The protocol oracle HTTP sub-tests can be re-pointed to `GRPCTransport`/in-memory rather than requiring deletion | Pitfall 3 | LOW — `TestHandshake_InMemory` already exercises in-memory; the gRPC `GRPCTransport` provides an equivalent server-side bridge |
| A6 | `git diff api/proto/` will be empty (no proto change needed for the TCP bind — it reuses `StreamMCP`) | Zero-proto invariant | LOW — the TCP listener registers the SAME `ForwarderService`; no new RPC/message |

## Open Questions

1. **Does RETIRE-03's parity reference need to come through a deleted head specifically?**
   - What we know: The dual-run harness (`cli_e2e_test.go`, `golden_test.go`) compares CLI subprocess vs `forwarder.CallTool` (gRPC). Both old heads dispatch into the same SDK.
   - What's unclear: Whether "compares CLI output against the pre-removal MCP path" means against the gRPC path (retained) or specifically the stdio/HTTP heads.
   - Recommendation: In the parity commit (heads still alive), capture the reference via the HTTP `NewHTTPSession` harness AND assert CLI parity — maximally faithful to "pre-removal MCP path." This costs nothing since the harness exists and is deleted only in the next commit.

2. **Keep `--mode` flag or remove it entirely?**
   - What we know: After removing `stdio`/`http` arms, only `auto` remains, which equals bare-help.
   - Recommendation: Keep `--serve` (direct daemon), remove `--mode` and `--http-addr` to eliminate dead flags and help-text references; ensure no test asserts `--mode` exists. (Claude's discretion.)

3. **Config key name for the TCP bind.**
   - Recommendation: `daemon.grpc_addr` (mirrors `daemon.socket_path`/`daemon.http_addr` naming) with an empty default; CLI flag `--grpc-addr`; loopback-gated like `--admin-addr`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test | ✓ | (repo go.mod) | — |
| `gopls` | LS-backed dual-run parity verbs (go-to-definition etc.) | ✓ (per MEMORY/SMTC env: gopls full) | — | Use non-LS verbs (`search-in-files`) for parity if absent; `harness.RequireGopls` skips cleanly |
| `golang.org/x/tools/cmd/deadcode` | dead-code sweep (optional) | likely via `go run` | — | `go vet` + compiler errors |

**Missing dependencies with no fallback:** None — this is a deletion phase using the existing toolchain.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (stdlib) + `testify` (`github.com/stretchr/testify`) |
| Config file | none — `go test`; tagged suites via `-tags integration\|llm\|llmjudge` |
| Quick run command | `go build ./cmd/helix && go vet ./... && go test ./internal/cli/... ./internal/forwarder/... ./internal/daemon/... ./internal/mcp/...` |
| Full suite command | `go test ./...` then (false-green guard) `go build -tags integration ./... && go vet -tags integration ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RETIRE-03 | Dual-run parity: CLI verb stdout locus == pre-removal MCP-path locus for a representative tool set | integration (HELIX_BIN-gated) | `HELIX_BIN=$(pwd)/helix go test -tags integration -run TestCLI_DualRunParity -count=1 ./internal/cli/...` | ❌ Wave 1 — extend `cli_e2e_test.go:234` `TestCLI_E2E_OneShot` into a multi-verb parity test |
| RETIRE-01 | No reachable stdio MCP server path; CLI still dials | unit + build | `go build ./cmd/helix` (RunForwarder gone) + `go test ./internal/cli/... -run TestRunRoot` | ⚠️ `root_test.go` exists; update to assert `stdio` route is gone |
| RETIRE-01 | `helix --mode=stdio` no longer opens a stdio MCP session | integration | new sub-test: `helix --mode=stdio` errors/help, not a session | ❌ Wave 2 |
| RETIRE-02 | `/mcp` endpoint gone; `--mode http` no longer serves MCP | integration | new: assert no listener on the (removed) http addr; `--mode http` errors | ❌ Wave 2 — replaces `test/integration/smoke_http_test.go` |
| RETIRE-02 | Daemon still serves over gRPC unix socket | integration | retained `TestCLI_E2E_OneShot` (gRPC path) green post-deletion | ✅ `internal/cli/cli_e2e_test.go` |
| RETIRE-04 | CLI targets a configured TCP daemon endpoint; default stays unix | integration | new: `validateGRPCAddr` unit (loopback ok / non-loopback refused) + a tcp-dial round-trip | ❌ Wave 1 — mirror `telemetry_test.go` admin-addr tests |
| RETIRE-04 | Windows local-dial smoke | integration (windows) | `go test ./internal/forwarder/...` on the windows CI runner | ⚠️ add a `//go:build windows` smoke |
| (invariant) | `git diff api/proto/` empty | verify step | `git diff --exit-code api/proto/` | ❌ add to phase verify |
| (invariant) | zero new deps | verify step | `git diff --exit-code go.mod go.sum` | ❌ add to phase verify |

### Sampling Rate
- **Per task commit:** `go build ./cmd/helix && go vet ./...` (catches orphaned refs immediately — the primary risk in a delete phase).
- **Per wave merge:** `go test ./...` + `go build -tags integration ./...` (false-green guard for the HTTP oracle re-target).
- **Phase gate:** full suite green + `HELIX_BIN`-gated dual-run parity green in the pre-deletion commit + `git diff --exit-code api/proto/ go.mod go.sum`.

### Wave 0 Gaps
- [ ] `internal/cli/cli_e2e_test.go` — extend `TestCLI_E2E_OneShot` into `TestCLI_DualRunParity` over a representative verb set (RETIRE-03); capture the MCP reference via the still-present HTTP/forwarder path in the parity commit (A2/Q1).
- [ ] `internal/daemon/grpc_tcp_test.go` (or extend `telemetry_test.go` pattern) — `validateGRPCAddr` loopback-gate unit + a tcp round-trip (RETIRE-04).
- [ ] `internal/forwarder/dial_tcp_test.go` or a `//go:build windows` local-dial smoke (success criterion 2).
- [ ] Re-target `test/harness/runner.go` `NewHTTPSession` → a gRPC/in-memory session; fix `test/oracle/protocol/{handshake,reconnect}_test.go` and delete/replace `test/integration/smoke_http_test.go` (Pitfall 3).

## Security Domain

> `security_enforcement` is not explicitly false; treated as enabled. This phase is security-relevant because it ADDS a network listener (RETIRE-04).

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture | yes | Reduce attack surface: deleting the HTTP `/mcp` head removes an unauthenticated network endpoint (`:8080` default). Net security improvement. |
| V2 Authentication | yes (deferred) | The new TCP bind has NO auth — RETIRE-04 GATES it loopback-only; non-loopback refused with an error pointing at REMOTE-01 (mirrors `validateAdminAddr`). Full auth is explicitly out of scope. |
| V4 Access Control | yes | Profile/mode enforcement (`ProfileEnforcementMiddleware`, SEC-01/02) stays on `mcpServer.SDK()` — retained for the gRPC path. Destructive verbs remain refused under read/ci-bot. |
| V5 Input Validation | yes | `validateGRPCAddr` validates the bind address (host:port, loopback) before `net.Listen`. |
| V9 Communications | yes (deferred) | Non-loopback gRPC would be plaintext today; gating to loopback avoids exposing cleartext IPC. mTLS deferred to REMOTE-01. |

### Known Threat Patterns for the new TCP bind
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unauthenticated remote tool execution if bound non-loopback | Elevation of Privilege / Tampering | Default-empty (unix-only); `validateGRPCAddr` refuses non-loopback (loopback-only opt-in); ADR documents the remote scope as deferred |
| Information disclosure over plaintext TCP | Information Disclosure | Loopback-only confines traffic to the host; document the no-TLS caveat in the ADR + flag help |
| Removing the only auth-less HTTP head while the daemon stays up | (positive) | Attack surface shrinks; assert no listener binds the old http addr post-deletion |

## Sources

### Primary (HIGH confidence) — codebase, verified via Read/grep this session
- `internal/cli/root.go:108-202` — `--mode`/`--http-addr`/`--admin-addr` flags; `runRoot` mode switch; `runForwarder`/`runDaemon` [VERIFIED]
- `internal/forwarder/forwarder.go:22` (`RunForwarder` — the stdio head) + `oneshot.go:32` (`CallTool` — retained) + `dial.go:35,91,139,151` [VERIFIED]
- `internal/mcp/server.go:191` (`RunStdio` dead) + `:195-200` (`HTTPHandler`) [VERIFIED]
- `internal/daemon/daemon.go:1316-1431` (`listenHTTP`, `/mcp` mount), `:1354-1397` (`listenSocket`, retained), `:1481-1531` (`StreamMCP`, retained) [VERIFIED]
- `internal/daemon/http_session_middleware.go` (HTTP-only middleware to delete) [VERIFIED]
- `internal/daemon/telemetry.go:40-121` (`listenAdmin`/`validateAdminAddr` — RETIRE-04 template) [VERIFIED]
- `internal/config/config.go:64-108` (`ObservabilityConfig.AdminAddr`, `DaemonConfig.HTTPAddr`) + `defaults.go:30,37` [VERIFIED]
- `api/proto/serena/v1/ipc.proto:9-21` (`ForwarderService.StreamMCP` retained) [VERIFIED]
- `internal/cli/cli_e2e_test.go:234-278` (dual-run shape) + `test/oracle/contract/golden_test.go` (subprocess golden harness) + `cli_parity_test.go` (static verb/registry parity) [VERIFIED]
- `test/harness/runner.go:67-84` (`NewHTTPSession`) + `test/oracle/protocol/{handshake,reconnect}_test.go` + `test/integration/smoke_http_test.go` (HTTP-transport test fallout) [VERIFIED]
- grep: `RunStdio` zero callers; `RunForwarder` one caller (`root.go:230`); `HTTPHandler` three callers (listenHTTP + 2 test harnesses); `mergeJSONConfig` referenced only by tests [VERIFIED]

### Secondary (MEDIUM confidence)
- CLAUDE.md Layer 0 MCP Runtime description; STATE.md Critical Roadmap Constraints (delete heads last, RETIRE-03 gate, RETIRE-04 rides phase) [CITED]
- MEMORY: helix-mcp-to-cli-skill-migration (locked pivot decisions), helix-integration-tagged-suite-preexisting-failures (false-green trap), helix-tool-docs-drift (docgen blank-imports == daemon's) [CITED]

### Tertiary (LOW confidence)
- None — all claims anchored to codebase reads.

## Metadata

**Confidence breakdown:**
- Delete/keep boundary (heads vs wire): HIGH — every symbol's callers verified via grep; the head functions and the retained dial path are in distinct files.
- RETIRE-04 pattern: HIGH — exact admin-addr analog exists and was read in full.
- RETIRE-03 dual-run harness: HIGH for existence/shape; MEDIUM on whether the reference must come specifically through a deleted head (A2/Q1 — resolved by capturing the reference in the pre-deletion commit).
- Test fallout scope: HIGH — all `HTTPHandler`/`NewHTTPSession` callers enumerated.

**Research date:** 2026-06-21
**Valid until:** 2026-07-21 (stable; codebase-internal, no fast-moving external deps)

## RESEARCH COMPLETE
