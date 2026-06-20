---
phase: 82-multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-leaderboard
plan: 04
subsystem: bench
tags: [cost, cost-table, freshness-gate, tdd, go, bench-aggregator, COST-02]

# Dependency graph
requires:
  - phase: 82-01
    provides: "bench/cost package — CostTable, LoadCostTable, PriceFor (D-13 fail-closed freshness gate), CostRow, DateLayout, StalenessWindowDays"
  - phase: 79
    provides: "evaluators.Metrics nullable token contract (TokensInput/Output/InputCachedRead/InputCacheWrite *int); aggregator rowMetrics mirror in load.go"
provides:
  - "bench/aggregator/cost.go: perResultUSD(rowMetrics, cost.CostRow) (float64, bool) — D-12/D-14 per-result USD"
  - "bench/aggregator/cost.go: costPerSolvedTask(map[string]float64) (float64, bool) — COST-02 headline reducer"
  - "Golden + stale cost-table fixtures under bench/aggregator/testdata/"
affects: [82-06, leaderboard, cost_quality.md]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Cost rollup delegates parsing + freshness gate to bench/cost (one source of truth, Open Q1) — no duplicate YAML decoder or gate in the aggregator"
    - "Injected-clock determinism for the D-13 freshness gate in tests (fixed today=2026-06-20; fresh fixture passes, stale fixtures trip)"

key-files:
  created:
    - bench/aggregator/cost.go
    - bench/aggregator/cost_test.go
    - bench/aggregator/testdata/cost-table.golden.yaml
    - bench/aggregator/testdata/cost-table.past-valid-until.yaml
    - bench/aggregator/testdata/cost-table.stale-verified.yaml
  modified: []

key-decisions:
  - "Reused the existing rowMetrics struct (load.go) as the metricView rather than defining a new struct — its four nullable token pointers are exactly the cost inputs, avoiding a parallel type."
  - "cache-write priced at input_per_mtok (D-14 v1 approximation) with the exact // TODO(D-14) comment; no cost-table schema expansion."
  - "Freshness gate fully delegated to cost.PriceFor — the aggregator carries no date/staleness logic of its own."

patterns-established:
  - "perResultUSD null discipline: per-term nil-skip; all-nil token set returns (0,false) = no computable cost, never fabricated $0 (Pitfall 4)."
  - "Reducer-takes-already-reduced-map: costPerSolvedTask consumes a per-solved-task USD map; the per-task mean-over-N reduction is Plan 06's job."

requirements-completed: [COST-02]

# Metrics
duration: 4min
completed: 2026-06-21
---

# Phase 82 Plan 04: Cost-per-Solved-Task Rollup (COST-02) Summary

**Per-result USD and `cost_per_solved_task` in `bench/aggregator`, joining canonical nullable token metrics to the `bench/cost` table through its fail-closed D-13 freshness gate; anchored to the hand-computed 3.555 USD golden.**

## Performance

- **Duration:** ~4 min
- **Started:** 2026-06-20T22:04:18Z
- **Completed:** 2026-06-21
- **Tasks:** 2 (TDD: RED + GREEN)
- **Files created:** 5

## Accomplishments
- `perResultUSD(rowMetrics, cost.CostRow) (float64, bool)` implementing the D-12/D-14 USD formula with per-term nil-skip and all-nil exclusion.
- `costPerSolvedTask(map[string]float64) (float64, bool)` — sum/count over solved tasks; empty map -> (0,false), never NaN.
- cache-write priced at the standard input rate with the exact `// TODO(D-14)` comment (no schema expansion).
- Cost-table load + freshness gate delegated entirely to `bench/cost` (no duplicate parser/gate — grep-confirmed).
- Golden fixture (`cost-table.golden.yaml`) plus two gate-tripping fixtures (past `valid_until`, >90d-stale `last_verified`).

## Task Commits

1. **Task 1: RED — COST-02 golden + freshness + nil-exclusion tests** — `96714ff3` (test)
2. **Task 2: GREEN — implement cost.go (USD formula + rollup, reusing bench/cost.PriceFor)** — `d2c8c924` (feat)

_REFACTOR: none needed — GREEN implementation was already clean (gofmt + vet clean, no restructuring)._

