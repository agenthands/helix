---
phase: 10-observability-foundation
plan: 03
subsystem: observability
tags: [slog, benchmark, benchgate, obs, context-handler, allocations]

requires:
  - phase: 10-observability-foundation
    provides: "obs.NewContextHandler (Plan 10-01) — the wrapper under measurement"
  - phase: 09-benchmark-harness-v1-1-baseline
    provides: "test/bench/ package, v1.1-github-hosted.txt placeholder, benchgate binary"
provides:
  - "BenchmarkSlogHotPath_Baseline and BenchmarkSlogHotPath_WithContextHandler in test/bench/obs_bench_test.go"
  - "test/bench/baselines/v1.2-phase10-github-hosted.txt — Phase 11 reference baseline"
  - "Measured OBS-06 proof: 0 alloc/op delta on the no-span fast path"
affects: [phase-11-metrics, phase-12-tracing, obs-package]

tech-stack:
  added: []
  patterns:
    - "Additive bench file convention — new benches ship as their own *_bench_test.go, no existing file modified"
    - "Per-phase baseline file (v1.X-phaseN-*.txt) as the anchor for the next phase's delta gate"

key-files:
  created:
    - test/bench/obs_bench_test.go
    - test/bench/baselines/v1.2-phase10-github-hosted.txt
  modified: []

key-decisions:
  - "Baseline captured on local darwin/arm64 (Apple M4 Pro) as a placeholder; Phase 11 / CI will re-capture on ubuntu-latest during the two-step rollout"
  - "benchgate run against v1.1 placeholder exits 0 with 'no overlapping benchmarks' warning — that is the expected Phase 9 -> Phase 10 handshake until the first CI capture lands"
  - "OBS-06 delta measured as 0 allocs/op (not +1) because ContextHandler.Handle early-returns before touching the slog.Record — Plan 10-01 already got this right"

patterns-established:
  - "Hot-path benchmark pattern: slog.LogAttrs + typed attrs + io.Discard JSONHandler + b.Loop()/b.ReportAllocs() — zero IO noise, allocations reflect only handler work"
  - "Per-phase baseline file naming: test/bench/baselines/v{milestone}-phase{N}-{runner}.txt"

requirements-completed: [OBS-06]

duration: 6min
completed: 2026-04-08
---

# Phase 10 Plan 03: Slog Hot-Path Allocation Gate Summary

**OBS-06 measured and locked: obs.ContextHandler adds 0 alloc/op over a bare slog.JSONHandler on the no-span fast path, with a committed v1.2 baseline for Phase 11 to measure against.**

## Performance

- **Duration:** ~6 min
- **Started:** 2026-04-08
- **Completed:** 2026-04-08
- **Tasks:** 2
- **Files modified:** 2 (both created)

## Accomplishments

- Added dedicated slog hot-path benchmark pair (`BenchmarkSlogHotPath_Baseline` vs `BenchmarkSlogHotPath_WithContextHandler`) that exercises the exact emission shape the daemon runs — `slog.LogAttrs` with typed attrs through a JSON handler writing to `io.Discard`.
- Captured `-count=10` numbers on local darwin/arm64 and committed them as `test/bench/baselines/v1.2-phase10-github-hosted.txt` for Phase 11 to measure against.
- Ran `benchgate` (PR tier) against the Phase 9 `v1.1-github-hosted.txt` placeholder — exits 0 with the expected "no overlapping benches" informational warning.
- Proved the OBS-06 contract: delta is **0 allocs/op**, well inside the `<= +1 alloc/op` budget. The Phase 10 `spanContextFromContext` stub always returns `false`, so the ContextHandler fast path is genuinely zero-overhead (0 B/op, 0 allocs/op on both variants across all 10 iterations).

## Task Commits

Each task was committed atomically:

1. **Task 1: Add slog hot-path benchmarks** — `f54423b7` (test)
2. **Task 2: Capture v1.2 baseline + benchgate verification** — `81569734` (chore)

