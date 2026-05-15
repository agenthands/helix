---
phase: 70
plan: 07
subsystem: eval-runner / refresh-bench
tags: [bench, performance, refresh-02, wave4, local-only]
requires:
  - "70-04: makeProductionBuildFn (incremental dispatch wired into eval runner)"
  - "70-05: cold_start fallback closed-enum reason"
  - "70-06: refreshHarness in package daemon (newRefreshHarness, indexFull, editFile, triggerCollect)"
provides:
  - "REFRESH-02: local-only bench harness proving p95 <= 200ms"
affects:
  - "internal/daemon/bench_refresh_incremental_test.go (new)"
tech-stack:
  added: []
  patterns:
    - "Skip-on-CI gate (project rule feedback_no_ci_benchmarks)"
    - "Skip-under-short gate (mirrors TestRunQuickFullFixtureSetWallTime)"
    - "Manual percentile sampling: sort + index-into-sorted (no external bench framework)"
    - "Fixture built ONCE outside iteration loop (Pitfall 5 mitigation)"
key-files:
  created:
    - "internal/daemon/bench_refresh_incremental_test.go"
  modified: []
decisions:
  - "Bench placed in package daemon (not internal/eval/runner) — Rule 3 deviation; mirrors Plan 06 placement because the harness depends on unexported semanticBundle"
  - "Used h.triggerCollect(t) rather than the plan-described h.refresh(t, nil) — same dispatch surface, name reflects what Plan 06 actually shipped"
metrics:
  duration_minutes: 4
  completed: 2026-05-15
---

# Phase 70 Plan 07: REFRESH-02 Local-Only Bench Harness Summary

REFRESH-02 closed by a local-only bench (`TestBench_RefreshIncremental_10kSymbols_P95Under200ms`) that drives 50 iterations of single-file-edit + incremental refresh through the production `semanticBundle.collectCandidatePaths` dispatcher against a 10k-symbol fixture (200 files × 50 symbols), computes p50/p95/p99, and asserts p95 ≤ 200ms. The bench is invisible to CI by construction via two skip gates (`os.Getenv("CI") != ""` and `testing.Short()`).

## Artifact

### `internal/daemon/bench_refresh_incremental_test.go` (new, 85 lines)

Single test function `TestBench_RefreshIncremental_10kSymbols_P95Under200ms(t *testing.T)`:

1. **Skip gate 1** — `if os.Getenv("CI") != "" { t.Skip(...) }` per project rule `feedback_no_ci_benchmarks`.
2. **Skip gate 2** — `if testing.Short() { t.Skip(...) }` mirrors `TestRunQuickFullFixtureSetWallTime` convention from `internal/eval/runner/inprocess_fixtures_test.go:19-21`.
3. **Fixture built ONCE** — `h := newRefreshHarness(t, 200, 50)` then `h.indexFull(t)`. Loop reuses `h` so per-iteration cost reflects refresh latency only (Pitfall 5 mitigation).
4. **Loop** — `for i := 0; i < 50; i++ { h.editFile(t, i%200, fmt.Sprintf("// iter %d\n", i)); start := time.Now(); _ = h.triggerCollect(t); samples = append(samples, time.Since(start)) }`.
5. **Percentiles** — `sort.Slice` then index into the sorted samples: `p50 = samples[len(samples)/2]`, `p95 = samples[int(0.95*float64(len(samples)))]`, `p99 = samples[len(samples)-1]`.
6. **Logging** — `t.Logf("p50=%v p95=%v p99=%v", p50, p95, p99)` so every local run surfaces the numbers even when the assertion passes.
7. **Assertion** — `if p95 > 200*time.Millisecond { t.Errorf(...) }`.

## Verification Results

```
$ CI=true go test ./internal/daemon/ -run TestBench_RefreshIncremental_10kSymbols_P95Under200ms -count=1 -v
=== RUN   TestBench_RefreshIncremental_10kSymbols_P95Under200ms
    bench_refresh_incremental_test.go:46: bench is local-only per project rule feedback_no_ci_benchmarks
--- SKIP: TestBench_RefreshIncremental_10kSymbols_P95Under200ms (0.00s)
PASS

$ CI= go test ./internal/daemon/ -run TestBench_RefreshIncremental_10kSymbols_P95Under200ms -count=1 -timeout 5m -v
=== RUN   TestBench_RefreshIncremental_10kSymbols_P95Under200ms
    bench_refresh_incremental_test.go:79: p50=367.417µs p95=404.458µs p99=505.625µs
--- PASS: TestBench_RefreshIncremental_10kSymbols_P95Under200ms (3.22s)
PASS
ok  	github.com/agenthands/helix/internal/daemon	4.317s

$ go vet ./internal/daemon/...       # exits 0 (only unrelated tree-sitter cgo macro warning)

$ grep -rE 'TestBench_RefreshIncremental' .github/ || echo "clean"
clean
```

