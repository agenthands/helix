# Phase 34: Setup CLI Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md -- this log preserves the alternatives considered.

**Date:** 2026-04-21
**Phase:** 34-setup-cli-foundation
**Areas discussed:** CLI structure, Client registration strategies, LS pre-install behavior, Setup output & verification

---

## CLI Structure

| Option | Description | Selected |
|--------|-------------|----------|
| Add setup subcommand only | Keep root flat for `serena` (enters stdio mode). Add `serena setup` as a subcommand tree. Minimal disruption. | checkmark |
| Full subcommand refactor | Refactor to `serena serve`, `serena setup`, `serena status` etc. Root becomes help-only. | |
| Separate binary | `serena-setup` as a separate command. Avoids touching root CLI. | |

**User's choice:** Add setup subcommand only (Recommended)
**Notes:** Preserves existing no-args behavior while adding the setup tree.

| Option | Description | Selected |
|--------|-------------|----------|
| List clients | `serena setup` alone prints available clients with brief descriptions. | checkmark |
| Require client | `serena setup` without a client is an error. | |
| Interactive picker | Launches interactive prompt to select a client. | |

**User's choice:** List clients (Recommended)
**Notes:** Discoverable -- user can see what's available without reading docs.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, include uninstall | `serena setup claude-code --uninstall` removes MCP registration and hooks. | checkmark |
| No, keep it simple | Manual removal is fine for now. | |
| You decide | Claude's discretion. | |

**User's choice:** Yes, include uninstall
**Notes:** Clean removal support from day one.

---

## Client Registration Strategies

| Option | Description | Selected |
|--------|-------------|----------|
| Write config files | CLI-first where available, file-write as fallback for clients without CLIs. | checkmark |
| Print instructions only | For clients without CLIs, print JSON snippet and file path. User pastes manually. | |
| CLI-only clients | Only support clients with CLI registration. Drop VS Code/JetBrains/Claude Desktop. | |

**User's choice:** Write config files (Recommended)
**Notes:** CLI subprocess for Claude Code/Gemini CLI, direct file writing for VS Code/JetBrains/Claude Desktop.

| Option | Description | Selected |
|--------|-------------|----------|
| Project-scoped | Registers for current working directory only. | |
| Global registration | Registers system-wide. | |
| Both via flag | Project-scoped by default, `--global` for system-wide. | checkmark |

**User's choice:** Both via flag
**Notes:** Default is project-scoped, `--global` flag for system-wide registration.

| Option | Description | Selected |
|--------|-------------|----------|
| Print JSON to stdout | Output MCP config JSON to stdout. | |
| Write to .serena/mcp.json | Write standard config file. | |
| Both | Print to stdout by default, `--output path` writes to file. | checkmark |

**User's choice:** Both
**Notes:** Generic client prints to stdout by default, `--output path` writes to file.

---

## LS Pre-Install Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Pre-install by default | Detect languages, run installer for each. Report results. | checkmark |
| Report only, install on demand | Detect and list languages, but don't install. | |
| Ask per language | Show detected languages and let user confirm which to install. | |

**User's choice:** Pre-install by default (Recommended)
**Notes:** First tool call should be instant.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes | `--skip-install` flag to bypass LS downloads. | checkmark |
| No | Pre-install always happens. | |
| You decide | Claude's discretion. | |

**User's choice:** Yes (Recommended)
**Notes:** For CI, air-gapped environments, or when user just wants registration.

---

## Setup Output & Verification

| Option | Description | Selected |
|--------|-------------|----------|
| No verification | Setup writes config and reports. Verification happens when agent starts. | |
| Basic verification | Start daemon briefly, check MCP handshake succeeds, exit. | |
| Full health check | Start daemon, check all detected LSes respond, report per-language status. | checkmark |

**User's choice:** Full health check
**Notes:** Setup starts daemon, verifies LSes respond, and reports per-language status before exiting.

| Option | Description | Selected |
|--------|-------------|----------|
| Compact with color | Short lines with checkmarks/crosses. Color-coded. | checkmark |
| Verbose table | Detailed table with columns for client, status, config path, etc. | |
| JSON output mode | Human-readable default, `--json` for machine-parseable. | |

**User's choice:** Compact with color (Recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Yes | `--dry-run` prints what would happen without writing. | checkmark |
| No | Skip dry-run. Setup is idempotent. | |
| You decide | Claude's discretion. | |

**User's choice:** Yes (Recommended)
**Notes:** Standard CLI practice for commands that modify external config.

---

## Claude's Discretion

- Config file paths per client (documented in INSTALL.md and research)
- Error message wording and formatting
- Internal package structure within `internal/cli/`

## Deferred Ideas

None -- discussion stayed within phase scope.
