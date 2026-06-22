# Phase 90: CLI One-Shot Dial Spine + Race-Free Warm Reuse - Pattern Map

**Mapped:** 2026-06-21
**Files analyzed:** 8 (6 new, 2 modified)
**Analogs found:** 8 / 8 (every new file has a strong in-tree analog; this phase is ~80% wiring of existing primitives)

## File Classification

| New/Modified File | New/Mod | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|---------|------|-----------|----------------|---------------|
| `internal/cli/verb.go` | NEW | command (cobra) | request-response | `internal/cli/activate.go` | exact (CLI cmd over `ConnectOrStartDaemon`) |
| `internal/cli/root.go` | MODIFY | command/router | request-response | self (`runRoot` lines 98-118) | exact (edit existing dispatch) |
| `internal/forwarder/oneshot.go` | NEW | service (IPC helper) | request-response | `internal/forwarder/forwarder.go` `RunForwarder` (lines 22-90) | role-match (stream wiring, one-shot vs streaming) |
| `internal/forwarder/grpc_client_transport.go` | NEW | transport (adapter) | streaming/bridge | `internal/mcp/grpc_transport.go` (lines 18-130) | exact-mirror (inverted io.Pipe ends) |
| `internal/forwarder/dial.go` | MODIFY | service (dial) | request-response | self (`ConnectOrStartDaemon` lines 24-40) | exact (add flock around spawn window) |
| `internal/forwarder/dial_lock.go` | NEW (optional) | utility | — | `internal/daemon/socket.go` `ensureSocket` + flock idiom | partial (path derivation + lock) |
| `internal/cli/verb_test.go` | NEW | test (unit, no daemon) | — | cobra command tests in `internal/cli` | role-match |
| `internal/forwarder/dial_race_test.go` | NEW | test (synctest) | event-driven | `internal/kernel/lspool/pool_synctest_test.go` (lines 30-60) | exact (synctest harness) |
| `internal/cli/cli_e2e_test.go` (or `test/integration/`) | NEW | test (E2E subprocess) | request-response | `bench/runtime/daemon_tap_integration_test.go` (lines 1-75) + `internal/eval/sandbox/sandbox.go` (lines 315+) | exact (HELIX_BIN-gated real daemon) |

---

## Pattern Assignments

### `internal/cli/verb.go` (cobra command, request-response) — NEW

**Analog:** `internal/cli/activate.go` (a working CLI command that already does `ConnectOrStartDaemon` → call → close).

**Command construction + flag binding pattern** (`activate.go:19-30`):
```go
func newActivateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "activate",
		Short:         "Activate workspace for current project",
		RunE:          runActivate,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().String("workspace", "", "Workspace directory (default: current directory)")
	return cmd
}
```
Copy the `SilenceUsage: true, SilenceErrors: true` convention (every command in `internal/cli` uses it). Register the new command in `root.go` via `rootCmd.AddCommand(...)` exactly like `activate.go` is registered at `root.go:89`.

**Connect-or-start + defer-close pattern** (`activate.go:48-57`) — REUSE VERBATIM, then wrap with the new one-shot helper:
```go
socketPath := config.DefaultSocketPath()
logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
// Pass noop tracer — a CLI command needs no OTel.
client, conn, err := forwarder.ConnectOrStartDaemon(cmd.Context(), socketPath, logger, noop.NewTracerProvider())
if err != nil {
	return fmt.Errorf("connecting to daemon: %w", err)
}
defer conn.Close()
```
Note: `activate.go` issues a *unary* gRPC call (`client.ActivateWorkspace`). The verb command instead issues a *one-shot `tools/call`* via the new `internal/forwarder/oneshot.go` helper + the MCP SDK client (see below) — DO NOT hand-frame JSON-RPC (anti-pattern, RESEARCH §Anti-Patterns).

**Default socket path** (`activate.go:48`): `config.DefaultSocketPath()` — the per-uid `os.TempDir()/helix-<uid>/daemon.sock` from `config/loader.go:64-76`.

---

### `internal/cli/root.go` (router, request-response) — MODIFY

**Analog:** itself. The single dispatch point is `runRoot` (lines 98-118).

**Current no-arg path** (`root.go:112-117`) — CLI-04 must change this:
```go
switch mode {
case "stdio", "auto":
	return runForwarder(cmd)   // ← CLI-04: no-arg must NOT enter stdio forwarder
default:
	return fmt.Errorf("unknown mode: %s", mode)
}
```
**Change:** when invoked with no verb/args, return grouped help and exit 0 (`cmd.Help()` / print usage) WITHOUT opening a stdio MCP session. Preserve `--serve`/`--mode=http` (→ `runDaemon`, lines 108-110) and explicit `--mode=stdio` paths so Phase 94 (full forwarder-head deletion) is not pre-empted. Use cobra `AddGroup` for grouped command help (cobra v1.10.2 supports it; RESEARCH Standard Stack).

