---
phase: 89-reports-ci-policy-contamination-canary
plan: 03
subsystem: bench/aggregator + cmd/helix-bench
tags: [bench, aggregator, cli, helix-bench, report, determinism, byte-reproducible]
requires: ["89-01", "89-02"]
provides:
  - "bench/aggregator.renderAll: shared 4-report render path (Aggregate + report)"
  - "helix-bench report --run-id <id>: validated, fail-closed report regeneration"
  - "REPORT-05 byte-reproducibility proof (double-render diff-empty + golden match)"
affects:
  - "bench/aggregator/aggregate.go (now writes all 4 reports via renderAll)"
  - "cmd/helix-bench report subcommand (no longer a notYetImplemented stub)"
tech-stack:
  added: []
  patterns:
    - "single shared render path, two CLI entry points (aggregate + report)"
    - "cloned (not imported) isValidRunID segment guard before filepath.Join (V5)"
    - "hermetic double-render byte-reproducibility golden test (no live run)"
key-files:
  created:
    - cmd/helix-bench/report.go
    - cmd/helix-bench/report_test.go
    - bench/aggregator/render_all_test.go
    - bench/aggregator/byte_reproducible_test.go
    - bench/aggregator/testdata/goldenfix_per_language.golden.md
    - bench/aggregator/testdata/goldenfix_ablations.golden.md
  modified:
    - bench/aggregator/aggregate.go
    - cmd/helix-bench/main.go
    - bench/aggregator/aggregate_swebench_test.go
    - bench/aggregator/aggregate_canary_test.go
decisions:
  - "renderAll builds the scatter internally so report and aggregate share the exact single-source join"
  - "per_language/ablations over goldenFixture need OWN goldens (goldenfix_*) — the standalone renderer tests use DIFFERENT fixtures"
  - "isValidRunID is CLONED into cmd/helix-bench (the swebench copy is package-private), not imported"
metrics:
  duration: ~7m
  completed: 2026-06-21
---

# Phase 89 Plan 03: helix-bench report --run-id + shared renderAll + byte-reproducibility Summary

Wired `helix-bench report --run-id <id>` as a thin, validated, fail-closed RunE over a newly-factored shared `renderAll(rep, runDir)` that emits all 4 reports (leaderboard + cost_quality + per_language + ablations), and proved the load-bearing REPORT-05 contract: every report regenerates byte-identically on re-render (double-render diff-empty) and matches its committed golden — a hermetic test, never a live run.

## What was built

**Task 1 — shared renderAll + report --run-id (TDD RED→GREEN):**
- `bench/aggregator/aggregate.go`: factored the inline 2-report render+write block into a shared `renderAll(rep *Report, runDir string) error` that builds the scatter internally and atomically writes ALL 4 reports (`leaderboard.md`, `cost_quality.md`, `per_language.md`, `ablations.md`) via `writeReport`. `Aggregate` now calls `renderAll`. Before this plan only the first two were written to disk; per_language/ablations existed as renderers tested via direct calls only.
- `cmd/helix-bench/report.go`: real `newReportCmd` with a `--run-id` RunE mirroring `newAggregateCmd` EXACTLY (same `--runs 3 / --seed 42 / --iterations 10000 / --ci-level 0.95 / --cost-table` defaults, `Today: time.Now().UTC()` injected at the boundary, verbatim error propagation for fail-closed non-zero exit). It validates `--run-id` via a **cloned** `isValidRunID` (`[A-Za-z0-9_-]+`, rejects `..`/`/`/leading-`-`/empty) BEFORE `filepath.Join(out, runID)`, resolves `<out>/<run-id>/` with `--out` default `bench/reports`, then calls `Aggregate`.
- `cmd/helix-bench/main.go`: removed the `notYetImplemented("report")` stub body (registration at `main.go:101` untouched — subcommand count stays 6), updated the root help line.

