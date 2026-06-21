---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: CLI-First — MCP Surface Retirement
current_phase: 90
current_phase_name: CLI One-Shot Dial Spine + Race-Free Warm Reuse
status: executing
stopped_at: Completed 90-02-PLAN.md
last_updated: "2026-06-21T12:13:08.678Z"
last_activity: 2026-06-21
last_activity_desc: Phase 90 execution started
progress:
  total_phases: 6
  completed_phases: 0
  total_plans: 4
  completed_plans: 2
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-21)

**Core value (v2.0):** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.
**Current focus:** Phase 90 — CLI One-Shot Dial Spine + Race-Free Warm Reuse

## Current Position

Phase: 90 (CLI One-Shot Dial Spine + Race-Free Warm Reuse) — EXECUTING
Plan: 3 of 4
Status: Ready to execute
Last activity: 2026-06-21 — Phase 90 execution started

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:** v1.12 closed at 15/15 phases. Per-plan history for shipped milestones lives in the archived milestone ROADMAPs; this table resets at the v2.0 start.

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

*Updated after each plan completion.*
| Phase 90 P01 | 5min | 2 tasks | 5 files |
| Phase 90 P02 | 3min | 1 tasks | 2 files |
| Phase 90 P02 | 3min | 1 tasks | 2 files |

## Accumulated Context

### Roadmap Evolution

- 2026-06-21: v2.0 roadmap created from REQUIREMENTS.md (31 REQ-IDs across CLI/VERB/OUT/SEC/SKILL/RETIRE/DOCS/TEST) and the 5 research files. Strangler-fig 6-phase shape (90→95) honored; TEST-* threaded into 90/92/93 (E2E oracle early, contract oracle at output-freeze, behavioral oracle with the skill).
- 2026-06-21: v1.12 Bench Stack & Tool Evaluation functionally complete at Phase 89 (15/15 phases); formal `/gsd-complete-milestone` archival pending.

### Critical Roadmap Constraints (carried forward to Phase 90+)

- **Zero-proto invariant (Phase 90):** the one-shot `tools/call` rides the existing gRPC `StreamMCP` wire; `git diff api/proto/` must stay empty. Fix the cross-process daemon-spawn race (`dial.go` has no lock today) and set a 2nd-call latency SLO.
- **Security gate co-located with verbs (Phase 91):** `ProfileFilterMiddleware` only filters `tools/list` — there is no `tools/call` rejection path. The moment the always-visible generated verbs land, profile/mode enforcement at `tools/call` MUST ship in the same phase, or a read-mode/ci-bot agent could invoke destructive edit verbs. Do NOT defer SEC-01/02.
- **Output shape freezes before SKILL.md (Phase 92 → 93):** the terse renderer is the load-bearing product work and precedes SKILL.md authoring (the decision table cites real verb names + real output shape).
- **Delete MCP heads LAST (Phase 94):** stdio forwarder head + Streamable-HTTP `/mcp` removed only after dual-run parity proves CLI is the sole surface (RETIRE-03 is the gate). RETIRE-04 optional gRPC TCP bind rides with the retirement phase. Keep the SDK, gRPC IPC, 5 middlewares, and all tool handlers behind the wire.
- **Docs/identity + docgen regen against the FROZEN surface (Phase 95):** keep docgen's blank imports == the daemon's (the v1.12 docgen-drift lesson).
- **Tool count = 53** (live registry, reconciled at v1.12 quick 260617-t7x). VERB-01 acceptance is generated-count == live-registry-count enumerated by name, not a hardcoded number.

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

## Deferred Items

Items carried forward from the v1.12 milestone close (see prior STATE history / v1.12 deferred-items.md for full context):

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| v1.12 audit | Live execution of public-benchmark adapters (Docker/swebench/HF-network paths) honestly gated-skip; decision logic hermetically proven | Tracked | v1.12 close |

## Session Continuity

Last session: 2026-06-21T12:13:08.673Z
Stopped at: Completed 90-02-PLAN.md
Resume file: None

## Decisions

- [Phase ?]: Phase 90-01: per-socket gofrs/flock startup lock + double-checked tryConnect in ConnectOrStartDaemon; N parallel cold callers spawn exactly one daemon (CLI-03)
- [Phase ?]: Phase 90-01: synctest seam pattern - startupGuard takes injectable seams; race test uses an in-process mutex locker since real OS flock deadlocks synctest virtual clock
- [Phase ?]: Phase 90-02: no-arg helix (mode=auto) prints grouped cobra help and exits 0 (CLI-04); explicit --mode=stdio still runs forwarder (Phase 94 owns head deletion); 3 command groups scaffold Phase 91 verbs
