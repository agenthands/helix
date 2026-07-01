---
gsd_state_version: 1.0
milestone: v2.13
milestone_name: True intraprocedural DATA_FLOWS (in-body origins, variable-level)
status: planned
last_updated: "2026-07-01T00:00:00.000Z"
last_activity: 2026-07-01 — v2.13 KICKOFF complete. REQUIREMENTS (FLOW-04/05/06) + ROADMAP (Phases 138-140) authored; red-team agent://RedTeamV213 (PROCEED-WITH-FIXES) folded (B1 union gap: 5/11 grammars uncovered, Ruby zero DATA_FLOWS since v2.9; M1 non-regression reframe + append-order pin; M2 function-seeded multi-hop; M3 false invariants; M4 bridge determinism; M5 all-11 unit test → Phase 138). Not yet built. Prior: v2.12 SHIPPED (f7e5f4b3), AUDIT PASSED.
progress:
  total_phases: 3
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-26)

**Core value (v2.0):** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.
**Current focus:** v2.13 True intraprocedural DATA_FLOWS — model in-body call-return origins (`y := producer(); sink(y)`) by widening the flow-summary origin space (`{param idx}` → `{param idx ∪ callReturn(callee)}`), emitting `producer.function → consumer.param` edges (`def_use_inbody`) + a reclaimed `param → own-function` return-bridge (`def_use_return`) for multi-hop reachability. No schema change; edges anchor on existing symbol nodes. All 11 languages E2E (co-driver). See `.planning/milestones/v2.13-ROADMAP.md` + `agent://RedTeamV213`.

## Current Position

Phase: 138-140 (v2.13) — kickoff complete (REQUIREMENTS + ROADMAP + red-team fold); not yet built. Next: `/anvil-discuss-phase 138` or `/anvil-plan-phase 138`.
Status: v2.13 planned. The single load-bearing risk is folded: the flow-engine tree-sitter kind unions (`internal/semantic/dataflow/summary.go:54-83`) cover only 6/11 languages today — Python/Ruby/Kotlin/C/C++ need union extensions + a Kotlin `property_declaration` split (B1), verified by a cheap all-11 unit test in Phase 138 (M5). The design reclaims the dead `ParamFlow.Returns` flag as the multi-hop return-bridge (D-BRIDGE, red-team-confirmed correct). Invariants: zero new deps, no schema migration, deterministic, v2.9 def_use edge SET unchanged for the 6 already-working langs.
Last activity: 2026-07-01

## Performance Metrics

**Velocity:** v2.6 completed 1/1 phases. v2.7 completed 1/1 phases.

## Accumulated Context

### v2.8–v2.12 Outcome (graph intelligence depth + type resolution)

- **v2.8** (121-124): cbm-mcp graph-intelligence parity — SEMANTICALLY_RELATED (Random Indexing), SIMILAR_TO (MinHash), STRUCTURAL_TWIN (AST profile); reclaimed the `DATA_FLOWS` name from the structural edge.
- **v2.9** (125-126): DATA_FLOWS real producer — case-1 **param→param** interprocedural flow, 11-lang seam (`FingerprintBody`). Honest limit recorded: no in-body origins / variable-level nodes (→ v2.13).
- **v2.10** (127-129): `trace_data_flow` verb — DATA_FLOWS read surface (BFS reachability from a param seed). 51st frozen verb.
- **v2.11** (130-134): C-family type-resolution depth (full tiered resolvers, C/C++/C#/Java). Data-plumbing unblock (`QueryEffectiveSymbolFact`).
- **v2.12** (135-137, `f7e5f4b3`): production-wired the orphaned type-resolver dispatcher — a C type ref → committed `RESOLVES_TO`/`has_type` edge, real-binary E2E. Option A (ChainTokens). C proven; C++/C#/Java fast-follow.

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
