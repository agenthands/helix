---
phase: 01-foundation
plan: 03
subsystem: mcp-runtime
tags: [mcp, grpc, protobuf, stdio-forwarder, streamable-http, tool-registry]

# Dependency graph
requires:
  - phase: 01-foundation/01-02
    provides: "Daemon lifecycle, workspace registry, config loader, CLI skeleton"
provides:
  - "MCP server with SDK integration and dummy tools (ping, echo, activate_project)"
  - "Dynamic tool registry with runtime add/remove"
  - "Structured MCP error types with domain sentinel errors"
  - "gRPC IPC proto for forwarder-daemon communication"
  - "GRPCTransport bridging gRPC streams to MCP SDK via io.Pipe"
  - "Stdio forwarder with daemon auto-start (gopls pattern)"
  - "Streamable HTTP endpoint for direct MCP connections"
affects: [02-lsp-kernel, 03-skills, 04-profiles]

# Tech tracking
tech-stack:
  added: [github.com/modelcontextprotocol/go-sdk v1.5.0, google.golang.org/grpc v1.80.0, google.golang.org/protobuf v1.36.11]
  patterns: [grpc-bidirectional-stream, io-pipe-transport-bridge, daemon-auto-start, receiving-middleware]

key-files:
  created:
    - internal/mcp/server.go
    - internal/mcp/registry.go
    - internal/mcp/errors.go
    - internal/mcp/middleware.go
    - internal/mcp/session.go
    - internal/mcp/grpc_transport.go
    - internal/mcp/server_test.go
    - internal/forwarder/forwarder.go
    - internal/forwarder/dial.go
    - internal/forwarder/forwarder_test.go
    - api/proto/serena/v1/ipc.proto
    - api/proto/serena/v1/ipc.pb.go
    - api/proto/serena/v1/ipc_grpc.pb.go
  modified:
    - internal/daemon/daemon.go
    - internal/cli/root.go
    - internal/config/loader.go
    - go.mod
    - go.sum

key-decisions:
  - "Used MCP Go SDK v1.5.0 (latest) instead of v1.4.1 from plan -- latest stable with same API"
  - "jsonschema struct tags use plain description (not key=value) per SDK v1.5.0 requirements"
  - "GRPCTransport uses io.Pipe bridge to IOTransport rather than raw Connection impl -- cleaner, SDK-native"
  - "Force-added generated .pb.go files to git despite gitignore -- needed for build without protoc"
  - "First gRPC message replayed into transport pipe to preserve MCP initialization handshake"

patterns-established:
  - "MCP tool registration: define Args struct with json/jsonschema tags, use mcpsdk.AddTool generic"
  - "Transport bridging: io.Pipe pairs + goroutine pumps for custom transport -> IOTransport"
  - "Middleware: mcpsdk.Middleware type wrapping MethodHandler for cross-cutting concerns"
  - "Daemon auto-start: try connect, start background process, poll for readiness"

requirements-completed: [MCP-01, MCP-02, MCP-03, MCP-04, MCP-05, MCP-06, MCP-07, DMN-03, DMN-04]

# Metrics
duration: 7min
completed: 2026-04-07
---

# Phase 01 Plan 03: MCP Runtime Summary

**MCP server with official SDK, dummy tools (ping/echo/activate_project), gRPC forwarder-daemon IPC, stdio forwarder with auto-start, and Streamable HTTP endpoint**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-07T13:21:38Z
- **Completed:** 2026-04-07T13:28:50Z
- **Tasks:** 2 of 2 auto tasks completed (Task 3 is human-verify checkpoint)
- **Files modified:** 19

## Accomplishments
- MCP server wrapping official Go SDK with three registered tools and JSON Schema definitions
- Dynamic tool registry supporting runtime add/remove without server restart
- Structured error types with five domain sentinel errors and ErrorDetail JSON serialization
- gRPC bidirectional streaming proto for forwarder-daemon IPC over Unix socket
- GRPCTransport bridging gRPC streams to MCP SDK via io.Pipe pairs
- Stdio forwarder that auto-starts daemon if not running, with keepalive for crash detection
- Streamable HTTP endpoint at /mcp for direct browser/remote MCP connections
- Full CLI wiring: --serve runs daemon, --mode=stdio runs forwarder, --mode=auto defaults to forwarder

