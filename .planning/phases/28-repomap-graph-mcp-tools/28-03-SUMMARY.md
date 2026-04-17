---
phase: 28-repomap-graph-mcp-tools
plan: 03
subsystem: skill/repomap
tags: [mcp-tools, skill, pagerank, token-budget, path-traversal]
dependency_graph:
  requires: [28-01]
  provides: [RepoMapSkill, get_repo_map, get_context]
  affects: []
tech_stack:
  added: []
  patterns: [caddy-init-registration, tool-dispatch-switch, binary-search-budget, lazy-graph-build]
key_files:
  created:
    - internal/skill/repomap/skill.go
    - internal/skill/repomap/skill_test.go
  modified:
    - internal/daemon/imports.go
decisions:
  - "Do NOT modify SkillDeps -- repomap skill creates own TagCache and GrammarRegistry in Init() from ProjectDir"
  - "Workspace root resolved via os.Getwd() at execution time, not injected via SkillDeps"
  - "Inline tree rendering with ElisionRenderer -- does not depend on render.go (Plan 02)"
metrics:
  duration: 188s
  completed: "2026-04-17T13:51:53Z"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 1
---

# Phase 28 Plan 03: RepoMapSkill MCP Tools Summary

RepoMapSkill registered via Caddy-style init() with get_repo_map (uniform PageRank) and get_context (personalized PageRank) MCP tools, both token-budgeted with path traversal prevention.

## One-liner

Two MCP tools exposing PageRank-ranked repository structure maps with binary-search token budgeting, path traversal prevention, and lazy graph caching via TagCache version dirty-flag.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Extend SkillDeps and create RepoMapSkill | 8ff51418 | skill.go, imports.go |
| 2 | RepoMapSkill tool dispatch and validation tests (TDD) | e887d015 | skill_test.go |

## Implementation Details

### RepoMapSkill (skill.go)
- `init()` registers `&RepoMapSkill{}` with skill system (Caddy-style)
- `Init(deps)` creates own TagCache, GrammarRegistry, and ElisionRenderer from `deps.ProjectDir`
- `Tools()` returns 2 ToolDef entries: `get_repo_map` and `get_context`
- `ExecuteTool()` dispatches via switch statement following memory skill pattern
- `ensureGraph()` lazy graph build with dirty-flag based on `TagCache.Version()`
- `renderBudgeted()` binary search on file count per D-13, chars/4 estimation per D-12
- `renderTree()` renders ranked files with elided symbols via ElisionRenderer
- Token budget: default 4096 (repo map) / 2048 (context), capped at 32768, min 64 (T-28-06)
- Path traversal: reject any file path containing `..` (T-28-05), logged at warn level
- `task_description` parameter accepted but reserved for future use (D-08)

### Daemon Wiring (imports.go)
- Blank import `_ "github.com/postfix/serena/internal/skill/repomap"` added in alphabetical order

### Design Decision: No SkillDeps Modification
Per plan's final decision, SkillDeps was NOT modified. The repomap skill creates its own dependencies:
- TagCache from `filepath.Join(deps.ProjectDir, "index", "tags.db")`
- GrammarRegistry via `treesitter.NewGrammarRegistry()`
- ElisionRenderer via `repomap.NewElisionRenderer(registry)`
- Workspace root via `os.Getwd()` fallback at execution time

## Test Coverage

- 9 tests total, all passing:
  - `TestRepoMapSkill_ExecuteTool_UnknownTool` -- unknown tool returns InvalidArgs
  - `TestRepoMapSkill_GetRepoMap_DefaultBudget` -- empty args uses default 4096
  - `TestRepoMapSkill_GetRepoMap_BudgetCap` -- excessive budget capped at 32768
  - `TestRepoMapSkill_GetContext_MissingFiles` -- missing files returns InvalidArgs
  - `TestRepoMapSkill_GetContext_EmptyFiles` -- empty files array returns InvalidArgs
  - `TestRepoMapSkill_GetContext_PathTraversal` -- `..` in path rejected
  - `TestRepoMapSkill_GetRepoMap_EmptyCache` -- empty cache returns "No files found"
  - `TestRepoMapSkill_GetContext_WithFiles` -- populated cache returns ranked output
  - `TestRepoMapSkill_Init` -- Init creates TagCache and ElisionRenderer

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Nil logger panic in path traversal validation**
- **Found during:** Task 2 (test execution)
- **Issue:** Test helper constructed RepoMapSkill with nil logger; `s.logger.Warn()` call in path traversal check panicked with nil pointer dereference
- **Fix:** Set `logger: slog.Default()` in test helper instead of nil
- **Files modified:** skill_test.go
- **Commit:** e887d015

### Plan Adjustments

**2. SkillDeps not modified (plan-specified)**
- Plan's final decision explicitly says "Do NOT modify SkillDeps" -- followed as written
- `internal/skill/skill.go` was NOT changed

**3. Inline rendering instead of TreeRenderer dependency**
- render.go (Plan 02) does not exist yet -- skill implements its own `renderBudgeted()` and `renderTree()` using ElisionRenderer directly
- This makes the skill fully functional without waiting for Plan 02

## Known Stubs

None -- all functionality is fully implemented and tested.

## TDD Gate Compliance

- Task 2 is marked `tdd="true"` but implementation existed from Task 1
- Tests were written and all passed immediately against existing implementation
- RED gate: tests written in skill_test.go
- GREEN gate: e887d015 (tests pass against Task 1 implementation)

## Self-Check: PASSED

All files exist, all commits verified, all tests pass, go vet clean.
