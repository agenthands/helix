---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: CLI-First — MCP Surface Retirement
current_phase: 95
status: verifying
stopped_at: None
last_updated: "2026-06-21T23:34:57.615Z"
last_activity: 2026-06-21
last_activity_desc: Phase 95 complete
progress:
  total_phases: 6
  completed_phases: 6
  total_plans: 19
  completed_plans: 19
  percent: 100
current_phase_name: identity-docs-rewrite-docgen-regen
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-21)

**Core value (v2.0):** The `helix` CLI is the only surface an agent touches — terse, `relpath:line:col`-anchored, zero schema-preload tax — driving the unchanged warm LSP/RepoMap kernel behind it, so agents use the toolset instead of falling back to grep/sed/cat.
**Current focus:** Phase 95 — identity-docs-rewrite-docgen-regen

## Current Position

Phase: 95
Plan: Not started
Status: Phase complete — ready for verification
Last activity: 2026-06-21 — Phase 95 complete

Progress: [██░░░░░░░░] 19%

## Performance Metrics

**Velocity:** v1.12 closed at 15/15 phases. Per-plan history for shipped milestones lives in the archived milestone ROADMAPs; this table resets at the v2.0 start.

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 90 | 4 | - | - |
| 91 | 4 | - | - |
| 92 | 3 | - | - |
| 93 | 4 | - | - |
| 94 | 2 | - | - |
| 95 | 2 | - | - |

*Updated after each plan completion.*
| Phase 90 P01 | 5min | 2 tasks | 5 files |
| Phase 90 P02 | 3min | 1 tasks | 2 files |
| Phase 90 P02 | 3min | 1 tasks | 2 files |
| Phase 90 P03 | 6min | 2 tasks | 6 files |
| Phase 90 P04 | 35min | 3 tasks | 5 files |
| Phase 91 P01 | 17min | 3 tasks | 12 files |
| Phase 91 P02 | 3min | 2 tasks | 3 files |
| Phase 91 P03 | 3min | 2 tasks | 2 files |
| Phase 91 P04 | 6min | 2 tasks | 2 files |
| Phase 92 P01 | 20min | 3 tasks | 6 files |
| Phase 92 P02 | 6min | 2 tasks | 8 files |
| Phase 92 P03 | ~10min | 2 tasks | 9 files |
| Phase 93 P01 | 50m | 2 tasks | 3 files |
| Phase 93 P02 | 4min | 2 tasks | 2 files |
| Phase 93 P03 | 9min | 2 tasks | 3 files |
| Phase 93 P04 | 310 | 2 tasks | 21 files |
| Phase 94 P01 | 18min | 3 tasks | 14 files |
| Phase 94 P02 | 25min | 3 tasks | 28 files |
| Phase 95 P01 | 3min | 3 tasks | 6 files |
| Phase 95 P02 | 18 | 3 tasks | 3 files |

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

Last session: 2026-06-21T23:20:38.202Z
Stopped at: None
Resume file: None

## Decisions

