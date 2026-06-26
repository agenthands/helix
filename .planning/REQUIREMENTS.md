# Requirements — Milestone v2.6 Turn Budget Tuning

## Overview

**Goal:** Increase the agent's turn budget and re-measure the steering delta. v2.5 showed the harness is fixed (agent uses tools) but delta=0 because 8 turns is insufficient for Exercism-style problems.

**Why now:** v2.5's attribution showed 0/51 tasks solved in 8 turns. The agent IS using tools (ON costs 2.3x higher than OFF), but terminates before converging. A higher turn budget may reveal whether steering helps given more time, or whether the corpus is uniformly too hard.

---

## BUDGET — Turn Budget Increase

### BUDGET-01: Default Turn Budget to 20

- [ ] **BUDGET-01a:** Change `run_real.py:73` default from `max_turns=8` to `max_turns=20`
- [ ] **BUDGET-01b:** Change `optimize.py:255` default from `8` to `20`
- [ ] **BUDGET-01c:** Keep `_DEFAULT_MAX_TURNS = 12` in `agent/react.py` (attribution overrides it)

### BUDGET-02: Environment Override Preserved

- [ ] **BUDGET-02a:** `AGENT_MAX_TURNS` env var still works (no code change needed)
- [ ] **BUDGET-02b:** Attribution uses the new default (20) unless env var overrides

### BUDGET-03: Re-run Attribution

- [ ] **BUDGET-03a:** Run attribution with `max_turns=20`
- [ ] **BUDGET-03b:** Record ON/OFF success rates and costs
- [ ] **BUDGET-03c:** Calculate delta (ON - OFF)

### BUDGET-04: Update Report

- [ ] **BUDGET-04a:** Add v2.6 section to `REPORT.md`
- [ ] **BUDGET-04b:** Document honest verdict (delta > 0 or "corpus too hard")
- [ ] **BUDGET-04c:** Record per-arm costs

---

## Success Criteria

1. Attribution completes with `max_turns=20`
2. ON and OFF both run with higher budget
3. **Measurable delta OR documented "corpus too hard"**
4. Cost < $2.00 total (abort if exceeded)
5. All tests pass (`uv run pytest test_agent.py test_gepa.py test_scale.py`)

---

## Cost Estimate

- v2.5: $0.06 OFF + $0.14 ON = $0.20 total (8 turns, 0/51 solved)
- v2.6 estimate: $0.15-0.30 OFF + $0.30-0.60 ON = $0.50-1.00 total (20 turns)

---

## Constraint Cross-Check

| Constraint | Phase 119 |
|------------|-----------|
| Zero new Go deps | ✓ |
| Tuning in dev-venv Python | ✓ |
| All tests pass | ✓ |
| Honest verdict recorded | ✓ |

---

## Out of Scope

- Corpus filtering (Phase 120, conditional on delta ≈ 0)
- SWE-bench scale-up (TUNE-FUT-05)
- Significance test in `decide_ship` (future enhancement)
- Model tuning (same DeepSeek model)