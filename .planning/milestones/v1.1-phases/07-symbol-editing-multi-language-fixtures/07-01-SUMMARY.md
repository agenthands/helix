---
phase: 07-symbol-editing-multi-language-fixtures
plan: 01
subsystem: test/integration, internal/kernel/edit
tags: [integration-tests, edit-tools, gopls, bug-fixes]
dependency_graph:
  requires: [phase-06 harness]
  provides: [edit-integration-tests, requireLS-helper]
  affects: [internal/kernel/edit, internal/kernel/lspool, internal/kernel/symbols]
tech_stack:
  added: []
  patterns: [two-phase-lease, hierarchical-document-symbols]
key_files:
  created:
    - test/integration/edit_test.go
  modified:
    - test/integration/helpers.go
    - test/integration/harness.go
    - testdata/fixtures/go/main.go
    - internal/kernel/edit/tools.go
    - internal/kernel/edit/planner.go
    - internal/kernel/edit/insert.go
    - internal/kernel/edit/delete.go
    - internal/kernel/edit/replace.go
    - internal/kernel/edit/rename.go
    - internal/kernel/lspool/worker.go
    - internal/kernel/symbols/overview.go
decisions:
  - Two-phase lease pattern for edit tools (clean for read, dirty for write)
  - HierarchicalDocumentSymbolSupport declared in client capabilities
  - WithPlan variants for edit operations to support pre-computed plans
metrics:
  duration: 26min
  completed: "2026-04-08T21:53:00Z"
  tasks: 2
  files: 12
---

# Phase 07 Plan 01: Go Edit Tool Integration Tests Summary

Integration tests for all 6 symbol editing MCP tools against Go fixture with gopls, plus critical bug fixes discovered during test development.

## Completed Tasks

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Add requireLS helper and UnusedFunc fixture | 5d913eb7 | test/integration/helpers.go, testdata/fixtures/go/main.go |
| 2 | Create edit tool integration tests | 6f1b9fe5 | test/integration/edit_test.go + 9 production files |

## What Was Built

### Test File: test/integration/edit_test.go

5 test functions covering all 6 edit tools:

1. **TestEdit_ReplaceBody** (EDIT-01): Round-trip read -> replace -> re-read confirming "Modified Helper" present
2. **TestEdit_InsertBeforeAfter** (EDIT-02): Two sub-tests for insert_before and insert_after with position verification
3. **TestEdit_RenameCrossFile** (EDIT-03): Renames Helper to RenamedHelper, verifies cross-file reference update in UsingHelper
4. **TestEdit_SafeDelete** (EDIT-04): Blocked delete returns reference info for Helper; unreferenced UnusedFunc deletes successfully
5. **TestEdit_VerifyEdit**: verify_edit returns clean diagnostics after valid body replacement

### Helper: requireLS

Generic `requireLS(t, binary)` function in helpers.go for multi-language test skipping (Plans 02/03).

### Fixture Update

Added `UnusedFunc()` to `testdata/fixtures/go/main.go` for safe_delete unreferenced symbol testing.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Edit tools did not resolve relative paths against workspace root**
- **Found during:** Task 2
- **Issue:** `filePathToURI` in edit/tools.go prepended `file://` without joining workspace root, producing invalid URIs like `file://main.go`
- **Fix:** Changed `filePathToURI` to accept root parameter and resolve relative paths (matching symbol tools' `pathToURI`)
- **Files modified:** internal/kernel/edit/tools.go
- **Commit:** 6f1b9fe5

**2. [Rule 1 - Bug] Worker initialization declared empty client capabilities**
- **Found during:** Task 2
- **Issue:** Worker `InitializeParams.Capabilities` was `gen.ClientCapabilities{}` (empty). Without `HierarchicalDocumentSymbolSupport=true`, gopls returned `SymbolInformation[]` format which decoded to `DocumentSymbol[]` with zero-valued ranges. This caused all edit operations to compute wrong byte offsets.
- **Fix:** Set `TextDocument.DocumentSymbol.HierarchicalDocumentSymbolSupport = true` in worker init capabilities
- **Files modified:** internal/kernel/lspool/worker.go
- **Commit:** 6f1b9fe5

**3. [Rule 1 - Bug] Rename only handled WorkspaceEdit.Changes, not DocumentChanges**
- **Found during:** Task 2
- **Issue:** gopls returns `DocumentChanges` (array of `TextDocumentEdit`) not `Changes` (map). Rename appeared to succeed but didn't modify files.
- **Fix:** Added `DocumentChanges` handling with JSON marshal/unmarshal for union type extraction
- **Files modified:** internal/kernel/edit/rename.go
- **Commit:** 6f1b9fe5

**4. [Rule 1 - Bug] SafeDelete used full Range start for reference lookup instead of SelectionRange**
- **Found during:** Task 2
- **Issue:** `plan.Range.Start` pointed to doc comment, not identifier. gopls returned "no identifier found" for references.
- **Fix:** Added `SelectionRange` to `SymbolOutline` and `EditPlan`, used it for reference lookup position
- **Files modified:** internal/kernel/symbols/overview.go, internal/kernel/edit/planner.go, internal/kernel/edit/delete.go
- **Commit:** 6f1b9fe5

**5. [Rule 3 - Blocking] Edit tools used fresh dirty workers with unindexed workspace**
- **Found during:** Task 2
- **Issue:** Edit tool handlers acquired dirty (new) workers for ALL operations including read-only PlanEdit. Fresh workers hadn't indexed the workspace, returning zero-valued ranges.
- **Fix:** Refactored to two-phase approach: acquire clean (shared, warm) lease for PlanEdit, dirty lease only for the actual file mutation. Added `WithPlan` variants for all edit operations.
- **Files modified:** internal/kernel/edit/tools.go, insert.go, delete.go, replace.go
- **Commit:** 6f1b9fe5

## Decisions Made

1. **Two-phase lease for edit tools**: Clean lease for planning (documentSymbol), dirty lease for mutation (file writes). This ensures edit planning uses warm, indexed workers.
2. **WithPlan API pattern**: Edit operations expose both the original function (calls PlanEdit internally) and a `WithPlan` variant that accepts a pre-computed plan. Tool handlers use the latter for two-phase execution.
3. **MaxWorkers increased to 4**: Edit tools need multiple concurrent dirty workers; 2 was insufficient.

## Known Stubs

None.

## Self-Check: PASSED

- All 6 key files verified present on disk
- Both task commits (5d913eb7, 6f1b9fe5) verified in git log
- All 5 TestEdit_* functions pass with `go test -tags integration -run TestEdit_ -timeout 120s`
- All existing TestSymbols_* tests still pass
- `go vet ./...` passes clean
- `go test ./...` (non-integration) passes clean
