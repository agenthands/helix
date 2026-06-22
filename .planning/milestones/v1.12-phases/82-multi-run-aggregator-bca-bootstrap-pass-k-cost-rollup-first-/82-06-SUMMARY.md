---
phase: 82-multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-leaderboard
plan: 06
subsystem: bench/aggregator
tags: [STATS-02, STATS-03, STATS-04, COST-03, aggregator, leaderboard, cost-quality, overlap-gate, fair-03, tdd]
requires:
  - aggregator.Load / Loaded / Row / rowMetrics (82-05 loader + N-gate)
  - aggregator.BCaInterval / StatMean (82-02 BCa bootstrap)
  - aggregator.PassAtK (82-03 unbiased pass@k)
  - aggregator.perResultUSD / costPerSolvedTask (82-04 cost rollup)
  - bench/cost.LoadCostTable / PriceFor / CostTable / CostRow (cost-table join)
  - bench/runtime.BuildResult / Validate (test fixtures)
provides:
  - aggregator.Aggregate(runDir, cfg Config) (*Report, error) — pure orchestrator
  - aggregator.Config / Report / LeaderRow / CostRow / VarianceFlag / Footer / ciValue
  - aggregator.renderLeaderboard / renderCostQuality (markdown render layer)
  - aggregator.ciOverlap (STATS-04 predicate) + coefVariation (FAIR-03 detector)
  - leaderboard.md + cost_quality.md byte-stable artifacts
affects:
  - bench/aggregator (new aggregate.go + report.go + tests + golden testdata)
  - bench/aggregator/load.go (added Loaded.Modes accessor)
tech-stack:
  added: []
  patterns:
    - "two-level reduction (D-07): Level 1 per (task,mode) scalar, Level 2 BCa across-task vector"
    - "ONE seeded *rand.Rand per Aggregate threaded into every BCaInterval (D-08 determinism)"
    - "fail-closed: Load deficiency -> return error, write nothing (D-05)"
    - "STATS-04 interval-intersection overlap predicate on adjacent sorted rows -> warn + suppress X>Y"
    - "FAIR-03 CV = stddev/mean of per-run USD > 0.05 -> named variance warning (A2)"
    - "null discipline: nil-across-all -> null CI -> em-dash, never fabricated 0"
    - "sort-before-emit everywhere + atomic temp+rename writeReport (cell.go:writeDurable analog)"
    - "golden .md diff lock with -update flag for regeneration"
key-files:
  created:
    - bench/aggregator/aggregate.go
    - bench/aggregator/report.go
    - bench/aggregator/aggregate_test.go
    - bench/aggregator/report_test.go
    - bench/aggregator/testdata/leaderboard.golden.md
    - bench/aggregator/testdata/cost_quality.golden.md
  modified:
    - bench/aggregator/load.go
decisions:
  - "Report holds BOTH Leaderboard ([]LeaderRow) and Cost ([]CostRow) plus a shared Footer; Aggregate builds it, the two renderers consume it — keeps the orchestrator pure and the render layer unit-testable with injected rows"
  - "FAIR-03 detector = coefficient of variation (sample stddev / mean, n-1) of per-run USD, threshold strictly > 0.05 (A2 cost-aligned reading); empty/zero-mean returns 0 (no NaN leak)"
  - "per-task USD = MEAN over the task's runs (A3); cost_per_solved_task = Σ over solved / count solved (costPerSolvedTask); cost BCa CI bootstraps the per-solved-task mean-USD vector"
  - "'solved' gate for cost_per_solved_task = cell success-rate > 0.5 (D-12 solved-task filter over the multi-run cell)"
  - "cost-table load failure degrades cost cells to null (em-dash) but never blocks the leaderboard; valid_until footer = earliest valid_until across rows (binding freshness horizon)"
  - "leaderboard sorted task_success DESC (mode tiebreaker); cost_quality sorted cost ASC (mode tiebreaker); null CIs sort last — total deterministic order"
  - "overlap warning rendered as a dedicated '## CI overlap warnings (STATS-04)' section keyed by mode pair so the X>Y suppression is explicit and greppable"
  - "added Loaded.Modes(task) accessor to load.go (Rule 3 — orchestrator needs to enumerate (mode x benchmark) rows; Load only exposed Rows/Tasks)"
metrics:
  duration: ~12m
  completed: 2026-06-21
  tasks: 3
  files: 7
---

# Phase 82 Plan 06: Aggregator Orchestrator + Leaderboard/Cost-Quality Reports Summary

