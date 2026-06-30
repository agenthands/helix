---
gsd_state_version: 1.0
milestone: v2.8
milestone_name: Graph Intelligence Depth (cbm-mcp parity)
status: shipped
last_updated: "2026-06-30T15:06:55.336Z"
last_activity: 2026-06-30 — v2.8 shipped (Workstream A + B0-a); MILESTONE-AUDIT PASSED; archived
progress:
  total_phases: 4
  completed_phases: 4
  total_plans: 0
  completed_plans: 0
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-26)

**Core value (v2.0):** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.
**Current focus:** v2.8 Graph Intelligence Depth — Workstream A (SEMANTICALLY_RELATED via Random Indexing) COMPLETE; Workstream B (true interprocedural DATA_FLOWS) paused at the B0 design-fork gate.

## Current Position

Phase: 121-124 (Workstream A + B0-a) — COMPLETE, verified, AUDIT PASSED
Status: SEMANTICALLY_RELATED emitted via Random Indexing (distinct/selective/deterministic/readable); structural edge renamed STRUCTURAL_TWIN, DATA_FLOWS reserved for real flow. Red-team folded; MILESTONE-AUDIT passed (see .planning/milestones/v2.8-MILESTONE-AUDIT.md). debt=0, ledger 0 errors. NOT committed; complete-milestone/archive not yet run (separate ship decision). Workstream B feature build = future milestone.
Last activity: 2026-06-30

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
