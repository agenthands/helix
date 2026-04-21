# Phase 36: Client Hooks - Pattern Map

**Mapped:** 2026-04-21
**Files analyzed:** 10 new/modified files
**Analogs found:** 10 / 10

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/cli/activate.go` | controller (CLI) | request-response | `internal/cli/status.go` | exact |
| `internal/cli/deactivate.go` | controller (CLI) | request-response | `internal/cli/status.go` | exact |
| `internal/cli/nudge.go` | controller (CLI) | file-I/O | `internal/cli/status.go` | role-match |
| `internal/cli/setup_hooks.go` | utility | file-I/O | `internal/cli/setup_clients.go` | exact |
| `internal/cli/setup_clients.go` (modify) | utility | file-I/O | self | exact |
| `internal/cli/root.go` (modify) | config | -- | self | exact |
| `api/proto/serena/v1/ipc.proto` (modify) | config | request-response | self | exact |
| `internal/cli/activate_test.go` | test | -- | `internal/cli/status_test.go` | exact |
| `internal/cli/nudge_test.go` | test | -- | `internal/cli/setup_test.go` | exact |
| `internal/cli/setup_hooks_test.go` | test | -- | `internal/cli/setup_test.go` | exact |
| `internal/cli/deactivate_test.go` | test | -- | `internal/cli/status_test.go` | role-match |

## Pattern Assignments

### `internal/cli/activate.go` (controller, request-response)

**Analog:** `internal/cli/status.go`

**Imports pattern** (lines 1-18):
```go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	serenav1 "github.com/postfix/serena/api/proto/serena/v1"
	"github.com/postfix/serena/internal/config"
)
```

**Command constructor pattern** (status.go lines 22-36):
```go
func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show workspace health and language server status",
		Long: `Query the running Serena daemon and display language server health.
Defaults to showing only unhealthy servers. Use --verbose for full output.`,
		RunE:          runStatus,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Flags().Bool("json", false, "Output as JSON (same format as get_health tool)")
	cmd.Flags().BoolP("verbose", "v", false, "Show all language servers including healthy ones")

	return cmd
}
```

**gRPC connection pattern** (status.go lines 39-77):
```go
func runStatus(cmd *cobra.Command, _ []string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	verbose, _ := cmd.Flags().GetBool("verbose")

	socketPath := config.DefaultSocketPath()

	// Check if daemon is running per D-07.
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Daemon not running")
		os.Exit(1)
	}

	// Verify socket is alive with a short dial timeout per T-35-06.
	testConn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Daemon not running")
		os.Exit(1)
	}
	testConn.Close()

	// Connect via gRPC.
	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connecting to daemon: %w", err)
	}
	defer conn.Close()

	client := serenav1.NewForwarderServiceClient(conn)

	// Query health with 5-second timeout per T-35-06.
	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer cancel()

	resp, err := client.GetStatus(ctx, &serenav1.StatusRequest{Verbose: verbose})
	if err != nil {
		return fmt.Errorf("querying daemon status: %w", err)
	}
	// ...
}
```

**Key difference for activate:** Unlike `status`, `activate` should use the forwarder's `connectOrStartDaemon` pattern (dial.go lines 25-41) to auto-start the daemon if not running, rather than exiting with error. The `connectOrStartDaemon` function is currently unexported -- needs to be exported or its logic extracted. See `internal/forwarder/dial.go` lines 25-41 and 84-101 for daemon start + poll pattern.

---

### `internal/cli/deactivate.go` (controller, request-response)

**Analog:** `internal/cli/status.go` (same as activate)

Same command constructor and gRPC patterns as activate. Key difference: deactivate should silently succeed if daemon is not running (D-15), so instead of `os.Exit(1)` on missing socket, return nil.

**Silent-fail pattern** (contrast with status.go lines 46-49):
```go
// status.go exits on missing daemon:
if _, err := os.Stat(socketPath); os.IsNotExist(err) {
	fmt.Fprintln(os.Stderr, "Daemon not running")
	os.Exit(1)
}

