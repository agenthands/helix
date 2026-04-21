# Phase 35: Health & Status - Pattern Map

**Mapped:** 2026-04-21
**Files analyzed:** 8 new/modified files
**Analogs found:** 8 / 8

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/kernel/lspool/health.go` | service | request-response | `internal/kernel/lspool/pool.go` (WorkerCount/LeaseCount) | exact |
| `internal/kernel/health/tools.go` | controller | request-response | `internal/kernel/symbols/tools.go` | exact |
| `internal/cli/status.go` | controller | request-response | `internal/cli/setup.go` | exact |
| `internal/cli/status_output.go` | utility | transform | `internal/cli/setup_output.go` | exact |
| `api/proto/serena/v1/ipc.proto` | config | request-response | `api/proto/serena/v1/ipc.proto` (existing) | exact |
| `internal/daemon/daemon.go` | controller | request-response | `internal/daemon/daemon.go` (existing, modify) | exact |
| `internal/kernel/lspool/health_test.go` | test | request-response | `internal/kernel/lspool/pool_test.go` | exact |
| `internal/kernel/health/tools_test.go` | test | request-response | `internal/kernel/lspool/pool_test.go` | role-match |

## Pattern Assignments

### `internal/kernel/lspool/health.go` (service, request-response)

**Analog:** `internal/kernel/lspool/pool.go`

**Imports pattern** (lines 1-13):
```go
package lspool

import (
	"sync"

	gen "github.com/postfix/serena/protocol/gen"
)
```

**Core pattern -- RLock snapshot** (pool.go lines 217-220, WorkerCount pattern):
```go
// WorkerCount returns the number of active workers.
func (p *Pool) WorkerCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers)
}
```

The new `HealthSnapshot()` must follow this exact same pattern: acquire `p.mu.RLock()`, iterate `p.workers` and `p.circuits` maps, copy all needed data into value structs, release lock, then return. No formatting or string building under the lock.

**Worker state access** (worker.go lines 317-357):
```go
w.State()          // WorkerState (atomic load)
w.Language()       // string
w.WorkDir()        // string
w.ID()             // string
w.Capabilities()   // gen.ServerCapabilities (RLock-protected)
w.Pid()            // int (-1 if not started)
```

Note: `w.lsCommand` is unexported (worker.go line 132). A new `Command() string` getter must be added following the same pattern as the existing getters above.

**Circuit state access** (circuit.go lines 146-157):
```go
cb.Failures()          // int (mutex-protected)
cb.BackoffDuration()   // time.Duration
// State constants from metrics.go:
// CircuitClosed float64 = 0
// CircuitHalfOpen float64 = 1
// CircuitOpen float64 = 2
```

Note: `cb.state` is unexported. A new `State() float64` getter (or equivalent) is needed to read the circuit state for health snapshots.

**WorkerState enum** (worker.go lines 22-34):
```go
const (
	WorkerStarting WorkerState = iota
	WorkerInitializing
	WorkerReady
	WorkerShuttingDown
	WorkerStopped
)
```

State classification logic for health:
- `WorkerReady` + `CircuitClosed` => "healthy"
- `WorkerReady` + `CircuitHalfOpen` => "degraded"
- `WorkerInitializing` => "healthy (indexing)" per D-13
- `CircuitOpen` (budget exhausted) => "failed"
- `WorkerStopped` (unexpected) => "failed"

---

### `internal/kernel/health/tools.go` (controller, request-response)

**Analog:** `internal/kernel/symbols/tools.go`

**Imports pattern** (lines 1-15):
```go
package health

import (
	"context"
	"encoding/json"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	"github.com/postfix/serena/internal/kernel"
	"github.com/postfix/serena/internal/mcp"
)
```

**RegisterTools signature** (symbols/tools.go line 84):
```go
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey) {
	tracer := k.Tracer()
	// ... register individual tools
}
```

Note: health tool does NOT need `wsKeyFn` since it queries all workspaces, not a specific one. Signature should be:
```go
func RegisterTools(server *mcp.SerenaMCPServer, k *kernel.Kernel) {
```

**Tool registration pattern** (symbols/tools.go lines 186-209):
```go
func registerGoToDefinition(server *mcp.SerenaMCPServer, k *kernel.Kernel, wsKeyFn func() workspace.WorkspaceKey, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "go_to_definition",
		Description: "Go to the definition of a symbol at a given position",
	}, kernel.WrapToolSpan(tracer, "go_to_definition", func(ctx context.Context, req *mcpsdk.CallToolRequest, args GoToDefinitionArgs) (*mcpsdk.CallToolResult, any, error) {
		// ... handler body
		return textResult(formatLocations(locs)), nil, nil
	}))
	server.Registry().Register(&mcp.ToolDef{Name: "go_to_definition", Description: "Go to the definition of a symbol at a given position"})
}
```

Key elements to copy:
1. `mcpsdk.AddTool(server.SDK(), ...)` with typed args struct
2. `kernel.WrapToolSpan(tracer, toolName, handler)` wrapper
3. `server.Registry().Register(...)` for catalog entry
4. Return `*mcpsdk.CallToolResult` with `TextContent`

**Args struct pattern** (symbols/tools.go lines 20-24):
```go
type GoToDefinitionArgs struct {
	Path string `json:"path" jsonschema:"File path"`
	Line int    `json:"line" jsonschema:"Line number (0-indexed)"`
	Col  int    `json:"column" jsonschema:"Column number (0-indexed)"`
}
```

For health:
```go
type GetHealthArgs struct {
	Verbose bool `json:"verbose,omitempty" jsonschema:"Show all LSes including healthy ones"`
}
```

**Helper functions** (symbols/tools.go lines 99-114):
```go
func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: text},
		},
	}
}

func errorResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: msg},
		},
		IsError: true,
	}
}
```

---

### `internal/cli/status.go` (controller, request-response)

**Analog:** `internal/cli/setup.go`

**Imports pattern** (setup.go lines 1-13):
```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)
```

**Cobra subcommand constructor** (setup.go lines 16-38):
```go
func newSetupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup [client]",
		Short: "Register Serena as MCP server for a coding agent",
		Long:  `Register Serena as an MCP server...`,
		Args:      cobra.MaximumNArgs(1),
		RunE:      runSetup,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Flags().Bool("global", false, "Register globally (user-scoped) instead of project-scoped")
	return cmd
}
```

For status:
```go
func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show workspace health and language server status",
		RunE:  runStatus,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().Bool("json", false, "Output as JSON")
	cmd.Flags().Bool("verbose", false, "Show all LSes including healthy ones")
	return cmd
}
```

**Registration in root.go** (root.go line 50):
```go
// Subcommands
rootCmd.AddCommand(newSetupCommand())
```

Add: `rootCmd.AddCommand(newStatusCommand())`

**Daemon connection pattern** (root.go lines 92-104, runForwarder):
```go
func runForwarder(cmd *cobra.Command) error {
	socketPath, _ := cmd.Flags().GetString("socket")
	if socketPath == "" {
		socketPath = config.DefaultSocketPath()
	}
	return forwarder.RunForwarder(cmd.Context(), socketPath, logger)
}
```

The status command needs the same socket path resolution to connect via gRPC, then call the new `GetStatus` unary RPC.

---

### `internal/cli/status_output.go` (utility, transform)

**Analog:** `internal/cli/setup_output.go`

**Full file** (setup_output.go lines 1-48):
```go
package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

type SetupPrinter struct {
	DryRun bool
}

func (p *SetupPrinter) Success(format string, args ...any) {
	green := color.New(color.FgGreen)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", green.Sprint("\u2713"), msg)
}

func (p *SetupPrinter) Failure(format string, args ...any) {
	red := color.New(color.FgRed)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", red.Sprint("\u2717"), msg)
}

func (p *SetupPrinter) Info(format string, args ...any) {
	blue := color.New(color.FgBlue)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", blue.Sprint("->"), msg)
}
```

StatusPrinter should reuse or embed SetupPrinter. Key pattern: all output to `os.Stderr`, uses `fatih/color`, respects `NO_COLOR` automatically.

---

### `api/proto/serena/v1/ipc.proto` (config, modify)

**Analog:** `api/proto/serena/v1/ipc.proto` (self)

**Existing proto** (full file):
```protobuf
syntax = "proto3";

package serena.v1;

option go_package = "github.com/postfix/serena/api/proto/serena/v1;serenav1";

service ForwarderService {
  rpc StreamMCP(stream MCPMessage) returns (stream MCPMessage);
}

message MCPMessage {
  bytes payload = 1;
  string session_id = 2;
}
```

Add a new unary RPC and messages:
```protobuf
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

After modifying, regenerate with `protoc` (check for existing Makefile target).

---

### `internal/daemon/daemon.go` (controller, modify)

**Analog:** `internal/daemon/daemon.go` (self)

**Tool registration section** (daemon.go lines 226-228):
```go
symbols.RegisterTools(mcpServer, k, wsKeyFn)
edit.RegisterTools(mcpServer, k, bodyExtractor, diagStore, wsKeyFn)
fileops.RegisterTools(mcpServer, workspaceRootFn, observability.Tracer())
```

Add health tool registration in the same block:
```go
health.RegisterTools(mcpServer, k)
```

