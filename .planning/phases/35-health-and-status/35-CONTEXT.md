# Phase 35: Health & Status - Context

**Gathered:** 2026-04-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Agents and users can inspect workspace health and LS status at any time. Covers HLTH-01 through HLTH-04: `get_health` MCP tool for agents, `serena status` CLI for users, error-only default mode.

</domain>

<decisions>
## Implementation Decisions

### MCP Tool Interface (get_health)
- **D-01:** `get_health` returns structured JSON with per-workspace breakdown: root path, detected languages, and per-language LS status.
- **D-02:** Per-language status fields: language name, LS command, state (healthy/degraded/failed), capabilities list (hover, completion, references, etc.), indexing progress (if applicable).
- **D-03:** `get_health` is a kernel-level tool registered via `RegisterTools` (same pattern as symbol/edit/diag tools), not a skill tool. It needs direct access to the worker pool and workspace state.
- **D-04:** Tool accepts optional `verbose` boolean parameter. Default (false) returns error-only output per HLTH-04. Verbose (true) returns full status for all LSes including healthy ones.

### CLI Status Command (serena status)
- **D-05:** `serena status` is a new cobra subcommand (like `setup`). It connects to the running daemon via gRPC and calls the same health query the MCP tool uses.
- **D-06:** Output matches Phase 34's `SetupPrinter` style: compact lines with checkmarks/crosses and color. Table format for multi-workspace scenarios.
- **D-07:** If daemon is not running, report "Daemon not running" rather than failing silently. Exit code 1 when any LS is in failed state (useful for CI scripts).
- **D-08:** Support `--json` flag for machine-readable output (same JSON as `get_health` verbose mode).

### Error-Only Default Mode
- **D-09:** Default mode suppresses healthy LSes. Only surfaces: crashed/missing LSes, circuit breaker open, indexing stalled (>60s without progress).
- **D-10:** Verbose mode (MCP: `verbose: true`, CLI: `--verbose`) shows all LSes including healthy ones with capabilities.
- **D-11:** When all LSes are healthy in default mode, return a single summary line: "All N language servers healthy" rather than empty output.

### LS Status Granularity
- **D-12:** Three primary states: `healthy` (responding, initialized), `degraded` (circuit-tripped, timeouts, partial capabilities), `failed` (crashed, missing binary, unreachable).
- **D-13:** Indexing is a transient sub-state of `healthy` — reported as `healthy (indexing)` with progress percentage when available from LSP `$/progress` notifications.
- **D-14:** Capabilities reported as a list of supported LSP methods (hover, definition, references, etc.) derived from the LS's `ServerCapabilities` response. This directly satisfies HLTH-02.

### Claude's Discretion
- Internal health query API between kernel and MCP tool layer
- Exact formatting of degraded/failed error messages
- Timeout duration for health check probe to each LS
- Whether to cache health state or query live each time

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture & Existing Health Infrastructure
- `internal/kernel/lspool/pool.go` — Worker pool with circuit breaking, adaptive TTL. Health status derives from pool state.
- `internal/obs/obs.go` — Observability provider with metrics. Health endpoint context.
- `internal/obs/metrics.go` — Prometheus metrics including lspool gauges (worker counts, circuit state).
- `internal/daemon/daemon.go` — Daemon bootstrap, admin listener setup. Health tool registers here.
- `internal/daemon/telemetry.go` — Admin listener with /healthz, /metrics endpoints already wired.

### Phase 34 Foundation (predecessor)
- `internal/cli/setup_health.go` — Binary-existence health check (Phase 34 scope). Phase 35 replaces this with daemon-based live health.
- `internal/cli/setup.go` — SetupPrinter and setup command structure. CLI status reuses printer patterns.
- `internal/cli/root.go` — Cobra root command. `status` subcommand registers here.

### Requirements
- `.planning/REQUIREMENTS.md` — HLTH-01 through HLTH-04 requirements
- `.planning/ROADMAP.md` — Phase 35 success criteria

### MCP & Tool Registration
- `internal/mcp/server.go` — MCP server and tool registration patterns
- `internal/kernel/symbols/` — Example of kernel tool registration via RegisterTools pattern

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `lspool.Pool` — Worker pool already tracks worker state, circuit breaker status, and language associations. Health can query this directly.
- `obs.Metrics` — Prometheus gauges for `lspool_workers` and circuit state already exist. Health tool can read these.
- `SetupPrinter` in `internal/cli/setup_output.go` — Color output with Success/Failure/Info methods. Reuse for `serena status` CLI output.
- `/healthz` endpoint in admin listener — Already returns basic health. Can be extended or health logic shared.
- `workspace.Workspace` — Workspace struct tracks root path and language associations.

### Established Patterns
- Kernel tools use `RegisterTools(server, kernel, pool)` with typed args and `mcpsdk.AddTool` — follow same for `get_health`.
- Cobra subcommands added via `rootCmd.AddCommand()` — follow same for `status`.
- gRPC IPC between forwarder and daemon — `status` CLI connects same way.
- Error-only defaults established by Phase 34's `SetupPrinter` pattern (informational, not blocking).

### Integration Points
- `internal/kernel/` — New health query method on Kernel or Pool
- `internal/cli/root.go` — Add `status` subcommand
- `internal/daemon/daemon.go` — Register `get_health` tool
- `api/proto/serena/v1/` — May need health-specific gRPC message for status CLI

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 35-health-and-status*
*Context gathered: 2026-04-21*
