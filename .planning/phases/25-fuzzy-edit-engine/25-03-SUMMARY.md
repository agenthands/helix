---
phase: 25-fuzzy-edit-engine
plan: 03
subsystem: fuzzy
tags: [fuzzy-matching, strategies, line-sweep, tdd]
dependency_graph:
  requires: [25-01]
  provides: [sweepExact, sweepWhitespace, sweepIndentFlex]
  affects: [internal/fuzzy/match.go]
tech_stack:
  added: []
  patterns: [line-sweep O(N*M), table-driven tests, TDD RED/GREEN]
key_files:
  created:
    - internal/fuzzy/strategies.go
    - internal/fuzzy/strategies_test.go
  modified: []
decisions:
  - "sweepIndentFlex strips only leading spaces/tabs (TrimLeft), NOT trailing -- asymmetric with sweepWhitespace (TrimSpace) by design"
metrics:
  duration: "3 min"
  completed: "2026-04-16T14:03:32Z"
  tasks_completed: 2
  tasks_total: 2
---

# Phase 25 Plan 03: Sweep Strategy Functions Summary

Three pure line-sweep functions (sweepExact, sweepWhitespace, sweepIndentFlex) as []string -> []int helpers for the 4-tier fuzzy match cascade.

## What Was Built

Three unexported sweep functions in `internal/fuzzy/strategies.go`:

1. **sweepExact** -- byte-for-byte line comparison. Returns all starting indices where `part` matches contiguously in `whole`.
2. **sweepWhitespace** -- `strings.TrimSpace` per line before comparing. Leading and trailing whitespace ignored; internal whitespace remains significant.
3. **sweepIndentFlex** -- `strings.TrimLeft(line, " \t")` per line. Tabs and spaces are interchangeable at line start; trailing whitespace remains significant.

All three share the same guard (`partLen == 0 || partLen > len(whole)` -> nil) and upper bound (`i <= len(whole)-partLen`) to prevent off-by-one at EOF.

## TDD Gate Compliance

- RED: `69982d4a` -- test(25-03): add failing tests for sweep strategies (15 test cases, compilation failure confirmed)
- GREEN: `ec26b26f` -- feat(25-03): implement sweepExact, sweepWhitespace, sweepIndentFlex (all 15 tests pass)
- REFACTOR: not needed -- implementation is already minimal

## Task Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Failing tests for sweep strategies | 69982d4a | internal/fuzzy/strategies_test.go |
| 1 (GREEN) | Implement sweep strategies | ec26b26f | internal/fuzzy/strategies.go |
| 2 | Format, vet, test | (no changes) | Already clean |

## Test Coverage

15 table-driven test cases across 3 test functions:

- **TestSweepExact** (7 cases): single hit, ambiguous 2 hits, no hit, empty part, part longer than whole, 1-line at EOF off-by-one, exact match whole-file
- **TestSweepWhitespace** (4 cases): leading ws ignored, trailing ws ignored, internal ws preserved (no match), empty part
- **TestSweepIndentFlex** (4 cases): tab vs spaces, spaces vs no indent, internal ws preserved, trailing ws NOT stripped

## Deviations from Plan

None -- plan executed exactly as written.

## Known Stubs

None -- all functions are fully implemented.

## Self-Check: PASSED
