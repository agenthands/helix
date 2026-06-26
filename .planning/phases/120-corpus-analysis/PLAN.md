# Plan: Phase 120 — Corpus Analysis & Easy Subset

## Context

v2.6 showed that increasing turn budget from 8 to 20 produced identical results (delta=+0.0000, 0/51 tasks solved). The agent terminates on `no_progress` long before hitting `max_turns`. The corpus is too hard for current agent capability.

**Analysis of heldout split (51 tasks):**
- Easy (<1000 chars): 13 tasks (25%)
- Medium (1000-2000): 19 tasks (37%)
- Hard (>=2000): 19 tasks (37%)
- Outlier: `beer-song` has 12,639 chars (3x the next hardest)

**Hypothesis:** Easier tasks may show a steering effect. If the agent can solve simpler problems, steering might produce a measurable delta.

---

## Phase 120: Easy Subset Attribution

### What

Filter the corpus to "easy" tasks (task_len < 1000) and re-run attribution to see if steering helps on simpler problems.

### Key Files

1. **`tools/dspy-tune/run_real.py`** — Add `--easy-only` flag
2. **`tools/dspy-tune/output/heldout_test.json`** — Filter to easy subset

### Requirements

| REQ-ID | Description |
|--------|-------------|
| CORPUS-01 | Identify easy tasks (task_len < 1000) |
| CORPUS-02 | Create easy subset filter |
| CORPUS-03 | Re-run attribution on easy subset |
| CORPUS-04 | Compare delta with full corpus |

### Implementation

**Phase 120-A: Create Easy Subset**

```python
# Filter heldout_test.json to easy tasks only
easy_threshold = 1000
easy_tasks = [t for t in heldout if get_task_len(t) < easy_threshold]
# Save to output/heldout_easy.json
```

**Phase 120-B: Re-run Attribution**

```bash
cd tools/dspy-tune
# Modify run_real.py to accept --easy-only flag
uv run python run_real.py attribution --easy-only
```

**Phase 120-C: Compare Results**

Compare:
- Full corpus (v2.5/v2.6): delta=+0.0000, 0/51 solved
- Easy subset: delta=?, ?/13 solved

### Success Criteria

1. Easy subset attribution completes
2. ON and OFF results recorded for easy tasks
3. Comparison documented in REPORT.md
4. **If delta > 0 on easy tasks:** Steering helps on simpler problems
5. **If delta ≈ 0 on easy tasks:** Agent capability is the issue, not corpus difficulty

### Cost Estimate

Easy subset has 13 tasks, so:
- Expected cost: ~$0.05-0.10 (13/51 of full corpus cost)

---

## Decision Gate After Phase 120

**If delta > 0 on easy subset:**
- Steering produces measurable improvement on simpler problems
- Next step: Focus on easier tasks for tuning

**If delta ≈ 0 on easy subset:**
- Agent capability is insufficient for even simple Exercism problems
- Next step: Switch to simpler benchmark (HumanEval, MBPP) or improve agent architecture

---

## Out of Scope

- New agent architecture (out of scope for v2.6)
- Model tuning (same DeepSeek model)
- Significance test (future enhancement)