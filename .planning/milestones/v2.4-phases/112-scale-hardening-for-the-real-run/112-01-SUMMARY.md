# Phase 112 — Plan 01 SUMMARY

**Status:** Complete
**Requirements:** SCALE-01, SCALE-02, SCALE-03
**Date:** 2026-06-24

## What shipped

- **SCALE-01 — optimizer DeepSeek-primary LM pin** (`optimize.py`): added `DEFAULT_LM_MODEL="deepseek-v4-flash"`, `DEFAULT_FALLBACK_MODEL`, `DEEPSEEK_BASE_URL`, `LMConfig`, and `resolve_lm(environ)` (DeepSeek-primary / OpenAI-fallback, config var `DSPY_LM_MODEL`, `num_retries=4`). `main()` rewired: the program LM (`dspy.configure`) and `reflection_lm` are both built via `_build_lm()` from the resolved config; the missing-key guard now checks the selected provider (DeepSeek primary / OpenAI fallback) and keeps the quiet hermetic-gate exit-0. Replaces the old `openai/gpt-4.1-mini` default + `OPENAI_API_KEY`-only guard. Verified live: `resolve_lm()` → `deepseek/deepseek-v4-flash`, api_base `https://api.deepseek.com`, retries 4.
- **SCALE-02 — cost caps**: `build_gepa_kwargs(environ)` → `auto="light"` (~600 rollouts) by default, OR `max_metric_calls=int(GEPA_MAX_METRIC_CALLS)` (explicit hard cap, replaces auto), plus bounded `num_threads` (default 4, `GEPA_NUM_THREADS`, clamped 1–8 so concurrency never goes unbounded into DeepSeek's 429 limit) + `track_stats` + fixed `seed=0`. `GEPA(...)` now takes `**build_gepa_kwargs()`. LM `num_retries=4` gives 429 backoff. Agent turn cap is now a config var: `agent/react.py::_resolve_max_turns()` (explicit > `AGENT_MAX_TURNS` env > default 12), wired into both `run()` entry points — the highest-leverage cost knob after the held-out split.
- **SCALE-03 — SWE-bench footgun re-verify**: confirmed the Phase-109 `grade_swebench.grade_report` already recomputes `resolved` from `tests_status` and raises `GradeError` on 0 tests (defeating the harness's `compute_fail_to_pass`→1.0-on-`total==0`). Added `test_ignores_harness_resolved_flag_on_zero_tests` (a report carrying `resolved:true` + empty buckets must STILL raise) — encodes the research footgun explicitly as a break-the-invariant test.
- **New tests**: `test_scale.py` (11 hermetic tests — LM resolution matrix, default-pin-not-deprecated-alias, GEPA cap kwargs, 429 retries, agent turn-cap env override + default-12). +1 in `test_swebench.py`.

## Verification
- `uv run pytest test_scale.py test_swebench.py test_corpus.py test_split.py test_agent.py test_parity.py test_degenerate.py -q` → **49 passed** (key-free, network-free).
- `git diff go.mod go.sum` empty; `make vet` green (incl. `vet-tools-quarantine`). Dev-time Python only.

## Deviations
- None. SCALE-03 was largely pre-satisfied by Phase 109; this phase re-verified it and added the explicit footgun test rather than rebuilding the grader.
