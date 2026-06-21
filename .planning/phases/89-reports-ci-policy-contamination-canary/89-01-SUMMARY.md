---
phase: 89-reports-ci-policy-contamination-canary
plan: 01
subsystem: bench/aggregator
tags: [bench, aggregator, report, render, determinism]
requires:
  - bench/aggregator/load.go rowMetrics.VerifiedCorrectness (*bool decoded at :32)
  - bench/aggregator/aggregate.go pooledRate / reduceCanaryRate / reduceSwebenchScores (pooled-rate precedent)
  - bench/aggregator/report.go ciOverlap / fmtCI / renderFooter / writeReport
  - bench/aggregator/aggregate.go bca + the single seeded rng (BCa determinism)
  - bench/languages (Tier-1 set derivation: 8 dirs, no `c`)
provides:
  - reduceVerifiedCorrectness (RNG-free pooled true-fraction over the decoded *bool)
  - LeaderRow.VerifiedCorrectness + LeaderRow.CostPerSolved (single-sourced) columns
  - renderPerLanguage (8 Tier-1 langs, n/a for no-coverage)
  - AblationRow + ablationComparisons + reduceAblations (aggregate-time full vs no_semantic) + renderAblations
  - ScatterPoint + renderScatter + buildScatterPoints (cost vs verified_correctness ASCII scatter)
  - regenerated leaderboard.golden.md / cost_quality.golden.md + new per_language.golden.md / ablations.golden.md / cost_quality_scatter.golden.md
affects:
  - 89-02 (canary exclusion + footnote: appends to these renderers)
  - 89-03 (helix-bench report --run-id, renderAll factoring, byte-reproducibility + lockstep guard updates)
tech-stack:
  added: []
  patterns: [zero-RNG-in-render, em-dash null discipline, sort-before-emit, pooled-rate additive reduce, fixed-grid deterministic ASCII plot]
key-files:
  created:
    - bench/aggregator/per_language_test.go
    - bench/aggregator/ablations_test.go
    - bench/aggregator/testdata/per_language.golden.md
    - bench/aggregator/testdata/ablations.golden.md
    - bench/aggregator/testdata/cost_quality_scatter.golden.md
  modified:
    - bench/aggregator/aggregate.go
    - bench/aggregator/report.go
    - bench/aggregator/report_test.go
    - bench/aggregator/aggregate_test.go
    - bench/aggregator/testdata/leaderboard.golden.md
    - bench/aggregator/testdata/cost_quality.golden.md
decisions:
  - "Tier-1 language set DERIVED from bench/languages: 8 IDs (cpp,csharp,go,java,javascript,python,rust,typescript); NO `c` (research 9-list was wrong)."
  - "reduceAblations threads the single seeded rng AFTER the per-mode leaderboard/cost loop, so the already-computed leaderboard/cost BCa CIs stay byte-identical and the ablation draws are last in a fixed order."
  - "full vs no_semantic computed at aggregate-time from the loaded full + no_semantic task_success vectors (deltas.go deliberately omits no_semantic as a delta operand)."
  - "cost_per_solved single-sourced from reduceCostRow output (no second cost reduce); leaderboard cost == cost_quality cost for the same row."
  - "renderCostQuality gained a scatter param; the 3 existing unit-test call sites pass nil so the existing table/footer bytes are untouched for non-Aggregate callers."
metrics:
  duration: ~35m
  completed: 2026-06-21
---

# Phase 89 Plan 01: Report Renderers (verified_correctness + per_language + ablations + cost/quality scatter) Summary

verified_correctness reduce + two new leaderboard columns, plus the `per_language.md`, `ablations.md`, and `cost_quality.md` ASCII-scatter renderers — all RNG-free, em-dash-honest, and proven by committed golden `.md` fixtures over the hermetic multi-run fixture tree.

## What Was Built

### REPORT-01 — verified_correctness reduce + leaderboard columns (Task 1)
- `reduceVerifiedCorrectness(loaded, tasks, mode)` (aggregate.go): pooled true-fraction over the decoded `*bool` (`load.go:32`), nil-skipped from both numerator and denominator, zero total → `ciValue{OK:false}` → em-dash. Consumes **no RNG** (mirrors `reduceCanaryRate`/`reduceSwebenchScores` via `pooledRate`).
- `LeaderRow` gains `VerifiedCorrectness ciValue` + `CostPerSolved ciValue`; `renderLeaderboard` emits both columns via `fmtCI`.
- `CostPerSolved` is **single-sourced** from the matching `reduceCostRow` output (no second cost reduce) — leaderboard cost == cost_quality cost (locked by `TestVerifiedCorrectness/leaderboard cost_per_solved equals cost_quality cost`).

### REPORT-02 — per_language.md (Task 2)
- `tier1Languages` fixed sorted var (8 IDs, derived from `bench/languages`, no `c`).
- `renderPerLanguage(byLang, footer)`: indexes `ByLanguage` into a map then iterates the fixed Tier-1 slice; a covered language renders `pass_rate %.4f` + `n`, a no-coverage language renders `n/a` (never omitted, never a fabricated 0). The `""`/non-Tier-1 buckets are intentionally not rendered.

