---
phase: 36-client-hooks
verified: 2026-04-21T19:30:00Z
status: human_needed
score: 4/4
overrides_applied: 0
human_verification:
  - test: "Run `serena setup claude-code` in a test project and verify .claude/settings.json contains SessionStart, PreToolUse, and Stop hooks"
    expected: "settings.json has hooks object with all three event types, each containing serena_managed entries"
    why_human: "Requires claude CLI installed and a real project directory; verifying full setup orchestration end-to-end"
  - test: "Start a Claude Code session in a Serena-configured project and verify workspace auto-activates"
    expected: "SessionStart hook runs `serena activate`, daemon starts, workspace is activated, agent context shows activation message"
    why_human: "Requires running Claude Code session with live daemon; cannot simulate hook invocation chain programmatically"
  - test: "In a Claude Code session, use Grep/Read 5+ times without using symbolic tools and verify nudge message appears"
    expected: "After 5th grep/read call, PreToolUse hook outputs nudge tip about find_symbol and get_symbols_overview"
    why_human: "Requires live Claude Code PreToolUse hook pipeline with stdin JSON delivery"
  - test: "End a Claude Code session and verify session-stats.json is cleaned up"
    expected: "Stop hook runs `serena deactivate`, session-stats.json is removed from .serena/"
    why_human: "Requires live Claude Code Stop hook invocation"
---

# Phase 36: Client Hooks Verification Report

**Phase Goal:** Claude Code sessions automatically activate Serena and guide agents toward symbolic tools
**Verified:** 2026-04-21T19:30:00Z
**Status:** human_needed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Starting a new Claude Code session triggers automatic workspace activation via SessionStart hook | VERIFIED | `setup_hooks.go:31-42` generates SessionStart hook config with `serena activate --workspace "$CLAUDE_PROJECT_DIR"` command; `activate.go` uses `ConnectOrStartDaemon` for auto-start; daemon handler at `daemon.go:576` calls `kernel.ActivateWorkspace` |
| 2 | PreToolUse hook nudges agents toward symbolic tools when overusing grep/read | VERIFIED | `nudge.go:98` fires nudge at threshold >= 5 GrepReadCount with 0 SerenaToolCount; `setup_hooks.go:44-55` installs PreToolUse hook matching "Grep\|Read\|Bash"; 9 nudge tests all pass |
| 3 | Ending a Claude Code session triggers cleanup via Stop hook | VERIFIED | `setup_hooks.go:57-68` generates Stop hook with `serena deactivate --workspace "$CLAUDE_PROJECT_DIR"`; `deactivate.go:49-50` removes session-stats.json; daemon notification is best-effort with silent failure tolerance |
| 4 | Running `serena setup claude-code` installs all three hooks automatically | VERIFIED | `setup_clients.go:172-185` calls `mergeHooksIntoSettings` after MCP registration; hook config includes SessionStart, PreToolUse, Stop; `--no-hooks` flag skips installation; 8 hook merge/remove tests pass |

