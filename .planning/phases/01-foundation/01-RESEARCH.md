# Phase 1: Foundation - Research

**Researched:** 2026-04-07
**Domain:** Go daemon skeleton, MCP runtime, gRPC IPC, stdio forwarder, Streamable HTTP, repo migration
**Confidence:** HIGH

## Summary

Phase 1 delivers the foundational Go binary: a persistent daemon that accepts MCP connections via stdio forwarder and Streamable HTTP, dispatches to registered tools (initially echo/ping stubs), and returns structured responses. The repo migrates Python code to `legacy/`.

The MCP Go SDK (v1.4.1) provides server creation, tool registration with struct-tag schema inference, dynamic add/remove of tools, Streamable HTTP handler (implements `http.Handler`), middleware support, and session iteration. It does NOT provide per-session tool filtering natively -- this must be implemented via middleware (intercept `tools/list` and `tools/call`, filter by session state). The SDK's `Server.Run()` handles stdio; `NewStreamableHTTPHandler()` handles HTTP with automatic session management. Both can be used simultaneously on the same `*Server`.

For forwarder-to-daemon IPC, gRPC over Unix domain socket is the locked decision. The forwarder receives MCP JSON-RPC on stdin, wraps each message in a gRPC call to the daemon, and streams responses back. This is a thin bidirectional stream -- a single gRPC service with `StreamMCP(stream MCPMessage)` covers it. The daemon side unwraps gRPC messages and feeds them into the MCP SDK via a custom `Transport` implementation. Proto definitions live in `api/proto/`.

**Primary recommendation:** Build bottom-up: repo migration, then daemon skeleton with signal handling and socket cleanup, then MCP server with dummy tools over both transports, then gRPC IPC layer with forwarder, then config loading, then integration test end-to-end.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Binary name is `serena` (same as current Python CLI)
- **D-02:** Flat CLI with flags, not subcommand style. E.g. `serena --mode=stdio`, `serena --serve`
- **D-03:** Daemon auto-starts from forwarder (gopls pattern). No explicit `serena daemon start` required. Forwarder checks for running daemon, launches if not found.
- **D-04:** All Python-related code moves to `legacy/` folder -- src/, test/, scripts/, pyproject.toml, docs/, etc. Go project owns the repo root.
- **D-05:** CI preserved in legacy/ so Python tests can still run if needed during transition
- **D-06:** Legacy code serves as reference for Go reimplementation -- consult it for tool behavior, LSP quirk handling, config schema
- **D-07:** gRPC for internal communication between stdio forwarder and daemon over Unix socket
- **D-08:** Proto definitions live in `api/proto/` (top-level, Google convention)
- **D-09:** Keep `.serena/` as project config directory name
- **D-10:** YAML format for config files (continuity with current Serena)
- **D-11:** Full config scope matching current Serena -- contexts, modes, tool overrides, memory config, LS-specific settings
- **D-12:** Standard Go layout -- `cmd/serena/`, `internal/` (private), `pkg/` (public API if any)
- **D-13:** Proto files in `api/proto/`
- **D-14:** Daemon Unix socket at `/tmp/serena-$UID/daemon.sock` (gopls-similar pattern)
- **D-15:** Windows uses named pipes as Unix socket equivalent (full Windows support in v1)
- **D-16:** Configurable log format -- text by default, `--json` flag for structured JSON (Go slog)
- **D-17:** Logs go to stderr (forwarder) + rotated file (daemon at ~/.serena/logs/)
- **D-18:** MCP error codes with structured JSON detail (cause, suggestion) surfaced to clients
- **D-19:** Internal Go errors use sentinel errors + wrapping (errors.Is/As pattern) with domain-specific sentinels (ErrLSCrashed, ErrSessionExpired, etc.)

