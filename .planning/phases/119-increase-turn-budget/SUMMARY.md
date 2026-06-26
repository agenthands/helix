# Phase 119: Increase Turn Budget - Summary

**Completed:** 2026-06-26
**Status:** COMPLETE

## What Was Done

### Task 1: Change Default Turn Budget ✓

Changed `max_turns` default from 8 to 20:
- `run_real.py:73`: `attribution(max_turns=20)`
- `run_real.py:123`: `mt = int(os.environ.get("AGENT_MAX_TURNS") or 20)`
- `optimize.py:255`: `_gepa_max_turns = int(os.environ.get("AGENT_MAX_TURNS") or 20)`

### Task 2: Fix JSON Error Handling ✓

Added graceful error handling for malformed LLM output:
- `agent/tools.py:175`: Catch `JSONDecodeError` and return error `VerbResult` instead of crashing

### Task 3: Re-run Attribution ✓

Executed attribution with `max_turns=20`:
- OFF arm: 51/51 tasks, 0 passed, cost $0.0589
- ON arm: 51/51 tasks, 0 passed, cost $0.1389
- Delta: +0.0000 (NO-SHIP)

### Task 4: Update REPORT.md ✓

Added v2.6 section with comparison table showing identical results between v2.5 (8 turns) and v2.6 (20 turns).

## Key Findings

### 1. Turn Budget Increase Didn't Help

| Version | max_turns | ON pass | OFF pass | Delta | Cost |
|--------|-----------|---------|----------|-------|------|
| v2.5 | 8 | 0/51 | 0/51 | +0.0000 | $0.20 |
| v2.6 | 20 | 0/51 | 0/51 | +0.0000 | $0.20 |

Both versions solve **zero tasks**. The delta is identical.

### 2. Agent Hits `no_progress` Early

Costs are nearly identical ($0.1971 vs $0.1978) despite 2.5x turn budget. The agent
terminates on `no_progress` (identical output for N consecutive turns) rather than
`max_turns`. The extra turns are never used.

### 3. Corpus Too Hard

The Aider corpus (Exercism-style problems) requires:
- Algorithm design
- Test understanding
- Iteration patterns

The agent demonstrates tool usage (ON costs 2.4x higher than OFF) but doesn't
converge on solutions within any reasonable turn budget.

## Verification Criteria

- [x] Both defaults changed (run_real.py, optimize.py)
- [x] All tests pass (30 tests)
- [x] Attribution completes with max_turns=20
- [x] ON/OFF delta recorded (delta=+0.0000)
- [x] Cost breakdown recorded
- [x] REPORT.md updated with v2.6 results
- [x] Honest verdict documented

## Exit Gate

**BUDGET-04 SATISFIED:** Honest verdict recorded. Increasing turn budget did not
produce a meaningful delta. **Recommendation: Proceed to Phase 120 (Corpus Analysis).**

## Files Changed

1. **`tools/dspy-tune/run_real.py`** — Increased default max_turns from 8 to 20
2. **`tools/dspy-tune/optimize.py`** — Increased GEPA default max_turns from 8 to 20
3. **`tools/dspy-tune/agent/tools.py`** — Added JSONDecodeError handling
4. **`tools/dspy-tune/REPORT.md`** — Added v2.6 attribution results

## Commits

```
3bc9679a feat(119): increase default max_turns from 8 to 20
fc45217b fix(agent): handle malformed JSON in tool arguments
```

## Next Steps

**Phase 120: Corpus Analysis** — Filter the corpus to easier tasks or switch
to a simpler benchmark. The current corpus doesn't demonstrate the steering effect.