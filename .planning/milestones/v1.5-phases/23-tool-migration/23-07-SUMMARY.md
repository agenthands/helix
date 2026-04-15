---
phase: 23-tool-migration
plan: 07
subsystem: profile
tags: [typed-errors, serr, profile, mode-switching]
dependency_graph:
  requires: [22-01]
  provides: [typed-errors-profile]
  affects: [internal/profile/skill.go]
tech_stack:
  added: []
  patterns: [serr.New, WithTool, WithDetail]
key_files:
  created: []
  modified: [internal/profile/skill.go]
decisions:
  - "Left Init() startup errors as fmt.Errorf per Pitfall 5 -- they cause daemon fail-fast before MCP"
  - "loader.go untouched -- all errors are startup-only, never reach MCP responses"
metrics:
  duration: ~3min
  completed: 2026-04-15T11:42:24Z
  tasks: 1/1
  files_modified: 1
---

# Phase 23 Plan 07: Profile Tools Typed Error Migration Summary

Migrated 10 MCP-facing error sites in profile/skill.go from raw fmt.Errorf to typed serr constructors, enabling agents to programmatically match error kinds on mode switch and token budget operations.

## Task Completion

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Migrate profile/skill.go to typed errors | 47e65cb9 | internal/profile/skill.go |

## Changes Made

### internal/profile/skill.go (10 error sites migrated)

**Mode validation (validateModeTransition):**
- Unknown mode: `serr.New(serr.InvalidArgs, "unknown mode").WithDetail(...)` -- includes available modes
- Unknown profile: `serr.New(serr.InvalidArgs, "unknown profile").WithDetail(profileName)`
- No transitions defined: `serr.New(serr.InvalidArgs, "mode has no transitions defined").WithDetail(...)`
- Transition not allowed: `serr.New(serr.InvalidArgs, "mode transition not allowed").WithDetail(...)`

**ExecuteSwitchMode:**
- No session provider: `serr.New(serr.Internal, "no session provider configured")`
- No active session: `serr.New(serr.Internal, "no active session")`

**computeTokenBudget:**
- Unknown profile: `serr.New(serr.InvalidArgs, "unknown profile").WithDetail(profileName)`
- Unknown mode: `serr.New(serr.InvalidArgs, "unknown mode").WithDetail(modeName)`

**ExecuteTool dispatch:**
- Missing target_mode: `serr.New(serr.InvalidArgs, "missing required parameter: target_mode").WithTool("switch_mode")`
- Unknown tool: `serr.New(serr.InvalidArgs, "unknown tool").WithTool(name)`

### Not migrated (by design)

**Init() errors (3 sites, L44/51/57):** Loading embedded profiles, global overrides, project overrides. These are startup-only errors that cause daemon fail-fast before any MCP tool call is processed. Left as `fmt.Errorf` per Pitfall 5.

**loader.go:** Zero changes. All error sites are startup/loading errors that never reach MCP responses.

## Deviations from Plan

None -- plan executed exactly as written.

## Verification

- `go vet ./internal/profile/` -- passed
- `go test ./internal/profile/ -count=1` -- passed (all existing tests pass; error string assertions like `Contains("not allowed")` and `Contains("unknown mode")` still match via serr.Error() output format)
- `go build ./cmd/serena` -- succeeded

## Self-Check: PASSED
