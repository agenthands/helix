---
phase: 107-react-tool-using-agent-deepseek-openai-client-on-off-steerin
plan: 01
subsystem: tooling
tags: [dspy-tune, react-agent, openai, deepseek, helix-cli, uv, pytest]

requires:
  - phase: v2.2 Phase 106
    provides: tools/dspy-tune/ DSPy GEPA harness (optimize.py, scorer.py) — the sibling the agent package lands next to
provides:
  - Importable dev-time ReAct agent (tools/dspy-tune/agent/) that drives `helix <verb>` via fixed-argv subprocess in a bounded loop
  - Provider-agnostic LLM.chat() — DeepSeek primary / OpenAI fallback, AGENT_MODEL config var, loud-fail on missing key
  - Steering ON/OFF system-prompt builder (STEERING_SENTINEL + independent OFF_CONTROL_PROMPT)
  - Hermetic test_agent.py with the anti-vacuity break-the-invariant pair
affects: [Phase 108 (Aider task-success metric imports this agent in-process), Phase 109, Phase 110]

tech-stack:
  added: [openai==2.43.0 (dev venv only)]
  patterns: [in-process-importable Python agent under tools/, fixed-argv subprocess shim, duck-typed-fake hermetic tests, uv/uvx venv]

key-files:
  created:
    - tools/dspy-tune/agent/__init__.py
    - tools/dspy-tune/agent/llm.py
    - tools/dspy-tune/agent/tools.py
    - tools/dspy-tune/agent/react.py
    - tools/dspy-tune/test_agent.py
  modified:
    - tools/dspy-tune/requirements.txt
    - tools/dspy-tune/README.md
    - CLAUDE.md

key-decisions:
  - "All Python work uses uv/uvx + venv (never bare python/pip) — recorded in CLAUDE.md after user correction"
  - "Agent is Python under tools/dspy-tune/agent/ (in-process metric import), not Go bench/runtime — zero new Go deps"
  - "llm.py guards `from openai import OpenAI` so the hermetic suite passes with openai uninstalled; OpenAI is the fallback monkeypatch seam"
  - "OFF control prompt is an independent constant, never a runtime redaction of the steering text"

patterns-established:
  - "Anti-vacuity: break-the-invariant tests mutation-verified to RED-flip before declaring done"
  - "Loud-fail RAISE on missing provider key (inverts optimize.py's quiet return 0)"

requirements-completed: [AGENT-01, AGENT-02, AGENT-03]

duration: ~25min
completed: 2026-06-24
status: complete
---

# Phase 107: ReAct Tool-Using Agent + DeepSeek/OpenAI Client + ON/OFF Steering — Summary

**Shipped a dev-time, importable Python ReAct agent that drives the real `helix` CLI in a bounded loop with DeepSeek-primary/OpenAI-fallback provider selection and a first-class steering ON/OFF switch — the foundation the v2.3 task-success metric (Phase 108) imports in-process.**

## Performance

- **Duration:** ~25 min (incl. a mid-run correction to enforce `uv`/venv conventions)
- **Tasks:** 3 (TDD RED → GREEN → GREEN), executed inline with `uv`
- **Files:** 4 created (agent package), 1 hermetic test, 3 modified (requirements, README, CLAUDE.md)

## What was built

- **`agent/llm.py`** — one `openai` client wrapping both providers: DeepSeek primary (`base_url=https://api.deepseek.com`, model `AGENT_MODEL` default `deepseek-v4-flash`, alias-deprecation-safe), OpenAI fallback. **Loud-fail RAISE** on a missing provider key (the explicit inversion of `optimize.py`'s quiet `return 0`). The `from openai import OpenAI` import is guarded so the hermetic suite runs with `openai` uninstalled; the module-level `OpenAI` symbol is the documented fallback monkeypatch seam.
- **`agent/tools.py`** — fixed-argv `helix <verb>` subprocess shim (`subprocess.run([...])`, never `shell=True`) + curated routing-table `TOOL_SCHEMAS`; `VerbResult` dataclass.
- **`agent/react.py`** — bounded think→act→observe loop with three deterministic termination reasons (`done` / `no_progress` / `tool_error_budget` / `max_turns`), assistant-message-before-tool-results ordering via `model_dump(exclude_none=True)`, `Transcript`/`Step` trace, and `build_system_prompt` with `STEERING_SENTINEL` + an independent `OFF_CONTROL_PROMPT`.
- **`test_agent.py`** — flat, fixture-free, script-runnable hermetic suite (fake LLM + fake `helix`, no network, keys unset): 7 tests covering AGENT-01/02/03 plus the **anti-vacuity break-the-invariant pair**.

## Verification

- **7/7 hermetic tests pass** via `uv run pytest test_agent.py` AND `uv run python test_agent.py` (`agent OK`), with both API keys unset.
- **Anti-vacuity mutation-confirmed (folded before verify):** leaking `STEERING_SENTINEL` into OFF → `test_steering_on_off_omission` RED; coercing a fabricated "done" in the no-progress branch → both `test_no_progress_terminates` and `test_degenerate_always_grep_scores_zero` RED. Reverted; suite green.
- **No regression:** existing `test_split.py`/`test_parity.py`/`test_degenerate.py` all green.
- **Boundary (ADOPT-04):** `git diff go.mod` empty, no new `.go` under `tools/`, `make vet` (incl. `vet-tools-quarantine`) green, no `shell=True` in `agent/`.

## Deviations

- **uv/venv convention** (user correction mid-run): the initial executor dispatch used bare `python -m pytest`/`pip`. Stopped, recorded the `uv`/`uvx`+venv rule in CLAUDE.md + memory, converted `requirements.txt` and the dspy-tune README from `python3 -m venv`/`pip` to `uv`, and finished the phase inline with `uv`.
- **Subagent roster changed mid-milestone** (`gsd-executor`/`gsd-verifier` no longer available; replaced by `engineer`/`qa`/`anvil-*`). Per user choice, Phase 107 was finished inline rather than via the (now-unavailable) gsd-executor.
