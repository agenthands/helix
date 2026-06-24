# Project Research Summary

**Project:** Helix
**Milestone:** v2.4 — Corpus Growth & Real Optimization Verdict (TUNE-FUT-01)

Targeted research on the three external cost/logistics unknowns that drive a *cost-aware, for-real* run of the v2.3 task-success pipeline. The pipeline itself (ReAct agent, Aider + SWE-bench graders, GEPA metric, `helix-refgen --check` adoption gate) was built and verified in v2.3 and is **not** re-researched here. Supporting detail: [`DEEPSEEK.md`](DEEPSEEK.md), [`GEPA-COST.md`](GEPA-COST.md), [`SWEBENCH.md`](SWEBENCH.md).

## Key Findings

### 1. Model pin — DeepSeek (`deepseek-v4-flash`, pin explicitly NOW)
- Pin `DSPY_LM_MODEL=deepseek-v4-flash` (cheapest tool-calling model: $0.14 in / $0.28 out per 1M; cache-hit input $0.0028). Reserve `deepseek-v4-pro` ($0.435/$0.87) for harder reasoning only.
- The legacy `deepseek-chat`/`deepseek-reasoner` aliases **retire 2026-07-24 15:59 UTC** — inside this milestone's likely window — and `deepseek-reasoner` has **no function calling** (would break the ReAct loop). Pinning the explicit `deepseek-v4-*` id makes the cutover a one-line change and removes mid-run breakage risk.
- `base_url="https://api.deepseek.com"`, `openai==2.43.0`, standard `tools` calling. Limit is **concurrency** (2500 flash / 500 pro → HTTP 429): cap in-flight calls + enable retry/backoff. Stable system-prompt prefix → ~50× cache-hit input discount (the single biggest cost lever).

### 2. GEPA cost is small and corpus-growth-cheap
- `auto="light"`, 1-predictor, `valset≈55` ⇒ **~600 program rollouts** + ~10 reflection calls. **Trainset size is free under `auto`** — only valset + predictor-count drive the budget. So growing the corpus to clear `val_size>50` adds cost only via the gentle `~3·V` full-eval terms.
- Worked estimate on `deepseek-v4-flash`: ~5,400 agent calls ⇒ **≈ $5–15 per full optimization run** (cache-miss worst case; far less with prompt caching). The run is cheaply **re-runnable** — "cost-aware" is comfortably met.
- **Cost levers:** keep valset just above the gate (≈51–55); **cap the agent's `max_iters` (T)** (multiplies all 600 rollouts — highest leverage); cheaper `reflection_lm`; consider an explicit `max_metric_calls` for a hard cap.
- **Split safety (load-bearing):** GEPA reflects on **trainset** and scores/selects candidates on **valset** — so valset is *leaked into model selection*. The sequestered TEST/attribution split must **never** be passed as trainset OR valset. `optimize.py` already asserts train∩val=∅ and keeps TEST separate.

### 3. SWE-bench Verified confirming run — cheap K-slice, one grader footgun
- `princeton-nlp/SWE-bench_Verified` pin still resolves (500 instances, `test` split; mirror at `SWE-bench/`). v4.1.0's *default* dataset moved to `SWE-bench/SWE-bench_Lite`, so **always pass `--dataset_name`** (we do). There is no "Verified-lite"; for a *confirming* signal subsample Verified via `--instance_ids` — **do not** substitute Lite (a different instance set).
- Recommended **K = 25–50** Verified instances, **stratified across repos/difficulty** (~3–5 GiB image pull for 50, ~8 s/instance warm compute). Below ~15 the binomial noise is too high to confirm anything.
- **CRITICAL footgun:** the harness scores **0 tests evaluated as `resolved=True`** (`compute_fail_to_pass` returns `1.0` on `total==0`). Our `grade_swebench.py` must independently assert the F2P bucket is non-empty and matches the expected test ids (the Phase 109 "0 tests ⇒ GradeError" rule) — **not** trust the harness `resolved` flag. Re-verify this holds at corpus scale.
- **Podman:** our `podman system service --time=0 &` + `DOCKER_HOST` pin is the supported path; pre-check rootless storage (exec-capable, roomy `graphroot`), `/etc/subuid`+`/etc/subgid`, and the overlay driver. **Smoke 1–2 instances with `predictions_path=gold` first** before any batch.

## Implications for Roadmap

- **Corpus growth is the spine (TUNE-FUT-01).** The GEPA reward corpus is the **Aider-polyglot** set loaded via `_load_aider_corpus(AIDER_TASKS_DIR)` (the `data/{train,test}.jsonl` 8/3 rows are only the `choice_rate` diagnostic pre-screen). `optimize.py` splits 50/50, so the corpus needs **≳101 Aider tasks** for `valset>50`. The vendored loader already exposes ~225 tasks/6 tracks (Phase 85) — wire it up at scale + sequester a disjoint TEST/attribution split.
- **Pipeline hardening for scale (fix-as-needed) is a real phase, not an afterthought:** pin `deepseek-v4-flash`; add concurrency cap + 429 retry/backoff; cap agent `max_iters` as the cost lever; enable prompt-prefix caching; and re-verify `grade_swebench.py` defeats the `total==0 ⇒ resolved` footgun (break-the-invariant test). Defects surfaced by corpus-scale execution are in scope.
- **The real cost-aware GEPA run** (execute `optimize.py` → git-ignored `output/optimized.json`) + **ON/OFF attribution on the sequestered split** (per-arm cost via the metered LLM wrapper) produces the actual delta. Budget is small (~$5–15/run) so it can be run and, if needed, re-run.
- **SWE-bench confirming leg:** K=25–50 stratified Verified slice on Podman; gold-patch smoke first; HELIX_BIN/requested-run fail-not-skip honesty (no silent empty success).
- **REPORT-only close:** fill `tools/dspy-tune/REPORT.md` with real ON/OFF/Δ/`val_size`/per-arm-cost; keep adoption a separate human `helix-refgen --check`-gated SKILL.md edit (no auto-write, no SKILL.md commit this milestone). Re-verify the ADOPT-04 single-binary / no-runtime-Python boundary (zero new Go deps; new pins are dev-venv Python only) as a cross-cutting exit gate.
- **No new external dependencies needed** beyond the already-pinned dev-venv Python (`dspy==3.2.1`, `openai==2.43.0`, `swebench==4.1.0`) — research surfaced no library/version blockers; only config pins (model id, dataset name) and run logistics.

## Sources

- **DeepSeek:** https://api-docs.deepseek.com/quick_start/pricing · /quick_start/rate_limit · /guides/function_calling · /guides/reasoning_model
- **DSPy GEPA:** installed `dspy==3.2.1` source `dspy/teleprompt/gepa/gepa.py` (`auto_budget`, `AUTO_RUN_SETTINGS`, `GEPAFeedbackMetric`); DSPy GEPA docs (dspy.ai); GEPA paper
- **SWE-bench:** HF dataset cards (princeton-nlp / SWE-bench orgs); `swebench` v4.1.0 source (`run_evaluation.py`, `grading.py`, `reporting.py`, `constants/`); swebench.com FAQ + docker_setup guide; epoch.ai/blog/swebench-docker; Podman troubleshooting/rootless docs
