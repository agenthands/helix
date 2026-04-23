---
phase: 39-readme-rewrite
plan: 01
subsystem: build-tooling
tags: [docgen, readme, skill-adapters]
dependency_graph:
  requires: []
  provides: [accurate-readme-tables, complete-skill-adapters]
  affects: [README.md, cmd/docgen]
tech_stack:
  added: []
  patterns: [skill-adapter-for-kernel-tools]
key_files:
  created:
    - internal/kernel/health/skill_adapter.go
    - internal/kernel/help/skill_adapter.go
  modified:
    - cmd/docgen/main.go
    - README.md
decisions:
  - Created skill adapters for health and help packages to expose them via skill.ToolProviders()
metrics:
  duration: 2m
  completed: 2026-04-23
---

# Phase 39 Plan 01: Fix Docgen Imports and Regenerate README Tables Summary

Added missing skill adapters for health/help kernel packages and 3 blank imports to docgen, regenerating README with all 40 MCP tools and 52 languages.

## Task Summary

| Task | Name | Commit | Status |
|------|------|--------|--------|
| 1 | Add missing blank imports to docgen and regenerate tables | 0c2b0480 | Done |

## What Was Done

1. **Added 3 missing blank imports to `cmd/docgen/main.go`**: health, help, repomap packages now imported, matching daemon/imports.go plus the two additional kernel packages.

2. **Created `internal/kernel/health/skill_adapter.go`**: New skill adapter that registers health as a ToolProvider via init(), exposing `get_health` through `skill.ToolProviders()`.

3. **Created `internal/kernel/help/skill_adapter.go`**: New skill adapter that registers help as a ToolProvider via init(), exposing `get_tool_help` through `skill.ToolProviders()`.

4. **Regenerated README.md tables**: Tool table now contains 40 tools (was 35), language table contains 52 languages.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Created skill adapters for health and help packages**
- **Found during:** Task 1
- **Issue:** The plan assumed blank imports would be sufficient for health and help packages, but these packages lacked `init()` / `skill.Register()` calls. They use the kernel-style `RegisterTools` pattern only, meaning `skill.ToolProviders()` (which docgen reads) would not include them.
- **Fix:** Created `skill_adapter.go` files for both packages, following the established pattern from `internal/kernel/diag/skill_adapter.go`. Each adapter implements the `skill.Skill` and `skill.ToolProvider` interfaces with `init()` registration.
- **Files created:** `internal/kernel/health/skill_adapter.go`, `internal/kernel/help/skill_adapter.go`
- **Commit:** 0c2b0480

## Verification

- `go vet ./cmd/docgen/...` passes
- `go test ./internal/kernel/health/... ./internal/kernel/help/... ./cmd/docgen/...` all pass
- `go run ./cmd/docgen --check` reports "README.md is up to date"
- Tool table has 40 rows (verified with grep)
- All 5 key tools present: get_repo_map, get_context, get_health, get_tool_help, fuzzy_edit
- Language table has 52 rows

## Self-Check: PASSED
