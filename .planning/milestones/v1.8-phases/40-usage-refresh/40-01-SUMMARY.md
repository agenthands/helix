---
phase: 40-usage-refresh
plan: 01
subsystem: documentation
tags: [usage-guide, feature-guide, tutorials, v1.6, v1.7]
dependency_graph:
  requires: []
  provides: [feature-guide-section, updated-tutorial-1]
  affects: [USAGE.md]
tech_stack:
  added: []
  patterns: [concept-plus-example, section-heading-hierarchy]
key_files:
  created: []
  modified:
    - USAGE.md
decisions:
  - Feature Guide placed between Quick Tutorials and Profiles and Modes (per D-01)
  - Tree-sitter grammar list included in RepoMap subsection rather than standalone (per Claude discretion)
  - Hooks documented as sub-feature of Setup CLI (per A2 assumption)
metrics:
  duration: 205s
  completed: 2026-04-23
  tasks_completed: 1
  tasks_total: 1
  files_modified: 1
---

# Phase 40 Plan 01: Feature Guide and Tutorial Update Summary

Feature Guide section with 7 subsections documenting all v1.6 and v1.7 features, plus Tutorial 1 updated to use setup CLI instead of manual JSON config.

## What Was Done

### Task 1: Update Tutorial 1 and add Feature Guide section

**Commit:** `7cebf9bc` feat(40-01): add Feature Guide section and update Tutorial 1 setup CLI

**Changes:**

1. **Tutorial 1 Step 2 updated** -- Replaced manual JSON `mcpServers` config block with `serena setup claude-code` command. Kept HTTP mode as secondary alternative. Step title changed from "Configure your MCP client" to "Register with your MCP client".

2. **Feature Guide section inserted** (line 137) between Quick Tutorials and Profiles and Modes, containing 7 subsections:

   - **Fuzzy Editing** -- 4-strategy cascade (Exact, Whitespace, IndentFlex, Failed), ellipsis support, `fuzzy_edit` tool example
   - **RepoMap and Context** -- `get_repo_map` (default 4096 tokens) and `get_context` (default 2048 tokens), PageRank ranking, 23 tree-sitter languages listed
   - **Setup CLI** -- `serena setup <client>` with 6 supported clients, `--global`/`--dry-run` flags, Claude Code hooks (SessionStart, PreToolUse, Stop), `--no-hooks` and `--uninstall` flags
   - **Smart Errors** -- Levenshtein distance suggestions for misspelled parameters, automatic middleware behavior
   - **Progressive Descriptions** -- Two-tier system (brief in tools/list, full via `get_tool_help`)
   - **Lazy Workspace Init** -- Transparent activation on first `tools/call`, automatic middleware
   - **Health Monitoring** -- `get_health` tool with verbose mode, cross-reference to Configuration Reference

**Files modified:** `USAGE.md` (85 insertions, 12 deletions, 771 total lines)

## Deviations from Plan

None -- plan executed exactly as written.

## Verification Results

All acceptance criteria verified:

- USAGE.md contains exactly 1 `## Feature Guide` heading (line 137)
- All 7 subsections present with correct headings (lines 139-204)
- "4-strategy cascade" present in Fuzzy Editing subsection
- `get_repo_map` and `get_context` present in RepoMap subsection
- 23 tree-sitter languages listed in RepoMap subsection
- `serena setup` and `SessionStart` present in Setup CLI subsection
- `Levenshtein` present in Smart Errors subsection
- `get_tool_help` present in Progressive Descriptions subsection
- `first tools/call` present in Lazy Workspace Init subsection
- `get_health` present in Health Monitoring subsection
- Tutorial 1 Step 2 contains `serena setup claude-code`
- No `mcpServers` JSON block in Tutorial 1
- Feature Guide (line 137) appears after ci-bot line (135) and before Profiles and Modes (217)
- Each subsection has at least one code example

## Self-Check: PASSED

- [x] USAGE.md exists and contains 771 lines
- [x] Commit 7cebf9bc exists in git log
- [x] No unexpected file deletions