## Task Commits

Each task was committed atomically:

1. **Task 1: MCP server, tool registry, errors, and middleware** - `9c52c1c9` (feat)
2. **Task 2: gRPC proto, IPC bridge, forwarder with daemon auto-start** - `a3778ff4` (feat)

## Files Created/Modified
- `internal/mcp/server.go` - MCP server wrapping SDK with tool registration
- `internal/mcp/registry.go` - Dynamic tool registry with thread-safe add/remove
- `internal/mcp/errors.go` - Domain sentinel errors and structured ErrorDetail
- `internal/mcp/middleware.go` - Logging middleware for request tracing
- `internal/mcp/session.go` - Per-session state tracking type
- `internal/mcp/grpc_transport.go` - gRPC-to-MCP SDK bridge via io.Pipe
- `internal/mcp/server_test.go` - Tests for registry, errors, session, server creation
- `internal/forwarder/forwarder.go` - Stdio-to-gRPC proxy with session management
- `internal/forwarder/dial.go` - Connection, auto-start, and wait-for-daemon logic
- `internal/forwarder/forwarder_test.go` - Tests for connection, timeout, session ID
- `api/proto/serena/v1/ipc.proto` - gRPC service definition for ForwarderService
- `api/proto/serena/v1/ipc.pb.go` - Generated protobuf Go code
- `api/proto/serena/v1/ipc_grpc.pb.go` - Generated gRPC Go code
- `internal/daemon/daemon.go` - Updated with MCP server, gRPC server, HTTP listener
- `internal/cli/root.go` - Wired forwarder mode for stdio/auto
- `internal/config/loader.go` - Added DefaultSocketPath() helper
- `go.mod` / `go.sum` - Added MCP SDK, gRPC, protobuf dependencies

## Decisions Made
- Used MCP Go SDK v1.5.0 (latest stable) instead of plan's v1.4.1 -- same API surface, newer bugfixes
- jsonschema struct tags use plain description strings, not key=value format, per SDK requirements
- GRPCTransport implemented via io.Pipe + IOTransport bridge (Approach B from plan) rather than raw Connection interface -- cleaner separation, uses SDK's own newline-delimited JSON framing
- Force-added generated .pb.go files despite gitignore to ensure builds work without protoc installed
- First gRPC message is replayed into the transport pipe since daemon's StreamMCP handler reads it for session ID

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed jsonschema struct tag format**
- **Found during:** Task 1 (MCP server creation)
- **Issue:** Plan used `jsonschema:"description=..."` format but SDK v1.5.0 requires plain description without key= prefix
- **Fix:** Changed all tags to `jsonschema:"Description text"` format
- **Files modified:** internal/mcp/server.go
- **Verification:** Tests pass, AddTool no longer panics
- **Committed in:** 9c52c1c9

**2. [Rule 3 - Blocking] Removed duplicate shutdown method**
- **Found during:** Task 2 (daemon wiring)
- **Issue:** daemon.go rewrite included a shutdown() method that already existed in shutdown.go
- **Fix:** Removed duplicate from daemon.go, kept existing shutdown.go implementation
- **Files modified:** internal/daemon/daemon.go
- **Verification:** go build ./... succeeds
- **Committed in:** a3778ff4

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking)
**Impact on plan:** Both fixes necessary for compilation and correctness. No scope creep.

## Issues Encountered
- Generated .pb.go files were in .gitignore; force-added them since they're needed for builds without protoc

## Pending Verification (Task 3 Checkpoint)

Task 3 is a human-verify checkpoint for end-to-end MCP communication. Manual verification steps:

1. Build binary: `go build -o serena ./cmd/serena`
2. Test daemon direct mode with `--serve` flag
3. Test MCP tool listing via stdio forwarder
4. Test ping tool call via stdio forwarder
5. Test Streamable HTTP at /mcp endpoint
6. Verify graceful shutdown removes socket file

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- MCP runtime complete with both stdio and HTTP transports
- Tool registry ready for real tool implementations in Phase 2
- Workspace activation wired end-to-end (activate_project tool -> Registry)
- Ready for LSP kernel integration in Phase 2

---
*Phase: 01-foundation*
*Completed: 2026-04-07*