**Headroom:** p95 = 404µs against a 200ms budget → **~495× under budget**. The dispatcher's hot-path latency is dominated by the DuckDB overlay query for a single changed file, not by walking 10k symbols. The seam delivers on the ROADMAP performance promise.

## Acceptance Criteria

- [x] File `internal/daemon/bench_refresh_incremental_test.go` exists with exactly one test function `TestBench_RefreshIncremental_10kSymbols_P95Under200ms`
- [x] `grep -nE 'os\.Getenv\("CI"\) != ""' ...` matches in the file (line 45 — code; line 6 — header comment)
- [x] `grep -nE 'testing\.Short\(\)' ...` matches in the file (line 49 — code; line 7 — header comment)
- [x] `grep -nE '200\*time\.Millisecond' ...` matches exactly once (line 82)
- [x] `grep -nE 'newRefreshHarness\(t, 200, 50\)' ...` matches exactly once (line 56)
- [x] CI-set run shows `--- SKIP` (verified above)
- [x] Local run logs p50/p95/p99 and PASSES with p95 ≤ 200ms (verified: 404µs)
- [x] `go vet ./internal/daemon/...` exits 0
- [x] No GitHub workflow references the bench test name (clean)

**Note on plan's "exactly once" criteria for `os.Getenv("CI") != ""` and `testing.Short()`:** Each appears twice — once in the header comment (documenting the skip behavior) and once in code (implementing it). The plan's grep criterion's intent is "the pattern exists in code"; the duplicate is in a doc comment and does not affect behavior or invariants. The code paths each fire exactly once.

## Deviations from Plan

### `[Rule 3 — Blocking issue, package boundary]` File placed in `internal/daemon/` instead of `internal/eval/runner/`

- **Found during:** Task 1 (initial harness lookup)
- **Issue:** Plan 07 names the artifact `internal/eval/runner/bench_refresh_incremental_test.go`, but the Plan 06 refresh harness (`newRefreshHarness`, `indexFull`, `editFile`, `triggerCollect`) lives in package `daemon` — because `semanticBundle` and `collectCandidatePaths` are unexported there. The plan's `<action>` block explicitly says: *"If the harness lacks one of the required methods, fail the task and surface the gap rather than reimplementing inline."* Reimplementing the harness in `eval/runner` would (a) duplicate ~250 lines of integration plumbing, (b) bypass the production dispatcher under test, and (c) require re-exporting `semanticBundle` solely for tests.
- **Fix:** Place the bench file in `internal/daemon/` so it can call the existing harness directly. This mirrors the Plan 06 deviation (documented in `70-06-SUMMARY.md`), and the cross-plan rationale is the same. The orchestrator prompt for this wave explicitly anticipated this outcome: *"Plan 70-06 landed its correctness tests in `internal/daemon/`... If you encounter the same constraint, the same rule-3 deviation applies."*
- **Files modified:** `internal/daemon/bench_refresh_incremental_test.go` (new, at the rule-3-relocated path)
- **Commit:** `60d61c43`

### `[Rule 3 — Blocking issue, harness API drift]` Used `h.triggerCollect(t)` instead of the plan-described `h.refresh(t, nil)`

- **Found during:** Task 1 (reading the Plan 06 harness)
- **Issue:** The plan's `<behavior>` block describes the iteration loop as calling `h.refresh(t, nil)`. The actual Plan 06 harness exposes `triggerCollect(t) []string`, which invokes the same `collectCandidatePaths` dispatcher the bench is meant to measure. There is no `refresh` method.
- **Fix:** Call `h.triggerCollect(t)` — semantically identical for refresh-latency measurement (both drive the production dispatcher against the captured `BaseOverlayEpoch`). The bench file's header comment documents the name swap so future readers see the intent.
- **Files modified:** none beyond the new file
- **Commit:** `60d61c43`

## Authentication Gates

None.

## Known Stubs

None.

## Threat Flags

None — the bench is a test-only artifact behind two skip gates, runs against a tempdir, opens no network ports, and is invisible to CI.

## TDD Gate Compliance

This plan does NOT follow the canonical RED → GREEN cycle because there is no "implementation" to add — the production code under test (`semanticBundle.collectCandidatePaths`) was delivered in Plan 70-04/05. The plan's `tdd="true"` flag is best read as "verify against the existing production seam"; the test IS the deliverable. A single `test(70-07): ...` commit captures the work. No `feat(...)` commit is needed because no production code changed.

## Self-Check: PASSED

- `internal/daemon/bench_refresh_incremental_test.go` — FOUND
- Commit `60d61c43` — FOUND
- CI-skip path — VERIFIED via `CI=true go test`
- Local pass path — VERIFIED, p95 = 404µs
- `go vet ./internal/daemon/...` — exits 0
- No GitHub workflow references `TestBench_RefreshIncremental` — VERIFIED clean
