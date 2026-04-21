# Phase 36: Client Hooks - Research

**Researched:** 2026-04-21
**Domain:** Claude Code hooks integration, CLI subcommands, gRPC IPC
**Confidence:** HIGH

## Summary

This phase adds three Claude Code lifecycle hooks (SessionStart, PreToolUse, Stop) that integrate Serena's workspace activation, nudge-toward-symbolic-tools behavior, and session cleanup. It also extends the existing `serena setup claude-code` command to install these hooks automatically.

The implementation builds on two well-established patterns already in the codebase: (1) the cobra CLI subcommand pattern from `status.go` for `activate`/`deactivate` commands, and (2) the `ClaudeCodeRegistrar` in `setup_clients.go` for hook installation. The primary new domain is the Claude Code hooks JSON format in `settings.json`, which is well-documented and uses direct file manipulation (no CLI exists for hook management).

**Primary recommendation:** Implement hooks as thin CLI commands (`serena activate`, `serena nudge`, `serena deactivate`) that the Claude Code hook system invokes as shell commands, and extend `ClaudeCodeRegistrar` to merge hook entries into `settings.json` alongside MCP registration.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Extend `ClaudeCodeRegistrar.Register()` in `internal/cli/setup_clients.go` to install hooks alongside MCP registration. Hooks are written into `.claude/settings.json` (project-scoped) or `~/.claude/settings.json` (global, with `--global` flag).
- **D-02:** Claude Code hooks are JSON entries in the `hooks` object of `settings.json`. Each hook type (SessionStart, PreToolUse, Stop) maps to an array of hook commands. Serena adds its entries without removing existing user hooks.
- **D-03:** `serena setup claude-code --uninstall` removes Serena hook entries from settings.json (preserving other hooks) alongside MCP deregistration.
- **D-04:** Hook commands invoke the `serena` binary directly (shell commands), not MCP tool calls. This keeps hooks independent of the MCP session lifecycle.
- **D-05:** SessionStart hook runs `serena activate --workspace <CWD>`. This ensures the daemon is running (auto-starts if needed via the existing forwarder auto-start mechanism) and activates the workspace for the current project directory.
- **D-06:** Add `activate` as a new cobra subcommand. It connects to the daemon via gRPC (same pattern as `serena status`), sends an ActivateWorkspace request, and exits. If the daemon isn't running, it starts it first.
- **D-07:** Activation is idempotent -- calling it when workspace is already active is a no-op that returns quickly.
- **D-08:** PreToolUse hook runs `serena nudge --tool <tool_name>` and returns a nudge message (or empty) to stderr. Claude Code injects non-empty hook output as system context.
- **D-09:** Nudge triggers when the tool being invoked is `Grep`, `Read`, or `Bash` (with grep/find patterns). The nudge script checks a local counter file (`.serena/session-stats.json`) tracking tool call counts per session.
- **D-10:** Nudge threshold: after 5+ grep/read calls without any Serena symbolic tool calls in the same session, return a nudge message like "Serena offers `find_symbol` and `get_symbols_overview` for code navigation -- try those instead of grep for symbol lookups."
- **D-11:** Nudge is advisory only -- it adds context but never blocks tool execution. The hook exit code is always 0.
- **D-12:** Counter resets when any Serena symbolic tool is called (indicating the agent took the nudge or is already using symbolic tools).
- **D-13:** Stop hook runs `serena deactivate --workspace <CWD>`. This cleans up session-scoped state (counter files, temporary workspace markers) but does NOT stop the daemon -- other sessions may be using it.
- **D-14:** Add `deactivate` as a new cobra subcommand. It connects to the daemon, sends a DeactivateWorkspace request (or simply cleans up local session files), and exits.
- **D-15:** If daemon is not running when Stop fires, deactivate silently succeeds (no error output) -- the daemon may have already been stopped by other means.
- **D-16:** `serena setup claude-code` installs hooks by default alongside MCP registration. No separate flag needed -- hooks are part of the standard Claude Code setup.
- **D-17:** Support `--no-hooks` flag to skip hook installation (for users who want MCP only without lifecycle hooks).
- **D-18:** `--dry-run` shows what hook entries would be written to settings.json.

