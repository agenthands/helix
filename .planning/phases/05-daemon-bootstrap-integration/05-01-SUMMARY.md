---
phase: 05-daemon-bootstrap-integration
plan: 01
subsystem: kernel
tags: [skill-adapters, pool-installer, contract-fixes]
dependency_graph:
  requires: []
  provides: [kernel-skill-adapters, installer-pool-wiring]
  affects: [05-02, 05-03]
tech_stack:
  added: []
  patterns: [caddy-style-init-registration, three-tier-resolution]
key_files:
  created:
    - internal/kernel/symbols/skill.go
    - internal/kernel/edit/skill.go
    - internal/kernel/fileops/skill.go
    - internal/kernel/diag/skill_adapter.go
  modified:
    - internal/kernel/lspool/pool.go
    - internal/kernel/lspool/pool_test.go
    - internal/kernel/kernel.go
decisions:
  - Skill adapters return nil RegisterFn since daemon owns MCP registration
  - Pool falls back to entry.Command when installer is nil for backward compatibility
metrics:
  duration: 3min
  completed: "2026-04-08T14:13:24Z"
---

# Phase 05 Plan 01: Contract Fixes Summary

Kernel tool skill adapters with three-tier installer wiring for pool LS resolution.

## What Was Done

### Task 1: Create kernel tool skill adapters and fix profile YAMLs

Created 4 thin ToolProvider adapters wrapping each kernel tool package:

- **SymbolRetrievalSkill** (symbols/skill.go): 9 ToolDefs -- go_to_definition, find_references, get_symbol_overview, search_symbols, get_hover_info, find_implementations, get_call_hierarchy, get_type_hierarchy, analyze_blast_radius
- **SymbolEditingSkill** (edit/skill.go): 6 ToolDefs -- replace_symbol_body, insert_before_symbol, insert_after_symbol, rename_symbol, safe_delete_symbol, verify_edit
- **FileOpsSkill** (fileops/skill.go): 6 ToolDefs -- read_file, create_file, list_directory, find_files, search_in_files, replace_in_file
- **DiagnosticsSkill** (diag/skill_adapter.go): 3 ToolDefs -- get_diagnostics, get_code_actions, format_code

All register via `init()` using `skill.Register()`. Profile and mode YAMLs already reference matching skill names -- no changes needed.

### Task 2: Wire pool to use Installer.Resolve()

- Added `installer *langregistry.Installer` field to Pool struct
- Updated `NewPool` signature to accept installer parameter
- Modified `spawnWorkerLocked` to call `p.installer.Resolve(ctx, entry)` when installer is non-nil, falling back to `entry.Command` for backward compatibility
- Updated `NewKernel` to accept and forward installer parameter
- Fixed 3 test call sites to pass `nil` installer

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | f00e1733 | Create kernel tool skill adapters and verify profile YAMLs |
| 2 | ae323064 | Wire pool to use Installer.Resolve() for three-tier LS resolution |

## Deviations from Plan

None -- plan executed exactly as written.

## Known Stubs

None -- all adapters return complete ToolDef lists matching registered MCP tools.

## Verification

- `go build ./internal/kernel/...` -- passes
- `go vet ./internal/kernel/...` -- passes
- `go test ./internal/kernel/lspool/ -count=1 -short` -- passes
- All 4 skill adapter files exist with correct method signatures
