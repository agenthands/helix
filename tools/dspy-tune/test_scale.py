"""Hermetic tests for Phase 112 scale-hardening (SCALE-01 / SCALE-02).

DEV-TIME / OFFLINE ONLY — no network, no LM key, no toolchain. Covers:
  * SCALE-01: optimizer LM is DeepSeek-primary / OpenAI-fallback, default pinned
    to the explicit (non-deprecated) `deepseek-v4-flash` id.
  * SCALE-02: cost caps are WIRED, not silently unbounded — GEPA rollout cap
    (auto=light or explicit max_metric_calls), bounded num_threads, LM 429-backoff
    num_retries, and a config-var agent turn cap.
"""

import pytest

import optimize
from agent.tools import VerbResult
import agent.react as react
from agent.react import ReActAgent


# --- SCALE-01: optimizer model pin (DeepSeek-primary / OpenAI-fallback) ------ #

def test_default_pin_is_deepseek_v4_flash():
    assert optimize.DEFAULT_LM_MODEL == "deepseek-v4-flash"
    # Must NOT default to a deprecation-bound alias (retire 2026-07-24); the
    # reasoner alias additionally has no tool-calling.
    assert optimize.DEFAULT_LM_MODEL not in ("deepseek-chat", "deepseek-reasoner")


def test_resolve_lm_deepseek_primary():
    lm = optimize.resolve_lm({"DEEPSEEK_API_KEY": "dsk-x"})
    assert lm is not None
    assert lm.model == "deepseek/deepseek-v4-flash"
    assert lm.api_key == "dsk-x"
    assert lm.api_base == "https://api.deepseek.com"


def test_resolve_lm_openai_fallback_when_only_openai_key():
    lm = optimize.resolve_lm({"OPENAI_API_KEY": "sk-y"})
    assert lm is not None
    assert lm.model.startswith("openai/")
    assert lm.api_key == "sk-y"
    assert lm.api_base is None


def test_resolve_lm_respects_explicit_openai_model():
    lm = optimize.resolve_lm({"OPENAI_API_KEY": "sk-y", "DSPY_LM_MODEL": "openai/gpt-4o"})
    assert lm is not None and lm.model == "openai/gpt-4o"


def test_resolve_lm_none_without_any_key():
    assert optimize.resolve_lm({}) is None


def test_resolve_lm_num_retries_for_429_backoff():
    lm = optimize.resolve_lm({"DEEPSEEK_API_KEY": "dsk-x"})
    assert lm.num_retries >= 3, "429 backoff retries must be wired (DeepSeek limits by concurrency)"


# --- SCALE-02: GEPA rollout / concurrency caps ------------------------------ #

def test_gepa_kwargs_default_auto_light():
    kw = optimize.build_gepa_kwargs({})
    assert kw.get("auto") == "light"
    assert "max_metric_calls" not in kw
    assert 1 <= kw.get("num_threads", 0) <= 8, "num_threads must be bounded, not unbounded"
    assert kw.get("track_stats") is True
    assert "seed" in kw  # determinism


def test_gepa_kwargs_explicit_max_metric_calls_overrides_auto():
    kw = optimize.build_gepa_kwargs({"GEPA_MAX_METRIC_CALLS": "200"})
    assert kw.get("max_metric_calls") == 200
    assert "auto" not in kw, "an explicit hard cap must replace auto, not coexist"


def test_gepa_kwargs_num_threads_env_override_still_bounded():
    kw = optimize.build_gepa_kwargs({"GEPA_NUM_THREADS": "3"})
    assert kw["num_threads"] == 3


# --- SCALE-02: agent turn cap (cost lever) is a config var ------------------- #

class _ToolFn:
    def __init__(self, name):
        self.name = name
        self.arguments = "{}"


class _ToolCall:
    def __init__(self, i):
        self.id = f"c{i}"
        self.function = _ToolFn("search-in-files")


class _Msg:
    """Always emits a tool call so the loop never returns 'done'."""
    def __init__(self, i):
        self.content = None
        self.tool_calls = [_ToolCall(i)]

    def model_dump(self, exclude_none=False):
        return {"role": "assistant", "content": None, "tool_calls": []}


class _ForeverLLM:
    def __init__(self):
        self.n = 0

    def chat(self, messages, tools=None):
        self.n += 1
        return _Msg(self.n)


def test_agent_turn_cap_env_override(monkeypatch):
    # Vary verb output per call so the no-progress bound does not fire before the cap.
    counter = {"i": 0}

    def fake_run_verb(call, cwd):
        counter["i"] += 1
        return VerbResult(argv=["helix", "search-in-files", str(counter["i"])], exit=0,
                          stdout=f"out-{counter['i']}")

    monkeypatch.setattr(react, "run_verb", fake_run_verb)
    monkeypatch.setenv("AGENT_MAX_TURNS", "2")
    agent = ReActAgent(system_prompt="x", llm=_ForeverLLM())
    t = agent.run("task", cwd=".")  # no explicit max_turns -> env-resolved
    assert t.reason == "max_turns"
    assert len(t.steps) == 2, f"AGENT_MAX_TURNS=2 must cap at 2 turns, got {len(t.steps)}"


def test_agent_turn_cap_default_is_twelve(monkeypatch):
    monkeypatch.delenv("AGENT_MAX_TURNS", raising=False)
    assert react._resolve_max_turns(None) == 12


if __name__ == "__main__":
    import sys
    sys.exit(pytest.main([__file__, "-q"]))