**Score:** 4/4 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/cli/setup_hooks.go` | Hook JSON generation, merge/remove for settings.json | VERIFIED | 214 lines; exports hookSettingsPath, serenaHookConfig, mergeHooksIntoSettings, removeHooksFromSettings; uses serena_managed marker |
| `internal/cli/nudge.go` | Nudge CLI subcommand with counter tracking and threshold | VERIFIED | 198 lines; newNudgeCommand, runNudge, loadSessionStats, saveSessionStats (atomic write), isSerenaSymbolicTool (9 tools), isGrepReadTool |
| `internal/cli/activate.go` | Activate subcommand with daemon auto-start | VERIFIED | 73 lines; newActivateCommand, runActivate; uses forwarder.ConnectOrStartDaemon; 30s timeout; filepath.Abs path safety |
| `internal/cli/deactivate.go` | Deactivate subcommand with silent daemon-missing tolerance | VERIFIED | 86 lines; newDeactivateCommand, runDeactivate; removes session-stats.json; silent success if daemon not running |
| `api/proto/serena/v1/ipc.proto` | ActivateWorkspace and DeactivateWorkspace RPC definitions | VERIFIED | Contains rpc ActivateWorkspace(ActivateRequest) returns (ActivateResponse) and rpc DeactivateWorkspace(DeactivateRequest) returns (DeactivateResponse) |
| `internal/daemon/daemon.go` | gRPC handler methods for activate/deactivate | VERIFIED | forwarderServiceHandler.ActivateWorkspace at line 576 calls kernel.ActivateWorkspace; DeactivateWorkspace at line 610 acknowledges best-effort |
| `internal/forwarder/dial.go` | Exported ConnectOrStartDaemon | VERIFIED | Exported function at line 25; used by activate.go |
| `internal/cli/setup_hooks_test.go` | Tests for hook merge, remove, preserve, idempotent | VERIFIED | 8 test functions, all pass |
| `internal/cli/nudge_test.go` | Tests for counter logic, threshold, tool matching | VERIFIED | 9 test functions, all pass |
| `internal/cli/activate_test.go` | Tests for activate command structure | VERIFIED | 2 test functions, pass |
| `internal/cli/deactivate_test.go` | Tests for deactivate command structure and cleanup | VERIFIED | 3 test functions, pass |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `setup_clients.go` | `setup_hooks.go` | ClaudeCodeRegistrar.Register calls mergeHooksIntoSettings | WIRED | Line 178: `mergeHooksIntoSettings(settingsPath, cfg.BinaryPath)` |
| `setup_clients.go` | `setup_hooks.go` | ClaudeCodeRegistrar.Unregister calls removeHooksFromSettings | WIRED | Line 219: `removeHooksFromSettings(settingsPath)` |
| `nudge.go` | session-stats.json | File I/O for counter tracking | WIRED | Line 79: `filepath.Join(wsDir, ".serena", "session-stats.json")` with load/save |
| `activate.go` | `forwarder/dial.go` | forwarder.ConnectOrStartDaemon | WIRED | Line 53: `forwarder.ConnectOrStartDaemon(cmd.Context(), socketPath, logger, noop.NewTracerProvider())` |
| `activate.go` | proto ActivateWorkspace RPC | gRPC call | WIRED | Line 63: `client.ActivateWorkspace(ctx, &serenav1.ActivateRequest{...})` |
| `daemon.go` | `kernel.go` | kernel.ActivateWorkspace call from gRPC handler | WIRED | Line 597: `h.kernel.ActivateWorkspace(ctx, absPath)` |
| `root.go` | `activate.go` | rootCmd.AddCommand(newActivateCommand()) | WIRED | Line 52 |
| `root.go` | `deactivate.go` | rootCmd.AddCommand(newDeactivateCommand()) | WIRED | Line 53 |
| `root.go` | `nudge.go` | rootCmd.AddCommand(newNudgeCommand()) | WIRED | Line 54 |

### Data-Flow Trace (Level 4)

Not applicable -- phase produces CLI commands and hook configuration, not components that render dynamic data.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Project builds | `go build ./...` | Compiles (only pre-existing swift warning) | PASS |
| go vet clean | `go vet ./internal/cli/...` | Clean | PASS |
| All 22 phase tests pass | `go test ./internal/cli/... -run "TestMergeHooks\|TestRemoveHooks\|..."` | 22/22 PASS | PASS |
| nudge command registered | root.go line 54 | newNudgeCommand() present | PASS |
| activate command registered | root.go line 52 | newActivateCommand() present | PASS |
| deactivate command registered | root.go line 53 | newDeactivateCommand() present | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| HOOK-01 | 36-02 | SessionStart hook auto-activates workspace | SATISFIED | activate.go with ConnectOrStartDaemon + ActivateWorkspace gRPC; SessionStart hook config in setup_hooks.go |
| HOOK-02 | 36-01 | PreToolUse hook nudges toward symbolic tools | SATISFIED | nudge.go tracks grep/read counts, fires at threshold 5; PreToolUse hook config in setup_hooks.go |
| HOOK-03 | 36-02 | Stop hook cleans up session data | SATISFIED | deactivate.go removes session-stats.json, notifies daemon best-effort; Stop hook config in setup_hooks.go |
| HOOK-04 | 36-01 | Hooks auto-installed by serena setup claude-code | SATISFIED | setup_clients.go Register calls mergeHooksIntoSettings; --no-hooks flag available |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| None | - | - | - | No anti-patterns detected |

### Human Verification Required

### 1. End-to-End Setup Hook Installation

**Test:** Run `serena setup claude-code` in a test project directory
**Expected:** `.claude/settings.json` contains hooks object with SessionStart, PreToolUse, and Stop entries, each with `serena_managed: true`
**Why human:** Requires claude CLI installed and real project directory for full orchestration

### 2. SessionStart Auto-Activation

**Test:** Start a Claude Code session in a Serena-configured project
**Expected:** Daemon auto-starts, workspace activates, agent context shows "Serena workspace activated" message
**Why human:** Requires live Claude Code session with hook pipeline

### 3. PreToolUse Nudge Behavior

**Test:** In a Claude Code session, use Grep/Read 5+ times without symbolic tools
**Expected:** After 5th call, nudge tip about find_symbol and get_symbols_overview appears in agent context
**Why human:** Requires live Claude Code PreToolUse hook stdin JSON delivery

### 4. Stop Hook Session Cleanup

**Test:** End a Claude Code session in a Serena-configured project
**Expected:** session-stats.json is removed from `.serena/` directory
**Why human:** Requires live Claude Code Stop hook invocation

### Gaps Summary

No automated verification gaps found. All 4 roadmap success criteria are verified at the code level: artifacts exist, are substantive, are wired, and pass their test suites. The 4 human verification items cover end-to-end behavior that requires a live Claude Code session with the hook pipeline active.

---

_Verified: 2026-04-21T19:30:00Z_
_Verifier: Claude (gsd-verifier)_
