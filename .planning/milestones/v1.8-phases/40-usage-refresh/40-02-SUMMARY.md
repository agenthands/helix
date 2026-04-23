---
phase: 40-usage-refresh
plan: 02
subsystem: documentation
tags: [usage-guide, troubleshooting, jdtls, gopls, language-servers]
dependency_graph:
  requires: [40-01]
  provides: [jdtls-troubleshooting, gopls-troubleshooting]
  affects: [USAGE.md]
tech_stack:
  added: []
  patterns: [symptom-cause-fix]
key_files:
  created: []
  modified:
    - USAGE.md
decisions:
  - Entries placed between Mode Restrictions and Circuit Breaker Open (rust-analyzer entry referenced in plan does not exist in current USAGE.md)
  - jdtls entry references .jdtls-data workspace directory and degradation.timeout_index config key
  - gopls entry documents v0.17.1/Go 1.25 linux/amd64 incompatibility and testing.B.Loop Go 1.24 requirement
metrics:
  completed: 2026-04-23
  tasks: 1
  files_modified: 1
---

# Phase 40 Plan 02: jdtls and gopls Troubleshooting Entries Summary

Two new troubleshooting entries added to USAGE.md documenting jdtls cold-start indexing delay with timeout_index 300s workaround and gopls v0.17.1/Go 1.25 version incompatibility with upgrade instructions.

## Tasks Completed

### Task 1: Add jdtls and gopls troubleshooting entries
- **Commit:** a225e277
- **Files modified:** USAGE.md (+38 lines)
- Added `### jdtls cold-start indexing delay` entry with Symptom/Cause/Fix pattern
- Added `### gopls version incompatibility with Go 1.25` entry with Symptom/Cause/Fix pattern
- jdtls entry includes `timeout_index: 300` YAML config example
- gopls entry documents v0.17.1 incompatibility, `testing.B.Loop` requirement, and `capture-baseline.yml` CI workflow
- Both entries positioned between Mode Restrictions and Circuit Breaker Open sections

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] rust-analyzer entry not found**
- **Found during:** Task 1
- **Issue:** Plan specified inserting entries "AFTER the rust-analyzer entry" but no rust-analyzer troubleshooting section exists in current USAGE.md. The patterns file references lines 420-436 as the rust-analyzer analog, but those lines contain the Language Server Not Starting section.
- **Fix:** Inserted entries before Circuit Breaker Open heading as the plan's secondary anchor point, maintaining correct document ordering.
- **Files modified:** USAGE.md

## Verification Results

- `grep -c "### jdtls cold-start indexing delay"` returns 1
- `grep -c "### gopls version incompatibility with Go 1.25"` returns 1
- Both entries use Symptom/Cause/Fix pattern matching existing entries
- Entries positioned between Mode Restrictions (line 470) and Circuit Breaker Open (line 531)
- `go vet ./...` passes (exit code 0)
