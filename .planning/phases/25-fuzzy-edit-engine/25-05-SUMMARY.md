---
phase: 25-fuzzy-edit-engine
plan: 05
subsystem: fuzzy-edit-engine
tags: [fuzzy, diff, formatter, pure-functions]
dependency_graph:
  requires: ["25-01 (types.go)"]
  provides: ["formatAmbiguity", "formatFailureDiff", "findNearestWindow"]
  affects: ["25-06 (match.go wraps these in serr.InvalidArgs)"]
tech_stack:
  added: []
  patterns: ["golden-string testing", "sliding-window best-match", "unified-diff-like payload"]
key_files:
  created:
    - internal/fuzzy/diff.go
    - internal/fuzzy/diff_test.go
  modified: []
decisions:
  - "Hand-rolled diff format (no third-party difflib) per RESEARCH.md guidance"
  - "5-hit cap on ambiguity line list with overflow suffix"
metrics:
  duration: "2m"
  completed: "2026-04-16T14:02:25Z"
  tasks_completed: 2
  tasks_total: 2
---

# Phase 25 Plan 05: Failure-Diff and Ambiguity Formatters Summary

Pure string formatters for FUZZ-01 failure-diff payloads and FUZZ-07 ambiguity detail, pinned with golden-string boundary tests at 2/5/6/10 hits.

## Tasks Completed

| Task | Name | Commit(s) | Key Files |
|------|------|-----------|-----------|
| 1 | Create diff.go with formatFailureDiff, formatAmbiguity, findNearestWindow | d91bc91f (RED), ee106f9e (GREEN) | internal/fuzzy/diff.go, internal/fuzzy/diff_test.go |
| 2 | Format, vet, test | (no changes needed) | internal/fuzzy/ |

## TDD Gate Compliance

- RED gate: d91bc91f -- test(25-05): add failing tests for diff formatters
- GREEN gate: ee106f9e -- feat(25-05): implement formatFailureDiff, formatAmbiguity, findNearestWindow
- REFACTOR gate: not needed (code already clean)

## Implementation Details

### formatAmbiguity(hits []int) string
Renders the agent-facing ambiguity error detail. Takes 1-indexed line numbers, caps the shown list at 5, appends `, ...and N more` for overflow. Boundary behavior:
- Exactly 5 hits: no overflow suffix
- 6 hits: "...and 1 more"
- 10 hits: "...and 5 more"

### formatFailureDiff(search, nearestWindow string) string
Renders a minimal unified-diff-like payload with `--- search` / `+++ nearest source region` / `@@ -1,N +1,M @@` headers and `-`/`+` prefixed lines. Handles empty nearest window gracefully.

### findNearestWindow(source, search []string) []string
Sliding window selector: finds the source window of same length as search with the most byte-equal lines. Returns entire source when shorter than search. Always allocates a new slice (no mutation).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Removed diff_stub.go from parallel Plan 03**
- **Found during:** Task 2
- **Issue:** `diff_stub.go` existed on disk (from a parallel wave-1 agent) with duplicate function declarations, causing `go vet` to fail with "redeclared in this block"
- **Fix:** Deleted the untracked stub file since real implementation now provides all three functions
- **Files modified:** internal/fuzzy/diff_stub.go (deleted, was untracked)

## Known Stubs

None -- all functions are fully implemented with production logic.

## Verification

- All 7 test functions pass (4 ambiguity boundary rows + 3 diff/window tests)
- `go vet ./internal/fuzzy/...` clean
- `gofmt -l internal/fuzzy/` empty
- No third-party diff imports (no difflib, no sergi/go-diff)
- No serr import (pure string returns)

## Self-Check: PASSED

- internal/fuzzy/diff.go: FOUND
- internal/fuzzy/diff_test.go: FOUND
- Commit d91bc91f (RED): FOUND
- Commit ee106f9e (GREEN): FOUND
