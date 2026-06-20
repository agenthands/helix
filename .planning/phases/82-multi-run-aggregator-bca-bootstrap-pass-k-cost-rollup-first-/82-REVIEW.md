---
phase: 82-multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-
reviewed: 2026-06-21T00:00:00Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - bench/aggregator/bootstrap.go
  - bench/aggregator/bootstrap_test.go
  - bench/aggregator/passk.go
  - bench/aggregator/passk_test.go
  - bench/aggregator/cost.go
  - bench/aggregator/load.go
  - bench/aggregator/aggregate.go
  - bench/aggregator/report.go
  - bench/aggregator/report_test.go
  - bench/aggregator/aggregate_test.go
  - bench/cost/cost_table.go
  - bench/runtime/matrix.go
  - cmd/helix-bench/aggregate.go
  - cmd/helix-bench/main.go
  - cmd/helix-bench/validate_cost_table.go
findings:
  critical: 1
  warning: 3
  info: 3
  total: 7
status: issues_found
---

# Phase 82: Code Review Report

**Reviewed:** 2026-06-21
**Depth:** standard
**Files Reviewed:** 14 (+ adjacent tests consulted)
**Status:** issues_found

## Summary

Phase 82 adds the `bench/aggregator` package (multi-run aggregator: BCa bootstrap CIs,
unbiased pass@k, cost rollup, leaderboard/cost-quality renderers), moves the cost-table
contract into `bench/cost`, and adds the `helix-bench aggregate` subcommand. The
statistical core is, in the main, correct and well-tested:

- **pass@k is the correct HumanEval unbiased estimator** — the product runs `i = n-c+1 .. n`
  (exactly `c` terms, NOT `k`), `PassAtK(10,3,5) = 11/12 = 0.91667` is locked, anti-naive and
  form-agreement (lgamma) cross-checks are present. Verified against the closed form.
- **BCa is a genuine BCa, not a percentile bootstrap** — z0 via `math.Erfinv`, jackknife
  acceleration, ≥10,000 seeded resamples, degenerate matrix (empty/all-identical/m==1) handled,
  the fake-BCa discriminator test is real. Verified the divergence test cannot be passed by a
  percentile-only impl.
- **Cost move is clean** — the cost-table types/validator were MOVED to `bench/cost`
  byte-for-byte (diffed against `dccf8352~1`); no duplicate/divergent definition remains in
  package main (`aggregator.CostRow` is an unrelated report-row type, not a clone of
  `cost.CostRow`). The USD formula, solved-task reduction, nil-token discipline, and
  fail-closed freshness gate are all correct.
- All package tests pass (`go test ./bench/aggregator/ ./bench/cost/ ./bench/runtime/`).

The review nonetheless surfaces one **fail-open hole in the supposedly fail-closed N-gate**
(CR-01, proven by a throwaway test) and one **numeric robustness gap in BCa endpoint ordering**
(WR-01, proven by enumeration). Because this is an externally-publishable artifact, both bear
directly on "a wrong/empty number ships."

## Critical Issues

### CR-01: Empty / wrong run directory FAILS OPEN — the N-gate is bypassed by zero-discovery

**File:** `bench/aggregator/load.go:115-157`, `bench/aggregator/aggregate.go:64-118`
**Issue:** The N-gate only fails on cells it *discovers*. If `globRows` discovers **zero**
cells — an empty `runDir`, a non-existent `runDir`, a wrong path, or a tree whose run-index
segments are all non-numeric — then `byTask` is empty, `deficientCells` returns an empty slice
(it ranges over nothing), and `Load` returns a valid `&Loaded{}` with **no error**.
`Aggregate` then renders and writes empty `leaderboard.md` + `cost_quality.md` and exits 0.

`filepath.Glob` returns `(nil, nil)` for a non-existent directory, so a typo'd or missing
`runDir` is indistinguishable from a clean run — the exact "partial/empty matrix silently
accepted" failure the fail-closed gate (D-05, Pitfall 3) exists to prevent.

Proven with a throwaway test against an empty `t.TempDir()`:
```
err=<nil> rep=&{Leaderboard:[] Cost:[] ...}
FAIL-OPEN: empty runDir wrote leaderboard.md with no data
```
For a published artifact, "exits 0 and writes an empty leaderboard" is worse than failing —
an operator pointing `helix-bench aggregate` at the wrong directory gets a clean, empty,
authoritative-looking report.

**Fix:** Treat zero discovery as a hard error in `Load` (the gate, not the renderer):
```go
// after grouping, before deficientCells:
if len(byTask) == 0 {
    return nil, fmt.Errorf("aggregate: no result.v2.json rows discovered under %q "+
        "(expected <task>/<mode>/<run_index>/result.v2.json)", runDir)
}
```
Optionally also stat `runDir` up front in `globRows` and error if it does not exist, so a
missing path is named distinctly from a present-but-empty one.

## Warnings

### WR-01: BCa percentile endpoints can invert (lo > hi) under extreme z0/a — no ordering guard

**File:** `bench/aggregator/bootstrap.go:113-119, 184-194`
**Issue:** `bcaPercentiles` returns `(a1, a2)` and `BCaInterval` reads `lo = quantile(.., a1)`,
`hi = quantile(.., a2)` with **no guarantee a1 ≤ a2**. Under a large bias-correction `z0`
combined with a sizable acceleration `a`, the adjusted percentiles cross, yielding `lo > hi`.
Enumerated examples (alpha=0.05):
```
z0=-1.00 a=-0.50 -> a1=1.0000 a2=0.3627   (inverted)
z0= 2.00 a= 1.00 -> a1=0.9794 a2=0.7461   (inverted)
z0=-3.00 a=-0.50 -> a1=0.6373 a2=0.0000   (inverted)
```
These z0 magnitudes are reachable in practice: with few tasks (small m) and a discrete,
skewed bootstrap-of-means distribution, a single replicate below θ̂ over B=10,000 gives
p0=0.0001 → z0 ≈ −3.7. The degenerate guard only catches `p0 ∈ {0,1}` (bootstrap.go:104), not
`p0 = 0.0001`. The result is a published CI rendered as `point [lo, hi]` with `lo > hi` — a
visibly wrong number in the leaderboard.

