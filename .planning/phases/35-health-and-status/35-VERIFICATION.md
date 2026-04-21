---
phase: 35-health-and-status
verified: 2026-04-21T20:45:00Z
status: human_needed
score: 10/10
overrides_applied: 0
human_verification:
  - test: "Run serena status against a live daemon with at least one workspace activated"
    expected: "Colored output showing workspace health with checkmarks for healthy LSes"
    why_human: "Requires running daemon with real LS workers to validate end-to-end gRPC flow and terminal color rendering"
  - test: "Run serena status --json against a live daemon"
    expected: "Valid JSON output matching HealthReport schema with all workspace data"
    why_human: "Requires running daemon to verify JSON output format end-to-end"
  - test: "Run serena status when daemon is not running"
    expected: "Prints 'Daemon not running' to stderr and exits with code 1"
    why_human: "Requires verifying exit code behavior with no daemon socket present"
  - test: "Call get_health MCP tool from an agent session with verbose=false when all LSes healthy"
    expected: "Returns JSON with summary 'All N language servers healthy' and empty workers arrays"
    why_human: "Requires live MCP session with active workspaces to verify agent-facing behavior"
---

# Phase 35: Health & Status Verification Report

**Phase Goal:** Agents and users can inspect workspace health and LS status at any time
**Verified:** 2026-04-21T20:45:00Z
**Status:** human_needed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Agent can call get_health MCP tool and receive per-workspace LS status | VERIFIED | `health.RegisterTools` wired in daemon.go:241; tool handler calls `k.HealthStatus()` which calls `pool.HealthSnapshot()`; returns JSON with WorkspaceHealth/WorkerHealth structs |
| 2 | get_health response includes capabilities and indexing state per LS | VERIFIED | `capabilitiesFromServer()` extracts 14 capability names from ServerCapabilities nil checks; `WorkerInitializing` maps to `"healthy (indexing)"` with `Indexing: true` |
| 3 | Default mode returns only unhealthy LSes; verbose returns all | VERIFIED | `FilterReport()` in health/tools.go filters workers with `State != "healthy"` and circuits with `State != "closed"` when verbose=false; verbose=true returns as-is |
| 4 | When all LSes healthy, default mode returns summary line | VERIFIED | `FilterReport` sets `report.Summary = fmt.Sprintf("All %d language servers healthy", totalWorkers)` when allHealthy && totalWorkers > 0; test `TestFilterReport_AllHealthySummary` passes |
| 5 | gRPC GetStatus unary RPC is available for CLI consumption | VERIFIED | `rpc GetStatus(StatusRequest) returns (StatusResponse)` in ipc.proto:15; handler at daemon.go:564 calls `h.kernel.HealthStatus()` + `health.FilterReport` |
| 6 | User can run serena status and see workspace health summary | VERIFIED | `newStatusCommand()` registered in root.go:51; `runStatus` connects via gRPC, unmarshals HealthReport, prints via StatusPrinter |
| 7 | If daemon not running, user sees 'Daemon not running' with exit code 1 | VERIFIED | status.go:46-56 checks `os.Stat(socketPath)` + `net.DialTimeout` with 2s timeout; prints "Daemon not running" to stderr + `os.Exit(1)` |
| 8 | serena status --json outputs same JSON as get_health verbose mode | VERIFIED | status.go:86-101 re-queries with `Verbose: true` when json flag set; marshals with `json.MarshalIndent` to stdout |
| 9 | Exit code 1 when any LS is in failed state | VERIFIED | status.go:109 calls `printer.HasFailures(&report)` which checks for `State == "failed"` or circuit `State == "open"`; `os.Exit(1)` on true |
| 10 | Default mode shows only unhealthy LSes, verbose shows all (CLI) | VERIFIED | CLI passes `verbose` flag to `StatusRequest`; daemon applies `FilterReport`; StatusPrinter renders filtered report |

