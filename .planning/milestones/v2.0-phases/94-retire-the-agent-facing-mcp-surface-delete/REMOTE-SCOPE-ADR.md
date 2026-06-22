# ADR: Remote / Multi-Client Scope After the MCP-Head Retirement (RETIRE-04)

**Status:** Accepted
**Date:** 2026-06-22
**Phase:** 94 — Retire the Agent-Facing MCP Surface (DELETE)
**Plan:** 94-01 (the strangler-fig pre-deletion wave)
**Deciders:** Helix maintainers
**Related:** RETIRE-02 (deletes the HTTP `/mcp` head, plan 94-02), RETIRE-03 (the
dual-run parity gate), REMOTE-01 / REMOTE-02 (deferred follow-on work)

---

## Context

Before this milestone, Helix exposed three agent-facing surfaces into the daemon:

1. The **stdio forwarder head** (`internal/forwarder/forwarder.go` `RunForwarder`)
   — a per-process stdin/stdout MCP pump proxying to the daemon over gRPC-unix.
2. The **Streamable-HTTP `/mcp` head** (`internal/daemon/daemon.go` `listenHTTP`,
   bound on `daemon.http_addr`, default `:8080`) — the SDK's HTTP MCP handler.
3. The **gRPC-over-unix wire** (`internal/daemon/daemon.go` `listenSocket`) — the
   transport the CLI one-shot dial path (`forwarder.CallTool`) rides.

The v2.0 milestone retires the two **agent-facing MCP heads** (#1, #2) in favor of
the CLI as the sole sufficient surface. The retained gRPC-over-unix wire (#3) is
NOT an agent-facing MCP head — it is the CLI↔daemon IPC and stays.

Deleting the HTTP `/mcp` head removes the **only network-transparent topology**
Helix shipped: the unix socket is same-host-only, so an HTTP client on another host
(or another container sharing a network namespace but not the socket dir) had `/mcp`
as its only way in. The CLI dial path, as it stood before this plan, was unix-only.

**Decision needed:** what, if anything, replaces the network-transparent topology
the deleted `/mcp` head covered, and what is explicitly deferred?

---

## Decision

1. **Replace `/mcp`'s network reach with an OPTIONAL, loopback-gated gRPC TCP
   listener** (`internal/daemon/grpc_tcp.go` `listenGRPCTCP`), bound on the new
   `daemon.grpc_addr` config (CLI `--grpc-addr`). It reuses the **same**
   `ForwarderService.StreamMCP` RPC the unix socket serves — **no proto change, no
   new dependency**. The CLI dial path (`tryConnect`) gained a TCP branch that
   targets `passthrough:///host:port` when an endpoint is configured.

2. **Default stays the local unix socket.** `daemon.grpc_addr` is empty by default
   (`internal/config/defaults.go`); when empty the daemon binds only the unix
   socket and the CLI dials unix. The TCP listener is strictly opt-in.

3. **The gRPC TCP bind is loopback-only.** `validateGRPCAddr` (a line-for-line
   analog of the v1.2 `validateAdminAddr` admin-addr gate) refuses any non-loopback
   address **before `net.Listen`**, using `net.IP.IsLoopback()` as the single source
   of truth. The refusal error points at **REMOTE-01**.

4. **No remote auth, no TLS, no multi-client fan-out in this milestone.** The gRPC
   TCP path is plaintext and unauthenticated; loopback-only gating is the entire
   trust boundary. Authenticated/TLS remote bind is deferred to **REMOTE-01**;
   multi-client fan-out / session multiplexing is deferred to **REMOTE-02**.

---

## Security Domain (STRIDE)

Mirrors the 94-RESEARCH "Security Domain" framing. The new network-facing trust
boundary is `CLI process → daemon (gRPC TCP)`; the operator-supplied bind address
crossing into `net.Listen` is the second boundary.

| Threat ID | Category | Component | Disposition | Mitigation |
|-----------|----------|-----------|-------------|------------|
| T-94-01 | Elevation of Privilege / Tampering | `listenGRPCTCP` bound non-loopback, unauthenticated | **mitigate** | Default empty (unix-only); `validateGRPCAddr` refuses non-loopback before `net.Listen` (loopback-only opt-in); error points at REMOTE-01; this ADR records remote scope as deferred. |
| T-94-02 | Information Disclosure | plaintext tool traffic over TCP | **mitigate** | Loopback-only confines traffic to the host (no on-wire exposure beyond the loopback interface); the `--grpc-addr` flag help + this ADR document the no-TLS caveat; mTLS deferred to REMOTE-01. |
| T-94-03 | Spoofing | unauthenticated TCP client impersonating the CLI | **accept** | Loopback-only scope makes same-host trust the boundary; full client auth deferred to REMOTE-01. |
| T-94-04 | Denial of Service | malformed bind address (host:port parse failure) | **mitigate** | `validateGRPCAddr` surfaces `SplitHostPort` errors as a clean startup error; empty addr is a no-op (no listener). |

**Why loopback-only is sufficient for this milestone:** confining the bind to the
loopback interface means an attacker must already have code-execution on the host to
reach the endpoint — at which point they already have the unix socket and the
filesystem. The TCP opt-in therefore adds split-process / split-container-on-same-
host reach (its intended use) **without** widening the trust boundary beyond
same-host, which is exactly the property the deleted unix-only path had. Genuine
cross-host remote access requires authentication and transport security, which is
why it is deferred rather than shipped half-built.

---

## Consequences

**Positive**
- The CLI is provably the sole sufficient surface: RETIRE-03's `TestCLI_DualRunParity`
  shows the CLI subprocess yields the same load-bearing loci as the pre-removal MCP
  path for a representative verb set, against one live daemon with both heads alive.
- The one network-transparent topology `/mcp` covered (same-host, cross-process) is
  preserved via the gated gRPC TCP opt-in — zero new deps, zero proto change.
- Attack surface is unchanged by default (unix-only); the TCP listener is opt-in and
  loopback-confined.

**Negative / deferred**
- No cross-host remote access in v2.0 (was effectively available, though unauthenticated,
  via `/mcp` on a routable `http_addr`). Deferred to **REMOTE-01** (authn + TLS).
- No multi-client fan-out or session multiplexing over a single TCP endpoint.
  Deferred to **REMOTE-02**.
- Operators who relied on `/mcp` for genuine remote (non-loopback) access must wait
  for REMOTE-01; the loopback gate will refuse their old bind address with an error
  pointing here.

---

## Deferred Work (referenced by the loopback-refusal error)

- **REMOTE-01** — Authenticated + TLS non-loopback gRPC bind. The `validateGRPCAddr`
  refusal message names this ID so an operator hitting the gate is routed to the plan
  that will lift it.
- **REMOTE-02** — Multi-client fan-out / session multiplexing over the TCP endpoint.
