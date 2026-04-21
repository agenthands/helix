# Phase 35: Health & Status - Research

**Researched:** 2026-04-21
**Domain:** Go daemon health introspection / MCP tool + CLI
**Confidence:** HIGH

## Summary

Phase 35 adds runtime health introspection via two surfaces: a `get_health` MCP tool for agents and a `serena status` CLI command for humans. Both query the same underlying data -- the lspool worker pool state, circuit breaker status, and workspace language detection results already maintained by the kernel.

The existing codebase provides strong foundations. The `lspool.Pool` already tracks workers by language/workdir with state machines (Starting/Initializing/Ready/ShuttingDown/Stopped), circuit breakers per language with failure counts and backoff state, and `ServerCapabilities` stored on each worker after LSP initialization. The `Kernel` tracks `WorkspaceRuntime` objects with detected languages. The gRPC IPC between forwarder and daemon (used by the CLI) currently only supports `StreamMCP`; the `status` CLI needs a new unary RPC or must connect via the existing MCP transport to call `get_health`.

**Primary recommendation:** Add a `HealthSnapshot` method to `lspool.Pool` that returns a snapshot struct (under a single lock acquisition), wire it through the kernel to a new `get_health` MCP tool registered via `RegisterTools`, and expose it via CLI through either a new gRPC `GetStatus` unary RPC or by connecting to the daemon's MCP endpoint and calling the `get_health` tool directly.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: `get_health` returns structured JSON with per-workspace breakdown: root path, detected languages, and per-language LS status.
- D-02: Per-language status fields: language name, LS command, state (healthy/degraded/failed), capabilities list, indexing progress.
- D-03: `get_health` is a kernel-level tool registered via `RegisterTools`, not a skill tool. Needs direct pool/workspace access.
- D-04: Tool accepts optional `verbose` boolean parameter. Default (false) returns error-only. Verbose (true) returns full status.
- D-05: `serena status` is a new cobra subcommand connecting to daemon via gRPC, calling same health query.
- D-06: Output matches SetupPrinter style: compact lines with checkmarks/crosses and color. Table format for multi-workspace.
- D-07: If daemon not running, report "Daemon not running". Exit code 1 when any LS is failed.
- D-08: Support `--json` flag for machine-readable output (same JSON as `get_health` verbose mode).
- D-09: Default mode suppresses healthy LSes. Only surfaces: crashed/missing, circuit breaker open, indexing stalled (>60s without progress).
- D-10: Verbose mode shows all LSes including healthy ones with capabilities.
- D-11: When all healthy in default mode, return "All N language servers healthy" summary.
- D-12: Three primary states: healthy, degraded, failed.
- D-13: Indexing is transient sub-state of healthy -- reported as `healthy (indexing)` with progress percentage.
- D-14: Capabilities derived from LS ServerCapabilities response.

### Claude's Discretion
- Internal health query API between kernel and MCP tool layer
- Exact formatting of degraded/failed error messages
- Timeout duration for health check probe to each LS
- Whether to cache health state or query live each time

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| HLTH-01 | Agent can call `get_health` MCP tool to see active LSes and their status | Kernel-level tool via RegisterTools pattern; Pool.HealthSnapshot() provides worker state, circuit state |
| HLTH-02 | `get_health` reports indexing state and available capabilities per workspace | Worker.Capabilities() returns gen.ServerCapabilities; indexing state needs $/progress tracking (see Open Questions) |
| HLTH-03 | User can run `serena status` CLI to see workspace health summary | New cobra subcommand; connects to daemon via gRPC unary RPC or MCP tool call |
| HLTH-04 | Health reporting defaults to error-only mode | Filter healthy workers from response; return summary line when all healthy |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Health data collection | Kernel (lspool) | -- | Pool owns worker state, circuit breakers, capabilities |
| Health MCP tool | Kernel tool layer | -- | Kernel-level tool per D-03; needs direct pool access |
| Health CLI display | CLI (cobra) | -- | Human-readable formatting belongs in CLI layer |
| Health IPC transport | gRPC / daemon | -- | CLI must reach daemon to query live state |
| State classification | Kernel (lspool) | -- | Mapping worker states to healthy/degraded/failed is pool domain logic |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib | 1.25.1 | Language, encoding/json | Already in use [VERIFIED: go version] |
| cobra | v1.9.1 | CLI subcommand | Already in use for root + setup commands [VERIFIED: codebase] |
| fatih/color | v1.19.0 | Terminal color output | Already in use by SetupPrinter [VERIFIED: go.mod] |
| MCP Go SDK | (existing) | Tool registration | Already in use for all kernel tools [VERIFIED: codebase] |
| protobuf/gRPC | (existing) | IPC between CLI and daemon | Already in use for forwarder [VERIFIED: ipc.proto] |