**Subcommand registration block** (`root.go:86-93`) — add the new verb command here:
```go
rootCmd.AddCommand(newSetupCommand())
rootCmd.AddCommand(newStatusCommand())
rootCmd.AddCommand(newActivateCommand())
// ... add newVerbCommand() / newVerbDispatch() here ...
```

---

### `internal/forwarder/grpc_client_transport.go` (transport bridge, streaming) — NEW

**Analog:** `internal/mcp/grpc_transport.go` (lines 18-130) — the SERVER-side bridge. The new file is its CLIENT-side **mirror**: same io.Pipe-pair shape, opposite direction.

**Server-side data-flow doc + struct** to invert (`grpc_transport.go:18-40`):
```go
// GRPCTransport bridges a gRPC bidirectional stream to the MCP SDK's Transport interface.
// gRPC stream.Recv() -> pipeWriter (clientToServer) -> IOTransport.Reader -> MCP SDK
// MCP SDK -> IOTransport.Writer -> pipeReader (serverToClient) -> gRPC stream.Send()
type GRPCTransport struct {
	stream    GRPCStream
	sessionID string
	firstMsg  *serenav1.MCPMessage
}
```
For the CLIENT mirror: MCP SDK *client* WRITES JSON-RPC → `IOTransport.Writer` → goroutine → `stream.Send(MCPMessage{Payload, SessionId})`; `stream.Recv()` → goroutine → `IOTransport.Reader` → MCP SDK client READS. The CLI side has no `firstMsg` replay (the daemon-side replay at lines 54-64 exists because the daemon already consumed the first stream message; the client does not).

**Newline-delimited framing in/out** — copy both pump goroutines (`grpc_transport.go:50-121`), keeping the exact framing the IOTransport expects:
```go
// Recv → pipe (feeds SDK reader): write payload + "\n"
if _, err := clientWriter.Write(msg.Payload); err != nil { return }
if _, err := clientWriter.Write([]byte("\n")); err != nil { return }

// pipe → Send (drains SDK writer): split on '\n', Send each non-empty line
line := buf[:nlIdx]; buf = buf[nlIdx+1:]
if len(line) > 0 {
	t.stream.Send(&serenav1.MCPMessage{Payload: line, SessionId: t.sessionID})
}
```

**IOTransport wiring + Connect return** (`grpc_transport.go:123-129`):
```go
ioTransport := &mcpsdk.IOTransport{Reader: clientReader, Writer: serverWriter}
return ioTransport.Connect(ctx)
```
Reuse the `GRPCStream` interface (`grpc_transport.go:12-16`: `Recv()`/`Send()`) for testability — the CLI's `client.StreamMCP(ctx)` result satisfies it.

**Package-placement note (RESEARCH A4):** place this in `internal/forwarder` (NOT `internal/mcp`) to keep MCP-SDK-client wiring on the CLI side and avoid a forwarder→mcp import-cycle question. Confirm import direction during planning; worst case it lands in a new `internal/clirpc`.

---

### `internal/forwarder/oneshot.go` (one-shot IPC helper, request-response) — NEW

**Analog:** `internal/forwarder/forwarder.go` `RunForwarder` (lines 22-90) — the streaming forwarder; the one-shot helper is its single-call cousin.

**Stream-open pattern** (`forwarder.go:45-58`):
```go
client, conn, err := ConnectOrStartDaemon(ctx, socketPath, logger, tp)
if err != nil { return fmt.Errorf("connecting to daemon: %w", err) }
defer conn.Close()

stream, err := client.StreamMCP(ctx)
if err != nil { return fmt.Errorf("opening MCP stream: %w", err) }

sessionID := generateSessionID()   // forwarder.go:57, crypto/rand
```

