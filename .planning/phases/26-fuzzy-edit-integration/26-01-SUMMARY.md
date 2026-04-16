---
phase: 26-fuzzy-edit-integration
plan: 01
subsystem: kernel/fileops
tags: [fuzzy-edit, mcp-tool, replace-fallback]
dependency_graph:
  requires: [internal/fuzzy]
  provides: [fuzzy_edit MCP tool, replace_in_file fuzzy fallback]
  affects: [internal/kernel/fileops]
tech_stack:
  added: []
  patterns: [fuzzy cascade fallback, DisableEllipsis zero-value-safe bool]
key_files:
  created:
    - internal/kernel/fileops/fuzzy_edit.go
    - internal/kernel/fileops/fuzzy_edit_test.go
  modified:
    - internal/kernel/fileops/tools.go
    - internal/kernel/fileops/skill.go
decisions:
  - Used DisableEllipsis (negative bool) instead of AllowEllipsis to honor Go zero-value semantics where default is ellipsis-enabled
  - Fuzzy fallback wired in MCP handler (not in ReplaceInFile function) to preserve existing (int, error) signature
  - replace_in_file fuzzy fallback uses AllowEllipsis=false since it is literal-oriented
metrics:
  duration: 222s
  completed: 2026-04-16
  tasks: 2/2
---

# Phase 26 Plan 01: Standalone fuzzy_edit MCP tool and replace_in_file fuzzy fallback Summary

FuzzyEdit function with 4-strategy cascade (exact/whitespace/indent-flex/fail), fuzzy_edit MCP tool registration, and replace_in_file auto-fallback to fuzzy matching when literal match returns 0 hits on non-regex patterns.

## What Was Built

### Task 1: FuzzyEdit function + fuzzy_edit tool + skill update (7632fa6b)

- Created `fuzzy_edit.go` with `FuzzyEdit(root, path, search, replacement, allowEllipsis)` that reads file, runs `fuzzy.Match`, and writes atomically via `OverwriteFile`
- Added `FuzzyEditArgs` struct with `DisableEllipsis` (negative bool for Go zero-value = enabled)
- Added `registerFuzzyEdit` handler with input validation, strategy/score reporting in response
- Updated `skill.go` to list 7 tools (was 6), added `fuzzy_edit` entry

### Task 2: replace_in_file fuzzy fallback + unit tests (570bd633)

- Wired fuzzy fallback in `registerReplaceInFile` handler: when `count == 0 && !args.IsRegex`, calls `fuzzy.Match` with `AllowEllipsis: false`
- Fuzzy success path reports `match_strategy` and `similarity_score` in response text
- Exact match path preserves existing `"N replacement(s) made in PATH"` format unchanged
- Created 7 unit tests covering: exact match, whitespace-normalized, no-match error, invalid path, ellipsis enabled/disabled, literal-no-match-fuzzy-succeeds, regex-no-fuzzy-fallback

## Deviations from Plan

None - plan executed exactly as written.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 7632fa6b | FuzzyEdit function, fuzzy_edit MCP tool, skill registration |
| 2 | 570bd633 | replace_in_file fuzzy fallback and unit tests |

## Verification

- `go build ./internal/kernel/fileops/...` -- PASS
- `go vet ./internal/kernel/fileops/...` -- PASS
- `go test ./internal/kernel/fileops/... -v -count=1` -- all 24 tests PASS
- `go build ./cmd/serena` -- PASS (full binary builds)

## Self-Check: PASSED

- [x] internal/kernel/fileops/fuzzy_edit.go exists
- [x] internal/kernel/fileops/fuzzy_edit_test.go exists
- [x] internal/kernel/fileops/tools.go contains FuzzyEditArgs, registerFuzzyEdit, fuzzy.Match
- [x] internal/kernel/fileops/skill.go lists 7 tools
- [x] Commit 7632fa6b exists
- [x] Commit 570bd633 exists
