# Phase 34: Setup CLI Foundation - Research

**Researched:** 2026-04-21
**Domain:** CLI subcommand, client MCP registration, language detection
**Confidence:** HIGH

## Summary

Phase 34 adds a `serena setup <client>` subcommand that registers Serena as an MCP server with 6 supported coding agents. The implementation reuses the existing cobra root command (adding `setup` as a child), the `langregistry` package for language detection and LS installation, and the `gatherProjectInfo()` language detection logic from the workflow skill.

Two registration strategies are needed: CLI-first (Claude Code via `claude mcp add`, Gemini CLI via `gemini mcp add`) and file-writing (VS Code `.vscode/mcp.json`, JetBrains `.junie/mcp/mcp.json`, Claude Desktop `claude_desktop_config.json`). The `generic` client simply outputs JSON to stdout or a file.

**Primary recommendation:** Create an `internal/cli/setup/` package with a client registry pattern where each client implements a `ClientRegistrar` interface. The setup command orchestrates: resolve binary path, detect languages, register with client, pre-install LSes, run health check.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Add `setup` as a cobra subcommand while keeping root behavior unchanged (no-args = stdio mode). This is the first subcommand, superseding the original D-02 "flat CLI" decision for this specific case.
- **D-02:** `serena setup` without a client argument lists available clients with brief descriptions (discoverable help).
- **D-03:** Support `--uninstall` flag to remove Serena registration and hooks from a client.
- **D-04:** CLI-first registration where available (e.g., `claude mcp add-json` for Claude Code). Fall back to writing config files directly for clients without registration CLIs.
- **D-05:** Project-scoped by default (registers for current working directory). Support `--global` flag for system-wide registration.
- **D-06:** `generic` client prints MCP config JSON to stdout by default. `--output path` writes to file instead.
- **D-07:** 6 supported clients: claude-code, vscode, jetbrains, claude-desktop, gemini-cli, generic.
- **D-08:** Pre-install language servers for all detected languages by default during setup. Uses existing three-tier installer (PATH/download/error).
- **D-09:** Support `--skip-install` flag to bypass LS pre-installation.
- **D-10:** Full health check after registration: start daemon, verify all detected LSes respond, report per-language status before exiting.
- **D-11:** Compact output with color: short lines with checkmarks/crosses.
- **D-12:** Support `--dry-run` flag to show what would happen without writing any config or installing LSes.

