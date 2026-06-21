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
- [x] Phase 80: Five-of-Six Ablation Runners + Fairness Enforcement (completed 2026-06-19)
- [x] Phase 81: `no_semantic` Kernel Flag + E2E Config-Gate Test
- [x] Phase 82: Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard (completed 2026-06-20)
- [x] Phase 83: `cmd/helix-bench-rag` + baseline_rag Mode + Embedding-Index Builder (completed 2026-06-21)
- [x] Phase 84: Container Runtime + Cosign-Signed GHCR Mirror + Disk-Budget Guard (completed 2026-06-21)
- [x] Phase 85: Aider Polyglot Adapter + 7 Remaining Per-Language Runners (completed 2026-06-21)
- [x] Phase 86: CrossCodeEval + RepoBench Adapters + Multi-Oracle Completion Gate (completed 2026-06-21)
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

### Phase 80: Five-of-Six Ablation Runners + Fairness Enforcement

**Goal**: All 6 ablation modes are operational end-to-end on the Go ToolBench corpus, holding the same-model-same-budget invariant via the Phase 75 fairness contract. `your_agent_full` + `baseline_plain` + `no_lsp` + `no_structured_edit` produce real per-mode `result.v2.json` rows; `no_semantic` scaffolding is in place but the kernel flag (ABLATE-06) lands in Phase 81; `baseline_rag` runner stub exists but its real implementation lands in Phase 83.
**Depends on**: Phase 76 (bench profile YAMLs + no_lsp / no_structured_edit kernel flags), Phase 79 (evaluators produce real metrics), Phase 78 (Go ToolBench is the corpus)
**Requirements**: ABLATE-01, ABLATE-03
**Success Criteria** (what must be TRUE):

  1. Each of 6 modes runs end-to-end on a smoke task; mode definition lives in `bench/runners/<mode>/MODE.md`; ablation runs produce schema-valid `result.v2.json` rows tagged with the mode.
  2. `baseline_plain` reuses existing `internal/profile/profiles/baseline.yaml` (no new YAML); tool inventory for `baseline_plain` is empty except for the shell/grep/read/edit/test that the agent runtime exposes natively (documented decision in `bench/BENCH.md`).
  3. Same-model-same-budget invariant is enforced at runner-startup: per-task token budget is identical across modes; CI contract test asserts every runner's effective `(model_id, temperature, max_tokens, system_prompt_hash, retry_policy, cache_policy)` equals the fairness contract.
  4. Ablation deltas (`your_agent_full` − `baseline_plain`, `full` − `no_lsp`, `full` − `no_structured_edit`) compute correctly on the Go ToolBench corpus and surface in the per-mode result rows.

**Plans:** 5/5 plans complete
Plans:
**Wave 1**

- [x] 80-01-PLAN.md — 5 new bench/runners/<mode>/MODE.md definitions + resolver test + BENCH.md docs (ABLATE-01/03; D-01/02/03)
- [x] 80-02-PLAN.md — additive ablation_status field on the result.v2 builder + schema doc (TDD; D-03)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 80-03-PLAN.md — RunCell fairness Validate() startup gate + baseline_rag fail-close (no row) + ablation_status wiring (TDD; D-01/02/03/04)
- [x] 80-04-PLAN.md — unconditional CI effective-config contract test (TDD; D-04)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 80-05-PLAN.md — minimal 3-delta pass surfaced in per-mode rows + five-of-six multi-mode scripted smoke (TDD; D-05)

**Full details:** `.planning/milestones/v1.12-ROADMAP.md` (Phase Details > Phase 80)

### Phase 81: `no_semantic` Kernel Flag + E2E Config-Gate Test

