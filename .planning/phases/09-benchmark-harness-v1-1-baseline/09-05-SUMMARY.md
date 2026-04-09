---
phase: 09-benchmark-harness-v1-1-baseline
plan: 05
subsystem: testing
tags: [benchmark, memory, pprof, rss, bench04]

# Dependency graph
requires:
  - phase: 09-benchmark-harness-v1-1-baseline
    provides: "Plan 09-02 bench daemon helpers + rss.CurrentRSS platform reader"
provides:
  - "BenchmarkMemory with 4 D-06 scenarios (daemon idle, post-workspace, post-first-call, post-100-calls)"
  - "Dual RSS reporting (runtime.ReadMemStats.Sys + rss.CurrentRSS) with distinct labels per Pitfall 7"
  - "Per-scenario pprof heap snapshots named {scenario}-{git-short-sha}.pb.gz per D-07/D-08"
  - "pprof artifact policy: CI-uploaded only, committed exceptions for baseline-*.pb.gz"
affects: [09-06, 10, 11]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "pprof scenario snapshots via runtime.GC() + pprof.WriteHeapProfile at scenario boundaries (09-RESEARCH.md Pattern 5)"
    - "Dual RSS reporting: rss_go_sys_bytes (what Go asked OS for) vs rss_kernel_bytes (what's actually resident)"
    - "Git-ignored CI artifacts with explicit baseline-* override for intentional re-baseline commits"

key-files:
  created:
    - test/bench/heap_snapshot_test.go
    - test/bench/memory_bench_test.go
    - test/bench/pprof/.gitkeep
    - test/bench/pprof/.gitignore
  modified: []

key-decisions:
  - "heap_snapshot.go renamed to heap_snapshot_test.go: package bench_test only compiles in _test.go files (blocking deviation)"
  - "Memory scenarios drive find_references from the shared benchTools manifest to prevent arg drift vs Plan 09-03 latency bench"
  - "Pitfall 8 policy codified via .gitignore + load-bearing top-of-file comment in memory_bench_test.go"

patterns-established:
  - "Pattern: scenario-boundary heap snapshots with git SHA filename and GC before capture"
  - "Pattern: dual-source RSS reporting labeled distinctly to avoid conflating Go-managed vs kernel views"
  - "Pattern: benchmark files that need package-level helpers declare them in _test.go siblings under package bench_test"

requirements-completed: [BENCH-04]

# Metrics
duration: 6min
completed: 2026-04-08
---

# Phase 9 Plan 05: Memory Profile Benchmark Summary

**BenchmarkMemory walks 4 D-06 scenarios with dual Go/kernel RSS reporting and per-scenario pprof heap snapshots, locking the v1.1 memory baseline for BENCH-04.**

## Performance

- **Duration:** ~6 min
- **Tasks:** 2
- **Files created:** 4
- **Files modified:** 0

## Accomplishments

- `BenchmarkMemory` captures four D-06 scenarios in a single `b.Loop` body:
  - `s1_idle` — daemon idle, no workspace
  - `s2_workspace` — after `activate_project` + LS readiness
  - `s3_first_call` — after first `find_references` invocation
  - `s4_post_100_calls` — after 100 additional `find_references` calls
- Dual RSS reporting: every scenario logs AND `b.ReportMetric`s both
  `rss_go_sys_bytes` (from `runtime.ReadMemStats.Sys`) and
  `rss_kernel_bytes` (from `rss.CurrentRSS`), distinctly labeled per
  Pitfall 7.
- `heapSnapshot` helper runs `runtime.GC()` then `pprof.WriteHeapProfile`
  and writes `test/bench/pprof/{scenario}-{git-short-sha}.pb.gz`,
  tolerating git-less CI caches via a `"nogit"` fallback.
- `.gitignore` in `test/bench/pprof/` excludes `*.pb.gz` globally and
  un-excludes `baseline-*.pb.gz`, enforcing Pitfall 8 while leaving a
  clean door for Plan 09-06's intentional re-baseline commits.
- Smoke run (`go test -bench=BenchmarkMemory -benchtime=1x`) produced all
  four `.pb.gz` artifacts and reported sane RSS pairs on darwin/arm64
  (idle: 14MB Go / 27MB kernel; post-100-calls: 19MB Go / 32MB kernel).

## Task Commits

1. **Task 1: heap_snapshot helper + pprof scaffolding** — `fd99ca35` (feat)
2. **Task 2: BenchmarkMemory with 4 D-06 scenarios + dual RSS** — `08aaa363` (feat)

