---
phase: 102-repomap-quality-fuzzy-robustness-evals-baselines
plan: 02
subsystem: bench/evaluators/fuzzyrobust
status: complete
tags: [bench, evaluator, fuzzy, strategy-selection, ambiguity-refusal, drift-corpus, anti-vacuity, stdlib-leaf]
requires:
  - bench/datasets/aider-polyglot/fixtures (vendored py/go/rust ground-truth blocks)
  - bench/evaluators/editsim.ES (the one in-repo leaf import, D-05)
  - internal/fuzzy.Match + ErrAmbiguous/ErrNoMatch (regenerator only; NOT the leaf)
provides:
  - bench/evaluators/fuzzyrobust (stdlib + editsim fuzzy strategy/refusal leaf)
  - "PerturbWhitespace / PerturbIndent / PerturbEllipsis / PerturbExact deterministic transforms"
  - "ScoreCase strategy-selection + editsim.ES similarity + refusal/no_match classification"
  - "committed py/go/rust drift + captured corpus (testdata/{drift,captured})"
  - bench/runtime/fuzzy_robust_capture_regen.go (//go:build ignore non-leaf capture harness)
affects:
  - Plan 102-03 (baseline render + Makefile/.gitignore wiring consume this leaf)
tech-stack:
  added: []
  patterns:
    - stdlib + editsim-only leaf + TestLeafImports self-test (vet-ablation-leakage does NOT gate bench/evaluators/*)
    - committed-vs-committed scoring + separate //go:build ignore capture harness (Phase 100 contract)
    - expected strategy DERIVED structurally from per-tier perturbation, never from observed behavior (D-05)
    - anti-vacuity duplicate-block must-refuse (ambiguous_match) + TestAmbiguousBites flipped-outcome gate (D-06)
    - path-segment validation before filepath.Join (T-102-05)
key-files:
  created:
    - bench/evaluators/fuzzyrobust/perturb.go
    - bench/evaluators/fuzzyrobust/fuzzyrobust.go
    - bench/evaluators/fuzzyrobust/corpus.go
    - bench/evaluators/fuzzyrobust/perturb_test.go
    - bench/evaluators/fuzzyrobust/corpus_test.go
    - bench/evaluators/fuzzyrobust/fuzzyrobust_test.go
    - bench/evaluators/fuzzyrobust/ambiguous_test.go
    - bench/evaluators/fuzzyrobust/leafimports_test.go
    - bench/evaluators/fuzzyrobust/CORPUS.md
    - bench/evaluators/fuzzyrobust/testdata/gen_corpus.go
    - bench/evaluators/fuzzyrobust/testdata/drift/{go,python,rust}.json
    - bench/evaluators/fuzzyrobust/testdata/captured/{go,python,rust}.json
    - bench/runtime/fuzzy_robust_capture_regen.go
  modified:
    - .planning/phases/102-repomap-quality-fuzzy-robustness-evals-baselines/deferred-items.md
decisions:
  - "expected strategy = perturbation tier; captured = observed fuzzy.Match outcome — they DIVERGE on purpose (D-05)"
  - "indentation_flexible cases capture as whitespace_normalized (tier-2 TrimSpace is a superset of tier-3 TrimLeft); a real measured selection mismatch, not a bug"
  - "ellipsis cases capture as exact (segmented match reports the weakest tier across un-drifted head/tail anchors); exempt from the 0.99 similarity floor in the golden"
  - "ambiguous case = shared BODY FRAGMENT present at TWO sites in Source (not a whole duplicated function); whitespace-drift of the fragment matches both → ErrAmbiguous refusal"
  - "17 cases/lang = 4 per strategy per language (16 single-site) + 1 duplicate-block ambiguous; 51 total"
metrics:
  duration_min: 14
  completed: 2026-06-23
  tasks: 4
  files: 17
  tests_passing: 15
---

# Phase 102 Plan 02: Fuzzy-Robustness Stdlib Leaf + Drift Corpus + Must-Refuse Anti-Vacuity Gate Summary

A stdlib-only (+ `editsim.ES`) `bench/evaluators/fuzzyrobust` leaf that scores `internal/fuzzy`'s 4-strategy cascade selection and ambiguity refusal against a committed py/go/rust **drift** corpus — each case a real vendored fixture block transformed by a deterministic per-tier perturbation whose tier IS the expected strategy (never read from tool behavior), gated by a duplicate-block must-refuse `ambiguous_match` case with a flipped-outcome bite test, plus a `//go:build ignore` non-leaf capture harness that calls `internal/fuzzy.Match` outside the leaf.

## What Was Built

- **`perturb.go`** — deterministic, stdlib-only (`strings`), RNG-free/time-free per-tier perturbation transforms: `PerturbWhitespace` (leading/trailing whitespace → `whitespace_normalized`), `PerturbIndent` (spaces→tabs + depth shift → `indentation_flexible`), `PerturbEllipsis` (middle line(s) → `...` → ellipsis tier), `PerturbExact` (identity → `exact`). The transform IS the expected-strategy derivation (D-05). Holds the five-value outcome vocabulary constants (`failed` is never a strategy label, Pitfall 6).
- **`fuzzyrobust.go`** — `ScoreCase(expectedStrategy, capturedStrategy, matchedText, expectedText)` returns a `CaseScore` grading strategy SELECTION (match/mismatch) AND `editsim.ES` similarity (orthogonal axes), classifying a refusal (`ambiguous_match`) distinct from a `no_match` and from a successful match. Reuses `editsim.ES` (the ONLY in-repo import, D-05); total on every input.
- **`corpus.go`** — `DriftCase`/`DriftCorpus`/`CapturedOutcome`/`CapturedOutcomes` types + `LoadDrift`/`LoadCaptured` with path-segment validation before `filepath.Join` (T-102-05). Stdlib only.
- **Drift corpus** (`testdata/drift/{go,python,rust}.json`, authored by `testdata/gen_corpus.go`) — 17 cases/language = 4 per strategy per language (16 single-site, from real wordy/bowling/two-bucket/pig-latin/leap fixture blocks) + 1 duplicate-block ambiguous case. 51 total.
- **Captured outcomes** (`testdata/captured/{go,python,rust}.json`) — real `internal/fuzzy.Match` outcomes recorded by the harness; byte-reproducible.
- **Tests** (15 passing) — `TestPerturb*` (per-tier determinism), `TestCorpusFloor` + `TestLoadRejectsTraversal`, `TestStrategy_{Match,Mismatch,Refusal,TotalNoPanic}`, `TestLeafImports` (+ self-discriminator), `TestAmbiguousRefused` + `TestAmbiguousBites` (anti-vacuity), `TestScoreDriftCorpus` (hermetic golden).
- **`bench/runtime/fuzzy_robust_capture_regen.go`** — `//go:build ignore` `package main` non-leaf harness: calls `fuzzy.Match` per case, maps `ErrAmbiguous`→`ambiguous_match`, `ErrNoMatch`→`no_match`, success→`Result.Strategy`; fail-not-skip (`os.Exit(2)`) on empty corpus / empty bucket / out-of-vocabulary outcome; deterministic-only bytes.
- **`CORPUS.md`** — outcome vocabulary, the per-tier perturbation parameters, the floor, the must-refuse construction, and the documented expected-vs-captured divergences.

## Key Findings (honest measurements, not bugs)

The leaf grades strategy SELECTION, so the EXPECTED (structural) strategy and the CAPTURED (observed) outcome deliberately diverge in two inherent ways, both documented in `CORPUS.md`:

1. **`indentation_flexible` → captured `whitespace_normalized`.** The cascade tries whitespace (tier 2, `TrimSpace` per line) before indentation-flexible (tier 3, `TrimLeft(" \t")` per line). `TrimSpace` is strictly more aggressive than `TrimLeft`, so any pure leading-indentation drift already re-matches at tier 2 — tier 3 is never the first unique hit. Verified empirically against `internal/fuzzy.Match`.
2. **`ellipsis` → captured `exact`.** The segmented ellipsis path reports the weakest tier across segments; un-drifted head/tail anchors all match exactly. The matched text is the collapsed `head\n...\ntail` form, so the golden exempts ellipsis cases from the 0.99 text-similarity floor (a low `editsim.ES` is correct there).

The duplicate-block ambiguous case had to be reworked: an initial whole-function duplicate matched only ONE site (unique → `whitespace_normalized`). The shipped construction is a shared BODY FRAGMENT present at TWO sites in `Source`, whose whitespace-drift matches both → `ErrAmbiguous` refusal in all three languages. This is the anti-vacuity proof.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Ambiguous case construction (whole-function duplicate matched only one site)**
- **Found during:** Task 3 (seeding captured outcomes via real `fuzzy.Match`)
- **Issue:** The Task-1 ambiguous `dupBlock` was a whole `func helper(){...}`; the whitespace-drifted search matched only the `helper` site uniquely → captured `whitespace_normalized`, NOT the required `ambiguous_match`. A vacuous ambiguous case would fail the FUZZBENCH anti-vacuity gate.
- **Fix:** Reworked all three languages' `dupBlock`/`dupSource` in `gen_corpus.go` to a shared body FRAGMENT present at two sites; regenerated the drift corpus. `fuzzy.Match` now refuses all three (`ambiguous_match`). Caught and corrected within Task 3, before the Task-1 corpus would have shipped a non-refusing case.
- **Files modified:** `bench/evaluators/fuzzyrobust/testdata/gen_corpus.go`, `testdata/drift/{go,python,rust}.json`
- **Commit:** folded into `e743a0c8` (Task 1 corpus) and validated in `5b047bad` (Task 3)

**2. [Rule 2 - Missing critical correctness] Source field on DriftCase**
- **Found during:** Task 1 → Task 3 wiring
- **Issue:** The harness must run `fuzzy.Match(haystack, search)` where the haystack for the ambiguous case is the two-site `Source`, not the single base block. Without a committed `Source` the harness could not distinguish the single-site haystack from the duplicate-block haystack.
- **Fix:** Added `Source string` to `DriftCase` and the generator; single-site cases use the base block, the ambiguous case uses the two-site source.
- **Commit:** `e743a0c8`

### Golden-test refinement (within scope)
- `TestScoreDriftCorpus` initially required >= 0.99 `editsim.ES` for every successful match; ellipsis matched-text is the collapsed form, so the floor now applies only to non-ellipsis successful matches (documented as a structural exemption). Not a deviation from plan intent — the plan's hermetic golden requirement is met.

## Pre-existing / Out-of-Scope (logged, not fixed)

- `cmd/helix-bench` `TestRunSubcommandWiresDeltaPass` fails on `go test ./...`. This is PRE-EXISTING — it fails identically on the 102-02 baseline commit (`e743a0c8~1`), and 102-02 never touched `cmd/helix-bench` (disjoint `bench/evaluators/fuzzyrobust/` + an ignore-tagged harness). The associated crosscodeeval/repobench dataset-fetch failures are network-gated HTTP 404s. Logged in `deferred-items.md`.

## Verification

- `go test ./bench/evaluators/fuzzyrobust/ -count=1` → **15/15 PASS** (NO `HELIX_BIN`, NO network — the sole authoritative proof).
- `go vet ./bench/...` → clean. `gofmt -l bench/evaluators/fuzzyrobust/ bench/runtime/fuzzy_robust_capture_regen.go` → no diff.
- `go build ./bench/runtime/` → succeeds and does NOT compile the `//go:build ignore` harness.
- `go run bench/runtime/fuzzy_robust_capture_regen.go` → regenerates the committed captured outcomes BYTE-IDENTICALLY (empty `git diff`).
- `go build ./cmd/helix` → OK. `go test ./...` → green except the pre-existing `cmd/helix-bench` failure above.

## Requirements Satisfied

- **FUZZBENCH-01** — the `fuzzyrobust` leaf measures 4-strategy selection + ambiguity refusal via `editsim.ES`, stdlib + editsim only (`TestLeafImports`).
- **FUZZBENCH-02** — broad py/go/rust drift corpus, expected strategy derived from the per-tier perturbation (not observed behavior), size floor enforced (`TestCorpusFloor`), >= 1 duplicate-block ambiguous case that MUST be refused (`TestAmbiguousRefused` + `TestAmbiguousBites`).

## Self-Check: PASSED

- All 13 created source/test/data files + 1 modified file present on disk (verified below).
- All 4 task commits present in git history (`e743a0c8`, `f4c9d6cf`, `5b047bad`, `235fdb0f`).
