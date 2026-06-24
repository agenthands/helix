"""Hermetic contract test for the dev-time ReAct agent (tools/dspy-tune/agent/).

This is the Phase 107 RED baseline: it pins the FULL behavioral contract of the
`agent` package (AGENT-01/02/03 + the break-the-invariant anti-vacuity pair)
BEFORE the package exists, mirroring the flat, fixture-free, script-runnable
convention of test_parity.py / test_degenerate.py (pytest-discoverable AND
runnable as `python3 test_agent.py`).

Hermetic / no-network discipline (107-RESEARCH lines 437-438):
  * The tests inject a DUCK-TYPED fake LLM (a scripted `chat()` returning objects
    that expose exactly `.content`, `.tool_calls`, and `.model_dump(...)`) and a
    fake `helix` (monkeypatching `agent.tools.run_verb`). They construct NO
    `openai` client and reach NO socket — they pass even with `openai` UNINSTALLED.
  * They require NO API key: every test that touches the real LLM path
    monkeypatch.delenv's DEEPSEEK_API_KEY / OPENAI_API_KEY, proving the loud-fail
    path and the hermetic path are distinct (this shell may have keys set; the
    tests must still pass with them removed).

FAKE ASSISTANT-MESSAGE CONTRACT (the binding Task-1↔Task-3 surface; WARNING-2/-3):
107-RESEARCH Example 1 (line 259) appends the assistant message via
`messages.append(msg.model_dump(exclude_none=True))` BEFORE the tool-result
messages. Therefore the FakeAssistantMessage that FakeLLM.chat() returns
duck-types EXACTLY the surface react.py consumes and NOTHING else:
  * `.content`      — assistant text (None on a tool-call turn, str on the final turn).
  * `.tool_calls`   — a Python list of FakeToolCall (None/[] => final answer, reason="done").
  * each FakeToolCall exposes `.id` (str -> tool_call_id), `.function.name` (str:
    the kebab helix verb), `.function.arguments` (a JSON STRING, NOT a dict).
  * `.model_dump(exclude_none=True)` — the OpenAI-shaped assistant-message dict
    (option (a) pinned: the fake IMPLEMENTS model_dump; react.py MUST consume it,
    not hand-build a dict, or the OpenAI tool-call-ordering contract drifts away
    from RESEARCH Example 1).
react.py MUST consume exactly that attribute set — `.content`, `.tool_calls`,
`call.id`, `call.function.name`, `call.function.arguments` (JSON string), and
`msg.model_dump(exclude_none=True)`.
"""

import json
import os

import pytest

from agent.react import (
    OFF_CONTROL_PROMPT,
    STEERING_SENTINEL,
    ReActAgent,
    build_system_prompt,
)
from agent.tools import TOOL_SCHEMAS, VerbResult
import agent.tools as agent_tools
import agent.llm as agent_llm
from agent.llm import LLM


# --------------------------------------------------------------------------- #
# Inline duck-typed fakes (no conftest.py; no openai import; no network).      #
# --------------------------------------------------------------------------- #


class FakeFunction:
    """Duck-types `tool_call.function`: `.name` (str), `.arguments` (JSON str)."""

    def __init__(self, name, arguments):
        self.name = name
        self.arguments = arguments  # a JSON STRING, decoded by run_verb via json.loads


class FakeToolCall:
    """Duck-types an OpenAI tool_call: `.id`, `.function.name/.arguments`."""

    def __init__(self, call_id, name, arguments):
        self.id = call_id
        self.function = FakeFunction(name, arguments)


class FakeAssistantMessage:
    """Duck-types `choices[0].message` exactly as react.py consumes it.

    Exposes `.content`, `.tool_calls`, and `.model_dump(exclude_none=True)`.
    `model_dump` returns the OpenAI-shaped assistant-message dict with None
    fields dropped (option (a), pinned: react.py appends via this method).
    """

    def __init__(self, content=None, tool_calls=None):
        self.content = content
        self.tool_calls = tool_calls

    def model_dump(self, exclude_none=False):
        d = {"role": "assistant", "content": self.content}
        if self.tool_calls:
            d["tool_calls"] = [
                {
                    "id": c.id,
                    "type": "function",
                    "function": {
                        "name": c.function.name,
                        "arguments": c.function.arguments,
                    },
                }
                for c in self.tool_calls
            ]
        if exclude_none:
            d = {k: v for k, v in d.items() if v is not None}
        return d