## Files Created/Modified

- `test/bench/heap_snapshot_test.go` — `heapSnapshot(tb, scenario)` +
  `gitShortSHA(tb)` helpers with Pattern 5 GC-before-profile discipline.
- `test/bench/memory_bench_test.go` — `BenchmarkMemory`, `reportMemory`,
  `benchToolArgsFor` helpers and the load-bearing pprof artifact policy
  comment block at the top of the file.
- `test/bench/pprof/.gitkeep` — anchors the artifact directory in git so
  downstream plans can assume the path exists.
- `test/bench/pprof/.gitignore` — `*.pb.gz` excluded except
  `baseline-*.pb.gz` (Pitfall 8 mitigation).

## Decisions Made

- **Tool choice for scenarios 3/4:** The plan's example snippet called
  `find_symbol`, which does not exist in the 38-tool registry
  (`search_symbols` and `find_references` are the closest matches).
  `find_references` was chosen because it exercises the LS pipeline
  harder than `search_symbols` (cross-file reference scan), making the
  post-100-calls drift scenario more sensitive to per-call allocations.
  Args come from the shared `benchTools` manifest via `benchToolArgsFor`.
- **File suffix deviation:** The plan listed `heap_snapshot.go` and the
  task's package is `bench_test`. Go only permits `package bench_test`
  inside `_test.go` files, so the helper is a `_test.go` sibling
  (documented inline in both files).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Renamed `heap_snapshot.go` → `heap_snapshot_test.go`**
- **Found during:** Task 1 (helper scaffolding)
- **Issue:** Plan specified `test/bench/heap_snapshot.go` but the package
  is `bench_test`, which Go only compiles inside `_test.go` files. A
  non-test `.go` file declaring `package bench_test` fails to build.
- **Fix:** Created the helper as `heap_snapshot_test.go`. Behavior is
  identical because the helper is only consumed by `BenchmarkMemory`,
  which is itself a test-file benchmark. Documented inline at the top
  of both files.
- **Files modified:** `test/bench/heap_snapshot_test.go`
- **Verification:** `go vet ./test/bench/...` clean; smoke run of
  `BenchmarkMemory` produced all four expected snapshot files.
- **Committed in:** `fd99ca35`

**2. [Rule 3 - Blocking] Substituted `find_symbol` → `find_references` in scenarios 3/4**
- **Found during:** Task 2 (BenchmarkMemory body)
- **Issue:** Plan snippet referenced `find_symbol`, which is not in the
  38-tool registry or `benchTools` manifest; `benchToolArgsFor("find_symbol")`
  would panic at runtime.
- **Fix:** Switched to `find_references`, which is in the manifest, hits
  a known-offset `Helper` symbol in the Go fixture, and exercises more
  of the LS pipeline (better for drift detection).
- **Files modified:** `test/bench/memory_bench_test.go`
- **Verification:** Smoke run succeeded; all four scenarios emitted
  dual-RSS log lines and heap snapshots.
- **Committed in:** `08aaa363`

---

**Total deviations:** 2 auto-fixed (both Rule 3 blocking)
**Impact on plan:** Both deviations were mechanical corrections to typos
in the plan snippets. Semantics, artifacts, and acceptance criteria
unchanged; no scope creep.

## Issues Encountered

None beyond the two deviations above.

## User Setup Required

None — benchmark is self-contained and runs under `go test` with `gopls`
available on PATH.

## Next Phase Readiness

- BENCH-04 satisfied: Plan 09-06 (CI wiring) can now reference
  `BenchmarkMemory` for its `benchstat` + `upload-artifact` steps.
- Baseline artifact naming convention (`baseline-*.pb.gz`) is pre-wired
  in `.gitignore` so 09-06 can commit its captured baselines without any
  further .gitignore changes.
- `rss.CurrentRSS` path exercised end-to-end on darwin/arm64; Linux path
  validated transitively via the existing `test/bench/rss` unit tests.

## Self-Check: PASSED

- `test/bench/heap_snapshot_test.go` — FOUND
- `test/bench/memory_bench_test.go` — FOUND
- `test/bench/pprof/.gitkeep` — FOUND
- `test/bench/pprof/.gitignore` — FOUND
- Commit `fd99ca35` — FOUND (feat(09-05): heapSnapshot helper + pprof policy)
- Commit `08aaa363` — FOUND (feat(09-05): BenchmarkMemory + dual RSS)

---
*Phase: 09-benchmark-harness-v1-1-baseline*
*Completed: 2026-04-08*
