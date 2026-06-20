# Phase 82: Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-20
**Phase:** 82-multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-leaderboard
**Mode:** `--auto` (auto_advance chain from Phase 81 — no interactive prompts; recommended option auto-selected per question)
**Areas discussed:** Aggregator shape & invocation, N≥3 enforcement, BCa bootstrap design, pass@k estimator, Cost rollup, Leaderboard & honesty gate

---

## Aggregator shape & invocation

| Option | Description | Selected |
|--------|-------------|----------|
| New `bench/aggregator/` pure-function package over the run dir + `helix-bench aggregate` subcommand | Decoupled, mostly unit-testable without HELIX_BIN; supersedes inline deltas | ✓ |
| Extend `bench/runtime/deltas.go` in place | Reuses Phase 80 call site but couples aggregation to the matrix run | |
| Auto-run only at end of RunMatrix (no subcommand) | No re-runnable entry point | |

**Auto-selected:** New `bench/aggregator/` package + `helix-bench aggregate` subcommand (D-01/D-02/D-03). **Notes:** keeps the aggregator pure-functional over `reports/<run_id>/...` so most tests need no daemon; supersedes Phase 80's single-run `ComputeAndWriteDeltas`.

## N≥3 enforcement (STATS-01)

| Option | Description | Selected |
|--------|-------------|----------|
| Per-suite N, default 3; ExpandMatrix emits N cells; aggregator fail-closed if any cell < N | Matrix enforces; aggregator refuses partial reports | ✓ |
| Fixed N=3 hardcoded | Not configurable per-suite | |
| Aggregate whatever rows exist on disk | Hides partial matrices (unsafe) | |

**Auto-selected:** Configurable N default 3; `Cell.RunIndex 0..N-1`; fail-closed aggregator comparing against *expected* N from manifest/flag, not disk count (D-04/D-05).

## BCa bootstrap design (STATS-02)

| Option | Description | Selected |
|--------|-------------|----------|
| Proper BCa (z0 bias-correction + jackknife acceleration), ≥10k resamples, seeded, per-task-aggregate unit, 95% CI | Passes closed-form acceptance test; reproducible | ✓ |
| Percentile bootstrap only | Fails on skewed metrics | |
| Normal-approx CI | Not bootstrap; fails acceptance | |

**Auto-selected:** Full BCa with jackknife acceleration, 10,000-resample floor, deterministic seed, resampling over per-task aggregates, 95% default, null-safe (D-06–D-09).

## pass@k estimator (STATS-03)

| Option | Description | Selected |
|--------|-------------|----------|
| HumanEval unbiased `1 − C(n−c,k)/C(n,k)`, log-space | Matches published reference values | ✓ |
| Naive `1 − (1−p)^k` | Biased — FAILS the STATS-03 acceptance test | |

**Auto-selected:** HumanEval unbiased closed-form in log-space (D-10/D-11). **Notes:** flagged as the phase's #1 correctness pitfall — the naive estimator must not be used.

## Cost rollup (COST-02 / COST-03)

| Option | Description | Selected |
|--------|-------------|----------|
| usage×cost-table.yaml join by model_id; cost_per_solved_task = Σcost(solved)/count(solved); honor freshness gates; cache_write≈input rate (v1) | Matches hand-computed example; respects FAIR-02 staleness | ✓ |
| Ignore cached/cache-write token columns | Understates cost; breaks FAIR-03 intent | |
| Add a cache_write_per_mtok column now | Cost-table schema change = scope creep | |

**Auto-selected:** Fresh YAML cost helper, model_id join, freshness gates honored, cache_write priced at input rate with documented TODO (D-12–D-15).

## Leaderboard & honesty gate (STATS-04)

| Option | Description | Selected |
|--------|-------------|----------|
| (mode×benchmark) rows, internal-only, metrics+BCa CIs, CI-overlap warning suppresses X>Y claims | Honest first leaderboard | ✓ |
| Render superiority claims without CI-overlap check | Violates STATS-04 | |

**Auto-selected:** leaderboard.md + cost_quality.md with BCa CIs and the STATS-04 overlap gate; internal ToolBench-Go only (D-15/D-16/D-17).

## Claude's Discretion

- Exact Go package layout under `bench/aggregator/`, function signatures, and markdown table formatting.
- Whether multi-run deltas-with-CIs fully replace Phase 80's single-run deltas inline or live only in the aggregator output.

## Deferred Ideas

- External/public benchmark leaderboard rows (Phases 84–88; rendering Phase 89).
- `baseline_rag` arm (Phase 83).
- Precise `cache_write_per_mtok` cost-table column (follow-up).
- Broader fairness-warning reporting surface beyond cost_quality.md (Phase 89).
