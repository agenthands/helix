# Phase 117: Real GEPA Module - Context

**Gathered:** 2026-06-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Make GEPA actually evolve — rebuild as a `dspy.Module` that emits a reflectable trace. This phase addresses the root cause identified in the v2.4 investigation: "GEPA tuning is a no-op — `AgentProgram.forward` emits no predictor LM trace, so reflective mutation proposes nothing."

**In scope:**
- HARNESS-04a: Agent rebuilt as `dspy.Module` (already done — verify)
- HARNESS-04b: AgentProgram emits predictor trace
- HARNESS-04c: GEPA `forward` returns trace for reflective mutation
- HARNESS-04d: Anti-vacuity test (empty trace → FAIL)

**Out of scope:**
- Task-solving prompt (Phase 115 — already complete)
- Feedback loop (Phase 115 — already complete)
- Verb-arg hardening (Phase 116 — already complete)
- Attribution re-run (Phase 118)
</domain>

<decisions>
## Implementation Decisions

### D-01: Agent is already a dspy.Module
The current `AgentProgram` in `optimize.py` (line 257) already inherits from `dspy.Module`:
```python
class AgentProgram(dspy.Module):
    def __init__(self):
        super().__init__()
        self.solve = dspy.Predict(Solve)
```
This requirement is already satisfied.

### D-02: The trace problem
The `forward` method currently:
1. Calls `runlib.run_one()` which runs the real ReAct agent via subprocess
2. Returns `dspy.Prediction(passed=..., tests_run=..., feedback=...)`
3. **Does NOT return a predictor trace** — GEPA receives `trace=None`

GEPA's reflective mutation needs the predictor trace to propose candidate changes. Without a trace, `metric(trace=None)` receives nothing to reflect on.

### D-03: What GEPA needs
GEPA's reflective mutation works on the predictor trace:
- `self.solve(task=...)` returns a prediction with `dspy.Prediction(trace=...)`
- The trace shows what LM calls were made
- GEPA can reflect on this trace to propose instruction mutations

### D-04: The integration challenge
The ReAct agent runs a **multi-turn loop** with tool calls. This is fundamentally different from a single `dspy.Predict` call. Options:

**Option A: Keep ReAct loop, trace separately**
- Run the real agent via `runlib.run_one()`
- Build a trace structure from `Transcript.steps`
- Return the trace in the prediction
- GEPA reflects on step-level behavior

**Option B: Replace ReAct with DSPy ReAct**
- Use DSPy's built-in `dspy.ReAct` module
- This would require significant refactoring
- May not match the current agent behavior

**Recommended: Option A** — keep the working ReAct loop, add trace emission.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Core Harness Files
- `tools/dspy-tune/optimize.py` — GEPA optimization, `AgentProgram` class, `metric` function
- `tools/dspy-tune/runlib.py` — `run_one()` function, sandbox setup, grading
- `tools/dspy-tune/agent/react.py` — ReActAgent, Transcript, `to_trace()` function
- `tools/dspy-tune/agent/tools.py` — Verb specs, run_verb, TOOL_SCHEMAS

### Requirements and Roadmap
- `.planning/REQUIREMENTS.md` — HARNESS-04a/b/c/d requirements
- `.planning/ROADMAP.md` — Phase 117 success criteria and exit gate
- `.planning/.continue-here.md` — Root-cause investigation: GEPA reflection is a no-op

### DSPy Documentation
- DSPy ReAct pattern and trace emission
- `dspy.Prediction` with trace parameter
- GEPA reflective mutation semantics

</canonical_refs>

<code_context>
## Existing Code Insights

### Current AgentProgram (optimize.py:257-276)
```python
class AgentProgram(dspy.Module):
    def __init__(self):
        super().__init__()
        self.solve = dspy.Predict(Solve)

    def forward(self, task, task_dir="", gold_src="", gold_tests=None, language="python"):
        import runlib

        steering = self.solve.signature.instructions or ""
        desc = {
            "task": task, "task_dir": task_dir, "gold_src": gold_src,
            "gold_tests": gold_tests or [], "language": language,
        }
        r = runlib.run_one(desc, steering, "on", max_turns=_gepa_max_turns)
        fb = (
            f"task solved: {r['tests_run']} test(s) passed"
            if r["passed"]
            else f"task NOT solved: {r['tests_run']} test(s) ran, not all green"
        )
        return dspy.Prediction(passed=r["passed"], tests_run=r["tests_run"], feedback=fb)
```

### Key Observations
1. `self.solve` is defined but **never called** — the steering is extracted from signature.instructions
2. `runlib.run_one()` runs the real agent (subprocess)
3. No trace is returned in the prediction
4. GEPA's `metric(gold, pred, trace=None)` receives no trace

### The `to_trace()` function (react.py:232-247)
```python
def to_trace(transcript, task, steering, model):
    """Serialize a Transcript to the AGENT-01 JSON trace shape (no secrets)."""
    return json.dumps({
        "task": task,
        "steering": steering,
        "model": model,
        "reason": transcript.reason,
        "final": transcript.final,
        "steps": [
            {"verb": s.verb, "argv": s.argv, "exit": s.exit, "stdout": s.stdout}
            for s in transcript.steps
        ],
    }, indent=2)
```

This already exists! It just needs to be wired into `AgentProgram.forward`.

</code_context>

<specifics>
## Specific Ideas

- The `to_trace()` function already exists and produces a serializable trace
- `run_one()` returns a dict with `agent_reason` and `steps` — these map to trace fields
- The fix: `forward()` should build a trace from the transcript and include it in the prediction
- GEPA's `metric` should receive this trace and use it for reflection

## Key Question for Planning

**How does GEPA use the trace for reflective mutation?**
- Need to understand DSPy's GEPA implementation
- The trace should show step-level agent behavior
- GEPA needs to reflect on *why* certain steps succeeded/failed

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. Attribution re-run (Phase 118) depends on this phase completing.

</deferred>

---

*Phase: 117-Real-GEPA-Module*
*Context gathered: 2026-06-26*