**Score:** 10/10 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/kernel/lspool/health.go` | Pool.HealthSnapshot() returning HealthReport struct | VERIFIED | 260 lines; HealthReport, WorkspaceHealth, WorkerHealth, CircuitHealth structs; classifyWorker with 3-state model; capabilitiesFromServer with 14 capabilities |
| `internal/kernel/health/tools.go` | get_health MCP tool registration | VERIFIED | 109 lines; RegisterTools, FilterReport, GetHealthArgs; tool handler with verbose filtering |
| `internal/daemon/daemon.go` | health tool registration + GetStatus gRPC handler | VERIFIED | `health.RegisterTools(mcpServer, k)` at line 241; `GetStatus` handler at line 564; kernel field on forwarderServiceHandler |
| `api/proto/serena/v1/ipc.proto` | GetStatus unary RPC definition | VERIFIED | `rpc GetStatus(StatusRequest) returns (StatusResponse)` with StatusRequest.verbose and StatusResponse.payload |
| `internal/cli/status.go` | Cobra status subcommand with --json and --verbose flags | VERIFIED | 114 lines; newStatusCommand with flags; runStatus with gRPC client, daemon detection, JSON output |
| `internal/cli/status_output.go` | StatusPrinter for colored terminal output | VERIFIED | 111 lines; PrintReport, HasFailures, Success/Failure/Warning/Info methods; Writer field for testability |
| `internal/cli/status_test.go` | Unit tests for status output formatting | VERIFIED | 10 tests covering PrintReport (all-healthy, verbose, failures, degraded, circuit), HasFailures, command flags |
| `internal/kernel/lspool/health_test.go` | Unit tests for health snapshot | VERIFIED | 8 tests covering empty pool, healthy/degraded/failed/indexing/stopped workers, capabilities extraction |
| `internal/kernel/health/tools_test.go` | Unit tests for FilterReport | VERIFIED | 5 tests covering verbose, default-hides-healthy, all-healthy-summary, degraded-shown, non-closed-circuits |
| `internal/kernel/lspool/worker.go` | Worker.Command() getter | VERIFIED | `func (w *Worker) Command() string` at line 357 |
| `internal/kernel/lspool/circuit.go` | CircuitBreaker.State() getter | VERIFIED | `func (cb *CircuitBreaker) State() float64` at line 153 with mutex protection |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| internal/kernel/health/tools.go | internal/kernel/kernel.go | `k.HealthStatus()` | WIRED | tools.go:28 calls `k.HealthStatus()` |
| internal/kernel/kernel.go | internal/kernel/lspool/health.go | `k.pool.HealthSnapshot()` | WIRED | kernel.go:127 calls `k.pool.HealthSnapshot()` |
| internal/daemon/daemon.go | internal/kernel/health/tools.go | `health.RegisterTools(mcpServer, k)` | WIRED | daemon.go:241 |
| internal/cli/status.go | api/proto/serena/v1/ipc_grpc.pb.go | `client.GetStatus()` | WIRED | status.go:74 calls gRPC GetStatus |
| internal/cli/root.go | internal/cli/status.go | `rootCmd.AddCommand(newStatusCommand())` | WIRED | root.go:51 |
| internal/daemon/daemon.go | kernel.Kernel | `forwarderServiceHandler.kernel` field | WIRED | daemon.go:532 kernel field typed `*kernel.Kernel` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| health/tools.go | HealthReport | k.HealthStatus() -> pool.HealthSnapshot() | Pool iterates live workers map + circuits map under RLock | FLOWING |
| cli/status.go | lspool.HealthReport | gRPC GetStatus -> daemon handler -> k.HealthStatus() | Same pool snapshot via gRPC transport | FLOWING |
| cli/status_output.go | *lspool.HealthReport | Passed from runStatus after gRPC unmarshal | Renders workers/circuits from report | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Binary builds with status command | `go build ./cmd/serena` | BUILD_OK | PASS |
| Health snapshot tests pass | `go test ./internal/kernel/lspool/ -run TestHealth -count=1` | 6 pass | PASS |
| Capabilities extraction tests pass | `go test ./internal/kernel/lspool/ -run TestCapabilities -count=1` | 2 pass | PASS |
| FilterReport tests pass | `go test ./internal/kernel/health/ -run TestFilter -count=1` | 5 pass | PASS |
| StatusPrinter tests pass | `go test ./internal/cli/ -run TestStatus -count=1` | 10 pass (including flags test) | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| HLTH-01 | 35-01 | Agent can call `get_health` MCP tool to see active LSes and their status | SATISFIED | get_health tool registered with MCP server; returns WorkerHealth with state per language |
| HLTH-02 | 35-01 | `get_health` reports indexing state and available capabilities per workspace | SATISFIED | WorkerHealth.Capabilities from capabilitiesFromServer (14 caps); Indexing bool set for WorkerInitializing/WorkerStarting |
| HLTH-03 | 35-02 | User can run `serena status` CLI to see workspace health summary | SATISFIED | Cobra subcommand registered; connects via gRPC GetStatus; StatusPrinter formats colored output |
| HLTH-04 | 35-01, 35-02 | Health reporting defaults to error-only mode | SATISFIED | FilterReport hides healthy workers/closed circuits by default; summary line when all healthy; CLI and MCP tool both default to non-verbose |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | -- | -- | -- | No anti-patterns detected in any phase artifact |

### Human Verification Required

### 1. Live Daemon Status Output

**Test:** Start Serena daemon with a workspace activated, then run `serena status`
**Expected:** Colored output with checkmarks for healthy LSes, workspace root, language names
**Why human:** Requires running daemon with real LS workers; verifies end-to-end gRPC flow and terminal color rendering

### 2. JSON Output Format

**Test:** Run `serena status --json` against a live daemon
**Expected:** Valid JSON matching HealthReport schema with workspaces, workers, capabilities
**Why human:** Requires live daemon to verify complete JSON output format

### 3. Daemon Not Running Detection

**Test:** Ensure daemon is stopped, run `serena status`
**Expected:** Prints "Daemon not running" to stderr, exits with code 1
**Why human:** Requires verifying exit code and stderr output in real terminal environment

### 4. Agent MCP Tool Behavior

**Test:** From an MCP client session, call `get_health` with `verbose: false` when all LSes are healthy
**Expected:** JSON response with `summary: "All N language servers healthy"` and filtered workspace data
**Why human:** Requires live MCP session with activated workspaces

### Gaps Summary

No gaps found. All 10 observable truths verified against the codebase. All artifacts exist, are substantive (no stubs), are wired to their consumers, and have real data flowing through them. All 22 unit tests pass across 3 test files. The binary compiles successfully.

Human verification is needed for end-to-end behavior with a live daemon, which cannot be tested programmatically without starting the full daemon infrastructure.

---

_Verified: 2026-04-21T20:45:00Z_
_Verifier: Claude (gsd-verifier)_
