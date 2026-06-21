---
phase: 94-retire-the-agent-facing-mcp-surface-delete
plan: 02
subsystem: infra
tags: [mcp, daemon, cli, forwarder, http, stdio, deletion, strangler-fig]

# Dependency graph
requires:
  - phase: 94-retire-the-agent-facing-mcp-surface-delete
    plan: 01
    provides: "RETIRE-03 dual-run parity gate (GREEN with both heads alive) + gated gRPC-TCP listener — authorizes deletion in the commit AFTER the parity proof"
provides:
  - "CLI as the sole agent-facing MCP surface (per-verb forwarder.CallTool over the retained gRPC unix wire)"
  - "Deleted stdio forwarder head (RunForwarder) + the --mode=stdio route"
  - "Deleted HTTP /mcp head (listenHTTP + HTTPHandler + httpSessionMiddleware) + the full --http-addr thread"
affects: [REMOTE-01, REMOTE-02]

# Tech tracking
tech-stack:
  added: []  # zero new deps — git diff go.mod go.sum empty
  patterns:
    - "Strangler-fig completion: deletion commits land strictly AFTER 94-01's parity-proof commit (c61dbca0)"
    - "Re-target-before-delete: HTTP test callers moved to gRPC/in-memory BEFORE HTTPHandler deletion so the build never breaks mid-task"
    - "In-memory second session as gRPC stand-in: both the unix gRPC wire and mcp.NewInMemoryTransports() funnel into the identical mcpServer.SDK()"

key-files:
  created:
    - internal/forwarder/dial_windows_test.go
    - test/integration/smoke_grpc_test.go  # renamed from smoke_http_test.go
  modified:
    - internal/forwarder/forwarder.go
    - internal/forwarder/forwarder_test.go
    - internal/forwarder/dial.go
    - internal/cli/root.go
    - internal/cli/root_test.go
    - internal/cli/cli_e2e_test.go
    - internal/mcp/server.go
    - internal/daemon/daemon.go
    - internal/config/config.go
    - internal/config/defaults.go
    - internal/config/loader_test.go
    - internal/eval/sandbox/sandbox.go
    - test/harness/runner.go
    - test/integration/harness.go
    - test/oracle/protocol/handshake_test.go
    - test/oracle/protocol/reconnect_test.go
    - internal/daemon/daemon_test.go
    - internal/daemon/bootstrap_test.go
    - internal/daemon/shutdown_graceful_test.go
    - internal/daemon/daemon_integration_test.go
    - test/integration/trace_propagation_test.go
    - test/integration/trace_continuity_test.go
    - test/integration/trace_shutdown_test.go
    - test/bench/bench_helpers_test.go
  deleted:
    - internal/daemon/http_session_middleware.go
    - internal/daemon/http_session_middleware_test.go

key-decisions:
  - "Slimmed forwarder.go to ONLY the relocated generateSessionID helper (kept in package forwarder, not moved into oneshot.go) — CallTool's sole remaining dependency; build proves the relocation"
  - "--mode kept with only the auto arm; legacy stdio/http modes now surface the unknown-mode error (Q2 discretion: keep --mode coherent rather than removing it)"
  - "NewHTTPSession -> NewGRPCSession via mcp.NewInMemoryTransports() against the same daemon SDK (the in-memory path is the proven TestHandshake_InMemory transport; both heads + CallTool funnel through the identical mcpServer.SDK())"
  - "Windows local-dial smoke spawns the daemon directly (NOT via internal/eval/sandbox, which is Unix-only: syscall.Kill/Setpgid) and dials via the retained forwarder.CallTool path"
  - "loader_test http_addr assertions re-pointed to the retained daemon.grpc_addr key (the remaining daemon address key) rather than deleted"

requirements-completed: [RETIRE-01, RETIRE-02]

# Metrics
duration: 25min
completed: 2026-06-22
status: complete
---

# Phase 94 Plan 02: Retire the Agent-Facing MCP Surface (DELETE) Summary