### Claude's Discretion
- Exact JSON structure of hook entries in settings.json (follow Claude Code's documented hook format)
- Counter file format and location within `.serena/`
- Nudge message wording and variation
- Whether `activate`/`deactivate` subcommands are top-level or nested under a `session` group
- Error handling for edge cases (settings.json doesn't exist yet, malformed JSON, etc.)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| HOOK-01 | Claude Code SessionStart hook auto-activates project workspace on session start | D-05/D-06: `serena activate` CLI + gRPC ActivateWorkspace RPC, reuses forwarder auto-start pattern from `dial.go` |
| HOOK-02 | Claude Code PreToolUse hook nudges agents toward symbolic tools when overusing grep/read | D-08/D-09/D-10: `serena nudge` CLI with counter file in `.serena/session-stats.json`, threshold logic, advisory output |
| HOOK-03 | Claude Code Stop hook cleans up session data on session end | D-13/D-14/D-15: `serena deactivate` CLI, cleans counter files, silent on missing daemon |
| HOOK-04 | Hooks are auto-installed by `serena setup claude-code` into user settings | D-01/D-16/D-17: Extend `ClaudeCodeRegistrar`, merge into `settings.json` hooks object, `--no-hooks` flag |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Hook installation (HOOK-04) | CLI (`internal/cli/`) | -- | File manipulation of `.claude/settings.json`, extends existing `ClaudeCodeRegistrar` |
| Workspace activation (HOOK-01) | CLI + Daemon IPC | Kernel (`internal/kernel/`) | CLI sends gRPC request, daemon delegates to `Kernel.ActivateWorkspace()` |
| Nudge logic (HOOK-02) | CLI (`internal/cli/`) | -- | Stateless CLI reads counter file, outputs message; no daemon interaction needed |
| Session cleanup (HOOK-03) | CLI (`internal/cli/`) | Daemon (optional) | CLI removes local files; optionally notifies daemon to release workspace resources |
| Counter tracking (HOOK-02) | Local filesystem (`.serena/`) | -- | JSON file tracking tool call counts per session, no daemon state needed |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| cobra | v1.9.1 | CLI subcommands (activate, nudge, deactivate) | Already used for root, setup, status commands [VERIFIED: go.mod] |
| google.golang.org/grpc | (existing) | gRPC IPC for activate/deactivate | Already used by forwarder and status commands [VERIFIED: codebase] |
| encoding/json | stdlib | settings.json manipulation | Already used extensively in setup_clients.go [VERIFIED: codebase] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| fatih/color | (existing) | Colored terminal output for setup | Already used by SetupPrinter [VERIFIED: setup_output.go] |
| os/exec | stdlib | Not needed for hooks (no claude CLI for hooks) | Only for MCP registration (existing) |

**Installation:** No new dependencies. All required libraries are already in go.mod.

## Architecture Patterns

### System Architecture Diagram

```
Claude Code Session
    |
    |-- SessionStart event
    |       |
    |       v
    |   serena activate --workspace /path/to/project
    |       |
    |       +--> connectOrStartDaemon() [reuse from forwarder/dial.go]
    |       |       |
    |       |       v
    |       |   gRPC: ActivateWorkspace(path)
    |       |       |
    |       |       v
    |       |   Daemon -> Kernel.ActivateWorkspace(ctx, path)
    |       |
    |       +--> exit 0 (stdout: JSON with systemMessage or empty)
    |
    |-- PreToolUse event (matcher: Grep|Read|Bash)
    |       |
    |       v
    |   serena nudge --tool <tool_name>  (reads stdin for hook JSON)
    |       |
    |       +--> Read .serena/session-stats.json
    |       |       |
    |       |       v
    |       |   Increment counter for tool_name
    |       |   Check: grep_count >= 5 AND serena_tool_count == 0?
    |       |       |
    |       |       +-- YES --> stdout: nudge message (added to context)
    |       |       +-- NO  --> stdout: empty (no nudge)
    |       |
    |       +--> Write updated .serena/session-stats.json
    |       +--> exit 0 (always, advisory only)
    |
    |-- Stop event
    |       |
    |       v
    |   serena deactivate --workspace /path/to/project
    |       |
    |       +--> Remove .serena/session-stats.json
    |       +--> (Optional) gRPC: DeactivateWorkspace if daemon running
    |       +--> exit 0 (always, even if daemon not running)
    |
    v
Session ends
```

### Hook Installation Flow

```
serena setup claude-code
    |
    +--> ClaudeCodeRegistrar.Register(cfg)
    |       |
    |       +--> MCP registration via `claude mcp add-json` (existing)
    |       |
    |       +--> Hook installation (new):
    |               |
    |               +--> Resolve settings.json path (project or global)
    |               +--> Read existing settings.json (or create empty)
    |               +--> Merge Serena hooks into "hooks" object
    |               |       (preserve existing user hooks)
    |               +--> Write settings.json back
    |
    +--> Language detection + LS pre-install (existing)
```

### Recommended Project Structure

```
internal/cli/
    activate.go          # serena activate subcommand
    deactivate.go        # serena deactivate subcommand
    nudge.go             # serena nudge subcommand
    setup_clients.go     # Extended: hook installation in ClaudeCodeRegistrar
    setup_hooks.go       # Hook JSON generation, merge/remove helpers
    setup_hooks_test.go  # Tests for hook JSON manipulation
    activate_test.go     # Tests for activate command
    nudge_test.go        # Tests for nudge logic (counter, threshold)
    deactivate_test.go   # Tests for deactivate command
api/proto/serena/v1/
    ipc.proto            # Extended: ActivateWorkspace RPC
```

### Pattern 1: Claude Code Hook JSON Format

**What:** The exact JSON structure for hooks in `.claude/settings.json`
**When to use:** When generating or merging hook entries during `serena setup claude-code`

```json
// Source: https://code.claude.com/docs/en/hooks [VERIFIED: official docs]
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/serena activate --workspace \"$CLAUDE_PROJECT_DIR\"",
            "timeout": 30,
            "statusMessage": "Activating Serena workspace..."
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Grep|Read|Bash",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/serena nudge --tool \"$TOOL_NAME\"",
            "timeout": 5
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/serena deactivate --workspace \"$CLAUDE_PROJECT_DIR\"",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

**Key details:**
- `$CLAUDE_PROJECT_DIR` is provided by Claude Code as an environment variable [VERIFIED: official docs]
- Hook commands receive JSON on stdin with `tool_name`, `tool_input`, `session_id`, `cwd` fields [VERIFIED: official docs]
- stdout from hooks (exit 0) is added to Claude's context as a system message [VERIFIED: official docs]
- Exit code 0 = success (parse stdout); exit code 2 = blocking error; exit codes 1/3+ = non-blocking error [VERIFIED: official docs]
- Nudge hooks MUST exit 0 (advisory only per D-11)
- Hook output is capped at 10,000 characters [VERIFIED: official docs]
- SessionStart matcher `"startup"` fires on new session start (not resume/clear) [VERIFIED: official docs]

### Pattern 2: Hook Input JSON (stdin)

**What:** The JSON that Claude Code pipes to hook commands on stdin
**When to use:** In `serena nudge` to read tool name and decide whether to nudge

```go
// Source: https://code.claude.com/docs/en/hooks [VERIFIED: official docs]
// hookInput represents the JSON piped to hook commands on stdin.
type hookInput struct {
    SessionID      string         `json:"session_id"`
    TranscriptPath string         `json:"transcript_path"`
    CWD            string         `json:"cwd"`
    HookEventName  string         `json:"hook_event_name"`
    ToolName       string         `json:"tool_name"`       // PreToolUse/PostToolUse only
    ToolInput      map[string]any `json:"tool_input"`      // PreToolUse only
}
```

### Pattern 3: Merging Hooks into settings.json (Additive)

**What:** Merge Serena hooks into existing settings.json without overwriting user hooks
**When to use:** During `serena setup claude-code`

```go
// mergeHooksConfig reads settings.json, merges Serena hook entries into the
// "hooks" object under each event type, and writes back. Preserves existing
// hooks from other sources. Uses the same JSON marshal pattern as mergeJSONConfig.
func mergeHooksConfig(path string, serenaHooks map[string][]hookMatcher) error {
    existing := make(map[string]any)
    if data, err := os.ReadFile(path); err == nil {
        if err := json.Unmarshal(data, &existing); err != nil {
            return fmt.Errorf("parsing settings %s: %w", path, err)
        }
    }

    hooks, ok := existing["hooks"].(map[string]any)
    if !ok {
        hooks = make(map[string]any)
    }

    // For each event type, append Serena matchers to existing array
    for event, matchers := range serenaHooks {
        existingMatchers, _ := hooks[event].([]any)
        // Remove any existing Serena entries first (by matching command prefix)
        filtered := removeSerenaEntries(existingMatchers)
        // Append new Serena entries
        for _, m := range matchers {
            filtered = append(filtered, m)
        }
        hooks[event] = filtered
    }

    existing["hooks"] = hooks
    // ... write back with json.MarshalIndent
}
```

### Pattern 4: gRPC ActivateWorkspace RPC

**What:** New gRPC message for workspace activation from CLI
**When to use:** Extending `ipc.proto`

```protobuf
// Add to ipc.proto
rpc ActivateWorkspace(ActivateRequest) returns (ActivateResponse);

message ActivateRequest {
    string workspace_path = 1;
}

message ActivateResponse {
    bool already_active = 1;
    string status = 2;  // "activated" or "already_active"
}
```

### Anti-Patterns to Avoid

- **Writing hooks to stdout from PreToolUse as blocking (exit 2):** D-11 requires advisory-only. Always exit 0.
- **Using `claude` CLI for hook management:** No `claude hooks add` command exists. Must use direct file manipulation. [VERIFIED: claude --help]
- **Storing nudge counters in the daemon:** Counters are per-session, per-project local state. Storing in daemon adds complexity with no benefit. Use local `.serena/session-stats.json`.
- **Starting daemon from nudge command:** The nudge command should be fast and stateless. If daemon isn't running, nudge should just skip (no activation needed -- SessionStart hook handles that).
- **Hardcoding binary path in hook commands:** Use `cfg.BinaryPath` (resolved at setup time via `os.Executable() + filepath.EvalSymlinks()`).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON merge for settings.json | Custom string manipulation | `encoding/json` Unmarshal/MarshalIndent | Existing pattern in `mergeJSONConfig`; string concat breaks JSON [VERIFIED: setup_clients.go] |
| gRPC client connection | Custom socket dialing | Reuse `connectOrStartDaemon` from `forwarder/dial.go` | Already handles auto-start, keepalive, socket checking [VERIFIED: dial.go] |
| Hook JSON structure | Invent custom format | Follow Claude Code documented format exactly | Must match what Claude Code expects [VERIFIED: official docs] |
| Session ID generation | Custom ID scheme | Use `session_id` from hook stdin JSON | Claude Code provides unique session IDs per session [VERIFIED: official docs] |

## Common Pitfalls

### Pitfall 1: Hook Command Path Resolution
**What goes wrong:** Hook commands use a hardcoded path to `serena` binary that breaks after upgrades or on different machines.
**Why it happens:** The binary path at setup time may differ from runtime path.
**How to avoid:** Store the absolute path resolved at setup time (`resolveBinaryPath()`), same as MCP registration. Users re-run `serena setup claude-code` after moving the binary (which they'd need to do for MCP too).
**Warning signs:** "command not found" errors in hook output.

### Pitfall 2: Overwriting Existing User Hooks
**What goes wrong:** Writing the entire `hooks` object replaces user-defined hooks for the same event types.
**Why it happens:** Naive approach writes `hooks.PreToolUse = [serenaHooks]` instead of appending.
**How to avoid:** Read existing hooks array for each event type, filter out previous Serena entries (identify by command path containing "serena"), then append new entries. Same additive pattern as `mergeJSONConfig`.
**Warning signs:** User reports lost custom hooks after running `serena setup`.

### Pitfall 3: Nudge Counter File Locking
**What goes wrong:** Concurrent hook invocations (unlikely but possible) corrupt the counter file.
**Why it happens:** Claude Code fires PreToolUse hooks before each tool call; rapid sequential calls could overlap.
**How to avoid:** Use atomic file writes (write to temp file, rename). Counter corruption is non-fatal -- just reset counter.
**Warning signs:** Malformed JSON in session-stats.json.

### Pitfall 4: SessionStart Matcher Scope
**What goes wrong:** Hook fires on `resume`, `clear`, and `compact` events too, causing redundant activation.
**Why it happens:** Omitting the matcher or using `"*"` matches all session events.
**How to avoid:** Use `"matcher": "startup"` to fire only on new session creation. Activation is idempotent per D-07, so extra calls are harmless but waste time.
**Warning signs:** Slow session resume due to unnecessary workspace activation.

### Pitfall 5: Hook Timeout for Activation
**What goes wrong:** `serena activate` starts daemon + activates workspace which can take >10s on first run (LS installation).
**Why it happens:** Default hook timeout may be too short for cold-start scenario.
**How to avoid:** Set timeout to 30s for SessionStart hook. Use `statusMessage` field to show "Activating Serena workspace..." during wait.
**Warning signs:** Hook timeout errors on first session in a new project.

### Pitfall 6: Nudge Reads stdin But Also Needs Tool Name
**What goes wrong:** The `--tool` flag becomes redundant since tool_name is in stdin JSON.
**Why it happens:** D-08 specifies `--tool <tool_name>` flag, but Claude Code also passes this in stdin JSON.
**How to avoid:** Read tool_name from stdin JSON (it's more reliable and includes tool_input for Bash pattern matching). The `--tool` flag can serve as a fallback or be the primary source if stdin is not piped.
**Warning signs:** Mismatch between flag and stdin data.

### Pitfall 7: settings.json Doesn't Exist Yet
**What goes wrong:** First-time setup on a fresh project where `.claude/` directory doesn't exist.
**Why it happens:** Project hasn't been used with Claude Code before.
**How to avoid:** Create `.claude/` directory and `settings.json` with just the hooks object if it doesn't exist. Same `os.MkdirAll` pattern as `mergeJSONConfig`.
**Warning signs:** File not found errors during setup.

## Code Examples

### Example 1: Activate Subcommand

```go
// Source: follows pattern from internal/cli/status.go [VERIFIED: codebase]
func newActivateCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "activate",
        Short: "Activate workspace for current project",
        Long:  "Ensures the Serena daemon is running and activates the workspace for the given directory.",
        RunE:  runActivate,
        SilenceUsage:  true,
        SilenceErrors: true,
    }
    cmd.Flags().String("workspace", "", "Workspace directory (default: current directory)")
    return cmd
}

