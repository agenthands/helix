---
gsd_state_version: 1.0
milestone: v1.12
milestone_name: Bench Stack & Tool Evaluation
status: Phase 75 context gathered; ready for plan-phase
last_updated: "2026-06-14T12:00:00.000Z"
last_activity: 2026-06-14 — discuss-phase 75 complete (4/4 areas, 16 decisions captured in 75-CONTEXT.md)
progress:
  total_phases: 1
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-13)

**Core value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.
**Current focus:** v1.12 Bench Stack & Tool Evaluation — Phase 75 (Schema, Fairness Contract & Tree Skeleton) ready to plan.

## Current Position

Phase: Phase 75 — Schema, Fairness Contract & Tree Skeleton
Plan: —
Status: Context gathered (discuss-phase complete); ready to plan
Last activity: 2026-06-14 — discuss-phase 75 complete; 75-CONTEXT.md written (16 decisions across 4 areas)

### Session Continuity

Last session: 2026-06-14
Stopped at: Phase 75 context gathered (discuss-phase complete, 4/4 areas)
Resume file: .planning/phases/75-schema-fairness-contract-tree-skeleton/75-CONTEXT.md

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

- Run `/gsd-plan-phase 75` to decompose Phase 75 (Schema, Fairness Contract & Tree Skeleton) into executable plans.
