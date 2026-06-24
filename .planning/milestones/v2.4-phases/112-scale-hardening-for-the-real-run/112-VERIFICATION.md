---
status: passed
phase: 112
verified: 2026-06-24
must_haves: 4
must_haves_verified: 4
---

# Phase 112 Verification — Scale Hardening for the Real Run

## Success Criteria

1. **SCALE-01 — optimizer DeepSeek-primary pin** ✓
   - `optimize.DEFAULT_LM_MODEL == "deepseek-v4-flash"` (NOT `deepseek-chat`/`deepseek-reasoner`). `resolve_lm({DEEPSEEK_API_KEY})` → `deepseek/deepseek-v4-flash` + base url; `{OPENAI_API_KEY}`-only → `openai/…` fallback; explicit `openai/gpt-4o` honored; no key → None. `main()` builds program LM + `reflection_lm` from the resolved config. Tests: `test_default_pin_is_deepseek_v4_flash`, `test_resolve_lm_*` GREEN; live `resolve_lm()` → `deepseek/deepseek-v4-flash`.

2. **SCALE-02 — cost caps wired (not silently unbounded)** ✓
   - `build_gepa_kwargs({})` → `auto="light"`, `num_threads=4` (bounded 1–8), `track_stats`, `seed=0`; `GEPA_MAX_METRIC_CALLS=200` → `max_metric_calls=200` and no `auto`. LM `num_retries=4` (429 backoff). Agent turn cap config-var (`AGENT_MAX_TURNS`, default 12) enforced — `test_agent_turn_cap_env_override` shows a tool-call-forever agent stops at 2 turns. Tests GREEN.

3. **SCALE-03 — SWE-bench 0-tests/`resolved` footgun refused** ✓
   - `grade_report` recomputes `resolved` from `tests_status` and raises `GradeError` on 0 tests; `test_ignores_harness_resolved_flag_on_zero_tests` (harness `resolved:true` + empty buckets ⇒ GradeError) + `test_zero_tests_is_hard_error` GREEN — break-the-invariant.

4. **Boundary (ADOPT-04)** ✓
   - `git diff go.mod go.sum` empty; `make vet` green; all changes dev-time Python under `tools/dspy-tune/`.

## Test evidence
- `uv run pytest test_scale.py test_swebench.py test_corpus.py test_split.py test_agent.py test_parity.py test_degenerate.py -q` → **49 passed**.
- `make vet` exit 0; `git diff --quiet go.mod go.sum` clean.

## Verdict
**PASSED** — 4/4 must-haves verified. The optimizer is DeepSeek-primary-pinned, the run is cost-bounded (rollout cap + bounded concurrency + 429 backoff + turn cap), and the SWE-bench footgun is refused. No human-verification items (hermetic; the billed run is Phase 113).
