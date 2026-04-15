---
phase: 24-validation-testing
plan: 01
subsystem: kernel
tags: [validation, typed-errors, input-validation, security]
dependency_graph:
  requires: [internal/errors]
  provides: [inline-validation-for-24-kernel-tools]
  affects: [symbols, edit, diag, fileops]
tech_stack:
  added: []
  patterns: [inline-validation-before-workspace-check, typed-error-builder-chain]
key_files:
  created:
    - internal/kernel/symbols/tools_test.go
    - internal/kernel/edit/tools_test.go
    - internal/kernel/diag/tools_test.go
    - internal/kernel/fileops/validate_test.go
  modified:
    - internal/kernel/symbols/tools.go
    - internal/kernel/edit/tools.go
    - internal/kernel/diag/tools.go
    - internal/kernel/fileops/tools.go
decisions:
  - "Empty-string validation at handler top, before acquireLease/workspace check"
  - "Zero-value line=0 and column=0 NOT rejected (valid 0-indexed LSP positions)"
  - "Diag tools migrated from hardcoded error strings to typed serr.NoWorkspace"
  - "verify_edit now checks wsKey.RepoRoot before accessing it"
metrics:
  duration: 286s
  completed: "2026-04-15T14:28:29Z"
  tasks_completed: 1
  tasks_total: 1
  files_created: 4
  files_modified: 4
---

# Phase 24 Plan 01: Kernel Tool Input Validation Summary

Inline empty-string validation for all 24 kernel tool handlers using typed serr.InvalidArgs errors, plus verify_edit workspace guard and diag legacy error migration.

## Task Completion

| Task | Name | Commit | Status |
|------|------|--------|--------|
| 1 (RED) | Add validation tests for all kernel tool packages | a37b2a86 | Done |
| 1 (GREEN) | Add inline validation to all 24 kernel tool handlers | 8bd267fb | Done |

## Changes Made

### symbols/tools.go (9 tools)
- Added `args.Path == ""` check at top of all 9 handlers (go_to_definition, find_references, get_symbol_overview, get_hover_info, find_implementations, get_call_hierarchy, get_type_hierarchy, analyze_blast_radius)
- Added `args.Query == ""` check for search_symbols
- No line/col validation (zero values are valid LSP positions)

### edit/tools.go (6 tools)
- replace_symbol_body: validates path, symbol_name, new_body (3 checks)
- insert_before_symbol: validates path, symbol_name, content (3 checks)
- insert_after_symbol: validates path, symbol_name, content (3 checks)
- rename_symbol: validates path, new_name (2 checks)
- safe_delete_symbol: validates path, symbol_name (2 checks)
- verify_edit: added workspace check (wsKey.RepoRoot == "") before path validation

### diag/tools.go (3 tools)
- Replaced legacy `"no active workspace - activate a project first"` with typed `serr.New(serr.NoWorkspace, "no active workspace")` in all 3 handlers
- Added `args.Path == ""` validation after workspace check
- Added `serr` import

### fileops/tools.go (6 tools)
- read_file, create_file, list_directory: validates path
- find_files, search_in_files: validates pattern
- replace_in_file: validates both path and pattern

### Test Files (4 created)
- symbols/tools_test.go: error format tests for 9 tools + zero line/col validity assertion
- edit/tools_test.go: detectLang edge cases + error format for 6 edit tools + verify_edit workspace check
- diag/tools_test.go: typed NoWorkspace error format + InvalidArgs for 3 diag tools
- fileops/validate_test.go: ValidatePath empty path + error format for 6 fileops tools

## Decisions Made

1. **Validation before workspace check**: Empty-string checks execute at handler entry, before acquireLease or workspace root access, so agents get immediate InvalidArgs errors
2. **Zero values allowed**: line=0 and column=0 are valid 0-indexed LSP positions and are NOT rejected
3. **Typed error migration for diag**: Replaced 3 hardcoded error strings with typed serr.NoWorkspace errors
4. **verify_edit workspace guard**: Added wsKey.RepoRoot check to prevent accessing empty workspace key (previously unguarded)

## Deviations from Plan

None - plan executed exactly as written.

## Verification

```
go build ./cmd/serena     -> OK
go vet ./internal/kernel/... -> OK (clean)
go test ./internal/kernel/... -count=1 -> OK (all pass)
```

Validation block counts confirmed by grep:
- symbols/tools.go: 9 InvalidArgs checks
- edit/tools.go: 14 InvalidArgs checks + 1 NoWorkspace check
- diag/tools.go: 3 InvalidArgs checks + 3 NoWorkspace checks
- fileops/tools.go: 7 InvalidArgs checks

## Threat Mitigations

| Threat ID | Status | Implementation |
|-----------|--------|----------------|
| T-24-01 | Mitigated | All 24 handlers validate empty-string required fields at entry |
| T-24-02 | Mitigated | verify_edit checks wsKey.RepoRoot before access |
| T-24-03 | Mitigated | fileops tools check empty path/pattern before ValidatePath |
| T-24-04 | Accepted | Error messages contain field names only (already in MCP schemas) |

## Self-Check: PASSED

All 8 files verified present. Both commits (a37b2a86, 8bd267fb) verified in git log.
