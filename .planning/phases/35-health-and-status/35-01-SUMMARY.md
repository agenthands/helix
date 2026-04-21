---
phase: 35-health-and-status
plan: 01
subsystem: kernel/health
tags: [health, mcp-tool, grpc, lspool]
dependency_graph:
  requires: []
  provides: [Pool.HealthSnapshot, Kernel.HealthStatus, get_health-tool, GetStatus-rpc]
  affects: [internal/daemon/daemon.go, api/proto/serena/v1/ipc.proto]
tech_stack:
  added: []
  patterns: [RLock-snapshot, FilterReport-shared-logic]
key_files:
  created:
    - internal/kernel/lspool/health.go
    - internal/kernel/lspool/health_test.go
    - internal/kernel/health/tools.go
    - internal/kernel/health/tools_test.go
  modified:
    - internal/kernel/lspool/worker.go
    - internal/kernel/lspool/circuit.go
    - internal/kernel/kernel.go
    - internal/daemon/daemon.go
    - api/proto/serena/v1/ipc.proto
    - api/proto/serena/v1/ipc.pb.go
    - api/proto/serena/v1/ipc_grpc.pb.go
decisions:
  - "RLock snapshot pattern: copy all worker/circuit data under single RLock, format outside lock (T-35-04)"
  - "Shared FilterReport function used by both MCP tool and gRPC handler to avoid logic duplication"
  - "Indexing detected as boolean only (no percentage) per research assumption A1 -- $/progress not captured"
  - "14 capability names extracted from ServerCapabilities struct field nil checks"
metrics:
  duration: 7m
  completed: 2026-04-21
---

# Phase 35 Plan 01: Health Data Layer and MCP Tool Summary

Pool.HealthSnapshot with 3-state worker classification (healthy/degraded/failed), get_health MCP tool with verbose/default filtering, and GetStatus gRPC unary RPC for CLI consumption.

## What Was Built

### Task 1: Health Data Layer (165c1c1e)
- Created `internal/kernel/lspool/health.go` with `HealthReport`, `WorkspaceHealth`, `WorkerHealth`, `CircuitHealth` structs
- Implemented `Pool.HealthSnapshot()` using RLock-snapshot pattern: copies all worker and circuit state under a single lock, formats outside
- State classification per D-12: `healthy` (Ready+Closed), `degraded` (Ready+HalfOpen), `failed` (Open/Stopped/ShuttingDown)
- Indexing detection per D-13: `WorkerInitializing` maps to `"healthy (indexing)"` with `Indexing: true`
- `capabilitiesFromServer()` extracts 14 capability names from `gen.ServerCapabilities` nil-checked fields
- Added `Worker.Command()` getter for LS command string display
- Added `CircuitBreaker.State()` getter for circuit state exposure
- Added `Kernel.HealthStatus()` bridge enriching pool snapshot with workspace language data
- 11 unit tests covering all health states, capability extraction, getters

### Task 2: MCP Tool, gRPC RPC, Daemon Wiring (46d7766c)
- Created `internal/kernel/health/` package with `get_health` MCP tool
- `GetHealthArgs` with optional `verbose` boolean parameter per D-04
- `FilterReport()` shared function: default mode hides healthy workers/closed circuits per D-09; sets "All N language servers healthy" summary per D-11; verbose mode returns all per D-10
- Updated `ipc.proto` with `GetStatus` unary RPC, `StatusRequest`, `StatusResponse` messages
- Regenerated proto Go code via `make proto`
- Added `GetStatus` gRPC handler to `forwarderServiceHandler` with kernel field
- Wired `health.RegisterTools(mcpServer, k)` in daemon bootstrap
- 5 unit tests for FilterReport covering verbose, default-hides-healthy, all-healthy-summary, degraded-shown, non-closed-circuits

## Deviations from Plan

None - plan executed exactly as written.

## Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 | 165c1c1e | feat(35-01): health data layer with Pool.HealthSnapshot, worker/circuit getters, Kernel bridge |
| 2 | 46d7766c | feat(35-01): get_health MCP tool, gRPC GetStatus RPC, daemon wiring |

## Verification Results

- `go test ./internal/kernel/lspool/ ./internal/kernel/health/ -count=1` -- all 16 tests pass
- `go vet ./internal/kernel/lspool/ ./internal/kernel/health/ ./internal/daemon/` -- no warnings
- `go build ./cmd/serena` -- binary builds successfully
- `make proto` -- proto regeneration succeeds

## Self-Check: PASSED

All files verified:
- FOUND: internal/kernel/lspool/health.go
- FOUND: internal/kernel/lspool/health_test.go
- FOUND: internal/kernel/health/tools.go
- FOUND: internal/kernel/health/tools_test.go
- FOUND: 165c1c1e (Task 1 commit)
- FOUND: 46d7766c (Task 2 commit)
