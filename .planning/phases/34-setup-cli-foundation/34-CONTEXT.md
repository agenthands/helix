# Phase 34: Setup CLI Foundation - Context

**Gathered:** 2026-04-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can register Serena with any of 6 supported coding agents in one command (`serena setup <client>`), with automatic language detection and LS pre-installation. Covers SETUP-01 through SETUP-08.

</domain>

<decisions>
## Implementation Decisions

### CLI Structure
- **D-01:** Add `setup` as a cobra subcommand while keeping root behavior unchanged (no-args = stdio mode). This is the first subcommand, superseding the original D-02 "flat CLI" decision for this specific case.
- **D-02:** `serena setup` without a client argument lists available clients with brief descriptions (discoverable help).
- **D-03:** Support `--uninstall` flag to remove Serena registration and hooks from a client.

### Client Registration
- **D-04:** CLI-first registration where available (e.g., `claude mcp add-json` for Claude Code). Fall back to writing config files directly for clients without registration CLIs (VS Code, JetBrains, Claude Desktop).
- **D-05:** Project-scoped by default (registers for current working directory). Support `--global` flag for system-wide registration.
- **D-06:** `generic` client prints MCP config JSON to stdout by default. `--output path` writes to file instead.
- **D-07:** 6 supported clients: claude-code, vscode, jetbrains, claude-desktop, gemini-cli, generic.

### LS Pre-Install
- **D-08:** Pre-install language servers for all detected languages by default during setup. Uses existing three-tier installer (PATH/download/error).
- **D-09:** Support `--skip-install` flag to bypass LS pre-installation (for CI, air-gapped environments, or speed).

### Setup Output & Verification
- **D-10:** Full health check after registration: start daemon, verify all detected LSes respond, report per-language status before exiting.
- **D-11:** Compact output with color: short lines with checkmarks/crosses (e.g., `checkmark claude-code registered`, `checkmark gopls installed`, `cross pyright failed: ...`).
- **D-12:** Support `--dry-run` flag to show what would happen without writing any config or installing LSes.

### Claude's Discretion
- Implementation details of config file paths per client (well-documented in INSTALL.md and research)
- Error message wording and formatting
- Internal package structure within `internal/cli/`

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture
- `.planning/research/ARCHITECTURE.md` -- Integration map showing where setup components fit in the 4-layer architecture
- `.planning/research/FEATURES.md` -- Feature landscape with complexity estimates and dependency analysis

### Existing Code
- `internal/cli/root.go` -- Current cobra root command (must preserve no-args = stdio behavior)
- `internal/langregistry/installer.go` -- Three-tier LS resolution to reuse for pre-install
- `internal/skill/workflow/skill.go` -- `gatherProjectInfo()` with language detection logic to reuse
- `internal/config/config.go` -- SerenaConfig structure for config layer understanding

### Client Config Paths
- `INSTALL.md` -- Documents MCP config paths and formats for all 6 supported clients

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `langregistry.Installer` -- Three-tier LS resolution (PATH/download/error). Use directly for pre-install step.
- `gatherProjectInfo()` in `internal/skill/workflow/skill.go` -- Language detection via file extensions. Reuse for detecting project languages during setup.
- `config.Load()` -- 4-layer config loading. Setup needs to understand config paths for each client.
- cobra v1.9.1 -- Already in use for root command. Add `setup` as `rootCmd.AddCommand()`.

### Established Patterns
- Single-binary Go architecture -- setup is a new subcommand, not a separate binary
- koanf v2 config -- 4-layer precedence (CLI > project > user > profile defaults)
- `slog` logging throughout -- setup should use the same logger pattern

### Integration Points
- `internal/cli/root.go` -- Add `setup` subcommand registration
- `internal/langregistry/` -- Language detection and LS installation
- `internal/daemon/` -- Health check requires starting daemon briefly
- `internal/config/` -- Need client config path resolution

</code_context>

<specifics>
## Specific Ideas

No specific requirements -- open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None -- discussion stayed within phase scope

</deferred>

---

*Phase: 34-setup-cli-foundation*
*Context gathered: 2026-04-21*
