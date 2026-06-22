# Phase 94: Retire the Agent-Facing MCP Surface (DELETE) - Pattern Map

**Mapped:** 2026-06-22
**Files analyzed:** 12 source files + 5 test files (delete/keep/mirror seams)
**Analogs found:** 6 / 6 (all anchored to file:line, verified this session via Read + grep)

> This is a DELETE phase. The unit of work is not "a new file copying a pattern"
> but "an exact seam — delete THIS, keep THAT, mirror THIS." Every row below is
> anchored to a verified file:line. The load-bearing risk is deleting one line
> too many inside the shared `internal/forwarder` package, so the keep-list is as
> important as the delete-list.

## Seam Classification

| Change | Role | Data Flow | Action | Anchor (file:line) | Verified |
|--------|------|-----------|--------|--------------------|----------|
| `RunForwarder` stdio pump | forwarder/transport-head | streaming (stdin/stdout↔gRPC) | DELETE | `internal/forwarder/forwarder.go:22-131` | ✅ 1 caller (root.go:230) |
| `isToolsCall` / `sendWithSpan` | forwarder helpers | streaming | DELETE | `internal/forwarder/forwarder.go:149-176` | ✅ forwarder-only |
| `generateSessionID` | forwarder helper | utility | MOVE/KEEP (not delete) | `internal/forwarder/forwarder.go:141-147` | ✅ also used by oneshot.go:53 |
| `--mode=stdio` route arm | cli/route | request-response | DELETE arm | `internal/cli/root.go:188-201` | ✅ |
| `runForwarder`/`runForwarderFn` | cli/route | request-response | DELETE (becomes unused) | `internal/cli/root.go:73,218-231` | ✅ |
| `listenHTTP` + `/mcp` mount | daemon/transport-head | request-response (HTTP) | DELETE | `internal/daemon/daemon.go:1399-1431` | ✅ |
| `listenHTTP` goroutine launch | daemon/wiring | event-driven | DELETE block | `internal/daemon/daemon.go:1316-1321` | ✅ |
| `HTTPHandler` | mcp/SDK-adapter | request-response | DELETE (after test re-target) | `internal/mcp/server.go:195-200` | ✅ 3 callers (1 prod + 2 test) |
| `httpSessionMiddleware` | daemon/middleware | request-response | DELETE whole file | `internal/daemon/http_session_middleware.go:10-34+` | ✅ HTTP-only |
| `RunStdio` (dead) | mcp/SDK-adapter | streaming | DELETE | `internal/mcp/server.go:190-193` | ✅ ZERO callers |
| `--http-addr` flag + threading | cli/config | config | DELETE all sites | root.go:127,237,252-254; config.go:104-105; defaults.go:30; daemon.go:1336,1412; dial.go:151 | ✅ |
| `CallTool` (CLI one-shot) | forwarder/dial-path | request-response | **KEEP** | `internal/forwarder/oneshot.go:32-87` | ✅ retained surface |
| `ConnectOrStartDaemon`/`tryConnect`/`startDaemon`/`waitForDaemon` | forwarder/dial-path | request-response | **KEEP** | `internal/forwarder/dial.go:35,91,139,179` | ✅ |
| `GRPCClientTransport` | forwarder/dial-path | streaming | **KEEP** | `internal/forwarder/grpc_client_transport.go` | ✅ used by oneshot.go:54 |
| `dial_unix.go`/`dial_windows.go`/`dial_lock.go` | forwarder/dial-path | utility | **KEEP** | (package files) | ✅ Windows local-dial |
| `listenSocket` (gRPC-over-unix) | daemon/wire | request-response | **KEEP** | `internal/daemon/daemon.go:1354-1397` | ✅ CLI dials this |
| `StreamMCP` handler / `forwarderServiceHandler` | daemon/wire | streaming | **KEEP** | `internal/daemon/daemon.go:1370,1481+` | ✅ proto-retained |
| 6 receiving middlewares | mcp/SDK | request-response | **KEEP** | attach to `mcpServer.SDK()` | ✅ transport-agnostic |
| `validateAdminAddr`/`listenAdmin` | daemon/listener | request-response | **MIRROR** (RETIRE-04) | `internal/daemon/telemetry.go:40-121` | ✅ template |
| `--admin-addr` flag wiring | cli/config | config | **MIRROR** (RETIRE-04) | root.go:132-134,240,258-262 | ✅ template |
| `TestCLI_E2E_OneShot` dual-run | cli/test | test | **EXTEND** (RETIRE-03) | `internal/cli/cli_e2e_test.go:234-280` | ✅ harness exists |
| `NewHTTPSession` harness | test/harness | test | **RE-TARGET** | `test/harness/runner.go:64-85`; `test/integration/harness.go:79-90` | ✅ 2 sites |
| HTTP oracle/smoke tests | test | test | **RE-TARGET or DELETE** | handshake_test.go (TestHandshake_HTTP), reconnect_test.go, smoke_http_test.go | ✅ integration-tagged |

