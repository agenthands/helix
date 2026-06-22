---
phase: 82-multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-leaderboard
plan: 02
subsystem: bench/aggregator
tags: [statistics, bootstrap, bca, confidence-interval, pure-go, tdd]
requires: []
provides:
  - "aggregator.BCaInterval — proper BCa bootstrap CI primitive (STATS-02)"
  - "aggregator.StatMean — per-task mean statistic helper"
  - "aggregator package (new) under bench/"
affects:
  - "Plan 06 (report rendering) calls BCaInterval for every leaderboard/cost_quality cell"
tech-stack:
  added: []
  patterns:
    - "Hand-rolled numerics via stdlib math.Erfinv/Erfc (no third-party stats dep)"
    - "Injected seeded *rand.Rand (math/rand/v2 PCG) for deterministic output (D-08)"
    - "Degenerate-guard discipline: null CI / point CI / a=0, never NaN/Inf (D-09)"
key-files:
  created:
    - "bench/aggregator/bootstrap.go"
    - "bench/aggregator/bootstrap_test.go"
  modified: []
decisions:
  - "Quantile rule locked to nearest-rank idx=clamp(round(p*(B-1)),0,B-1) for byte-stable reproduction (A5)"
  - "ok==false (null CI) reserved for empty input only; degenerate-but-non-empty returns point CI with ok==true"
  - "No separate REFACTOR commit: helpers were extracted in the initial GREEN implementation"
metrics:
  duration: "~3m"
  completed: "2026-06-21"
  tasks: 2
  files: 2
---

# Phase 82 Plan 02: BCa Bootstrap Primitive (STATS-02) Summary

Proper BCa (bias-corrected accelerated) bootstrap confidence-interval primitive in a new
`bench/aggregator` package: `BCaInterval` computes z0 (bias correction from the fraction of
bootstrap replicates below the observed statistic, via `Φ⁻¹ = √2·math.Erfinv`) AND acceleration
`a` (jackknife leave-one-out skewness), reads endpoints off a seeded `math/rand/v2` PCG bootstrap
distribution, and survives every degenerate case without a crash or NaN/Inf — built TDD, RED before
GREEN.

## What Was Built

- **`bench/aggregator/bootstrap.go`** (212 lines): `BCaInterval(vals, stat, B, alpha, rng) (lo, hi, ok)`
  plus `phi`, `phiInv`, `clamp01`, exported `StatMean`, and the factored helpers `bootstrapReplicates`,
  `jackknifeAccel`, `bcaPercentiles`, `quantile`.
  - **z0** = `phiInv(p0)`, `p0 = #{θ*_b < θ̂}/B` (Efron & Tibshirani 1993 §14.3).
  - **a** = jackknife skewness `Σ(θ̄−θ̂_(i))³ / (6·(Σ(θ̄−θ̂_(i))²)^1.5)` (eq. 14.15), den==0 ⇒ a=0.
  - **endpoints** = `bcaPercentiles` adjustment (eq. 14.10) with `|1−a(z0+z)| < 1e-12` percentile
    fallback + `clamp01`, then nearest-rank quantile off the sorted bootstrap distribution.
- **`bench/aggregator/bootstrap_test.go`** (189 lines): `TestBCa` with six behavior groups — see below.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | RED — BCa test suite | `bf3cfeac` | bench/aggregator/bootstrap_test.go |
| 2 | GREEN — implement BCaInterval | `4a497eeb` | bench/aggregator/bootstrap.go |

## TDD Gate Compliance

- **RED:** `test(82-02): add failing BCa bootstrap tests` (`bf3cfeac`) committed first; `go test
  ./bench/aggregator/ -run TestBCa` failed with `undefined: BCaInterval` (build failed) — RED-OK.
- **GREEN:** `feat(82-02): implement BCa bootstrap interval` (`4a497eeb`) after it; all 8 subtests PASS.
- **REFACTOR:** not a separate commit — helpers (`jackknifeAccel`, `bcaPercentiles`, `quantile`,
  `bootstrapReplicates`) were extracted directly in the GREEN implementation, kept tests green.

Gate sequence verified in `git log`: `test(82-02)` precedes `feat(82-02)`.

## Correctness Evidence (the fake-BCa discriminator)

The decisive test, `BCa_differs_from_percentile_on_skew`, builds a percentile interval over the
**same** seeded bootstrap distribution (re-seeded identically, same nearest-rank quantile rule) so
the ONLY difference is the z0/a adjustment, then asserts the BCa endpoints diverge by > 1e-6. A
percentile-bootstrap-masquerading-as-BCa would produce identical endpoints and FAIL. This passed,
proving z0 and acceleration actually fire. Complementary anchors: symmetric sample ⇒ BCa ≈ percentile;
known-mean sample ⇒ CI contains the mean; width shrinks as m grows; determinism (D-08); D-09
degenerate matrix (empty→null CI, all-identical/m==1→point CI, no NaN/Inf).

## Verification

| Command | Result |
|---------|--------|
| `go test ./bench/aggregator/ -run TestBCa -v` | PASS — 8/8 subtests (RUN + PASS confirmed) |
| `go build ./...` | clean |
| `go vet ./bench/aggregator/...` | clean |
| `go vet ./...` | clean |
| `make vet` (incl. 5 custom vettools) | clean |
| `go test ./...` | ALL-TESTS-PASS |
| `gofmt -l` on touched files | empty (clean) |

No `HELIX_BIN` required — this is a pure unit package (D-01). Helix binary not committed.

## Deviations from Plan

None — plan executed exactly as written. Both tasks followed the RED→GREEN flow; the optional
REFACTOR commit was folded into GREEN because the helper extraction was done up front.

## Known Stubs

None. `BCaInterval` is fully wired and exercised; no placeholders or hardcoded sentinels.

## Self-Check: PASSED

- FOUND: bench/aggregator/bootstrap.go
- FOUND: bench/aggregator/bootstrap_test.go
- FOUND commit: bf3cfeac (test/RED)
- FOUND commit: 4a497eeb (feat/GREEN)
