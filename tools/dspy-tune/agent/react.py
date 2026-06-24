"""Bounded ReAct loop + steering ON/OFF system-prompt builder + transcript.

DEV-TIME / OFFLINE ONLY. This module lives under tools/ and is NEVER linked
into the helix binary, `helix setup`, go.mod, or the default `go test ./...`
path. It is imported in-process by the Phase-108 GEPA task-success metric.

The loop drives `helix <verb>` via `agent.tools.run_verb` (fixed-argv
subprocess), observes each verb's stdout/exit, and returns a Transcript with a
deterministic termination reason. Two bounds prevent runaway loops (T-107-03):
a hard `max_turns` cap and a no-progress heuristic (byte-identical argv+output
across consecutive turns), plus a tool-error budget.
"""

import json
from dataclasses import dataclass, field
from typing import Optional

from agent.tools import run_verb, TOOL_SCHEMAS

# A distinctive token that appears ONLY in the steered (ON) system prompt. The
# OFF control prompt is an INDEPENDENT constant that provably omits it — the
# break-the-invariant control for AGENT-03 (leaking this into OFF turns the
# omission test RED).
STEERING_SENTINEL = "<<HELIX_STEERING>>"

_BASE_SYSTEM_PROMPT = (
    "You are a coding agent working in a repository. You drive the `helix` CLI "
    "by emitting tool calls; each runs `helix <verb>` and returns its output. "
    "When you have the answer, respond with a final message and no tool call."
)

# Independent OFF control prompt — NOT derived from the steering text at runtime.
OFF_CONTROL_PROMPT = _BASE_SYSTEM_PROMPT

# Loop bounds.
_DEFAULT_MAX_TURNS = 12
_TOOL_ERROR_BUDGET = 5
_STDOUT_CAP = 4000


def build_system_prompt(steering_text, steering):
    """Build the agent system prompt.

    `steering == "on"`  -> base prompt + the steering text under STEERING_SENTINEL.
    `steering == "off"` -> the independent OFF_CONTROL_PROMPT (provably omits the
                           sentinel AND the steering text).
    """
    if steering == "on":
        return f"{_BASE_SYSTEM_PROMPT}\n\n{STEERING_SENTINEL}\n{steering_text}"
    return OFF_CONTROL_PROMPT


@dataclass
class Step:
    """One observed `helix <verb>` invocation."""

    verb: str
    argv: list
    exit: int
    stdout: str


@dataclass
class Transcript:
    """The result of a bounded ReAct run. `reason` is the termination cause."""

    reason: str  # one of: done | max_turns | no_progress | tool_error_budget
    final: Optional[str]  # the final answer text, or None if terminated on a bound
    steps: list = field(default_factory=list)


class ReActAgent:
    """A bounded think -> act -> observe loop over the helix CLI."""

    def __init__(self, system_prompt, llm):
        self.system_prompt = system_prompt
        self.llm = llm

    def run(self, task, cwd=".", max_turns=_DEFAULT_MAX_TURNS):
        messages = [
            {"role": "system", "content": self.system_prompt},
            {"role": "user", "content": task},
        ]
        steps = []
        last_sig = None
        tool_errors = 0

        for _turn in range(max_turns):
            msg = self.llm.chat(messages, tools=TOOL_SCHEMAS)

            # No tool call -> final answer. reason="done".
            if not getattr(msg, "tool_calls", None):
                return Transcript(reason="done", final=msg.content, steps=steps)

            # OpenAI ordering: append the assistant tool-call message via
            # model_dump(exclude_none=True) BEFORE the tool-result messages.
            messages.append(msg.model_dump(exclude_none=True))

            turn_sig = []
            for call in msg.tool_calls:
                result = run_verb(call, cwd)
                steps.append(
                    Step(
                        verb=call.function.name,
                        argv=result.argv,
                        exit=result.exit,
                        stdout=result.stdout,
                    )
                )
                if result.exit != 0:
                    tool_errors += 1
                messages.append(
                    {
                        "role": "tool",
                        "tool_call_id": call.id,
                        "content": result.stdout[:_STDOUT_CAP],
                    }
                )
                turn_sig.append((tuple(result.argv), result.stdout))

            # No-progress bound: identical tool activity two turns running.
            sig = tuple(turn_sig)
            if sig == last_sig:
                return Transcript(reason="no_progress", final=None, steps=steps)
            last_sig = sig

            # Tool-error budget bound.
            if tool_errors >= _TOOL_ERROR_BUDGET:
                return Transcript(reason="tool_error_budget", final=None, steps=steps)

        return Transcript(reason="max_turns", final=None, steps=steps)


def run(task, steering_text="", steering="off", llm=None, cwd=".", max_turns=_DEFAULT_MAX_TURNS):
    """Convenience entry point: build the system prompt, construct an agent, run.

    `llm` must be provided (a constructed `agent.llm.LLM` or a duck-typed fake);
    the agent never builds its own client so callers control provider/key handling.
    """
    if llm is None:
        raise RuntimeError("run() requires an llm instance (no implicit client construction)")
    system_prompt = build_system_prompt(steering_text, steering)
    agent = ReActAgent(system_prompt=system_prompt, llm=llm)
    return agent.run(task, cwd=cwd, max_turns=max_turns)


def to_trace(transcript, task, steering, model):
    """Serialize a Transcript to the AGENT-01 JSON trace shape (no secrets)."""
    return json.dumps(
        {
            "task": task,
            "steering": steering,
            "model": model,
            "reason": transcript.reason,
            "final": transcript.final,
            "steps": [
                {"verb": s.verb, "argv": s.argv, "exit": s.exit, "stdout": s.stdout}
                for s in transcript.steps
            ],
        },
        indent=2,
    )