**Deleted both agent-facing MCP heads — the stdio forwarder (RunForwarder) and the HTTP /mcp listener (listenHTTP/HTTPHandler/httpSessionMiddleware) — plus the dead RunStdio and the entire --http-addr thread, leaving the CLI's per-verb gRPC dial path as the sole agent surface; the retained E2E + RETIRE-03 parity tests stay GREEN post-deletion, with zero proto change and zero new deps.**

## Performance

- **Duration:** ~25 min
- **Tasks:** 3 / 3
- **Commits:** 3 task commits (a1d4da7c, d8081b47, abd76b13), landed strictly AFTER 94-01's parity-proof commit c61dbca0 (strangler-fig sequencing invariant held)
- **Files:** 28 changed (2 created, 2 deleted, 24 modified, incl. 1 rename)

## What Was Built (Deleted)

### Task 1 — Delete the stdio head (commit a1d4da7c)
- `forwarder.RunForwarder` (the only stdin/stdout MCP pump) + the forwarder-only helpers `isToolsCall`/`sendWithSpan` + `toolsCallMethod`/`toolsCallMethodSpaced` vars deleted. `forwarder.go` slimmed to ONLY the relocated `generateSessionID` (CallTool's dependency).
- `cli.runForwarder` + the `runForwarderFn` seam var + the `--mode=stdio` switch arm deleted; `--mode` now accepts only `auto` (legacy stdio/http surface the unknown-mode error).
- Dead `mcp.SerenaMCPServer.RunStdio` (zero callers) deleted.
- `root_test.go` routing tests re-targeted (stdio now rejected); `forwarder_test.go` trimmed (dropped `isToolsCall`/`sendWithSpan`/`fakeStream` tests, kept `TestGenerateSessionID` + dial-helper tests).

### Task 3 — Re-target HTTP test fallout (commit d8081b47, landed before Task 2)
- `runner.NewHTTPSession` / `harness.NewHTTPSession` → `NewGRPCSession`: a fresh in-memory session against the same `mcpServer.SDK()` the gRPC StreamMCP wire funnels into (drops `httptest` + `HTTPHandler` usage).
- `handshake_test`: `TestHandshake_HTTP` → `TestHandshake_GRPC`; `ProtocolVersionMatch` compares in-memory vs gRPC; **`TestHandshake_InMemory` retained**.
- `reconnect_test`: 3 `NewHTTPSession` → `NewGRPCSession`.
- `smoke_http_test.go` → `smoke_grpc_test.go`: gRPC round-trip smoke replacing the HTTP-endpoint smoke.
- `loader_test`: `http_addr` assertions re-pointed to the retained `grpc_addr` key.
- Dropped all dangling `cfg.Daemon.HTTPAddr = ""` lines (field deleted in Task 2).
- Added `//go:build windows` `dial_windows_test.go` local-dial smoke (CI-gated).

### Task 2 — Delete the HTTP /mcp head + thread out --http-addr (commit abd76b13)
- `daemon.go`: deleted the gated `listenHTTP` launch, the `listenHTTP` func + the `/mcp` mount, the `http_addr` startup log field; dropped the now-unused `net/http` import.
- `mcp/server.go`: deleted `HTTPHandler` (its test callers were re-targeted first); dropped the now-unused `net/http` import.
- Deleted `internal/daemon/http_session_middleware.go` (+ `_test.go`) entirely.
- Removed the `--http-addr` flag, the `mode == "http"` daemon-start guard arm, the `GetString` + `daemon.http_addr` override block (root.go); deleted `DaemonConfig.HTTPAddr` (config.go) + the `daemon.http_addr` default (defaults.go).
- **LOAD-BEARING:** dropped the `--http-addr=` exec arg from `startDaemon` (dial.go) AND the sandbox `StartDaemon` argv (sandbox.go) so the auto-spawned daemon does not die on an unknown flag (the 10s cold-start hang, Pitfall 4 / T-94-06).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] sandbox.go carried an unenumerated 10th `--http-addr=` exec-arg site**
- **Found during:** Task 2 (tree-wide grep for `--http-addr`).
- **Issue:** The plan enumerated 9 `--http-addr` sites, but `internal/eval/sandbox/sandbox.go:349` also passed `--http-addr=` in the daemon argv. After the flag was deleted, this would make every sandbox-spawned daemon (the E2E + parity harness) die on an unknown flag — the exact 10s cold-start hang T-94-06 warns about, masking the deletion behind opaque timeouts.
- **Fix:** Dropped the `--http-addr=` arg from the sandbox argv (mirrors the dial.go:185 fix), added an explanatory comment.
- **Files modified:** internal/eval/sandbox/sandbox.go.
- **Commit:** abd76b13.

