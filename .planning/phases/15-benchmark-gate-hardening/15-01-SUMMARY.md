---
phase: 15-benchmark-gate-hardening
plan: 01
subsystem: ci-benchmark-gate
tags: [benchmark, ci, github-actions, regression-gate]
dependency_graph:
  requires: []
  provides: [capture-baseline-workflow, blocking-benchmark-gate]
  affects: [bench.yml, baselines-readme]
tech_stack:
  added: [stefanzweifel/git-auto-commit-action@v5]
  patterns: [workflow_dispatch-baseline-capture, blocking-benchgate]
key_files:
  created:
    - .github/workflows/capture-baseline.yml
  modified:
    - .github/workflows/bench.yml
    - test/bench/baselines/README.md
decisions:
  - "D-01: Dedicated capture-baseline.yml workflow separate from gate workflow"
  - "D-02: Auto-commit via git-auto-commit-action@v5 with contents:write"
  - "D-03: Removed --warn-only immediately, no escape hatch or conditional logic"
  - "D-04: Per-milestone re-baseline policy documented in baselines/README.md"
metrics:
  duration: 5min
  completed: 2026-04-10
  tasks: 2
  files: 3
---

# Phase 15 Plan 01: Benchmark Gate Hardening Summary

Dedicated baseline capture workflow with auto-commit and blocking benchgate gate replacing warn-only mode, completing the Phase 9 three-step rollout.

## What Was Done

### Task 1: Create capture-baseline.yml and update baselines/README.md
**Commit:** 3f523884

Created `.github/workflows/capture-baseline.yml` -- a `workflow_dispatch`-only workflow that captures benchmark numbers on ubuntu-latest and auto-commits them to the triggering branch. The workflow mirrors bench.yml environment exactly (GOMAXPROCS=4, gopls v0.17.1, Go 1.25.x, `-short -bench=. -benchmem -count=10 -run=^$`) to prevent baseline-gate parameter mismatch (Pitfall 1). Includes a verification step requiring at least 5 `^Benchmark` lines before committing (Pitfall 2).

Updated `test/bench/baselines/README.md`:
- Renamed "Two-step rollout" to "Three-step rollout (COMPLETED)" with past-tense language
- Added "Re-baseline workflow" section documenting how to trigger capture-baseline.yml
- Updated refresh policy to reference the capture workflow instead of manual re-baseline PRs only

### Task 2: Remove --warn-only from bench.yml
**Commit:** 893486f6

Surgical edit to `.github/workflows/bench.yml`:
- Removed `--warn-only` flag from benchgate invocation
- Updated step name from "Run benchgate (WARN-ONLY during baseline rollout)" to "Run benchgate"
- Replaced 5-line WARN-ONLY comment block with concise blocking-mode documentation
- Updated artifact upload step name and comments to remove warn-only references
- Cleaned up benchstat TODO comment referencing "warn-only rollout window"
- Removed all remaining "warn-only" string references (case-insensitive: 0 matches)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Stale warn-only reference in header comment**
- **Found during:** Task 2 verification
- **Issue:** The file header comment on line 6 still contained "warn-only" after the main edits, causing the case-insensitive verification check to fail
- **Fix:** Updated header to "three-step rollout (now in blocking mode)" removing the warn-only reference
- **Files modified:** .github/workflows/bench.yml
- **Commit:** 893486f6

**2. [Rule 1 - Bug] Stale warn-only reference in benchstat TODO comment**
- **Found during:** Task 2 implementation
- **Issue:** The benchstat install step had a TODO comment referencing "the warn-only rollout window"
- **Fix:** Simplified to generic supply-chain hardening TODO
- **Files modified:** .github/workflows/bench.yml
- **Commit:** 893486f6

## Verification Results

| Check | Result |
|-------|--------|
| capture-baseline.yml has workflow_dispatch trigger | PASS |
| bench.yml has no warn-only references (case-insensitive) | PASS |
| capture-baseline.yml has git-auto-commit-action | PASS |
| baselines/README.md references capture-baseline | PASS |
| go vet ./... | PASS |

## Self-Check: PASSED

- FOUND: .github/workflows/capture-baseline.yml
- FOUND: .github/workflows/bench.yml (modified)
- FOUND: test/bench/baselines/README.md (modified)
- FOUND: 3f523884 (Task 1 commit)
- FOUND: 893486f6 (Task 2 commit)
