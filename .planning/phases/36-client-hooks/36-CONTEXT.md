# Phase 36: Client Hooks - Context

**Gathered:** 2026-04-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Claude Code sessions automatically activate Serena and guide agents toward symbolic tools. Covers HOOK-01 through HOOK-04: SessionStart hook auto-activates workspace, PreToolUse hook nudges agents away from grep/read overuse toward symbolic tools, Stop hook cleans up session data, and hooks are auto-installed by `serena setup claude-code`.

</domain>

<decisions>
## Implementation Decisions

### Hook Installation Mechanism
- **D-01:** Extend `ClaudeCodeRegistrar.Register()` in `internal/cli/setup_clients.go` to install hooks alongside MCP registration. Hooks are written into `.claude/settings.json` (project-scoped) or `~/.claude/settings.json` (global, with `--global` flag).
- **D-02:** Claude Code hooks are JSON entries in the `hooks` object of `settings.json`. Each hook type (SessionStart, PreToolUse, Stop) maps to an array of hook commands. Serena adds its entries without removing existing user hooks.
- **D-03:** `serena setup claude-code --uninstall` removes Serena hook entries from settings.json (preserving other hooks) alongside MCP deregistration.
- **D-04:** Hook commands invoke the `serena` binary directly (shell commands), not MCP tool calls. This keeps hooks independent of the MCP session lifecycle.

### SessionStart Hook (HOOK-01)
- **D-05:** SessionStart hook runs `serena activate --workspace <CWD>`. This ensures the daemon is running (auto-starts if needed via the existing forwarder auto-start mechanism) and activates the workspace for the current project directory.
- **D-06:** Add `activate` as a new cobra subcommand. It connects to the daemon via gRPC (same pattern as `serena status`), sends an ActivateWorkspace request, and exits. If the daemon isn't running, it starts it first.
- **D-07:** Activation is idempotent — calling it when workspace is already active is a no-op that returns quickly.

### PreToolUse Nudge (HOOK-02)
- **D-08:** PreToolUse hook runs `serena nudge --tool <tool_name>` and returns a nudge message (or empty) to stderr. Claude Code injects non-empty hook output as system context.
- **D-09:** Nudge triggers when the tool being invoked is `Grep`, `Read`, or `Bash` (with grep/find patterns). The nudge script checks a local counter file (`.serena/session-stats.json`) tracking tool call counts per session.
- **D-10:** Nudge threshold: after 5+ grep/read calls without any Serena symbolic tool calls in the same session, return a nudge message like "Serena offers `find_symbol` and `get_symbols_overview` for code navigation — try those instead of grep for symbol lookups."
- **D-11:** Nudge is advisory only — it adds context but never blocks tool execution. The hook exit code is always 0.
- **D-12:** Counter resets when any Serena symbolic tool is called (indicating the agent took the nudge or is already using symbolic tools).

### Stop Hook (HOOK-03)
- **D-13:** Stop hook runs `serena deactivate --workspace <CWD>`. This cleans up session-scoped state (counter files, temporary workspace markers) but does NOT stop the daemon — other sessions may be using it.
- **D-14:** Add `deactivate` as a new cobra subcommand. It connects to the daemon, sends a DeactivateWorkspace request (or simply cleans up local session files), and exits.
- **D-15:** If daemon is not running when Stop fires, deactivate silently succeeds (no error output) — the daemon may have already been stopped by other means.

### Hook Auto-Installation (HOOK-04)
- **D-16:** `serena setup claude-code` installs hooks by default alongside MCP registration. No separate flag needed — hooks are part of the standard Claude Code setup.
- **D-17:** Support `--no-hooks` flag to skip hook installation (for users who want MCP only without lifecycle hooks).
- **D-18:** `--dry-run` shows what hook entries would be written to settings.json.

### Claude's Discretion
- Exact JSON structure of hook entries in settings.json (follow Claude Code's documented hook format)
- Counter file format and location within `.serena/`
- Nudge message wording and variation
- Whether `activate`/`deactivate` subcommands are top-level or nested under a `session` group
- Error handling for edge cases (settings.json doesn't exist yet, malformed JSON, etc.)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 34 Foundation (predecessor — hook installation point)
- `internal/cli/setup_clients.go` — `ClaudeCodeRegistrar` that will be extended to install hooks
- `internal/cli/setup.go` — Setup command structure and `RegistrationConfig`
- `internal/cli/setup_output.go` — `SetupPrinter` for consistent output formatting

### Phase 35 Foundation (predecessor — CLI pattern)
- `internal/cli/status.go` — `serena status` cobra subcommand pattern (gRPC client connection to daemon)
- `internal/cli/root.go` — Cobra root command where new subcommands register

### Daemon & Workspace
- `internal/daemon/daemon.go` — Daemon bootstrap, workspace management, tool registration
- `internal/forwarder/forwarder.go` — Stdio-to-gRPC proxy with auto-start (activate reuses this auto-start)
- `api/proto/serena/v1/` — gRPC IPC definitions (may need new messages for activate/deactivate)

### Requirements
- `.planning/REQUIREMENTS.md` — HOOK-01 through HOOK-04 requirements

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ClaudeCodeRegistrar` in `internal/cli/setup_clients.go` — Extend Register/Unregister to handle hooks
- `SetupPrinter` in `internal/cli/setup_output.go` — Reuse for hook installation output
- `statusCmd` in `internal/cli/status.go` — Pattern for new CLI subcommands that connect to daemon via gRPC
- Forwarder auto-start in `internal/forwarder/` — `activate` can reuse this to ensure daemon is running

### Established Patterns
- Cobra subcommands added via `rootCmd.AddCommand()` — same for `activate`, `deactivate`
- gRPC IPC between CLI and daemon — same for activate/deactivate RPCs
- `RegistrationConfig` struct carries flags (DryRun, Global, Printer) — extend for hooks

### Integration Points
- `internal/cli/setup_clients.go` — Add hook write/remove logic to ClaudeCodeRegistrar
- `internal/cli/root.go` — Register `activate` and `deactivate` subcommands
- `api/proto/serena/v1/` — Add ActivateWorkspace/DeactivateWorkspace RPC messages
- `.claude/settings.json` — Target file for hook installation (read/merge/write)

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

*Phase: 36-client-hooks*
*Context gathered: 2026-04-21*