**Task 2 — byte-reproducibility proof + lockstep guard updates (TDD):**
- `bench/aggregator/byte_reproducible_test.go`: `TestReportByteReproducible` renders all 4 reports, re-renders over the same deterministic tree, and asserts each is byte-identical to the first render AND to its committed golden (the REPORT-05 capstone). `TestReportByteReproducibleSameDir` proves re-rendering into the SAME runDir (overwriting prior files via temp+rename) is also byte-identical.
- `bench/aggregator/render_all_test.go`: `TestRenderAllEmitsFourReports` locks that `Aggregate` writes all 4 to disk, each matching its golden (`goldenfix_*` for per_language/ablations over the goldenFixture tree).

## Deviations from Plan

None — plan executed as written. One implementation detail clarified during execution: the committed `per_language.golden.md` / `ablations.golden.md` were generated from DIFFERENT fixtures (a partial-language fixture and `abFixture`), so the renderAll/byte-reproducible tests over `goldenFixture` required their own goldens (`goldenfix_per_language.golden.md`, `goldenfix_ablations.golden.md`), generated via `-update`. This is the documented `-update` golden mechanism, not a deviation.

## Lockstep golden guards — UPDATED, not bypassed (Pitfall 1)

| Guard | Before (intent) | After (intent) | Status |
|-------|-----------------|----------------|--------|
| `TestSwebenchColumnsGoldenStable` (aggregate_swebench_test.go) | "the additive swebench column is invisible / perturbs nothing / not rendered" | REPORT-01 `verified_correctness` + `cost_per_solved` columns RENDER CORRECTLY (headers present, golden byte-stable); Phase 87 raw/rescored stay unrendered-additive (no header leak) | RUNNING, updated |
| `TestAggregateCanaryAbsentIsEmDash` (aggregate_canary_test.go) | "absent canary leaves a NULL CanaryPassRate (em-dash)" | NULL CanaryPassRate AND the INFRA-05 contamination footnote RENDERS CORRECTLY by being ABSENT (no footnote fabricated when nothing contaminated) | RUNNING, updated |

Both guards still execute (no `t.Skip`, no deletion) and pass with their new "renders correctly" assertions.

## REPORT-05 reconciliation

REPORT-05 (regenerate all 4 reports byte-identically from a run-id, proven hermetically) is satisfied: `report --run-id` and `aggregate` funnel through the single `renderAll` path (`TestReportEqualsAggregate` proves report==aggregate byte-equality), and `TestReportByteReproducible` proves the double-render diff-empty + committed-golden match for all 4 reports. Path-traversal (`T-89-03-01`) is mitigated by the cloned `isValidRunID` segment guard before any `filepath.Join` (`TestReportRunIDValidation` rejects `../etc`, `..`, `-rf`, `a/b`, whitespace, empty with non-zero exit and zero filesystem writes).

## Verification

- `go build ./...` — pass
- `go vet ./bench/... ./cmd/helix-bench/...` — pass
- `make vet` (all 7 vettools) — pass
- `go test ./bench/aggregator/... ./cmd/helix-bench/...` — pass
- `helix-bench report --help` — works; subcommand count unchanged at 6 (`TestHelixBenchHelpListsSubcommands` green)

## Commits

- `a78d3b50` test(89-03): add failing renderAll + report --run-id tests (RED)
- `969637f7` feat(89-03): factor shared renderAll + wire report --run-id (GREEN)
- `02370f0d` test(89-03): byte-reproducibility proof + update lockstep guards (GREEN)

## Self-Check: PASSED

- `cmd/helix-bench/report.go` — FOUND
- `bench/aggregator/byte_reproducible_test.go` — FOUND
- `bench/aggregator/render_all_test.go` — FOUND
- `bench/aggregator/testdata/goldenfix_per_language.golden.md` — FOUND
- `bench/aggregator/testdata/goldenfix_ablations.golden.md` — FOUND
- Commits `a78d3b50`, `969637f7`, `02370f0d` — FOUND in git log