---

## Seam 1: stdio head DELETE (RETIRE-01)

### DELETE — the stdin/stdout pump
**File:** `internal/forwarder/forwarder.go:22-131` (`RunForwarder`)

This is the ONLY agent-facing stdio head. It reads `os.Stdin` (`forwarder.go:66`),
frames each line as `serenav1.MCPMessage`, sends over `client.StreamMCP`, and writes
`os.Stdout` (`forwarder.go:121`). Its only caller is `root.go:230` (via the
`--mode=stdio` arm). Delete the whole function plus the forwarder-only helpers:

- `isToolsCall` (`forwarder.go:159-162`) + `toolsCallMethod`/`toolsCallMethodSpaced` vars (`:151,:154`)
- `sendWithSpan` (`forwarder.go:171-176`)

### MOVE, do NOT delete — `generateSessionID`
**File:** `internal/forwarder/forwarder.go:141-147`

```go
func generateSessionID() string {                     // forwarder.go:141
	b := make([]byte, 16)
	if _, err := io.ReadFull(cryptoRand.Reader, b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return fmt.Sprintf("%x", b)
}
```
`CallTool` depends on it (`oneshot.go:53: sessionID := generateSessionID()`).
Deleting `forwarder.go` wholesale orphans `CallTool`. Relocate `generateSessionID`
(and its `cryptoRand`/`io`/`fmt` imports) into `oneshot.go`, or keep a slim
`forwarder.go` containing only it. **Compiler proof:** after deletion
`go build ./cmd/helix` must succeed; an "undefined: generateSessionID" error means
the move was missed.

### DELETE — the route arm
**File:** `internal/cli/root.go:188-201`

```go
switch mode {
case "stdio":
	return runForwarderFn(cmd)   // root.go:192 — DELETE this arm
case "auto":
	return cmd.Help()            // root.go:198 — KEEP (CLI-04 bare-help)
default:
	return fmt.Errorf("unknown mode: %s", mode)
}
```
Removing the `stdio` arm makes `runForwarder` (`root.go:218-231`) and the
`runForwarderFn` seam var (`root.go:73`) unused — delete both. The `forwarder`
import in root.go for `RunForwarder` (root.go:230) goes; the `forwarder` import for
`CallTool` (used by the verb path / activate) STAYS.

### KEEP — the entire CLI dial path (the retained agent surface)
**Files (DO NOT DELETE):** `internal/forwarder/oneshot.go` (`CallTool`),
`internal/forwarder/dial.go` (`ConnectOrStartDaemon:35`, `tryConnect:91`,
`startDaemon:139`, `waitForDaemon:179`), `internal/forwarder/grpc_client_transport.go`
(`NewGRPCClientTransport`, used at `oneshot.go:54`), `dial_unix.go`, `dial_windows.go`,
`dial_lock.go`.