func runActivate(cmd *cobra.Command, _ []string) error {
    wsPath, _ := cmd.Flags().GetString("workspace")
    if wsPath == "" {
        var err error
        wsPath, err = os.Getwd()
        if err != nil {
            return fmt.Errorf("getting working directory: %w", err)
        }
    }

    socketPath := config.DefaultSocketPath()

    // Reuse connectOrStartDaemon from forwarder -- ensures daemon is running.
    // This is the key reuse point: same auto-start logic as stdio forwarder.
    client, conn, err := forwarder.ConnectOrStartDaemon(ctx, socketPath, logger)
    if err != nil {
        return fmt.Errorf("connecting to daemon: %w", err)
    }
    defer conn.Close()

    ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
    defer cancel()

    resp, err := client.ActivateWorkspace(ctx, &serenav1.ActivateRequest{
        WorkspacePath: wsPath,
    })
    if err != nil {
        return fmt.Errorf("activating workspace: %w", err)
    }

    // Output for Claude Code hook stdout (added to context)
    fmt.Fprintf(os.Stdout, "Serena workspace activated: %s\n", wsPath)
    return nil
}
```

### Example 2: Nudge Subcommand

```go
// Source: design from CONTEXT.md decisions [VERIFIED: CONTEXT.md]
func runNudge(cmd *cobra.Command, _ []string) error {
    // Read hook input from stdin
    var input hookInput
    if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
        // If stdin isn't piped (manual invocation), silently exit
        return nil
    }

    toolName := input.ToolName
    sessionID := input.SessionID
    wsDir := input.CWD

    // Load or create session stats
    statsPath := filepath.Join(wsDir, ".serena", "session-stats.json")
    stats := loadSessionStats(statsPath, sessionID)

    // Check if this is a Serena symbolic tool call (reset counter)
    if isSerenaSymbolicTool(toolName) {
        stats.SerenaToolCount++
        stats.GrepReadCount = 0 // Reset -- agent is using symbolic tools
        saveSessionStats(statsPath, stats)
        return nil
    }

    // Increment grep/read counter
    if isGrepReadTool(toolName, input.ToolInput) {
        stats.GrepReadCount++
        saveSessionStats(statsPath, stats)

        // Check threshold
        if stats.GrepReadCount >= 5 && stats.SerenaToolCount == 0 {
            // Output nudge to stdout (added to Claude's context)
            fmt.Println("Serena offers `find_symbol` and `get_symbols_overview` for code navigation -- try those instead of grep for symbol lookups.")
        }
    }

    return nil // Always exit 0
}

