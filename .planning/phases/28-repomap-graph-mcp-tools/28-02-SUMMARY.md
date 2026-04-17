---
phase: 28-repomap-graph-mcp-tools
plan: 02
subsystem: repomap
tags: [tree-rendering, token-budgeting, binary-search]
dependency_graph:
  requires: [28-01]
  provides: [TreeRenderer, NewTreeRenderer, RenderBudgeted, EstimateTokens, langFromExt]
  affects: [28-03]
tech_stack:
  added: []
  patterns: [binary-search-budget-fitting, tree-structured-output, chars-div-4-estimation]
key_files:
  created:
    - internal/repomap/render.go
    - internal/repomap/render_test.go
  modified: []
decisions:
  - "Tree rendering sorts by directory structure (dirs first, alphabetical), not by rank order -- rank order controls which files are included via binary search"
  - "15% tolerance on budget fitting per aider pattern allows slight over-budget for better coverage"
metrics:
  duration: 145s
  completed: "2026-04-17T13:50:39Z"
  tasks_completed: 1
  tasks_total: 1
  files_created: 2
  files_modified: 0
---

# Phase 28 Plan 02: TreeRenderer with Binary Search Token Budgeting Summary

Token-budgeted tree rendering with binary search to maximize file coverage within a specified token budget.

## One-liner

TreeRenderer with binary search on file count, chars/4 token estimation, directory-nested tree output with elided symbols, and minimum 1-file guarantee.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Failing tests for TreeRenderer | 5dd4298a | render_test.go |
| 1 (GREEN) | TreeRenderer implementation | 5e8662a5 | render.go, render_test.go |

## Implementation Details

### TreeRenderer (render.go)
- `TreeRenderer` struct with `elider *ElisionRenderer`, `cache *TagCache`, `rootDir string`
- `NewTreeRenderer(elider, cache, rootDir)` constructor
- `EstimateTokens(s string) int` -- package-level, returns `len(s)/4` per D-12
- `RenderBudgeted(ranked []RankedFile, budget int) string` -- binary search on file count:
  - lower=1, upper=len(ranked), tracks bestOutput/bestTokens
  - 15% tolerance (`0.15`) per aider pattern
  - Guaranteed minimum 1 file regardless of budget
- `renderTree(ranked []RankedFile) string` -- builds directory tree from relative paths, renders with 2-space indentation per level, elided symbols indented 4 spaces under filenames
- `langFromExt(path string) string` -- maps 11 file extensions to language identifiers
- `renderFileContent(filePath string) string` -- reads file, loads tags from cache, renders via ElisionRenderer

### Test Coverage (render_test.go)
- `TestEstimateTokens` -- chars/4 estimation with various inputs
- `TestRenderBudgeted_AllFilesFit` -- large budget includes all 3 test files
- `TestRenderBudgeted_MinOneFile` -- very small budget still includes 1 file
- `TestRenderBudgeted_BudgetFitting` -- output within budget or 15% tolerance
- `TestRenderBudgeted_EmptyRankedList` -- nil and empty input returns empty
- `TestRenderTree_TreeStructure` -- directory nesting verified
- `TestRenderBudgeted_RankOrder` -- highest-ranked file included with tight budget
- `TestLangFromExt` -- 12 test cases covering all mapped extensions + unknown

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Rank order test assumption corrected**
- **Found during:** Task 1 (TDD GREEN phase)
- **Issue:** Original test assumed higher-ranked files appear first in tree output, but tree rendering naturally sorts by directory structure (directories first, alphabetical). Rank order controls which files are *included* via binary search, not their position in the tree.
- **Fix:** Changed test to verify all files present with large budget and highest-ranked file present with tight budget.
- **Files modified:** render_test.go
- **Commit:** 5e8662a5

## Known Stubs

None -- all functionality is fully implemented and tested.

## TDD Gate Compliance

- RED gate: 5dd4298a (test commit -- compilation fails, tests undefined)
- GREEN gate: 5e8662a5 (implementation + test fix, all tests pass)
