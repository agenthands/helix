---
phase: 25-fuzzy-edit-engine
plan: 04
subsystem: fuzzy
tags: [indentation, reflow, whitespace, tdd]
dependency_graph:
  requires: ["25-01 (types.go)"]
  provides: ["reflow function for Plan 06 match.go"]
  affects: ["internal/fuzzy/"]
tech_stack:
  added: []
  patterns: ["Aider-style common-prefix dedent + reapply", "pure function pipeline"]
key_files:
  created:
    - internal/fuzzy/indent.go
    - internal/fuzzy/indent_test.go
  modified: []
decisions:
  - "splitLines included in indent.go as package-private utility (no trailing empty element on final newline)"
  - "leadingWhitespace counts only ' ' and '\\t' -- no Unicode whitespace, matching CONTEXT.md contract"
metrics:
  duration: 98s
  completed: "2026-04-16T14:02:19Z"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 0
---

# Phase 25 Plan 04: Indentation Reflow Summary

Aider-style indentation reflow with common-prefix dedent and source-region reapply, fully TDD with RED/GREEN commits.

## What Was Built

Three pure helpers (`commonLeadingPrefix`, `dedent`, `reapplyPrefix`) and one orchestrator (`reflow`) that together transform a replacement block from the search block's indentation to the matched source region's indentation. The `splitLines` utility handles newline splitting without trailing empty elements.

The pipeline:
1. Compute `searchPrefix` = longest common leading whitespace across non-empty search lines
2. Strip `searchPrefix` from each replacement line (dedent)
3. Extract `sourcePrefix` = leading whitespace of first non-empty matched source line
4. Prepend `sourcePrefix` to each non-empty dedented replacement line

Key invariants:
- Tabs and spaces preserved verbatim from source -- no expansion, no normalization
- Empty replacement lines stay as "" -- never gain trailing whitespace
- Whitespace-only lines treated as empty for prefix calculation

## TDD Gate Compliance

- RED commit: `04d78282` -- `test(25-04): add failing tests for indentation reflow (RED)`
- GREEN commit: `f912d2d9` -- `feat(25-04): implement indentation reflow pipeline (GREEN)`
- REFACTOR: not needed -- code already clean

## Task Summary

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create indent.go with reflow pipeline (TDD) | 04d78282, f912d2d9 | internal/fuzzy/indent.go, internal/fuzzy/indent_test.go |
| 2 | Format, vet, test | (no changes) | internal/fuzzy/ |

## Test Coverage

- `TestCommonLeadingPrefix`: 7 subtests (spaces, tabs, empty-line ignored, no prefix, mixed, all-empty, single)
- `TestDedent`: 4-line block with empty interior line
- `TestReapplyPrefix_PreservesEmptyLines`: tabs prefix, empty line guard
- `TestReflow_AppliesSourcePrefix`: 4-space search to 8-space source
- `TestReflow_TabsAndSpaces`: spaces in search, tabs in source
- `TestReflow_PreservesEmptyLines`: blank line survives as ""

## Deviations from Plan

None -- plan executed exactly as written.

## Known Stubs

None.

## Verification

- `go test ./internal/fuzzy/ -run "TestReflow"` -- all pass
- `go vet ./internal/fuzzy/...` -- clean
- `gofmt -l internal/fuzzy/` -- empty

## Self-Check: PASSED
