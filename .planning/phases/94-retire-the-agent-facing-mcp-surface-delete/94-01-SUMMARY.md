---
phase: 94-retire-the-agent-facing-mcp-surface-delete
plan: 01
subsystem: infra
tags: [grpc, daemon, cli, mcp, loopback, parity, strangler-fig, tcp]

# Dependency graph
requires:
  - phase: 90-cli-dial-spine
    provides: ConnectOrStartDaemon / tryConnect / CallTool dial path + E2E sandbox harness
  - phase: 92-terse-renderer
    provides: frozen terse locus shape + render-class taxonomy the parity gate compares against
provides:
  - Gated, loopback-only gRPC TCP daemon listener (listenGRPCTCP + validateGRPCAddr) reusing StreamMCP
  - daemon.grpc_addr config + --grpc-addr flag + HELIX_GRPC_ADDR env dial selector
  - tryConnect/ConnectOrStartDaemon/CallTool tcp:// dial branch (passthrough:/// target)
  - TestCLI_DualRunParity — the strangler-fig pre-deletion parity gate (CLI == pre-removal MCP path)
  - REMOTE-SCOPE-ADR.md (remote/multi-client scope decision record)
affects: [94-02-mcp-head-deletion, REMOTE-01, REMOTE-02]

# Tech tracking
tech-stack:
  added: []  # zero new deps — gate satisfied (git diff go.mod go.sum empty)
  patterns:
    - "Loopback-gate mirror: validateGRPCAddr is a line-for-line analog of validateAdminAddr (net.IP.IsLoopback single source of truth)"
    - "Shared gRPC handler factory (newForwarderServiceHandler) so unix + tcp listeners are byte-identical"
    - "passthrough:///host:port gRPC target as the direct analog of unix://path (tcp:// is NOT a valid gRPC scheme)"
    - "Dual-run parity gate: per-render-class comparison (locus-set / verbatim / symbol-set)"

key-files:
  created:
    - internal/daemon/grpc_tcp.go
    - internal/daemon/grpc_tcp_test.go
    - internal/forwarder/dial_tcp_test.go
    - .planning/phases/94-retire-the-agent-facing-mcp-surface-delete/REMOTE-SCOPE-ADR.md
  modified:
    - internal/daemon/daemon.go
    - internal/config/config.go
    - internal/config/defaults.go
    - internal/forwarder/dial.go
    - internal/forwarder/oneshot.go
    - internal/forwarder/forwarder.go
    - internal/cli/root.go
    - internal/cli/verb.go
    - internal/cli/activate.go
    - internal/cli/cli_e2e_test.go

key-decisions:
  - "Used passthrough:///host:port (not tcp://) as the gRPC TCP target — tcp:// is not a valid gRPC scheme and triggers a 'too many colons' parse error (Rule 1 fix vs plan text)"
  - "No auto-start on the TCP dial path: a split-host/TCP daemon is operator-managed, so ConnectOrStartDaemon dials-or-fails-fast (the unix lock+spawn cold path is unix-only)"
  - "Added a test-only testServeSession seam on Daemon so the listenGRPCTCP lifecycle test drives a real tcp StreamMCP round-trip without a full MCP server"
  - "Parity gate compares per render class: locus-set (classLocusList), verbatim (classOpaque), symbol-set (classTree) — robust to abs-vs-rel paths and CLI re-rendering"

patterns-established:
  - "Optional network listener gate: empty config = no-op (default), loopback-only opt-in, non-loopback refused before net.Listen pointing at a deferral ADR ID"
  - "Strangler-fig pre-deletion proof: the parity test lands GREEN with both old heads alive in the commit BEFORE the deletion commit"

requirements-completed: [RETIRE-03, RETIRE-04]

# Metrics
duration: 18min
completed: 2026-06-22
status: complete
---

# Phase 94 Plan 01: Strangler-Fig Parity Gate + Gated gRPC-TCP Listener Summary

**Landed the CLI-is-sole-sufficient-surface proof (TestCLI_DualRunParity) and the loopback-gated gRPC-TCP opt-in that replaces the to-be-deleted HTTP /mcp head's network reach — both green with BOTH agent-facing heads still alive, zero new deps, zero proto change.**

## Performance

- **Duration:** ~18 min
- **Started:** 2026-06-21T21:20:11Z
- **Completed:** 2026-06-22T00:36Z (wall clock spans local-time rollover during the run)
- **Tasks:** 3 / 3
- **Files modified:** 14 (4 created, 10 modified)

## What Was Built

### Task 1 — RETIRE-04: gated gRPC TCP listener (TDD RED→GREEN)
- `validateGRPCAddr` (internal/daemon/grpc_tcp.go): line-for-line analog of `validateAdminAddr` — empty = disabled (valid), loopback forms accepted via `net.IP.IsLoopback()`, non-loopback refused with an error pointing at REMOTE-01.
- `listenGRPCTCP`: empty-addr no-op + validate gate + `net.Listen("tcp", …)`, serving the SAME `ForwarderService` with `GracefulStop()` on `ctx.Done()` (the gRPC form, not HTTP `server.Shutdown`).
- `newForwarderServiceHandler`: shared factory so `listenSocket` (unix) and `listenGRPCTCP` (tcp) register byte-identical handlers; `testServeSession` seam wired through it.
- Config: `DaemonConfig.GRPCAddr` koanf field + `daemon.grpc_addr` default `""`; gated errgroup launch alongside `listenSocket`; `grpc_addr` added to the startup log.
- Commits: `85b8056b` (RED), `92aff4f8` (GREEN).

