"""Dev-time ReAct agent package for the v2.3 task-success tuning harness.

DEV-TIME / OFFLINE ONLY — lives under tools/, NEVER linked into the helix
binary, `helix setup`, go.mod, or the default `go test ./...` path.

Thin barrel so the Phase-108 GEPA metric can `from agent import ReActAgent, run,
build_system_prompt` and call the agent in-process (exactly as optimize.py does
`from scorer import score_choice_rate` today).
"""

from agent.llm import LLM, DEFAULT_MODEL, DEFAULT_FALLBACK_MODEL
from agent.tools import TOOL_SCHEMAS, VerbResult, run_verb
from agent.react import (
    ReActAgent,
    Transcript,
    Step,
    build_system_prompt,
    run,
    to_trace,
    STEERING_SENTINEL,
    OFF_CONTROL_PROMPT,
)

__all__ = [
    "LLM",
    "DEFAULT_MODEL",
    "DEFAULT_FALLBACK_MODEL",
    "TOOL_SCHEMAS",
    "VerbResult",
    "run_verb",
    "ReActAgent",
    "Transcript",
    "Step",
    "build_system_prompt",
    "run",
    "to_trace",
    "STEERING_SENTINEL",
    "OFF_CONTROL_PROMPT",
]
