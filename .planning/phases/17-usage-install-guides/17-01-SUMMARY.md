---
phase: 17-usage-install-guides
plan: 01
subsystem: documentation
tags: [usage, observability, metrics, benchmarks, documentation]
dependency-graph:
  requires: []
  provides: [accurate-usage-docs, benchmark-docs]
  affects: [USAGE.md]
tech-stack:
  added: []
  patterns: []
key-files:
  modified:
    - USAGE.md
decisions:
  - Corrected serena_tool_duration_seconds labels from (tool_name, outcome) to (tool_name, profile, mode, language) per actual source code
  - Documented that serena_tool_calls_total has 5 labels while serena_tool_duration_seconds has 4 (no outcome on histogram)
metrics:
  duration: 132s
  completed: "2026-04-11T09:05:33Z"
  tasks: 2/2
  files-modified: 1
requirements-completed: [USAGE-01, USAGE-02, USAGE-03]
---

# Phase 17 Plan 01: Update USAGE.md Summary

USAGE.md accuracy gaps fixed against v1.2 source code (6 metrics documented, service_name config added, label dimensions with PromQL examples) and benchmark workflow subsection added under Observability Quickstart.

## Commits

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Fix USAGE.md accuracy gaps in observability and metrics sections | `08a40213` | USAGE.md |
| 2 | Add benchmark subsection under Observability Quickstart | `fea655be` | USAGE.md |

## Task Details

### Task 1: Fix USAGE.md accuracy gaps

Fixed 4 verified accuracy gaps:

1. **Gap 1 (service_name config key):** Added `observability.service_name` (string, default `serena`) to the Observability Settings table and added `service_name: "my-serena-instance"` to the example YAML config block.

2. **Gap 2 (missing metrics):** Added `serena_tool_calls_total` (counter) and `serena_lspool_restarts_total` (counter) to the Prometheus Metrics table. Table now has 6 metrics matching all vectors in `internal/obs/metrics.go`.

3. **Gap 3 (label documentation):** Added metric label documentation paragraph with 4 PromQL query examples (error rate, p95 latency, active workers, eviction rate) after the Prometheus scrape config block.

4. **Gap 4 (eviction reason label):** Updated `serena_lspool_evictions_total` description to include `reason` label with closed enum: `idle`, `pressure`, `crash`, `shutdown`.

Additionally corrected `serena_tool_duration_seconds` label documentation from `(tool_name, outcome)` to `(tool_name, profile, mode, language)` per actual source code, and added accurate labels to all other metrics.

### Task 2: Add benchmark subsection

Added `### Benchmarks` subsection at line 585, positioned after `### Enable pprof` (line 560) and before `## Performance Tuning` (line 632). Content covers:

- Local benchmark run commands (`go test -bench=.`)
- Specific suite targeting (`-bench=BenchmarkTools`)
- Memory allocation stats (`-benchmem`)
- benchstat install and comparison workflow with `-count=5`
- Interpretation guidance (confidence intervals, `~`/`+`/`-` meanings)
- Tips: `-short` flag, `GOMAXPROCS=4`, link to CONTRIBUTING.md for CI details

No CI gate content (benchgate, thresholds, baseline capture) per D-05.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected serena_tool_duration_seconds label documentation**
- **Found during:** Task 1
- **Issue:** USAGE.md documented labels as `(tool_name, outcome)` but actual source code in `internal/obs/metrics.go` line 68 shows labels are `(tool_name, profile, mode, language)` -- no `outcome` label on the histogram
- **Fix:** Updated to accurate 4-label set; documented that `serena_tool_calls_total` has the 5th `outcome` label
- **Files modified:** USAGE.md
- **Commit:** 08a40213

**2. [Rule 1 - Bug] Added missing label documentation to all metrics**
- **Found during:** Task 1
- **Issue:** Plan only specified adding labels for 2 new metrics, but existing metrics also lacked label documentation
- **Fix:** Added accurate label dimensions to all 6 metric descriptions in the table
- **Files modified:** USAGE.md
- **Commit:** 08a40213

**3. [Scope note] Pre-staged git deletions included in Task 1 commit**
- **Found during:** Task 1 commit
- **Issue:** The git index already had many `.planning/phases/` file deletions staged from a prior session. The `git add USAGE.md && git commit` picked up these pre-staged changes.
- **Impact:** Non-functional. The deleted files are old planning artifacts from phases 01-15 that were already removed from the working tree.

## Verification

- All config keys verified against `internal/config/config.go` koanf struct tags
- All 6 metric names and labels verified against `internal/obs/metrics.go` newMetrics()
- Benchmark commands verified against `test/bench/baselines/README.md`
- No `benchgate` mention in USAGE.md (D-05 compliance confirmed)
- `go vet ./...` passes (pre-existing fixture warning only)
- `go test -short ./...` all tests pass

## Self-Check: PASSED