The INTEGRATION plan: `Aggregate(runDir, cfg)` is a pure orchestrator that composes the Wave-1/2
primitives (Load, BCaInterval/StatMean, PassAtK, perResultUSD/costPerSolvedTask) into the two-level
reduction (D-07) and renders the milestone's first two observable artifacts — `leaderboard.md`
(STATS-02/03 CIs + STATS-04 overlap honesty gate) and `cost_quality.md` (COST-03 cost-per-solved CI +
FAIR-03 between-run variance warning) — fail-closed on a Load deficiency and byte-deterministic under a
single seeded RNG.

## What Was Built

- **`Aggregate(runDir string, cfg Config) (*Report, error)`** — pure orchestrator. Loads the durable
  tree (fail-closed: any deficient (task,mode) returns an error and writes NOTHING, D-05); performs the
  two-level reduction for every (mode x benchmark); seeds ONE `*rand.Rand` per call and threads it into
  every `BCaInterval`; renders + atomically writes both reports; returns the built `*Report`.
- **Two-level reduction (D-07)** — Level 1 per (task,mode): boolean `task_success` -> success-rate
  c/n; continuous metrics (tokens_input/output, tool_calls, files_read, edit_locality) -> mean over
  NON-NIL runs; pass@N -> `PassAtK(n, c, kN)`; cost -> mean USD over the task's priced runs. Level 2
  per (mode), per metric: `BCaInterval(acrossTaskVec, StatMean, iterations, alpha, rng)`. A metric nil
  across every run of every task -> empty vector -> null CI -> em-dash.
- **`Config`** — `{ExpectedN, Seed, Iterations(floor 10000), CILevel(default 0.95),
  KValues(default {1,N}), CostTablePath, Today}` with `withDefaults()` applying the documented floors.
- **`report.go` render layer** — `renderLeaderboard` (mode x benchmark sorted task_success desc, each
  metric `point [lo, hi]`, null -> em-dash, STATS-04 overlap section) and `renderCostQuality`
  (cost_per_solved_task CI, FAIR-03 variance section naming each flagged cell, valid_until footer).
  Shared types: `ciValue`, `LeaderRow`, `CostRow`, `VarianceFlag`, `Footer`, `Report`.
- **Honesty gates** — `ciOverlap(a,b)` is the interval-intersection predicate
  (`lo_a<=hi_b && lo_b<=hi_a`, null on either side -> false); adjacent sorted rows whose task_success
  CIs overlap render `⚠`-style warning lines that explicitly suppress the X>Y claim (D-17).
  `coefVariation` is the FAIR-03 CV detector (sample stddev/mean of per-run USD); CV>0.05 flags the cell.
- **Determinism** — sort-before-emit (rows, columns, footer fields all fixed order) + atomic
  temp+rename `writeReport` (mirrors `cell.go:writeDurable`, T-82-06-03). Golden `.md` testdata committed;
  `TestAggregateEndToEnd` diffs the rendered reports against them and `TestDeterministic` proves two
  seeded runs are byte-identical.
- **`load.go`** — added `Loaded.Modes(task) []string` (sorted) so the orchestrator can enumerate rows.

## Correctness Gate Compliance

- **Compose, not reimplement** — `Aggregate` calls `Load`, `BCaInterval`/`StatMean`, `PassAtK`,
  `perResultUSD`/`costPerSolvedTask`, `cost.LoadCostTable`/`PriceFor`. No BCa/pass@k/cost/load math is
  duplicated. Resampling unit = per-task aggregates (D-07); iterations floored to 10000; ONE seeded RNG
  threaded so reports are byte-deterministic (D-08).
- **STATS-04 CI-overlap gate** — `TestOverlapGate` exercises the predicate directly (overlap, disjoint,
  touching-endpoint, null cases) AND renders the warning for a synthetic-overlap fixture
  (full 0.70 [0.55,0.85] vs no_lsp 0.60 [0.45,0.75]) while confirming its ABSENCE for a non-overlap
  fixture (0.90 [0.85,0.95] vs 0.50 [0.40,0.60]). Injected-CI unit tests keep the assertion off
  bootstrap randomness.
- **D-15 FAIR-03 variance** — `TestCV` + `TestCostQualityRender` assert a high-variance cell renders a
  warning naming the cell and a low-variance cell renders none. The golden `cost_quality.md` flags
  `task-3 / no_lsp` (per-run USD CV 0.60).