**2. [Rule 1 - Test fixup] root_test.go / loader_test.go / cli_e2e_test.go asserted deleted behavior or referenced deleted fields**
- **Found during:** Tasks 1–2.
- **Issue:** `root_test.go` asserted `--mode=stdio` routes to the forwarder and `--mode=http` routes to the daemon (both deleted), and swapped a deleted `runForwarderFn` seam; `loader_test.go` asserted the deleted `daemon.http_addr` default/override; `cli_e2e_test.go` had a stale `--http-addr=` comment in the pgrep argv-matcher.
- **Fix:** Re-targeted `root_test.go` (stdio now expects the unknown-mode error; dropped the http-mode test; `withRoutingSeams` lost its forwarder seam); re-pointed `loader_test.go` to the retained `grpc_addr` key; corrected the cli_e2e comment.
- **Files modified:** internal/cli/root_test.go, internal/config/loader_test.go, internal/cli/cli_e2e_test.go.
- **Commits:** a1d4da7c, abd76b13.

**3. [Rule 3 - Blocking] Windows smoke could not use the sandbox harness (Unix-only)**
- **Found during:** Task 3 (GOOS=windows vet).
- **Issue:** The plan suggested mirroring the `!windows` cli_e2e shape (which uses `internal/eval/sandbox`), but that package does not compile on Windows (`syscall.Kill`, `SysProcAttr.Setpgid`). A sandbox-based Windows smoke would never build on Windows.
- **Fix:** The `//go:build windows` smoke spawns the daemon directly via `exec.Command(helixBin, "--serve", "--socket=…")` and dials through the retained `forwarder.CallTool` path — no sandbox dependency. CI-Windows-gated; reported honestly as Manual-Only (not run on this Linux host).
- **Files modified:** internal/forwarder/dial_windows_test.go (new).
- **Commit:** d8081b47.

## Deferred Issues

- **`internal/obs` `TestConnCallProducesLspoolSpan`** — pre-existing flake: FAILs under full-tree parallel `go test ./...` load, passes 3/3 in isolation. References no deleted symbol; `obs` untouched. Logged in deferred-items.md.
- **`test/oracle/scenario` + `test/oracle/runtime`** (under `-tags integration`) — pre-existing gopls/LS-readiness env failures. Verified identical failure on the pre-deletion baseline `c61dbca0` (worktree run). Reference no deleted symbols. The plan-specified transport gate `./test/oracle/protocol/...` is fully GREEN. Logged in deferred-items.md.

## Critical Invariants Held

- **Strangler-fig sequencing:** all 3 deletion commits land strictly AFTER 94-01's parity-proof commit c61dbca0 (RETIRE-03 acceptance).
- **No reachable stdio MCP path:** `RunForwarder`, the stdio mode arm, `runForwarder`/`runForwarderFn`, `RunStdio` — zero non-comment references.
- **No HTTP /mcp route:** `listenHTTP`, `HTTPHandler`, `httpSessionMiddleware` (file gone), `/mcp`, `HTTPAddr`, `--http-addr`, `http_addr` — zero non-comment references across internal/ + test/. `--mode http` no longer serves MCP; `--serve` still starts the daemon.
- **Retained CLI dial path intact:** `CallTool`, dial files, `grpc_client_transport.go`, `listenSocket`, `StreamMCP`, `GRPCTransport`, `forwarderServiceHandler`, the middlewares, and 94-01's `listenGRPCTCP` — all present. `generateSessionID` relocated, not deleted.
- **`git diff api/proto/` empty; `git diff go.mod go.sum` empty** — verified after every task.

