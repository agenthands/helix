---
phase: 23-tool-migration
plan: 02
subsystem: edit-tools
tags: [error-migration, serr, edit-package]
dependency_graph:
  requires: [22-01]
  provides: [typed-errors-edit-package]
  affects: [internal/kernel/edit/]
tech_stack:
  added: []
  patterns: [serr-typed-errors, kind-based-classification]
key_files:
  created: []
  modified:
    - internal/kernel/edit/treesitter.go
    - internal/kernel/edit/planner.go
    - internal/kernel/edit/delete.go
    - internal/kernel/edit/insert.go
    - internal/kernel/edit/rename.go
    - internal/kernel/edit/replace.go
    - internal/kernel/edit/tools.go
decisions:
  - "Operation result errors from implementation files pass through to tools.go handlers directly (err.Error()) since they already carry typed serr information"
  - "replace.go retains fmt import for non-error Sprintf usage in byte range validation message"
metrics:
  duration: 292s
  completed: 2026-04-15
  tasks: 2/2
---

# Phase 23 Plan 02: Edit Tools Error Migration Summary

Migrated all ~30 error sites in 7 edit package files from raw fmt.Errorf to typed serr constructors with semantic Kind classification.

## One-liner

All 6 symbol editing tools plus tree-sitter and planner use typed serr errors with NotFound, Unsupported, NoWorkspace, and Internal kinds.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Migrate edit implementation files | 0dddaca9 | treesitter.go, planner.go, delete.go, insert.go, rename.go, replace.go |
| 2 | Migrate edit/tools.go handler-layer errors | 7bb92a17 | tools.go |

## Changes Made

### Task 1: Implementation Files (30 error sites)
- **treesitter.go** (5 sites): Unsupported for unknown languages, Internal for parse failures, NotFound for missing declarations/body fields. WithDetail carries lang/symbolName.
- **planner.go** (2 sites): Internal for symbol overview failures, NotFound for missing symbols.
- **delete.go** (5 sites): Internal wraps for plan/references/read/write/didChange.
- **insert.go** (8 sites): Internal wraps for plan/read/write/didChange across InsertBefore and InsertAfter.
- **rename.go** (5 sites): Internal wraps for rename/apply-edits/read/write. WithDetail carries file URIs.
- **replace.go** (5 sites): Internal wraps for plan/read/validate/write/didChange.
- Removed unused `fmt` imports from treesitter.go, planner.go, delete.go, insert.go, rename.go.

### Task 2: Handler Layer (tools.go, ~19 error sites)
- 5 workspace-not-activated errors use `serr.Wrap(serr.NoWorkspace, ...)`.
- 9 acquire-session errors use `serr.Wrap(serr.Internal, ...)`.
- 5 operation result errors pass through typed errors from implementation layer directly via `err.Error()`.
- Retained `fmt` import for non-error Sprintf usage in appendVerifyInfo and text formatting.

## Verification Results

- `go vet ./internal/kernel/edit/` -- PASSED
- `go test ./internal/kernel/edit/ -count=1` -- PASSED
- `go build ./cmd/serena` -- PASSED
- `grep -rn 'fmt.Errorf' internal/kernel/edit/` -- zero matches

## Deviations from Plan

None -- plan executed exactly as written.

## Known Stubs

None.
