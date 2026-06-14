---
gsd_state_version: 1.0
milestone: v1.12
milestone_name: Bench Stack & Tool Evaluation
status: Phase 75 planned; ready to execute
stopped_at: Phase 75 planned (5 plans across 3 waves; plan-checker passed)
last_updated: "2026-06-14T12:39:51.326Z"
last_activity: 2026-06-14 — plan-phase 75 complete; 5 plans created, verified (1 revision pass), coverage gates green
progress:
  total_phases: 1
  completed_phases: 0
  total_plans: 5
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-13)

**Core value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.
**Current focus:** v1.12 Bench Stack & Tool Evaluation — Phase 75 (Schema, Fairness Contract & Tree Skeleton) planned; ready to execute.

## Current Position

Phase: Phase 75 — Schema, Fairness Contract & Tree Skeleton
Plan: 5 plans across 3 waves (Wave 0: 75-01 git mv; Wave 1: 75-02/03/04; Wave 2: 75-05)
Status: Planned — plan-checker passed (1 revision pass); requirements + decision coverage gates green; ready to execute
Last activity: 2026-06-14 — plan-phase 75 complete; 5 plans created, verified, committed

### Session Continuity

Last session: 2026-06-14
Stopped at: Phase 75 planned (5 plans, 3 waves; checker passed)
Resume file: .planning/phases/75-schema-fairness-contract-tree-skeleton/75-01-PLAN.md

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
