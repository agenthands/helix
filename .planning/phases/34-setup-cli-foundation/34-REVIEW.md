---
phase: 34-setup-cli-foundation
reviewed: 2026-04-21T14:30:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - internal/cli/root.go
  - internal/cli/setup.go
  - internal/cli/setup_clients.go
  - internal/cli/setup_detect.go
  - internal/cli/setup_health.go
  - internal/cli/setup_output.go
  - internal/cli/setup_test.go
findings:
  critical: 0
  warning: 4
  info: 2
  total: 6
status: issues_found
---

# Phase 34: Code Review Report

**Reviewed:** 2026-04-21T14:30:00Z
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

The setup CLI foundation is well-structured with clean separation across files: client registration, language detection, health checks, and output formatting. JSON config merging is done safely via encoding/json (no string concatenation). Tests cover the core paths. However, there are several issues: a flag default value that silently overrides project config, dry-run requiring external CLI binaries to be installed, and silent error swallowing in path resolution that could produce broken config file paths.

## Warnings

### WR-01: --http-addr default always overrides project config

**File:** `internal/cli/root.go:131`
**Issue:** The `--http-addr` flag has a default value of `":8080"` (line 38). In `runDaemon`, the override is gated by `if httpAddr != ""` (line 131), but since `GetString` returns the default when the flag is not explicitly set, this condition is always true. This means the `:8080` default always writes into the overrides map, silently replacing any `daemon.http_addr` value from project or user config files. The comment on line 137 even acknowledges this pattern for `--admin-addr` (Pitfall #5) but the same fix was not applied to `--http-addr`.
**Fix:**
```go
// Replace line 131:
if httpAddr != "" {
// With:
if cmd.Flags().Changed("http-addr") {
```

### WR-02: Dry-run mode blocked by exec.LookPath for CLI-based registrars

**File:** `internal/cli/setup_clients.go:140`
**Issue:** `ClaudeCodeRegistrar.Register()` calls `exec.LookPath("claude")` on line 140 before checking `cfg.DryRun` on line 156. This means `--dry-run` fails with an error if the `claude` CLI is not installed. The same pattern affects `ClaudeCodeRegistrar.Unregister()` (line 172), `GeminiCLIRegistrar.Register()` (line 205), and `GeminiCLIRegistrar.Unregister()` (line 231). Dry-run should show what would happen without requiring external dependencies. The test `TestClaudeCodeRegistrarDryRun` only passes in environments where `claude` is already installed.
**Fix:**
```go
func (r *ClaudeCodeRegistrar) Register(cfg RegistrationConfig) error {
    // Move dry-run check before LookPath
    scope := "project"
    if cfg.Global {
        scope = "user"
    }
    serverJSON, err := json.Marshal(serverConfigJSON(cfg.BinaryPath))
    if err != nil {
        return fmt.Errorf("marshaling server config: %w", err)
    }
    cmdArgs := []string{"mcp", "add-json", "serena", string(serverJSON), "--scope", scope}

    if cfg.DryRun {
        cfg.Printer.DryRunAction("would run: claude %s", strings.Join(cmdArgs, " "))
        return nil
    }

    if _, err := exec.LookPath("claude"); err != nil {
        return fmt.Errorf("claude CLI not found in PATH; install Claude Code first")
    }
    // ... rest of execution
```
Apply the same pattern to all four affected methods.

### WR-03: Silent error discard in userConfigDir produces broken paths

**File:** `internal/cli/setup_clients.go:127-129`
**Issue:** `userConfigDir()` discards the error from `os.UserConfigDir()`. If it fails (e.g., missing HOME env var in a container), it returns an empty string. Downstream callers like `VSCodeRegistrar.configPath()` (line 294) would then produce a path like `/Code/User/mcp.json`, writing config to an unexpected root-relative location. The same issue affects `os.UserHomeDir()` calls in `ClaudeDesktopRegistrar.configPath()` (lines 374, 380) and `JetBrainsRegistrar.configPath()` (line 332).
**Fix:** Propagate errors from `configPath` methods to callers:
```go
func (r *VSCodeRegistrar) configPath(cfg RegistrationConfig) (string, error) {
    if cfg.Global {
        dir, err := os.UserConfigDir()
        if err != nil {
            return "", fmt.Errorf("cannot determine user config directory: %w", err)
        }
        return filepath.Join(dir, "Code", "User", "mcp.json"), nil
    }
    return filepath.Join(cfg.ProjectDir, ".vscode", "mcp.json"), nil
}
```
This requires updating `Register` and `Unregister` to handle the returned error. Apply similarly to JetBrains and ClaudeDesktop registrars.

### WR-04: Health check message says "not responding" but only checks binary existence

**File:** `internal/cli/setup_health.go:30`
**Issue:** The failure message says `"%s (%s) not responding"` and the success message says `"responding"`, but `exec.LookPath` only checks if the binary exists in PATH -- it does not actually invoke or communicate with the language server. This is misleading to users who may think there is a runtime problem when it is simply a missing binary.
**Fix:**
```go
// Line 30:
printer.Failure("%s (%s) not found in PATH: %v", entry.Language, entry.Command, err)
// Line 33:
printer.Success("%s (%s) found at %s", entry.Language, path)
```

## Info

### IN-01: Duplicated logger setup in runForwarder and runDaemon

**File:** `internal/cli/root.go:83-93` and `internal/cli/root.go:114-123`
**Issue:** The logger construction logic (check `jsonLog`, create handler, wrap with `obs.NewContextHandler`) is duplicated verbatim between `runForwarder` and `runDaemon`. This is a minor maintainability concern -- if the logging setup changes, both locations must be updated in sync.
**Fix:** Extract a shared helper:
```go
func newLogger(jsonLog bool) *slog.Logger {
    var handler slog.Handler
    if jsonLog {
        handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
    } else {
        handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
    }
    handler = obs.NewContextHandler(handler)
    return slog.New(handler)
}
```

### IN-02: Commented-out-style TODO in health check scope note

**File:** `internal/cli/setup_health.go:13-14`
**Issue:** The comment references deferred work ("Full daemon-based health check ... deferred to Phase 35"). While this is documentation rather than a TODO marker, it should be tracked as a backlog item so it does not get lost in comments.
**Fix:** No code change needed; ensure Phase 35 planning captures HLTH-01 through HLTH-04 items referenced in the comment.

---

_Reviewed: 2026-04-21T14:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
