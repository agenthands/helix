# Requirements: Helix — v2.4 Corpus Growth & Real Optimization Verdict (TUNE-FUT-01)

**Defined:** 2026-06-24
**Core Value:** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.

**Milestone goal:** Grow the optimization corpus past the strict `val_size > 50` held-out gate and run the v2.3 task-success pipeline for real (cost-aware, both tracks) to turn v2.3's "NO-SHIP by design" into an actual, numbers-backed adopt/no-adopt verdict — REPORT-only.

**Scope note (fix-as-needed):** This milestone reuses the v2.3 pipeline (the dev-time Python agent + Aider/SWE-bench graders + GEPA metric + `helix-refgen --check` gate). Defects that corpus-scale execution exposes in the agent (Phase 107) or graders (108/109) are in scope to fix, each with a regression test. The named hardening reqs below (SCALE-*) are the known-needed ones; ad-hoc fixes ride under the phase that surfaces them.

## v1 Requirements

Requirements for this milestone. Each maps to exactly one roadmap phase.

### Corpus (CORPUS) — the spine

- [ ] **CORPUS-01**: A materialized Aider-polyglot task-success corpus of **≥101 tasks** is produced/pointed at `AIDER_TASKS_DIR` so `optimize.py`'s 50/50 split yields **`val_size > 50`** (clears the TUNE-03 adoption gate). Reuses the vendored `bench/datasets/aider-polyglot` loader (~225 tasks / 6 tracks); **zero new Go deps**, dev-venv Python only.
- [ ] **CORPUS-02**: The held-out **TEST/attribution split is sequestered and disjoint** — never passed to `optimizer.compile()` as `trainset` OR `valset` (research: GEPA leaks `valset` into candidate selection). A test asserts `train ∩ val ∩ test = ∅` and that the attribution split is distinct from both.

### Scale Hardening (SCALE) — make the for-real run correct & bounded

- [ ] **SCALE-01**: `optimize.py`'s program LM **and** `reflection_lm` use an explicit, config-var model pin (`DSPY_LM_MODEL`) defaulting to **`deepseek-v4-flash`**, DeepSeek-primary / OpenAI-fallback consistent with the Phase-107 agent (replaces the `openai/gpt-4.1-mini` default + `OPENAI_API_KEY`-only guard). One-line cutover; legacy `deepseek-*` aliases (retire 2026-07-24) not used.
- [ ] **SCALE-02**: The run is **cost-bounded**: the agent's tool-turn cap (`max_iters`/`T`) is enforced, provider **concurrency is capped with HTTP-429 retry/backoff**, and a hard rollout cap is available (explicit `max_metric_calls` or `auto="light"`). A test asserts the caps are wired (not silently unbounded).
- [ ] **SCALE-03**: `grade_swebench.py` **hard-errors on the harness "0 tests evaluated ⇒ resolved=True" footgun** — it independently asserts the `FAIL_TO_PASS` bucket is non-empty and matches the expected test ids, never trusting the harness `resolved` flag. Shipped with a **break-the-invariant → assert-RED** test that confirms an empty-bucket eval is refused.

### Real Run (RUN)

- [ ] **RUN-01**: The cost-aware GEPA optimization (`optimize.py`) is **executed for real** against the grown corpus, writing the git-ignored `output/optimized.json` and printing `train`/`val` sizes that **prove `val_size > 50`** (the gate clears, not the no-ship short-circuit).
- [ ] **RUN-02**: An honest **ON-vs-OFF attribution** on the sequestered split produces a real task-success **delta with per-arm cost** (via the metered LLM wrapper) — the steering-ON vs steering-OFF comparison the v2.3 REPORT could not fill.
- [ ] **RUN-03**: The **SWE-bench Verified confirming leg** runs a **K=25–50 stratified** instance slice on Podman (gold-patch K≈2 smoke first). It is **fail-not-skip**: a requested real run that yields no result FAILS loudly (never a silent empty success).

### Verdict & Boundary (REPORT / ADOPT)

- [ ] **REPORT-01**: `tools/dspy-tune/REPORT.md` records the **real ship/no-ship verdict** — ON/OFF/Δ, `val_size`, per-arm cost, and the SWE-bench confirming agreement. **REPORT-only**: no `SKILL.md`/`reference.md` adoption is committed this milestone; adoption stays a separate human `helix-refgen --check`-gated decision.
- [ ] **ADOPT-05**: The **ADOPT-04 single-binary / no-runtime-Python boundary is re-verified end-to-end** at the new corpus scale — `go.mod`/`go.sum` untouched (zero new Go deps), no `helix` subcommand shells to Python, the agent/optimizer/graders stay off `helix setup` + default `go test ./...` + the merge path, and the human-gated `helix-refgen --check` adoption path is proven ready (optimizer writes only git-ignored output; no auto-write of the shipped surface).

## v2 Requirements

Deferred to a future milestone. Tracked, not in this roadmap.

### Future Tuning (TUNE)

- **TUNE-FUT-03**: If the verdict is a positive, gate-clearing delta, **adopt** the human-reviewed steering into `SKILL.md` (through `helix-refgen --check`) and ship it — the adoption this milestone deliberately defers (REPORT-only).
- **TUNE-FUT-04**: Grow the **`choice_rate` diagnostic pre-screen** corpus (`data/{train,test}.jsonl`, currently 8/3) commensurately, so the optional non-optimized pre-screen stays representative.
- **TUNE-FUT-05**: Broaden the confirming grader beyond a K-slice toward a larger SWE-bench Verified run if a stronger external signal is wanted.

## Out of Scope

| Feature | Reason |
|---------|--------|
| Committing an adopted `SKILL.md`/`reference.md` steering edit | REPORT-only by decision; adoption is a separate human-gated step (TUNE-FUT-03) |
| Any new Go module dependency / runtime Python in the binary | ADOPT-04 invariant — the shipped single binary stays Python-free; all new pins are dev-venv only |
| Re-running `optimize.py` in CI / as a merge gate | LLM optimization is not bit-reproducible; gate the committed artifact, never the optimizer process |
| Switching the confirming grader to SWE-bench_Lite | Lite is a different, non-overlapping instance set — cannot confirm a Verified signal |
| Rebuilding the agent / graders / GEPA metric from scratch | v2.3 already built them; this milestone grows the corpus + runs them (fix-as-needed only) |

## Traceability

Populated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CORPUS-01 | Phase 111 | Complete |
| CORPUS-02 | Phase 111 | Complete |
| SCALE-01 | Phase 112 | Pending |
| SCALE-02 | Phase 112 | Pending |
| SCALE-03 | Phase 112 | Pending |
| RUN-01 | Phase 113 | Pending |
| RUN-02 | Phase 113 | Pending |
| RUN-03 | Phase 113 | Pending |
| REPORT-01 | Phase 114 | Pending |
| ADOPT-05 | Phase 114 | Pending |

**Coverage:**
- v1 requirements: 10 total
- Mapped to phases: 10
- Unmapped: 0 ✓

---
*Requirements defined: 2026-06-24*
*Last updated: 2026-06-24 after initial definition*
