---
phase: 34-setup-cli-foundation
plan: 01
subsystem: cli
tags: [setup, mcp-registration, cobra, cli]
dependency_graph:
  requires: []
  provides: [setup-command, client-registrars, setup-output]
  affects: [internal/cli/root.go]
tech_stack:
  added: [fatih/color]
  patterns: [client-registrar-interface, json-merge-write, cli-subprocess]
key_files:
  created:
    - internal/cli/setup.go
    - internal/cli/setup_clients.go
    - internal/cli/setup_output.go
  modified:
    - internal/cli/root.go
    - go.mod
    - go.sum
decisions:
  - "Used fatih/color for terminal output with NO_COLOR env support"
  - "All output to stderr except generic client JSON (stdout reserved for MCP JSON-RPC)"
  - "Flat package structure in internal/cli/ rather than sub-package"
metrics:
  duration: 272s
  completed: "2026-04-21"
  tasks_completed: 2
  tasks_total: 2
  files_created: 3
  files_modified: 3
---

# Phase 34 Plan 01: Setup Command & Client Registrars Summary

Setup command with 6 client registrars using CLI-subprocess (claude-code, gemini-cli) and JSON merge-write (vscode, jetbrains, claude-desktop, generic) strategies, colored terminal output via fatih/color.

## Commits

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Create ClientRegistrar interface, RegistrationConfig, output helpers, and setup command scaffold | `2e17939d` | setup.go, setup_clients.go, setup_output.go |
| 2 | Implement all 6 client registrars with Register/Unregister and wire into root | `e000a32e` | setup_clients.go, root.go |

## What Was Built

### Setup Command (`internal/cli/setup.go`)
- Cobra subcommand `serena setup [client]` with flags: `--global`, `--uninstall`, `--skip-install`, `--dry-run`, `--output`
- Zero-arg discovery: `serena setup` lists all 6 clients with descriptions
- Client name validation against hardcoded allowlist (T-34-06)
- Binary path resolution via `os.Executable()` + `filepath.EvalSymlinks()`
- Orchestration: resolve binary, build config, dispatch to registrar

### Client Registrars (`internal/cli/setup_clients.go`)
- **ClaudeCodeRegistrar**: subprocess `claude mcp add-json` with project/user scope
- **GeminiCLIRegistrar**: subprocess `gemini mcp add` with project/user scope
- **VSCodeRegistrar**: writes `.vscode/mcp.json` with `"servers"` key (not `"mcpServers"`)
- **JetBrainsRegistrar**: writes `.junie/mcp/mcp.json` with `"mcpServers"` key
- **ClaudeDesktopRegistrar**: platform-specific path via `runtime.GOOS`, global-only, preserves existing keys
- **GenericRegistrar**: JSON to stdout (default) or `--output` file
- Shared helpers: `mergeJSONConfig()`, `removeFromJSONConfig()`, `serverConfigJSON()`

### Output Helpers (`internal/cli/setup_output.go`)
- `SetupPrinter` with `Success`, `Failure`, `Info`, `DryRunAction` methods
- Green checkmark, red cross, blue arrow, yellow dry-run prefix
- All output to stderr; NO_COLOR respected via fatih/color

### Root Command Integration (`internal/cli/root.go`)
- `rootCmd.AddCommand(newSetupCommand())` added
- `RunE: runRoot` preserved -- bare `serena` still enters stdio/forwarder mode

## Threat Mitigations Applied

- T-34-01: JSON marshal/unmarshal only (no string concatenation), dirs 0755, files 0644
- T-34-03: Config files written 0644, no secrets in MCP config
- T-34-05: removeFromJSONConfig preserves all keys except `"serena"` under the MCP servers key
- T-34-06: Client name validated against hardcoded 6-client allowlist

## Deviations from Plan

None -- plan executed exactly as written.

## Decisions Made

1. **Flat cli package**: Kept all setup files in `internal/cli/` rather than `internal/cli/setup/` sub-package, matching the plan's file layout
2. **stderr for all output**: All printer output goes to stderr; only GenericRegistrar writes to stdout for piping
3. **fatih/color v1.19.0**: Added as new dependency for colored terminal output with NO_COLOR support

## Verification Results

1. `go build ./cmd/serena` -- PASS
2. `go vet ./internal/cli/...` -- PASS
3. Bare `serena` enters stdio/forwarder mode -- PASS (no regression)
4. `serena setup` lists all 6 clients -- PASS
5. `serena setup --help` shows all 5 flags -- PASS
6. `serena setup --dry-run claude-code` shows planned action -- PASS
7. `serena setup --dry-run generic` shows dry-run message -- PASS
8. `serena setup invalid-client` exits non-zero with error -- PASS
