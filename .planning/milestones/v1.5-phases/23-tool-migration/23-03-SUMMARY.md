---
phase: 23-tool-migration
plan: "03"
subsystem: kernel-fileops
tags: [error-migration, serr, fileops]
dependency_graph:
  requires: [22-01]
  provides: [typed-errors-fileops]
  affects: [internal/kernel/fileops]
tech_stack:
  added: []
  patterns: [serr-typed-errors, errors-is-matching]
key_files:
  created: []
  modified:
    - internal/kernel/fileops/validate.go
    - internal/kernel/fileops/read.go
    - internal/kernel/fileops/write.go
    - internal/kernel/fileops/list.go
    - internal/kernel/fileops/replace.go
    - internal/kernel/fileops/find.go
    - internal/kernel/fileops/search.go
    - internal/kernel/fileops/tools.go
    - internal/kernel/fileops/fileops_test.go
decisions:
  - "Preserved fmt.Errorf sentinels for result limit reached in find.go and search.go as internal control flow"
  - "Added noWorkspaceError() helper in tools.go for DRY workspace-not-active checks"
  - "Added errors.Is-based assertions to tests plus new test cases for invalid regex"
metrics:
  duration: ~3min
  completed: "2026-04-15"
  tasks_completed: 2
  tasks_total: 2
---

# Phase 23 Plan 03: Fileops Error Migration Summary

Migrated all 31 MCP-facing error sites in internal/kernel/fileops/ from raw fmt.Errorf to typed serr constructors, preserving 2 internal sentinels for WalkDir control flow.

## What Was Done

### Task 1: Migrate implementation files (7 files, 31 error sites)

Converted all fmt.Errorf calls to serr.New/serr.Wrap with appropriate Kind values:

| File | Sites | Kinds Used |
|------|-------|------------|
| validate.go | 5 | NoWorkspace, Internal, InvalidArgs |
| read.go | 9 | NotFound, Internal, InvalidArgs |
| write.go | 8 | InvalidArgs, Internal |
| list.go | 4 | NotFound, Internal, InvalidArgs |
| replace.go | 2 | InvalidArgs, Internal |
| find.go | 1 | Internal (sentinel preserved) |
| search.go | 2 | InvalidArgs, Internal (sentinel preserved) |

### Task 2: Migrate tools.go and update tests

- Added `serr` import and `noWorkspaceError()` helper to tools.go for all 6 handler workspace checks
- Updated test assertions to use `errors.Is(err, serr.ErrXxx)` for typed error checking
- Added new test cases: `TestValidatePathEmptyRoot`, `TestSearchPatternInvalidRegex`, `TestReplaceInFileInvalidRegex`

## Deviations from Plan

None -- plan executed exactly as written.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 7dba81d8 | Migrate 7 implementation files to typed serr errors |
| 2 | 3091d125 | Migrate tools.go handlers and update test assertions |

## Verification

- `go vet ./internal/kernel/fileops/` -- passes
- `go test ./internal/kernel/fileops/ -count=1` -- passes (all existing + 3 new tests)
- `go build ./cmd/serena` -- succeeds
- `grep -rn 'fmt.Errorf' internal/kernel/fileops/` -- returns only 2 "result limit reached" sentinels
