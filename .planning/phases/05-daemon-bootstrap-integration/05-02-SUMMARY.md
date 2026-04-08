---
phase: 05-daemon-bootstrap-integration
plan: 02
subsystem: daemon
tags: [daemon, bootstrap, kernel, skills, mcp, middleware, profile]
dependency_graph:
  requires: ["05-01"]
  provides: ["fully-wired-daemon", "30+-mcp-tools", "profile-middleware"]
  affects: ["internal/daemon", "internal/mcp", "internal/profile", "internal/cli"]
tech_stack:
  added: []
  patterns: ["Caddy-style blank imports for init()", "platform-specific build tags for pressure", "generic skill tool dispatch via ExecuteTool interface", "callback pattern for activate_project kernel wiring"]
key_files:
  created:
    - internal/daemon/imports.go
    - internal/daemon/pressure_darwin.go
    - internal/daemon/pressure_linux.go
    - internal/daemon/pressure_other.go
  modified:
    - internal/daemon/daemon.go
    - internal/daemon/shutdown.go
    - internal/daemon/daemon_test.go
    - internal/mcp/server.go
    - internal/profile/profile.go
    - internal/cli/root.go
decisions:
  - "daemon.New returns (*Daemon, error) for fail-fast on core subsystems (langregistry, profile)"
  - "Platform pressure via build-tagged helpers (darwin/linux/other) rather than runtime detection"
  - "Kernel skill adapters skipped during skill tool registration (registered via RegisterTools directly)"
  - "AddSkillTool uses generic mcpsdk.AddTool with map[string]any for auto schema generation"
  - "ProfileStore implements ProfileResolver via ToolDescriptionOverrides method"
  - "SetActivateCallback pattern decouples MCP server from kernel dependency"
metrics:
  duration: 9min
  completed: "2026-04-08T14:28:00Z"
  tasks: 1
  files: 10
---

# Phase 05 Plan 02: Daemon Skill Wiring Summary

Full daemon bootstrap wiring: kernel creation, skill initialization, 30+ MCP tool registration, profile middleware, and graceful shutdown with kernel-first ordering.

## What Was Done

### Task 1: Wire daemon bootstrap with kernel, skills, tools, middleware, and config

Rewrote the daemon bootstrap to create all subsystems and register all tools:

**daemon.go** -- Complete rewrite of `New()`:
1. Creates `langregistry.Registry` and `Installer` (fail-fast)
2. Creates platform-specific `MemoryPressure` via build-tagged helpers
3. Converts `WorkerPoolConfig` to `lspool.PoolConfig` with defaults
4. Creates `kernel.Kernel` with all dependencies
5. Creates `DiagnosticStore` and `BodyExtractor`
6. Creates `SerenaMCPServer`
7. Resolves profile via `config.ResolveProfile` (fail-fast)
8. Initializes all skills via `skill.InitAll` (degraded mode on failure)
9. Registers 9 symbol tools, 6 edit tools, 6 file ops tools, 3 diagnostic tools directly
10. Registers memory (7 tools) and workflow (2 tools) via generic `ExecuteTool` dispatch
11. Wires profile skill session provider
12. Installs `ProfileFilterMiddleware` on MCP server
13. Sets activate callback for kernel workspace activation

**shutdown.go** -- Two-phase shutdown per D-10:
1. Phase 1: Stop kernel (drain workers, close LS processes)
2. Phase 2: Close listeners, clean up socket

**imports.go** -- Blank imports for 7 skill packages triggering init() registration

**pressure_darwin.go / pressure_linux.go / pressure_other.go** -- Platform-specific memory pressure helpers

**server.go** -- New MCP server capabilities:
- `SetActivateCallback` for kernel wiring on project activation
- `AddSkillTool` for generic skill tool registration with auto schema
- `SkillToolExecutor` interface for skills with `ExecuteTool`
- Updated `activate_project` handler to call activation callback

**profile.go** -- `ToolDescriptionOverrides` method on `ProfileStore` satisfying `mcp.ProfileResolver`

**cli/root.go** -- Updated for new `daemon.New` signature returning error

**daemon_test.go** -- Updated for new `daemon.New` signature

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] daemon.New signature change required caller updates**
- **Found during:** Task 1
- **Issue:** Changing `New()` to return `(*Daemon, error)` broke `internal/cli/root.go` and `internal/daemon/daemon_test.go`
- **Fix:** Updated both callers to handle the error return
- **Files modified:** internal/cli/root.go, internal/daemon/daemon_test.go

**2. [Rule 1 - Bug] MCP SDK raw AddTool requires input schema**
- **Found during:** Task 1 verification
- **Issue:** `Server.AddTool` (raw) panics without input schema; skill tools used the raw API
- **Fix:** Switched to generic `mcpsdk.AddTool` with `map[string]any` type for auto schema generation
- **Files modified:** internal/mcp/server.go

## Known Stubs

None -- all tools are fully wired with real implementations.

## Verification

- `go build ./cmd/serena/` -- exits 0
- `go vet ./internal/daemon/ ./internal/mcp/ ./internal/profile/ ./internal/cli/` -- exits 0
- `go test ./internal/... -count=1 -short -timeout 120s` -- all 18 packages pass

## Self-Check: PASSED

All created files exist, commit f9bbdc15 verified, all key patterns confirmed in target files.
