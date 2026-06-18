# Roadmap: Helix

## Milestones

- [x] **v1.0 MVP** -- Phases 1-5 (shipped 2026-04-08)
- [x] **v1.1 Integration Testing** -- Phases 6-8 (shipped 2026-04-09)
- [x] **v1.2 Performance & Production Hardening** -- Phases 9-15 (shipped 2026-04-10)
- [x] **v1.3 Documentation Catchup** -- Phases 16-17 (shipped 2026-04-11)
- [x] **v1.4 Integration Testing v2** -- Phases 18-21 (shipped 2026-04-14)
- [x] **v1.5 Typed Errors & Hardening** -- Phases 22-24 (shipped 2026-04-15)
- [x] **v1.6 Context Intelligence & Resilient Editing** -- Phases 25-33 (shipped 2026-04-20)
- [x] **v1.7 Developer Experience & Auto-Setup** -- Phases 34-38 (shipped 2026-04-22)
- [x] **v1.8 Documentation Overhaul** -- Phases 39-45 (shipped 2026-04-24)
- [x] **v1.9 Polish & Infra** -- Phases 46-56 (shipped 2026-05-03)
- [x] **v1.10 Live Semantic Index** -- Phases 57-67 (shipped 2026-05-12)
- [x] **v1.11 Semantic Index Completion & P1 MCP Tools** -- Phases 68-74 (shipped 2026-06-07) — see `.planning/milestones/v1.11-ROADMAP.md`
- [ ] **v1.12 Bench Stack & Tool Evaluation** -- Phases 75-89 (active, started 2026-06-13) — see `.planning/milestones/v1.12-ROADMAP.md`

## Phases

### 🚧 v1.12 Bench Stack & Tool Evaluation (Phases 75-89) — ACTIVE

15 phases, 63 v1 requirements, 100% mapped. Headline claim: *"Same model + same budget — with Helix the agent solves more tasks, with fewer tokens, fewer files read, and fewer destructive edits."*

- [x] Phase 75: Schema, Fairness Contract & Tree Skeleton (completed 2026-06-15)
- [x] Phase 76: Ablation Profiles + Kernel Subsystem Disable Flags (completed 2026-06-16)
- [x] Phase 77: Bench Runtime & First E2E Smoke
- [x] Phase 78: Internal ToolBench — Go First + LanguageRunner Interface (completed 2026-06-17)
- [x] Phase 79: Evaluators & Result-Schema Metrics Layer (completed 2026-06-18)
- [ ] Phase 80: Five-of-Six Ablation Runners + Fairness Enforcement
- [ ] Phase 81: `no_semantic` Kernel Flag + E2E Config-Gate Test
- [ ] Phase 82: Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard
- [ ] Phase 83: `cmd/helix-bench-rag` + baseline_rag Mode + Embedding-Index Builder
- [ ] Phase 84: Container Runtime + Cosign-Signed GHCR Mirror + Disk-Budget Guard
- [ ] Phase 85: Aider Polyglot Adapter + 7 Remaining Per-Language Runners
- [ ] Phase 86: CrossCodeEval + RepoBench Adapters + Multi-Oracle Completion Gate
- [ ] Phase 87: SWE-bench Verified Adapter + UTBoost Rescorer + Multi-Oracle `verified_correctness`
- [ ] Phase 88: Multi-SWE-bench + Terminal-Bench 2.0 Adapters
- [ ] Phase 89: Reports, CI Policy & Contamination Canary

**Full details:** `.planning/milestones/v1.12-ROADMAP.md`

### Phase 75: Schema, Fairness Contract & Tree Skeleton

**Goal**: Every downstream phase has a versioned `result.v2.json` schema to write into and a single `fairness_contract.go` struct to load model config from — so no benchmark adapter ever defines its own model snapshot, temperature, or cost row.

**Depends on**: v1.10 Phase 67 (`internal/eval/` patterns — sandbox, trace tap, score DSL)

**Requirements**: BENCH-01, BENCH-02, BENCH-03, BENCH-06, FAIR-01, FAIR-02, FAIR-03, COST-01, INFRA-01, INFRA-02, INFRA-03

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 75)