## Verification Results

- `go build ./...`, `go vet ./...`: clean.
- `go build -tags integration ./...`, `go vet -tags integration ./...`: clean (false-green guard — the HTTP transport tests are integration-tagged).
- `go build -tags llm ./...` / `-tags llmjudge ./...`: clean (handshake/reconnect carry those tags too).
- `GOOS=windows go build ./internal/forwarder/...` + `GOOS=windows go vet ./internal/forwarder/...`: clean (Windows smoke compiles).
- Touched-package suite green: `go test ./internal/forwarder/... ./internal/cli/... ./internal/daemon/... ./internal/config/... ./internal/mcp/... ./internal/eval/sandbox/... -count=1`.
- **False-green guard RUNS (not skip):** `HELIX_BIN=$(pwd)/helix go test -tags integration ./test/oracle/protocol/... -count=1` — GREEN (TestHandshake_InMemory/GRPC/ProtocolVersionMatch, TestReconnect_*, all pass).
- **Retained surface GREEN post-deletion:** `HELIX_BIN=$(pwd)/helix go test -tags integration -run 'TestCLI_E2E_OneShot|TestCLI_DualRunParity' ./internal/cli/...` — GREEN (CLI still dials the daemon; no cold-start hang).
- `git diff --exit-code api/proto/ go.mod go.sum`: empty.

## Threat Flags

None — the deletion is a net attack-surface reduction:
- **T-94-05** (mitigated): the unauthenticated HTTP /mcp listener (default :8080) is gone — no listener binds the old http addr; `HTTPAddr` field removed.
- **T-94-06** (mitigated): startDaemon + sandbox argv cleaned; the auto-start E2E (TestCLI_E2E_OneShot cold path) proves no unknown-flag death / 10s hang.
- **T-94-07** (mitigated): keep-list enforced; `go build ./cmd/helix` + retained TestCLI_E2E_OneShot + TestCLI_DualRunParity gate the CLI surface post-deletion — all green.
- No new network surface, no schema/trust-boundary additions, no package installs (zero-new-deps gate held).

## Self-Check: PASSED

All created files exist (dial_windows_test.go, smoke_grpc_test.go, 94-02-SUMMARY.md), both deleted middleware files are gone, and all 3 task commits (a1d4da7c, d8081b47, abd76b13) are present in git history.

---

## FIX (fix(94-02)): Migrate the two internal stdio-forwarder consumers onto the gRPC StreamMCP wire

The 94-02 deletion of `helix --mode=stdio` left two INTERNAL harnesses broken — they
still spawned the deleted stdio forwarder head as a subprocess and piped MCP
JSON-RPC NDJSON over its stdin/stdout. This follow-on fix migrates both onto the
RETAINED gRPC `StreamMCP` wire. NOT a new plan — atomic `fix(94-02)` commit.

### New retained internal driver
- **`internal/forwarder/session.go` (new):** `Session` + `OpenSession()` — the
  multi-call analog of the one-shot `forwarder.CallTool`. It opens ONE `StreamMCP`
  stream via `ConnectOrStartDaemon` (reusing `grpc_client_transport.go` +
  `generateSessionID`), performs the MCP SDK initialize handshake ONCE, then
  exposes `CallTool(name, args)` for a SEQUENCE of calls over that single
  daemon-side session. `Close()` performs the same ordered teardown as
  `oneshot.go` (SDK shutdown flush → `stream.CloseSend()` clean-EOF →
  `conn.Close()`), so the daemon records `outcome="ended"`, not an RST abort.
  This is an in-process Go API — NO CLI subcommand/flag, NO stdio MCP server path,
  NO `--mode=stdio` route (SC2 held).