// deactivate should instead:
if _, err := os.Stat(socketPath); os.IsNotExist(err) {
	// Daemon not running, nothing to deactivate -- silent success per D-15
	return nil
}
```

---

### `internal/cli/nudge.go` (controller, file-I/O)

**Analog:** `internal/cli/status.go` (command constructor pattern only)

**Command constructor** -- same pattern as status.go lines 22-36 but with `--tool` flag instead of `--json`/`--verbose`.

**No gRPC needed** -- nudge is stateless, reads/writes a local JSON file. Uses `encoding/json` for stdin parsing and counter file, following the same JSON marshal/unmarshal discipline as `setup_clients.go`.

**JSON file read/write pattern** from `internal/cli/setup_clients.go` (lines 60-91):
```go
// Read existing JSON file or start with empty map
existing := make(map[string]any)
if data, err := os.ReadFile(path); err == nil {
	if err := json.Unmarshal(data, &existing); err != nil {
		return fmt.Errorf("parsing existing config %s: %w", path, err)
	}
}
// ... modify ...
data, err := json.MarshalIndent(existing, "", "  ")
// ...
if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
	return fmt.Errorf("creating config directory: %w", err)
}
if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
	return fmt.Errorf("writing config %s: %w", path, err)
}
```

---

### `internal/cli/setup_hooks.go` (utility, file-I/O)

**Analog:** `internal/cli/setup_clients.go`

**Core JSON merge pattern** (setup_clients.go lines 60-91):
```go
func mergeJSONConfig(path, key, serverName string, serverConfig map[string]any) error {
	existing := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parsing existing config %s: %w", path, err)
		}
	}

	servers, ok := existing[key].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}
	servers[serverName] = serverConfig
	existing[key] = servers

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}
```

**Core JSON remove pattern** (setup_clients.go lines 95-123):
```go
func removeFromJSONConfig(path, key, serverName string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading config %s: %w", path, err)
	}

	existing := make(map[string]any)
	if err := json.Unmarshal(data, &existing); err != nil {
		return fmt.Errorf("parsing config %s: %w", path, err)
	}

	servers, ok := existing[key].(map[string]any)
	if !ok {
		return nil
	}

	delete(servers, serverName)
	existing[key] = servers

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return os.WriteFile(path, append(out, '\n'), 0644)
}
```

**Key adaptation:** Hook merge operates on arrays within the `hooks` map (not nested maps like `mcpServers`). Each event type (`SessionStart`, `PreToolUse`, `Stop`) maps to an array of matcher objects. Serena entries must be identified by command path containing "serena" for idempotent add/remove.

---

### `internal/cli/setup_clients.go` (modify -- extend ClaudeCodeRegistrar)

**Self-analog.** Extend `Register()` (lines 142-171) and `Unregister()` (lines 173-197) to call hook install/remove helpers from `setup_hooks.go`.

**Registration extension point** (lines 142-171): After existing MCP registration via `claude mcp add-json`, add hook installation call. Follow the same error pattern -- cfg.Printer for output.

**RegistrationConfig extension point** (lines 26-33): Add `NoHooks bool` field.

---

### `internal/cli/root.go` (modify -- add subcommands)

**Self-analog.** Add new subcommands at lines 50-51:
```go
// Subcommands
rootCmd.AddCommand(newSetupCommand())
rootCmd.AddCommand(newStatusCommand())
// Add:
rootCmd.AddCommand(newActivateCommand())
rootCmd.AddCommand(newDeactivateCommand())
rootCmd.AddCommand(newNudgeCommand())
```

---

### `api/proto/serena/v1/ipc.proto` (modify -- add RPCs)

**Self-analog.** Follow existing `GetStatus` pattern (lines 15-16, 27-34):
```protobuf
// GetStatus returns workspace health for the CLI status command.
rpc GetStatus(StatusRequest) returns (StatusResponse);

// StatusRequest queries daemon health.
message StatusRequest {
  bool verbose = 1;
}