### Claude's Discretion
- Implementation details of config file paths per client
- Error message wording and formatting
- Internal package structure within `internal/cli/`

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SETUP-01 | `serena setup claude-code` registers MCP server and installs hooks | Claude Code has `claude mcp add-json` CLI (verified on this machine). Scope: `--scope project` (default) or `--scope user` for global. |
| SETUP-02 | `serena setup vscode` registers MCP server for VS Code | Write `.vscode/mcp.json` (project) or `~/Library/Application Support/Code/User/mcp.json` (global). JSON uses `"servers"` key, not `"mcpServers"`. |
| SETUP-03 | `serena setup jetbrains` registers MCP server for JetBrains IDEs | Write `.junie/mcp/mcp.json` (project) or `~/.junie/mcp/mcp.json` (global). JSON uses `"mcpServers"` key. |
| SETUP-04 | `serena setup claude-desktop` registers MCP server for Claude Desktop | Write `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS), `%APPDATA%\Claude\` (Windows), `~/.config/Claude/` (Linux). JSON uses `"mcpServers"` key. |
| SETUP-05 | `serena setup gemini-cli` registers MCP server for Gemini CLI | Gemini CLI has `gemini mcp add` CLI (verified on this machine). Scope: `--scope project` (default) or `--scope user` for global. |
| SETUP-06 | `serena setup generic` outputs MCP config for any stdio client | Print JSON to stdout (default) or write to `--output path`. Standard `mcpServers` format. |
| SETUP-07 | Setup detects project languages and pre-installs available LSes | Reuse `langregistry.Registry.ByExtension()` for detection and `langregistry.Installer.Resolve()` for installation. Existing `gatherProjectInfo()` pattern shows file walking approach. |
| SETUP-08 | Setup uses client CLIs as subprocess (not direct file manipulation) for config stability | `claude mcp add-json` and `gemini mcp add` verified available. VS Code, JetBrains, Claude Desktop have no registration CLIs -- file writing is the only option. |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| CLI subcommand parsing | CLI Layer (cobra) | -- | Cobra handles argument parsing, flag binding, help text |
| Client registration | CLI Layer | OS filesystem | CLI subprocess calls or config file writes -- purely a client-side operation |
| Language detection | Kernel (langregistry) | CLI Layer (file walking) | Registry owns extension-to-language mapping; CLI walks directory |
| LS pre-installation | Kernel (langregistry/Installer) | -- | Three-tier resolver already handles PATH/download/error |
| Health check | Daemon Layer | CLI Layer (reporting) | Requires starting daemon briefly to verify LS connectivity |
| Output formatting | CLI Layer | -- | Color output, checkmarks, dry-run reporting |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| spf13/cobra | v1.9.1 | CLI framework | Already in use for root command [VERIFIED: go.mod] |
| fatih/color | latest | Terminal color output | De facto Go standard for cross-platform colored terminal output [ASSUMED] |

### Supporting (existing, no new dependencies)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| langregistry.Registry | internal | Language detection via ByExtension() | Detect project languages from file extensions |
| langregistry.Installer | internal | Three-tier LS resolution | Pre-install language servers |
| os/exec | stdlib | Subprocess execution | Run `claude mcp add-json`, `gemini mcp add` |
| encoding/json | stdlib | JSON config generation/manipulation | Write/merge config files for VS Code, JetBrains, Claude Desktop |
| os.Executable() | stdlib | Resolve running binary path | Get absolute path to serena binary for MCP config |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| fatih/color | ANSI codes directly | fatih/color handles Windows terminal compat, NO_COLOR env |
| os/exec for CLI registration | Direct file write for all clients | CLI subprocess is more stable (D-04, D-08) but adds external dependency |

**Installation:**
```bash
go get github.com/fatih/color
```

## Architecture Patterns

### System Architecture Diagram

```
serena setup <client> [flags]
        |
        v
  +-----------------+
  | SetupCommand     |  cobra subcommand, parses flags
  | (internal/cli/) |
  +-----------------+
        |
        +---> ResolveBinaryPath()  -- os.Executable() + filepath.EvalSymlinks
        |
        +---> DetectLanguages(dir)  -- walk files, Registry.ByExtension()
        |
        +---> ClientRegistrar.Register(cfg)
        |       |
        |       +---> [claude-code]  exec: claude mcp add-json "serena" '{...}'
        |       +---> [gemini-cli]   exec: gemini mcp add serena <cmd> -- <args>
        |       +---> [vscode]       write: .vscode/mcp.json (merge)
        |       +---> [jetbrains]    write: .junie/mcp/mcp.json (merge)
        |       +---> [claude-desktop] write: claude_desktop_config.json (merge)
        |       +---> [generic]      stdout or --output file
        |
        +---> PreInstallLSes(languages)  -- Installer.Resolve() per language
        |
        +---> HealthCheck()  -- start daemon, verify LS responses
        |
        v
  Compact colored output with per-step status
```

### Recommended Project Structure
```
internal/cli/
  root.go              # Existing -- add setup subcommand registration
  setup.go             # Setup command definition, flag parsing, orchestration
  setup_clients.go     # ClientRegistrar interface + all 6 client implementations
  setup_detect.go      # Language detection (walk + ByExtension)
  setup_health.go      # Health check logic (daemon start + LS verify)
  setup_output.go      # Colored terminal output helpers
  setup_test.go        # Unit tests
```

### Pattern 1: ClientRegistrar Interface
**What:** Each client type implements a common interface for registration and unregistration.
**When to use:** All 6 client types.
**Example:**
```go
// ClientRegistrar handles MCP registration for a specific coding agent.
type ClientRegistrar interface {
    // Name returns the client identifier (e.g., "claude-code").
    Name() string
    // Description returns a one-line description for help output.
    Description() string
    // Register adds Serena as an MCP server for this client.
    Register(cfg RegistrationConfig) error
    // Unregister removes Serena from this client.
    Unregister(cfg RegistrationConfig) error
}

