---
gsd_state_version: 1.0
milestone: v2.6
milestone_name: Turn Budget Tuning
status: planning_complete
stopped_at: Plan approved
last_updated: "2026-06-26T22:00:00.000Z"
last_activity: 2026-06-26 — v2.6 plan approved, ready for execution
progress:
  total_phases: 1
  completed_phases: 0
  total_plans: 1
  completed_plans: 0
  percent: 0
current_phase: 119
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-26)

**Core value (v2.0):** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.
**Current focus:** v2.6 Turn Budget Tuning — increase turn budget to 20 and re-measure steering delta.

## Current Position

Phase: 119 (Increase Turn Budget)
Plan: Approved
Status: Ready for execution
Last activity: 2026-06-26 — v2.6 plan approved

## Performance Metrics

**Velocity:** v2.5 completed 4/4 phases. v2.6 is a single-phase milestone.

## Accumulated Context

### v2.5 Outcome

v2.5 fixed the agent harness (HARNESS-01/02/03/04) and ran attribution:
- **ON arm:** 0/51 passed, cost $0.1364
- **OFF arm:** 0/51 passed, cost $0.0607
- **Delta:** +0.0000 (NO-SHIP)
- **Tool-call rate:** Verified (ON costs 2.3x higher)

The harness is functionally correct — the agent uses tools. The issue is the turn budget (8 turns) is too aggressive for Exercism problems.

### v2.6 Hypothesis

Increasing `max_turns` from 8 to 20 will give the agent enough time to iterate, and steering may produce a measurable delta. If delta remains ~0 after 20 turns, the corpus may be uniformly too hard.

---

## Phase 119: Increase Turn Budget

### Task List

1. **BUDGET-01:** Change default max_turns from 8 to 20
2. **BUDGET-02:** Verify environment override still works
3. **BUDGET-03:** Re-run attribution with new budget
4. **BUDGET-04:** Update REPORT.md with honest verdict

### Decision Gate After Phase 119

If delta ≈ 0 and both arms ~0% pass rate, proceed to Phase 120 (Corpus Analysis). Otherwise, milestone complete.