### Supporting
No new libraries needed. This phase is entirely built on existing dependencies.

## Architecture Patterns

### System Architecture Diagram

```
Agent (MCP client)              Human (terminal)
       |                              |
  get_health tool call          serena status
       |                              |
       v                              v
  MCP Server                    Cobra CLI
  (daemon)                     |         |
       |                 gRPC unary   --json flag
       v                   RPC            |
  health.RegisterTools         v          v
       |               Daemon handler  JSON output
       v                      |
  Kernel.HealthStatus()       |
       |                      |
       v                      v
  Pool.HealthSnapshot()  (same query)
       |
       +---> workers map (state, capabilities, language, workdir)
       +---> circuits map (failures, state, backoff)
       +---> Kernel.workspaces (detected languages per root)
```

### Recommended Project Structure
```
internal/kernel/lspool/
  health.go              # Pool.HealthSnapshot() + HealthReport/WorkerHealth structs
internal/kernel/health/
  tools.go               # RegisterTools for get_health MCP tool + args
internal/cli/
  status.go              # Cobra status subcommand
  status_output.go       # StatusPrinter (reuses SetupPrinter patterns)
api/proto/serena/v1/
  ipc.proto              # Add GetStatus unary RPC + HealthResponse message
```

### Pattern 1: Pool Health Snapshot (query live, single lock)
**What:** A `HealthSnapshot()` method on `lspool.Pool` that captures all worker and circuit state under a single `RLock`, returning a value struct.
**When to use:** Every health query (both MCP tool and CLI).
**Why live, not cached:** Worker state changes are infrequent (seconds-to-minutes), health queries are rare (human or agent-initiated), and caching adds staleness complexity. A single `RLock` acquisition over the existing maps is cheap.

```go
// Source: derived from existing Pool.WorkerCount() pattern [VERIFIED: pool.go]
type WorkerHealth struct {
    ID           string   `json:"id"`
    Language     string   `json:"language"`
    WorkDir      string   `json:"work_dir"`
    Command      string   `json:"command"`
    State        string   `json:"state"`      // "healthy", "degraded", "failed"
    Capabilities []string `json:"capabilities"` // e.g., ["hover", "definition", "references"]
    Indexing     bool     `json:"indexing"`
    IndexPct     int      `json:"index_pct,omitempty"` // 0-100, only when indexing
}

type CircuitHealth struct {
    Language string  `json:"language"`
    State    string  `json:"state"` // "closed", "half_open", "open"
    Failures int     `json:"failures"`
}

type HealthReport struct {
    Workspaces []WorkspaceHealth `json:"workspaces"`
}

type WorkspaceHealth struct {
    Root      string         `json:"root"`
    Languages []string       `json:"languages"`
    Workers   []WorkerHealth `json:"workers"`
    Circuits  []CircuitHealth `json:"circuits"`
}
```

### Pattern 2: Kernel-Level Tool Registration
**What:** Follow exact same `RegisterTools(server, kernel, wsKeyFn)` pattern used by symbols, edit, diag packages.
**When to use:** For `get_health` tool.

```go
// Source: symbols/tools.go RegisterTools pattern [VERIFIED: codebase]
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel) {
    mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
        Name:        "get_health",
        Description: "Get workspace health status and language server states",
    }, func(ctx context.Context, req *mcpsdk.CallToolRequest, args GetHealthArgs) (*mcpsdk.CallToolResult, any, error) {
        report := k.HealthStatus(args.Verbose)
        // ... format and return
    })
}
```

### Pattern 3: gRPC Unary RPC for CLI
**What:** Add a `GetStatus` unary RPC to the proto alongside `StreamMCP`, so the CLI can query health without maintaining a full MCP session.
**When to use:** `serena status` command.
**Why not use MCP:** Starting a full MCP session (connect, initialize, call tool, disconnect) for a simple status query is heavyweight and fragile. A unary gRPC call is simpler, faster, and doesn't require MCP client logic in the CLI.