### Consumer migrations
- **`bench/runtime/drive.go`:** `driveScript` now dials the daemon via
  `forwarder.OpenSession` instead of `exec.Command(helixBin, "--mode=stdio", ...)`.
  Timing fidelity preserved: per-step `AtTime` dispatch instants (Pitfall 5),
  per-step `StepResult.Err` capture that does NOT abort the drive, per-step context
  deadlines, and parent-ctx cancellation. The old id-keyed pending buffer /
  out-of-order demux is obsolete (each `session.CallTool` blocks for its own
  response); the stdin-close-after-reads race is moot under gRPC (replaced by
  `Session.Close()` running after the loop returns — documented). `helixBin` param
  retained for signature stability (no longer used — the gRPC dial needs no binary).
- **`bench/runtime/ndjson.go` (new):** the NDJSON reader/dispatcher
  (`jsonrpcResp`, `readResponses`, `parseRPCLine`, `respDispatcher`) was extracted
  from the old `drive.go` because it is STILL legitimately used by
  `rag.go:driveRAGServer` — that drives the standalone `cmd/helix-bench-rag` server,
  which IS an MCP endpoint over its OWN stdio (`StdioTransport`, not a forwarder,
  not the retired agent head). Keeping it unbroken is in scope; it is unrelated to
  the deleted `--mode=stdio` forwarder.
- **`internal/eval/runner/daemon_tap_integration_test.go`:** `driveSingleToolCall`
  now uses `forwarder.CallTool` (the retained one-shot dial) instead of the stdio
  spawn. **Real behavioral fix surfaced:** the test drove `get_health`, which is
  REFUSED by `ProfileEnforcementMiddleware` under the baseline profile's empty tool
  whitelist (`baseline.yaml: tools: []`). That refusal short-circuits BEFORE
  `TelemetryMiddleware` (execution order `…→ ProfileEnforce →…→ Telemetry →
  handler`), so a denied call emits ZERO `msg="tool call"` lines and the tap sees
  nothing. Switched to `activate_project` (in `alwaysAllowedCoreTools`), which
  reaches the handler and triggers the TelemetryMiddleware emission the tap
  asserts. (The pre-existing `get_health`-in-every-profile comment was stale.)
- **`bench/runtime/subprocess/ragserver.go`:** [Rule 1] corrected a stale doc
  comment that described the daemon being driven through `helix --mode=stdio` and
  passing `--http-addr=` (both deleted in 94-02) — now describes the gRPC
  `OpenSession` dial.

### Critical invariants held
- **No stdio MCP server path reintroduced:** zero non-comment `--mode=stdio` refs
  in `bench/` + `internal/eval/`; zero `RunForwarder`/`RunStdio`/`runForwarder`.
- **`git diff go.mod go.sum api/proto/` empty** — zero new deps, zero proto change.
- **Retained gates GREEN:** `TestCLI_E2E_OneShot` + `TestCLI_DualRunParity`
  (`-tags integration`) pass post-migration.

### Verification
- `go build ./...` + `go build -tags integration ./...` clean; `go vet ./...` +
  `go vet -tags integration ./...` clean; all new files gofmt-clean.
- `HELIX_BIN=$(pwd)/helix go test ./bench/runtime/... -count=1` GREEN
  (TestCrossCell / TestDaemonTap / TestFiveOfSixSmoke / TestNoSemanticStoreOnZeroReads
  / TestStoreIsolationParallel — 33.7s).
- `PATH=$(pwd):$PATH HELIX_BIN=$(pwd)/helix go test -run TestDaemonTapIntegration
  ./internal/eval/runner/...` GREEN (the F-07 tap regression guard).
- `go test ./internal/forwarder/... ./internal/eval/... -count=1` GREEN.
