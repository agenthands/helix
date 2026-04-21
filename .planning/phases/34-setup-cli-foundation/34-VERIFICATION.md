---
phase: 34-setup-cli-foundation
verified: 2026-04-21T18:30:00Z
status: human_needed
score: 8/8
overrides_applied: 0
human_verification:
  - test: "Run `serena setup claude-code` on a machine with Claude Code installed and verify serena appears in `claude mcp list`"
    expected: "serena entry appears in Claude Code MCP server list with correct command and args"
    why_human: "Requires Claude Code CLI installed and running; subprocess integration cannot be verified without the real binary"
  - test: "Run `serena setup gemini-cli` on a machine with Gemini CLI installed and verify registration"
    expected: "serena entry appears in Gemini CLI MCP server list"
    why_human: "Requires Gemini CLI installed; subprocess integration cannot be verified without the real binary"
  - test: "Run `serena setup vscode` in a project directory, open VS Code, and verify Serena appears as MCP server"
    expected: "VS Code recognizes Serena as available MCP server from .vscode/mcp.json"
    why_human: "End-to-end client integration requires the actual IDE"
  - test: "Verify colored output renders correctly in terminal (green checkmarks, red crosses, blue arrows, yellow dry-run)"
    expected: "Colors display correctly, NO_COLOR env disables them"
    why_human: "Visual terminal rendering cannot be verified programmatically"
---

# Phase 34: Setup CLI Foundation Verification Report

**Phase Goal:** Users can register Serena with any supported coding agent in one command
**Verified:** 2026-04-21T18:30:00Z
**Status:** human_needed
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Running `serena setup claude-code` invokes `claude mcp add-json` subprocess with correct JSON | VERIFIED | `ClaudeCodeRegistrar.Register()` calls `exec.Command("claude", "mcp", "add-json", "serena", serverJSON, "--scope", scope)` at setup_clients.go:161. Dry-run test passes. |
| 2 | Running `serena setup vscode` writes .vscode/mcp.json with `servers` key (not `mcpServers`) | VERIFIED | `VSCodeRegistrar.Register()` calls `mergeJSONConfig(path, "servers", ...)` at setup_clients.go:279. Unit test `TestVSCodeRegistrarRegister` confirms `servers` key with `serena` entry containing `type: stdio`. |
| 3 | Running `serena setup jetbrains` writes .junie/mcp/mcp.json with `mcpServers` key | VERIFIED | `JetBrainsRegistrar.Register()` calls `mergeJSONConfig(path, "mcpServers", ...)` at setup_clients.go:316. Unit test `TestJetBrainsRegistrarRegister` confirms correct path and key. |
| 4 | Running `serena setup claude-desktop` merges into claude_desktop_config.json preserving existing keys | VERIFIED | `ClaudeDesktopRegistrar.Register()` uses `mergeJSONConfig` at setup_clients.go:355 with platform-specific path via `runtime.GOOS`. `TestMergeJSONConfigPreservesNonMCPKeys` confirms key preservation. |
| 5 | Running `serena setup gemini-cli` invokes `gemini mcp add` subprocess with correct args | VERIFIED | `GeminiCLIRegistrar.Register()` calls `exec.Command("gemini", "mcp", "add", "--scope", scope, "-t", "stdio", "serena", binaryPath, "--", "--mode=stdio")` at setup_clients.go:221. Dry-run test passes. |
| 6 | Running `serena setup generic` prints MCP JSON to stdout | VERIFIED | `GenericRegistrar.Register()` writes to `os.Stdout` at setup_clients.go:426. `TestGenericRegistrarStdout` captures stdout and confirms valid JSON with `mcpServers.serena`. |
| 7 | Setup detects languages from file extensions using Registry.ByExtension() | VERIFIED | `detectLanguages()` in setup_detect.go walks dirs with `filepath.WalkDir`, calls `reg.ByExtension(ext)`. `TestDetectLanguages` confirms Go/Python detection from temp dir. Live run detects 52 languages. |
| 8 | CLI-based clients use subprocess for config stability | VERIFIED | ClaudeCodeRegistrar and GeminiCLIRegistrar use `exec.Command` (setup_clients.go:161, 221). VSCode/JetBrains/ClaudeDesktop use file-write (correct per requirements -- only CLI-based clients use subprocess). |

