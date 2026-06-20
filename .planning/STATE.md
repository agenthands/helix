---
gsd_state_version: 1.0
milestone: v1.12
milestone_name: Bench Stack & Tool Evaluation
status: Wave 3 — aggregator orchestrator + leaderboard.md/cost_quality.md landed (STATS-02/03/04, COST-03); STATS-04 overlap gate + FAIR-03 CV warning live
stopped_at: Completed 82-06-PLAN.md
last_updated: "2026-06-21T01:25:00.000Z"
last_activity: 2026-06-21 -- Phase 82 Plan 06 executed (aggregator orchestrator + leaderboard/cost_quality reports, STATS-02/03/04 + COST-03)
progress:
  total_phases: 15
  completed_phases: 7
  total_plans: 42
  completed_plans: 42
  percent: 48
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-13)

**Core value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.
**Current focus:** Phase 82 — multi-run aggregator, BCa bootstrap, pass@k, cost rollup, first leaderboard

## Current Position

Phase: 82
Plan: 06 complete (Wave 3 — 82-01..06 all done; 82-07 remains)
Status: Wave 3 — aggregator orchestrator + leaderboard.md/cost_quality.md landed (STATS-02/03/04, COST-03); STATS-04 overlap gate + FAIR-03 CV warning live
Last activity: 2026-06-21 -- Phase 82 Plan 06 executed (aggregator orchestrator + leaderboard/cost_quality reports, STATS-02/03/04 + COST-03)

### Session Continuity

