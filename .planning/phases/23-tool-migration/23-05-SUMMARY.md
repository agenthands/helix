---
phase: 23-tool-migration
plan: 05
subsystem: memory-skill
tags: [error-migration, typed-errors, memory-tools]
dependency_graph:
  requires: [22-01]
  provides: [typed-errors-memory]
  affects: [internal/skill/memory]
tech_stack:
  added: []
  patterns: [serr-typed-errors, builder-chain-WithTool]
key_files:
  created: []
  modified:
    - internal/skill/memory/skill.go
    - internal/skill/memory/skill_test.go
decisions:
  - "Used serr.InvalidArgs for all parameter validation and unknown tool errors"
  - "Used serr.Wrap(serr.Internal, ...) for store operation failures"
  - "Init error uses serr.Wrap(serr.Internal, ...) for consistency"
metrics:
  duration: ~3min
  completed: 2026-04-15
---

# Phase 23 Plan 05: Memory Skill Typed Errors Summary

Migrated all 21 error sites in memory/skill.go from raw fmt.Errorf to typed serr constructors with .WithTool() attribution, enabling agent auto-correction on InvalidArgs errors.

## What Changed

### internal/skill/memory/skill.go
- Added `serr "github.com/postfix/serena/internal/errors"` import
- **1 init error**: `fmt.Errorf("memory skill init: %w")` -> `serr.Wrap(serr.Internal, "memory skill init", err)`
- **1 unknown tool**: `fmt.Errorf("unknown memory tool: %s")` -> `serr.New(serr.InvalidArgs, "unknown memory tool").WithTool(name)`
- **10 parameter validation errors**: All use `serr.New(serr.InvalidArgs, "...").WithTool("tool_name")`
- **9 operation errors**: All use `serr.Wrap(serr.Internal, "... operation failed", err).WithTool("tool_name")`
- Removed tool name prefix from error messages per D-05 convention
- Zero `fmt.Errorf` calls remain

### internal/skill/memory/skill_test.go
- Added `errors` and `serr` imports
- `TestMemorySkill_ErrorOnMissingParams`: Added `errors.Is(err, serr.ErrInvalidArgs)` assertions for all 6 tools
- `TestMemorySkill_UnknownTool`: Added `errors.Is(err, serr.ErrInvalidArgs)` assertion

## Error Site Breakdown

| Tool | InvalidArgs | Internal (Wrap) | Total |
|------|-------------|-----------------|-------|
| write_memory | 2 | 1 | 3 |
| read_memory | 1 | 1 | 2 |
| list_memories | 0 | 2 | 2 |
| search_memories | 1 | 2 | 3 |
| rename_memory | 2 | 1 | 3 |
| edit_memory | 3 | 1 | 4 |
| delete_memory | 1 | 1 | 2 |
| unknown tool | 1 | 0 | 1 |
| init | 0 | 1 | 1 |
| **Total** | **11** | **10** | **21** |

## Deviations from Plan

None - plan executed exactly as written.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | ea37bd52 | Migrate all 21 error sites to typed serr errors |

## Verification

- `go vet ./internal/skill/memory/` -- PASSED
- `go test ./internal/skill/memory/ -count=1` -- PASSED
- `go build ./cmd/serena` -- PASSED
- `grep fmt.Errorf internal/skill/memory/skill.go` -- zero matches

## Self-Check: PASSED