- [Phase ?]: Phase 90-01: per-socket gofrs/flock startup lock + double-checked tryConnect in ConnectOrStartDaemon; N parallel cold callers spawn exactly one daemon (CLI-03)
- [Phase ?]: Phase 90-01: synctest seam pattern - startupGuard takes injectable seams; race test uses an in-process mutex locker since real OS flock deadlocks synctest virtual clock
- [Phase ?]: Phase 90-02: no-arg helix (mode=auto) prints grouped cobra help and exits 0 (CLI-04); explicit --mode=stdio still runs forwarder (Phase 94 owns head deletion); 3 command groups scaffold Phase 91 verbs
- [Phase ?]: 90-03: client transport placed in internal/forwarder (no cycle; clirpc fallback unneeded)
- [Phase ?]: 90-03: forwarder.CallTool takes version as a param to avoid an internal/cli↔forwarder import cycle
- [Phase ?]: 90-04: E2E oracle fixed 3 latent 90-03 spine bugs (search_for_pattern wrong tool name, no socket override, cold-start :8080 bind); CLI-02 SLO=max(p50*5,50ms), observed p50 ~12ms
- [Phase ?]: 91-01: AST scan (go/packages+go/ast) recovers tool-name->*Args binding; no manual per-tool table (VERB-04)
- [Phase ?]: 91-01: verbs_gen.go generated+committed; helix-cligen --check drift gate in CI + make verify-cligen (VERB-02)
- [Phase ?]: 91-01: internal/cli.VerbToolNames() is the read-only catalog seam for 91-03 integration tests
- [Phase ?]: SEC-01: tools/call authz returns typed serr.PermissionDenied as an error (not IsError) so errors.Is round-trips CLI-side
- [Phase ?]: ProfileEnforcementMiddleware installed between Guardrail and LazyInit (step 14b.6); LIFO keeps LazyInit first, ProfileEnforce before Guardrail
- [Phase ?]: SEC-02: CLI verb surface = intersection(cli.VerbToolNames(), listSessionTools(profile)); byte-identical to profile allowed set via 91-01 parity, compares cleanly vs unchanged goldens; oracle re-pointed in TestProfile_Contract_Golden
- [Phase ?]: SEC-02: hidden-AND-refused contract — TestProfile_CLI_Surface_Refusal asserts an out-of-profile destructive verb (replace_symbol_body) is absent from CLI surface AND CallTool returns a Go error (91-02 PermissionDenied)
- [Phase ?]: 91-03 deferred: TestProfile_ExcludedToolNotInvocable + TestProfile_ModeAndBudget/switch_mode fail pre-existing under 91-02 enforcement (expect IsError, now get Go error) — out of 91-03 file scope, logged to deferred-items.md
- [Phase ?]: 91-04: Phase 90 dial oracle re-pointed to flat helix <verb> (search-in-files/--pattern); call parent removed in 91-01
- [Phase ?]: 91-04: SEC-01 live proof — ci-bot daemon refuses real helix replace-symbol-body (non-zero exit + typed permission_denied); full profile allows the edit (CLI->gRPC->daemon round trip proven)
- [Phase ?]: 92-02: --json repurposed as a persistent dual-read flag (log-format on daemon path, verb-output JSON on verb path)
- [Phase ?]: 92-02: readSnippetLine clamps the CLI-side snippet read to the workspace root before opening (T-92-04)
- [Phase ?]: 92-03: Contract oracle re-targeted to real helix subprocess stdout goldens (HELIX_BIN-gated); MCP schema meta-validation replaced by default-suite typed-args to cobra-flags parity; OUT-04 chain + OUT-03 self-contained nav snippet proven end-to-end
- [Phase ?]: 93-01: SKILL.md ships via go:embed (string form); installSkill writes it atomically with skills/helix containment; zero new deps, drift-gated against VerbToolNames
- [Phase ?]: 93-02: PreToolUse nudge repurposed to per-call advisory steering grep/sed/cat to helix verbs via hookSpecificOutput.additionalContext, fail-open and exit-0 always (T-93-04)
- [Phase ?]: 93-02: classifyBashTarget uses a static code-extension allowlist (not per-call Registry) for the hot hook path; mixed code+non-code operands classify conservatively as non-code
- [Phase ?]: Phase 93-03: helix setup flipped to skill+hooks install with MCP-only hook-preserving teardown across all 7 clients; daemon MCP head intact for Phase 94
- [Phase ?]: 94-02: CLI is the sole agent-facing MCP surface — both stdio (RunForwarder) and HTTP (/mcp) heads deleted; daemon/wire/middleware engine retained behind forwarder.CallTool
- [Phase ?]: 94-02: --mode retained with only the auto arm; legacy stdio/http modes surface the unknown-mode error
- [Phase ?]: Phase 95-01: docgen derives helix verbs inline via ReplaceAll; docgen drift gate wired as make verify-docs + CI step (closes v1.12 hole); blank-import parity via --check gate + cross-ref comments, not literal equality (D-02 preserved)
- [Phase ?]: Phase 95-02: DOCS-03 satisfied by ADDING a Helix-CLI routing matrix to CLAUDE.md; external SMTC matrix left byte-for-byte intact (mcp__smtc__ count unchanged at 31)
- [Phase ?]: Phase 95-02: docs reframed CLI-first without over-claiming MCP removal — MCP Go SDK + gRPC IPC are retained internal daemon plumbing
