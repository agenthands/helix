---
phase: 34-setup-cli-foundation
fixed_at: 2026-04-21T14:45:00Z
review_path: .planning/phases/34-setup-cli-foundation/34-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 34: Code Review Fix Report

**Fixed at:** 2026-04-21T14:45:00Z
**Source review:** .planning/phases/34-setup-cli-foundation/34-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4
- Fixed: 4
- Skipped: 0

## Fixed Issues

### WR-01: --http-addr default always overrides project config

**Files modified:** `internal/cli/root.go`
**Commit:** 52f9cb67
**Applied fix:** Changed `if httpAddr != ""` to `if cmd.Flags().Changed("http-addr")` so the `:8080` default does not silently override project/user config values for `daemon.http_addr`.

### WR-02: Dry-run mode blocked by exec.LookPath for CLI-based registrars

**Files modified:** `internal/cli/setup_clients.go`
**Commit:** 7d299f08
**Applied fix:** Moved dry-run check before `exec.LookPath` in all four affected methods: `ClaudeCodeRegistrar.Register()`, `ClaudeCodeRegistrar.Unregister()`, `GeminiCLIRegistrar.Register()`, `GeminiCLIRegistrar.Unregister()`. Now `--dry-run` works without requiring external CLI binaries to be installed.

### WR-03: Silent error discard in userConfigDir produces broken paths

**Files modified:** `internal/cli/setup_clients.go`
**Commit:** d94742c2
**Applied fix:** Changed `userConfigDir()` to return `(string, error)`. Updated `VSCodeRegistrar.configPath()`, `JetBrainsRegistrar.configPath()`, and `ClaudeDesktopRegistrar.configPath()` to return `(string, error)` and propagate errors from `os.UserConfigDir()` / `os.UserHomeDir()`. Updated all Register/Unregister callers to handle the returned error. Also added `APPDATA` env var check for Windows path in ClaudeDesktop.

### WR-04: Health check message says "not responding" but only checks binary existence

**Files modified:** `internal/cli/setup_health.go`
**Commit:** 2b59ee30
**Applied fix:** Changed failure message from `"not responding"` to `"not found in PATH"` and success message from `"responding"` to `"found at <path>"` to accurately describe the LookPath-based check.

---

_Fixed: 2026-04-21T14:45:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
