# Phase 120 SUMMARY — Corpus Analysis & Easy Subset

**Date:** 2026-06-26
**Status:** Complete
**Verdict:** NO-SHIP — even easy tasks are beyond current agent capability

## What Was Done

1. **CORPUS-01:** Identified easy tasks (task_len < 1000 chars) — 13 of 51 heldout tasks
2. **CORPUS-02:** Added `--easy` flag to `run_real.py` for easy-subset filtering
3. **CORPUS-03:** Ran attribution on easy subset (13 tasks, max_turns=20)
4. **CORPUS-04:** Compared delta with full corpus

## Results

| Metric | Full Corpus (v2.6) | Easy Subset (Phase 120) |
|--------|-------------------|------------------------|
| Tasks | 51 | 13 |
| ON success rate | 0.0000 (0/51) | 0.0000 (0/13) |
| OFF success rate | 0.0000 (0/51) | 0.0000 (0/13) |
| Delta (ON−OFF) | +0.0000 | +0.0000 |
| ON cost | $0.0607 | $0.0284 |
| OFF cost | $0.1364 | $0.0129 |
| Total cost | $0.1971 | $0.0413 |

## Easy Tasks (13)

All 13 tasks with task_len < 1000 chars, across Go and Python:

| Task | Language | Length |
|------|----------|--------|
| hexadecimal | go | 380 |
| paasio | go | 411 |
| word-search | go | 514 |
| poker | go | 566 |
| ledger | go | 682 |
| react | go | 727 |
| markdown | go | 733 |
| trinary | go | 740 |
| dominoes | python | 818 |
| transpose | go | 850 |
| counter | go | 912 |
| forth | go | 936 |
| matrix | go | 979 |

## Decision Gate Outcome

**Delta ≈ 0 on easy subset → Agent capability is insufficient for even simple Exercism problems.**

The agent terminates on `no_progress` early — it cannot solve even the simplest Exercism tasks (380-979 char descriptions) with 20 turns. The turn budget is not the limiting factor; the agent's problem-solving capability is.

## Recommendation

Switch to a simpler benchmark (HumanEval, MBPP) or improve agent architecture. Exercism problems — even the "easy" ones — require multi-step reasoning the current ReAct agent cannot perform.

## Files Changed

- `tools/dspy-tune/run_real.py` — Added `--easy` flag and `_heldout(easy_only=True)` filtering
- `tools/dspy-tune/output/heldout_easy.json` — Easy subset (13 tasks)
- `tools/dspy-tune/output/attribution.json` — Updated with easy-subset results
- `tools/dspy-tune/output/REPORT-RUN.md` — Updated with easy-subset verdict

## Cost

$0.0413 total (ON + OFF arms, 13 tasks × 2 arms = 26 agent runs)
