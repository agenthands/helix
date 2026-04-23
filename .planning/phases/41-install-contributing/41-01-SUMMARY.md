---
phase: 41-install-contributing
plan: 01
subsystem: documentation
tags: [install, setup-cli, mcp-config, documentation]
dependency_graph:
  requires: []
  provides: [INSTALL.md]
  affects: []
tech_stack:
  added: []
  patterns: [setup-cli-primary, collapsed-manual-config, details-summary]
key_files:
  created: []
  modified: [INSTALL.md]
decisions:
  - "Removed '/ Cursor' from VS Code setup comment to fully satisfy D-04 stale agent removal"
  - "Used CLI commands (not JSON) for Claude Code and Gemini CLI manual config per Research pitfall 4"
  - "VS Code config uses 'servers' key with 'type: stdio' per verified setup_clients.go source"
metrics:
  duration: 74s
  completed: "2026-04-23T14:31:22Z"
---

# Phase 41 Plan 01: INSTALL.md Rewrite Summary

Rewrote INSTALL.md to lead with `serena setup <client>` as the one-command installation path, with manual configs in a collapsed secondary section aligned to the 6 clients in `clientRegistry()`.

## What Changed

**INSTALL.md** was restructured from a flat per-agent manual config format to:

1. **Prerequisites** -- `go install` with build-from-source alternative, PATH verification
2. **Quick Start** -- `serena setup` commands for all 5 named clients, with flag documentation
3. **Manual Configuration** -- collapsed `<details>` section with accurate configs for all 6 clients
4. **HTTP Mode** -- separate section for shared daemon / multi-agent scenarios
5. **Verify Installation** -- references both `get_health` and `onboard_project`
6. **Next Steps** -- links to USAGE.md and README.md
7. **Legacy Python** -- one-liner at bottom

**Removed:** Codex, OpenCode, Cursor, Antigravity agent sections (stale, not in `clientRegistry()`).

**Fixed:** VS Code config now correctly uses `"servers"` key (not `"mcpServers"`) with required `"type": "stdio"` field. Claude Code and Gemini CLI sections show their native CLI commands instead of inapplicable JSON configs.

## Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Rewrite INSTALL.md with setup CLI as primary path | b3932958 | INSTALL.md |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed "/ Cursor" from VS Code comment**
- **Found during:** Task 1 verification
- **Issue:** `serena setup vscode # VS Code / Cursor` triggered the stale-entry grep check for "cursor" (D-04 requires removal)
- **Fix:** Changed comment to `# VS Code` to satisfy acceptance criteria. The README.md still shows "/ Cursor" -- that is out of scope for this plan.
- **Files modified:** INSTALL.md
- **Commit:** b3932958

## Verification

- All acceptance criteria pass (setup commands for 5 named clients, "servers" key, get_health, onboard_project, HTTP Mode, Manual configuration, Legacy Python)
- No stale agent entries (codex, opencode, cursor, antigravity)
- `go vet ./...` clean (pre-existing tree-sitter Swift C warning is not a Go vet error)

## Self-Check: PASSED

- [x] INSTALL.md exists and contains expected content
- [x] Commit b3932958 exists in git log