## Files Created/Modified
- `bench/aggregator/cost.go` - perResultUSD + costPerSolvedTask; imports bench/cost; D-14 cache-write TODO.
- `bench/aggregator/cost_test.go` - TestCostPerSolved (3.555 golden, solved-only, nil-exclusion, partial-nil) + TestCostFreshnessGate (past valid_until / stale / unknown model_id).
- `bench/aggregator/testdata/cost-table.golden.yaml` - frozen rate mirroring the committed COST-02 row (fresh vs today=2026-06-20).
- `bench/aggregator/testdata/cost-table.past-valid-until.yaml` - past valid_until fixture.
- `bench/aggregator/testdata/cost-table.stale-verified.yaml` - >90d-stale last_verified fixture.

## Correctness Gate Evidence

**RED→GREEN:**
- RED (before cost.go): `go test ./bench/aggregator/ -run 'TestCostPerSolved|TestCostFreshnessGate'` → `undefined: perResultUSD`, `undefined: costPerSolvedTask`, `FAIL [build failed]` (committed 96714ff3 before any implementation).
- GREEN (after cost.go): all 7 subtests PASS.

**Golden assertion (within 1e-9):**
- T1 {ti=1_000_000, cr=0, cw=0, to=100_000} → USD 4.50 ✓
- T2 {ti=500_000, cr=200_000, cw=100_000, to=50_000} → USD 2.61 (cache-write 100_000×3.0/1e6 = 0.30 at input rate, D-14) ✓
- cost_per_solved_task = (4.50 + 2.61)/2 = **3.555** ✓

**Other invariants asserted:**
- No solved tasks → `(0, false)`, not NaN (verified `!math.IsNaN`).
- Fully-nil tokens → `perResultUSD` `ok==false`, value 0.0 marked absent (never fabricated $0).
- Partial-nil (only tokens_output) → prices only the present term (1.50) with ok==true.
- Freshness gate fail-closed: past valid_until → error containing `valid_until`; stale last_verified → error containing `last_verified`; unknown model_id → error containing `no row for model_id`.

**Commands run (all clean):**
- `go build ./...` → exit 0
- `go vet ./...` → exit 0
- `make vet` (incl. all custom vettools) → exit 0
- `go test ./bench/aggregator/... -run TestCostPerSolved -v` → RUN + PASS (4 subtests)
- `go test ./bench/aggregator/ -run 'TestCostPerSolved|TestCostFreshnessGate' -v` → 7/7 PASS
- `go test ./...` → exit 0 (no failures)
- `gofmt -w` applied to both touched Go files.

**One-source-of-truth check:** `grep -rn "yaml.NewDecoder|StalenessWindowDays|ValidateCostTable|func.*PriceFor" bench/aggregator/*.go` → NONE; `bench/cost` imported in cost.go. No HELIX_BIN used (pure package, testdata fixtures).

## Decisions Made
See `key-decisions` frontmatter. Notably: reused `rowMetrics` (load.go) instead of introducing a new `metricView`; the plan offered either path and the existing struct already exposes exactly the four token pointers.

## Deviations from Plan
None - plan executed exactly as written. (Plan suggested an optional new `metricView` struct; reusing the existing `rowMetrics` is the plan's stated alternative — "reuse the nullable token pointers directly" — not a deviation.)

## Issues Encountered
None.

## TDD Gate Compliance
- RED gate: `test(82-04): ...` commit `96714ff3` (build-failed/undefined before implementation).
- GREEN gate: `feat(82-04): ...` commit `d2c8c924` after RED.
- REFACTOR: not required (no commit).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `perResultUSD` and `costPerSolvedTask` are ready for Plan 06 to wire: Plan 06 supplies the per-solved-task USD map (mean USD over each task's N runs, A3) and the cost_quality.md BCa CI over the per-task USD vector.
- The cost column renders `—` for usage-absent (scripted) runs — correct per RESEARCH, not a bug.

---
*Phase: 82-multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-leaderboard*
*Completed: 2026-06-21*

## Self-Check: PASSED
- All 5 created files exist on disk.
- Both commits present in git history: 96714ff3 (RED test), d2c8c924 (GREEN feat).
