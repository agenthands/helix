---
gsd_state_version: 1.0
milestone: v2.14
milestone_name: Codebase-Map Re-mapping (Go tree)
status: shipped
last_updated: "2026-07-01T22:30:00.000Z"
last_activity: 2026-07-01 — v2.14 SHIPPED. All 7 .planning/codebase/*.md re-mapped from the removed Python serena tree to the current Go tree at HEAD; every sampled claim source-grounded, residue clean (only labeled retained-lineage SerenaMCPServer + serena/v1), staleness banners removed, all Analysis Dates → 2026-07-01. MILESTONE-AUDIT PASSED. Writers corrected 3 stale source-of-record facts (cmd=16 not 17; middleware 6-deep not 4; setup=8 clients not 7; cobra v1.10.2). Bundled planning-integrity fix: regenerated stale ROADMAP `## Phases` (v2.5-era 115-126) that made roadmap analyze manufacture phantom phases; analyze now reports 141/142/143 clean. Documentation-only, zero source change. Prior: v2.13 SHIPPED (uncommitted, co-driver's call).
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
**Current focus:** v2.14 Codebase-Map Re-mapping — re-map all 7 `.planning/codebase/*.md` from the removed pre-Go Python serena tree to the current Go tree at HEAD, source-grounded, staleness banners removed. Assess-and-document milestone (docs-only, no source change). See `.planning/milestones/v2.14-REQUIREMENTS.md`.

## Current Position

Phase: 141-143 (v2.14) — kickoff complete (REQUIREMENTS + ROADMAP authored; stale Phases section regenerated). Not yet built. Next: discuss/plan phase 141.
Status: v2.14 planned. Ground truth measured: 85,472 non-test internal LOC, 24 `internal/` packages, 16 `cmd/` binaries, 291-file `bench/`; largest subsystem `internal/semantic/` at 30.8k LOC (must be surveyed fresh — nearly absent from CLAUDE.md's arch section); 51 frozen verbs (`internal/cli/verbs_gen.go`); 590 `*_test.go`; 0 TODO/FIXME in internal+cmd; Go 1.25.1 CGO=1. Deliberately-retained lineage artifacts (NOT residue): `SerenaMCPServer` Go identifier + proto `serena/v1` package dir (Phase 52-03). `docs/` (edge-types, type-resolution, runbooks) is current — out of scope.
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