`CallTool` (`oneshot.go:32`) and `RunForwarder` share `ConnectOrStartDaemon` +
`client.StreamMCP` + `generateSessionID`. The SEAM is: **the stdin/stdout pump
(`forwarder.go`) is the head; the dial helpers are the wire.** Package name
`forwarder` is misleading — only one file in it is the deleted head.

---

## Seam 2: HTTP `/mcp` head DELETE (RETIRE-02)

### DELETE — the listener + mount
**File:** `internal/daemon/daemon.go:1399-1431` (`listenHTTP`)

```go
mux.Handle("/mcp", httpSessionMiddleware(d.mcpServer.HTTPHandler(), d.obs.Metrics()))  // daemon.go:1405
```

### DELETE — the conditional launch
**File:** `internal/daemon/daemon.go:1316-1321`

```go
// HTTP listener for Streamable HTTP MCP (DMN-04, MCP-02)
if d.config.Daemon.HTTPAddr != "" {
	g.Go(func() error {
		return d.listenHTTP(gctx)
	})
}
```

### DELETE — the SDK HTTP adapter (after test re-target — see Seam 6)
**File:** `internal/mcp/server.go:195-200` (`HTTPHandler`). 3 callers verified:
`daemon.go:1405` (deleted here), `test/harness/runner.go:70`,
`test/integration/harness.go:89` (re-target first — Pitfall in Seam 6).

### DELETE — the HTTP-only middleware (whole file)
**File:** `internal/daemon/http_session_middleware.go` (`httpSessionMiddleware:34`).
Used only by `listenHTTP:1405`. Delete the file and its `_test.go` sibling if present.

### DELETE — `--http-addr` thread (all sites; missing one = 10s cold-start hang)
- `internal/cli/root.go:127` — flag declaration (`String("http-addr", ":8080", ...)`)
- `internal/cli/root.go:184` — `if serve || mode == "http"` → reduce to `if serve` (drop the http arm)
- `internal/cli/root.go:237` — `httpAddr, _ := cmd.Flags().GetString("http-addr")`
- `internal/cli/root.go:252-254` — the `daemon.http_addr` override block
- `internal/config/config.go:104-105` — `DaemonConfig.HTTPAddr` field
- `internal/config/defaults.go:30` — `"daemon.http_addr": ":8080"` default
- `internal/daemon/daemon.go:1336` — `"http_addr", d.config.Daemon.HTTPAddr` log field
- `internal/daemon/daemon.go:1412` — `"addr", d.config.Daemon.HTTPAddr` log field (inside deleted listenHTTP)
- `internal/forwarder/dial.go:151` — **load-bearing:** `exec.Command(exe, "--serve", "--socket="+socketPath, "--http-addr=")`. Drop the `--http-addr=` arg; leaving it after the flag is removed makes the auto-spawned daemon die with "unknown flag" → 10s `waitForDaemon` timeout (dial.go:198). Update the comment block (dial.go:145-150) too.

### KEEP — the gRPC-over-unix wire the CLI dials
**File:** `internal/daemon/daemon.go:1354-1397` (`listenSocket`). It registers
`serenav1.RegisterForwarderServiceServer` (daemon.go:1370) and `GracefulStop`s on
`ctx.Done()` (daemon.go:1390-1396). The `StreamMCP` handler (daemon.go:1481+) and
`forwarderServiceHandler` stay fully intact. Both deleted heads AND `CallTool`
dispatch into the SAME `mcpServer.SDK()` with all 6 middlewares — the middlewares
attach to the SDK, not the transport, so this deletion does not touch them.

> **`--mode` flag decision (Q2):** after removing the `stdio`+`http` arms, only
> `auto` (=bare help) remains. Discretion: either keep `--mode` with just `auto`,
> or remove `--mode` entirely and keep only `--serve`. Whichever — `runRoot`'s
> switch (root.go:188) must stay coherent and no test may assert a removed arm.

---

## Seam 3: Dead code — `RunStdio` (RETIRE-01)

