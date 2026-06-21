# Phase 90: CLI One-Shot Dial Spine + Race-Free Warm Reuse - Context

**Gathered:** 2026-06-21
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

A `helix <verb>` invocation round-trips a single `tools/call` through the warm
daemon over the existing gRPC `StreamMCP` wire (zero proto change), auto-starting
the daemon on a cold host and reusing it warm thereafter — with the daemon-spawn
race fixed by a cross-process startup lock and warm reuse held to a measured
second-call latency SLO. A CLI-over-daemon E2E oracle is stood up here so every
later phase has a real-subprocess harness to extend.

**Requirements:** CLI-01, CLI-02, CLI-03, CLI-04, TEST-01

**Success Criteria (what must be TRUE):**
1. A representative verb invoked as `helix <verb>` returns the same tool result
   the MCP path returns, and `git diff api/proto/` is empty (zero proto changes).
2. A first `helix` call on a cold host spawns the daemon exactly once and a warm
   second call reuses it; the second-call p50 meets the SLO recorded in the phase.
3. N parallel cold `helix` invocations result in exactly one daemon process
   (verified by a fan-out / synctest stress test exercising the cross-process
   startup lock).
4. `helix` with no arguments exits 0 with grouped command help and opens no MCP
   stdio session.
5. The CLI-over-daemon E2E oracle runs a representative verb as a real subprocess
   against a live daemon and is green under `go test`.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped
per user setting. Use ROADMAP phase goal, success criteria, and codebase
conventions to guide decisions.

### Carried-forward roadmap constraints (from STATE.md)
- **Zero-proto invariant:** the one-shot `tools/call` rides the existing gRPC
  `StreamMCP` wire; `git diff api/proto/` must stay empty. Fix the cross-process
  daemon-spawn race (`dial.go` has no lock today) and set a 2nd-call latency SLO.
- Reuse v1.0 forwarder primitives as-is: `ConnectOrStartDaemon`, `StreamMCP`,
  `GRPCTransport`.
- Reuse `bench/runtime/subprocess` + `internal/eval/sandbox` patterns for the
  E2E oracle.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research.

</code_context>

<specifics>
## Specific Ideas

No specific requirements — discuss phase skipped. Refer to ROADMAP phase
description and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