### Task 2 — RETIRE-04: --grpc-addr flag + tcp dial branch (TDD RED→GREEN)
- `tryConnect`/`ConnectOrStartDaemon`/`CallTool` gained a `tcpAddr` parameter; when set the unix-file liveness probe is skipped and the dial targets `passthrough:///host:port`. Default (empty) is byte-identical to the prior unix path.
- `--grpc-addr` root flag (daemon bind on `--serve`; dial target on a verb) with the non-empty-only override guard (mirrors `--admin-addr`, not `.Changed()`); `resolveVerbGRPCAddr` precedence: flag > `HELIX_GRPC_ADDR` env > "" (unix).
- All dial callers updated (forwarder/oneshot/activate + cross-tree tests).
- Commits: `7dac4782` (RED), `309eed97` (GREEN).

### Task 3 — RETIRE-03: dual-run parity gate + ADR
- `TestCLI_DualRunParity` (internal/cli/cli_e2e_test.go): table-driven over search_in_files, read_file, get_symbol_overview, go_to_definition, find_references against ONE live daemon with both heads present. Per render class: locus-set parity (classLocusList), verbatim payload (classOpaque), symbol-set parity (classTree). LS-backed verbs skip without gopls; the whole test skips when HELIX_BIN is unset.
- `mcpCall` generalizes `mcpSearch` over the retained gRPC `forwarder.CallTool` path (the minimum "pre-removal MCP path" — both deleted heads funnel through the identical `mcpServer.SDK()`).
- `REMOTE-SCOPE-ADR.md`: records the gated gRPC-TCP replacement, STRIDE framing (T-94-01..04), and the REMOTE-01 (authn/TLS) / REMOTE-02 (multi-client) deferral.
- Commit: `c61dbca0`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] gRPC TCP target scheme: passthrough:/// not tcp://**
- **Found during:** Task 1 (listenGRPCTCP lifecycle test) and Task 2 (tryConnect).
- **Issue:** The plan and PATTERNS asserted `grpc.NewClient("tcp://"+addr)` works ("grpc.NewClient already accepts the tcp:// scheme"). It does NOT — `tcp://` is not a registered gRPC target scheme; the client parses it as scheme=tcp and applies a default `:443` port, producing `invalid target address … too many colons in address`. Verified against grpc-go docs.
- **Fix:** Used `passthrough:///host:port`, the correct direct analog of the existing `unix://path` form (dials the address verbatim, no resolution). Applied in both `tryConnect` and the daemon lifecycle test.
- **Files modified:** internal/forwarder/dial.go, internal/daemon/grpc_tcp_test.go, internal/cli/cli_e2e_test.go (comment).
- **Commits:** 92aff4f8, 309eed97.

**2. [Rule 2 - Missing critical functionality] testServeSession seam for daemon-side lifecycle testing**
- **Found during:** Task 1.
- **Issue:** The plan's `TestListenGRPCTCP_LoopbackLifecycle` requires a real StreamMCP round-trip over tcp, but a minimal daemon has a nil `mcpServer`, so `defaultSessionRunner` would panic. The existing `forwarderServiceHandler.serveSession` seam was not reachable from a `Daemon`-level listener.
- **Fix:** Added a test-only `Daemon.testServeSession sessionRunner` field threaded through the new shared `newForwarderServiceHandler` factory; nil in production (falls back to `defaultSessionRunner`).
- **Files modified:** internal/daemon/daemon.go, internal/daemon/grpc_tcp.go.
- **Commit:** 92aff4f8.

## Deferred Issues

- **`internal/guardrails` `TestNewReceiptID/consecutive_IDs_are_monotonically_non-decreasing`** intermittently fails under the full-tree `go test ./...` (parallel load) but passes deterministically in isolation. Pre-existing flake unrelated to this plan (zero files in `internal/guardrails` were touched). Logged in `deferred-items.md`; not fixed (out of scope).

## Critical Invariants Held

- BOTH heads (stdio forwarder + HTTP `/mcp`) remain alive and functional — no head deleted (that is 94-02).
- `git diff api/proto/` empty; `git diff go.mod go.sum` empty — verified after every task.
- gRPC TCP bind loopback-gated by default; default `daemon.grpc_addr` empty → unix-socket only.
- The parity gate is GREEN with both heads present, in its own commit preceding any deletion (94-02 `depends_on: ["94-01"]`).

## Verification Results

- `go build ./...`, `go vet ./...`: clean.
- `go build -tags integration ./...`: clean (integration false-green guard).
- `go test ./internal/cli/... ./internal/daemon/... ./internal/forwarder/... -count=1`: green.
- `HELIX_BIN=$(pwd)/helix go test -tags integration -run TestCLI_DualRunParity -count=1 ./internal/cli/...`: green (RUNS, not skip — all 5 parity verbs pass).
- `git diff --exit-code api/proto/ go.mod go.sum`: empty.

## Threat Flags

None — no security surface beyond the registered T-94-01..04 (all mitigated/accepted per the plan's threat_model and the ADR).

## Self-Check: PASSED

All created files exist on disk (grpc_tcp.go, grpc_tcp_test.go, dial_tcp_test.go, REMOTE-SCOPE-ADR.md, 94-01-SUMMARY.md) and all 5 task commits (85b8056b, 92aff4f8, 7dac4782, 309eed97, c61dbca0) are present in git history.