### Claude's Discretion
No areas deferred to Claude's discretion -- all decisions locked.

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MCP-01 | Server exposes tools via stdio transport with JSON-RPC framing | MCP SDK `Server.Run()` with `StdioTransport` handles this directly; forwarder proxies via gRPC |
| MCP-02 | Server exposes tools via Streamable HTTP transport with session management | `NewStreamableHTTPHandler()` implements `http.Handler`; automatic session creation per connection |
| MCP-03 | Tools are listed with JSON Schema definitions for agent discovery | MCP SDK `AddTool` with struct tags auto-generates JSON Schema; `tools/list` built into SDK |
| MCP-04 | Errors returned as structured MCP error codes with parseable detail | SDK returns `*mcp.CallToolResult` with `IsError` flag + `Content`; extend with structured error middleware |
| MCP-05 | Long-running operations support cancellation via MCP protocol | SDK supports context cancellation; MCP cancellation notification handling via middleware |
| MCP-06 | Server sends progress notifications during indexing | `ServerSession.NotifyProgress()` sends `notifications/progress` to clients |
| MCP-07 | Dynamic tool registry allows adding/removing tools at runtime | SDK has `Server.AddTool()` and `Server.RemoveTools(names)` -- fully supported natively |
| DMN-01 | Persistent supervisor daemon manages workspace registry, LS workers, caches | Daemon skeleton with errgroup-based subsystem management; workspace registry is a map of workspace key to state |
| DMN-02 | Daemon survives MCP client disconnects without losing warm state | Daemon is separate process; sessions detach on disconnect, workspace state persists in daemon memory |
| DMN-03 | Thin stdio forwarder proxies MCP traffic to daemon via Unix socket | gRPC bidirectional stream over Unix socket; forwarder is ~200 lines of Go |
| DMN-04 | Daemon serves Streamable HTTP directly for multi-client scenarios | `StreamableHTTPHandler` mounted on daemon's HTTP listener; runs alongside socket listener |
| DMN-05 | Workspace state keyed by repo root + language + toolchain fingerprint | Workspace key type: `hash(repoRoot, language, toolchainVersion)` -- struct with these fields |
| DMN-06 | Session state keyed by MCP session + dirty buffer overlay + mode/capability profile | Session struct wrapping SDK's `ServerSession` with additional state fields |
| DMN-12 | Graceful shutdown with signal handling and child process cleanup | `signal.NotifyContext` for SIGTERM/SIGINT; errgroup cancellation; two-phase shutdown |
| DMN-13 | Stale Unix socket files detected and cleaned up on daemon start | Startup probe: try connect to existing socket; if refused, unlink and proceed; if connected, exit with message |
| WRK-01 | User can activate a project by pointing at a repo path | `activate_project` tool stub that registers workspace key in daemon's workspace registry |
| WRK-04 | Project-local configuration via .serena/ directory | koanf v2 with ordered providers: defaults -> global ~/.serena/serena_config.yml -> project .serena/project.yml -> CLI flags |
| MIG-01 | Current Python Serena moved to legacy/ folder | `git mv` of src/, test/, scripts/, pyproject.toml, docs/, uv.lock, compose.yaml, Dockerfile to legacy/ |
| MIG-02 | Go project initialized at repo root with standard Go module layout | `go mod init github.com/postfix/serena`; cmd/serena/, internal/, api/proto/ directories |
| MIG-03 | Single binary distribution | `go build -o serena ./cmd/serena` produces single binary |
</phase_requirements>

## Standard Stack

