---
gsd_state_version: 1.0
milestone: v1.12
milestone_name: Bench Stack & Tool Evaluation
status: executing
stopped_at: Completed 77-02-PLAN.md
last_updated: "2026-06-17T00:00:00.000Z"
last_activity: 2026-06-17 -- Completed Phase 77 Plan 02 (result.v2 builder + CC-tap synth)
progress:
  total_phases: 3
  completed_phases: 2
  total_plans: 14
  completed_plans: 11
  percent: 79
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-13)

**Core value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.
**Current focus:** Phase 77 — bench-runtime-first-e2e-smoke

## Current Position

Phase: 77 (bench-runtime-first-e2e-smoke) — EXECUTING
Plan: 3 of 5
Status: Ready to execute
Last activity: 2026-06-17 -- Completed Phase 77 Plan 02 (result.v2 builder + CC-tap synth)

### Session Continuity

Last session: 2026-06-17T00:00:00.000Z
Stopped at: Completed 77-02-PLAN.md
Resume file: .planning/phases/77-bench-runtime-first-e2e-smoke/77-03-PLAN.md

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
| Phase 75 P04 | 10min | 1 tasks | 4 files |
| Phase 75 P05 | ~25min | 2 tasks | 10 files |
| Phase 76 P01 | 35min | 3 tasks | 7 files |
| Phase 76 P02 | ~4min | 2 tasks | 8 files (5 created, TDD RED+GREEN) |
| Phase 76 P03 | 12min | 2 tasks | 10 files |
| Phase 76 P04 | ~50min | 2 tasks | 7 files |
| Phase 77 P01 | 291 | 3 tasks | 11 files |
| Phase 77 P02 | ~9min | 2 tasks | 5 files (TDD RED+GREEN x2) |

## Decisions

- [Phase ?]: Phase 64 microbench relocated to internal/semantic/bench/ via per-path git mv (renames preserved, git log --follow continuity)
- [Phase ?]: Kept package bench + benchfts/ignore build tags unchanged; no go.mod edit (single module, absolute import path unaffected)
- [Phase ?]: Left make bench Makefile target alone; Phase 77 BENCH-05 name-collision deferred, not fixed in Wave 0
- [Phase 75 P02]: bench/PROVIDERS.md TOS flags set to PERMISSIVE DEFAULTS (benchmarking + publish permitted) for all providers + local models; live-TOS verification explicitly deferred to a future phase (unblocks provider-independent bench system; honesty note added in-file)
- [Phase 75 P03]: result.v2.schema.json keeps schema_version as the ONLY required field; additionalProperties left OPEN at top level so additive fields stay minor (D-03/D-04); additive-only=minor / breaking=v3 policy recorded in a schema $comment
- [Phase 75 P03]: FAIR-03 delivered as SCHEMA SUBSTRATE ONLY (cached-input columns tokens_input_cached_read/tokens_input_cache_write + fairness.overrides[]); variance detector deferred to Phase 82, cost_quality.md warning to Phase 89 — NOT graded as full FAIR-03 here
- [Phase ?]: [Phase 75 P04]: Fairness contract ModelID pinned to dated snapshot claude-sonnet-4-5-20260128 (never bare alias claude-sonnet-4-6); FAIR-02 guarded by TestModelIDIsDatedSnapshot
- [Phase ?]: [Phase 75 P04]: Validate() returns error (unit-testable) not log.Fatal; DeprecationGate takes injected today clock (D-11); 30d boundary inclusive (==30d passes, <30d fails)
- [Phase ?]: Phase 76-01: reused serr.Unsupported with greppable subsystem_disabled: prefix for ablation-disabled tools (D-06, no new kind)
- [Phase ?]: Phase 76-01: kernel subsystem-disable flags on KernelConfig (extend-in-place) + accessors; both LSP and structured-edit flags landed, LSP consumed by 76-04
- [Phase 76 P02]: bench-no-lsp drops symbol-retrieval+diagnostics SKILLS (skill-selection); bench-no-semantic + bench-no-structured-edit keep full skill set and use exclude_tools (D-09/RESEARCH-Q3)
- [Phase 76 P02]: bench-no-semantic is tool-filter-only this phase (10 semantic tools excluded), NO kernel flag; keeps get_repo_map/get_context — kernel disable_semantic_subsystem guard deferred to Phase 81 (D-11/D-12)
- [Phase 76 P02]: ProfileStore.Validate() fail-closes LoadEmbedded on unknown mode (default_mode + transition source + target); golden tests blank-import skill packages so skill.ResolveTools resolves the real per-arm surface (ABLATE-02, T-76-03/04)
- [Phase 77 P02]: result.v2 open provenance keys frozen as snake_case outcome/trace_ref/model_id (Open Q3); Phase 79 consumes without rename (additive-only=minor)
- [Phase 77 P02]: schema reached by the builder via go:embed in new bench/schema/schema.go (ResultV2SchemaBytes), not a cwd-relative read — single source of truth, no test-vs-prod cwd skew
- [Phase 77 P02]: result doc is a typed struct (not map[string]any) so rich metrics (edit_locality/regression_rate/pass@k) are absent by construction (metric-sparse holds structurally, D-04)
- [Phase 77 P02]: SynthCCTap emits NO Source:daemon KindToolCall events — Merge counts ToolCallSummary only from the real daemon leg (merge.go:97-99); CC leg adds 2nd-leg continuity without double-counting (D-02)
- [Phase 77 P02]: synth Usage is zero (scripted has no model, Pitfall 6) and event timestamps come from each StepResult.AtTime not a batch time.Now() (Pitfall 5)
- [Phase ?]: D-05: bench mode->profile resolver reads MODE.md frontmatter (table-driven; Phase 80 extends without code change)
- [Phase ?]: D-07: bench/runtime/sandbox embeds eval sandbox (no fork); subprocess StartDaemon keeps --http-addr empty (no TCP port, D-06)