**Fix:** Order the endpoints after reading them (cheap, preserves determinism):
```go
lo = quantile(thetaStar, a1)
hi = quantile(thetaStar, a2)
if lo > hi {
    lo, hi = hi, lo
}
return lo, hi, true
```
Consider also asserting `lo ≤ θ̂ ≤ hi` is not violated, or documenting the swap.

### WR-02: `PassAtK`'s `k > n` branch returns 1.0 — mathematically wrong as a fallback

**File:** `bench/aggregator/passk.go:30-32`
**Issue:** When `k > n`, `PassAtK` returns `1.0`. This is correct only when there are at least
`k` distinct guaranteed-correct draws; in general, sampling `k > n` items (the estimator is
undefined for k>n without replacement) does NOT imply success — e.g. `PassAtK(n=2, c=0, k=3)`
returns 1.0 despite zero correct samples. The doc claims callers always pass `k ≤ n`, and in
the current call path `pickKN` caps `kN ≤ ExpectedN ≤ nBool`, so the branch is effectively
dead. But it is a public, exported function with a wrong silent answer for an out-of-contract
input, and the `k > n` early-return masks the bug rather than rejecting it.

**Fix:** Either fold `k > n` into the genuinely-correct branch only when `c == n` (all
correct), or make the contract explicit and refuse:
```go
if k > n {
    // Estimator undefined for k>n (sampling without replacement). Only a
    // task with c==n is guaranteed; otherwise this input is a caller bug.
    if c == n {
        return 1.0
    }
    return math.NaN() // or panic — surface the contract violation, don't fabricate 1.0
}
```
At minimum, do not return 1.0 for `c < n` when `k > n`.

### WR-03: Cost-table load failure is silently swallowed — every cost cell becomes em-dash with no operator signal

**File:** `bench/aggregator/aggregate.go:78-82`, `cmd/helix-bench/aggregate.go:72-79`
**Issue:** `ct, ctErr := cost.LoadCostTable(...)` — `ctErr` is captured and then used only to
degrade every cost cell to a null CI (`reduceCostRow` returns early on `ctErr != nil`). The
only surfaced signal is an empty `cost_table_valid_until:` footer line. The CLI
(`cmd/helix-bench/aggregate.go`) prints `"aggregate: wrote .../leaderboard.md"` and exits 0
regardless. A stale/missing/malformed cost table — exactly what the fail-closed freshness gate
in `bench/cost` is designed to catch elsewhere — here produces a complete, zero-cost-data
report with no warning. The freshness gate's fail-closed intent is undermined at the
aggregator boundary.

**Fix:** Surface `ctErr` to the operator (non-fatal is defensible for the leaderboard, but it
must be visible). Either emit a warning line to stderr from the CLI, or record the error in
the footer / a "cost unavailable" banner in `cost_quality.md`:
```go
if ctErr != nil {
    fmt.Fprintf(os.Stderr, "aggregate: cost table unavailable (%v); cost cells degraded to —\n", ctErr)
}
```

## Info

### IN-01: `pickKN` mislabels the pass@N column when KValues carries an intermediate k

**File:** `bench/aggregator/aggregate.go:184-191, 349-362`
**Issue:** The leaderboard column header is hard-coded `pass@N` (report.go:176) but `pickKN`
selects the **largest configured k ≤ ExpectedN**, not necessarily N. With `KValues=[1,2]` and
ExpectedN=5, the "pass@N" column actually holds pass@2. The number is computed correctly; only
the header is misleading. Consider rendering the actual k in the header (`pass@%d`) so the
published artifact is self-describing.

### IN-02: `costPerSolvedTask` value is computed then discarded; only its `ok` flag is used

**File:** `bench/aggregator/aggregate.go:232-236`, `bench/aggregator/cost.go:64-73`
**Issue:** `reduceCostRow` calls `costPerSolvedTask(perSolvedUSD)` but discards the value
(`if _, ok := ...`), then publishes `bca(solvedUSDVec...).Point = StatMean(solvedUSDVec)`. The
two are equal by construction, so the result is correct, but the COST-02 primitive is invoked
purely as an emptiness check — its headline value never reaches the report. This is harmless
today but invites drift if either reduction changes (e.g. weighting). Prefer using the
primitive's returned value as the point, or drop the call and check `len(perSolvedUSD)`
directly with a comment.

### IN-03: RNG stream is order- and presence-sensitive across metrics

**File:** `bench/aggregator/aggregate.go:184-191, 338-347`
**Issue:** A single `*rand.Rand` is threaded sequentially through every metric's `BCaInterval`.
`bca()` returns early WITHOUT consuming RNG when a metric vector is empty (aggregate.go:339),
so the presence/absence of one metric shifts the bootstrap draws of every *subsequent* metric.
Output stays deterministic for a fixed input (so byte-determinism holds), but two runs that
differ only in which metrics are nil are not comparable replicate-for-replicate, and reordering
the metric reductions would silently change all CIs. This is acceptable given the locked column
order, but worth a one-line note in the code that the metric reduction order is part of the
determinism contract.

---

_Reviewed: 2026-06-21_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