**Score:** 8/8 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/cli/setup.go` | Setup cobra command, RegistrationConfig, client registry, runSetup orchestrator | VERIFIED | 171 lines. newSetupCommand, runSetup, resolveBinaryPath, listClients. All flags wired. langregistry integration present. |
| `internal/cli/setup_clients.go` | ClientRegistrar interface and 6 implementations | VERIFIED | 440 lines. Interface + 6 registrars + mergeJSONConfig + removeFromJSONConfig + serverConfigJSON helpers. |
| `internal/cli/setup_detect.go` | Language detection via file walking + Registry.ByExtension, LS pre-installation | VERIFIED | 97 lines. detectLanguages with WalkDir + ByExtension, preInstallLanguageServers with Installer.Resolve. |
| `internal/cli/setup_health.go` | Post-registration health check via exec.LookPath | VERIFIED | 37 lines. runHealthCheck verifies LS binary existence. Informational, non-blocking. |
| `internal/cli/setup_output.go` | Colored terminal output helpers | VERIFIED | 48 lines. SetupPrinter with Success/Failure/Info/DryRunAction. Uses fatih/color, outputs to stderr. |
| `internal/cli/setup_test.go` | Unit tests for all setup components | VERIFIED | 509 lines, 22 tests. All pass. Covers mergeJSON, removeJSON, all registrars, detect, health, command integration. |
| `internal/cli/root.go` | Root command with setup subcommand wired | VERIFIED | Line 50: `rootCmd.AddCommand(newSetupCommand())`. RunE: runRoot preserved at line 23. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| root.go | setup.go | `rootCmd.AddCommand(newSetupCommand())` | WIRED | Line 50 of root.go |
| setup.go | setup_clients.go | `clientRegistry()` map lookup | WIRED | Lines 42, 54 of setup.go |
| setup.go | setup_output.go | `SetupPrinter` usage | WIRED | Lines 43, 82, 108, 142 of setup.go |
| setup.go | setup_detect.go | `detectLanguages()` call | WIRED | Line 118 of setup.go |
| setup.go | setup_health.go | `runHealthCheck()` call | WIRED | Line 134 of setup.go |
| setup_detect.go | langregistry | `Registry.ByExtension()` | WIRED | Line 45 of setup_detect.go |
| setup_detect.go | langregistry | `Installer.Resolve()` | WIRED | Line 87 of setup_detect.go |
| setup.go | langregistry | `NewRegistry()` + `NewInstaller()` | WIRED | Lines 111, 125 of setup.go |

### Data-Flow Trace (Level 4)

Not applicable -- CLI command, not a data-rendering component. The data flow is: user input -> cobra args -> registrar dispatch -> filesystem/subprocess. Verified via behavioral spot-checks below.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| List all 6 clients | `./serena setup` | Lists claude-code, claude-desktop, gemini-cli, generic, jetbrains, vscode with descriptions | PASS |
| Show all flags in help | `./serena setup --help` | Shows --global, --uninstall, --skip-install, --dry-run, --output | PASS |
| Dry-run generic outputs steps | `./serena setup --dry-run generic` | Shows dry-run print, registration, language detection (52 langs), install plans, health check | PASS |
| Invalid client error | `./serena setup invalid-client` | Exits 1 with "unknown client" listing valid clients | PASS |
| Build succeeds | `go build ./cmd/serena` | Exit 0 (warning in swift binding, not related) | PASS |
| Vet passes | `go vet ./internal/cli/...` | Exit 0 | PASS |
| All tests pass | `go test ./internal/cli/... -count=1` | 22/22 tests pass | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| SETUP-01 | 34-01 | claude-code registration via `claude mcp add-json` subprocess | SATISFIED | ClaudeCodeRegistrar uses exec.Command("claude", "mcp", "add-json", ...) |
| SETUP-02 | 34-01 | vscode registration writes .vscode/mcp.json with `servers` key | SATISFIED | VSCodeRegistrar writes with "servers" key, confirmed by unit test |
| SETUP-03 | 34-01 | jetbrains registration writes .junie/mcp/mcp.json with `mcpServers` key | SATISFIED | JetBrainsRegistrar writes with "mcpServers" key, confirmed by unit test |
| SETUP-04 | 34-01 | claude-desktop merges into claude_desktop_config.json preserving existing keys | SATISFIED | ClaudeDesktopRegistrar uses mergeJSONConfig with platform-specific path |
| SETUP-05 | 34-01 | gemini-cli registration via `gemini mcp add` subprocess | SATISFIED | GeminiCLIRegistrar uses exec.Command("gemini", "mcp", "add", ...) |
| SETUP-06 | 34-01 | generic prints MCP JSON to stdout | SATISFIED | GenericRegistrar writes to os.Stdout, confirmed by stdout capture test |
| SETUP-07 | 34-02 | Language detection from file extensions using Registry.ByExtension() | SATISFIED | detectLanguages uses WalkDir + ByExtension, detects 52 languages in live run |
| SETUP-08 | 34-01 | CLI-based clients use subprocess for config stability | SATISFIED | claude-code and gemini-cli use exec.Command; file-writers (vscode, jetbrains, claude-desktop) correctly use mergeJSONConfig |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | - | No TODOs, FIXMEs, stubs, or placeholder content found | - | - |

### Human Verification Required

### 1. Claude Code End-to-End Registration

**Test:** Run `serena setup claude-code` on a machine with Claude Code installed and verify serena appears in `claude mcp list`
**Expected:** serena entry appears in Claude Code MCP server list with correct command and args
**Why human:** Requires Claude Code CLI installed and running; subprocess integration cannot be verified without the real binary

### 2. Gemini CLI End-to-End Registration

**Test:** Run `serena setup gemini-cli` on a machine with Gemini CLI installed and verify registration
**Expected:** serena entry appears in Gemini CLI MCP server list
**Why human:** Requires Gemini CLI installed; subprocess integration cannot be verified without the real binary

### 3. VS Code MCP Server Recognition

**Test:** Run `serena setup vscode` in a project directory, open VS Code, and verify Serena appears as MCP server
**Expected:** VS Code recognizes Serena as available MCP server from .vscode/mcp.json
**Why human:** End-to-end client integration requires the actual IDE

### 4. Colored Terminal Output

**Test:** Verify colored output renders correctly in terminal (green checkmarks, red crosses, blue arrows, yellow dry-run)
**Expected:** Colors display correctly, NO_COLOR env disables them
**Why human:** Visual terminal rendering cannot be verified programmatically

### Gaps Summary

No automated gaps found. All 8 must-have truths are verified against the codebase. All 8 requirements (SETUP-01 through SETUP-08) are satisfied with implementation evidence. All artifacts exist, are substantive, and are properly wired. All 22 unit tests pass.

4 items require human verification: end-to-end integration testing with real client CLIs (claude-code, gemini-cli, vscode) and visual terminal output verification.

---

_Verified: 2026-04-21T18:30:00Z_
_Verifier: Claude (gsd-verifier)_