class FakeLLM:
    """A scripted, duck-typed LLM: chat() pops the next pre-built message.

    Records every (messages, tools) call so a test can assert the assistant
    message was appended BEFORE the tool-result messages (the OpenAI ordering
    contract). Constructs no client, touches no network.
    """

    def __init__(self, scripted_messages):
        self._script = list(scripted_messages)
        self.calls = []  # snapshots of the messages list passed to each chat()

    def chat(self, messages, tools=None):
        # snapshot a shallow copy so later mutation doesn't rewrite history
        self.calls.append(list(messages))
        if self._script:
            return self._script.pop(0)
        # exhausted script => behave as a final-answer turn (defensive)
        return FakeAssistantMessage(content="(script exhausted)", tool_calls=None)


def _tool_turn(call_id, verb, args_dict):
    return FakeAssistantMessage(
        content=None,
        tool_calls=[FakeToolCall(call_id, verb, json.dumps(args_dict))],
    )


def _final_turn(text):
    return FakeAssistantMessage(content=text, tool_calls=None)


def _stub_run_verb(canned_stdout, canned_exit=0):
    """Return a run_verb stand-in that ignores the real `helix` binary and
    returns a canned VerbResult, recording the argv it would have run."""

    def _fake(call, cwd, timeout=60):
        verb = call.function.name
        args = json.loads(call.function.arguments or "{}")
        argv = ["helix", verb, *[str(v) for v in args.values()]]
        return VerbResult(argv=argv, exit=canned_exit, stdout=canned_stdout)

    return _fake


# --------------------------------------------------------------------------- #
# AGENT-01: bounded loop emits + observes; no-progress terminates.            #
# --------------------------------------------------------------------------- #


def test_loop_emits_and_observes(monkeypatch):
    """A fake LLM emits a curated helix verb on turn 0, then a final answer on
    turn 1. run_verb is stubbed to return canned stdout+exit 0. Assert the
    Transcript records the verb + argv + exit + stdout, the final answer is set,
    and reason == "done". Also assert the assistant tool-call message was
    appended to `messages` BEFORE the tool-result message (OpenAI ordering)."""
    monkeypatch.delenv("DEEPSEEK_API_KEY", raising=False)
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)

    llm = FakeLLM(
        [
            _tool_turn("call_0", "find-references", {"location": "x.go:3:5"}),
            _final_turn("done: found the references"),
        ]
    )
    monkeypatch.setattr(agent_tools, "run_verb", _stub_run_verb("REF: x.go:10:2\n"))
    # react.py imports run_verb into its own namespace; patch there too if present.
    import agent.react as agent_react
    if hasattr(agent_react, "run_verb"):
        monkeypatch.setattr(agent_react, "run_verb", _stub_run_verb("REF: x.go:10:2\n"))

    agent = ReActAgent(system_prompt="SYS", llm=llm)
    transcript = agent.run("find refs to X", cwd=".", max_turns=5)

    assert transcript.reason == "done"
    assert transcript.final == "done: found the references"
    assert len(transcript.steps) == 1
    step = transcript.steps[0]
    # the step records the verb, the fixed argv, the exit code, and the stdout.
    assert step.verb == "find-references"
    assert step.argv[0] == "helix" and step.argv[1] == "find-references"
    assert step.exit == 0
    assert "REF: x.go:10:2" in step.stdout

    # OpenAI ordering: on the turn-1 chat() call, the messages list must already
    # contain the assistant tool-call message followed by the tool-result message.
    turn1_messages = llm.calls[1]
    roles = [m.get("role") for m in turn1_messages]
    assert "assistant" in roles and "tool" in roles
    assert roles.index("assistant") < roles.index("tool"), (
        "assistant tool-call message must be appended before the tool result"
    )


def test_no_progress_terminates(monkeypatch):
    """A fake LLM emits the SAME tool call every turn (byte-identical argv) with
    identical canned output. Assert the loop terminates with reason ==
    "no_progress" within the cap (must NOT hang; bound max_turns small)."""
    monkeypatch.delenv("DEEPSEEK_API_KEY", raising=False)
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)

    # Emit the identical tool call on every turn — never a final answer.
    repeating = [
        _tool_turn("call_rep", "find-references", {"location": "x.go:3:5"})
        for _ in range(20)
    ]
    llm = FakeLLM(repeating)
    stub = _stub_run_verb("identical output\n")
    monkeypatch.setattr(agent_tools, "run_verb", stub)
    import agent.react as agent_react
    if hasattr(agent_react, "run_verb"):
        monkeypatch.setattr(agent_react, "run_verb", stub)

    agent = ReActAgent(system_prompt="SYS", llm=llm)
    transcript = agent.run("loop forever", cwd=".", max_turns=12)

    assert transcript.reason == "no_progress", (
        f"expected deterministic no_progress termination, got {transcript.reason!r}"
    )
    assert transcript.final is None