**File:** `internal/mcp/server.go:190-193`

```go
// RunStdio runs the MCP server over stdio transport (MCP-01).
func (s *SerenaMCPServer) RunStdio(ctx context.Context) error {
	return s.sdk.Run(ctx, &mcpsdk.StdioTransport{})
}
```
**Verified ZERO callers** (grep shows only the definition). The stdio path always
went through the forwarder + gRPC, never this SDK `StdioTransport`. Delete freely;
the compiler confirms no orphan.

---

## Seam 4: RETIRE-04 TCP-bind — MIRROR the admin-addr loopback gate

### The template to copy verbatim
**File:** `internal/daemon/telemetry.go:108-121` (`validateAdminAddr`)

```go
func validateAdminAddr(addr string) error {                 // telemetry.go:108
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
Build `validateGRPCAddr` as a line-for-line analog. Change only the error text to
point at REMOTE-01 (not "v1.3"). Do NOT hand-roll a host allowlist —
`net.IP.IsLoopback()` is the one source of truth.

### The listener gate to mirror
**File:** `internal/daemon/telemetry.go:40-103` (`listenAdmin`) + launch at
`daemon.go:1325-1332`.

```go
func (d *Daemon) listenAdmin(ctx context.Context) error {   // telemetry.go:40
	addr := d.config.Observability.AdminAddr
	if addr == "" { return nil }              // disabled (default) — MIRROR THIS
	if err := validateAdminAddr(addr); err != nil { return err }
	ln, err := net.Listen("tcp", addr)
	...
	// graceful shutdown via server.Shutdown on ctx.Done() (telemetry.go:95-102)
}
```
Build `listenGRPCTCP(ctx)`: empty-addr → no-op; validate (loopback-only); then
`net.Listen("tcp", addr)` and reuse the SAME `grpc.Server` registration pattern as
`listenSocket` (`RegisterForwarderServiceServer`, daemon.go:1370) and the SAME
`GracefulStop()`-on-`ctx.Done()` shutdown (daemon.go:1390-1396 — NOT the HTTP
`server.Shutdown` form, since this is gRPC). Launch it gated:
`if d.config.Daemon.GRPCAddr != "" { g.Go(func() error { return d.listenGRPCTCP(gctx) }) }`
alongside the `listenSocket` launch (daemon.go:1312-1314).

### Flag/config wiring to mirror
**File:** `internal/cli/root.go:132-134` (flag), `:240` (`adminAddr, _ := GetString`),
`:258-262` (the **non-empty-only** override — copy this guard exactly so a blank
invocation cannot wipe a project config value):

```go
if adminAddr != "" {                                  // root.go:260
	overrides["observability.admin_addr"] = adminAddr
}
```
Add `--grpc-addr` flag (empty default), `DaemonConfig.GRPCAddr` (config.go, mirror
the deleted `HTTPAddr` field slot), `daemon.grpc_addr` default = `""` (defaults.go),
and the non-empty override guard. (Recommended key `daemon.grpc_addr` per Research Q3.)

### CLI dial side — extend, don't replace
**File:** `internal/forwarder/dial.go:91-127` (`tryConnect`). It currently hard-codes
unix:

```go
testConn, err := net.DialTimeout("unix", socketPath, 2*time.Second)   // dial.go:98
conn, err := grpc.NewClient("unix://"+socketPath, ...)                // dial.go:104
```
`grpc.NewClient` already accepts `tcp://host:port`. Branch on a configured TCP
endpoint: when set, skip the `os.Stat`/unix `net.DialTimeout` liveness probe
(dial.go:93-102, which is unix-file-specific) and `grpc.NewClient("tcp://"+addr)`
with the same keepalive + `obs.ClientStatsHandler(tp)` options. Default stays unix.

---

## Seam 5: RETIRE-03 dual-run parity — EXTEND the existing harness