**MCP SDK client connect + CallTool** (analog `test/integration/harness.go:176-206`) — the correct one-shot framing (does the `initialize` handshake the daemon's SDK server requires):
```go
client := mcp.NewClient(&mcp.Implementation{Name: "helix-cli", Version: cli.CurrentVersion()}, nil)
session, err := client.Connect(ctx, grpcClientTransport, nil) // wraps stream from StreamMCP
if err != nil { return err }
defer session.Close()   // Pitfall 3: clean close, else daemon counts an `error` session
res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: toolName, Arguments: argsMap})
if err != nil { return err }
if res.IsError { /* tool-level error */ }
```
Note `mcp.Implementation.Version` uses `cli.CurrentVersion()` (`root.go:38`) to keep CLI/MCP identity in lockstep. Teardown order: `defer session.Close()` THEN `defer conn.Close()` (Pitfall 3) so the SDK flushes before the gRPC conn drops.

---

### `internal/forwarder/dial.go` (dial service, request-response) — MODIFY

**Analog:** itself. The unlocked spawn window is `ConnectOrStartDaemon` lines 26-39.

**Current unlocked window** (`dial.go:26-39`) — the race (N cold callers each spawn a daemon):
```go
conn, client, err := tryConnect(ctx, socketPath, tp)
if err == nil { return client, conn, nil }           // warm reuse
// — NO LOCK HERE: N callers all reach startDaemon —
if err := startDaemon(socketPath); err != nil { ... }
return waitForDaemon(ctx, socketPath, 10*time.Second, tp)
```

**Change (CLI-03):** add a `gofrs/flock` guard on `<socketPath>.lock` around the spawn+wait window, with **double-checked `tryConnect`** after acquiring (Pitfall 1 — TOCTOU between the pre-lock probe and lock acquisition). Pattern (RESEARCH §Pattern 3):
```go
fl := flock.New(socketPath + ".lock")
if locked, _ := fl.TryLock(); !locked {
	_ = fl.Lock()              // peer is spawning; wait for ITS daemon
	defer fl.Unlock()
	if c, conn, err := tryConnect(ctx, socketPath, tp); err == nil { return c, conn, nil }
} else {
	defer fl.Unlock()
}
// double-check: a peer may have finished between first tryConnect and lock acquire
if c, conn, err := tryConnect(ctx, socketPath, tp); err == nil { return c, conn, nil }
// ... existing startDaemon + waitForDaemon ...
```
Keep `startDaemon` (lines 83-100) and `waitForDaemon` (lines 103-123) unchanged. Per-socket lockfile (RESEARCH Open-Q 2) so bench per-cell sockets don't serialize. Lock is a regular file (never `bind`ed) so it is not AF_UNIX-length-limited (Pitfall 2). The daemon's `net.Listen("unix")` bind (`daemon.go:1318`) remains the defense-in-depth backstop. Keep the lock portable — `gofrs/flock`, NOT `syscall.Flock` (Pitfall 6, Windows local-dial smoke).

**Discretion — keep the existing exported signature:** wrap the lock inside `ConnectOrStartDaemon` so existing callers (`activate.go:53`, `forwarder.go:45`, `runForwarder`) inherit the fix for free, OR add a `connectOrStartDaemonLocked` and route `ConnectOrStartDaemon` through it. Prefer the former (least churn).

---

### `internal/forwarder/dial_lock.go` (utility) — NEW (optional)

**Analog:** `internal/daemon/socket.go` `ensureSocket`/`createSocketDir` (stale-socket handling + 0700 dir, RESEARCH Sources). Only needed if the flock helper + lockfile-path derivation grows beyond a few lines; otherwise inline in `dial.go`. The lockfile inherits the per-uid 0700 socket dir (`socket.go:33`), so no new permission code is required.

---

### `internal/forwarder/dial_race_test.go` (synctest unit) — NEW

**Analog:** `internal/kernel/lspool/pool_synctest_test.go` (lines 30-60).

**Synctest harness shell** (`pool_synctest_test.go:30-43`):
```go
func TestRace_OneDaemon_Synctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// N goroutines call the lock-guarded spawn fn against a FAKE spawner
		// that counts startDaemon invocations. Virtual clock; assert count == 1.
		time.Sleep(300 * time.Second) // virtual; deterministic
		assert.Equal(t, 1, spawnCount)
	})
}
```
Pin the in-process lock/double-check ALGORITHM here (a spawn-counter seam — inject a fake spawner so no real `exec`). Pitfall 4: synctest CANNOT drive real subprocesses — the real-PID assertion lives in the E2E test below. `testing/synctest` is GA (Go 1.25+, module `go 1.25.1`); no GOEXPERIMENT flag.

---

### `internal/cli/cli_e2e_test.go` (E2E subprocess oracle) — NEW (TEST-01, CLI-01/02/03)

**Analog:** `bench/runtime/daemon_tap_integration_test.go` (lines 1-75) for the gate/skip + `internal/eval/sandbox/sandbox.go` `StartDaemon`/`DaemonHandle` (lines 315+) for the real daemon lifecycle.

**Build tag + binary resolution + skip** (`daemon_tap_integration_test.go:1-2, 24-34, 61-64`):
```go
//go:build !windows

func resolveHelixBin() string {
	if env := os.Getenv("HELIX_BIN"); env != "" {
		if _, err := os.Stat(env); err == nil { return env }
	}
	if p, err := exec.LookPath("helix"); err == nil { return p }
	return ""
}
// in the test:
helixBin := resolveHelixBin()
if helixBin == "" { t.Skip("helix binary not resolvable (set HELIX_BIN); skipping") }
```

**Real daemon via the sandbox** (`sandbox.go:315` `StartDaemon` → `DaemonHandle`):
```go
sb, _ := sandbox.NewSandbox(runID, helixBin)
defer sb.Cleanup()
sb.Prepare(task, mode)
h, _ := sb.StartDaemon(ctx, task, mode, "", "")  // waits for socket; "" profile/cfg
defer h.Kill()
pid := h.Pid()                                    // CLI-03 real: assert exactly one daemon PID
```
`StartDaemon` owns socket-path-length safety (macOS 104-byte AF_UNIX limit), env allowlist (`HOME`, `HELIX_LOG_LEVEL`, `PATH` only — `sandbox.go:350-357`), own process group (`Setpgid`, `sandbox.go:347`), and single-`Wait` reaping via `DaemonHandle`. DO NOT hand-roll an `exec.Command` harness.

**Sub-tests this file carries (per RESEARCH Test→Map):**
- `CLI_E2E` / `CLI_OneShot` (TEST-01, CLI-01): run a representative verb as a real subprocess against the live daemon; assert same result as the MCP path.
- `CLI_WarmReuseSLO` (CLI-02): time the WARM second `CallTool` round-trip; assert against the phase-recorded p50 SLO (Pitfall 5 — pick a no-LS verb, e.g. memory/health/fileops, OR gate on `WaitForLS` (`harness.go:225`) so the SLO isolates dial+IPC+dispatch). Assert the RECORDED number, not a hardcoded guess.
- `CLI_ParallelColdSingleDaemon` (CLI-03 real): N parallel cold callers/invocations against the SAME socket → count daemon PIDs == 1.

---

## Shared Patterns

### Daemon connect + auto-start (cross-cutting)
**Source:** `internal/forwarder/dial.go:24-40` `ConnectOrStartDaemon`
**Apply to:** `verb.go`, `oneshot.go` (and the race fix lands inside it for ALL callers).
Reuse as-is; the only edit is the flock guard. Pass `noop.NewTracerProvider()` from CLI commands (no OTel needed — `activate.go:53`).

### Cobra command convention
**Source:** every command in `internal/cli` (`activate.go:19-30`, registered at `root.go:86-93`)
**Apply to:** `verb.go`.
`SilenceUsage: true`, `SilenceErrors: true`; `RunE` returns wrapped errors (`fmt.Errorf("...: %w", err)`); flags via `cmd.Flags().String(...)`; register with `rootCmd.AddCommand(...)`.

### gRPC↔MCP-SDK byte bridge
**Source:** `internal/mcp/grpc_transport.go:44-130`
**Apply to:** `grpc_client_transport.go` (invert direction; reuse the `GRPCStream` interface + newline-delimited framing).

### Clean session teardown (telemetry hygiene)
**Source:** Pitfall 3 + daemon session-lifecycle classification (`daemon.go:1442-1470`)
**Apply to:** every one-shot call. `defer session.Close()` THEN `defer conn.Close()`; never drop the stream mid-flight (else `helix_session_lifecycle{outcome="error"}` climbs).

### HELIX_BIN-gated real-subprocess test convention
**Source:** `bench/runtime/daemon_tap_integration_test.go:1, 24-34`
**Apply to:** `cli_e2e_test.go`. `//go:build !windows`, `resolveHelixBin()` (HELIX_BIN → PATH → SKIP).

### Zero-proto invariant (hard gate)
**Source:** `api/proto/serena/v1/ipc.proto:9-30` (`StreamMCP` + `MCPMessage`)
**Apply to:** ALL files. `StreamMCP` already carries arbitrary JSON-RPC payloads — NO proto edit. CI gate: `git diff --exit-code api/proto/`.

---

## No Analog Found

None. Every file has a strong in-tree analog. The two genuinely net-new logic pieces (cross-process startup lock; client-side transport mirror) are both small and have direct structural analogs (`dial.go` unlocked window; `grpc_transport.go` server-side bridge).

## New Dependency

- `github.com/gofrs/flock v0.13.0` — `go get github.com/gofrs/flock@v0.13.0` then `go mod tidy`. RESEARCH A1 / Legitimacy Audit flag it for ONE `checkpoint:human-verify` (name sourced from training knowledge; existence + version verified via `go list`). Cross-platform `TryLock`/`Lock`/`Unlock`, pure-Go, no CGO — required for the Windows local-dial smoke (do not fall back to `syscall.Flock`).

## Metadata

**Analog search scope:** `internal/cli/`, `internal/forwarder/`, `internal/mcp/`, `internal/eval/sandbox/`, `internal/kernel/lspool/`, `bench/runtime/`, `test/integration/`, `api/proto/serena/v1/`
**Files read for excerpts:** 8 (grpc_transport.go, dial.go, root.go, activate.go, forwarder.go, harness.go, daemon_tap_integration_test.go, sandbox.go, pool_synctest_test.go)
**Pattern extraction date:** 2026-06-21
