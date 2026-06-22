---
phase: 87
plan: 04
subsystem: bench/aggregator
tags: [aggregator, swebench, verified-correctness, additive-column, determinism, tdd]
requires:
  - "bench/runtime: SwebenchRawResolvedKey / SwebenchRescoredVerifiedKey pinned consts (Plan 01)"
  - "bench/aggregator: CanaryPassRate / reduceCanaryRate / rowCanary precedent (Phase 86 Plan 05)"
provides:
  - "LeaderRow.RawScore / LeaderRow.RescoredScore additive ciValue fields"
  - "aggregate.go rowSwebenchScores + reduceSwebenchScores (pooled, zero-RNG) reading the pinned open doc keys"
affects:
  - "Phase 89 downstream SWE-bench-specific reporter (reads the same pinned consts + these fields)"
tech-stack:
  added: []
  patterns:
    - "Additive score-time column reduced from open-provenance doc keys via shared pinned consts (CanaryPassRate clone)"
    - "Degenerate [point,point] ciValue pooled rate consuming zero RNG (no bootstrap), null->em-dash discipline"
key-files:
  created:
    - bench/aggregator/aggregate_swebench_test.go
  modified:
    - bench/aggregator/report.go
    - bench/aggregator/aggregate.go
decisions:
  - "RawScore/RescoredScore populated on LeaderRow but NOT rendered into leaderboard.md/cost_quality.md, keeping the locked goldens byte-identical (Phase 86 CanaryPassRate discipline); SWE-bench render is downstream Phase 89"
  - "present := raw != nil || rescored != nil — a row carrying either key is a SWE-bench row; each rate counts only the rows carrying ITS key (excluded from both numerator and denominator otherwise)"
metrics:
  duration: "~12m"
  completed: "2026-06-21"
---

# Phase 87 Plan 04: Aggregator Raw-vs-UTBoost-Rescored Side-by-Side Column Summary

Added the VERIFIED-02 additive `RawScore` / `RescoredScore` side-by-side columns to the aggregator `LeaderRow`, reduced at score time from the open-provenance doc keys `runtime.SwebenchRawResolvedKey` / `runtime.SwebenchRescoredVerifiedKey` via `rowSwebenchScores`/`reduceSwebenchScores` — a flat pooled rate consuming zero RNG, cloning the Phase 86 `CanaryPassRate` precedent exactly, with the existing leaderboard/cost goldens proven byte-identical.

## What Was Built

- `report.go`: two additive `ciValue` fields `RawScore` and `RescoredScore` on `LeaderRow`, with a doc comment citing Plan 03's `rescore.ApplyToRow` as the real producer and the additive/zero-RNG/null-em-dash discipline (mirrors `CanaryPassRate`).
- `aggregate.go`:
  - `rowSwebenchScores(r Row) (raw, rescored *bool, present bool)` reads both open doc keys via the PINNED `runtime.*Key` consts (never literal strings), `present` true when the row carries either key; `rowBoolKey` helper json.Unmarshals each, returning nil on absence/parse-error (excluded, never coerced).
  - `reduceSwebenchScores(loaded, tasks, mode) (rawCI, rescoredCI ciValue)` pooled fractions over rows carrying each respective key; `pooledRate` helper builds a degenerate `[point,point]` ciValue, zero total -> NULL ci (OK==false). Consumes no RNG.
  - Wired `leader.RawScore, leader.RescoredScore = reduceSwebenchScores(...)` AFTER the determinism-locked metric reductions (mirror the CanaryPassRate assignment site). Imported `bench/runtime`.
- `aggregate_swebench_test.go` (hermetic): `writeSwebenchRow` injects the raw/rescored open keys post-build under the same pinned consts the producer stamps; `TestRowSwebenchScoresReadsKeys`; Case A divergence (raw 1.0 > rescored 0.5, both OK); Case B absent -> NULL ci (em-dash); Case C byte-stable golden guard over the non-swebench `goldenFixture`.

## TDD Gate Compliance

- **RED** (`6fdc3f77`): `test(87-04)` — swebench test added; failed to compile (undefined `rowSwebenchScores`, `RawScore`, `RescoredScore`). Confirmed failing before implementation.
- **GREEN** (`64183224`): `feat(87-04)` — fields + reader + reducer + wiring; all four swebench tests pass, full aggregator/bench suite green, `make vet` clean.
- REFACTOR: none needed (direct clone of the established precedent).

## Byte-Stable-Golden Proof (Case C)

`TestSwebenchColumnsGoldenStable` runs `Aggregate` over the non-swebench `goldenFixture` and asserts `leaderboard.md` and `cost_quality.md` are byte-identical to the committed `testdata/*.golden.md`. The additive fields are populated on the struct but never rendered into those reports, and `reduceSwebenchScores` calls no `bca()`/RNG — so the IN-03 metric-order/presence determinism contract and the locked goldens are untouched. `TestAggregateEndToEnd` and `TestDeterministic` also remain green.

## Verification

- `go build ./...` — pass
- `go vet ./bench/aggregator/...` and `go vet ./bench/...` — pass
- `go test ./bench/aggregator/...` — pass (incl. TestSwebench*, TestAggregateEndToEnd, TestDeterministic)
- `go test ./bench/...` — pass
- `make vet` (incl. all custom vettools) — pass

## Deviations from Plan

None - plan executed exactly as written. (The plan named `rowSwebenchScores`/`reduceSwebenchScores`; two small private helpers `rowBoolKey` and `pooledRate` were factored out to avoid duplicating the per-key parse/rate logic — not a deviation, an internal factoring within the named functions.)

## Known Stubs

None. The columns are populated from real open doc keys; they are intentionally not rendered into the existing reports (the SWE-bench-specific render is owned by downstream Phase 89, per the plan), which is the documented byte-stable-golden discipline, not a stub.

## Self-Check: PASSED

- FOUND: bench/aggregator/aggregate_swebench_test.go
- FOUND: bench/aggregator/report.go (RawScore/RescoredScore fields)
- FOUND: bench/aggregator/aggregate.go (rowSwebenchScores/reduceSwebenchScores)
- FOUND commit 6fdc3f77 (RED), 64183224 (GREEN)
