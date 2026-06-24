# Phase 111 — Plan 01 SUMMARY

**Status:** Complete
**Requirements:** CORPUS-01, CORPUS-02
**Date:** 2026-06-24

## What shipped

- **`tools/dspy-tune/build_corpus.py`** (new) — dev-time Aider-polyglot corpus materializer. Shallow-clones `Aider-AI/polyglot-benchmark` @ pinned `7e0611e7…` (asserts HEAD==pin; reuses the cache on warm), enumerates `<track>/exercises/practice/*`, and emits one `_load_aider_corpus`-shaped descriptor per exercise (`{task, language, task_dir, gold_src, gold_tests}`, `gold_src==task_dir`==pristine dir, `gold_tests`==`config.files.test`). `--check` is a **fail-not-skip** gate: exit 1 if the requested build produced `< 101` descriptors. Writes a `corpus_manifest.json` (pinned sha + per-language counts + MIT-license pointer).
- **`tools/dspy-tune/optimize.py`** — added `split_corpus()` (3-way disjoint train/val/**heldout**), `_task_key()`, `_disjoint_ok()`. `main()` rewired: `train,val,heldout = split_corpus(examples)`; the **adoption gate moved to the sequestered held-out split** (`adoption_allowed(len(heldout))`) — the research-correct measure since GEPA leaks valset into candidate selection; `compile()` gets only train+val; the held-out descriptors are persisted to git-ignored `output/heldout_test.json` for attribution. `_load_aider_corpus` now skips the manifest / non-descriptor files.
- **`tools/dspy-tune/test_corpus.py`** (new, hermetic) — descriptor shape, practice-dir enumeration, 3-way disjoint + held-out `>50`, **planted-leak → assert-RED**, and compile-never-sees-test.
- **`tools/dspy-tune/testdata/fake_corpus_repo/`** (new) — tiny synthetic exercise so the descriptor test is network-free.
- **`.gitignore`** — ignore `/tools/dspy-tune/corpus/` (machine-local generated descriptors).

## Real materialization (CORPUS-01 proof)

```
$ uv run python build_corpus.py --tracks go,python,rust --check
materialized 103 descriptors into .../tools/dspy-tune/corpus
per_language: {'go': 39, 'python': 34, 'rust': 30}
OK: 103 >= 101   (exit 0)
```
`split_corpus` on 103 → `train=26 val=26 heldout=51` (held-out `51 > 50` ✓, pairwise disjoint ✓).

## Boundary (ADOPT-04)
- `git diff go.mod go.sum` empty; `make vet` green (all 8 analyzers incl. `vet-tools-quarantine`); `go build ./...` OK. All changes are dev-time Python under `tools/dspy-tune/` + `.gitignore` — no Go source touched.

## Deviations
- None. (Track selection go+python+rust chosen for self-contained, gradeable toolchains present in this env — the easiest route to ≥101 that Phase 113 can actually run.)
