---
phase: 36-client-hooks
plan: 01
subsystem: cli
tags: [hooks, claude-code, nudge, setup]
dependency_graph:
  requires: []
  provides: [setup_hooks, nudge_command, hook_merge_helpers]
  affects: [setup_clients, setup, root]
tech_stack:
  added: []
  patterns: [atomic-file-write, serena_managed-marker, counter-threshold-nudge]
key_files:
  created:
    - internal/cli/setup_hooks.go
    - internal/cli/nudge.go
    - internal/cli/setup_hooks_test.go
    - internal/cli/nudge_test.go
  modified:
    - internal/cli/setup_clients.go
    - internal/cli/setup.go
    - internal/cli/root.go
decisions:
  - Used serena_managed boolean marker in hook command objects for idempotent identification instead of command-path matching
  - Atomic write via temp file + rename for session-stats.json to handle concurrent hook invocations
  - Nudge reads stdin JSON as primary source, --tool flag as fallback for manual testing
metrics:
  duration: 273s
  completed: 2026-04-21T18:31:00Z
  tasks_completed: 2
  tasks_total: 2
  files_created: 4
  files_modified: 3
---

# Phase 36 Plan 01: Hook JSON Helpers and Nudge Command Summary

Hook installation helpers and nudge CLI command for Claude Code integration with serena_managed marker for idempotent add/remove and atomic counter tracking.

## What Was Done

### Task 1: Hook JSON helpers and nudge command
- Created `internal/cli/setup_hooks.go` with `hookSettingsPath`, `serenaHookConfig`, `mergeHooksIntoSettings`, `removeHooksFromSettings`, and `filterOutSerenaEntries` helper functions
- Created `internal/cli/nudge.go` with `newNudgeCommand`, `runNudge` (stdin parsing, counter tracking, threshold logic), `loadSessionStats`, `saveSessionStats` (atomic write), `isSerenaSymbolicTool` (9 tools, with/without prefix), `isGrepReadTool` (Grep/Read/Bash pattern matching)
- Registered nudge command in `root.go`
- **Commit:** 0e0c075e

### Task 2: Extend ClaudeCodeRegistrar and add tests
- Added `NoHooks bool` to `RegistrationConfig`
- Extended `ClaudeCodeRegistrar.Register` to call `mergeHooksIntoSettings` after MCP registration (non-fatal on failure per D-16)
- Extended `ClaudeCodeRegistrar.Unregister` to call `removeHooksFromSettings` after MCP removal
- Added `--no-hooks` flag to setup command, wired through to `RegistrationConfig`
- Created 8 hook tests: new file, existing hooks preserved, idempotent, remove serena-only, missing file, structure validation, project/global paths
- Created 9 nudge tests: load new/different session, round-trip, symbolic tool detection, grep/read detection, below/at threshold, reset by symbolic tool, atomic write
- **Commit:** d6a4e5ae

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

- `go build ./internal/cli/...` -- passes
- `go vet ./internal/cli/...` -- passes
- All 17 new tests pass (8 hook + 9 nudge)

## Self-Check: PASSED
