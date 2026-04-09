---
phase: 09
plan: 04
subsystem: test/bench
tags: [benchmark, lsp, indexing, gopls, cold-start, warm, fullrepo, pitfall-4, pitfall-11]
requires: [09-02]
provides:
  - BenchmarkLSPIndex_Cold (per-iteration daemon, -benchtime=1x trend metric)
  - BenchmarkLSPIndex_Warm (shared daemon, 3-call warmup, steady-state gated)
  - BenchmarkFullRepoSmoke (short-gated release-only full Serena indexing smoke)
affects: [test/bench/...]
tech_stack:
  added: []
  patterns:
    - "Pattern 4: cold-vs-warm LSP benchmarks (09-RESEARCH.md lines 216-251)"
    - "b.StopTimer/StartTimer fencing for per-iteration setup/teardown"
    - "testing.Short() gate for long-tail full-repo smoke (Pitfall 11)"
key_files:
  created:
    - test/bench/lsp_index_bench_test.go
    - test/bench/fullrepo_smoke_test.go
  modified: []
decisions:
  - "Warm bench reuses search_symbols with query=Helper (same as activateWorkspaceB readiness poll) to avoid manifest drift"
  - "Cold bench documented as trend metric only — gate lives on warm bench (Pitfall 4)"
  - "Full-repo smoke uses repoRoot() walking up from runtime.Caller(0) to locate go.mod, not a fragile relative path"
metrics:
  duration: ~8min
  completed: 2026-04-08
  tasks: 2
  files: 2
requirements: [BENCH-03]
---

# Phase 9 Plan 04: LSP Indexing Benchmarks (Cold + Warm + Full-Repo Smoke) Summary

BENCH-03 cold/warm LSP indexing plus D-05 full-Serena codebase smoke shipped as three `b.Loop`-style benchmarks in `test/bench/`, with Pitfall 4 (gopls cold-start dominance) and Pitfall 11 (full-repo CI budget) explicitly mitigated via documentation and short-gating.

## What Shipped

**Task 1 — Cold + warm LSP indexing benchmarks (`test/bench/lsp_index_bench_test.go`)** — commit `84526552`
- `BenchmarkLSPIndex_Cold` follows Pattern 4 exactly: each `b.Loop()` iteration calls `startBenchDaemon` inside the body, fences it with `b.StopTimer/StartTimer`, activates the workspace (the measured cold index), then stops and tears the daemon down. Intended for manual `-benchtime=1x` runs only and documented as a trend metric, not a gate.
- `BenchmarkLSPIndex_Warm` uses a single pre-warmed daemon: activate once, issue 3 warmup `search_symbols` calls per phase Q6, then `b.Loop()` reissues the same call. Warm args (`query=Helper`) deliberately match the readiness poll in `activateWorkspaceB` so no separate fixture symbol lookup can drift.
- Smoke-run numbers on this dev machine (Apple M4 Pro, darwin/arm64, `-benchtime=1x`): Cold ≈ 195 ms/op with ~1.06 MB allocs, Warm ≈ 3.9 ms/op with 324 kB allocs. Matches expected order-of-magnitude gap between cold-start and steady-state.

**Task 2 — Full Serena codebase indexing smoke (`test/bench/fullrepo_smoke_test.go`)** — commit `d265c017`
- `BenchmarkFullRepoSmoke` indexes the ~25,779-LOC repo root using the same per-iteration daemon fencing pattern as the cold bench.
- Short-gated with `if testing.Short() { b.Skip(...) }` per Pitfall 11 so default PR sweeps never run it.
- `repoRoot(tb)` helper walks up from `runtime.Caller(0)` to the nearest `go.mod`, fatal-ing if it runs out of parents — fails loudly rather than silently pointing at the wrong directory if the file is ever relocated.
- T-09-08 (CI runtime DoS) mitigated: release-tag-only execution is called out in the benchmark doc comment.

## Verification

- `go vet ./test/bench/...` — clean
- `go test -bench='BenchmarkLSPIndex_(Cold|Warm)' -benchtime=1x -run=^$ -count=1 ./test/bench/...` — both benchmarks produce nonzero ns/op and pass
- `go test -bench=^$ -run=^$ -count=1 ./test/bench/...` — compile-only dry run for the full-repo smoke, passes without actually running the 15-30s indexing loop
- Acceptance counts: `grep -c "for b\.Loop()"` = 2 in lsp_index_bench_test.go; `grep -c "b\.StopTimer\|b\.StartTimer"` = 6 (exceeds the minimum of 4)

## Deviations from Plan

**1. [Rule 3 — Blocking] Warm tool call uses `search_symbols`, not `find_symbol`**
- **Found during:** Task 1
- **Issue:** The plan template literally referenced a `find_symbol` tool, but Serena's 38-tool surface has no tool by that name. The closest retrieval tools are `search_symbols` (query-based) and `go_to_definition` (position-based).
- **Fix:** Used `search_symbols` with `{"query": "Helper"}` — the same call `activateWorkspaceB` uses for LS readiness detection, which guarantees a matching fixture symbol exists and eliminates drift against the Plan 02 manifest.
- **Files modified:** `test/bench/lsp_index_bench_test.go`
- **Commit:** `84526552`

No architectural changes, no auth gates, no further deviations.

## Threat Flags

None — the plan's `<threat_model>` (T-09-08 only) was fully mitigated via the existing `testing.Short()` gate, and no new security-relevant surface was introduced.

## Self-Check: PASSED

- FOUND: `test/bench/lsp_index_bench_test.go`
- FOUND: `test/bench/fullrepo_smoke_test.go`
- FOUND commit: `84526552` (Task 1)
- FOUND commit: `d265c017` (Task 2)
