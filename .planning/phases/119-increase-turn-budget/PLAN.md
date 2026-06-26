# Phase 119: Increase Turn Budget - Plan

**Created:** 2026-06-26
**Status:** Executing
**Depends on:** Phase 118 (COMPLETE)

## Summary

Increase the agent's turn budget from 8 to 20 and re-measure the steering delta. v2.5 showed the harness is fixed (agent uses tools) but delta=0 because 8 turns is insufficient for Exercism-style problems.

## Hard Bar

- **BUDGET-01:** Default turn budget increased from 8 to 20
- **BUDGET-04:** Honest verdict recorded in REPORT.md

## Goals

1. **Change defaults** in run_real.py and optimize.py
2. **Re-run attribution** with max_turns=20
3. **Record ON/OFF delta** with per-arm cost
4. **Update REPORT.md** with v2.6 section and honest verdict

## Prerequisites

Before execution, verify:
- [x] Phase 118 complete
- [x] All tests pass (30 tests verified)
- [x] `DEEPSEEK_API_KEY` set

## Tasks

### Task 1: Change Default Turn Budget ✓

**What:** Change `max_turns` default from 8 to 20.

**How:**
```python
# run_real.py:73
def attribution(max_turns=20):  # Was 8

# optimize.py:255
_gepa_max_turns = int(os.environ.get("AGENT_MAX_TURNS") or 20)  # Was 8
```

**Acceptance:**
- Both defaults changed
- All tests pass

**Status:** COMPLETE

### Task 2: Re-run Attribution

**What:** Execute the attribution pipeline with new budget.

**How:**
```bash
cd tools/dspy-tune
uv run python run_real.py attribution
```

**Time:** 10-30 minutes depending on model and corpus

**Acceptance:**
- Attribution completes without errors
- Both ON and OFF runs complete
- Per-task results recorded

**Status:** RUNNING

### Task 3: Record ON/OFF Delta with Cost

**What:** Compute and record the delta.

**How:**
- Compare v2.6 results with v2.5
- Calculate improvement (if any)
- Record per-arm costs

**Acceptance:**
- Delta computed correctly
- Costs recorded
- Results written to attribution.json

### Task 4: Update REPORT.md

**What:** Add v2.6 section with honest verdict.

**Template:**
```markdown
## v2.6 Attribution Results

**Date:** 2026-06-26
**Budget:** max_turns=20 (was 8 in v2.5)

### Tool-Call Rate
- ON: X/51 tasks with tool calls (Y%)
- OFF: Z/51 tasks with tool calls (W%)

### ON vs OFF Delta
- ON passed: N/51
- OFF passed: M/51
- Delta: +D (or -D if worse)
- ON cost: $X.XX
- OFF cost: $Y.YY

### Comparison with v2.5
- v2.5: delta=+0.0000, 0/51 passed
- v2.6: delta=?, ?/51 passed

### Verdict
[If delta > 0:] **SHIP-by-rule.** Steering helps with more time.
[If delta ≈ 0:] **NO-SHIP.** Corpus too hard for current agent capability.
```

**Acceptance:**
- REPORT.md updated with v2.6 section
- Verdict is honest (no overstatement)
- Comparison with v2.5 documented

## Rollout Order

1. ~~Task 1 (Change defaults)~~ ✓
2. Task 2 (Run attribution) — RUNNING
3. Task 3 (Record delta)
4. Task 4 (Update REPORT.md)

## Verification Criteria

- [x] Both defaults changed (run_real.py, optimize.py)
- [x] All tests pass
- [ ] Attribution completes with max_turns=20
- [ ] ON/OFF delta recorded
- [ ] Cost breakdown recorded
- [ ] REPORT.md updated with v2.6 results
- [ ] Honest verdict documented

## Out of Scope

- Corpus filtering (Phase 120)
- SWE-bench scale-up (TUNE-FUT-05)
- Model tuning

## Notes

- Expected cost: $0.50-1.00 total (2.5x v2.5's $0.20)
- Abort if cost exceeds $2.00