```protobuf
// Source: derived from existing ipc.proto [VERIFIED: codebase]
service ForwarderService {
  rpc StreamMCP(stream MCPMessage) returns (stream MCPMessage);
  rpc GetStatus(StatusRequest) returns (StatusResponse);
}

message StatusRequest {
  bool verbose = 1;
}

message StatusResponse {
  bytes payload = 1; // JSON-encoded HealthReport
}
```

### Pattern 4: State Classification Logic
**What:** Map worker states and circuit states to the three-tier model (healthy/degraded/failed).
**When to use:** In `Pool.HealthSnapshot()`.

```go
// Source: derived from worker.go WorkerState + circuit.go [VERIFIED: codebase]
// WorkerReady + CircuitClosed => "healthy"
// WorkerReady + CircuitHalfOpen => "degraded"
// WorkerStarting/Initializing => "healthy (starting)"
// CircuitOpen (budget exhausted) => "failed"
// WorkerStopped (unexpectedly) => "failed"
// Worker missing for a language with circuit failures => "failed"
```

### Anti-Patterns to Avoid
- **Don't probe each LS with a synthetic request for health:** The pool already tracks worker state and circuit state. Sending actual LSP requests (like `textDocument/hover` with a dummy file) would be slow, side-effectful, and unnecessary when the state machine already captures health.
- **Don't create a separate health goroutine:** Health is queried on-demand via tool/CLI, not continuously polled. No need for a background health check loop.
- **Don't bypass the pool lock:** Always use `Pool.HealthSnapshot()` which acquires `RLock` once, not per-worker queries that could produce inconsistent snapshots.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Terminal colors | Custom ANSI escape codes | fatih/color (already in use) | Handles NO_COLOR, terminal detection |
| CLI flags/subcommands | Manual flag parsing | cobra (already in use) | Consistent with existing CLI |
| gRPC service definition | Custom socket protocol | protobuf + grpc (already in use) | Type-safe, code-generated |
| JSON serialization | Manual string building | encoding/json (stdlib) | Struct tags handle everything |
| Capability name mapping | Hardcoded string lists | Reflection on ServerCapabilities struct | Non-nil provider fields = supported capabilities |

## Common Pitfalls

### Pitfall 1: Pool Lock Contention
**What goes wrong:** Health queries hold the pool lock while formatting output, blocking worker acquisition.
**Why it happens:** Temptation to iterate workers while holding the lock and doing string formatting.
**How to avoid:** `HealthSnapshot()` copies all needed data into value structs under a single short `RLock`, then formatting happens outside the lock.
**Warning signs:** High-latency `AcquireLease` calls when health is being queried.

### Pitfall 2: Indexing Progress Without $/progress Tracking
**What goes wrong:** D-13 requires indexing progress percentage, but the current worker implementation does not capture LSP `$/progress` notifications.
**Why it happens:** `$/progress` is an LS-to-client notification that requires a notification handler on the JSON-RPC connection. The current `jsonrpc.Conn` may not route these to the worker.
**How to avoid:** For v1, report indexing as boolean only (worker in Initializing state = "indexing"). Defer percentage tracking to a follow-up if `$/progress` handler isn't already wired. Mark as "indexing" if worker state is `WorkerInitializing`.
**Warning signs:** Always 0% indexing progress reported.

### Pitfall 3: CLI Daemon Detection
**What goes wrong:** `serena status` hangs trying to connect to a non-running daemon.
**Why it happens:** gRPC dial to Unix socket blocks or retries by default.
**How to avoid:** Check socket existence first (`os.Stat` on socket path), use short dial timeout (2-3s), and return "Daemon not running" immediately per D-07. Set exit code 1.
**Warning signs:** `serena status` hangs when daemon is down.

### Pitfall 4: Empty Workspace State
**What goes wrong:** `get_health` called before `activate_project` returns empty/misleading results.
**Why it happens:** Kernel has no workspace runtimes if no project is activated yet.
**How to avoid:** Return a clear message: "No workspaces activated. Run activate_project first." Don't return an error -- this is a valid state.
**Warning signs:** Agent gets confused by empty health response.

### Pitfall 5: Proto Regeneration
**What goes wrong:** Modified `.proto` file but forgot to regenerate Go code.
**Why it happens:** `protoc` is a manual step.
**How to avoid:** Check for existing `make proto` or `go generate` target. Run regeneration as part of the implementation task.
**Warning signs:** Compilation errors about missing types.

