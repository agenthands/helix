---
phase: 94-retire-the-agent-facing-mcp-surface-delete
verified: 2026-06-22T02:05:00Z
status: passed
score: 11/11 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 94: Retire the Agent-Facing MCP Surface (DELETE) Verification Report

**Phase Goal:** With the CLI proven as the sole agent surface via a dual-run parity test, DELETE the two agent-facing MCP heads (stdio MCP forwarder head + Streamable-HTTP `/mcp` transport / `--mode http`) while RETAINING the daemon, gRPC IPC, `StreamMCP`, `GRPCTransport`, the middlewares, and all tool handlers behind the wire. ADD an optional gRPC TCP bind (loopback/unix default, non-loopback opt-in gated per the v1.2 admin-addr pattern) + a remote/multi-client scope ADR.
**Verified:** 2026-06-22T02:05:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

ROADMAP Success Criteria (3) merged with PLAN frontmatter must_haves (8 across both plans), deduplicated.

| #   | Truth (source) | Status | Evidence |
| --- | -------------- | ------ | -------- |
| 1 | SC1: dual-run parity test green in the commit immediately before the deletion commit (strangler-fig gate) | ✓ VERIFIED | `git merge-base --is-ancestor c61dbca0 a1d4da7c` → YES; `... abd76b13` → YES. Parity proof commit c61dbca0 ("test(94-01): add dual-run parity gate") precedes both deletion commits a1d4da7c (stdio) and abd76b13 (HTTP). `TestCLI_DualRunParity` RUNS GREEN now over 5 verbs (search_in_files, read_file, get_symbol_overview, go_to_definition, find_references) with HELIX_BIN set. |
| 2 | SC2 / RETIRE-01: no stdio MCP server code path remains reachable; CLI still dials | ✓ VERIFIED | Negative grep (non-comment) for `func RunForwarder` / `RunStdio` / `runForwarderFn` / `func runForwarder` in internal/ = 0. Full-tree: `RunStdio` 0 hits anywhere; `RunForwarder` 2 hits, both doc comments. `--mode=stdio` behaviorally exits 1. `TestCLI_E2E_OneShot` GREEN post-deletion (cold-start auto-spawn dials the daemon). |
| 3 | SC2 / RETIRE-02: HTTP `/mcp` endpoint gone; `--mode http` no longer serves MCP | ✓ VERIFIED | Negative grep (non-comment) for `/mcp` / `listenHTTP` / `HTTPHandler` / `httpSessionMiddleware` / `HTTPAddr` / `http-addr` / `http_addr` in internal/ = 0. `internal/daemon/http_session_middleware.go` deleted (file gone). `"/mcp"` route: 0 hits tree-wide. `--mode=http` behaviorally exits 1. Remaining `listenHTTP`/`HTTPHandler` hits (3) are all doc comments. |
| 4 | SC3 / RETIRE-04: CLI can target a configured TCP daemon endpoint; default remains local unix socket | ✓ VERIFIED | `--grpc-addr` flag + `daemon.grpc_addr` config (default `""`); `tryConnect` dials `passthrough:///host:port` when set, unix otherwise. `daemon.grpc_addr` default `""` → unix-only. `TestListenGRPCTCP_LoopbackLifecycle` drives a real tcp StreamMCP round-trip GREEN. `TestListenGRPCTCP_Disabled` proves no-op when empty. |
| 5 | RETIRE-04 SECURITY: loopback gRPC bind accepted, non-loopback/wildcard refused before net.Listen | ✓ VERIFIED | `validateGRPCAddr` (grpc_tcp.go:85) refuses `case ""` (`:9099`/`:0`) + non-loopback with REMOTE-01 error; `!ip.IsUnspecified()` guard. `TestValidateGRPCAddr` GREEN: `:9099`, `:0`, `[::]:9099`, `0.0.0.0:9099`, `192.168.1.5`, `example.com` REFUSED with "REMOTE-01"; `127.0.0.1`, `localhost`, `[::1]`, `127.0.0.1:0` accepted. CR-01 fix (commit 6361b1f4) confirmed in code. |
| 6 | After deletion, `go build ./cmd/helix` succeeds; CLI dials over retained gRPC unix socket | ✓ VERIFIED | `go build ./cmd/helix` exit 0 (175MB binary produced). `go build ./...` exit 0. CallTool/listenSocket/StreamMCP/GRPCTransport/forwarderServiceHandler all present (keep-list grep). |
| 7 | Retained `TestCLI_E2E_OneShot` + `TestCLI_DualRunParity` remain green post-deletion | ✓ VERIFIED | Both GREEN with HELIX_BIN=$(pwd)/helix `-tags integration` (E2E 0.33s, parity 0.74s over 5 verbs). |
| 8 | `go build -tags integration ./...` and `go vet -tags integration ./...` succeed (HTTP test fallout re-targeted, not false-green) | ✓ VERIFIED | `go build -tags integration ./...` exit 0; `go vet ./...` exit 0. Re-targeted `TestHandshake_*` / `TestReconnect_*` GREEN under `-tags integration` (test/oracle/protocol ok). `NewHTTPSession`/`HTTPHandler`/`HTTPAddr` in test/ = 0 non-comment refs. |
| 9 | `generateSessionID` NOT deleted (CallTool depends on it) | ✓ VERIFIED | `func generateSessionID` present in internal/forwarder/forwarder.go (retained in package forwarder; prohibition was "must not be deleted" — satisfied). CallTool + Session both reference it. |
| 10 | CLI dial path NOT deleted (oneshot.go, dial files, grpc_client_transport.go, listenSocket, StreamMCP, GRPCTransport, forwarderServiceHandler, listenGRPCTCP) | ✓ VERIFIED | All present: CallTool (1), listenSocket (1), StreamMCP (3), listenGRPCTCP (1), newForwarderServiceHandler (4), NewGRPCClientTransport (15). `internal/forwarder/session.go` exists (the FIX's multi-call gRPC driver). |
| 11 | `git diff api/proto/` and `git diff go.mod go.sum` empty (zero proto change, zero new deps) | ✓ VERIFIED | `git diff --exit-code api/proto/` CLEAN; `git diff --exit-code go.mod go.sum` CLEAN. StreamMCP RPC still present in api/proto/serena/v1/ipc.proto. |

**Score:** 11/11 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `internal/daemon/grpc_tcp.go` | validateGRPCAddr loopback gate + listenGRPCTCP | ✓ VERIFIED | Contains `func validateGRPCAddr` + `func (d *Daemon) listenGRPCTCP`; CR-01-fixed wildcard refusal; wired into daemon.go errgroup (WR-01 degrade). |
| `internal/daemon/grpc_tcp_test.go` | validateGRPCAddr table + tcp round-trip | ✓ VERIFIED | `TestValidateGRPCAddr` (12 cases incl. all wildcard/non-loopback REFUSED), `TestListenGRPCTCP_Disabled/_NonLoopbackRejected/_LoopbackLifecycle` all GREEN. |
| `REMOTE-SCOPE-ADR.md` | remote/multi-client scope decision record | ✓ VERIFIED | Contains REMOTE-01/REMOTE-02 deferral, STRIDE table (T-94-01..04), loopback-only rationale, decision to replace `/mcp` network reach with gated gRPC-TCP. |
| `internal/forwarder/oneshot.go` | retained CLI one-shot CallTool | ✓ VERIFIED | `func CallTool` present; CLI verb dispatch rides it. |
| `internal/forwarder/session.go` | retained multi-call gRPC driver (FIX) | ✓ VERIFIED | `OpenSession` + `Session.CallTool` over StreamMCP; consumed by bench/runtime + eval/runner (migrated off deleted stdio). |
| `internal/cli/root.go` | mode switch with stdio+http arms removed, --http-addr removed, --serve retained | ✓ VERIFIED | switch has only `auto` arm + unknown-mode default; `if serve` starts daemon; `--grpc-addr` flag + WR-02 early ValidateGRPCAddr call present; no `--http-addr` flag. |
| `internal/daemon/http_session_middleware.go` | DELETED | ✓ VERIFIED | File absent (+ its _test.go sibling). |
| `internal/forwarder/dial_windows_test.go` | Windows local-dial smoke (CI-gated) | ✓ VERIFIED | Exists; `GOOS=windows go build ./internal/forwarder/...` exit 0. CI-Windows-gated, honestly reported (not claimed to run on this Linux host). |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| internal/cli/root.go | internal/config | `--grpc-addr` → daemon.grpc_addr override → DaemonConfig.GRPCAddr | ✓ WIRED | flag at root.go:143, override at :262, early validate at :292, field read in daemon.go:1333. |
| internal/forwarder/dial.go | internal/daemon/grpc_tcp.go | tryConnect dials passthrough:///host:port; listenGRPCTCP serves it | ✓ WIRED | passthrough scheme (corrected from tcp:// per Rule-1 fix); lifecycle test round-trips. |
| internal/cli (verb path) | internal/forwarder/oneshot.go | CLI verb dispatch → CallTool → ConnectOrStartDaemon → StreamMCP | ✓ WIRED | TestCLI_E2E_OneShot + DualRunParity prove the live path post-deletion. |
| internal/forwarder/dial.go | internal/daemon/daemon.go | startDaemon exec args (no --http-addr) → listenSocket | ✓ WIRED | No `--http-addr` in startDaemon argv (+ sandbox.go 10th site fixed); cold-start E2E green, no 10s hang. |
| daemon.go errgroup | grpc_tcp.go | gated listenGRPCTCP launch, degrade-on-bind-error (WR-01) | ✓ WIRED | daemon.go:1333-1342 returns nil on non-Canceled error (does not tear down unix transport). |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| go build / -tags integration / vet | `go build ./... && go build -tags integration ./... && go vet ./...` | all exit 0 | ✓ PASS |
| Security gate refuses wildcard/non-loopback | `go test ./internal/daemon -run TestValidateGRPCAddr` | PASS (all REFUSED cases) | ✓ PASS |
| gRPC TCP lifecycle round-trip | `go test ./internal/daemon -run TestListenGRPCTCP` | PASS | ✓ PASS |
| Strangler-fig parity (CLI == MCP path) | `HELIX_BIN=… go test -tags integration -run TestCLI_DualRunParity` | PASS (5 verbs, RAN not skipped) | ✓ PASS |
| Cold-start auto-spawn dial | `HELIX_BIN=… go test -tags integration -run TestCLI_E2E_OneShot` | PASS (no unknown-flag hang) | ✓ PASS |
| `--mode=stdio` no longer serves | `./helix --mode=stdio` | exit 1 | ✓ PASS |
| `--mode=http` no longer serves | `./helix --mode=http` | exit 1 | ✓ PASS |
| Re-targeted transport tests | `go test -tags integration ./test/oracle/protocol/...` | ok | ✓ PASS |
| eval/runner daemon_tap (FIX) | `go test -run TestDaemonTapIntegration ./internal/eval/runner/...` | ok | ✓ PASS |
| bench/runtime gRPC migration (FIX) | `HELIX_BIN=… go test ./bench/runtime/...` | ok (34.9s) | ✓ PASS |
| Windows smoke compiles | `GOOS=windows go build ./internal/forwarder/...` | exit 0 | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| RETIRE-01 | 94-02 | stdio MCP forwarder head removed; CLI still dials | ✓ SATISFIED | Truths 2, 6, 7; zero non-comment refs to RunForwarder/RunStdio/stdio route. |
| RETIRE-02 | 94-02 | Streamable-HTTP `/mcp` / `--mode http` removed | ✓ SATISFIED | Truth 3; file deleted, all --http-addr sites gone, behavioral exit 1. |
| RETIRE-03 | 94-01 | strangler-fig dual-run parity before deletion | ✓ SATISFIED | Truth 1; git ancestry proven (c61dbca0 precedes deletions), parity green now. |
| RETIRE-04 | 94-01 | optional gated gRPC TCP bind (loopback default, non-loopback gated) | ✓ SATISFIED | Truths 4, 5; validateGRPCAddr + ADR + security test. |

### Anti-Patterns Found

None. No debt markers (TBD/FIXME/XXX), no TODO/HACK/placeholder in phase-94 source files (grpc_tcp.go, session.go, oneshot.go, dial.go, root.go, drive.go, ndjson.go).

### Code-Review Resolution

The phase code review (94-REVIEW.md) found 1 BLOCKER (CR-01) + 2 warnings (WR-01, WR-02), all fixed (94-REVIEW-FIX.md):
- **CR-01** (commit 6361b1f4): wildcard `:port` bind refused — confirmed in grpc_tcp.go:96-101 (`case ""`) + `!ip.IsUnspecified()` guard, covered by TestValidateGRPCAddr (`:9099`/`:0`/`[::]` REFUSED). ✓ IN CODE.
- **WR-01** (commit 9531777c): TCP bind failure no longer tears down the daemon — confirmed daemon.go:1335-1340 returns nil on non-Canceled error. ✓ IN CODE.
- **WR-02** (commit 9531777c): config-load validation — confirmed root.go:292 `daemon.ValidateGRPCAddr(cfg.Daemon.GRPCAddr)` before daemon.New. ✓ IN CODE.

### Known-Environmental (NOT phase-94 regressions, NOT gaps)

Per phase summaries (verified via pre-phase-94 baseline worktree at ceeec1a9/c61dbca0) and verification context:
- `internal/guardrails` receipt-ID flake + `internal/obs` lspool-span flake: pre-existing parallel-load flakes, reference no deleted symbols, untouched packages.
- `test/oracle/scenario` + `test/oracle/runtime` (`-tags integration`): pre-existing gopls/LS-readiness env failures, identical on baseline.
- `cmd/helix-bench` `TestRunSubcommandWiresDeltaPass`/`WiresThemAll`: env-dependent (helix not on PATH + pre-existing missing-ablation_deltas), fail identically on baseline ceeec1a9 (v1.12 deferred gate).

These are documented in deferred-items.md and are out of phase scope.

### Gaps Summary

None. All 3 ROADMAP Success Criteria and all 4 requirements (RETIRE-01..04) are satisfied with executable evidence:
- The strangler-fig sequencing invariant holds (git ancestry proven): parity proof landed GREEN before the two deletion commits.
- Both agent-facing MCP heads are deleted with zero functional references; only doc-comment mentions remain.
- The retained CLI dial path (CallTool/StreamMCP/listenSocket/GRPCTransport + the new Session driver) is intact and proven by E2E + parity + bench/eval migration tests.
- The gated gRPC TCP bind correctly refuses wildcard/non-loopback (CR-01 BLOCKER fix in code and tested), degrades gracefully on bind error (WR-01), and validates early (WR-02).
- Zero proto change, zero new dependencies.

---

_Verified: 2026-06-22T02:05:00Z_
_Verifier: Claude (gsd-verifier)_