**Import addition** (daemon.go line 28):
```go
"github.com/postfix/serena/internal/kernel/symbols"
```

Add:
```go
"github.com/postfix/serena/internal/kernel/health"
```

**gRPC handler pattern** (daemon.go lines 525-556):
```go
type forwarderServiceHandler struct {
	serenav1.UnimplementedForwarderServiceServer
	mcpServer *serenaMCP.SerenaMCPServer
	logger    *slog.Logger
}

func (h *forwarderServiceHandler) StreamMCP(stream serenav1.ForwarderService_StreamMCPServer) error {
	// ... implementation
}
```

Add `GetStatus` method on the same handler struct. The handler needs access to the kernel to call `HealthSnapshot()`. Add kernel field:
```go
type forwarderServiceHandler struct {
	serenav1.UnimplementedForwarderServiceServer
	mcpServer *serenaMCP.SerenaMCPServer
	kernel    *kernel.Kernel
	logger    *slog.Logger
}
```

---

### `internal/kernel/lspool/health_test.go` (test)

**Analog:** `internal/kernel/lspool/pool_test.go`

**Test file pattern** (pool_test.go lines 1-13):
```go
package lspool

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/postfix/serena/internal/langregistry"
	"github.com/postfix/serena/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

Note: tests are in `package lspool` (not `_test`), giving access to unexported fields. Uses `testify/assert` and `testify/require`.

**Test utilities** (testutil_test.go):
```go
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
```

**Test helpers** (pool_test.go lines 29-52):
```go
func testKey() workspace.WorkspaceKey {
	return workspace.WorkspaceKey{
		RepoRoot: "/tmp/test-project",
		Language: "go",
	}
}

func testPoolConfig() PoolConfig {
	return PoolConfig{
		BaseTTL:               1,
		CeilingTTL:            10,
		MaxWorkers:            5,
		RSSHardCapMB:          2048,
		PressureCheckInterval: 1,
	}
}

func testRegistry() *langregistry.Registry {
	reg, _ := langregistry.NewRegistry()
	return reg
}
```

---

### `internal/kernel/health/tools_test.go` (test)

**Analog:** `internal/kernel/lspool/pool_test.go` (partial -- no direct kernel tool test analog exists)

Will need mock kernel or mock health snapshot. Follow same `testify/assert`+`require` pattern, `package health` (or `health_test`).

---

## Shared Patterns

### MCP Tool Result Formatting
**Source:** `internal/kernel/symbols/tools.go` lines 99-114
**Apply to:** `internal/kernel/health/tools.go`
```go
func textResult(text string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: text},
		},
	}
}

func errorResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: msg},
		},
		IsError: true,
	}
}
```

### Kernel Tool Registration (AddTool + Registry.Register)
**Source:** `internal/kernel/symbols/tools.go` lines 186-209
**Apply to:** `internal/kernel/health/tools.go`

Every kernel tool must:
1. Call `mcpsdk.AddTool(server.SDK(), tool, handler)` for runtime dispatch
2. Call `server.Registry().Register(toolDef)` for catalog entry
3. Wrap handler with `kernel.WrapToolSpan(tracer, name, fn)` for tracing

### Cobra Subcommand Pattern
**Source:** `internal/cli/setup.go` lines 16-38, `internal/cli/root.go` line 50
**Apply to:** `internal/cli/status.go`

1. Constructor `newXxxCommand() *cobra.Command` with flags
2. `RunE` function that extracts flags and orchestrates
3. `rootCmd.AddCommand(newXxxCommand())` in root.go

### Colored Terminal Output
**Source:** `internal/cli/setup_output.go` lines 1-48
**Apply to:** `internal/cli/status_output.go`

All output to `os.Stderr`. Uses `fatih/color` with `color.New(color.FgGreen)` etc. Three methods: `Success` (green checkmark), `Failure` (red cross), `Info` (blue arrow).

### gRPC Handler on forwarderServiceHandler
**Source:** `internal/daemon/daemon.go` lines 525-556
**Apply to:** `internal/daemon/daemon.go` (modification for GetStatus)

Embed `serenav1.UnimplementedForwarderServiceServer`, implement method matching proto service definition.

### Pool Lock Patterns (RLock for reads)
**Source:** `internal/kernel/lspool/pool.go` lines 217-228
**Apply to:** `internal/kernel/lspool/health.go`

All read-only pool queries use `p.mu.RLock()`/`defer p.mu.RUnlock()`, copy data out, return.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (none) | -- | -- | All files have close analogs in the codebase |

## Metadata

**Analog search scope:** `internal/kernel/`, `internal/cli/`, `internal/daemon/`, `api/proto/`
**Files scanned:** 15 analog candidates read
**Pattern extraction date:** 2026-04-21
