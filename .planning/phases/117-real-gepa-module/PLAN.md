# Phase 117: Real GEPA Module - Plan

**Created:** 2026-06-26
**Status:** Ready for execution
**Depends on:** Phase 115 (COMPLETE), Phase 116 (COMPLETE)

## Summary

Make GEPA actually evolve by ensuring `AgentProgram.forward` emits a predictor trace that GEPA's reflective mutation can use. The current implementation runs the agent but returns `trace=None`, making GEPA's reflection a no-op.

## Hard Bar

- **HARNESS-04:** AgentProgram MUST emit a reflectable trace. GEPA MUST be able to propose candidates. Anti-vacuity test MUST fail if trace is empty/constant.

## Goals

1. **Verify AgentProgram is dspy.Module** (already done)
2. **Wire trace emission into forward()**
3. **Update metric to use trace for reflection**
4. **Add anti-vacuity test** for empty trace

## Tasks

### Task 1: Research DSPy GEPA Trace Semantics ✓

**What:** Understand how GEPA uses the trace parameter for reflective mutation.

**Why:** Need to know what trace format GEPA expects before implementing.

**Findings (from DSPy docs):**

The `metric(gold, pred, trace, pred_name, pred_trace)` signature:
- `trace` - The full execution trace of the module
- `pred_trace` - A list of `(predictor, inputs, outputs)` tuples, specific to the predictor under mutation

GEPA's reflective mutation:
1. Analyzes execution traces
2. Identifies inputs, outputs, failures
3. Proposes new instructions based on observed failures and feedback

**Key insight:** DSPy captures traces automatically when `dspy.Predict` is called. Our current code bypasses this by running the agent as a subprocess.

**The fix:** Build a synthetic trace from the ReAct transcript that GEPA can use. The `to_trace()` function in `react.py` already produces this format.

### Task 2: Wire Trace Emission into forward()

**What:** Modify `AgentProgram.forward` to return a trace in the prediction.

**Why:** GEPA receives `trace=None` currently. Need to build trace from agent run.

**Implementation:**

Current code:
```python
def forward(self, task, task_dir="", ...):
    # ...
    r = runlib.run_one(desc, steering, "on", max_turns=_gepa_max_turns)
    return dspy.Prediction(passed=r["passed"], tests_run=r["tests_run"], feedback=fb)
```

New code (conceptual):
```python
def forward(self, task, task_dir="", ...):
    # ...
    r = runlib.run_one(desc, steering, "on", max_turns=_gepa_max_turns)
    # Build trace from the transcript
    trace = {
        "steps": r.get("steps", []),  # Need to get this from run_one
        "reason": r.get("agent_reason"),
        "task": task,
    }
    return dspy.Prediction(
        passed=r["passed"],
        tests_run=r["tests_run"],
        feedback=fb,
        trace=trace,  # NEW: emit trace for GEPA reflection
    )
```

**Key insight:** `run_one()` returns a dict but may not include full step details. Need to check `runlib.py` and modify to return transcript info.

### Task 3: Update run_one to Return Trace Info

**What:** Ensure `runlib.run_one()` returns enough information to build a trace.

**Why:** Currently `run_one()` returns `{passed, tests_run, cost_usd, agent_reason, steps}` but not the full step details.

**Implementation:**
- Check if `Transcript.steps` contains verb/argv/exit/stdout
- Modify `run_one()` to return full step details
- Pass transcript through to `forward()`

### Task 4: Update metric to Use Trace

**What:** Modify the `metric` function to receive and potentially use the trace.

**Why:** Currently `metric(gold, pred, trace=None)` ignores trace. Need to ensure GEPA can access it.

**Implementation:**
```python
def metric(gold, pred, trace=None, pred_name=None, pred_trace=None):
    passed = bool(getattr(pred, "passed", False))
    # GEPA can now access pred.trace for reflection
    return dspy.Prediction(
        score=1.0 if passed else 0.0,
        feedback=getattr(pred, "feedback", "no prediction"),
    )
```

### Task 5: Add Anti-Vacuity Test

**What:** Add test that verifies trace is non-empty and varies across runs.

**Why:** Ensure GEPA has something to reflect on.

**Implementation:**
```python
# In tools/dspy-tune/test_gepa.py (new file) or test_agent.py

def test_agent_program_emits_trace():
    """HARNESS-04: Anti-vacuity test for trace emission.

    AgentProgram.forward must emit a non-empty trace.
    If trace is empty/constant, GEPA cannot reflect and the test FAILS.
    """
    # ... test that trace is built and non-empty
    # ... test that different tasks produce different traces
```

### Task 6: Verify GEPA Can Actually Evolve

**What:** Run a minimal GEPA optimization and verify candidates differ.

**Why:** Ensure the trace enables reflective mutation.

**Implementation:**
- Run `uv run python -c "from optimize import main; main()"` with minimal config
- Check that `output/optimized.json` contains evolved steering
- Verify candidates differ from initial

## Rollout Order

1. Task 1 (Research DSPy trace semantics)
2. Task 3 (Update run_one)
3. Task 2 (Wire trace emission)
4. Task 4 (Update metric)
5. Task 5 (Anti-vacuity test)
6. Task 6 (Verify GEPA evolution)

## Verification Criteria

- [ ] `AgentProgram` is confirmed `dspy.Module` (already true)
- [ ] `forward()` returns `dspy.Prediction` with `trace=...`
- [ ] `run_one()` returns step details
- [ ] `metric()` receives trace (even if not used directly)
- [ ] Anti-vacuity test passes (trace non-empty)
- [ ] GEPA run produces evolved candidates

## Out of Scope

- Task-solving prompt (Phase 115 — already complete)
- Feedback loop (Phase 115 — already complete)
- Verb-arg hardening (Phase 116 — already complete)
- Attribution re-run (Phase 118)

## Notes

The key insight from the root-cause investigation: GEPA currently receives `trace=None` and therefore cannot do reflective mutation. The fix is to build a trace from the agent's transcript and pass it through. The `to_trace()` function already exists in `react.py` — we just need to wire it.