## Code Examples

### Extracting Capabilities from ServerCapabilities
```go
// Source: protocol/gen/tsprotocol.go ServerCapabilities struct [VERIFIED: codebase]
func capabilitiesFromServer(caps gen.ServerCapabilities) []string {
    var result []string
    if caps.HoverProvider != nil { result = append(result, "hover") }
    if caps.DefinitionProvider != nil { result = append(result, "definition") }
    if caps.ReferencesProvider != nil { result = append(result, "references") }
    if caps.ImplementationProvider != nil { result = append(result, "implementation") }
    if caps.TypeDefinitionProvider != nil { result = append(result, "type_definition") }
    if caps.DocumentSymbolProvider != nil { result = append(result, "document_symbol") }
    if caps.WorkspaceSymbolProvider != nil { result = append(result, "workspace_symbol") }
    if caps.CodeActionProvider != nil { result = append(result, "code_action") }
    if caps.DocumentFormattingProvider != nil { result = append(result, "formatting") }
    if caps.RenameProvider != nil { result = append(result, "rename") }
    if caps.CallHierarchyProvider != nil { result = append(result, "call_hierarchy") }
    if caps.CompletionProvider != nil { result = append(result, "completion") }
    if caps.SignatureHelpProvider != nil { result = append(result, "signature_help") }
    if caps.DeclarationProvider != nil { result = append(result, "declaration") }
    return result
}
```

### SetupPrinter-Style Status Output
```go
// Source: internal/cli/setup_output.go [VERIFIED: codebase]
// Reuse same printer patterns:
printer.Success("go (gopls) healthy — hover, definition, references, ...")
printer.Failure("python (pylsp) failed — circuit breaker open (3 failures)")
printer.Info("All 2 language servers healthy")
```

### Existing Worker State Access
```go
// Source: internal/kernel/lspool/worker.go [VERIFIED: codebase]
w.State()          // WorkerState atomic
w.Language()       // string
w.WorkDir()        // string
w.Capabilities()   // gen.ServerCapabilities (RLock-protected)
w.Metrics()        // *WorkerMetrics (StartedAt, LastUsedAt, ColdStartMs)
w.Pid()            // int (-1 if not started)
```

### Existing Circuit Breaker Access
```go
// Source: internal/kernel/lspool/circuit.go [VERIFIED: codebase]
cb.Failures()          // int (mutex-protected)
cb.CanAttempt()        // bool
cb.BackoffDuration()   // time.Duration
// Circuit state constants: CircuitClosed=0, CircuitHalfOpen=1, CircuitOpen=2
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Phase 34 binary-existence health check | Phase 35 live daemon health check | Now | Moves from "is the binary on PATH?" to "is the LS running and responsive?" |
| No health MCP tool | `get_health` kernel tool | Now | Agents can self-diagnose LS issues without user intervention |
| No CLI status | `serena status` command | Now | Users get quick visibility into daemon state |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `$/progress` notifications are NOT currently captured by the jsonrpc.Conn worker handler | Pitfalls | If already captured, we can report real indexing progress; if not, we need to defer or add handler |
| A2 | The pool's `circuits` map keys match worker language strings | Architecture Patterns | If keying differs, circuit-to-worker correlation in health report would be wrong |
| A3 | Worker `lsCommand` field is accessible for display in health output | Code Examples | May need to add a getter; `Worker.lsCommand` is unexported |

## Open Questions

1. **$/progress notification handling**
   - What we know: Workers store `ServerCapabilities` after `initialize`, but the JSON-RPC connection's handling of LS-to-client notifications like `$/progress` is unclear.
   - What's unclear: Whether the current `jsonrpc.Conn` dispatches `$/progress` to any handler, or silently discards it.
   - Recommendation: For Phase 35, treat `WorkerInitializing` state as "indexing" without percentage. Defer `$/progress`-based progress tracking. This satisfies D-13 ("when available from LSP $/progress").

2. **Worker command export**
   - What we know: `Worker.lsCommand` is an unexported field. D-02 requires showing the LS command.
   - What's unclear: Whether adding a `Command() string` getter is the right approach, or if the health snapshot should resolve command from the language registry.
   - Recommendation: Add a simple `Command() string` getter to Worker. It's the most direct path and follows the existing ID/Language/WorkDir getter pattern.

3. **gRPC vs MCP for CLI health query**
   - What we know: The CLI currently only connects via gRPC `StreamMCP` for forwarder use. Status needs a simpler path.
   - What's unclear: Whether to add a new unary gRPC RPC or have the CLI open an MCP session to call `get_health`.
   - Recommendation: Add unary `GetStatus` RPC to `ipc.proto`. It's simpler, faster, and avoids MCP session overhead for a CLI utility command.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | All | Yes | 1.25.1 | -- |
| protoc | Proto regeneration | Yes | 34.1 | -- |
| protoc-gen-go | Proto regeneration | Yes | (installed) | -- |
| protoc-gen-go-grpc | Proto regeneration | Yes | (installed) | -- |

**Missing dependencies:** None.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | None (Go convention) |
| Quick run command | `go test ./internal/kernel/lspool/ ./internal/kernel/health/ ./internal/cli/ -run Health -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| HLTH-01 | get_health returns LS status | unit | `go test ./internal/kernel/health/ -run TestGetHealthTool -count=1` | Wave 0 |
| HLTH-01 | Pool.HealthSnapshot returns worker states | unit | `go test ./internal/kernel/lspool/ -run TestHealthSnapshot -count=1` | Wave 0 |
| HLTH-02 | Capabilities and indexing in response | unit | `go test ./internal/kernel/health/ -run TestHealthCapabilities -count=1` | Wave 0 |
| HLTH-03 | serena status CLI output | unit | `go test ./internal/cli/ -run TestStatusCommand -count=1` | Wave 0 |
| HLTH-04 | Error-only default, verbose shows all | unit | `go test ./internal/kernel/health/ -run TestHealthVerbose -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/kernel/lspool/ ./internal/kernel/health/ ./internal/cli/ -count=1`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green + `go vet ./...` before verify

