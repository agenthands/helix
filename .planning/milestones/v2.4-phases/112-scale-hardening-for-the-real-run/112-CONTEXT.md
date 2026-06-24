# Phase 112: Scale Hardening for the Real Run - Context

**Gathered:** 2026-06-24
**Status:** Ready for planning
**Mode:** Auto-generated (skip_discuss) + investigation findings

<domain>
## Phase Boundary
Make the for-real run correct and cost-bounded before spending — pin `deepseek-v4-flash` for the optimizer LM, bound cost (turns / concurrency / 429 / rollout cap), and defeat the SWE-bench "0 tests ⇒ resolved" footgun. Requirements: SCALE-01, SCALE-02, SCALE-03.
</domain>

<code_context>
## Investigated 2026-06-24

- **SCALE-01 gap**: `optimize.py` LM is `LM_MODEL = os.environ.get("DSPY_LM_MODEL", "openai/gpt-4.1-mini")` and `main()` reads `api_key = OPENAI_API_KEY` only (quiet `return 0` if absent). This is the DeepSeek-primary MISMATCH. The **agent** side (`agent/llm.py`) is already correct: `DEFAULT_MODEL = "deepseek-v4-flash"`, DeepSeek-primary/OpenAI-fallback, base_url `https://api.deepseek.com`, loud-fail-on-missing-key. The optimizer's program LM (`dspy.configure(lm=...)`) + `reflection_lm` must be brought into line.
- **dspy surface confirmed (installed 3.2.1)**: `dspy.LM(model, temperature, num_retries, **kwargs)` — `api_base`/`api_key` go through kwargs to litellm; `deepseek/<id>` is a native litellm provider. `dspy.GEPA(metric, auto, max_metric_calls, num_threads, reflection_lm, track_stats, seed, ...)` — `num_threads` and `max_metric_calls` are real kwargs.
- **SCALE-02**: agent loop already bounded (`react.py _DEFAULT_MAX_TURNS=12` + no-progress + tool-error budget). Missing: a config-var turn override, a bounded GEPA `num_threads`, LM `num_retries` (429 backoff — DeepSeek limits by concurrency → HTTP 429), and an explicit `max_metric_calls` hard-cap option (`auto="light"` ≈600 rollouts is the default).
- **SCALE-03 ALREADY DONE (Phase 109)**: `grade_swebench.grade_report` recomputes `resolved` from `tests_status` (never the harness flag), raises `GradeError` on 0 tests (`test_zero_tests_is_hard_error`) — defeats the research footgun (harness scores `total==0` as resolved). Fail-not-skip on a missing report.json. Only a small strengthening test is warranted.
- **Research (SUMMARY.md)**: pin `deepseek-v4-flash` explicitly (legacy aliases retire 2026-07-24; `deepseek-reasoner` has no tool-calling); cost levers = valset size + agent `max_iters` + cache; harness 0-tests footgun.
</code_context>

<decisions>
1. Add `resolve_lm(environ)` to `optimize.py`: DeepSeek-primary / OpenAI-fallback, default model `deepseek-v4-flash` (config var `DSPY_LM_MODEL`), returning a small config (model/api_key/api_base/num_retries). `main()` uses it for BOTH the program LM and `reflection_lm`; returns None ⇒ the existing quiet hermetic-gate exit-0.
2. Add `build_gepa_kwargs(environ)`: `auto="light"` by default, OR `max_metric_calls=int(GEPA_MAX_METRIC_CALLS)` when set (explicit hard cap); bounded `num_threads` (default 4, `GEPA_NUM_THREADS`); `track_stats=True`, `seed` fixed.
3. LM `num_retries=4` (429 backoff). Agent turn cap configurable via `AGENT_MAX_TURNS` (default 12) — the cost lever.
4. SCALE-03: keep the Phase-109 grader; add `test_ignores_harness_resolved_flag_on_zero_tests` (harness `resolved:true` + empty buckets ⇒ still `GradeError`) — encode the research footgun explicitly.
5. Boundary (ADOPT-04): dev-time Python only; `git diff go.mod` empty; `make vet` green.
</decisions>

<specifics>
- Edit: `optimize.py` (resolve_lm + build_gepa_kwargs + main rewire), `agent/react.py` (AGENT_MAX_TURNS env).
- New: `test_scale.py` (hermetic — resolve_lm matrix, gepa-kwargs caps, lm num_retries, agent turn cap, default-pin-not-deprecated-alias). Add 1 test to `test_swebench.py`.
</specifics>

<deferred>
- The actual billed run (uses this config) → Phase 113.
</deferred>
