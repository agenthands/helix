---
status: passed
phase: 111
verified: 2026-06-24
must_haves: 5
must_haves_verified: 5
---

# Phase 111 Verification — Corpus Growth & Sequestered Split

Goal-backward verification against the phase success criteria. Verified against code reality + real execution, not the SUMMARY.

## Success Criteria

1. **CORPUS-01 — ≥101 descriptors, correct shape** ✓
   - `build_corpus.py --tracks go,python,rust --check` materialized **103** descriptors (go 39 + python 34 + rust 30), exit 0; `corpus_manifest.json` records the pinned sha + counts. `--check` is fail-not-skip (exit 1 if `<101`).
   - `test_build_corpus_descriptor_shape` asserts the exact 5-field shape `_load_aider_corpus` consumes, `gold_src==task_dir`, `gold_tests==config.files.test`, non-empty `task`. GREEN.

2. **CORPUS-02 — 3-way sequestered split, held-out `>50`, never to compile()** ✓
   - `split_corpus(103)` → `train=26 val=26 heldout=51`; `heldout=51 > 50`. `main()` passes only `train`+`val` to `compile()`; held-out persisted to git-ignored `output/heldout_test.json`. Gate moved to `adoption_allowed(len(heldout))`.
   - `test_compile_never_sees_test` + `test_split_three_way_disjoint_and_heldout_gt_gate` GREEN.

3. **Anti-vacuity — planted leak → RED; gate boundary preserved** ✓
   - `test_planted_leak_goes_red`: injecting a held-out task into train makes `_disjoint_ok` return False (break-the-invariant). GREEN.
   - `test_split.py::test_val_size_adoption_gate_boundary` (50 no-ship / 51 adoptable / tiny-corpus no-ship) still GREEN — gate semantics unchanged.

4. **Provenance, no fabricated tasks** ✓
   - Descriptors point at the pinned upstream clone; `corpus_manifest.json` cites Exercism MIT via `bench/datasets/aider-polyglot/LICENSE-AUDIT.md`. No task content hand-authored (only the synthetic test fixture, clearly under `testdata/`).

5. **Boundary (ADOPT-04)** ✓
   - `git diff go.mod go.sum` empty; `make vet` green (8 analyzers incl. `vet-tools-quarantine`); `go build ./...` OK. All changes dev-time Python + `.gitignore`.

## Test evidence
- `uv run pytest test_corpus.py test_split.py test_parity.py test_degenerate.py -q` → **13 passed** (key-free, network-free).
- `uv run python build_corpus.py --tracks go,python,rust --check` → 103, exit 0.
- `make vet` → exit 0; `go build ./...` → OK; `git diff --quiet go.mod go.sum` → clean.

## Verdict
**PASSED** — 5/5 must-haves verified. The corpus clears the `val_size>50` gate (held-out 51), the sequestered split is disjoint and never reaches `compile()`, and the boundary holds. No human-verification items (hermetic phase; no billed run, no UI).
