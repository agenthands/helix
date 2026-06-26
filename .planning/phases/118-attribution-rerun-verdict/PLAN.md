# Phase 118: Re-run Attribution + Verdict - Plan

**Created:** 2026-06-26
**Status:** Ready for execution (requires LLM API key)
**Depends on:** Phase 115 (COMPLETE), Phase 116 (COMPLETE), Phase 117 (COMPLETE)

## Summary

Run the attribution pipeline on the fixed harness and record an honest verdict. Phases 115-117 fixed the agent to use tools; now we measure whether those fixes produced a measurable improvement.

## Hard Bar

- **HARNESS-05:** Attribution must run on the fixed harness. Tool-call rate ≥ 80%. Honest verdict with cost breakdown.

## Goals

1. **Run attribution pipeline** on fixed harness
2. **Measure tool-call rate** (must be ≥ 80%)
3. **Record ON/OFF delta** with per-arm cost
4. **Update REPORT.md** with honest verdict

## Prerequisites

Before execution, verify:
- [ ] `DEEPSEEK_API_KEY` or `OPENAI_API_KEY` environment variable is set
- [ ] `AIDER_TASKS_DIR` points to the corpus (already set from v2.4)
- [ ] Python environment is set up (`uv run pytest` works)

## Tasks

### Task 1: Verify Fixed Harness

**What:** Confirm all fixes from Phases 115-117 are in place.

**How:**
```bash
cd tools/dspy-tune
uv run pytest test_agent.py test_gepa.py -v
# Should pass 19 tests
```

**Acceptance:**
- All 19 tests pass
- No regressions

### Task 2: Run Attribution Pipeline

**What:** Execute the attribution script with the fixed harness.

**How:**
```bash
cd tools/dspy-tune
export AIDER_TASKS_DIR="$PWD/corpus"
uv run python run_real.py
```

**What it does:**
1. Loads held-out test split (51 tasks)
2. Runs ON steering (candidate) on each task
3. Runs OFF steering (control) on each task
4. Records: passed, tests_run, cost_usd, agent_reason, steps
5. Computes ON - OFF delta

**Time:** 5-15 minutes depending on model

**Acceptance:**
- Attribution completes without errors
- Both ON and OFF runs complete
- Per-task results recorded

### Task 3: Measure Tool-Call Rate

**What:** Verify the agent actually uses helix verbs on ≥ 80% of tasks.

**How:**
- Inspect the attribution results
- Count tasks where `steps > 0` (agent made tool calls)
- Calculate percentage

**Acceptance:**
- Tool-call rate ≥ 80%
- If < 80%, investigate and document why

### Task 4: Record ON/OFF Delta with Cost

**What:** Compute and record the delta.

**How:**
```python
# Read attribution results
# ON_pass = number of tasks passed with steering ON
# OFF_pass = number of tasks passed with steering OFF
# delta = ON_pass - OFF_pass
# ON_cost = sum of ON costs
# OFF_cost = sum of OFF costs
```

**Acceptance:**
- Delta computed correctly
- Per-arm costs recorded
- Results written to file

### Task 5: Update REPORT.md

**What:** Document honest verdict with caveats.

**Template:**
```markdown
## v2.5 Attribution Results

**Date:** 2026-06-26
**Harness:** Fixed (Phases 115-117)

### Tool-Call Rate
- ON: X/51 tasks with tool calls (Y%)
- OFF: Z/51 tasks with tool calls (W%)

### ON vs OFF Delta
- ON passed: N/51
- OFF passed: M/51
- Delta: +D (or -D if worse)
- ON cost: $X.XX
- OFF cost: $Y.YY

### Verdict
[If delta > 0 and significant:]
**SHIP-by-rule, meaningful delta.** The harness fixes enabled GEPA to produce
a measurable improvement. Adoption path: [document]

[If delta ≈ 0 or noisy:]
**NO-SHIP, noise floor.** The delta is within measurement noise. The agent
now uses tools, but the steering signal doesn't produce improvement on this
corpus. [document caveats]

### Caveats
- [List any limitations or concerns]
```

**Acceptance:**
- REPORT.md updated with all sections
- Verdict is honest (no overstatement)
- Caveats documented

## Rollout Order

1. Task 1 (Verify fixed harness)
2. Task 2 (Run attribution)
3. Task 3 (Measure tool-call rate)
4. Task 4 (Record delta)
5. Task 5 (Update REPORT.md)

## Verification Criteria

- [ ] All 19 tests pass
- [ ] Attribution pipeline runs end-to-end
- [ ] Tool-call rate ≥ 80%
- [ ] ON/OFF delta recorded with cost breakdown
- [ ] REPORT.md updated with honest verdict
- [ ] If delta > 0 and significant: adoption path documented
- [ ] If delta ≈ 0: documented as "not yet ready"

## Out of Scope

- Adoption (TUNE-FUT-03) — human-gated, separate milestone
- SWE-bench confirmation — optional follow-on
- SKILL.md generation — requires adoption decision first

## Notes

This phase requires:
- LLM API key (DEEPSEEK_API_KEY preferred, or OPENAI_API_KEY)
- ~15 minutes runtime
- ~$0.50-5.00 cost depending on model and corpus size

If attribution fails due to missing keys, document the blocker and exit cleanly.