**Goal**: Close the only ablation mode that cannot be expressed by profile YAML alone — `disable_semantic_subsystem` is a kernel-level config gate that un-wires v1.10 Phase 65's `SetSemanticLookup` strangler-fig integration at daemon bootstrap, so `get_repo_map`/`get_context` fall back to the v1.9 tree-sitter path (envelope `source == "tree_sitter"`) and zero DuckDB reads happen during a `no_semantic` run.
**Depends on**: Phase 80 (5/6 ablation runners working), v1.10 Phase 65 (SemanticLookup wiring — the thing being disabled)
**Requirements**: ABLATE-06
**Success Criteria** (what must be TRUE):

  1. With `semantic_index.bench_disabled: true` (or equivalent kernel flag), daemon bootstrap skips `SetSemanticLookup`; `get_repo_map` and `get_context` return v1.9 tree-sitter path with `source == "tree_sitter"`; `find_related_symbols`, `explain_symbol_deep`, `validate_graph_edge`, `analyze_blast_radius` SemanticLookup paths all see `NoopLookup`.
  2. An E2E `no_semantic` smoke task makes zero queries against the DuckDB semantic store; runtime assertion (`helix_semantic_*` counter family == 0) logs and fails the bench cell if violated.
  3. A vet-style boundary guard in `internal/lint/` flags any code path that conditionally bypasses the `bench_disabled` gate (extension of the `vet-ablation-leakage` analyzer from Phase 76).
  4. `bench/runners/your_agent_no_semantic/MODE.md` documents the config-key gate + the strangler-fig consumer enumeration so a future contributor cannot accidentally rip out the bypass.

**Plans**: 7 plans (5 waves) — 5 original + 2 gap-closure (81-06, 81-07) added after 81-VERIFICATION found criterion #2 vacuous (WR-02) and criterion #1 partial (CR-01)

**Wave 1**