- **Null discipline** — nil-across-all metric -> null CI -> em-dash (`TestAggregateNilMetricEmDash`,
  `TestCostQualityRender/null_cost_CI`), never a fabricated 0.
- **Internal ToolBench-Go only** — every row is `benchmark: internal-toolbench`; no external benchmarks.
- **Golden determinism** — `leaderboard.golden.md` + `cost_quality.golden.md` committed;
  `TestAggregateEndToEnd` (golden diff) + `TestDeterministic` (same seed => byte-identical) both pass.

## RED -> GREEN Evidence

1. **RED** (`test(82-06)`, 32ebb094): `go test ./bench/aggregator/` -> build failed, undefined
   `Aggregate`, `Config`, `Report`, `LeaderRow`, `CostRow`, `ciValue`, `Footer`. Plan's RED automated
   check (`grep -Eiq 'undefined|...|build failed'`) printed `RED-OK`.
2. **GREEN orchestrator** (`feat(82-06)`, 4ff7dc1b): implemented `aggregate.go` + `load.go` Modes
   accessor. `TestAggregate*` + `TestDeterministic` pass; `go vet` clean.
3. **GREEN render** (`feat(82-06)`, 44978ee5): implemented `report.go` + committed golden `.md`.
   Full `go test ./bench/aggregator/...` GREEN.

## Commands Run (project_test_gate — PURE package, NO HELIX_BIN)

- `go test ./bench/aggregator/...` -> `ok` (TestOverlapGate, TestLeaderboardRender, TestCV,
  TestCostQualityRender, TestAggregate{FailClosed,TwoLevelReduce,NilMetricEmDash,EndToEnd},
  TestDeterministic all RUN + PASS)
- `go build ./...` -> exit 0
- `go vet ./...` -> exit 0
- `make vet` -> exit 0 (base vet + 5 custom vettools)
- `go test ./...` -> no failures
- `gofmt -l bench/aggregator/*.go` -> clean
- Goldens regenerable via `go test ./bench/aggregator/ -run TestAggregateEndToEnd -update`

## Golden Output Confirmation

- `leaderboard.md`: `full` (task_success 1.0000 [1.0000,1.0000]) sorted above `no_lsp`
  (0.4444 [0.3333,0.5556]); pass@1 == task_success; pass@N (k=3) = 1.0000 for no_lsp (c<n -> any 3
  samples include a success, the unbiased estimator); footer records seed/iterations/ci_level/runs/
  cost_table_valid_until.
- `cost_quality.md`: cost_per_solved_task per mode with CI; FAIR-03 section flags
  `task-3 / no_lsp` (CV 0.6000 > 0.05); valid_until cited.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added `Loaded.Modes(task)` accessor to load.go**
- **Found during:** Task 2 (orchestrator implementation)
- **Issue:** the orchestrator must enumerate the (mode x benchmark) rows per task, but `Loaded` only
  exposed `Rows(task,mode)` and `Tasks()` — no way to list a task's modes.
- **Fix:** added a small additive `Modes(task) []string` accessor returning sorted mode IDs (nil-safe).
- **Files modified:** bench/aggregator/load.go
- **Commit:** 4ff7dc1b

## Threat Model Compliance

- **T-82-06-01** (unsupported X>Y claim) — `ciOverlap` predicate + adjacent-row warning section that
  suppresses the directional claim; synthetic overlap/non-overlap fixtures assert both directions.
- **T-82-06-02** (NaN/Inf or fabricated-0) — reuses BCaInterval/perResultUSD degenerate handling;
  null CI -> em-dash, `coefVariation` guards empty/zero-mean; deref helpers return (v,ok).
- **T-82-06-03** (torn markdown) — atomic temp-file + rename `writeReport` mirroring `cell.go:writeDurable`.
- **T-82-06-04** (non-byte-stable) — sort-before-emit + single seeded RNG; golden-diff + determinism tests.
- **T-82-06-SC** (installs) — none; stdlib + existing deps only.

## Self-Check: PASSED

- Files: aggregate.go, report.go, aggregate_test.go, report_test.go, leaderboard.golden.md,
  cost_quality.golden.md all FOUND; load.go modified.
- Commits: 32ebb094 (RED), 4ff7dc1b (GREEN orchestrator), 44978ee5 (GREEN render) all present in
  `git log`.

## TDD Gate Compliance

`test(82-06)` (RED) precedes both `feat(82-06)` commits (GREEN). No premature-pass: RED produced a
build failure on the undefined public surface, verified before any implementation. No separate refactor
commit was needed — the render helpers were factored cleanly during the GREEN render commit.