### Core (Phase 1 specific)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) | v1.4.1 | MCP server, tool registry, stdio + HTTP transports | Official SDK by Anthropic + Google. Struct-tag tool schemas, dynamic add/remove, `StreamableHTTPHandler`, middleware. Verified 2026-04-07. |
| [google.golang.org/grpc](https://github.com/grpc/grpc-go) | v1.80.0 | Forwarder-to-daemon IPC over Unix socket | Decision D-07. gRPC natively supports Unix sockets via `unix://` scheme. Typed proto definitions. |
| [google.golang.org/protobuf](https://pkg.go.dev/google.golang.org/protobuf) | v1.36.11 | Proto message generation | Required by gRPC. `protoc-gen-go` + `protoc-gen-go-grpc` installed on system. |
| [knadh/koanf/v2](https://github.com/knadh/koanf) | v2.3.4 | Layered config: defaults -> global -> project -> CLI | Decision D-10/D-11. Lightweight, preserves key case (unlike Viper), modular providers. |
| [spf13/cobra](https://github.com/spf13/cobra) | v1.10.2 | CLI flag parsing for flat flag-based CLI | Even for flat CLI (D-02), Cobra's root command provides flag parsing, help generation, shell completion. Use root command only, no subcommands. |
| `log/slog` (stdlib) | Go 1.25 | Structured logging with text/JSON handlers | Decision D-16. Zero dependencies. `slog.NewTextHandler` / `slog.NewJSONHandler` controlled by `--json` flag. |
| `gopkg.in/yaml.v3` | v3.0.1 | YAML parsing for config files | Used by koanf's YAML provider. Standard Go YAML library. |
| [stretchr/testify](https://github.com/stretchr/testify) | v1.11.1 | Test assertions and mocks | `assert`/`require` packages for readable tests. Mock package for interface testing. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `golang.org/x/sync/errgroup` | latest | Parallel subsystem startup with error propagation | Daemon startup: socket listener + HTTP listener + health checks run in errgroup |
| `os/signal` (stdlib) | Go 1.25 | Signal handling for graceful shutdown | `signal.NotifyContext(ctx, SIGTERM, SIGINT)` -- daemon entry point |
| `os/exec` (stdlib) | Go 1.25 | Spawn daemon process from forwarder | Forwarder auto-starts daemon (D-03) |
| `net` (stdlib) | Go 1.25 | Unix domain socket listener | `net.Listen("unix", path)` for daemon socket |
| `net/http` (stdlib) | Go 1.25 | HTTP server for Streamable HTTP MCP endpoint | `http.Server` with `StreamableHTTPHandler` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Cobra (root cmd only) | stdlib `flag` | Cobra gives help formatting, shell completion, version flag for free. Minimal overhead for flat CLI. |
| gRPC for IPC | Raw Unix socket + custom framing | gRPC gives typed messages, bidirectional streaming, code generation. Worth the dependency for IPC correctness. |
| gRPC for IPC | MCP SDK custom transport directly over Unix socket | Cleaner but couples forwarder to MCP SDK. gRPC separates IPC framing from MCP protocol. |

**Installation:**
```bash
go mod init github.com/postfix/serena

# Core
go get github.com/modelcontextprotocol/go-sdk@v1.4.1
go get github.com/knadh/koanf/v2@v2.3.4
go get github.com/knadh/koanf/providers/file
go get github.com/knadh/koanf/parsers/yaml
go get github.com/spf13/cobra@v1.10.2
go get google.golang.org/grpc@v1.80.0
go get google.golang.org/protobuf@v1.36.11
go get golang.org/x/sync
go get gopkg.in/yaml.v3

# Testing
go get github.com/stretchr/testify@v1.11.1
```

## Architecture Patterns

### Project Structure (Phase 1 scope)

```
serena/
  legacy/                    # All Python code moved here (MIG-01)
    src/
    test/
    scripts/
    pyproject.toml
    ...
  api/
    proto/
      serena/v1/
        ipc.proto            # Forwarder <-> daemon gRPC service
  cmd/
    serena/
      main.go                # Single binary entry point
  internal/
    cli/
      root.go                # Cobra root command with flags (--mode, --serve, --json, etc.)
    daemon/
      daemon.go              # Daemon lifecycle, subsystem orchestration
      socket.go              # Unix socket listener, stale file cleanup
      shutdown.go            # Two-phase graceful shutdown
      health.go              # Health check endpoint
    forwarder/
      forwarder.go           # Stdio-to-gRPC proxy, daemon auto-start
      dial.go                # Unix socket connection, daemon liveness check
    mcp/
      server.go              # MCP server wrapping official SDK
      session.go             # Session state (mode, profile, workspace ref)
      registry.go            # Tool registry with per-session filtering
      errors.go              # MCP error codes + structured detail
      middleware.go           # Receiving/sending middleware for logging, filtering
    config/
      config.go              # SerenaConfig struct (mirrors Python schema)
      project.go             # ProjectConfig struct
      loader.go              # koanf-based layered loading
      defaults.go            # Built-in default values
    workspace/
      workspace.go           # Workspace key, registry (stub for Phase 1)
      key.go                 # WorkspaceKey type
  go.mod
  go.sum
  Makefile                   # Proto generation, build targets
```

**Key difference from ARCHITECTURE.md:** Decision D-02 specifies a single binary (`serena`), not separate `cmd/serena/` and `cmd/serena-forwarder/`. The binary's mode is controlled by flags: `serena --mode=stdio` runs as forwarder, `serena --serve` runs as daemon directly, and default behavior auto-starts daemon then connects as forwarder.

### Pattern 1: Single Binary, Dual Mode

**What:** One `serena` binary operates as either forwarder or daemon based on flags.
**When to use:** Always -- this is the locked decision (D-01, D-02).
**Implementation:**

```go
// cmd/serena/main.go
func main() {
    rootCmd := &cobra.Command{
        Use:   "serena",
        Short: "Serena code intelligence MCP server",
        RunE:  run,
    }
    rootCmd.Flags().String("mode", "auto", "Transport mode: stdio, http, auto")
    rootCmd.Flags().Bool("serve", false, "Run as daemon directly (skip forwarder)")
    rootCmd.Flags().Bool("json", false, "Use JSON log format")
    rootCmd.Flags().String("socket", "", "Override daemon socket path")
    rootCmd.Flags().String("http-addr", ":8080", "HTTP listen address")
    rootCmd.Flags().String("config", "", "Path to config file")
    rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) error {
    serve, _ := cmd.Flags().GetBool("serve")
    if serve {
        return runDaemon(cmd)
    }
    mode, _ := cmd.Flags().GetString("mode")
    switch mode {
    case "stdio", "auto":
        return runForwarder(cmd)
    case "http":
        return runDaemon(cmd) // HTTP mode IS daemon mode
    }
}
```

### Pattern 2: gRPC Bidirectional Stream for IPC

**What:** Forwarder and daemon communicate via a single gRPC bidirectional streaming RPC over Unix socket.
**When to use:** All forwarder-to-daemon communication.
**Proto definition:**

```protobuf
// api/proto/serena/v1/ipc.proto
syntax = "proto3";
package serena.v1;

service ForwarderService {
  // Bidirectional stream of MCP JSON-RPC messages
  rpc StreamMCP(stream MCPMessage) returns (stream MCPMessage);
}

message MCPMessage {
  bytes payload = 1;  // Raw MCP JSON-RPC message
  string session_id = 2;  // Session identifier for routing
}
```

**Why bytes payload:** The forwarder is a thin proxy. It wraps raw JSON-RPC bytes in a gRPC message without parsing MCP semantics. The daemon unwraps and feeds into the MCP SDK via a custom Transport.

### Pattern 3: Custom MCP Transport over gRPC

**What:** Implement the MCP SDK's `Transport` interface backed by a gRPC stream, so the daemon can use the standard MCP SDK server with gRPC-delivered messages.
**Implementation approach:**

```go
// internal/mcp/grpc_transport.go
type GRPCTransport struct {
    stream pb.ForwarderService_StreamMCPServer
}

func (t *GRPCTransport) Connect(ctx context.Context) (mcp.Connection, error) {
    return &grpcConnection{stream: t.stream}, nil
}

// The Connection reads/writes JSON-RPC messages from/to the gRPC stream
type grpcConnection struct {
    stream pb.ForwarderService_StreamMCPServer
}

func (c *grpcConnection) Read(ctx context.Context) (json.RawMessage, error) {
    msg, err := c.stream.Recv()
    if err != nil {
        return nil, err
    }
    return msg.Payload, nil
}

func (c *grpcConnection) Write(ctx context.Context, msg json.RawMessage) error {
    return c.stream.Send(&pb.MCPMessage{Payload: msg})
}
```

### Pattern 4: Stale Socket Cleanup (DMN-13)

**What:** On startup, detect and clean up stale socket files from crashed daemons.
**Implementation:**

```go
func ensureSocket(path string) error {
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return nil // No socket file, good to go
    }
    // Socket exists -- try connecting
    conn, err := net.DialTimeout("unix", path, 2*time.Second)
    if err != nil {
        // Connection refused = stale socket
        slog.Info("removing stale socket", "path", path)
        return os.Remove(path)
    }
    conn.Close()
    return fmt.Errorf("daemon already running at %s", path)
}
```

### Pattern 5: Daemon Auto-Start from Forwarder (D-03)

**What:** Forwarder checks for running daemon, starts it if not found (gopls pattern).
**Implementation:**

```go
func connectOrStartDaemon(socketPath string) (*grpc.ClientConn, error) {
    // Try connecting to existing daemon
    conn, err := grpc.Dial("unix://"+socketPath, grpc.WithInsecure())
    if err == nil {
        // Verify health
        if healthy(conn) {
            return conn, nil
        }
        conn.Close()
    }
    // Start daemon as background process
    exe, _ := os.Executable()
    cmd := exec.Command(exe, "--serve", "--socket="+socketPath)
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // Detach from forwarder
    cmd.Start()
    cmd.Process.Release() // Don't wait for daemon
    // Poll for daemon readiness
    return waitForDaemon(socketPath, 10*time.Second)
}
```

### Pattern 6: Per-Session Tool Filtering via Middleware (MCP-07)

**What:** The MCP SDK does not natively support per-session tool filtering. Use receiving middleware.
**Implementation:**

```go
server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
    return func(ctx context.Context, req mcp.Request) (any, error) {
        if req.Method() == "tools/list" {
            result, err := next(ctx, req)
            if err != nil {
                return result, err
            }
            // Filter tools based on session state
            session := mcp.ServerSessionFromContext(ctx)
            return filterToolsForSession(session, result), nil
        }
        if req.Method() == "tools/call" {
            // Check if session is allowed to call this tool
            session := mcp.ServerSessionFromContext(ctx)
            toolName := extractToolName(req)
            if !sessionCanCallTool(session, toolName) {
                return nil, mcp.NewError(mcp.MethodNotFound, "tool not available", nil)
            }
        }
        return next(ctx, req)
    }
})
```

### Anti-Patterns to Avoid

- **Parsing MCP in forwarder:** Forwarder must be a dumb pipe. Never inspect MCP message content in forwarder -- let daemon handle all protocol logic.
- **Global mutable state for session tracking:** Use the MCP SDK's `Server.Sessions()` iterator and session-scoped state, not package-level maps.
- **Blocking daemon startup for LS initialization:** Phase 1 has no LS workers, but design the workspace activation path as async from day one. Return "initializing" status immediately.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| MCP JSON-RPC framing | Custom JSON-RPC parser | MCP Go SDK | SDK handles spec compliance, capability negotiation, transport multiplexing |
| Tool schema generation | Manual JSON Schema construction | MCP SDK struct tags + `AddTool[T]()` | Struct tags auto-generate correct JSON Schema from Go types |
| gRPC code generation | Hand-written proto stubs | `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc` | Generated code is type-safe and handles streaming correctly |
| Config file loading | Custom YAML + merge logic | koanf v2 with ordered providers | koanf handles merge precedence, type coercion, env var expansion |
| Signal handling | Custom signal loop | `signal.NotifyContext()` | Stdlib integrates signals with context cancellation |
| Log rotation | Custom file rotation | `lumberjack` or `gopkg.in/natefinnih/lumberjack.v2` | Well-tested log rotation; daemon logs at ~/.serena/logs/ need rotation |

**Key insight:** Phase 1 should use libraries for everything except the daemon lifecycle and gRPC-to-MCP bridge, which are the novel components specific to Serena's architecture.

## Common Pitfalls

### Pitfall 1: Forwarder Hangs When Daemon Crashes During gRPC Stream

**What goes wrong:** Forwarder has an active gRPC stream to daemon. Daemon crashes (SIGKILL). gRPC stream enters error state but forwarder goroutine blocks on `Recv()` until TCP keepalive timeout (often 2+ minutes).
**Why it happens:** gRPC over Unix sockets may not detect peer death immediately without keepalive configuration.
**How to avoid:** Configure gRPC keepalive on client side: `grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 10*time.Second, Timeout: 5*time.Second, PermitWithoutStream: true})`. Also set a request-level timeout on every `Recv()` call.
**Warning signs:** Forwarder appears hung after daemon crash. MCP client sees no response.

### Pitfall 2: Stale Socket File on Crash (from PITFALLS.md #4)

**What goes wrong:** Daemon crashes and leaves socket file. Next startup fails with "address already in use."
**How to avoid:** Startup probe (connect test, PID file check, unlink if stale). See Pattern 4 above.
**Warning signs:** "Works after manual socket delete" reports.

### Pitfall 3: Signal Handling Missing for Daemon Started as Background Process

**What goes wrong:** Forwarder starts daemon via `exec.Command().Start()` then `Process.Release()`. Daemon runs but never registers signal handlers because `main()` races between daemon startup and signal registration.
**How to avoid:** Register signal handlers FIRST in daemon's `main()`, before any goroutine or listener starts. Use `signal.NotifyContext` at the top of `run()`.
**Warning signs:** Daemon ignores SIGTERM, must be SIGKILL'd.

### Pitfall 4: MCP Session State vs Transport Confusion (from PITFALLS.md #5)

**What goes wrong:** Session state tied to transport connection. Client disconnect loses all state.
**How to avoid:** Separate three concerns from day one: transport connection, MCP session, workspace state. Design session as a lightweight view into workspace state. Workspace survives session death.
**Warning signs:** Slow reconnection, memory spikes on reconnect.

### Pitfall 5: Python CI Breaks After Migration

**What goes wrong:** Moving Python files to `legacy/` breaks import paths, test discovery, CI scripts.
**How to avoid:** Update pyproject.toml paths to reflect `legacy/` root. Update CI to `cd legacy/ && uv run poe test`. Verify Python tests pass AFTER migration before continuing Go work.
**Warning signs:** CI red after migration commit.

### Pitfall 6: Cobra Default Help Overriding Stdin Behavior

**What goes wrong:** Cobra prints help and exits when no flags provided, but default behavior should be stdio forwarder mode.
**How to avoid:** Set `rootCmd.RunE = run` (not `rootCmd.Run`) and handle the no-flags case explicitly. Set `rootCmd.SilenceUsage = true` to prevent help on errors.
**Warning signs:** `serena` with no args prints help instead of entering stdio mode.

## Code Examples

### MCP Server with Dummy Tool

```go
// internal/mcp/server.go
package mcp

import (
    "context"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type PingArgs struct {
    Message string `json:"message" jsonschema:"description=Message to echo back"`
}

func NewSerenaMCPServer() *mcp.Server {
    server := mcp.NewServer(
        &mcp.Implementation{Name: "serena", Version: "2.0.0-dev"},
        &mcp.ServerOptions{},
    )

    mcp.AddTool(server, &mcp.Tool{
        Name:        "ping",
        Description: "Echo a message back (diagnostic tool)",
    }, func(ctx context.Context, req *mcp.CallToolRequest, args PingArgs) (*mcp.CallToolResult, any, error) {
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: "pong: " + args.Message},
            },
        }, nil, nil
    })

    return server
}
```

### Daemon Lifecycle with Errgroup

```go
// internal/daemon/daemon.go
package daemon

import (
    "context"
    "log/slog"
    "net"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "golang.org/x/sync/errgroup"
)

type Daemon struct {
    mcpServer  *mcp.Server
    socketPath string
    httpAddr   string
    logger     *slog.Logger
}

func (d *Daemon) Run(ctx context.Context) error {
    ctx, cancel := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
    defer cancel()

    if err := ensureSocket(d.socketPath); err != nil {
        return err
    }
    os.MkdirAll(filepath.Dir(d.socketPath), 0700)

    g, gctx := errgroup.WithContext(ctx)

    // Unix socket listener for forwarder connections
    g.Go(func() error { return d.listenSocket(gctx) })

    // HTTP listener for Streamable HTTP MCP
    g.Go(func() error { return d.listenHTTP(gctx) })

    d.logger.Info("daemon started",
        "socket", d.socketPath,
        "http", d.httpAddr,
    )

    err := g.Wait()
    d.shutdown()
    return err
}

func (d *Daemon) shutdown() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    d.logger.Info("graceful shutdown starting")
    // Phase 1: drain sessions, close listeners
    // Phase 2: (future) stop LS workers
    _ = ctx // used in future phases
}
```

### Config Loading with koanf

```go
// internal/config/loader.go
package config

import (
    "github.com/knadh/koanf/parsers/yaml"
    "github.com/knadh/koanf/providers/confmap"
    "github.com/knadh/koanf/providers/file"
    "github.com/knadh/koanf/v2"
)

func Load(globalPath, projectPath string, cliOverrides map[string]interface{}) (*SerenaConfig, error) {
    k := koanf.New(".")

    // 1. Built-in defaults
    k.Load(confmap.Provider(defaultConfig(), "."), nil)

    // 2. Global config (~/.serena/serena_config.yml)
    if globalPath != "" {
        k.Load(file.Provider(globalPath), yaml.Parser())
    }

    // 3. Project config (.serena/project.yml)
    if projectPath != "" {
        k.Load(file.Provider(projectPath), yaml.Parser())
    }

    // 4. CLI flag overrides
    if len(cliOverrides) > 0 {
        k.Load(confmap.Provider(cliOverrides, "."), nil)
    }

    var cfg SerenaConfig
    if err := k.Unmarshal("", &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

### Structured MCP Errors (D-18)

```go
// internal/mcp/errors.go
package mcp

import "errors"

// Domain sentinel errors (D-19)
var (
    ErrSessionExpired    = errors.New("session expired")
    ErrWorkspaceNotReady = errors.New("workspace not ready")
    ErrToolNotAvailable  = errors.New("tool not available in current mode")
)

// Structured error detail for MCP responses (D-18)
type ErrorDetail struct {
    Code       string `json:"code"`
    Cause      string `json:"cause"`
    Suggestion string `json:"suggestion,omitempty"`
}

func NewMCPError(code int, detail ErrorDetail) error {
    // Wrap in MCP SDK error format
    data, _ := json.Marshal(detail)
    return &mcp.Error{
        Code:    code,
        Message: detail.Cause,
        Data:    json.RawMessage(data),
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| SSE transport | Streamable HTTP transport | MCP spec 2025-11-25 | SDK supports both; use Streamable HTTP for new code |
| mcp-go (community) | Official go-sdk | v1.0.0 (2025) | Official SDK is now stable (v1.4.1), use it |
| Viper for config | koanf v2 | 2024+ | koanf preserves key case, lighter deps |

**Deprecated/outdated:**
- SSE transport: Still supported in SDK but Streamable HTTP is the successor per MCP spec
- `Server.Run()` is convenience only for stdio; use `Server.Connect()` for custom transports

## Open Questions

1. **MCP SDK custom Transport interface exact signature**
   - What we know: SDK has Transport and Connection interfaces; IOTransport works with custom readers/writers
   - What's unclear: Exact method signatures for implementing a Transport backed by gRPC stream. May need to use IOTransport with pipe adapters instead of raw Transport implementation.
   - Recommendation: Spike a minimal gRPC-to-MCP bridge early in Phase 1 to validate the approach. Fallback: use IOTransport with `io.Pipe()` bridging gRPC stream to reader/writer.

2. **Per-session tool filtering middleware specifics**
   - What we know: SDK has `AddReceivingMiddleware` and `ServerSessionFromContext` (or similar) for accessing session in middleware
   - What's unclear: Exact API for extracting session from context in middleware, and whether tools/list response format allows filtering
   - Recommendation: Implement a proof-of-concept middleware in the first wave. The STATE.md already flags this as needing hands-on evaluation.

3. **Log rotation library**
   - What we know: Daemon needs file logging at ~/.serena/logs/ with rotation (D-17)
   - What's unclear: Whether to use lumberjack, a custom slog handler, or OS-level log rotation
   - Recommendation: Use `gopkg.in/natefinsh/lumberjack.v2` as slog writer. Well-tested, zero-config rotation by size.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go compiler | All Go code | Yes | 1.25.1 | -- |
| protoc | gRPC code generation | Yes | 33.4 (libprotoc) | -- |
| protoc-gen-go | Proto Go code gen | Yes | v1.36.11 | -- |
| protoc-gen-go-grpc | gRPC Go code gen | Yes | 1.6.1 | -- |
| git | Repo migration, version control | Yes | (system) | -- |

**Missing dependencies with no fallback:** None -- all required tools are available.

**Missing dependencies with fallback:**
- `buf` (proto management tool) not installed; protoc + manual Makefile targets work fine as alternative

## Project Constraints (from CLAUDE.md)

- **CLAUDE.md is Python-focused:** Commands like `uv run poe format/test/type-check` apply to legacy Python code only. Go code will use `go test`, `go vet`, `gofmt`.
- **GSD workflow enforcement:** All repo changes must go through GSD commands.
- **Binary name `serena`:** Same as current Python CLI entry point. After migration, `serena` becomes the Go binary.
- **Config directory `.serena/`:** Preserved (D-09). YAML format preserved (D-10).
- **Current Python tests must still pass after migration** (D-05): CI must be updated to run from `legacy/` directory.

## Sources

### Primary (HIGH confidence)
- [MCP Go SDK v1.4.1](https://github.com/modelcontextprotocol/go-sdk) -- Server API, AddTool, RemoveTools, StreamableHTTPHandler, middleware
- [MCP Go SDK pkg.go.dev](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp) -- Full API surface verified
- [gRPC-Go v1.80.0](https://github.com/grpc/grpc-go) -- Unix socket support, bidirectional streaming
- [koanf v2.3.4](https://github.com/knadh/koanf) -- Layered config loading
- [Go proxy](https://proxy.golang.org) -- All library versions verified against Go module proxy 2026-04-07
- Current Python Serena codebase -- config schema, context/mode YAML structure

### Secondary (MEDIUM confidence)
- [gRPC Unix domain socket examples](https://dev.to/gopher/grpc-over-unix-socket-protocol-471e) -- IPC pattern verified against gRPC docs
- [Cobra flat CLI pattern](https://github.com/spf13/cobra) -- Root command with RunE for flag-only CLI

### Tertiary (LOW confidence)
- MCP SDK custom Transport interface details -- needs spike to validate gRPC bridge approach

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all versions verified against Go module proxy, SDK API verified against pkg.go.dev
- Architecture: HIGH -- patterns from gopls (well-documented), SDK API confirmed
- gRPC IPC bridge: MEDIUM -- pattern is sound but exact MCP SDK Transport integration needs spike
- Pitfalls: HIGH -- drawn from verified PITFALLS.md research + gRPC-specific issues

**Research date:** 2026-04-07
**Valid until:** 2026-05-07 (30 days -- stable domain, libraries have infrequent breaking changes)
