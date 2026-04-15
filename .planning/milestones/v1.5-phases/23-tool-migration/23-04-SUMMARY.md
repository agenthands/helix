---
phase: 23-tool-migration
plan: "04"
subsystem: kernel/diag
tags: [error-migration, typed-errors, diagnostics]
dependency_graph:
  requires: [22-01]
  provides: [typed-errors-diag]
  affects: [internal/kernel/diag]
tech_stack:
  added: []
  patterns: [serr-typed-errors]
key_files:
  created: []
  modified:
    - internal/kernel/diag/actions.go
    - internal/kernel/diag/format.go
decisions:
  - "Used serr.Unsupported for missing workspace edit (semantic: action not applicable)"
  - "Used serr.Internal for all LSP request failures and I/O errors"
  - "Added .WithDetail(action.Title) for code action context"
metrics:
  duration: "<1 min"
  completed: "2026-04-15"
  tasks_completed: 1
  tasks_total: 1
---

# Phase 23 Plan 04: Diagnostic Tools Error Migration Summary

Migrated all 8 error sites in the diag package (actions.go, format.go) from raw fmt.Errorf to typed serr constructors with proper Kind classification.

## Task Completion

| Task | Name | Commit | Files Modified |
|------|------|--------|----------------|
| 1 | Migrate diag implementation files | 802eeec5 | actions.go, format.go |

## Changes Made

### actions.go (4 error sites)
- `codeAction request: %w` -> `serr.Wrap(serr.Internal, "code action request", err)`
- `code action has no workspace edit` -> `serr.New(serr.Unsupported, ...).WithDetail(action.Title)`
- `applying workspace edit: %w` -> `serr.Wrap(serr.Internal, "applying workspace edit", err)`
- `workspace edit was not applied` -> `serr.New(serr.Internal, "workspace edit was not applied by the server")`

### format.go (4 error sites)
- `formatting request: %w` -> `serr.Wrap(serr.Internal, "formatting request", err)`
- `reading file: %w` -> `serr.Wrap(serr.Internal, "reading file", err)`
- `writing temp file: %w` -> `serr.Wrap(serr.Internal, "writing temp file", err)`
- `renaming temp file: %w` -> `serr.Wrap(serr.Internal, "renaming temp file", err)`

### skill_adapter.go
Reviewed -- no error sites found. Contains only skill registration boilerplate.

## Verification Results

- `go vet ./internal/kernel/diag/` -- passed
- `go test ./internal/kernel/diag/ -count=1` -- passed
- `go build ./cmd/serena` -- succeeded
- `grep -rn 'fmt.Errorf' internal/kernel/diag/` -- zero matches

## Deviations from Plan

None -- plan executed exactly as written.

## Known Stubs

None.
