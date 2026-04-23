---
phase: 40-usage-refresh
plan: 03
subsystem: documentation
tags: [docs, gap-closure, usage, troubleshooting]
dependency_graph:
  requires: ["40-01", "40-02"]
  provides: ["corrected-repomap-params", "corrected-fuzzy-edit-example", "rust-analyzer-troubleshooting"]
  affects: ["USAGE.md"]
tech_stack:
  added: []
  patterns: []
key_files:
  created: []
  modified: ["USAGE.md"]
decisions: []
metrics:
  duration: "63s"
  completed: "2026-04-23T14:08:41Z"
  tasks_completed: 2
  tasks_total: 2
---

# Phase 40 Plan 03: Gap Closure for Verification Fixes Summary

Fixed three documentation accuracy gaps in USAGE.md: corrected RepoMap parameter name from max_tokens to token_budget matching source code, replaced invalid positional-args fuzzy_edit example with correct named-parameter form, and added rust-analyzer rename troubleshooting entry with Symptom/Cause/Workaround pattern.

## Task Results

### Task 1: Fix RepoMap parameter name and fuzzy_edit code example

**Commit:** `f2c835bb`

- Changed `max_tokens` to `token_budget` in RepoMap parameter description (line 153)
- Changed `get_repo_map(max_tokens=4096)` to `get_repo_map(token_budget=4096)` in code example (line 159)
- Replaced positional-args `fuzzy_edit` example with named-parameter form using `path`, `search`, `replacement`

### Task 2: Add rust-analyzer rename troubleshooting entry

**Commit:** `036baae2`

- Added "rust-analyzer rename fails in fresh workspaces" troubleshooting entry after gopls entry
- Documents rust-analyzer v1.90 limitation with textDocument/rename in fresh workspaces
- Recommends `replace_symbol_body` (tree-sitter-based) as workaround
- Follows existing Symptom/Cause/Workaround pattern

## Verification Results

- `grep "max_tokens" USAGE.md` -- no matches in RepoMap section (confirmed)
- `grep "token_budget" USAGE.md` -- matches on lines 153 and 159 (confirmed)
- `grep "fuzzy_edit(path=" USAGE.md` -- matches corrected named-parameter example (confirmed)
- `grep "rust-analyzer rename" USAGE.md` -- matches new troubleshooting heading at line 531 (confirmed)
- `go vet ./...` -- passes (pre-existing C macro warning only, no Go errors)

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None.
