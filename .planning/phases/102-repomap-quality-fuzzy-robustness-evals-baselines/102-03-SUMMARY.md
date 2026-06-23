---
phase: 102-repomap-quality-fuzzy-robustness-evals-baselines
plan: 03
subsystem: bench/aggregator
status: complete
tags: [bench, baseline, renderer, byte-reproducible, anti-vacuity, deterministic, gitignore-allowlist]
requires:
  - bench/evaluators/repomapeval (Plan 01 corpus scores — nDCG@10/recall@10/MRR/budget-fit)
  - bench/evaluators/fuzzyrobust (Plan 02 per-strategy summary + ambiguous-refused assertion)
  - bench/runtime.Validate (result.v2 schema gate)
  - bench/runtime/repomap_eval_capture_regen.go + fuzzy_robust_capture_regen.go (Plan 01/02 regenerators)
  - bench/aggregator/aider_edit_baseline.go (Phase 100 renderer analog, mirrored field-by-field)
provides:
  - "RenderRepoMapEvalBaseline — deterministic sort-before-emit renderer, fail-closed on missing nDCG@10"
  - "RenderFuzzyRobustBaseline — deterministic renderer, fail-closed on missing OR false ambiguous_refused"
  - "committed bench/reports/repomap-eval-baseline/{result.v2.json,BENCH-RESULTS.md}"
  - "committed bench/reports/fuzzy-robust-baseline/{result.v2.json,BENCH-RESULTS.md}"
  - ".gitignore allowlist force-tracking the two BASELINE-02 trees"
  - "Makefile bench-repomap-eval + bench-fuzzy-robust regen targets"
affects:
  - Phase 102 verification (BASELINE-02 satisfied; the committed bytes are the CI contract)
