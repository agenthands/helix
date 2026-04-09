---
phase: "06"
plan: "04"
subsystem: kernel/lspool, kernel/symbols, daemon
tags: [bug-fix, integration-tests, gopls, language-detection]
dependency_graph:
  requires: ["06-01", "06-02", "06-03"]
  provides: ["working-ls-backed-tools", "strict-integration-tests"]
  affects: ["internal/kernel/workspace.go", "internal/daemon/daemon.go", "internal/kernel/lspool/pool.go", "internal/kernel/symbols/tools.go"]
tech_stack:
  added: []
  patterns: ["lifecycle-context-for-workers", "workspace-root-uri-resolution"]
key_files:
  created: []
  modified:
    - internal/kernel/workspace.go
    - internal/daemon/daemon.go
    - internal/kernel/lspool/pool.go
    - internal/kernel/symbols/tools.go
    - test/integration/symbols_test.go
    - test/integration/diag_test.go
decisions:
  - "Worker processes use pool lifecycle context instead of request context to survive across tool calls"
  - "pathToURI resolves relative paths against workspace root, matching diag tools' fileURI pattern"
metrics:
  duration: "7m 45s"
  completed: "2026-04-08T20:25:33Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 6
---

# Phase 06 Plan 04: Gap Closure - Language Field Bug Fix Summary

Fixed three interconnected bugs preventing LS-backed tools from returning real gopls data, then updated all symbol and diagnostic integration tests to use strict assertions.

## Completed Tasks

| Task | Name | Commits | Key Files |
|------|------|---------|-----------|
| 1 | Fix Language field population | b0b07cd2, c47c98aa | workspace.go, daemon.go, pool.go, symbols/tools.go |
| 2 | Update tests to strict assertions | 98d07295 | symbols_test.go, diag_test.go |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Worker process killed when request context completes**
- **Found during:** Task 2 verification (symbol tools returning "connection closed: reading header: EOF")
- **Issue:** `spawnWorkerLocked` passed the request context to `worker.Start()`, which fed into `watchContext`. When the MCP tool call completed, the context was cancelled, and `watchContext` sent SIGTERM to gopls.
- **Fix:** Added `runCtx` field to Pool, stored from `Pool.Run(ctx)`. `spawnWorkerLocked` now uses `p.runCtx` for worker startup so LS processes outlive individual requests.
- **Files modified:** internal/kernel/lspool/pool.go
- **Commit:** c47c98aa

**2. [Rule 1 - Bug] pathToURI produced invalid URIs for relative paths**
- **Found during:** Task 2 verification (tools returning "no package metadata for file file:///main.go")
- **Issue:** `pathToURI("main.go")` produced `file://main.go` instead of `file:///tmp/.../main.go`. The diag tools had a correct `fileURI(root, path)` implementation but the symbol tools did not.
- **Fix:** Updated `pathToURI` to accept a `root` parameter and resolve relative paths. Updated all 8 callers to pass `rt.Key().RepoRoot`.
- **Files modified:** internal/kernel/symbols/tools.go
- **Commit:** c47c98aa

## Results

All 9 symbol retrieval tools return real gopls data:
- go_to_definition: returns file:line:col pointing to Helper definition
- find_references: returns 3 references (definition + 2 call sites)
- get_hover_info: returns Go doc with function signature
- search_symbols: returns DemoStruct and related symbols
- get_symbol_overview: returns all 5 file-level symbols
- find_implementations: returns SimpleGreeter implementing Greeter
- get_call_hierarchy: returns Helper's incoming callers (main, UsingHelper)
- get_type_hierarchy: returns SimpleGreeter -> Greeter relationship
- analyze_blast_radius: returns 6 locations across 1 file with caller tree

All 3 diagnostic tools return real results:
- get_diagnostics: "no diagnostics" (clean file, correct behavior)
- get_code_actions: returns 4 gopls code actions
- format_code: "file is already formatted" (correct for clean fixture)

Codebase smoke tests also pass with strict assertions against Serena's own codebase.

## Known Issues (Out of Scope)

- `internal/kernel/edit/rename.go` has same `pathToURI` bug (not resolved; tracked in deferred-items.md)
- `TestHTTPSmoke_WithLS` fails with schema validation error for "scope" property (pre-existing, unrelated)

## Self-Check: PASSED