func isGrepReadTool(name string, input map[string]any) bool {
    switch name {
    case "Grep", "Read":
        return true
    case "Bash":
        cmd, _ := input["command"].(string)
        return strings.Contains(cmd, "grep") || strings.Contains(cmd, "find")
    }
    return false
}
```

### Example 3: Hook Installation in ClaudeCodeRegistrar

```go
// Source: extends existing pattern in setup_clients.go [VERIFIED: codebase]
func (r *ClaudeCodeRegistrar) Register(cfg RegistrationConfig) error {
    // Existing MCP registration via claude CLI...
    if err := r.registerMCP(cfg); err != nil {
        return err
    }

    // New: Hook installation (unless --no-hooks)
    if !cfg.NoHooks {
        if err := r.installHooks(cfg); err != nil {
            // Non-fatal: hooks are supplementary
            cfg.Printer.Failure("hook installation failed: %s", err)
            cfg.Printer.Info("MCP registration succeeded; hooks can be installed manually")
            return nil
        }
        cfg.Printer.Success("hooks installed (SessionStart, PreToolUse, Stop)")
    }

    return nil
}
```

### Example 4: Session Stats File Format

```json
{
  "session_id": "abc123...",
  "grep_read_count": 3,
  "serena_tool_count": 0,
  "last_updated": "2026-04-21T10:30:00Z"
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No hook management CLI in Claude Code | Direct `settings.json` file manipulation | Current (2026) | Must use JSON merge, not CLI subprocess [VERIFIED: claude --help] |
| Hooks only had `command` type | 4 handler types: command, http, prompt, agent | March 2026 | We use `command` type (simplest, most portable) [VERIFIED: official docs] |
| Limited hook events | 25+ hook event types | 2026 | We use SessionStart, PreToolUse, Stop [VERIFIED: official docs] |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `connectOrStartDaemon` can be exported (currently unexported in `forwarder` package) or its logic extracted | Architecture Patterns | Would need to duplicate auto-start logic or refactor to shared package |
| A2 | `$CLAUDE_PROJECT_DIR` env var is available in all hook command executions | Pattern 1 | Would need to use `cwd` from stdin JSON instead |
| A3 | Claude Code hook stdin JSON includes `tool_name` for PreToolUse events | Pattern 2 | Would need to rely solely on `--tool` flag |
| A4 | Settings.json hook arrays are merged across scopes (not overwritten) | Pattern 3 | Project-scoped hooks could shadow user-scoped ones |

## Open Questions

1. **Export `connectOrStartDaemon` or extract shared helper?**
   - What we know: The function is unexported in `forwarder` package, used only by `RunForwarder`
   - What's unclear: Whether to export it, create a shared `daemonctl` package, or duplicate the connect+start logic
   - Recommendation: Export as `ConnectOrStartDaemon` in the forwarder package (simplest change); the activate command already depends on the same daemon socket pattern

2. **Should nudge command read tool_name from stdin or from --tool flag?**
   - What we know: Claude Code pipes JSON to stdin with tool_name; D-08 specifies `--tool` flag
   - What's unclear: Whether stdin is always available for command-type hooks
   - Recommendation: Read from stdin JSON (authoritative source), use `--tool` flag as fallback for manual testing

3. **ActivateWorkspace RPC: new RPC or reuse existing activate_project MCP tool?**
   - What we know: `activate_project` is an MCP tool that calls `Kernel.ActivateWorkspace()`. The CLI activate command needs the same effect but via gRPC.
   - What's unclear: Whether to add a new gRPC RPC or reuse the existing MCP tool path
   - Recommendation: Add a new `ActivateWorkspace` RPC to the ForwarderService in ipc.proto. This is cleaner than routing through MCP and follows the pattern of `GetStatus` RPC.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.9.0 |
| Config file | Standard `go test` (no config file) |
| Quick run command | `go test ./internal/cli/... -run TestHook -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| HOOK-01 | activate command starts daemon and activates workspace | unit | `go test ./internal/cli/... -run TestActivate -count=1` | Wave 0 |
| HOOK-02 | nudge command reads counter, returns message at threshold | unit | `go test ./internal/cli/... -run TestNudge -count=1` | Wave 0 |
| HOOK-03 | deactivate command cleans up session files | unit | `go test ./internal/cli/... -run TestDeactivate -count=1` | Wave 0 |
| HOOK-04 | setup installs hooks into settings.json, preserves existing | unit | `go test ./internal/cli/... -run TestHookInstall -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/cli/... -count=1 && go vet ./...`
- **Per wave merge:** `go test ./... -count=1`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `internal/cli/activate_test.go` -- covers HOOK-01 (activate command, idempotency)
- [ ] `internal/cli/nudge_test.go` -- covers HOOK-02 (counter logic, threshold, Bash pattern matching)
- [ ] `internal/cli/deactivate_test.go` -- covers HOOK-03 (cleanup, missing daemon tolerance)
- [ ] `internal/cli/setup_hooks_test.go` -- covers HOOK-04 (JSON merge, preserve existing hooks, uninstall)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | N/A -- hooks run locally as current user |
| V3 Session Management | No | N/A -- no network sessions |
| V4 Access Control | No | N/A -- local file operations |
| V5 Input Validation | Yes | Validate hook stdin JSON with json.Decoder; validate workspace paths |
| V6 Cryptography | No | N/A |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal in --workspace flag | Tampering | Resolve to absolute path, verify it's a directory |
| Malicious stdin JSON to nudge | Tampering | Use json.Decoder (safe), don't exec any values from stdin |
| settings.json injection | Tampering | Use encoding/json Marshal (never string concat), same as existing mergeJSONConfig |

## Sources

### Primary (HIGH confidence)
- Claude Code official hooks docs: https://code.claude.com/docs/en/hooks -- hook JSON format, event types, stdin/stdout behavior, exit codes, matchers
- Codebase: `internal/cli/setup_clients.go` -- ClaudeCodeRegistrar, mergeJSONConfig, removeFromJSONConfig patterns
- Codebase: `internal/cli/status.go` -- cobra subcommand + gRPC client pattern
- Codebase: `internal/forwarder/dial.go` -- connectOrStartDaemon auto-start pattern
- Codebase: `internal/kernel/kernel.go` -- ActivateWorkspace method (idempotent, already exists)
- Codebase: `api/proto/serena/v1/ipc.proto` -- existing gRPC service definition

### Secondary (MEDIUM confidence)
- Blog analysis of settings.json structure: https://blog.vincentqiao.com/en/posts/claude-code-settings-hooks/ -- confirmed hooks nest under "hooks" key
- Claude Code CLI `--help` output -- confirmed no `hooks` subcommand exists for programmatic hook management

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in codebase, no new dependencies
- Architecture: HIGH -- extends well-established patterns (cobra commands, gRPC IPC, JSON config merge)
- Pitfalls: HIGH -- verified hook format against official docs, codebase patterns well understood
- Hook format: HIGH -- verified against official Claude Code documentation

**Research date:** 2026-04-21
**Valid until:** 2026-05-21 (Claude Code hook format stable since March 2026)
