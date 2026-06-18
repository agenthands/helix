---
gsd_state_version: 1.0
milestone: v1.12
milestone_name: Bench Stack & Tool Evaluation
status: verifying
stopped_at: Phase 80 context gathered
last_updated: "2026-06-18T15:50:23.860Z"
last_activity: 2026-06-18
progress:
  total_phases: 6
  completed_phases: 5
  total_plans: 23
  completed_plans: 23
  percent: 83
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-13)

**Core value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.
**Current focus:** Phase 79 — Evaluators & Result-Schema Metrics Layer

## Current Position

Phase: 79
Plan: Not started
Status: Phase complete — ready for verification
Last activity: 2026-06-18

### Session Continuity

Last session: 2026-06-18T15:50:23.856Z
Stopped at: Phase 80 context gathered
Resume file: .planning/phases/80-five-of-six-ablation-runners-fairness-enforcement/80-CONTEXT.md

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

- Phase 77 is complete (bench runtime wired end-to-end; `make bench-quick` is the hermetic ≤90s CI smoke gate; `make bench-micro` preserves the Go microbench). Next: Phase 78 (first corpus tasks) per `.planning/milestones/v1.12-ROADMAP.md`.

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260617-j29 | Re-pin fairness contract ModelID to authoritative dated Sonnet 4.5 snapshot `claude-sonnet-4-5-20250929` (was placeholder `-20260128`, which didn't match the Claude API catalog); FAIR-02 dated-snapshot invariant preserved | 2026-06-17 | f62fb475 | [260617-j29-re-pin-fairness-contract-modelid-to-auth](./quick/260617-j29-re-pin-fairness-contract-modelid-to-auth/) |
| 260617-t7x | Resync tool docs to the live 53-tool MCP registry. Root cause: `cmd/docgen` was missing the `internal/skill/semantic` blank import, so README omitted the 10 semantic tools. Added the import → regenerated README table (40→51 ToolProvider rows), bumped bench `expectedCount` 47→53 + 6 new manifest entries, refreshed descriptions golden, updated CLAUDE.md prose "41+"→"53". Clears the two long-standing `test/bench` failures (`TestBenchToolsManifestMatchesRegistry`, `TestToolDescriptionsGoldenFile`); `go test ./test/bench/...` now green | 2026-06-17 | ed0abb97 | [260617-t7x-resync-helix-tool-capability-documentati](./quick/260617-t7x-resync-helix-tool-capability-documentati/) |

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
| Phase 77 P03 | 2400 | 3 tasks | 5 files |
| Phase 77 P04 | 455 | 2 tasks | 6 files |
| Phase 77 P05 | 540 | 2 tasks | 3 files |
| Phase 78 P01 | ~14min | 2 tasks | 7 files |
| Phase 78 P02 | 30min | 2 tasks | 24 files |
| Phase 78 P03 | ~12min | 2 tasks | 53 files |
| Phase 78 P04 | ~50min | 2 tasks | 7 files |
| Phase 78 P05 | ~30min | 3 tasks | 4 files (TDD RED+GREEN + 2 docs) |
| Phase 79 P01 | ~9min | 2 tasks | 4 files |
| Phase 79 P02 | 25m | 3 tasks | 6 files |
| Phase 79 P4 | 10m | 4 tasks | 7 files |

## Decisions

- [Phase ?]: Phase 64 microbench relocated to internal/semantic/bench/ via per-path git mv (renames preserved, git log --follow continuity)
- [Phase ?]: Kept package bench + benchfts/ignore build tags unchanged; no go.mod edit (single module, absolute import path unaffected)
- [Phase ?]: Left make bench Makefile target alone; Phase 77 BENCH-05 name-collision deferred, not fixed in Wave 0
- [Phase 75 P02]: bench/PROVIDERS.md TOS flags set to PERMISSIVE DEFAULTS (benchmarking + publish permitted) for all providers + local models; live-TOS verification explicitly deferred to a future phase (unblocks provider-independent bench system; honesty note added in-file)
- [Phase 75 P03]: result.v2.schema.json keeps schema_version as the ONLY required field; additionalProperties left OPEN at top level so additive fields stay minor (D-03/D-04); additive-only=minor / breaking=v3 policy recorded in a schema $comment
- [Phase 75 P03]: FAIR-03 delivered as SCHEMA SUBSTRATE ONLY (cached-input columns tokens_input_cached_read/tokens_input_cache_write + fairness.overrides[]); variance detector deferred to Phase 82, cost_quality.md warning to Phase 89 — NOT graded as full FAIR-03 here
- [Phase ?]: [Phase 75 P04]: Fairness contract ModelID pinned to dated snapshot claude-sonnet-4-5-20260128 (never bare alias claude-sonnet-4-6); FAIR-02 guarded by TestModelIDIsDatedSnapshot — _superseded by quick task 260617-j29: -20260128 was a placeholder not in the Claude API catalog; re-pinned to the authoritative claude-sonnet-4-5-20250929 (still a dated Sonnet 4.5 snapshot, FAIR-02 intact). Sonnet 4.6 was considered but has no dated snapshot, so it cannot satisfy FAIR-02._
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
- [Phase ?]: Plan 77-03: bench cell disables semantic_index per cell to avoid parallel DuckDB lock deadlock (T-57-02-01 forbids absolute store path; D-07 forbids forking eval StartDaemon cmd.Dir)
- [Phase ?]: Plan 77-03: activate_project uses arg key repo_path (not path); driven as harness setup, not recorded as a scripted StepResult
- [Phase ?]: Phase 77 Plan 04: helix-bench run subcommand wires ExpandMatrix/RunMatrix dispatch (--parallel bounded) over RunCell; --agent=claude wired-not-gating (D-01); E2E smoke green (1/1 cell, schema-valid result.v2.json, ~1s)
- [Phase 77 P05]: `make bench` collision RECONCILED by rename — Go microbench bench: -> bench-micro: (recipe byte-preserved); reclaimed `bench` runs helix-bench run; verified via make -n recipe identity (T-77-13). bench-<suite> = `make bench SUITE=<suite>` make var (NOT a bench-%: pattern rule, which would shadow bench-micro/bench-quick/bench-baseline)
- [Phase 77 P05]: bench-quick passes ABSOLUTE --helix-bin=$(CURDIR)/helix (per-cell ephemeral scratch cwd can't resolve relative ./helix); generated /bench/reports/* gitignored with .gitkeep negated (mirrors /eval/reports/)
- [Phase 77 P05]: Timed E2E gate PASSED under AUTO MODE — single-task smoke exit 0 in 2s (<=30s); make bench-quick exit 0 (<=90s); schema-valid result.v2.json (outcome/trace_ref/model_id/fairness); merged 2-leg trace (cc+daemon, total=2, zero foreign PID). Phase 77 CLOSED (BENCH-04 + BENCH-05)
- [Phase ?]: [Phase 78 P01]: LanguageRunner.RunTests Passed=(exit==0) authoritative gate; test2json rows advisory; compile-fail → Passed=false Tests=[] (Pitfall 4)
- [Phase ?]: [Phase 78 P01]: WithWorkingDir = only new behavior line in eval StartDaemon (cmd.Dir); additive variadic opts thread through subprocess.StartDaemon via embedded sandbox, no fork (P77 D-07/D-03)
- [Phase ?]: 78-03: LSP-backed capability tools return internal store-OFF (no warm gopls); marked expect_error and graded via store-off-stable replace_in_file/fuzzy_edit + go test, still naming the tool by-construction (D-05)
- [Phase ?]: 78-03: dependency_graph kept store-OFF (A2) — get_repo_map resolves cross-package edges deterministically; D-02 escape hatch not triggered
- [Phase ?]: 78-04: incremental_update is the ONE store-ON fixture (10th capability); D-03 parallel store isolation proven (2/2 cells, distinct .helix/semantic.duckdb, RejectedForeignPid==0), RED-proven by disabling WithWorkingDir
- [Phase 78 P05]: Coverage() aggregator (bench/languages/coverage.go) computes declared (Capabilities()) ∩ covered (task.json `capability` field, D-06 source of truth — NOT the id, mitigates T-78-11); Go reports 10/10, Missing empty (criterion C2). Gap-detection test (synthetic corpus omitting one cap → reported in Missing) proves it is not a rubber-stamp (D-11/T-78-12)
- [Phase 78 P05]: CAPABILITIES.md uses REGISTERED HELIX NATIVE tool names (get_symbol_overview singular, get_call_hierarchy) — verified against internal/kernel/symbols/skill.go + README inventory at the human-verify gate; deliberately NOT the serena/SMTC plugin spellings, left unchanged
- [Phase 78 P05]: PHASE67_CROSSWALK.md is inspiration-only — T-67-* are Phase 67 PLANNING task IDs (no on-disk corpus); IT-go-* fixtures authored fresh on the Phase 77 bench spine; namespaces disjoint, zero code migration (criterion C4), enforced by ^IT-go-/not-^T-67- static test
- [Phase 78 P05]: Phase 78 CLOSED — full Go corpus run 10/10 cells green (criterion C2 gate, human-verify approved); TOOLBENCH-01/02/10 satisfied
- [Phase ?]: [Phase 79 P01]: evaluators.Metrics has 19 pointer fields (17 METRIC-01/02 + 2 FAIR-03 cached-token columns), no omitempty -> nil marshals to explicit JSON null (METRIC-01 explicit-nulls, D-06/D-07)
- [Phase ?]: [Phase 79 P01]: result.v2 metrics object + metric_errors[] added additively (minor bump, no v3); top-level tokens_input/output RELAXED to nullable (Open Q5); metrics object canonical home; old golden still valid
- [Phase ?]: edit_distance_patch = sum(added+deleted) from git diff --numstat (79-02)
- [Phase ?]: regression numerator counts cached-passing tests now failing OR absent post-patch (79-02)
- [Phase ?]: Coordinator in package coordinator to avoid grader->evaluators import cycle (79-04)
- [Phase ?]: Durable bench path is <out>/<task>/<mode>/<run_index>/ with the run_index segment guarded by validateRunIndexSegment (79-04)
