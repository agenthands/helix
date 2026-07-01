---
gsd_state_version: 1.0
milestone: v2.12
milestone_name: Production wiring for the type resolvers (C-family E2E)
status: shipped
last_updated: "2026-07-01T00:00:00.000Z"
last_activity: 2026-07-01 — v2.12 SHIPPED (autonomous). 3 phases COMPLETE + independently verified: 135 extraction foundation (langFromExt C-family + var→type co-capture linkage + latent struct-collision dedup fix), 136 resolver ChainTokens + producer/driver/emit (in-process proof: committed RESOLVES_TO p→Foo), 137 real-binary E2E (helix explain-symbol-deep → has_type edge) + docs. Orphaned type-resolver subsystem now a production consumer. Red-team (agent://RedTeamV212, REWORK→all folded). MILESTONE-AUDIT PASSED. NOT committed — commit is the user's call.
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
**Current focus:** v2.12 Production wiring for the type resolvers — wire the orphaned 12-language dispatcher into the committed-snapshot batch build (`factsFromExtracted`) so a C-family type reference becomes a queryable `RESOLVES_TO`/`has_type` edge, proven in a real-binary E2E via `helix explain-symbol-deep`. Option A (ChainTokens): producer links var→type, resolver annotation tier consumes ChainTokens (fixing the v2.11 Signature-strip contract break). See `.planning/milestones/v2.12-ROADMAP.md` + `agent://RedTeamV212`.

## Current Position

Phase: 135-137 (v2.12) — COMPLETE + verified; red-team-folded; MILESTONE-AUDIT PASSED.
Status: v2.12 built end-to-end. The orphaned 12-language type-resolver dispatcher is now wired into the production index build (makeProductionBuildFn → factsFromExtracted → resolveTypeEdges): a C var→type reference becomes a committed RESOLVES_TO edge (real target via typeIndex from nameToNode), surfaced to agents as has_type via `helix explain-symbol-deep` — proven through the REAL binary (TestCLI_E2E_CTypeResolution) + in-process (TestResolveTypeEdges_PositiveCommitsRealEdge). Option A (ChainTokens) fixed the v2.11 Signature-strip contract break. Clean cutover: bootstrap dispatcher + dead TypeResolver/SetSemanticGraph seams removed. 12 pkg green, make vet (8 vettools) clean, zero new deps, no migration, extractor goldens byte-identical. Honest limit: C is the proven language; C++/C#/Java share the wiring but lack co-capture linkage + a dedicated E2E (fast-follow). Deferred: Tier-1 LSP, type_chain/Schema-v6, cross-package. NOT committed — commit is the user's call.
Last activity: 2026-07-01

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