# --------------------------------------------------------------------------- #
# AGENT-02: loud-fail on missing key; model config var; provider fallback.    #
# --------------------------------------------------------------------------- #


def test_missing_key_raises(monkeypatch):
    """With DEEPSEEK_API_KEY unset, constructing/using the REAL LLM
    (provider="deepseek") for a real run RAISES RuntimeError (NOT a silent empty
    result — the explicit inversion of optimize.py's quiet `return 0`)."""
    monkeypatch.delenv("DEEPSEEK_API_KEY", raising=False)
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)

    with pytest.raises(RuntimeError):
        LLM(provider="deepseek")


def test_model_config_var(monkeypatch):
    """The default model id is "deepseek-v4-flash" when AGENT_MODEL is unset;
    setting AGENT_MODEL overrides it. Asserted on the constructed LLM's `.model`
    string with a dummy key monkeypatched in — NEVER a real network client."""
    monkeypatch.delenv("AGENT_MODEL", raising=False)
    monkeypatch.setenv("DEEPSEEK_API_KEY", "dummy-key-not-real")

    # The module-level default constant must be the alias-safe successor.
    assert agent_llm.DEFAULT_MODEL == "deepseek-v4-flash"

    default_llm = LLM(provider="deepseek")
    assert default_llm.model == "deepseek-v4-flash"

    monkeypatch.setenv("AGENT_MODEL", "deepseek-v4-pro")
    overridden = agent_llm.LLM(provider="deepseek", model=os.environ["AGENT_MODEL"])
    assert overridden.model == "deepseek-v4-pro"


def test_provider_fallback(monkeypatch):
    """Exercise the REAL DeepSeek->OpenAI fallback BRANCH (never a constructor
    that raised on an unset key; WARNING-4). Construct a real LLM with a dummy
    DEEPSEEK_API_KEY so the primary client is built, then:
      * replace llm.primary with a fake whose .chat.completions.create RAISES,
      * monkeypatch the module-level `agent.llm.OpenAI` symbol (used inside the
        except block to lazily build the fallback) to return a fake OpenAI client
        whose create() returns a SENTINEL message.
    With OPENAI_API_KEY present, assert chat() returns the FALLBACK sentinel
    (proving the except->fallback branch ran). With OPENAI_API_KEY absent after
    the same primary failure, assert chat() RAISES RuntimeError.
    Constructs NO real network client and imports NO openai."""
    monkeypatch.setenv("DEEPSEEK_API_KEY", "dummy-primary-key")

    sentinel_msg = FakeAssistantMessage(content="FALLBACK_SENTINEL", tool_calls=None)

    class _RaisingCompletions:
        def create(self, **kwargs):
            raise RuntimeError("simulated DeepSeek failure")

    class _RaisingChat:
        completions = _RaisingCompletions()

    class _FakePrimary:
        chat = _RaisingChat()

    class _FallbackCompletions:
        def create(self, **kwargs):
            class _Choice:
                message = sentinel_msg

            class _Resp:
                choices = [_Choice()]

            return _Resp()

    class _FallbackChat:
        completions = _FallbackCompletions()

    class _FakeFallbackClient:
        def __init__(self, *args, **kwargs):
            self.chat = _FallbackChat()

    # Build the real LLM (dummy key => primary client constructed, not raised),
    # then inject the raising primary via the documented seam.
    llm = LLM(provider="deepseek")
    llm.primary = _FakePrimary()
    # Force the except->fallback branch to build a fake OpenAI client.
    monkeypatch.setattr(agent_llm, "OpenAI", _FakeFallbackClient)

    monkeypatch.setenv("OPENAI_API_KEY", "dummy-fallback-key")
    result = llm.chat([{"role": "user", "content": "hi"}], tools=TOOL_SCHEMAS)
    assert result.content == "FALLBACK_SENTINEL", (
        "chat() must return the FALLBACK client's sentinel (except->fallback ran)"
    )

    # Now the fallback cannot be built: OPENAI_API_KEY absent => RuntimeError.
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    llm.primary = _FakePrimary()
    with pytest.raises(RuntimeError):
        llm.chat([{"role": "user", "content": "hi"}], tools=TOOL_SCHEMAS)