// RegistrationConfig holds common registration parameters.
type RegistrationConfig struct {
    BinaryPath string   // Absolute path to serena binary
    Global     bool     // --global flag: user-scoped vs project-scoped
    DryRun     bool     // --dry-run flag: print actions without executing
    ProjectDir string   // Current working directory
}
```

### Pattern 2: Config File Merge (for file-writing clients)
**What:** Read existing JSON, merge `mcpServers.serena` key, write back. Preserves user's other entries.
**When to use:** VS Code, JetBrains, Claude Desktop.
**Example:**
```go
func mergeJSONConfig(path string, key string, serverName string, serverConfig map[string]any) error {
    // Read existing file (or start with empty object)
    existing := make(map[string]any)
    if data, err := os.ReadFile(path); err == nil {
        json.Unmarshal(data, &existing)
    }
    // Ensure the servers key exists
    servers, ok := existing[key].(map[string]any)
    if !ok {
        servers = make(map[string]any)
    }
    servers[serverName] = serverConfig
    existing[key] = servers
    // Write back with indentation
    data, _ := json.MarshalIndent(existing, "", "  ")
    return os.WriteFile(path, data, 0644)
}
```

### Pattern 3: CLI Subprocess Registration
**What:** Use `os/exec` to call client CLIs for registration.
**When to use:** Claude Code, Gemini CLI.
**Example:**
```go
// Claude Code registration via CLI
func (c *ClaudeCodeRegistrar) Register(cfg RegistrationConfig) error {
    serverJSON, _ := json.Marshal(map[string]any{
        "command": cfg.BinaryPath,
        "args":    []string{"--mode=stdio"},
    })
    scope := "project"
    if cfg.Global {
        scope = "user"
    }
    cmd := exec.Command("claude", "mcp", "add-json", "serena", string(serverJSON), "--scope", scope)
    cmd.Stdout = os.Stderr  // Show claude CLI output
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

### Anti-Patterns to Avoid
- **Hardcoding binary path:** Always use `os.Executable()` + `filepath.EvalSymlinks()` to resolve the actual binary location. Users may install via `go install`, manual download, or package managers.
- **Overwriting config files:** Never truncate existing config files. Always read-merge-write to preserve user's other MCP server entries.
- **Assuming client CLI is in PATH:** Check `exec.LookPath()` before attempting CLI registration; fall back to file writing with a warning if the CLI is missing.
- **Blocking on LS installation:** LS installation can be slow (npm, pip downloads). Show progress per-language, don't batch silently.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Language detection | Custom file scanner | `langregistry.Registry.ByExtension()` | Already handles 52 languages with proper extension mapping |
| LS installation | Custom installers | `langregistry.Installer.Resolve()` | Handles npm/pip/cargo/gem/dotnet/binary with checksums |
| Terminal colors | Raw ANSI codes | `fatih/color` or stdlib ANSI | NO_COLOR env support, Windows compat |
| JSON merge | String manipulation | `encoding/json` marshal/unmarshal | Preserves structure, handles edge cases |
| Config path resolution | Hardcoded paths | Platform-aware `os.UserConfigDir()` + known suffixes | Cross-platform (macOS/Linux/Windows) |

**Key insight:** The existing `langregistry` package already solves the two hardest sub-problems (language detection and LS installation). The setup command is primarily orchestration and client-specific config formatting.

## Common Pitfalls

### Pitfall 1: VS Code Uses "servers" Key, Not "mcpServers"
**What goes wrong:** Writing `"mcpServers"` to `.vscode/mcp.json` which VS Code ignores.
**Why it happens:** Every other client uses `"mcpServers"` as the top-level key.
**How to avoid:** VS Code uses `{"servers": {"serena": {...}}}` with a `"type": "stdio"` field inside each server entry. [VERIFIED: code.visualstudio.com/docs/copilot/reference/mcp-configuration]
**Warning signs:** VS Code shows no MCP tools after setup.

### Pitfall 2: Claude Desktop Config Has Non-MCP Keys
**What goes wrong:** Overwriting `claude_desktop_config.json` and losing user's preferences.
**Why it happens:** The file contains `"preferences"` and other keys alongside `"mcpServers"`. [VERIFIED: checked actual file on this machine]
**How to avoid:** Always read-merge-write. Only touch the `"mcpServers"` key.
**Warning signs:** User loses Claude Desktop settings after running setup.

### Pitfall 3: Root Command Behavior Change
**What goes wrong:** Adding a subcommand changes cobra's behavior when no args are provided -- it may show help instead of running stdio mode.
**Why it happens:** cobra's default behavior with subcommands is to show help when no subcommand is given.
**How to avoid:** Set `rootCmd.RunE = runRoot` (already done). With both `RunE` and subcommands, cobra runs `RunE` when no subcommand matches. Verify with tests. [VERIFIED: internal/cli/root.go line 22 already has RunE set]
**Warning signs:** Running bare `serena` shows help instead of starting stdio forwarder.

### Pitfall 4: Symlinked Binary Path Resolution
**What goes wrong:** `os.Executable()` returns a symlink path, client CLI stores it, symlink changes later.
**Why it happens:** `go install` creates binary in GOPATH/bin, Homebrew uses symlinks.
**How to avoid:** Use `filepath.EvalSymlinks(os.Executable())` to resolve to the actual binary. [VERIFIED: os.Executable docs say symlink behavior varies by OS]
**Warning signs:** MCP server stops working after binary update.

### Pitfall 5: Gemini CLI vs Claude CLI Scope Flags Differ
**What goes wrong:** Using `--scope local` for Gemini CLI (only supports `user` or `project`).
**Why it happens:** Claude CLI has `local/user/project` scopes; Gemini CLI only has `user/project`.
**How to avoid:** Map `--global` flag to the correct scope per client. Claude Code: `--scope project` (default) / `--scope user` (global). Gemini CLI: `--scope project` (default) / `--scope user` (global). [VERIFIED: CLI --help output on this machine]
**Warning signs:** Registration fails with "invalid scope" error.

### Pitfall 6: Health Check Requires Daemon Bootstrap
**What goes wrong:** Health check tries to verify LSes but daemon isn't running.
**Why it happens:** D-10 says "start daemon, verify all detected LSes respond."
**How to avoid:** The health check needs to temporarily start the daemon (or connect to a running one), send initialization requests, and verify LS responses before shutting down. This is the most complex part of the phase.
**Warning signs:** Health check hangs or times out.

## Code Examples

### Resolving Binary Path
```go
// Source: Go stdlib os.Executable + filepath.EvalSymlinks
func resolveBinaryPath() (string, error) {
    exe, err := os.Executable()
    if err != nil {
        return "", fmt.Errorf("cannot determine serena binary path: %w", err)
    }
    resolved, err := filepath.EvalSymlinks(exe)
    if err != nil {
        return exe, nil // fall back to unresolved
    }
    return resolved, nil
}
```

### Language Detection Using Registry
```go
// Source: internal/langregistry/registry.go ByExtension()
func detectLanguages(dir string, reg *langregistry.Registry) []langregistry.LSEntry {
    seen := make(map[string]bool)
    var entries []langregistry.LSEntry
    
    filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() {
            if d != nil && d.IsDir() && strings.HasPrefix(d.Name(), ".") {
                return filepath.SkipDir
            }
            return err
        }
        ext := filepath.Ext(d.Name())
        if ext == "" {
            return nil
        }
        matches := reg.ByExtension(ext)
        for _, m := range matches {
            if !seen[m.Language] {
                seen[m.Language] = true
                entries = append(entries, m)
            }
        }
        return nil
    })
    return entries
}
```

### Claude Code CLI Registration
```go
// Verified: claude mcp add-json --help on this machine
func registerClaudeCode(binaryPath string, global bool) error {
    serverJSON, _ := json.Marshal(map[string]any{
        "command": binaryPath,
        "args":    []string{"--mode=stdio"},
    })
    scope := "project"
    if global {
        scope = "user"
    }
    cmd := exec.Command("claude", "mcp", "add-json", "serena",
        string(serverJSON), "--scope", scope)
    cmd.Stdout = os.Stderr
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

### Gemini CLI Registration
```go
// Verified: gemini mcp add --help on this machine
func registerGeminiCLI(binaryPath string, global bool) error {
    scope := "project"
    if global {
        scope = "user"
    }
    cmd := exec.Command("gemini", "mcp", "add",
        "--scope", scope,
        "-t", "stdio",
        "serena", binaryPath, "--", "--mode=stdio")
    cmd.Stdout = os.Stderr
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
```

### VS Code Config Write
```go
// Source: https://code.visualstudio.com/docs/copilot/reference/mcp-configuration
// Note: VS Code uses "servers" key (NOT "mcpServers") and requires "type" field
func writeVSCodeConfig(binaryPath string, global bool, projectDir string) error {
    var configPath string
    if global {
        configDir, _ := os.UserConfigDir() // ~/Library/Application Support on macOS
        configPath = filepath.Join(configDir, "Code", "User", "mcp.json")
    } else {
        configPath = filepath.Join(projectDir, ".vscode", "mcp.json")
    }
    
    serverEntry := map[string]any{
        "type":    "stdio",
        "command": binaryPath,
        "args":    []string{"--mode=stdio"},
    }
    return mergeJSONConfig(configPath, "servers", "serena", serverEntry)
}
```

## Client Config Reference Table

| Client | CLI Available | Registration Method | Config Key | Project Path | Global Path (macOS) |
|--------|-------------|---------------------|------------|-------------|---------------------|
| claude-code | Yes | `claude mcp add-json` | `mcpServers` | `.claude/settings.json` (managed by CLI) | `~/.claude/settings.json` |
| gemini-cli | Yes | `gemini mcp add` | N/A (CLI-managed) | `.gemini/settings.json` (managed by CLI) | `~/.gemini/settings.json` |
| vscode | No | File write | `servers` | `.vscode/mcp.json` | `~/Library/Application Support/Code/User/mcp.json` |
| jetbrains | No | File write | `mcpServers` | `.junie/mcp/mcp.json` | `~/.junie/mcp/mcp.json` |
| claude-desktop | No | File write | `mcpServers` | N/A (app is global) | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| generic | No | stdout / --output | `mcpServers` | N/A | N/A |

[VERIFIED: claude CLI and gemini CLI --help output on this machine]
[VERIFIED: VS Code docs at code.visualstudio.com]
[VERIFIED: JetBrains Junie docs at junie.jetbrains.com]
[VERIFIED: Claude Desktop config file exists on this machine at ~/Library/Application Support/Claude/]

### Platform-Specific Global Paths

| Client | macOS | Linux | Windows |
|--------|-------|-------|---------|
| vscode | `~/Library/Application Support/Code/User/mcp.json` | `~/.config/Code/User/mcp.json` | `%APPDATA%\Code\User\mcp.json` |
| jetbrains | `~/.junie/mcp/mcp.json` | `~/.junie/mcp/mcp.json` | `~/.junie/mcp/mcp.json` |
| claude-desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` | `~/.config/Claude/claude_desktop_config.json` | `%APPDATA%\Claude\claude_desktop_config.json` |

[ASSUMED] Linux and Windows paths for VS Code and Claude Desktop are based on standard XDG/APPDATA conventions. Claude Code and Gemini CLI manage their own paths via their CLIs so no path knowledge needed.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual JSON editing per client | CLI-based registration (Claude Code, Gemini CLI) | 2025 | Clients with CLIs should use them for stability |
| `.cursor/mcp.json` style | `.vscode/mcp.json` with `"servers"` key | VS Code 1.99+ | VS Code uses different key from all other clients |
| JetBrains MCP in IDE settings | `.junie/mcp/mcp.json` file | Junie launch 2025 | File-based config alongside GUI settings |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `fatih/color` is the best choice for terminal color output | Standard Stack | Low -- could use stdlib ANSI or lipgloss instead; easy to swap |
| A2 | Linux/Windows config paths follow standard XDG/APPDATA conventions | Client Config Reference | Medium -- wrong path means global registration fails on non-macOS |
| A3 | Health check can start daemon temporarily for verification | Pitfalls | Medium -- may require specific daemon bootstrap mode or reuse existing socket |

## Open Questions

1. **Health check implementation complexity**
   - What we know: D-10 requires starting daemon and verifying LS responses
   - What's unclear: Whether we can reuse the existing daemon bootstrap or need a lightweight probe mode
   - Recommendation: Implement as "start daemon in background, send a test request per detected language, report results, shut down". May need a short timeout per LS (5-10s).

2. **Uninstall for CLI-registered clients**
   - What we know: `claude mcp remove serena` and `gemini mcp remove serena` exist
   - What's unclear: Whether remove commands use the same scope flags
   - Recommendation: Verify during implementation; fallback is to try all scopes.

3. **Claude Code `--scope local` vs `--scope project`**
   - What we know: Claude CLI has `local`, `user`, `project` scopes. `local` is default.
   - What's unclear: Exact difference between `local` and `project` scope in Claude Code.
   - Recommendation: Use `--scope project` for project-scoped (writes to `.mcp.json` in project root) and `--scope user` for global.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (if used) |
| Config file | None needed -- Go test conventions |
| Quick run command | `go test ./internal/cli/ -run TestSetup -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SETUP-01 | Claude Code registration via CLI subprocess | unit (mock exec) | `go test ./internal/cli/ -run TestClaudeCode -count=1` | No -- Wave 0 |
| SETUP-02 | VS Code config file write + merge | unit | `go test ./internal/cli/ -run TestVSCode -count=1` | No -- Wave 0 |
| SETUP-03 | JetBrains config file write + merge | unit | `go test ./internal/cli/ -run TestJetBrains -count=1` | No -- Wave 0 |
| SETUP-04 | Claude Desktop config merge | unit | `go test ./internal/cli/ -run TestClaudeDesktop -count=1` | No -- Wave 0 |
| SETUP-05 | Gemini CLI registration via subprocess | unit (mock exec) | `go test ./internal/cli/ -run TestGeminiCLI -count=1` | No -- Wave 0 |
| SETUP-06 | Generic JSON output to stdout/file | unit | `go test ./internal/cli/ -run TestGeneric -count=1` | No -- Wave 0 |
| SETUP-07 | Language detection + LS pre-install | unit | `go test ./internal/cli/ -run TestDetect -count=1` | No -- Wave 0 |
| SETUP-08 | CLI subprocess used where available | integration | Manual verify via `--dry-run` | N/A |

### Sampling Rate
- **Per task commit:** `go test ./internal/cli/ -count=1`
- **Per wave merge:** `go test ./... -count=1 && go vet ./...`
- **Phase gate:** Full suite green + `go vet` clean

### Wave 0 Gaps
- [ ] `internal/cli/setup_test.go` -- covers all SETUP-* requirements
- [ ] Test helpers for temp directory creation, config file assertions, exec mocking

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | N/A |
| V3 Session Management | No | N/A |
| V4 Access Control | No | N/A |
| V5 Input Validation | Yes | Validate client name against allowlist of 6 known clients |
| V6 Cryptography | No | N/A |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Config file injection | Tampering | JSON marshal (not string concat) for all config writes |
| PATH hijacking of client CLIs | Elevation | Use `exec.LookPath` but accept risk (user's PATH is trusted) |
| Symlink attack on config files | Tampering | `os.WriteFile` with `O_CREATE|O_TRUNC` to known paths; don't follow symlinks for config dirs |

## Sources

### Primary (HIGH confidence)
- `internal/cli/root.go` -- Verified existing cobra root command structure
- `internal/langregistry/` -- Verified Registry.ByExtension(), Installer.Resolve() APIs
- `internal/skill/workflow/skill.go` -- Verified gatherProjectInfo() language detection pattern
- `claude mcp add-json --help` -- Verified CLI interface on this machine
- `gemini mcp add --help` -- Verified CLI interface on this machine
- [VS Code MCP Config Reference](https://code.visualstudio.com/docs/copilot/reference/mcp-configuration) -- Verified `servers` key and JSON structure
- [Junie MCP Configuration](https://junie.jetbrains.com/docs/junie-cli-mcp-configuration.html) -- Verified `.junie/mcp/mcp.json` path and format
- Claude Desktop config file -- Verified exists at `~/Library/Application Support/Claude/claude_desktop_config.json` on this machine

### Secondary (MEDIUM confidence)
- Cross-platform config paths for VS Code, Claude Desktop on Linux/Windows (standard conventions)

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries already in use or well-known Go stdlib
- Architecture: HIGH -- straightforward CLI subcommand + interface pattern; existing code provides all complex subsystems
- Pitfalls: HIGH -- verified client config formats and CLI interfaces directly on this machine

**Research date:** 2026-04-21
**Valid until:** 2026-05-21 (client CLI interfaces change slowly; config paths may evolve)
