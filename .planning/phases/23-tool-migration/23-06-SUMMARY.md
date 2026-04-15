---
phase: 23-tool-migration
plan: "06"
subsystem: skill/workflow
tags: [error-migration, typed-errors, workflow]
dependency_graph:
  requires: [internal/errors]
  provides: [typed-errors-workflow]
  affects: [internal/skill/workflow]
tech_stack:
  added: []
  patterns: [serr.New, serr.InvalidArgs, WithTool]
key_files:
  created: []
  modified:
    - internal/skill/workflow/skill.go
    - internal/skill/workflow/skill_test.go
decisions:
  - "Used InvalidArgs kind for unknown tool error (semantic intent: bad input value)"
metrics:
  duration: ~2 min
  completed: "2026-04-15"
  tasks: 1
  files: 2
---

# Phase 23 Plan 06: Workflow Tools Error Migration Summary

Migrated the single error site in workflow/skill.go from raw fmt.Errorf to typed serr.New(serr.InvalidArgs) with .WithTool() attribution.

## Task Results

| Task | Name | Commit | Status |
|------|------|--------|--------|
| 1 | Migrate workflow/skill.go to typed errors | 59b4dd1d | Done |

## Changes Made

### Task 1: Migrate workflow/skill.go to typed errors

**skill.go:**
- Added `serr "github.com/postfix/serena/internal/errors"` import
- Replaced `fmt.Errorf("unknown workflow tool: %s", name)` with `serr.New(serr.InvalidArgs, "unknown workflow tool").WithTool(name)`
- Kept `fmt` import (still used by fmt.Sprintf in 3 other locations)

**skill_test.go:**
- Added `errors` and `serr` imports
- Enhanced `TestWorkflowSkill_UnknownTool` to assert `errors.Is(err, serr.ErrInvalidArgs)` in addition to existing string containment check

## Verification

- `go vet ./internal/skill/workflow/` -- passed
- `go test ./internal/skill/workflow/ -count=1` -- passed
- `go build ./cmd/serena` -- passed
- grep confirms 0 remaining `fmt.Errorf` in skill.go
- grep confirms `serr.New(serr.InvalidArgs, "unknown workflow tool")` present

## Deviations from Plan

None -- plan executed exactly as written.

## Self-Check: PASSED

- [x] internal/skill/workflow/skill.go exists and contains serr import
- [x] internal/skill/workflow/skill_test.go exists and contains errors.Is assertion
- [x] Commit 59b4dd1d exists in git log