Last session: 2026-06-20T22:20:35.059Z
Stopped at: Completed 82-06-PLAN.md
Resume file: None

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
| 260620-k5n | README docs cleanup: add language capability-tier matrix (52 LSP / 23 grammar = ~21 first-class / 3 semantic-graph) after the docgen BEGIN/END LANGUAGES block + semantic-graph provider subsection (Go/Python/TS-JS allow-list). Correct lineage prose — Helix is independent Go-native, not a fork/port/rewrite, partially inspired by Serena/Aider/Graphify. Rewrite Acknowledgements (was lifted verbatim from Serena's, falsely crediting Serena's community for Helix's language support). Docs only; `go build ./cmd/helix` unaffected | 2026-06-20 | ffc93420 | [260620-readme-langsupport-lineage](./quick/260620-readme-langsupport-lineage/) |

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
| Phase 80 P01 | ~7min | 2 tasks | 7 files |
| Phase 80 P02 | ~6min | 1 tasks | 3 files |
| Phase 80 P03 | ~9min | 2 tasks | 3 files |
| Phase 80 P04 | ~8min | 1 tasks | 1 files |
| Phase 80 P05 | ~8min | 2 tasks | 5 files |
| Phase 81 P01 | 20m | 2 tasks | 7 files |
| Phase 81 P02 | ~10m | 2 tasks | 5 files |
| Phase 81 P03 | ~25m | 2 tasks | 5 files |
| Phase 81 P04 | ~30m | 2 tasks | 5 files (TDD RED+GREEN x2) |
| Phase 81 P05 | ~35m | 3 tasks | 6 files (TDD RED+GREEN) |
| Phase 81 P06 | ~22m | 2 tasks | 4 files (gap closure, WR-02) |
| Phase 81 P07 | ~30min | 2 tasks | 5 files |
| Phase 82 P01 | ~12min | 3 tasks | 6 files (2 created in bench/cost) |
| Phase 82 P02 | ~3min | 2 tasks | 2 files (TDD RED+GREEN, bench/aggregator) |
| Phase 82 P03 | ~4m | 2 tasks | 2 files |
| Phase 82 P05 | ~3m | 2 tasks | 2 files (TDD RED+GREEN, bench/aggregator) |
| Phase 82 P06 | ~12m | 3 tasks | 7 files |

## Decisions

- [Phase 82 P06]: STATS-02/03/04 + COST-03 DONE — aggregator.Aggregate(runDir,cfg) is a PURE orchestrator composing Load + BCaInterval/StatMean + PassAtK + perResultUSD/costPerSolvedTask via the two-level reduction (D-07: Level 1 per-(task,mode) scalar = success-rate / mean-over-non-nil / mean-USD; Level 2 BCa across-task vector). pass@1==success-rate identity; pass@N via PassAtK. ONE seeded math/rand/v2 PCG per Aggregate threaded into every BCa => byte-deterministic (D-08), locked by committed leaderboard.golden.md/cost_quality.golden.md + TestDeterministic. STATS-04 overlap gate: ciOverlap(lo_a<=hi_b && lo_b<=hi_a) on adjacent sorted rows -> "## CI overlap warnings" section suppressing X>Y (null CI never overlaps). FAIR-03 (D-15/A2): coefVariation = sample stddev/mean of per-run USD > 0.05 -> named warning in cost_quality.md. Null discipline: nil-across-all -> null CI -> em-dash, never 0. cost solved-gate = cell success-rate>0.5 (D-12); per-task USD = MEAN over runs (A3); valid_until footer = earliest across cost-table rows. Atomic temp+rename writeReport (cell.go:writeDurable analog, T-82-06-03). Added Loaded.Modes(task) accessor (Rule 3). PURE unit, NO HELIX_BIN, zero new deps.
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
- [Phase 81 P02]: distinct semantic_index.bench_disabled koanf field (semantic.Config.BenchDisabled) — NOT a reuse of SemanticIndex.Enabled (D-01); build-but-block (D-04). Profile field Profile.DisableSemanticSubsystem (yaml disable_semantic_subsystem). CLI --disable-semantic-subsystem is only-when-set -> overrides["semantic_index.bench_disabled"] (D-03). bench-no-semantic.yaml carries disable_semantic_subsystem: true and the Phase 76 D-11/D-12 deferral assertion in bench_profiles_test.go was FLIPPED false->true (deliberate). Plan 04 resolves effSemanticDisabled := cfg.SemanticIndex.BenchDisabled || activeProfile.DisableSemanticSubsystem.
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
- [Phase ?]: [Phase 80 P01]: 5 ablation MODE.md added — baseline_plain reuses baseline.yaml (D-01/ABLATE-03, no new YAML); your_agent_no_semantic emits a real row marked ablation_status: guarantee_pending_phase_81 (kernel disable_semantic_subsystem lands Phase 81); baseline_rag is a registered fail-closed stub (placeholder profile: baseline) deferred to Phase 83; all MODE.md frontmatter two-key only (KnownFields-strict, zero Go resolver change)
- [Phase ?]: [Phase 80 P02]: ablation_status added as an additive OPTIONAL open provenance field (omitempty); schema_version stays v2, no v3, no additionalProperties:false (D-03). Honest modes omit it; no_semantic arm carries guarantee_pending_phase_81, value SET by Plan 03 cell wiring
- [Phase ?]: [Phase 80 P03]: RunCell fairness gate calls DefaultContract.Validate() UNCONDITIONALLY after profile resolution, fatal on non-nil (D-04, Open Q1 scope A); committed contract has no overrides so never fatals in CI; Plan 04 owns the always-on CI contract test
- [Phase ?]: [Phase 80 P03]: baseline_rag fail-closed by mode-name comparison in RunCell BEFORE benchsandbox.New (D-02) — no daemon, no result.v2.json, nil error; CellResult.Deferred/CellOutcome.Deferred make it a distinct third matrix outcome (Success==false AND Err==nil); name detection keeps the two-key MODE.md resolver change-free
- [Phase ?]: [Phase 80 P03]: ablation_status SET via ablationStatusFor(mode) (guarantee_pending_phase_81 iff your_agent_no_semantic) into the existing BuildResult call; honest modes leave it empty so omitempty omits the key
- [Phase ?]: [Phase 80 P04]: Plan 04 ships TestEffectiveConfigMatchesContract — always-on hermetic CI gate iterating all 6 modes, asserting projected model_id==DefaultContract.ModelID + Validate()==nil (D-04 layer 2, criterion #3, T-80-02). Scope A only: other 5 contract fields documented as wired-not-enforced (no live argv producer, Pitfall 5), NOT faked; RED proven via transient unregistered mode then restored
- [Phase 80 P05]: ablation_deltas is an open top-level property (comparison -> metric -> float64); the 3 fixed deltas (full vs baseline_plain/no_lsp/no_structured_edit) surface in EACH of the 4 real-mode rows, re-Validated + atomic; null-in-either-operand metrics skipped
- [Phase 80 P05]: delta operands use RESOLVABLE mode names no_lsp/no_structured_edit (NOT prior-wave aliases your_agent_no_lsp/your_agent_no_structured_edit; Rule 1 fix); row write-back decodes as map[string]json.RawMessage so additive keys survive; scope held single-run/3-delta (NOT Phase 82 aggregator)
- [Phase ?]: 81-01: helix_semantic_store_reads_total is a labelless read counter incremented at a single s.queryContext/s.queryRowContext chokepoint in internal/semantic/store; writes/maintenance deliberately excluded (reads-only) so the no_semantic arm can assert ==0.
- [Phase 81 P03]: vet-ablation-leakage extended with a D-06 narrow-AST call-site gate check: flags direct {ExpandFrom,RankFiles,ValidateCriticalEdges} calls outside the gate allowlist (internal/semantic, internal/skill/semantic, internal/daemon, internal/kernel/symbols, internal/kernel/health) and not routed through integ.ChooseSource. Check is gated on the file importing internal/semantic/integ (name-collision guard, e.g. repomap.RankFiles); _test pkg suffix stripped for allowlist match. Plan 04 production read wiring MUST stay inside the allowlist or route through ChooseSource. SSA is the deferred precision upgrade (Plan 05 runtime counter is the dynamic complement).
- [Phase 81 P04]: effSemanticDisabled := cfg.SemanticIndex.BenchDisabled || activeProfile.DisableSemanticSubsystem resolved ONCE at daemon.go:294 (D-02, mirror Phase 76 effDisableLSP). Threaded via gatedSymbolsLookupFn/gatedCfgGate (new internal/daemon/semantic_gate.go) into ALL FOUR integLookupAccessor hand-outs: symbols/health, repomap, guardrail middleware (the 4th, not in plan A5 — deviation Rule 2), and the SemanticSkill Set*Accessor block. Gate DISABLES the cfgGate (not just the lookup) so ChooseSource yields source=tree_sitter not fallback (Pitfall 4). Under the gate repomap SetSemanticLookup(nil) + all 16 SemanticSkill accessors explicitly cleared to nil (idempotent null-object on the process-global singletons, Pitfall 5 — the skill-clear caught an order-dependent staleness bug). Bundle-build guards (daemon.go:324/518) UNTOUCHED — store stays built (D-04 build-but-block). Plan 05 asserts zero semantic-store reads on this gated arm.
- [Phase 81 P05]: Counter exposure = path A (daemon-log shutdown line). Bench daemon runs --http-addr= (HTTP off) over a Unix socket so the Prometheus /metrics scrape is unreachable; daemon emits ONE line msg="semantic store reads total" count=N in shutdown.go (d.shutdown() Phase 1.6), reusing the trace.TapDaemonLog msg== JSONL substrate. bench cell scrapeSemanticReadsTotal parses it (last occurrence wins; missing line => 0). assertNoSemanticReads hard-FAILS the no_semantic cell (routes through preserve(), CellResult.SemanticReadViolation) on any non-zero read — fail-closed, NOT a warn (D-05/criterion #2, T-81-05-01). guarantee_pending_phase_81 deferral marker REMOVED: ablationStatusFor deleted, AblationStatus forced "" for every mode (no_semantic row no longer partial). MODE.md rewritten to no_lsp shape + names the gate key semantic_index.bench_disabled (precedence CLI > profile YAML > default-off) + all 8 strangler-fig consumers (criterion #4). Added obs.Metrics.SemanticStoreReadsValue() getter (dto.Metric.Write, nil-safe). Phase 81 CLOSED (ABLATE-06). _Superseded by 81-VERIFICATION (gaps_found): criterion #2 was vacuous end-to-end (WR-02) and criterion #1 partial (CR-01); two gap-closure plans 81-06/81-07 added._
- [Phase 81 P06]: GAP 1 / WR-02 closed. New DaemonHandle.Stop(timeout) sends SIGTERM to the daemon process GROUP (Setpgid leader) and waits up to timeout for graceful exit so d.shutdown() flushes the "semantic store reads total" line (shutdown.go:60-62) — the one thing SIGKILL can never do. RunCell now drains GRACEFULLY (h.Stop, daemonGracefulStopTimeout=12s = 10s ShutdownTimeout + slack) THEN Kill as a hard reaper (called unconditionally, no-op on the graceful path, before the daemon-log tap). scrapeSemanticReadsTotal returns (count, present, err): an ABSENT line is present=false, no longer a silent count=0 (root fail-open anti-pattern cell.go:101-103 removed). assertNoSemanticReads(mode, reads, present) HARD-FAILS the no_semantic arm when present==false (proof never ran) AND when reads!=0; off-arm both are no-ops. New real-daemon HELIX_BIN-gated integration test TestNoSemanticReadsTotalLineEmitted spawns a bare daemon, drives Stop-then-Kill, asserts daemon.log carries a real reads-total line with an integer count (replaces synthetic-os.WriteFile-only coverage). D-04 build-but-block + D-05 path-A emission preserved. GAP 2 / CR-01 (background read pipelines ungated) remains for 81-07.
- [Phase ?]: [Phase 81 P07]: GAP 2 / CR-01 closed — gated SIX daemon-internal semantic read-DRIVERS on effSemanticDisabled via backgroundSemanticReadsDisabled predicate (SetFileFactStore + 5 SetActivateCallback drivers + lazy-activate ScheduleInitialExtraction, Rule 2). D-04 build-but-block preserved (store-Open + newSemanticBundle UNTOUCHED). Store-ON no_semantic regression TestNoSemanticStoreOnZeroReads proves SemanticStoreReads==0 on an OPEN store (teeth from 81-06). 81-VERIFICATION.md:113 masking broken; ABLATE-06 runtime guarantee holds. Phase 81 CLOSED.
- [Phase 82 P01]: Open Q1 RESOLVED — MOVED CostRow/CostTable + freshness gate out of cmd/helix-bench package main into importable bench/cost (exactly one type CostTable in the tree). bench/cost exports CostRow, CostTable, LoadCostTable, ValidateCostTable(today,path), PriceFor(ct,modelID,today), DateLayout, StalenessWindowDays. PriceFor is the per-lookup fail-closed gate the Plan 04 aggregator reuses: unknown model_id / past valid_until / >90d-stale last_verified are all HARD errors (D-13). validate-cost-table CLI gutted to a thin cost.ValidateCostTable shim (behavior-identical); package-main dateLayout/stalenessWindowDays kept as local copies for the sibling verify-tos validator. ExpandMatrix gained a runs int axis (D-04, STATS-01 PRODUCER half): emits N cells per (b,l,m,t) with RunIndex 0..N-1, runs<1 clamps to 1, deterministic (runs innermost), no path-machinery change. helix-bench run --runs N (default 3) wired. STATS-01 not yet complete — the aggregator REFUSAL half lands in Plan 05.
- [Phase 82 P02]: STATS-02 DONE — bench/aggregator.BCaInterval(vals, stat, B, alpha, rng) (lo,hi,ok) is a PROPER BCa: z0 = phiInv(#{theta*<theta_hat}/B) with phiInv=Sqrt2*Erfinv, plus jackknife acceleration a (Efron-Tibshirani eq 14.15); endpoints via bcaPercentiles (eq 14.10) read off a seeded math/rand/v2 PCG bootstrap distribution. NOT a percentile bootstrap — the RED test asserts BCa endpoints diverge from a plain-percentile interval over the SAME seeded distribution (fake-BCa discriminator) and that proof passed GREEN. Determinism (D-08): same seed => bit-identical [lo,hi]. Degenerate matrix (D-09): empty->ok=false null CI (never fabricated [0,0]); all-identical/m==1->point CI [v,v]; den~0->percentile fallback + clamp01; no NaN/Inf. Quantile rule LOCKED to nearest-rank idx=round(p*(B-1)) for byte-stable reproduction (A5). StatMean exported for Plan 06 callers. Pure unit, no HELIX_BIN (D-01), zero new deps (stdlib math + math/rand/v2).
- [Phase ?]: pass@k locked to HumanEval unbiased c-term product form; exported PassAtK; lgamma logBinom independent cross-check (D-10/D-11)
- [Phase 82 P05]: STATS-01 consumer half DONE — bench/aggregator.Load(runDir, expectedN) (*Loaded, error) globs <runDir>/<task>/<mode>/<run_index>/result.v2.json, groups VALID rows by (task,mode), fail-closed N-gate (D-05). valid row = exists + json.Unmarshal + runtime.Validate==nil; an invalid/garbage file is EXCLUDED (counts as deficient, not silently skipped to pass). expectedN is the caller arg (--runs/manifest), NEVER len(glob) (Pitfall 3) — proven by the 3-files-but-1-garbage => got 2 want 3 test. Any cell < expectedN => fmt.Errorf("aggregate: insufficient runs: %v", deficient) with every "<task>/<mode>: got X want N" named (sorted), result nil => caller writes NOTHING. rowMetrics mirrors evaluators.Metrics pointer types (*bool/*int/*float64), nil stays nil never fabricated 0 (Pitfall 4); full doc preserved as map[string]json.RawMessage for write-back. Glob rooted at runDir via filepath.Join, non-numeric/negative run_index skipped (T-82-05-02). Pure unit, no HELIX_BIN (D-01). Plan 06 orchestrator calls Load first and aborts before rendering on any deficiency.