# --------------------------------------------------------------------------- #
# AGENT-03: steering ON injects sentinel; OFF provably omits it.              #
# --------------------------------------------------------------------------- #


def test_steering_on_off_omission():
    """ON injects the steering text under STEERING_SENTINEL; OFF returns an
    independent control prompt that provably OMITS the sentinel (the
    break-the-invariant control: leaking the sentinel into OFF turns this RED)."""
    steering_text = "Prefer helix verbs over grep when the question is semantic."
    on = build_system_prompt(steering_text, "on")
    off = build_system_prompt(steering_text, "off")

    assert STEERING_SENTINEL in on
    assert steering_text in on
    assert STEERING_SENTINEL not in off, (
        "OFF control prompt must provably omit the steering sentinel"
    )
    # OFF must be the independent constant, not a redaction of the steering text.
    assert off == OFF_CONTROL_PROMPT
    assert steering_text not in off


# --------------------------------------------------------------------------- #
# Anti-vacuity: a degenerate always-grep agent scores 0 / emits no helix verb. #
# --------------------------------------------------------------------------- #


def test_degenerate_always_grep_scores_zero(monkeypatch):
    """A fake LLM that ALWAYS emits a non-helix `grep` action drives the loop.
    Assert the transcript records NO helix-verb step and the run does NOT coerce
    a "solved" success.

    Break-the-invariant note: if the loop is mutated to coerce a default
    "solved"/done success (e.g. always return reason="done" with a fabricated
    final), OR if a degenerate non-helix action is silently counted as a helix
    step, this test goes RED. The agent must only ever record `helix <verb>`
    steps (the curated TOOL_SCHEMAS verb set), never a fabricated success."""
    monkeypatch.delenv("DEEPSEEK_API_KEY", raising=False)
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)

    # The degenerate model emits a non-helix `grep` tool call every turn. run_verb
    # is stubbed; whatever it "runs", the recorded argv must NEVER be a helix verb,
    # and no helix verb name from TOOL_SCHEMAS may appear in any step.
    grep_turns = [
        _tool_turn("call_grep", "grep", {"pattern": "TODO"}) for _ in range(20)
    ]
    llm = FakeLLM(grep_turns)

    def _grep_run_verb(call, cwd, timeout=60):
        # A degenerate non-helix action: record argv that is NOT a helix verb.
        return VerbResult(argv=["grep", "TODO"], exit=1, stdout="(no helix used)\n")

    monkeypatch.setattr(agent_tools, "run_verb", _grep_run_verb)
    import agent.react as agent_react
    if hasattr(agent_react, "run_verb"):
        monkeypatch.setattr(agent_react, "run_verb", _grep_run_verb)

    agent = ReActAgent(system_prompt="SYS", llm=llm)
    transcript = agent.run("do something", cwd=".", max_turns=6)

    helix_verb_names = {s["function"]["name"] for s in TOOL_SCHEMAS}
    # No step may record a helix verb, and the argv[0] must never be "helix".
    for step in transcript.steps:
        assert step.verb not in helix_verb_names, (
            f"degenerate agent must emit no helix verb, got {step.verb!r}"
        )
        assert step.argv[0] != "helix", (
            "degenerate agent must never produce a helix argv"
        )
    # The run must NOT coerce a fabricated success: a never-final degenerate loop
    # terminates on a bound (no_progress / max_turns), never reason="done".
    assert transcript.reason != "done", (
        "a degenerate never-finishing agent must not coerce a 'done' success"
    )
    assert transcript.final is None


if __name__ == "__main__":
    # Script-runnable path (the existing test_*.py convention). pytest's
    # monkeypatch fixture is unavailable here, so drive a minimal MonkeyPatch.
    from _pytest.monkeypatch import MonkeyPatch

    def _run(fn):
        import inspect

        params = inspect.signature(fn).parameters
        if "monkeypatch" in params:
            mp = MonkeyPatch()
            try:
                fn(mp)
            finally:
                mp.undo()
        else:
            fn()

    _run(test_loop_emits_and_observes)
    _run(test_no_progress_terminates)
    _run(test_missing_key_raises)
    _run(test_model_config_var)
    _run(test_provider_fallback)
    _run(test_steering_on_off_omission)
    _run(test_degenerate_always_grep_scores_zero)
    print("agent OK")