### The shape already present (extend, do not rebuild)
**File:** `internal/cli/cli_e2e_test.go:234-280` (`TestCLI_E2E_OneShot`)

```go
f.mcpActivate(t, ctx)
reference := f.mcpSearch(t, ctx)                          // MCP path (forwarder.CallTool over gRPC)
out, _ := f.runCLIVerb(ctx, representativeVerbName, representativeVerbFlag+searchPattern) // CLI subprocess
// extract (relpath, line) from each; assert cliRel==refLoc[1] && cliLine==refLoc[2]  (cli_e2e_test.go:277-279)
```
This already runs one verb (`search_in_files`) through BOTH the MCP path and the
real `helix` subprocess against ONE daemon, comparing the load-bearing locus
(relpath:line) in the frozen terse shape. Generalize into `TestCLI_DualRunParity`
over a representative verb set (e.g. `search_in_files`, `find_references`,
`goto_definition`, `list_file_outline`, `read_file`), table-driven.

**Sequencing invariant (RETIRE-03, CONTEXT line 14):** this test lands and is GREEN
in its OWN commit, with both heads STILL ALIVE, BEFORE the deletion commit. Two
plans / two waves: Wave 1 = parity test + RETIRE-04 TCP bind (green); Wave 2 =
deletion + test re-target. Never the same commit (Pitfall 6).

**Fidelity option (A2/Q1):** for maximal "pre-removal MCP path" fidelity, in the
parity commit you MAY capture the reference via the still-present `NewHTTPSession`
harness (`runner.go:67`) as well — it costs nothing because the harness is deleted
only in the next commit. The retained gRPC `forwarder.CallTool` is the minimum;
both old heads funnel through the identical `mcpServer.SDK()`, so gRPC is
representative.

### Subprocess golden analog (secondary)
**File:** `test/oracle/contract/golden_test.go:158-165` — brings up a daemon and
calls `forwarder.CallTool` for activate. Same dual harness shape if you prefer the
oracle layer for some verbs.

---

## Seam 6: HTTP/stdio test fallout — RE-TARGET before deleting `HTTPHandler`

> **Pitfall (false-green):** every file below is `//go:build integration` (or
> `llm`/`llmjudge`)-tagged. A plain `go test ./...` is GREEN even when they no
> longer compile. Add an explicit `go build -tags integration ./... && go vet
> -tags integration ./...` verify step (cf. MEMORY: integration-tagged false-green).

### RE-TARGET — the HTTP session factory (2 sites)
**File:** `test/harness/runner.go:64-85` (`NewHTTPSession` → `MCPServer().HTTPHandler()`
at runner.go:70) and `test/integration/harness.go:79-90` (`NewHTTPSession` →
`MCPServer().HTTPHandler()` at harness.go:89).

Re-point both to a gRPC/in-memory session. The in-memory transport is already
proven — `StartRunner` wires `InMemoryTransports` (`runner.go:87+`) and
`runner.Session` is consumed by `TestHandshake_InMemory` (handshake_test.go:17).
Provide a `NewGRPCSession` over the retained `GRPCClientTransport` / `listenSocket`,
or have callers use the existing in-memory `runner.Session`. Only THEN delete
`HTTPHandler` (server.go:195).

### RE-TARGET or DELETE — the HTTP-specific oracle/smoke sub-tests
- `test/oracle/protocol/handshake_test.go` — `TestHandshake_InMemory` (line 17) **KEEP** (in-memory, unaffected); `TestHandshake_HTTP` (line ~52, calls `runner.NewHTTPSession(t)`) → re-target to gRPC/in-memory or delete.
- `test/oracle/protocol/reconnect_test.go` — uses `NewHTTPSession`; re-target or delete the HTTP-only reconnect case.
- `test/integration/smoke_http_test.go` — `TestHTTPSmoke_ToolCallRoundTrip` (line 15) is entirely HTTP-endpoint. Replace with a gRPC unix round-trip smoke (or rely on the retained `TestCLI_E2E_OneShot`), then delete this file.
- Note: `test/harness/runner.go:250` and `test/integration/harness.go:333` already set `cfg.Daemon.HTTPAddr = ""` for non-HTTP runs — once `HTTPAddr` is removed, drop those lines too.

