---
gsd_state_version: 1.0
milestone: v2.7
milestone_name: Corpus Analysis & Benchmark Pivot
status: in_progress
stopped_at: Phase 120 complete
last_updated: "2026-06-26T23:59:00.000Z"
last_activity: 2026-06-26 — Phase 120 complete (easy subset attribution, delta=+0.0000, 0/13 solved)
progress:
  total_phases: 1
  completed_phases: 1
  total_plans: 1
  completed_plans: 1
  percent: 100
current_phase: 120
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-26)

**Core value (v2.0):** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.
**Current focus:** v2.7 Corpus Analysis & Benchmark Pivot — filter to easy tasks, confirm Exercism is too hard, pivot to simpler benchmark.

## Current Position

Phase: 120 (Corpus Analysis & Easy Subset) — COMPLETE
Status: Easy subset attribution complete; decision gate reached
Last activity: 2026-06-26 — Phase 120 complete (easy subset, delta=+0.0000, 0/13 solved)

## Performance Metrics

**Velocity:** v2.6 completed 1/1 phases. v2.7 completed 1/1 phases.

## Accumulated Context

### v2.5 Outcome

v2.5 fixed the agent harness (HARNESS-01/02/03/04) and ran attribution:
- **ON arm:** 0/51 passed, cost $0.1364
- **OFF arm:** 0/51 passed, cost $0.0607
- **Delta:** +0.0000 (NO-SHIP)
- **Tool-call rate:** Verified (ON costs 2.3x higher)

### v2.6 Outcome

Increased `max_turns` from 8 to 20:
- **ON arm:** 0/51 passed, cost $0.0607
- **OFF arm:** 0/51 passed, cost $0.1364
- **Delta:** +0.0000 (NO-SHIP)
- Agent terminates on `no_progress` early — turn budget isn't the limiting factor

### v2.7 / Phase 120 Outcome

Filtered to easy tasks (task_len < 1000, 13 tasks):
- **ON arm:** 0/13 passed, cost $0.0284
- **OFF arm:** 0/13 passed, cost $0.0129
- **Delta:** +0.0000 (NO-SHIP)
- Even the simplest Exercism problems are beyond current agent capability

### Decision Gate

Exercism is uniformly too hard for the current ReAct agent. The next step is to switch to a simpler benchmark (HumanEval, MBPP) or improve agent architecture.