### Phase 76: Ablation Profiles + Kernel Subsystem Disable Flags

**Goal**: Land the only invasive code paths inside the daemon (kernel-level `disable_lsp_subsystem` / `disable_structured_edit_subsystem` flags) plus the 4 new bench profile YAMLs, with a static `vet-ablation-leakage` analyzer so they are stable before any downstream phase consumes them. `no_semantic` flag is intentionally deferred to Phase 81 because it depends on the v1.10 Phase 65 SemanticLookup wiring being un-wired cleanly.

**Depends on**: Phase 75, v1.10 Phase 65 (SemanticLookup seam — read for context only)

**Requirements**: ABLATE-02, ABLATE-05, ABLATE-07, ABLATE-08
**Plans:** 4/4 plans complete
**Wave 1**

- [x] 76-01-PLAN.md — Wave 1: kernel disable flags + accessors + structured-edit Unsupported guard + replace_in_file exact-match-only (TDD; ABLATE-07)
- [x] 76-02-PLAN.md — Wave 1: 4 bench profile YAMLs + Profile disable-flag fields + golden tool-surface tests + loader unknown-mode rejection (TDD; ABLATE-02)
- [x] 76-03-PLAN.md — Wave 1: vet-ablation-leakage analyzer + cmd + testdata green→red + make vet wiring (TDD; ABLATE-08)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 76-04-PLAN.md — Wave 2: no_lsp null-object daemon wiring + CLI override flags + config fields + zero-span trace-tap (TDD; ABLATE-05)

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 76)

### Phase 77: Bench Runtime & First E2E Smoke

**Goal**: Stand up the bench orchestrator end-to-end on a single Go ToolBench task in a single mode — no container, no per-language sprawl — so the runtime shape is forced into existence and proven before evaluators or ablations land on top.

**Depends on**: Phase 75 (schema, tree), Phase 76 (bench profile YAMLs exist so daemon subprocess can be started with `--profile=bench-full`)

**Requirements**: BENCH-04, BENCH-05

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 77)

**Plans:** 5/5 plans complete
Plans:
**Wave 1**

- [x] 77-01-PLAN.md — Foundation primitives: bench sandbox (embed eval), subprocess daemon lifecycle, mode->profile resolver + your_agent_full/MODE.md, and the one seed toolbench-go task

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 77-02-PLAN.md — result.v2.json builder (schema-valid, metric-sparse) + CCTapResult synthesis from scripted StepResults (TDD)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 77-03-PLAN.md — Cell orchestrator spine: forwarder drive -> kill -> PID-gated tap -> 2-leg Merge -> result write; BENCH-04 + criterion-#4 integration tests

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 77-04-PLAN.md — helix-bench run subcommand + flags + matrix expander (--parallel bounded) + wired-not-gating claude branch

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 77-05-PLAN.md — Makefile reconciliation (bench collision -> bench-micro) + bench/bench-quick/bench-<suite> targets + timed E2E smoke gate + BENCH.md key-names

### Phase 78: Internal ToolBench — Go First + LanguageRunner Interface

**Goal**: The deterministic ground truth for "Helix tools work" — 10 capability test classes, all 10 covered on Go (Helix's own language, tightest debug loop, no container), and a common `LanguageRunner` interface ready for the remaining 7 languages.

**Depends on**: Phase 77 (bench runtime is operational)

**Requirements**: TOOLBENCH-01, TOOLBENCH-02, TOOLBENCH-10

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 78)

**Plans:** 5 plans (4 waves)
Plans:
**Wave 1**

- [x] 78-01-PLAN.md — LanguageRunner interface + GoRunner (go test -json) + WithWorkingDir daemon option (the Phase 85 seam, D-03/D-10/D-11)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 78-02-PLAN.md — Matrix language axis + seed git mv + toolbench-go→internal-toolbench cutover + runner dispatch/store opt-in wiring (D-07/D-08/D-09/D-10)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 78-03-PLAN.md — 8 store-off Go capability fixtures (semantic view, diagnostics, rename, fuzzy, call graph, dependency graph, context min, failure handling) (D-04/D-05/D-06)
- [x] 78-04-PLAN.md — Store-ON incremental_update fixture (real overlay-drain refresh) + --parallel store-isolation integration test (D-01/D-02/D-03)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 78-05-PLAN.md — CAPABILITIES.md + PHASE67_CROSSWALK.md + corpus-coverage aggregator (Go 10/10) + full-run human-verify checkpoint (D-06/D-11, C1/C2/C4)

