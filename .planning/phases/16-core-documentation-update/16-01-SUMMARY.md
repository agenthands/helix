---
phase: 16-core-documentation-update
plan: 01
subsystem: documentation
tags: [readme, observability, production, documentation]
dependency_graph:
  requires: []
  provides: [readme-v1.2-complete]
  affects: [README.md]
tech_stack:
  added: []
  patterns: [auto-generated-tables-via-docgen]
key_files:
  created: []
  modified: [README.md]
decisions:
  - Corrected tool count from "38+" to "35+" to match actual generated tool table (35 tools)
metrics:
  duration: 136s
  completed: "2026-04-11T08:16:34Z"
---

# Phase 16 Plan 01: README Update Summary

Updated README.md with Production & Observability section covering metrics, tracing, admin endpoints, and graceful degradation; corrected tool count to match generated table.

## Tasks Completed

### Task 1: Add Production & Observability section and update README content
- **Commit:** bdac3c4a
- **What:** Added new "Production & Observability" section with four subsections (Metrics & Monitoring, Distributed Tracing, Admin Endpoints, Graceful Degradation) positioned after "Key Advantages" and before "Programming Language Support". Added admin listener mention to Architecture section.
- **Files modified:** README.md

### Task 2: Regenerate auto-generated tables and verify install instructions
- **Commit:** 6ae8b25e
- **What:** Ran `make docs` to regenerate tables (already current, no changes). Verified `go build ./cmd/serena` and `go vet ./...` both pass. Confirmed install paths match `go.mod` module name. Fixed tool count claim from "38+" to "35+" to match actual 35-tool generated table.
- **Files modified:** README.md

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed inaccurate tool count claim**
- **Found during:** Task 2
- **Issue:** README claimed "38+ MCP tools" but the auto-generated tool table contains exactly 35 tools. The "38+" figure was inherited from earlier documentation and never matched the generated table.
- **Fix:** Changed claim to "35+ MCP tools" to match the generated table (source of truth).
- **Files modified:** README.md
- **Commit:** 6ae8b25e

## Deferred Items

- CLAUDE.md still references "38+ MCP tools" -- out of scope for this README-only plan. Should be updated in a separate task.

## Verification Results

- `make docs` exits 0 (tables regenerated)
- `go build ./cmd/serena` exits 0
- `go vet ./...` exits 0
- README contains "Production & Observability" section with all required subsections
- README contains `/healthz`, `/readyz`, `/metrics`, `/debug/pprof` endpoints
- README contains `OTLP` in tracing subsection
- README contains `GOMEMLIMIT` in degradation subsection
- Section ordering correct: Key Advantages (line 33) -> Production & Observability (line 43) -> Programming Language Support (line 79)
- Tool and language table markers present

## Self-Check: PASSED
