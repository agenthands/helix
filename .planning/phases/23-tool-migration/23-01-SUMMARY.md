---
phase: 23-tool-migration
plan: 01
subsystem: kernel/symbols
tags: [error-migration, typed-errors, symbols]
dependency_graph:
  requires: [22-01]
  provides: [typed-errors-symbols]
  affects: [internal/kernel/symbols/]
tech_stack:
  added: []
  patterns: [serr.Wrap, serr.New]
key_files:
  created: []
  modified:
    - internal/kernel/symbols/retrieval.go
    - internal/kernel/symbols/hierarchy.go
    - internal/kernel/symbols/overview.go
    - internal/kernel/symbols/search.go
    - internal/kernel/symbols/tools.go
decisions:
  - Used serr.Internal for all LSP call wrapper errors (semantic: LSP failures are internal)
  - Used serr.NoWorkspace for acquireLease (semantic: missing prerequisite)
  - Kept fmt import in retrieval.go (fmt.Sprintf in extractHoverContent, SymbolKindName) and tools.go (fmt.Sprintf in formatLocations, formatOutline, formatHierarchy, formatBlastRadius)
  - Removed fmt import from hierarchy.go, overview.go, search.go (no remaining fmt usage)
  - Handler operation errors use err.Error() since implementation functions now return typed serr.Error
metrics:
  duration: 181s
  completed: 2026-04-15T11:42:39Z
  tasks_completed: 2
  tasks_total: 2
  files_modified: 5
---

# Phase 23 Plan 01: Symbol Retrieval Tools Error Migration Summary

Migrated all 13 error sites in the symbols package from raw fmt.Errorf to typed serr.Wrap errors with correct Kind classification (Internal for LSP failures, NoWorkspace for missing workspace)

## Task Results

| Task | Name | Commit | Files Modified |
|------|------|--------|----------------|
| 1 | Migrate symbols implementation files | 445e1b05 | retrieval.go, hierarchy.go, overview.go, search.go |
| 2 | Migrate symbols/tools.go acquireLease and handlers | 0007a5a9 | tools.go |

## What Changed

### Implementation Files (Task 1)
- **retrieval.go**: 4 LSP call errors (definition, references, hover, implementation) migrated to `serr.Wrap(serr.Internal, ...)`
- **hierarchy.go**: 6 LSP call errors (prepare call/type hierarchy, incoming/outgoing calls, subtypes, supertypes) migrated to `serr.Wrap(serr.Internal, ...)`
- **overview.go**: 1 LSP call error (document symbol) migrated to `serr.Wrap(serr.Internal, ...)`
- **search.go**: 1 LSP call error (workspace symbol) migrated to `serr.Wrap(serr.Internal, ...)`

### Handler Layer (Task 2)
- **acquireLease**: 1 error migrated to `serr.Wrap(serr.NoWorkspace, "workspace not activated", err)`
- **9 acquire session errors**: All migrated to `serr.Wrap(serr.Internal, "acquire session", err).Error()`
- **9 operation result errors**: Simplified to `err.Error()` since implementation functions now return typed `*serr.Error`

### Import Changes
- Added `serr "github.com/postfix/serena/internal/errors"` to all 5 files
- Removed `"fmt"` from hierarchy.go, overview.go, search.go (no remaining usage)
- Kept `"fmt"` in retrieval.go and tools.go (still used by Sprintf formatting helpers)

## Verification

- `go vet ./internal/kernel/symbols/` -- passed
- `go test ./internal/kernel/symbols/ -count=1` -- passed
- `go build ./cmd/serena` -- succeeded
- `grep -rn 'fmt.Errorf' internal/kernel/symbols/` -- zero matches

## Deviations from Plan

None -- plan executed exactly as written.

## Self-Check: PASSED