// StatusResponse carries JSON-encoded health report.
message StatusResponse {
  bytes payload = 1;
}
```

Add `ActivateWorkspace` and `DeactivateWorkspace` RPCs following same request/response message pattern.

---

### `internal/cli/activate_test.go` (test)

**Analog:** `internal/cli/status_test.go`

**Command flag test pattern** (status_test.go lines 180-194):
```go
func TestNewStatusCommand_Flags(t *testing.T) {
	cmd := newStatusCommand()

	assert.Equal(t, "status", cmd.Use)
	assert.NotNil(t, cmd.RunE)

	jsonFlag := cmd.Flags().Lookup("json")
	assert.NotNil(t, jsonFlag, "json flag should exist")
	assert.Equal(t, "false", jsonFlag.DefValue)
}
```

---

### `internal/cli/nudge_test.go` and `internal/cli/setup_hooks_test.go` (test)

**Analog:** `internal/cli/setup_test.go`

**JSON file manipulation test pattern** (setup_test.go lines 19-41):
```go
func TestMergeJSONConfigNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	err := mergeJSONConfig(path, "mcpServers", "serena", map[string]any{
		"command": "/usr/local/bin/serena",
		"args":    []string{"--mode=stdio"},
	})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers, ok := result["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers key should exist")
}
```

**Preserve-existing test pattern** (setup_test.go lines 43-71):
```go
func TestMergeJSONConfigExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write initial config with another server.
	initial := map[string]any{
		"mcpServers": map[string]any{
			"other-server": map[string]any{"command": "other"},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	require.NoError(t, os.WriteFile(path, data, 0644))

	// Merge serena entry.
	err := mergeJSONConfig(path, "mcpServers", "serena", map[string]any{
		"command": "/usr/local/bin/serena",
	})
	require.NoError(t, err)

	data, err = os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers := result["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "other-server", "existing entries must be preserved")
	assert.Contains(t, servers, "serena", "new entry must be added")
}
```

**DryRun registrar test pattern** (setup_test.go lines 156-169):
```go
func TestClaudeCodeRegistrarDryRun(t *testing.T) {
	r := &ClaudeCodeRegistrar{}
	assert.Equal(t, "claude-code", r.Name())

	printer := &SetupPrinter{DryRun: true}
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/serena",
		DryRun:     true,
		ProjectDir: t.TempDir(),
		Printer:    printer,
	}
	err := r.Register(cfg)
	assert.NoError(t, err)
}
```

---

## Shared Patterns

### Cobra Subcommand Registration
**Source:** `internal/cli/root.go` lines 50-51
**Apply to:** activate.go, deactivate.go, nudge.go
```go
rootCmd.AddCommand(newSetupCommand())
rootCmd.AddCommand(newStatusCommand())
```
All new commands follow the `newXxxCommand() *cobra.Command` factory pattern with `SilenceUsage: true, SilenceErrors: true`.

### gRPC Client Connection
**Source:** `internal/cli/status.go` lines 43-67
**Apply to:** activate.go, deactivate.go
```go
socketPath := config.DefaultSocketPath()
conn, err := grpc.NewClient("unix://"+socketPath,
	grpc.WithTransportCredentials(insecure.NewCredentials()),
)
if err != nil {
	return fmt.Errorf("connecting to daemon: %w", err)
}
defer conn.Close()
client := serenav1.NewForwarderServiceClient(conn)
ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
defer cancel()
```

### Daemon Auto-Start (for activate only)
**Source:** `internal/forwarder/dial.go` lines 25-41, 84-101
**Apply to:** activate.go
```go
func connectOrStartDaemon(ctx context.Context, socketPath string, logger *slog.Logger, tp trace.TracerProvider) (serenav1.ForwarderServiceClient, *grpc.ClientConn, error) {
	conn, client, err := tryConnect(ctx, socketPath, tp)
	if err == nil {
		return client, conn, nil
	}
	if err := startDaemon(socketPath); err != nil {
		return nil, nil, fmt.Errorf("starting daemon: %w", err)
	}
	return waitForDaemon(ctx, socketPath, 10*time.Second, tp)
}
```
This function is currently unexported. Either export it or extract the connect+start logic into a shared helper.

### JSON Config File I/O
**Source:** `internal/cli/setup_clients.go` lines 60-91
**Apply to:** setup_hooks.go, nudge.go (for session-stats.json)
- Always use `encoding/json` Marshal/Unmarshal, never string concatenation
- `os.MkdirAll(filepath.Dir(path), 0755)` for directory creation
- `os.WriteFile(path, append(data, '\n'), 0644)` for file writes
- Graceful handling of missing files (start with empty map)

### Error Formatting
**Source:** `internal/cli/setup_clients.go`, `internal/cli/status.go`
**Apply to:** All new files
```go
return fmt.Errorf("connecting to daemon: %w", err)
return fmt.Errorf("parsing existing config %s: %w", path, err)
```
Consistent `fmt.Errorf` with `%w` wrapping, action-oriented prefix.

### Test Conventions
**Source:** `internal/cli/setup_test.go`, `internal/cli/status_test.go`
**Apply to:** All test files
- Package: `package cli` (same package, not `_test`)
- Imports: `testify/assert` and `testify/require`
- `t.TempDir()` for temp directories
- `require.NoError(t, err)` for fatal checks, `assert.*` for non-fatal
- Test names: `TestFunctionName_Scenario` pattern

### Setup Printer Output
**Source:** `internal/cli/setup_output.go` lines 18-47
**Apply to:** setup_hooks.go (hook installation output)
```go
cfg.Printer.Success("hooks installed (SessionStart, PreToolUse, Stop)")
cfg.Printer.Failure("hook installation failed: %s", err)
cfg.Printer.DryRunAction("would write hooks to %s", settingsPath)
cfg.Printer.Info("hooks can be installed manually")
```

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| (none) | -- | -- | All files have close analogs in the existing codebase |

## Metadata

**Analog search scope:** `internal/cli/`, `internal/forwarder/`, `api/proto/serena/v1/`
**Files scanned:** 12 existing files
**Pattern extraction date:** 2026-04-21
