---
phase: 38-progressive-descriptions-lazy-init
plan: 02
subsystem: tool-descriptions
tags: [progressive-descriptions, brief-description, help-text, tool-registry]
dependency_graph:
  requires: [38-01]
  provides: [brief-descriptions-populated, help-text-populated]
  affects: [tools-list-response, get-tool-help-response]
tech_stack:
  added: []
  patterns: [co-located-help-constants, brief-description-metadata]
key_files:
  created: []
  modified:
    - internal/kernel/symbols/tools.go
    - internal/kernel/edit/tools.go
    - internal/kernel/fileops/tools.go
    - internal/kernel/diag/tools.go
    - internal/kernel/health/tools.go
    - internal/skill/memory/skill.go
    - internal/skill/workflow/skill.go
    - internal/skill/repomap/skill.go
decisions:
  - Included fuzzy_edit in fileops tools (7 total) since it has its own registration
  - Used co-located const help strings near registration functions per D-05
  - Memory skill tools match actual tool names (rename_memory, edit_memory) rather than plan table names
metrics:
  duration: 500s
  completed: 2026-04-22T19:12:25Z
  tasks_completed: 2
  tasks_total: 2
  test_count: 0
  files_changed: 8
---

# Phase 38 Plan 02: Add BriefDescription and HelpText to All Tools Summary

Populated BriefDescription (under 15 words) and HelpText (usage examples and common patterns) for all 37 tool registration sites across kernel and skill packages, enabling progressive disclosure in tools/list and detailed help via get_tool_help.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | fbfefe95 | Add BriefDescription and HelpText to 26 kernel tools |
| 2 | fda97237 | Add BriefDescription and HelpText to 11 skill tools |

## Task Details

### Task 1: Kernel tools (symbols, edit, fileops, diag, health)

Added co-located help text constants and BriefDescription/HelpText fields to Registry().Register() calls for all kernel tool packages:

- **symbols/tools.go (9 tools):** go_to_definition, find_references, get_symbol_overview, search_symbols, get_hover_info, find_implementations, get_call_hierarchy, get_type_hierarchy, analyze_blast_radius
- **edit/tools.go (6 tools):** replace_symbol_body, insert_before_symbol, insert_after_symbol, rename_symbol, safe_delete_symbol, verify_edit
- **fileops/tools.go (7 tools):** read_file, create_file, list_directory, find_files, search_in_files, replace_in_file, fuzzy_edit
- **diag/tools.go (3 tools):** get_diagnostics, get_code_actions, format_code
- **health/tools.go (1 tool):** get_health

### Task 2: Skill tools (memory, workflow, repomap)

Added co-located help text constants and BriefDescription/HelpText fields to ToolDef returns in Tools() methods:

- **memory/skill.go (7 tools):** write_memory, read_memory, list_memories, search_memories, rename_memory, edit_memory, delete_memory
- **workflow/skill.go (2 tools):** onboard_project, prepare_for_new_conversation
- **repomap/skill.go (2 tools):** get_repo_map, get_context

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing] Included fuzzy_edit tool not in plan table**
- Found during: Task 1
- Issue: Plan listed 6 fileops tools but fuzzy_edit is a 7th tool registered in fileops/tools.go
- Fix: Added BriefDescription and HelpText for fuzzy_edit alongside the other 6
- Files modified: internal/kernel/fileops/tools.go
- Commit: fbfefe95

**2. [Rule 2 - Missing] Memory skill has rename_memory and edit_memory instead of plan table names**
- Found during: Task 2
- Issue: Plan listed read_global_memory and list_global_memories but actual tools are rename_memory and edit_memory
- Fix: Added descriptions matching actual tool names in the codebase
- Files modified: internal/skill/memory/skill.go
- Commit: fda97237

## Verification

- `go build ./...` exits 0
- `go vet ./internal/kernel/... ./internal/skill/...` clean (only pre-existing tree-sitter C warnings)
- 37 BriefDescription entries across 8 modified files
- Every help text contains `## Usage Examples` and `## Common Patterns`
- Every brief description is under 15 words

## Self-Check: PASSED