- [x] 81-01-PLAN.md — net-new helix_semantic_store_reads_total counter + DuckDB read-chokepoint instrumentation (D-05 verification target)
- [x] 81-02-PLAN.md — config surface: distinct bench_disabled field + profile field + CLI override + bench-no-semantic.yaml gate + assertion flip (D-01/D-03)
- [x] 81-03-PLAN.md — vet-ablation-leakage call-site gate check + green→red testdata (D-06, criterion #3)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 81-04-PLAN.md — effSemanticDisabled resolution at composition root + Noop/disabled-gate forcing on all 8 consumers (+a 4th guardrail hand-out), build-but-block (D-02/D-04, criterion #1)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 81-05-PLAN.md — E2E counter==0 bench-cell assertion + fail-cell + MODE.md rewrite (D-05, criteria #2 & #4)

**Wave 4** *(gap closure — 81-VERIFICATION.md GAP 1 / WR-02: criterion #2 runtime assertion was vacuous end-to-end)*

- [x] 81-06-PLAN.md — graceful DaemonHandle.Stop (SIGTERM) so d.shutdown() flushes the reads-total line on a real bench run + fail-CLOSED scrape/assert (absent line hard-fails the no_semantic arm) + real-daemon emission integration test (ABLATE-06, D-05)

**Wave 5** *(gap closure — 81-VERIFICATION.md GAP 2 / CR-01: criterion #1 background pipelines ungated; blocked on Wave 4's fail-closed teeth)*

- [x] 81-07-PLAN.md — gate SetActivateCallback background read pipelines + SetFileFactStore on effSemanticDisabled (build-but-block, store stays built per D-04) + store-ON no_semantic end-to-end regression proof (ABLATE-06)

### Phase 82: Multi-Run Aggregator, BCa Bootstrap, pass@k, Cost Rollup, First Leaderboard

**Goal**: First externally-publishable artifact — `bench/aggregator/` consumes N ≥ 3 runs per (task, mode), computes BCa bootstrap CIs (≥ 10,000 resamples) and `pass@k` via the HumanEval closed-form, rolls up `cost_per_solved_task` against the Phase 75 cost table, and emits the first `leaderboard.md` from internal-ToolBench-Go data only.
**Depends on**: Phase 80 (real ablation data for `your_agent_full` + baselines), Phase 79 (metrics layer feeds the aggregator)
**Requirements**: STATS-01, STATS-02, STATS-03, STATS-04, COST-02, COST-03
**Success Criteria** (what must be TRUE):

  1. Default `N ≥ 3` runs per (task, mode) is enforced by the matrix runner; the aggregator refuses to write `reports/` if any cell has fewer than the configured minimum; schema validates `runs` array length ≥ N.
  2. BCa (bias-corrected accelerated) bootstrap CIs with `N_resamples ≥ 10,000` compute for every metric on every leaderboard row; unit tests pass against a closed-form known distribution; reports flag any cell where the BCa CI overlaps a neighbor (no claim of "X > Y" without non-overlapping CIs).
  3. `pass@1` and `pass@k` computed per HumanEval closed-form `1 − C(n-c, k)/C(n, k)`; unit test against published reference values passes.
  4. `cost_per_solved_task` = (sum across solved tasks of provider-`usage`-derived USD cost) / count(solved); matches a hand-computed example for a known run; `cost_quality.md` renders cost-per-solved-task per mode × benchmark with BCa CIs.

**Plans**: 7 plans (4 waves) — 1/7 complete

**Wave 1**

- [x] 82-01-PLAN.md — Foundation: move cost-table types to importable bench/cost (Open Q1) + ExpandMatrix N-cell axis + --runs flag (D-04, STATS-01 producer) ✅ (commits dccf8352, abcae78c, 88f62367)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 82-02-PLAN.md — BCa bootstrap CI (z0 + jackknife a, seeded, >=10k resamples) (TDD; STATS-02)
- [x] 82-03-PLAN.md — HumanEval unbiased pass@k (product form + lgamma cross-check; k>=2 anti-naive) (TDD; STATS-03)
- [x] 82-04-PLAN.md — cost-per-solved rollup (USD formula + freshness gate via bench/cost; golden 3.555) (TDD; COST-02)
- [x] 82-05-PLAN.md — aggregator loader + fail-closed N-gate (expectedN from flag, not disk) (TDD; STATS-01)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 82-06-PLAN.md — orchestrator (two-level reduce) + leaderboard.md/cost_quality.md render + STATS-04 overlap gate + FAIR-03 CV variance + determinism (TDD; STATS-02/03/04/COST-03)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 82-07-PLAN.md — helix-bench aggregate subcommand + registration + E2E human-verify checkpoint (D-02; STATS-01/COST-03)

### Phase 83: `cmd/helix-bench-rag` + baseline_rag Mode + Embedding-Index Builder

**Goal**: The RAG baseline is a competent grep + embedding-RAG control arm, not a strawman — implemented as a standalone `cmd/helix-bench-rag` MCP server with exactly 4 fixed tools (`rag_search`, `rag_read_chunk`, `grep`, `read_file`), backed by `chromem-go` and OpenAI `text-embedding-3-small` (with Ollama `nomic-embed-text` offline fallback). Self-contained; benefits from soaking before public benchmarks land.
**Depends on**: Phase 82 (aggregator reports `baseline_rag` rows in the leaderboard), Phase 80 (scaffolding for `baseline_rag` runner exists)
**Requirements**: ABLATE-04
**Success Criteria** (what must be TRUE):

  1. `cmd/helix-bench-rag --help` works; tool-list returns exactly 4 tools; a vet test asserts no import from `internal/kernel/` or `internal/semantic/` (`baseline_rag` is provably NOT a Helix profile and shares no code with the daemon's tool surface).
  2. Per-corpus embedding index is built once per `(corpus, embedder_model)` and cached at `$HELIX_CACHE_DIR/bench-rag-index/<corpus_sha>/`; `bench/runners/baseline_rag_agent/EMBED-CHOICE.md` documents the model pin (OpenAI `text-embedding-3-small` primary, Ollama `nomic-embed-text` fallback) and chunking strategy.
  3. A `baseline_rag` ToolBench run on Go produces schema-valid `result.v2.json` rows; the embedder ID is recorded in every row so reviewer pushback on "weak embedder" can be addressed factually.
  4. The same-model-same-budget invariant holds: `baseline_rag` agent uses the identical fairness-contract model snapshot and budget as `your_agent_full`; embedding-API calls are NOT charged against the agent's per-task budget (documented in `BENCH.md`).

**Plans**: 3 plans
Plans:
**Wave 1**

- [x] 83-01-PLAN.md — bench/ragindex leaf package (chromem-go index build/cache, deterministic corpus_sha, embedder selection) + EMBED-CHOICE.md

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 83-02-PLAN.md — standalone cmd/helix-bench-rag MCP server (exactly 4 tools, --help) + no-kernel/no-semantic import-boundary vet gate

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 83-03-PLAN.md — bench wiring: embedder_id result key, real baseline_rag drive leg (out-of-band index, same contract+budget), flip deferral tests, BENCH.md/MODE.md rewrite

### Phase 84: Container Runtime + Cosign-Signed GHCR Mirror + Disk-Budget Guard

**Goal**: The container infra needed only for public benchmarks — `os/exec` to `docker` (with podman drop-in), pinned-SHA256 per-instance images, cosign-signed mirror under `ghcr.io/agenthands/helix-bench-*` (reusing v1.10 Phase 58 cosign keyless infra), and a pre-flight disk-budget guard so a contributor's laptop doesn't melt before the first SWE-bench pull.
**Depends on**: Phase 75 (cost table, INFRA hygiene), Phase 82 (aggregator can ingest container-cell results), v1.10 Phase 58 (cosign keyless flow reused for the mirror)
**Requirements**: CONTAINER-01, CONTAINER-02, CONTAINER-03, CONTAINER-04
**Success Criteria** (what must be TRUE):

  1. `grep "github.com/docker/docker"` in `go.mod` returns empty; bench harness works with either `docker` or `podman` on PATH; arch-mismatch refusal (e.g., SWE-bench Verified amd64 image on arm64 host) refuses to run unless `BENCH_ARCH_MISMATCH_OK=1` is set.
  2. Per-instance images are pinned by SHA256 digest, not tag; image-cache state lives under `$HELIX_CACHE_DIR/bench-images/<sha>/`; a cache-hit test passes on re-run.
  3. A cosign-signed mirror of SWE-bench / Multi-SWE-bench / Terminal-Bench instance images is published to `ghcr.io/agenthands/helix-bench-*`; bench harness verifies the cosign signature before pulling; a tampered image is rejected.
  4. Disk-budget guard fails the run if available disk on the bench host is < 50 GB before a SWE-bench full run; the synthetic low-disk test trips the guard with a one-line remediation message.

**Plans**: 4 plans
Plans:
**Wave 1**

- [x] 84-01-PLAN.md — Package foundation + os/exec docker/podman engine shim + arch-mismatch gate + go.mod docker-SDK grep gate (CONTAINER-01)
- [x] 84-02-PLAN.md — SHA256 digest-pinned image cache at $HELIX_CACHE_DIR/bench-images/<sha>/ + cross-platform disk-budget guard (CONTAINER-02, CONTAINER-04)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 84-03-PLAN.md — In-process cosign/sigstore-go verify (canonical-error) + verify-then-pull crane wiring (CONTAINER-03 runtime half)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 84-04-PLAN.md — bench-mirror.yml CI publish+sign of the GHCR mirror + BENCH.md container docs (CONTAINER-03 CI/publish half)

### Phase 85: Aider Polyglot Adapter + 7 Remaining Per-Language Runners

**Goal**: Cheapest external benchmark first — Aider Polyglot's 225 Exercism tasks across 6 languages requires no Docker and no upstream Python harness, just per-language test runners. Land the adapter AND simultaneously light up the remaining 7 ToolBench languages (Python, TypeScript, JavaScript, Java, C#, C++, Rust) since the per-language toolchain image work is the same dependency.
**Depends on**: Phase 78 (Go LanguageRunner is the template), Phase 79 (evaluators consume per-language test output), Phase 82 (aggregator handles per-language slicing)
**Requirements**: ADAPTER-AIDER-01, TOOLBENCH-03, TOOLBENCH-04, TOOLBENCH-05, TOOLBENCH-06, TOOLBENCH-07, TOOLBENCH-08, TOOLBENCH-09
**Success Criteria** (what must be TRUE):

  1. Aider Polyglot full run completes via `dataset-loader-only` adapter (shallow git clone of `Aider-AI/polyglot-benchmark` at pinned sha); 2-attempt protocol with stderr re-prompt; per-language pass-rate matches published sanity benchmarks for the pinned model.
  2. All 7 remaining languages have a `bench/languages/<L>/runner.go` implementing `LanguageRunner`: Python (`pytest --json-report`, ≥ 8/10 capabilities), TypeScript (`vitest --reporter=json`, ≥ 8/10 + tsserver diagnostics), JavaScript (`jest --json`, ≥ 8/10 + eslint diagnostics), Java (`mvn test`, ≥ 8/10 + jdtls semantic view), C# (`dotnet test --logger trx`, ≥ 6/10), C++ (`cmake/ctest`, ≥ 6/10 + clangd semantic view), Rust (`cargo test --message-format=json`, ≥ 8/10 + rust-analyzer semantic view).
  3. Pre-baked per-language toolchain images (offline-resolved deps via `mvn -o`, `cargo --offline`, `pnpm install --offline --frozen-lockfile`, `pip install --no-index --find-links=…`) ship; tests run with `--network=none`; non-hermetic tasks are flagged in the dataset.
  4. Per-track Exercism license audit lands in `bench/datasets/aider-polyglot/LICENSE-AUDIT.md` (per-track sha256 + redistribution clause excerpt); `make verify-licenses` is green.

**Plans**: TBD

### Phase 86: CrossCodeEval + RepoBench Adapters + Multi-Oracle Completion Gate

**Goal**: Mid-size completion-only public benchmarks via pure `dataset-loader-only` adapters — CrossCodeEval covers Python/Java/TS/C# (the only public coverage for C# we ship), RepoBench-R/-C/-P covers Python and Java. Multi-oracle gate (EM + edit-similarity + identifier match all required to pass, with abstain mode for low-confidence completions) is the v1.12-specific contribution.
**Depends on**: Phase 85 (Python + Java + TS + C# LanguageRunners), Phase 82 (aggregator), Phase 75 (HF dataset fetcher infra via `gomlx/go-huggingface` + arrow-go fallback)
**Requirements**: ADAPTER-CCE-01, ADAPTER-REPO-01, VERIFIED-03
**Success Criteria** (what must be TRUE):

  1. CrossCodeEval smoke run scores at least one task per language (Python, Java, TS, C#); EM + edit-similarity + identifier-match scorers are unit-tested against CCE paper examples; HF dataset fetched via the cached parquet pipeline.
  2. RepoBench smoke run for each sub-task (RepoBench-R retrieval `acc@k`, RepoBench-C completion `EM`/`ES`, RepoBench-P pipeline) covers Python and Java; metrics match published reference values on a sampled subset.
  3. Multi-oracle gate documented in `bench/evaluators/VERIFIED.md`: EM + edit-similarity + identifier match all required to pass; per-oracle threshold is configurable; abstain mode emits a `verified_correctness = false` row instead of a false-positive `true`.
  4. Both adapters use the canary-emission probe (per Phase 75 INFRA pattern) and flag potentially-contaminated tasks; canary pass-rate column populated in their leaderboard rows.

**Plans**: TBD

### Phase 87: SWE-bench Verified Adapter + UTBoost Rescorer + Multi-Oracle `verified_correctness`

**Goal**: The headline external benchmark — SWE-bench Verified 500-task adapter via `subprocess-shellout` to upstream `python -m swebench.harness.run_evaluation`, with raw upstream score and UTBoost-augmented rescored score reported side-by-side. Multi-oracle `verified_correctness` (canonical tests pass AND augmented tests pass AND no pre-existing tests regress) ships in the same phase because they are intrinsically coupled — shipping the adapter without UTBoost regresses the milestone's `verified_correctness` claim.
**Depends on**: Phase 84 (container runtime + GHCR mirror), Phase 85 (Python LanguageRunner + pre-baked Python toolchain image), Phase 86 (multi-oracle pattern proven on completion benchmarks)
**Requirements**: ADAPTER-SWE-01, VERIFIED-01, VERIFIED-02
**Success Criteria** (what must be TRUE):

  1. SWE-bench Verified smoke run of 5 tasks completes via `subprocess-shellout` to upstream harness; `predictions.jsonl` produced by the agent; result JSON ingested into `result.v2.json` schema with container ID + exit code preserved.
  2. `verified_correctness` is computed independently of `task_success` — a known-buggy patch that passes only canonical tests gets `task_success=true` AND `verified_correctness=false`; the metric requires (a) canonical tests pass AND (b) UTBoost-augmented tests pass AND (c) no pre-existing tests regress.
  3. SWE-bench Verified report shows both raw upstream score and UTBoost-augmented rescored score side-by-side; UTBoost augmented suite is ingested from the published source and reproducible from a `--run-id`.
  4. Run-all-tests override is wired (not just PR-modified tests as upstream's default); `bench/evaluators/swebench/differential.go` consumes the gold patch alongside the agent patch and emits diff-overlap signal.

**Plans**: 4 plansPlans:
**Wave 1**

- [x] 87-01-PLAN.md — substrate: additive container_id/exit_code result.v2 keys + harness subprocess argv wrapper + UTBoost pin/fetch + A1-A5 human-verify

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 87-02-PLAN.md — predictions.jsonl producer + harness report parser + harness-JSON→result.v2 ingestion (hermetic, TDD)
- [x] 87-04-PLAN.md — aggregator raw-vs-rescored side-by-side column (additive, byte-stable goldens, TDD)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 87-03-PLAN.md — 3-condition verified_correctness gate (SC#2 load-bearing) + differential.go + rescore + VERIFIED.md (TDD)

### Phase 88: Multi-SWE-bench + Terminal-Bench 2.0 Adapters

**Goal**: Final external coverage — Multi-SWE-bench (1,632 instances × Java/TS/JS/Go/Rust/C/C++; Mini set acceptable at ship, full set is the reach goal) and Terminal-Bench 2.0 (89 hard containerized long-horizon tasks driven through `tb run`). Both via `subprocess-shellout`. Per-language slicing is the v1.12 contribution; long-wall scheduler accommodates Terminal-Bench tasks whose wall-time exceeds a day.
**Depends on**: Phase 87 (subprocess-shellout pattern proven on SWE-bench), Phase 85 (7 per-language toolchain images + runners)
**Requirements**: ADAPTER-MULTI-01, ADAPTER-TERM-01
**Success Criteria** (what must be TRUE):

  1. Multi-SWE-bench Mini set runs end-to-end via `python -m multi_swe_bench.harness.run_evaluation --config <config.json>`; per-language slicing (Java, TS, JS, Go, Rust, C, C++) is exposed in the reporter; full set documented as reach goal with the license resolution status (defer to v1.13 if unresolved per Phase 75 INFRA-02).
  2. Terminal-Bench 2.0 smoke run of ≥ 5 tasks completes via `tb run` CLI; container-isolation invariant holds (per-task fresh container, no cross-task filesystem leakage); `tb` JSON output ingested into `result.v2.json`.
  3. Long-wall scheduler accommodates tasks whose expected wall-time exceeds a day; per-cell checkpointing means a 24h Terminal-Bench task can resume after harness restart.
  4. Both adapters carry forward Phase 87's run-all-tests override pattern where applicable; per-language ablation slicing built into the reporter (not the upstream harness).

**Plans**: TBD

### Phase 89: Reports, CI Policy & Contamination Canary

**Goal**: The publication artifact — `leaderboard.md` + `per_language.md` + `ablations.md` + `cost_quality.md` byte-reproducible from a `--run-id`, with the CI cost-policy split that protects the milestone budget (`make bench-quick` ≤ 5 min on PR, full `make bench` nightly or maintainer-gated). Contamination canary closes the loop on Pitfall 1 — emit a known-novel pattern in select tasks; if a model emits it verbatim, the task is excluded from headline numbers with a footnote.
**Depends on**: Phase 82 (aggregator + first leaderboard), Phase 83 (baseline_rag rows), Phase 87 (SWE-bench Verified headline), Phase 88 (final external coverage)
**Requirements**: REPORT-01, REPORT-02, REPORT-03, REPORT-04, REPORT-05, INFRA-04, INFRA-05
**Success Criteria** (what must be TRUE):

  1. `helix-bench report --run-id <id>` regenerates all 4 reports byte-identically (`diff` on regenerated vs original is empty); `leaderboard.md` shows `(mode × benchmark) → pass@1, verified_correctness, cost_per_solved` with BCa CIs and non-overlap markers; `per_language.md` lists languages with no benchmark coverage as `n/a`, not omitted.
  2. `ablations.md` delta tables (`full vs no_lsp`, `full vs no_semantic`, `full vs no_structured_edit`, `full vs baseline_plain`, `full vs baseline_rag`) compute correctly with CI overlap analysis; `cost_quality.md` scatter (cost vs verified_correctness) renders as ASCII/svg and cites cost-table `valid_until`.
  3. CI workflow file exists: `make bench-quick` runs on PR (ToolBench Go-only, no LLM cost, hard 5-min cap); full `make bench` runs nightly or on-demand, gated on a maintainer label; documented cost budget.
  4. Contamination canary: a known-novel pattern emitted in select tasks; a synthetic contaminated-response test trips the flag; flagged tasks are listed in `leaderboard.md` footnote and excluded from headline numbers.

**Plans**: TBD

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