---

## Shared Patterns / Invariants

### Loopback gate (single source of truth)
**Source:** `net.IP.IsLoopback()` + `validateAdminAddr` switch (`telemetry.go:113-120`).
**Apply to:** the new `validateGRPCAddr`. No custom allowlist.

### Graceful gRPC listener shutdown
**Source:** `listenSocket` `GracefulStop()`-on-`ctx.Done()` (`daemon.go:1390-1396`).
**Apply to:** `listenGRPCTCP` (it is gRPC, so use `GracefulStop`, NOT the HTTP
`server.Shutdown` form used by `listenAdmin`/`listenHTTP`).

### Non-empty-only config override guard
**Source:** `root.go:260-262` (`if adminAddr != "" { overrides[...] = ... }`).
**Apply to:** the `--grpc-addr` override (and verify the `--http-addr` removal does
not leave a dangling `cmd.Flags().Changed("http-addr")` at root.go:252).

### Zero-proto / zero-dep invariant (mechanical gate)
**Source:** `api/proto/serena/v1/ipc.proto` (`ForwarderService.StreamMCP` retained —
the TCP bind reuses the SAME RPC). Proto dir name `serena` is a wire-format lineage
artifact (CLAUDE.md / Phase 52-03) — do NOT rename.
**Apply to:** add `git diff --exit-code api/proto/` and `git diff --exit-code
go.mod go.sum` as phase verify steps.

### Compiler-as-verifier (delete-phase primary gate)
**Source:** Research "Don't Hand-Roll" + Pitfall 1.
**Apply to:** after EACH deletion task, `go build ./cmd/helix && go vet ./...`.
Orphaned refs (`undefined: ConnectOrStartDaemon`, `undefined: generateSessionID`,
`undefined: HTTPHandler`) surface immediately. Optional `deadcode ./...`.

---

## No Analog Found / Discretionary

| Item | Note |
|------|------|
| `mergeJSONConfig` (`setup_clients.go:60`) dead-code removal (A4) | Referenced only by `setup_test.go`. OUT of strict RETIRE scope — gate behind its own optional task so it does not entangle the head deletion. Confirm no live `setup` path calls it first. |
| Remote/multi-client scope ADR (RETIRE-04) | New doc, no code analog. Document: HTTP `/mcp` was the only network-transparent topology; replaced by gated gRPC-TCP (loopback default, non-loopback refused → REMOTE-01); plaintext, no auth, no multi-client fan-out (deferred). Mirror the security framing in 94-RESEARCH "Security Domain". |
| Windows local-dial smoke (success criterion 2) | No existing windows-tagged smoke in `internal/forwarder`. Add a `//go:build windows` test; keep `dial_windows.go` (`detachFromProcessGroup`) untouched. |

## Metadata

**Analog search scope:** `internal/forwarder/`, `internal/cli/`, `internal/daemon/`,
`internal/mcp/`, `internal/config/`, `test/harness/`, `test/integration/`,
`test/oracle/`.
**Files scanned:** forwarder.go, oneshot.go, dial.go, root.go, daemon.go (1300-1440),
telemetry.go (38-122), server.go (185-204), runner.go (60-90), handshake_test.go,
smoke_http_test.go, cli_e2e_test.go (234-280) + tree-wide grep for the 9 deleted symbols.
**Verification:** `RunStdio` zero callers; `RunForwarder` 1 caller (root.go:230);
`HTTPHandler` 3 callers (daemon.go:1405 + 2 test harnesses); `generateSessionID`
2 callers (forwarder.go + oneshot.go:53); `--http-addr`/`HTTPAddr` 9 sites enumerated.
**Pattern extraction date:** 2026-06-22
