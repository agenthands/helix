---
phase: 41-install-contributing
plan: 02
subsystem: documentation
tags: [contributing, project-structure, oracle-tests, test-harness]
dependency_graph:
  requires: []
  provides: [complete-contributing-guide, oracle-test-docs]
  affects: [CONTRIBUTING.md]
tech_stack:
  added: []
  patterns: []
key_files:
  created: []
  modified: [CONTRIBUTING.md]
decisions:
  - Organized project structure listing by architectural layer with all current packages
  - Documented oracle test hierarchy as table format for quick reference
  - Updated "Adding a New MCP Tool" guide to include health/help categories and oracle test step
metrics:
  duration: 77s
  completed: 2026-04-23T14:31:21Z
  tasks_completed: 1
  tasks_total: 1
---

# Phase 41 Plan 02: Update CONTRIBUTING.md Summary

Updated CONTRIBUTING.md with complete project structure (added 10 missing packages), corrected tool counts (fileops 6->7), and documented the 6-layer oracle test hierarchy with test harness reference.

## Task Results

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Update CONTRIBUTING.md project structure and test documentation | b3932958 | CONTRIBUTING.md |

## Changes Made

### Project Structure Section
- Added 10 missing packages: `internal/cli/`, `internal/errors/`, `internal/fuzzy/`, `internal/kernel/health/`, `internal/kernel/help/`, `internal/kernel/jsonrpc/`, `internal/repomap/`, `internal/skill/repomap/`, `internal/treesitter/`, `internal/workspace/`
- Added `internal/memory/` (was missing from listing)
- Added `test/harness/` and `test/oracle/` entries
- Updated `internal/mcp/` description to mention smart error suggestions and lazy workspace init
- Corrected `internal/kernel/fileops/` from "6 file operation tools" to "7 file operation tools (includes fuzzy_edit)"
- Updated `test/bench/` description to mention baselines

### New Oracle Test Suite Section
- Added full section between "Running Integration Tests" and "Running Benchmarks"
- Documented 6 layers in table format: protocol, contract, runtime, scenario, llm, judge
- Included run commands for full suite and individual layers
- Referenced test harness package with description of shared infrastructure

### Adding a New MCP Tool Section
- Updated "Choose the right layer" to include health and help categories
- Updated step 5 to reference oracle scenario tests alongside integration tests

## Deviations from Plan

None -- plan executed exactly as written.

## Verification

- All 10 missing packages present in project structure listing
- Tool count for fileops is 7, not 6
- Oracle test suite section documents all 6 layers with table format
- Test harness package referenced
- No reference to non-existent progressive.go
- Existing sections (integration tests, benchmarks, CI gate, language support) preserved unchanged
- `go vet ./...` passes cleanly
