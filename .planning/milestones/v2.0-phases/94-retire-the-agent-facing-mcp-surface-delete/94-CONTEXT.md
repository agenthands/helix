# Phase 94: Retire the Agent-Facing MCP Surface (DELETE) - Context

**Gathered:** 2026-06-21
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

With the CLI proven as the sole agent surface via a dual-run parity test, DELETE the two agent-facing MCP heads — the stdio MCP forwarder head and the Streamable-HTTP `/mcp` transport (`--mode http` MCP serving) — while RETAINING the daemon, the gRPC IPC, `StreamMCP`, `GRPCTransport`, the 5 middlewares, and all tool handlers behind the wire. ADD an optional gRPC TCP bind for split-host CLI↔daemon use (loopback/unix-socket default, non-loopback opt-in and gated per the v1.2 admin-addr pattern), plus an explicit remote/multi-client scope decision record (ADR) replacing the removed HTTP transport's only network-transparent topology.

Requirements: RETIRE-01, RETIRE-02, RETIRE-03, RETIRE-04.

**Strangler-fig safety gate (load-bearing, do NOT violate):** the dual-run parity test (RETIRE-03) comparing CLI output against the pre-removal MCP path for a representative tool set MUST be GREEN in the commit IMMEDIATELY BEFORE the deletion commit. Parity proof precedes deletion — sequence the plans so the parity test lands and passes first, then the deletion lands in a later commit/plan. Never delete a head before parity is proven green.
</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion (bounded by carried STATE constraints)
All implementation choices are at Claude's discretion — discuss was skipped per user setting. Bounded by the carried v2.0 roadmap constraints:
- **DELETE only the two agent-facing heads:** stdio MCP forwarder head + Streamable-HTTP `/mcp` (`--mode http` MCP serving). Do NOT remove the daemon, the gRPC IPC, `StreamMCP`, `GRPCTransport`, the 5 middlewares, or any tool handler — those stay behind the wire (the CLI dials them).
- **RETIRE-03 is the gate:** dual-run parity must be green in the commit before deletion.
- **RETIRE-04 (optional gRPC TCP bind) rides this phase:** loopback/unix-socket default; non-loopback opt-in, gated per the v1.2 admin-addr pattern (do not bind a non-loopback address by default).
- Provide a short ADR / decision record for the remote/multi-client scope that the removed HTTP transport previously covered.
- After deletion: no reachable stdio MCP server path; `/mcp` endpoint gone; `--mode http` no longer serves MCP; the CLI still dials the daemon (include the Windows local-dial smoke).
- Honor the docgen-drift lesson and zero-new-deps / proto-change discipline where applicable (proto stays; `StreamMCP` is retained, so `api/proto/` likely unchanged — assert it).
</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Known anchors: the stdio forwarder head lives under `internal/forwarder/` and `cmd/helix` `--mode=stdio`; the Streamable-HTTP `/mcp` transport + `--mode http` live in the daemon/MCP runtime (`internal/daemon/`, `internal/mcp/`); the gRPC `StreamMCP` wire is in `api/proto/serena/v1/` and is RETAINED; the v1.2 admin-addr loopback-gating pattern is the model for RETIRE-04's opt-in TCP bind. Phases 90–93 already made the CLI the proven, taught, set-up surface.
</code_context>

<specifics>
## Specific Ideas

No specific requirements beyond the ROADMAP — discuss skipped. Refer to the Phase 94 ROADMAP section's 3 success criteria: (1) parity green in the pre-deletion commit; (2) no reachable stdio MCP path, `/mcp` gone, `--mode http` no longer serves MCP, CLI still dials (Windows local-dial smoke); (3) CLI can target a configured TCP daemon endpoint when opted in, default stays local unix socket / named pipe.
</specifics>

<deferred>
## Deferred Ideas

Docs/identity rewrite + docgen regen against the frozen-and-now-MCP-free surface is Phase 95 (next), not this phase.
</deferred>
