# Phase 111: Corpus Growth & Sequestered Split - Context

**Gathered:** 2026-06-24
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss) + investigation findings folded in

<domain>
## Phase Boundary

Materialize a ≥101-task Aider-polyglot task-success corpus at `AIDER_TASKS_DIR` so `optimize.py`'s split clears `val_size > 50`, with a disjoint sequestered TEST/attribution split — the literal v2.2/v2.3 no-ship axis, proven before any spend. Requirements: CORPUS-01, CORPUS-02.
</domain>

<code_context>
## Existing Code Insights (investigated 2026-06-24)

- **Reward corpus source**: `optimize.py::_load_aider_corpus(dspy, tasks_dir)` reads `*.json` descriptors from `AIDER_TASKS_DIR`. Each descriptor: `{task, language, task_dir, gold_src, gold_tests[]}`. The `data/{train,test}.jsonl` (8/3 rows) are the `choice_rate` **diagnostic pre-screen only** — NOT this corpus.
- **Current split** (`optimize.py` ~line 130): `split = len//2; train, val = examples[:split], examples[split:]`; gate `adoption_allowed(len(valset))` (strict `>50`). Only 2-way today.
- **Sandbox/grader contract**: `sandbox.make_sandbox(task_dir)` copies the pristine dir into a throwaway temp; `restore_gold_tests(sb, gold_src, gold_tests)` overwrites tests from the agent-unwritable pristine `gold_src`. So **`gold_src == task_dir`** (the pristine clone path); the agent edits only the copy. `grade_aider.grade_task(sandbox, language)` runs the native test cmd; **0 tests ⇒ GradeError**.
- **Vendored loader** (`bench/datasets/aider-polyglot/`): Go leaf, pinned `Aider-AI/polyglot-benchmark @ 7e0611e7`. Exercism layout `<lang>/exercises/practice/<ex>/` with `.meta/config.json` (`files.solution`/`files.test`/`files.example`) + `.docs/instructions.md` (the prompt). Counts: cpp 26, go 39, java 47, javascript 49, python 34, rust 30 (225 total).
- **Attribution** (`attribution.py`): `compute_attribution(examples, run_arm)` scores ON vs OFF on a held-out split; `decide_ship` gates on `attr.val_size > VAL_SIZE_GATE` AND `delta > 0`. The held-out attribution split must be the sequestered one (never passed to `compile()`).
- **Research (SUMMARY.md)**: GEPA reflects on trainset, scores/selects candidates on **valset → valset is leaked into selection**. So the trustworthy adoption gate belongs on a THIRD sequestered split that `compile()` never sees.

## Environment (probed 2026-06-24)
- Network ✓ (clone HEAD == pinned sha). Toolchains: go ✓, uv/uvx ✓, cargo ✓, node/npm ✓, podman ✓ (docker absent). HELIX_CACHE_DIR unset → `~/.cache/helix`.
- ⚠️ `DEEPSEEK_API_KEY` (len 16) / `OPENAI_API_KEY` (len 14) look like **placeholders** — irrelevant to this hermetic phase; a Phase-113 concern.
</code_context>

<decisions>
## Implementation Decisions

1. **Track selection**: materialize **go + python + rust = 103** exercises — the three with confirmed self-contained toolchains here (go test / pytest / cargo test), clearing ≥101 and gradeable in Phase 113. Materializer is track-parameterized for easy adjustment.
2. **3-way disjoint split** (CORPUS-02): introduce `split_corpus(examples) -> (train, val, test)` in `optimize.py`. `compile()` gets `train` + `val`; `test` is the **sequestered attribution split**, never passed to `compile()`. The adoption gate moves to the **held-out attribution split size** (`len(test) > 50`) — the research-correct measure.
3. **Corpus is generated, not committed**: descriptors reference machine-local cache paths, so the corpus dir + a `corpus_manifest.json` are **git-ignored** (like `output/`). Hermetic tests use a small synthetic fixture; a separate requested-build count check (fail-not-skip) proves the real corpus ≥101.
4. **Anti-vacuity**: a planted TEST→train/val leak turns the disjointness test RED; the `val_size==50` no-ship / `51`+ adoptable boundary is preserved.
5. **Boundary (ADOPT-04)**: all work is dev-time Python under `tools/dspy-tune/` + the existing Go loader; `git diff go.mod` empty; `make vet` green.
</decisions>

<specifics>
## Specific Ideas
- New: `tools/dspy-tune/build_corpus.py` (materializer, idempotent, pinned-sha, `--check` count gate) + `tools/dspy-tune/test_corpus.py` (hermetic split tests).
- Edit: `optimize.py` (3-way `split_corpus`, gate on held-out attribution split), `.gitignore` (corpus dir), `attribution.py`/`_main` wiring to consume the sequestered split (light touch; full ON/OFF run is Phase 113).
- Provenance: record the pinned sha + per-language counts in `corpus_manifest.json`; reuse the existing `LICENSE-AUDIT.md` (Exercism MIT) — no new fixtures committed.
</specifics>

<deferred>
## Deferred Ideas
- The actual billed GEPA run + ON/OFF attribution + SWE-bench confirming → Phase 113.
- Model-id pin, cost caps, grade_swebench footgun → Phase 112.
</deferred>