tech-stack:
  added: []
  patterns:
    - sort-before-emit + fail-closed renderer + double-render byte-reproducibility (Phase 100 trio mirror)
    - decode-only-deterministic-fields struct so latency/timestamp can never leak
    - additive-open-key result.v2 (no schema bump; mirror Phase 100 edit_format_applied *bool)
    - anti-vacuity: a stripped/false load-bearing metric fails the assertion AND the renderer fail-closes
    - .gitignore allowlist force-tracking committed baseline trees under ignored bench/reports/*
key-files:
  created:
    - bench/aggregator/repomap_eval_baseline.go
    - bench/aggregator/repomap_eval_baseline_test.go
    - bench/aggregator/fuzzy_robust_baseline.go
    - bench/aggregator/fuzzy_robust_baseline_test.go
    - bench/reports/repomap-eval-baseline/result.v2.json
    - bench/reports/repomap-eval-baseline/BENCH-RESULTS.md
    - bench/reports/fuzzy-robust-baseline/result.v2.json
    - bench/reports/fuzzy-robust-baseline/BENCH-RESULTS.md
  modified:
    - .gitignore
    - Makefile
decisions:
  - "repomap baseline headline = nDCG@10 (D-07); load-bearing fail-closed key is the nDCG@10 pointer"
  - "fuzzy baseline load-bearing key = ambiguous_refused (D-06); fail-closed on absent OR false"
  - "deterministic metrics carried under additive open keys (repomap_eval / fuzzy_robust objects); no schema bump"
  - "renderers decode ONLY deterministic fields into the row struct so latency/timestamp/abs-path can never leak"
  - "result.v2.json restates the Wave 1 aggregate corpus scores, NOT a live capture (committed-vs-committed contract)"
metrics:
  duration_min: 16
  completed: 2026-06-23
  tasks: 3
  files: 10
  tests_passing: 6
---

# Phase 102 Plan 03: Committed RepoMap-Eval + Fuzzy-Robustness Baselines (BASELINE-02) Summary

Two committed, byte-reproducible baseline trees — `bench/reports/repomap-eval-baseline/` and `bench/reports/fuzzy-robust-baseline/` — each a deterministic-metrics-only `result.v2.json` plus a renderer-produced `BENCH-RESULTS.md`, authored by two new pure sort-before-emit aggregator renderers (`RenderRepoMapEvalBaseline` / `RenderFuzzyRobustBaseline`) that fail-CLOSED on their load-bearing metric, proven by double-render diff-empty + stripped-metric anti-vacuity tests, force-tracked via a `.gitignore` allowlist with `make bench-{repomap-eval,fuzzy-robust}` HELIX_BIN-gated regen targets — mirroring the Phase 100 `aider_edit_baseline` trio field-by-field.

## What Was Built

- **`repomap_eval_baseline.go`** — `RenderRepoMapEvalBaseline(resultV2 []byte) ([]byte, error)`: decodes ONLY deterministic ranking-quality fields into `repoMapEvalBaselineRow` (so latency/timestamp can never leak), restates the headline nDCG@10 (D-07) + recall@10 / MRR / budget-fit (D-08) + the reversed/seeded-random discriminator margin (D-09), `sort.Slice` metric rows before emit, **fail-CLOSED** when the `nDCG@10` pointer is nil (the `EditFormatApplied == nil` analog). Pure string-builder.
- **`fuzzy_robust_baseline.go`** — `RenderFuzzyRobustBaseline(...)`: decodes the deterministic per-strategy summary + the load-bearing `ambiguous_refused` boolean, sort-before-emit per-strategy rows, **fail-CLOSED** when `ambiguous_refused` is absent OR false (D-06 must-refuse). Pure string-builder.
- **Two committed `result.v2.json`** — deterministic-metrics-only documents restating the Wave 1 aggregate corpus scores (committed-vs-committed contract, NOT a live capture). RepoMap: nDCG@10/recall@10/MRR/budget-fit = 1.0 (n=28), discriminator forward 1.0 vs reversed 0.0048 vs seeded-random 0.5243, margin 0.30. Fuzzy: exact=24 / whitespace_normalized=24 / ambiguous_match=3 (all refused), 51 cases. Both `schema_version: "v2"` + additive open keys (`repomap_eval` / `fuzzy_robust` objects) — NO schema bump, NO latency/timestamp/abs-path.
- **Two committed `BENCH-RESULTS.md`** — rendered FROM each renderer's own output (so the byte-reproducible test matches its own golden).
- **Six tests** (in `*_baseline_test.go`) mirroring the three aider tests each: `*BaselineResultValid` (present + `benchruntime.Validate` schema-ok + load-bearing key), `*BaselineByteReproducible` (double-render diff-empty + golden match), `*BaselineAntiVacuity` (stripped/false metric fails the assertion AND the renderer fail-closes).
- **`.gitignore`** — four allowlist lines force-tracking the two baseline trees under the otherwise-ignored `bench/reports/*` (mirror Phase 100 lines 275-276).
- **`Makefile`** — `bench-repomap-eval` + `bench-fuzzy-robust` targets: build helix then run the Plan 01/02 `//go:build ignore` regenerators HELIX_BIN-gated (mirror `bench-aider-edit`); local-only, the committed bytes are the CI contract.

## Key Results (hermetic golden — no HELIX_BIN, no network)

- `go test ./bench/aggregator/ -count=1` → PASS (both baselines' result-valid + byte-reproducible + anti-vacuity tests, the sole authoritative BASELINE-02 proof).
- `git check-ignore -v bench/reports/{repomap-eval,fuzzy-robust}-baseline/result.v2.json` → rc=1 (NOT ignored; force-tracked).
- `make -n bench-repomap-eval` / `make -n bench-fuzzy-robust` → resolve and print the build-helix + HELIX_BIN regen recipe.

## TDD Gate Compliance

This is a `type: tdd` plan. For each renderer task the RED gate was the compile failure (`undefined: RenderRepoMapEvalBaseline` / `RenderFuzzyRobustBaseline`) with the test authored first; GREEN followed with the renderer implementation. The byte-reproducibility (double-render diff-empty) and anti-vacuity (stripped/false load-bearing metric) tests are the natural RED anchors the plan called for. Both tasks shipped as a single `feat(...)` commit each (test + implementation + golden authored atomically, since the golden is produced from the renderer's own output); Task 3 is config glue (`chore`).

## Deviations from Plan

### Notes (not deviations)

- **`result.v2.json` restates Wave 1 aggregate scores, does not re-derive them.** Per the committed-vs-committed contract and the plan's explicit "consume their committed corpus scores" instruction, the baseline `result.v2.json` carries the corpus aggregate scores reported by the Wave 1 SUMMARYs / hermetic-golden tests (nDCG@10=1.0 etc.; per-strategy exact=24/ws=24/ambiguous=3). The HELIX_BIN-gated capture legs live in the Plan 01/02 `//go:build ignore` regenerators (driven by the new Makefile targets) — they were NOT duplicated here, per the critical-constraints directive.
- **`git add -f` used to stage the committed baseline trees in Tasks 1-2.** The `.gitignore` allowlist lands in Task 3, so the baseline files were force-added in the earlier task commits. This is the intended force-tracking (the allowlist makes them tracked-by-default from Task 3 onward); these are legitimate committed CI-contract artifacts, not user-gitignored content.

Otherwise: plan executed exactly as written.

## Verification

- `go test ./bench/aggregator/ -count=1` → PASS (6 new tests + all pre-existing aggregator tests).
- `go test ./bench/... ` → rc=0 (full bench suite green; Wave 1 evaluators still pass consuming the same corpus).
- `go vet ./bench/...` → clean. `gofmt -l bench/aggregator/` → no diff.
- `go build ./cmd/helix` → OK.
- `git check-ignore` → both baseline trees NOT ignored. `make -n bench-repomap-eval` / `bench-fuzzy-robust` → resolve.

## Self-Check: PASSED

- All 8 created files + 2 modified files present on disk (verified).
- All 3 task commits present in git history: c152001f, adc69a5c, 777c37a7 (verified).
