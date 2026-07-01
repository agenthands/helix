---
gsd_state_version: 1.0
milestone: v2.10
milestone_name: trace_data_flow verb (DATA_FLOWS read surface)
status: active
last_updated: "2026-06-30T19:30:00.000Z"
last_activity: 2026-06-30 — v2.10 Phases 127-129 COMPLETE + verified (51st verb, build/tests/make vet/generated gates green); NOT yet committed. v2.9 committed earlier (6618de08).
progress:
  total_phases: 3
  completed_phases: 3
  total_plans: 3
  completed_plans: 3
  percent: 100
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-26)

**Core value (v2.0):** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.
**Current focus:** v2.9 Interprocedural DATA_FLOWS — build the `DATA_FLOWS` producer (case-1-only param→param interprocedural flow; a syntactic pass-through reachability substrate, NOT full taint analysis). 2 phases (125 flow-summary engine + 126 emission/read-surface/reachability). Roadmap red-teamed (agent://RedTeamV29, PROCEED-WITH-FIXES); all findings folded — notably that DEFINES is a flat container heuristic (callee params resolve by emit-order adjacency, not DEFINES).

## Current Position

Phase: 125-126 (v2.9) — COMPLETE + verified; red-team-folded
Status: v2.9 built end-to-end. DATA_FLOWS now has a real producer: case-1 param→param interprocedural flow (caller.param → callee.param via a resolved in-repo call), a syntactic pass-through reachability substrate (NOT full taint analysis). 15 pkgs green, make vet (8 vettools) clean, zero new deps, no migration, generated --check gates pass. Red-team (agent://RedTeamV29, PROCEED-WITH-FIXES) all 2 blocking + 6 major findings folded — notably DEFINES is a flat container heuristic (callee params resolve by emit-order adjacency) and reference nodes are a separate namespace (SrcNodeID is always a param symbol node). NOT committed — commit is the user's call.
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
