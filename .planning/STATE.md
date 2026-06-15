---
gsd_state_version: 1.0
milestone: v1.12
milestone_name: Bench Stack & Tool Evaluation
status: executing
stopped_at: Completed 75-03-PLAN.md (result.v2 schema contract + golden-validate tests)
last_updated: "2026-06-15T07:30:00.000Z"
last_activity: 2026-06-15 -- Phase 75 plan 03 complete
progress:
  total_phases: 1
  completed_phases: 0
  total_plans: 5
  completed_plans: 3
  percent: 60
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-13)

**Core value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.
**Current focus:** Phase 75 — schema-fairness-contract-tree-skeleton

## Current Position

Phase: 75 (schema-fairness-contract-tree-skeleton) — EXECUTING
Plan: 4 of 5
Status: Ready to execute
Last activity: 2026-06-15 -- Phase 75 plan 03 complete

### Session Continuity

Last session: 2026-06-15T07:30:00.000Z
Stopped at: Completed 75-03-PLAN.md (result.v2 schema contract + golden-validate tests)
Resume file: .planning/phases/75-schema-fairness-contract-tree-skeleton/75-04-PLAN.md

## Accumulated Context

### Roadmap Evolution

- 2026-06-13: v1.12 roadmap created from REQUIREMENTS.md (63 REQ-IDs) and research/SUMMARY.md (15-phase bottom-up shape).
- 2026-06-07: v1.11 closed at Phase 74 — close gap, wire P1 tool accessors in production daemon.
- Phase 74 added: Close gap: wire P1 tool accessors in production daemon (v1.11 carryover, shipped).

### Critical Roadmap Constraints (carried forward to Phase 75+)

- Schema + fairness contract must land in Phase 75 (first phase) — every later phase depends on the result-JSON schema and fairness invariants.
- Kernel subsystem-disable flags (ABLATE-05/06/07) are non-trivial — they require changes inside the daemon (not just bench/). `disable_semantic_subsystem` specifically depends on Phase 65 SemanticLookup wiring and lands in Phase 81 with a config-gate E2E test BEFORE the `no_semantic` mode is consumed by downstream public-benchmark phases.
- Container runtime (CONTAINER-*) lands BEFORE the SWE-bench / Multi-SWE-bench / Terminal-Bench adapters need it (Phase 84 → 87/88).
- Aider Polyglot (Phase 85) is the first external number target — cheapest adapter, no Docker, no upstream Python harness.
- UTBoost rescorer (VERIFIED-02) ships with the SWE-bench Verified adapter in Phase 87 — intrinsically coupled.
- Reports phase (89) is last because it aggregates everything.
- `baseline_rag` (Phase 83) is a separate phase — drags in chromem-go + embedding-index builder + the standalone `cmd/helix-bench-rag` MCP server.

## Operator Next Steps

- Run `/gsd-execute-phase 75` to execute Phase 75's 5 plans (Wave 0 `git mv` relocation runs first, then the contract/skeleton plans).

## Performance Metrics

| Phase | Plan | Duration | Notes |
|-------|------|----------|-------|
| Phase 75 P01 | 10min | 1 tasks | 8 files |
| Phase 75 P02 | 18min | 2 tasks | 9 files (1 modified in task 2) |
| Phase 75 P03 | 12min | 1 tasks | 3 files (TDD RED+GREEN) |

## Decisions

- [Phase ?]: Phase 64 microbench relocated to internal/semantic/bench/ via per-path git mv (renames preserved, git log --follow continuity)
- [Phase ?]: Kept package bench + benchfts/ignore build tags unchanged; no go.mod edit (single module, absolute import path unaffected)
- [Phase ?]: Left make bench Makefile target alone; Phase 77 BENCH-05 name-collision deferred, not fixed in Wave 0
- [Phase 75 P02]: bench/PROVIDERS.md TOS flags set to PERMISSIVE DEFAULTS (benchmarking + publish permitted) for all providers + local models; live-TOS verification explicitly deferred to a future phase (unblocks provider-independent bench system; honesty note added in-file)
- [Phase 75 P03]: result.v2.schema.json keeps schema_version as the ONLY required field; additionalProperties left OPEN at top level so additive fields stay minor (D-03/D-04); additive-only=minor / breaking=v3 policy recorded in a schema $comment
- [Phase 75 P03]: FAIR-03 delivered as SCHEMA SUBSTRATE ONLY (cached-input columns tokens_input_cached_read/tokens_input_cache_write + fairness.overrides[]); variance detector deferred to Phase 82, cost_quality.md warning to Phase 89 — NOT graded as full FAIR-03 here