**Plan metadata:** (pending — this SUMMARY commit)

## Files Created/Modified

- `test/bench/obs_bench_test.go` — Two benchmarks in `bench_test` package; imports only `context`, `io`, `log/slog`, `testing`, and `github.com/postfix/serena/internal/obs`. Stdlib + internal/obs only (no new go.mod entries).
- `test/bench/baselines/v1.2-phase10-github-hosted.txt` — Phase 10 reference baseline: header block documenting host/Go version/delta, followed by verbatim `go test -bench -count=10` output for both benchmarks.

## Measured Numbers

```
BenchmarkSlogHotPath_Baseline-14              	~3.1M	~391 ns/op	0 B/op	0 allocs/op
BenchmarkSlogHotPath_WithContextHandler-14    	~3.1M	~388 ns/op	0 B/op	0 allocs/op
```

Delta: **0 allocs/op**, **~0 ns/op** (noise-level). The ContextHandler wrapper is effectively free on the no-span path, which is the only path Phase 10 ever takes.

## Decisions Made

- **Local capture is acceptable for the Phase 10 -> Phase 11 handshake.** Phase 9's `v1.1-github-hosted.txt` is still a placeholder (documented in its header) and the two-step rollout in `test/bench/baselines/README.md` explicitly expects the first real capture to happen in CI. The v1.2-phase10 baseline inherits that same two-step posture: committed now so Phase 11 has a file to compare against, re-captured by the first ubuntu-latest CI run that also re-baselines v1.1.
- **No modification of `v1.1-github-hosted.txt`.** Per README refresh policy, v1.1 is only touched by an intentional re-baseline PR. Confirmed `git diff test/bench/baselines/v1.1-github-hosted.txt` shows zero changes.
- **0 alloc/op delta (not 1) confirms Plan 10-01 was correct.** The early-return in `ContextHandler.Handle` before any `r.AddAttrs` call is what makes the fast path truly zero-alloc. No changes to `internal/obs/handler.go` were needed.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

- `go run ./test/bench/cmd/benchgate ...` unexpectedly left a `benchgate` binary in the repo root (looks like a go toolchain quirk with cmd-package `go run` invocations). Removed manually; not committed, not added to `.gitignore` since `go run` is not the standard way to invoke benchgate in CI. No impact on the plan.

## Next Phase Readiness

- OBS-06 closed with a committed, measured baseline. Phase 10 is ready to close out (all three plans complete: 10-01 handler, 10-02 admin listener, 10-03 allocation gate).
- Phase 11 (metrics) has a concrete reference file to compare against. When the first CI ubuntu-latest capture lands, both `v1.1-github-hosted.txt` and `v1.2-phase10-github-hosted.txt` should be re-captured together to establish real cross-phase numbers.
- No blockers for Phase 11.

## Self-Check: PASSED

- `test/bench/obs_bench_test.go` exists and contains `BenchmarkSlogHotPath_Baseline`, `BenchmarkSlogHotPath_WithContextHandler`, `obs.NewContextHandler`, `b.ReportAllocs`, `b.Loop`, `LogAttrs`. Verified: no `slog.Any` in file.
- `test/bench/baselines/v1.2-phase10-github-hosted.txt` exists and contains both benchmark names, "Phase 10", and "allocs/op".
- Commit `f54423b7` present in git log.
- Commit `81569734` present in git log.
- `test/bench/baselines/v1.1-github-hosted.txt` unchanged (`git diff` clean).
- `go test ./test/bench/ -run=^$ -bench=BenchmarkSlogHotPath -benchmem` ran successfully (both variants report 0 allocs/op).
- `go run ./test/bench/cmd/benchgate -baseline test/bench/baselines/v1.1-github-hosted.txt -new /tmp/phase10-slog.txt` exited 0.

---
*Phase: 10-observability-foundation*
*Completed: 2026-04-08*
