---
phase: 82-multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-leaderboard
plan: 03
subsystem: bench/aggregator
tags: [statistics, pass-at-k, humaneval, unbiased-estimator, pure-go, tdd]
requires: []
provides:
  - "aggregator.PassAtK(n,c,k) — HumanEval UNBIASED pass@k estimator, stable c-term product form (STATS-03)"
  - "aggregator.logBinom(a,b) — ln C(a,b) via math.Lgamma (D-11 log-binomial cross-check helper)"
affects:
  - "Plan 06 (report rendering): per-task pass@k scalar is the across-task resampling vector for the BCa CI"
  - "Leaderboard pass@1/pass@N columns (D-11): mean of per-task PassAtK across tasks"
tech-stack:
  added: []
  patterns:
    - "Numerically-stable product form 1 - Π_{i=n-c+1}^{n}(1 - k/i); no factorials, no overflow"
    - "Independent two-form correctness proof: product (impl) vs lgamma log-binomial (test) agree ~1e-12"
    - "k>=2 anti-naive anchor (10,3,5)->0.91667 + explicit anti-naive assertion lock Pitfall 1"
key-files:
  created:
    - "bench/aggregator/passk.go"
    - "bench/aggregator/passk_test.go"
  modified: []
decisions:
  - "Public signature exported as PassAtK (plan-fixed) vs RESEARCH's lowercase passAtK; callers in Plan 06 need it cross-file"
  - "k>n folded into the n-c<k early-return (return 1.0) per D-11 guard rather than producing a nonsense product"
  - "logBinom kept in impl for the cross-check/log-space callers; the test defines its OWN local copy so form-agreement is a genuinely independent derivation"
  - "No separate REFACTOR commit: implementation was clear at GREEN; nothing to clean up"
metrics:
  duration: "~4m"
  completed: "2026-06-21"
  tasks: 2
  files: 2
---

# Phase 82 Plan 03: pass@k Unbiased Estimator (STATS-03) Summary

HumanEval UNBIASED pass@k estimator `1 − C(n−c,k)/C(n,k)` in `bench/aggregator/passk.go`, computed
in Chen et al.'s numerically-stable c-term product form `1 − Π_{i=n−c+1}^{n}(1 − k/i)` — built TDD,
RED before GREEN. The corrected BLOCKER (c-term vs k-term loop count) is locked by a k>=2 reference
anchor `(10,3,5) → 0.91667`, an explicit anti-naive assertion, and a grid that proves the product
form agrees with an independent lgamma log-binomial derivation to ~1e-12.

## What Was Built

- **`PassAtK(n, c, k int) float64`** — the locked HumanEval unbiased estimator (D-10). Early-returns
  `1.0` for `k > n` (D-11 guard) and the `n−c < k` branch. Otherwise runs the product over
  `i = n-c+1 .. n` (exactly **c** terms = number of successes, NOT k). Carries the mandatory comment
  "The term count is c (= number of successes), NOT k — looping k times is the classic wrong
  implementation". Cites Chen et al. 2021, arXiv:2107.03374 §2.1, and documents the pass@1==c/n
  identity. No naive `1−(1−p)^k` anywhere.
- **`logBinom(a, b int) float64`** — `ln C(a,b)` via `math.Lgamma`, `-Inf` for `b<0 || b>a`. Kept as
  the D-11 cross-check primitive and a log-space path for callers; commented as the SECONDARY form
  (product form is PRIMARY).
- **`passk_test.go`** — `TestPassAtK` (reference table incl. the (10,3,5)->0.91667 anchor, the
  pass@1==c/n identity cases, zero-c, and the n−c<k early-return), `TestPassAtKAntiNaive` (asserts the
  result is NOT the forbidden naive value 0.83193), and `TestPassAtKFormAgreement` (a full grid
  n∈1..12, c∈0..n, k∈1..n where the impl's product form must agree to ~1e-12 with an INDEPENDENT
  lgamma log-binomial computed inside the test).

## Correctness Gate Evidence (the corrected BLOCKER)

- Implementation uses the **c-term product loop** `for i := n - c + 1; i <= n; i++` (passk.go:45) —
  NOT the k-term loop. Verified by grep: no `math.Pow(1`, no `1-(1-...`, no `for i:=0;i<k`.
- `PassAtK(10,3,5)` evaluates to `11/12 = 0.91666...` (loop over i∈{8,9,10}:
  `(1−5/8)(1−5/9)(1−5/10) = 1/12`). The k-term-loop bug would give 0.97348; the naive estimator gives
  0.83193 — both are excluded by passing tests.
- `TestPassAtKAntiNaive` PASSED: result ≠ naive 0.83193 and is within 1e-4 of 0.91667.
- `TestPassAtKFormAgreement` PASSED across the full grid → product form and lgamma form agree.

## Verification

Commands run (all from repo root, NO HELIX_BIN — pure package):

| Command | Result |
|---|---|
| `go test ./bench/aggregator/ -run TestPassAtK -v` | RUN + PASS (TestPassAtK + all 10 subtests, TestPassAtKAntiNaive, TestPassAtKFormAgreement) |
| `go build ./...` | clean |
| `go vet ./bench/aggregator/...` | clean |
| `make vet` | clean (all custom vettools: noduckdb, nokernel2semantic, nosemantic2kernel, compact-uses-store, ablation-leakage) |
| `go test ./...` | ALL-PASS |
| `gofmt -l` on both files | clean (no output) |

RED→GREEN evidence:
- RED commit `3635b4a8`: `go test ./bench/aggregator/ -run TestPassAtK` failed with
  `undefined: PassAtK` (build failed) — test committed before implementation.
- GREEN commit `34a95f34`: same command PASSES; the 0.91667 anchor, anti-naive, and form-agreement
  all green.

The helix binary was NOT built or committed (pure-package plan).

## Deviations from Plan

None — plan executed exactly as written. The plan pre-specified the corrected c-term formula in
Task 2's action and the exported `PassAtK` signature; both were followed verbatim. No REFACTOR commit
was needed (Task 2 explicitly allows REFACTOR only "if needed").

## TDD Gate Compliance

- RED gate: `test(82-03): ...` commit `3635b4a8` (failing test, PassAtK undefined). ✓
- GREEN gate: `feat(82-03): ...` commit `34a95f34` (implementation, tests pass). ✓
- REFACTOR gate: not needed (implementation clear at GREEN; documented above). ✓

## Commits

- `3635b4a8` — `test(82-03): add failing pass@k reference + form-agreement tests (STATS-03)`
- `34a95f34` — `feat(82-03): implement HumanEval unbiased pass@k (STATS-03)`

## Self-Check: PASSED

- Files present: `bench/aggregator/passk.go`, `bench/aggregator/passk_test.go`,
  `.planning/phases/82-.../82-03-SUMMARY.md`.
- Commits present: `3635b4a8` (RED), `34a95f34` (GREEN).