### Phase 79: Evaluators & Result-Schema Metrics Layer

**Goal**: Every per-task `result.v2.json` is populated with all 17 normalized metrics from real graders — including the load-bearing `tokens_input/output` from the provider's `usage` block (not Helix's MCP counter), `edit_locality_given_solved` as the headline locality metric, and a single merged OTel trace per `(task, mode, run_index)`.
**Depends on**: Phase 77 (bench runtime), Phase 78 (Go ToolBench produces real test results to grade)
**Requirements**: METRIC-01, METRIC-02, METRIC-03, METRIC-04, METRIC-05, METRIC-06
**Success Criteria** (what must be TRUE):

  1. `bench/evaluators/{test_runner,patch_validator,token_meter,tool_trace_analyzer,regression_checker}/` produce all 12 base metrics + 5 extended metrics (`semantic_tool_calls`, `edit_distance_patch`, `retry_count`, `compile_errors_before`, `compile_errors_after`) on a real task; missing metrics are explicit nulls, not omissions.
  2. A regression test asserts `tokens_input/output` source-of-truth is the provider response's `usage` block, NOT Helix's MCP-side counter; cached-input tokens (`tokens_input_cached_read`, `tokens_input_cache_write`) are reported as separate columns.
  3. `edit_locality` definition (`1 − (modified_files / total_files_in_repo_subtree)`) and `regression_rate` definition (`(failing_pre-existing_tests_post_patch / passing_pre-existing_tests_pre_patch)`) are unit-tested at edge cases (root-only = 1.0, all-files ≈ 0.0, synthetic regression case); both definitions documented in `bench/evaluators/METRICS.md`.
  4. Trace merging produces a single merged trace per `(task, mode, run_index)` from Helix daemon OTel + agent CLI subprocess + bench harness span; Jaeger import shows full continuity from `bench.run_id` root to LSP leaves; no orphan spans, no cross-cell PID leakage.

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 79)

**Plans:** 4/4 plans complete
Plans:
**Wave 1**

- [x] 79-01-PLAN.md — Foundation: typed nullable `Metrics`/`MetricError` + schema typing of all 17 metrics + `metric_errors[]` (additive minor bump) [Wave 1]

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 79-02-PLAN.md — Exec/git graders: test_runner (exit-authoritative success), patch_validator (edit_locality/edit_distance), regression_checker (pre/post regression_rate) [Wave 2]
- [x] 79-03-PLAN.md — Trace graders: token_meter (provider-usage source-of-truth, scripted-null) + tool_trace_analyzer (trace-derived metrics, no re-merge) [Wave 2]

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 79-04-PLAN.md — Coordinator (D-07 isolation) + result.v2 wiring + run_index path + pre-patch snapshot + METRICS.md + E2E gate [Wave 3]

<details>
<summary>✅ v1.11 Semantic Index Completion & P1 MCP Tools (Phases 68-74) -- SHIPPED 2026-06-07</summary>

- [x] Phase 68: Precise FileFactDiff Populator (5/5 plans) — completed 2026-05-13
- [x] Phase 69: Production Status Accessors (6/6 plans) — completed 2026-05-14
- [x] Phase 70: Incremental Refresh Overlay-Drain (7/7 plans) — completed 2026-05-15
- [x] Phase 71: P1 Single-Symbol Read Tools (5/5 plans) — completed 2026-05-17
- [x] Phase 72: P1 Cluster & Impact Tools (5/5 plans) — completed 2026-05-19
- [x] Phase 73: P1 Tools Integration & E2E Verification (4/4 plans) — completed 2026-05-21
- [x] Phase 74: Close gap — wire P1 tool accessors in production daemon (6/6 plans) — completed 2026-06-03

**Full details:** `.planning/milestones/v1.11-ROADMAP.md`

</details>

## Backlog

_No items in backlog._
