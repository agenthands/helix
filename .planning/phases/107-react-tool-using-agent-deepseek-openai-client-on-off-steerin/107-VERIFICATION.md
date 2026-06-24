---
phase: 107
slug: react-tool-using-agent-deepseek-openai-client-on-off-steerin
status: passed
verified: 2026-06-24
method: inline (executed + verified by orchestrator with uv; gsd-verifier agent unavailable mid-milestone)
---

# Phase 107 — Verification Report

**Verdict: PASSED** — the codebase delivers the phase goal (a dev-time, OpenAI-compatible tool-using agent driving the real `helix` CLI in a bounded ReAct loop, config-driven provider, first-class steering ON/OFF). Verified goal-backward against code reality, not just task completion.

## Must-haves (goal-backward)

| # | Truth | Verdict | Evidence |
|---|-------|---------|----------|
| 1 | Bounded ReAct loop emits `helix <verb>` argv via subprocess, observes stdout/exit, returns a transcript with a termination reason (AGENT-01) | ✅ | `react.py` loop + `Transcript.reason ∈ {done,no_progress,tool_error_budget,max_turns}`; `test_loop_emits_and_observes` green |
| 2 | A repeating/no-progress agent deterministically terminates with `no_progress` (no infinite loop) (AGENT-01) | ✅ | `test_no_progress_terminates` green (max_turns=12, terminates at turn 1 via byte-identical-sig heuristic) |
| 3 | Config-driven LM: DeepSeek primary (`base_url=https://api.deepseek.com`) + OpenAI fallback; model `AGENT_MODEL` default `deepseek-v4-flash` (AGENT-02) | ✅ | `llm.py`; `test_model_config_var` green; grep confirms `deepseek-v4-flash` + `base_url` |
| 4 | A real run with the selected provider's key absent RAISES loudly (not silent skip) (AGENT-02) | ✅ | `test_missing_key_raises` green (RuntimeError); inverts optimize.py's quiet `return 0` |
| 5 | DeepSeek failure falls back to OpenAI when `OPENAI_API_KEY` present (AGENT-02) | ✅ | `test_provider_fallback` green — provably takes the except→fallback branch via the `agent.llm.OpenAI` seam; raises when fallback key absent |
| 6 | `build_system_prompt(text,'on')` injects steering under a sentinel; `'off'` returns an independent control prompt that provably omits it (AGENT-03) | ✅ | `test_steering_on_off_omission` green |
| 7 | Hermetic anti-vacuity pair (degenerate always-grep scores 0; OFF omits sentinel) — break-the-invariant → RED | ✅ | Both green AND mutation-confirmed to RED-flip (sentinel-leak → steering RED; coerce-done → degenerate+no_progress RED) |

## Gates

- `uv run pytest test_agent.py` → **7 passed**; `uv run python test_agent.py` → `agent OK`.
- Existing harness suite (`test_split`/`test_parity`/`test_degenerate`) → **7 passed** (no regression).
- `make vet` (incl. `vet-tools-quarantine`) → **green**.
- `git diff --exit-code go.mod` → **empty**; no new `.go` under `tools/`; no `shell=True` in `agent/`.

## Requirements coverage

AGENT-01 ✅, AGENT-02 ✅, AGENT-03 ✅ (3/3).

## Human verification

None required — all behaviors have hermetic automated coverage. The single live-only behavior (DeepSeek `deepseek-v4-flash` first-attempt tool-calling reliability) is deferred to a real run in Phase 108, per VALIDATION.md.