### Wave 0 Gaps
- [ ] `internal/kernel/lspool/health_test.go` -- covers Pool.HealthSnapshot()
- [ ] `internal/kernel/health/tools_test.go` -- covers MCP tool registration + response format
- [ ] `internal/cli/status_test.go` -- covers CLI output formatting + daemon-down detection

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | -- |
| V3 Session Management | no | -- |
| V4 Access Control | no | Health tool is read-only, no sensitive data |
| V5 Input Validation | yes | Validate `verbose` boolean arg via MCP SDK typed args |
| V6 Cryptography | no | -- |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Health response leaks internal paths | Information Disclosure | Already accepted: workspace paths are user's own project paths |
| Admin listener already loopback-only | All | validateAdminAddr in telemetry.go enforces loopback [VERIFIED: codebase] |

## Sources

### Primary (HIGH confidence)
- `internal/kernel/lspool/pool.go` -- Pool structure, worker map, circuit map, lock patterns
- `internal/kernel/lspool/worker.go` -- WorkerState enum, Capabilities(), State()
- `internal/kernel/lspool/circuit.go` -- CircuitBreaker, Failures(), state constants
- `internal/kernel/lspool/metrics.go` -- MetricsSink interface, circuit state constants
- `internal/kernel/kernel.go` -- Kernel structure, workspaces map, Pool() accessor
- `internal/kernel/workspace.go` -- WorkspaceRuntime, Languages(), detected languages
- `internal/daemon/daemon.go` -- Tool registration in New(), Run() lifecycle
- `internal/daemon/telemetry.go` -- Admin /healthz endpoint pattern
- `internal/cli/root.go` -- Cobra command structure, subcommand registration
- `internal/cli/setup_output.go` -- SetupPrinter colored output pattern
- `internal/cli/setup_health.go` -- Phase 34 binary health check (predecessor)
- `internal/kernel/symbols/tools.go` -- RegisterTools pattern for kernel tools
- `internal/mcp/server.go` -- MCP server, AddTool pattern
- `api/proto/serena/v1/ipc.proto` -- Existing gRPC service definition
- `protocol/gen/tsprotocol.go` -- ServerCapabilities struct fields

### Secondary (MEDIUM confidence)
None needed -- all findings from codebase inspection.

### Tertiary (LOW confidence)
None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in use, no new dependencies
- Architecture: HIGH -- clear patterns established by existing kernel tools and CLI commands
- Pitfalls: HIGH -- identified from direct code inspection of lock patterns and state machines

**Research date:** 2026-04-21
**Valid until:** 2026-05-21 (stable -- internal architecture, no external dependency churn)