### REPORT-03 — ablations.md with aggregate-time no_semantic (Task 2)
- `AblationRow` + fixed `ablationComparisons` (5: full vs no_lsp / no_semantic / no_structured_edit / baseline_plain / baseline_rag, canonical `your_agent_*`/`baseline_*` mode names).
- `reduceAblations` re-reduces each `(full, other)` task_success vector to a BCa CI via the shared `bca` helper and records presence; the **full vs no_semantic** pair is produced here (deltas.go omits it). `renderAblations` computes the point delta + reuses `ciOverlap` for the marker; an absent operand renders an em-dash.

### REPORT-04 — cost_quality.md ASCII scatter (Task 3)
- `ScatterPoint` + `renderScatter`: fixed 11×41 grid, `%.4f` rounding (`round4`), sort-before-plot, row-by-row deterministic emit (no map-order leak). Cost x-axis (min-max normalised) vs verified_correctness y-axis; a null cost/vc point is **not plotted**.
- `buildScatterPoints` joins cost (from `rep.Cost`) + verified_correctness (from `rep.Leaderboard`) by (mode×benchmark) — single-sourced.
- `renderCostQuality` gained a `scatter` param and appends the fenced block; the existing cost table, FAIR-03 variance section, and `valid_until` footer are preserved unchanged.

## TDD Gates (per task: RED → GREEN)
- Task 1: `test(89-01)` 4af6e56e → `feat(89-01)` bc53f31d
- Task 2: `test(89-01)` beb852e8 → `feat(89-01)` d2476d5c
- Task 3: `test(89-01)` 2ae819b9 → `feat(89-01)` 9bb35058

Each RED commit failed to compile (undefined fields/funcs); each GREEN commit made the new + existing tests pass.

## Golden Diffs (reviewed)
- **leaderboard.golden.md**: diff touched ONLY the two new columns (`verified_correctness` em-dash since the goldenFixture carries no verdict; `cost_per_solved` 0.0045/0.0150 == cost_quality). No reorder/precision drift in any existing column.
- **cost_quality.golden.md**: diff APPENDED only the scatter block ("no plottable points" — the goldenFixture has no verified_correctness verdict, so honestly nothing is plotted). Existing table/variance/footer bytes unchanged.
- **per_language.golden.md** (new): 8 Tier-1 rows, n/a for the 5 no-coverage langs.
- **ablations.golden.md** (new): all 5 comparisons incl. aggregate-time full vs no_semantic with ciOverlap markers.
- **cost_quality_scatter.golden.md** (new): a populated 2-point plot (+ 1 null-cost point correctly omitted), locking the grid layout.

## Determinism / Lockstep
- `reduceAblations` threads the single seeded rng LAST (after the leaderboard/cost loop), so the existing leaderboard/cost BCa CIs are byte-identical — `TestDeterministic` and `TestAggregateEndToEnd` stay green.
- The lockstep guard tests `TestSwebenchColumnsGoldenStable` and `TestAggregateCanaryAbsentIsEmDash` still pass (the new leaderboard columns are em-dash in the golden, consistent with their additive-perturbs-nothing intent). No changes needed to them this plan; 89-03 owns their final updates after the 89-02 canary footnote lands.

## Deviations from Plan
None — plan executed as written. The one mechanical addition beyond the plan's task list: a `cost_quality_scatter.golden.md` + `TestCostQualityScatterGolden` to lock the *populated* scatter grid (the goldenFixture's scatter is empty because it has no verified_correctness verdict). This is in the spirit of Task 3's "byte-stability assertion" and strengthens the proof; no behavior change.

## Authentication Gates
None.

## Known Stubs
None. All renderers are wired to real reduce outputs. The verified_correctness/scatter cells render em-dash / "no plottable points" over the existing fixture solely because that fixture carries no `verified_correctness` verdict (honest null discipline, not a stub) — a SWE-bench fixture with verdicts exercises the populated paths (locked by `TestVerifiedCorrectness` and `TestCostQualityScatterGolden`).

## Verification
- `go build ./...` — OK
- `go vet ./bench/...` — clean
- `make vet` — clean (incl. vet-ablation-leakage, vet-bench-rag-leakage)
- `go test ./bench/aggregator/... -count=1` — ok
- `go test ./bench/runtime/... -count=1` — ok
- No new external dependency (go.mod/go.sum unchanged; leaf-import discipline held).

## Notes for Downstream Plans
- The leaderboard/cost_quality goldens are now regenerated; **89-03** asserts their final byte-reproducibility after the **89-02** canary footnote lands, and owns the `renderAll` factoring + `helix-bench report --run-id` + lockstep guard updates.
- `renderCostQuality` now takes `(rows, scatter, footer)`; the `report` RunE / `renderAll` (89-03) should pass `buildScatterPoints(rep.Leaderboard, rep.Cost)`.

## Self-Check: PASSED
- All created files exist (per_language_test.go, ablations_test.go, 3 new goldens, SUMMARY.md).
- All 6 task commits exist (4af6e56e, bc53f31d, beb852e8, d2476d5c, 2ae819b9, 9bb35058).
