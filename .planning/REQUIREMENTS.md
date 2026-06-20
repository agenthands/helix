# Milestone v1.12 — Bench Stack & Tool Evaluation

**Status:** Active (locked 2026-06-13)
**Source of truth:** This file (REQ-IDs are normative for the roadmap)
**Upstream context:** `.planning/PROJECT.md` (Current Milestone), `.planning/research/{STACK,FEATURES,ARCHITECTURE,PITFALLS,SUMMARY}.md`

**Headline claim to land:** *"Same model + same budget — with Helix the agent solves more tasks, with fewer tokens, fewer files read, and fewer destructive edits."*

---

## v1 Requirements

Every REQ has a one-line acceptance test. The roadmap maps each REQ to exactly one phase.

### Bench Foundation (BENCH-*)

- [x] **BENCH-01**: Net-new `bench/` tree exists with the six-dir layout `bench/{datasets,runners,languages,evaluators,reports,schema}/` — the five runtime dirs plus the `schema/` **contract-only** dir (D-07; documented runtime-vs-contract in `bench/BENCH.md`). `eval/` (Phase 67) source/runtime code remains untouched. _Acceptance:_ `tree -d -L 2 bench/` matches the six-dir spec; `eval/` source/runtime is byte-identical to pre-milestone HEAD, with the single permitted change being the one-paragraph INFRA-03 reciprocal pointer in `eval/EVAL.md` (D-08/INFRA-03).
- [x] **BENCH-02**: `cmd/helix-bench` binary builds (`go build ./cmd/helix-bench`) and ships cobra subcommands `run`, `fetch-datasets`, `doctor`, `report`, `validate-cost-table`. _Acceptance:_ `helix-bench --help` lists all 5 subcommands; `helix-bench doctor` succeeds on a clean Linux host with documented prereqs.
- [x] **BENCH-03**: Normalized per-task result schema (JSON) versioned at `v2`, validated by JSON Schema; round-trips through `bench/evaluators/aggregator/`. _Acceptance:_ a golden result fixture validates; schema-version field present; backwards-incompatible changes bump major version.
- [x] **BENCH-04**: Bench runtime reuses Phase 67's `internal/eval/sandbox` and subprocess patterns; one daemon subprocess per `(task × mode)`. _Acceptance:_ in-process smoke run completes ≤ 30 s for the smallest task; no port collisions on parallel runs.
- [x] **BENCH-05**: `make bench`, `make bench-quick`, `make bench-<suite>` targets exist and invoke `cmd/helix-bench run --benchmarks=…`. _Acceptance:_ `make bench-quick` exits 0 with ≥1 task succeeding in CI in ≤ 90 s.
- [x] **BENCH-06**: `bench/BENCH.md` documents operator-side prereqs (Python 3.11+, Docker Engine, per-language toolchains) and provider-TOS attestation (retention-zero verified per provider) parallel to `eval/EVAL.md`. _Acceptance:_ `make verify-tos` exits non-zero if any provider's TOS attestation row is older than 90 days.

### Fairness Contract (FAIR-*)

- [x] **FAIR-01**: Single `bench/runners/fairness_contract.go` struct loaded by every mode — pins temperature, max_tokens, retry policy, system prompt, cached-input handling, model snapshot (e.g. `claude-sonnet-4-5-20260128`). _Acceptance:_ unit test asserts every runner's effective config equals the loaded contract; runtime mismatch is a fatal startup error.
- [x] **FAIR-02**: Dated model snapshot pinned (no alias resolution at runtime). Provider deprecation calendar gate fails the run if the pinned snapshot's EOL is < 30 days away. _Acceptance:_ test passes with a fresh pin; fails with a pin within 30 days of a known deprecation.
- [x] **FAIR-03**: A/B routing / cache-fingerprint variance is detected — each task records the provider-side `usage` block (including cache_read_input_tokens and cache_creation_input_tokens) and the bench reporter flags > 5 % between-run variance as a fairness warning. _Acceptance:_ a synthetic high-variance trace produces a fairness warning in `cost_quality.md`. _Phase split:_ **Phase 75 delivers the FAIR-03 schema substrate only** — the cached-input token columns (`tokens_input_cached_read`, `tokens_input_cache_write`) and the `fairness.overrides[]` array in `result.v2.schema.json` (D-12/D-10). The > 5 % between-run variance detector and the `cost_quality.md` fairness warning land downstream (variance gate consumes STATS-01 N≥3 in Phase 82; the warning renders in the reports phase, Phase 89). Do NOT grade the FAIR-03 acceptance test complete in Phase 75.

### Internal ToolBench (TOOLBENCH-*)

- [x] **TOOLBENCH-01**: 10 capability test classes implemented under `bench/datasets/internal-toolbench/` — semantic view, LSP diagnostics, rename safety, fuzzy search, call graph, dependency graph, patch apply, context minimization, incremental update, failure handling. _Acceptance:_ each capability has at least 1 deterministic test per supported language; capability list documented in `bench/datasets/internal-toolbench/CAPABILITIES.md`.
- [x] **TOOLBENCH-02**: Tier-1 language coverage **Go** — all 10 capabilities have ≥ 1 fixture; `bench/languages/go/runner.go` wraps `go test ./... -json`. _Acceptance:_ ToolBench-Go full run passes locally; capability coverage reported per language.
- [ ] **TOOLBENCH-03**: Tier-1 language coverage **Python** — `pytest --json-report`. _Acceptance:_ ≥ 8/10 capabilities covered; gaps logged.
- [ ] **TOOLBENCH-04**: Tier-1 language coverage **TypeScript** — `vitest --reporter=json` (or `jest --json`). _Acceptance:_ ≥ 8/10 capabilities; LSP diagnostics via tsserver.
- [ ] **TOOLBENCH-05**: Tier-1 language coverage **JavaScript** — `jest --json`. _Acceptance:_ ≥ 8/10 capabilities; eslint-based diagnostics.
- [ ] **TOOLBENCH-06**: Tier-1 language coverage **Java** — `mvn test -Dsurefire.useFile=false`. _Acceptance:_ ≥ 8/10 capabilities; jdtls-backed semantic view.
- [ ] **TOOLBENCH-07**: Tier-1 language coverage **C#** — `dotnet test --logger trx`. _Acceptance:_ ≥ 6/10 capabilities (C# has weaker LSP server coverage); gaps logged.
- [ ] **TOOLBENCH-08**: Tier-1 language coverage **C++** — `cmake/ctest`. _Acceptance:_ ≥ 6/10 capabilities; clangd-backed semantic view.
- [ ] **TOOLBENCH-09**: Tier-1 language coverage **Rust** — `cargo test --message-format=json`. _Acceptance:_ ≥ 8/10 capabilities; rust-analyzer-backed semantic view.
- [x] **TOOLBENCH-10**: `bench/languages/<L>/runner.go` implements a common `LanguageRunner` interface (`Detect`, `Setup`, `RunTests`, `Capabilities`). _Acceptance:_ `go vet` + interface-conformance test passes for all 8 languages.

### Ablation Modes (ABLATE-*)

- [x] **ABLATE-01**: 6 modes implemented: `baseline_plain`, `baseline_rag`, `your_agent_full`, `no_lsp`, `no_semantic`, `no_structured_edit`. _Acceptance:_ each mode runs end-to-end on a smoke task; mode definition lives in `bench/runners/<mode>/MODE.md`.
- [x] **ABLATE-02**: Mode YAML profiles for `your_agent_full`, `no_lsp`, `no_semantic`, `no_structured_edit` ship as `internal/profile/profiles/bench-*.yaml` (4 new YAMLs). _Acceptance:_ profile-filter golden tests cover each YAML; profile loader rejects an unknown mode name.
- [x] **ABLATE-03**: `baseline_plain` mode reuses existing `baseline.yaml` (Phase 67) — no new YAML, no Helix tools exposed. _Acceptance:_ tool inventory for `baseline_plain` is empty except for the shell/grep/read/edit/test the agent gets from the agent runtime itself.
- [ ] **ABLATE-04**: `baseline_rag` mode is implemented as a **standalone MCP server** `cmd/helix-bench-rag` exposing exactly 4 tools (`rag_search`, `rag_read_chunk`, `grep`, `read_file`). It is **not** a Helix profile and shares no code with the Helix daemon's tool surface. _Acceptance:_ `cmd/helix-bench-rag --help` works; tool-list returns exactly 4 tools; vet test asserts no import from `internal/kernel/` or `internal/semantic/`.
- [x] **ABLATE-05**: Kernel-level `disable_lsp_subsystem` flag prevents any back-channel LSP call (including from `analyze_blast_radius` strangler-fig, RepoMap `SetEnrichFn`, live-update OnEdit hooks). _Acceptance:_ E2E `no_lsp` task emits **zero** `lsp.*` OTel spans; trace-tap assertion is a hard fail.
- [x] **ABLATE-06**: Kernel-level `disable_semantic_subsystem` flag prevents any back-channel semantic-store read (including Phase 65 SemanticLookup, RankFiles, ExpandFrom). _Acceptance:_ E2E `no_semantic` task makes zero queries against the duckdb store; runtime assertion logs and fails.
- [x] **ABLATE-07**: Kernel-level `disable_structured_edit_subsystem` flag forces fall-through to plain unified-diff patches; structured-edit tools (replace_symbol_body, fuzzy_edit, etc.) return `unsupported` with a documented kind. _Acceptance:_ `no_structured_edit` mode's tool inventory excludes structured edits; agent receives plain `replace_in_file` only.
- [x] **ABLATE-08**: `vet-ablation-leakage` static analyzer (in `internal/lint/`) fails the build if any mode-restricted tool path reaches a disabled subsystem. Hooked into `make vet`. _Acceptance:_ a deliberate test-case violation makes `make vet` fail.

### Metrics Layer (METRIC-*)

- [x] **METRIC-01**: 12 normalized metrics per task: `task_success`, `verified_correctness`, `tokens_input`, `tokens_output`, `tool_calls`, `wall_time_seconds`, `files_read`, `bytes_read`, `files_modified`, `edit_locality`, `regression_rate`, `lsp_diagnostics_used`. _Acceptance:_ result JSON schema validates against the v1 schema; missing metrics are explicit nulls, not omissions.
- [x] **METRIC-02**: Additional metrics: `semantic_tool_calls`, `edit_distance_patch`, `retry_count`, `compile_errors_before`, `compile_errors_after`. _Acceptance:_ test records all 17 metrics for a real task run.
- [x] **METRIC-03**: `tokens_input/output` are sourced from the **provider's `usage` block**, not Helix's MCP-side counters. _Acceptance:_ regression test asserts source-of-truth for token counts is the provider response, not internal counters.
- [x] **METRIC-04**: `edit_locality` defined as `1 − (modified_files / total_files_in_repo_subtree)`; documented in `bench/evaluators/METRICS.md`. _Acceptance:_ definition test verifies edge cases (root-only edit = 1.0, all-files edit ≈ 0.0).
- [x] **METRIC-05**: `regression_rate` defined as `(failing_pre-existing_tests_post_patch / passing_pre-existing_tests_pre_patch)`. _Acceptance:_ definition test on a synthetic regression case.
- [x] **METRIC-06**: Trace merging from Helix daemon OTel + agent CLI subprocess + bench harness span — single merged trace per `(task, mode, run_index)`. Reuses Phase 67 trace-tap. _Acceptance:_ Jaeger import shows full continuity from `bench.run_id` root to LSP leaves; no orphan spans.

### Verified Correctness (VERIFIED-*)

- [ ] **VERIFIED-01**: `verified_correctness` is computed independently of `task_success` — multi-oracle verdict: (a) all canonical tests pass, (b) all augmented tests pass (UTBoost or equivalent for benchmarks that have them), (c) no pre-existing tests regress. _Acceptance:_ a known-buggy patch that passes only canonical tests gets `task_success=true`, `verified_correctness=false`.
- [ ] **VERIFIED-02**: SWE-bench Verified runs report **both** raw upstream score and UTBoost-augmented rescored score side-by-side. _Acceptance:_ SWE-bench Verified report has both columns; UTBoost augmented suite is wired and reproducible.
- [ ] **VERIFIED-03**: Multi-oracle gate for non-test-bearing benchmarks (CrossCodeEval, RepoBench): EM + edit-similarity + identifier match all required to pass; abstain mode for low-confidence completions. _Acceptance:_ gate documented in `bench/evaluators/VERIFIED.md`; threshold per oracle configurable.

### Statistical Rigor (STATS-*)

- [ ] **STATS-01**: Default `N ≥ 3` runs per task per mode; N configurable per-suite. _Acceptance:_ schema validates `runs` array length ≥ N; matrix runner enforces.
- [ ] **STATS-02**: BCa (bias-corrected accelerated) bootstrap CIs computed over per-task aggregates for every metric on every leaderboard row. Bootstrap iterations ≥ 10,000. _Acceptance:_ unit tests against a closed-form known distribution; CI width sanity-checked.
- [ ] **STATS-03**: `pass@1` and `pass@k` reported per HumanEval closed-form `1 − C(n-c, k)/C(n, k)`. _Acceptance:_ unit test against published reference values.
- [ ] **STATS-04**: Reports flag any cell where the BCa CI overlaps a neighboring cell (no claim of "X > Y" without non-overlapping CIs). _Acceptance:_ a synthetic-overlap test case is rendered with the overlap warning.

### Cost Conversion (COST-*)

- [x] **COST-01**: `bench/datasets/cost-table.yaml` ships with provider × model × `{input_per_mtok, output_per_mtok, currency}` and a `valid_until` date. _Acceptance:_ schema validates; CI gate fails if `valid_until` is past or > 90 days away from `last_verified`.
- [ ] **COST-02**: `cost_per_solved_task` = (sum across solved tasks of provider-side `usage`-derived USD cost) / count(solved). _Acceptance:_ matches a hand-computed example for a known run.
- [ ] **COST-03**: `cost_quality.md` report shows cost-per-solved-task per mode × benchmark with BCa CIs. _Acceptance:_ report renders for a sample run.

### Public Benchmark Adapters (ADAPTER-*)

- [ ] **ADAPTER-AIDER-01**: Aider Polyglot adapter wired via `dataset-loader-only` — shallow git clone `Aider-AI/polyglot-benchmark` at pinned sha; 225 tasks × 6 langs (C++, Go, Java, JS, Python, Rust); 2-attempt protocol with stderr re-prompt. _Acceptance:_ full Aider Polyglot run completes; per-language pass-rate matches sanity benchmarks.
- [ ] **ADAPTER-CCE-01**: CrossCodeEval adapter wired via `dataset-loader-only` — HF dataset; EM + edit-similarity + identifier-match scoring; Python, Java, TS, C#. _Acceptance:_ smoke run scores at least one task per language; scorers unit-tested against CCE paper examples.
- [ ] **ADAPTER-REPO-01**: RepoBench adapter wired via `dataset-loader-only` — RepoBench-R + RepoBench-C + RepoBench-P sub-tasks; Python + Java. _Acceptance:_ smoke run for each sub-task; EM/ES metrics match published reference values on a sampled subset.
- [ ] **ADAPTER-SWE-01**: SWE-bench Verified adapter wired via `subprocess-shellout` — produces `predictions.jsonl`; shells out to `python -m swebench.harness.run_evaluation`; ingests `<run_id>.json`. _Acceptance:_ smoke run of 5 tasks completes; result JSON ingested into bench schema.
- [ ] **ADAPTER-MULTI-01**: Multi-SWE-bench adapter wired via `subprocess-shellout` — `python -m multi_swe_bench.harness.run_evaluation --config`; Java, TS, JS, Go, Rust, C, C++ (Mini set acceptable at ship; full set reach goal). _Acceptance:_ Mini set runs; per-language slicing exposed in reporter.
- [ ] **ADAPTER-TERM-01**: Terminal-Bench 2.0 adapter wired via `subprocess-shellout` — drives agent through `tb run` CLI; ingests `tb` JSON. _Acceptance:_ smoke run of ≥ 5 tasks; container-isolation invariant holds.

### Container Runtime (CONTAINER-*)

- [ ] **CONTAINER-01**: Container orchestration uses `os/exec` to `docker` (or `podman` via drop-in compat). No Go Docker SDK in `go.mod`. _Acceptance:_ `grep "github.com/docker/docker"` in `go.mod` returns empty; bench harness works with either `docker` or `podman` on PATH.
- [ ] **CONTAINER-02**: Per-instance images pinned by SHA256 digest, not tag. Image-cache state cached at `$HELIX_CACHE_DIR/bench-images/<sha>/`. _Acceptance:_ digest-pin test; cache hit on re-run.
- [ ] **CONTAINER-03**: cosign-signed mirror of SWE-bench / Multi-SWE-bench / Terminal-Bench instance images published to a Helix-controlled GHCR namespace; bench harness verifies cosign signature before pulling. _Acceptance:_ mirror exists; verify step is mandatory; tampered image is rejected.
- [ ] **CONTAINER-04**: Disk-budget guard fails the run if available disk on the bench host is < 50 GB before SWE-bench full run. _Acceptance:_ synthetic low-disk test trips the guard.

### Reports (REPORT-*)

- [ ] **REPORT-01**: `bench/reports/leaderboard.md` — top-line table of `(mode × benchmark) → pass@1, verified_correctness, cost_per_solved` with BCa CIs and non-overlap markers. _Acceptance:_ regenerates from a sample run; renders in GitHub markdown.
- [ ] **REPORT-02**: `bench/reports/per_language.md` — per-language slice of all 8 Tier-1 languages × benchmarks that cover that language. _Acceptance:_ a language with no benchmark coverage is explicitly listed as `n/a`, not omitted.
- [ ] **REPORT-03**: `bench/reports/ablations.md` — delta tables: `full vs no_lsp`, `full vs no_semantic`, `full vs no_structured_edit`, `full vs baseline_plain`, `full vs baseline_rag`. Each delta has CI overlap analysis. _Acceptance:_ deltas computed correctly on a sample run.
- [ ] **REPORT-04**: `bench/reports/cost_quality.md` — cost-per-solved-task scatter (cost vs verified_correctness) per mode × benchmark. _Acceptance:_ scatter renders as ASCII / svg; report cites cost-table `valid_until`.
- [ ] **REPORT-05**: Reports are reproducible from a run-id: `helix-bench report --run-id <id>` regenerates all 4 reports byte-identically. _Acceptance:_ `diff` on regenerated vs original report is empty.

### Infrastructure & Hygiene (INFRA-*)

- [x] **INFRA-01**: Provider TOS attestation captured per provider in `bench/PROVIDERS.md` with retention-zero confirmation; CI gate `make verify-tos` (see BENCH-06). _Acceptance:_ attestation per provider used; stale attestations fail CI.
- [x] **INFRA-02**: License audit per dataset captured in `bench/LICENSES.md` — SWE-bench, Multi-SWE-bench, Aider Polyglot (Exercism MIT), CrossCodeEval, RepoBench, Terminal-Bench. _Acceptance:_ each dataset has an explicit license + redistribution clause.
- [x] **INFRA-03**: Eval ↔ bench separation note in `bench/BENCH.md`: `eval/` stays as the in-process PR-gate wiring smoke; `bench/` is the milestone artifact for headline claims (prose-enforced this milestone — no analyzer; D-08). _Acceptance:_ note rendered; reciprocal cross-link added to `eval/EVAL.md` (this one-paragraph pointer is the single permitted change to `eval/` under BENCH-01, which otherwise stays byte-identical).
- [ ] **INFRA-04**: CI policy documented: `make bench-quick` runs on PR (ToolBench Go-only, ≤ 5 minutes, no LLM cost). Full `make bench` runs nightly or on-demand, gated on a maintainer. _Acceptance:_ workflow file exists; PR cost ≤ documented budget.
- [ ] **INFRA-05**: Contamination canary — a known-novel "canary" pattern emitted in select tasks; if a model emits the canary verbatim, the task is flagged as potentially-contaminated and excluded from headline numbers. _Acceptance:_ a synthetic contaminated-response test trips the flag; flagged tasks listed in `leaderboard.md` footnote.

---

## Future Requirements (v1.13+)

- Tier-2 languages (PHP, Ruby, Kotlin, Swift, C, Scala) — ToolBench coverage.
- Tier-3 languages (Lua, Dart, Objective-C, R, Bash, PowerShell) — smoke-only via MultiPL-E / McEval.
- Pluggable agent runners — Codex CLI, Gemini CLI, OpenAI Agents SDK direct, Anthropic SDK direct.
- LLM judge as a CI gate (currently informational only per Phase 67 EVAL-07).
- Public-benchmark *leaderboard submission* infrastructure (SWE-bench leaderboard PR, Aider leaderboard automation).
- Hosted bench-as-a-service (per-PR cost-controlled bench run for any project).
- Differential test execution / mutation testing for `verified_correctness` beyond UTBoost.
- Dynamic cost-table updates via provider pricing-page polling.

---

## Out of Scope (v1.12 explicit exclusions)

- **HumanEval-style toy benchmarks as primary scoring.** MultiPL-E / HumanEval-X / McEval kept as smoke-only signals; never appear on the leaderboard headline.
- **Tier-2 and Tier-3 languages.** Future milestones.
- **LLM judge as a CI gate.** Stays informational per Phase 67 EVAL-07.
- **Comparing against Claude Code / Cursor / Continue as black boxes.** v1.12 only does controlled baselines using the *same* base model. External comparisons are a separate exercise.
- **Public-benchmark leaderboard submissions.** Generating local results is sufficient; submission infra deferred to v1.13+.
- **Embedding/vector search in the Helix daemon.** `baseline_rag` ships as a standalone `cmd/helix-bench-rag` MCP server, never as a Helix profile. The PROJECT.md "Vector/embedding search → out of scope" rule for the daemon stands.
- **CGO=0 build paths.** v1.10 dropped CGO=0 entirely; v1.12 inherits.
- **Reimplementing upstream Python harnesses in Go.** subprocess-shellout for SWE-bench / Multi-SWE-bench / Terminal-Bench; ~6-month effort for zero comparability gain.

---

## Traceability

Populated 2026-06-13 from `.planning/milestones/v1.12-ROADMAP.md`. Every v1 REQ-ID maps to exactly one phase (no orphans, no overlaps). 63/63 mapped.

| REQ-ID | Phase | Plan(s) | Status |
|---|---|---|---|
| BENCH-01 | Phase 75 | TBD | Pending |
| BENCH-02 | Phase 75 | TBD | Pending |
| BENCH-03 | Phase 75 (plan 03) | 479e5c14 | Complete |
| BENCH-04 | Phase 77 (plans 01-03) | 77-03 | Complete |
| BENCH-05 | Phase 77 (plan 05) | 9aa1fa49 | Complete |
| BENCH-06 | Phase 75 | TBD | Pending |
| FAIR-01 | Phase 75 | TBD | Pending |
| FAIR-02 | Phase 75 | TBD | Pending |
| FAIR-03 | Phase 75 plan 03 = schema substrate landed (479e5c14); variance gate Phase 82, warning Phase 89 | 479e5c14 (substrate) | Substrate done; full REQ pending Phase 82/89 |
| TOOLBENCH-01 | Phase 78 (plans 02-05) | 78-05 | Complete |
| TOOLBENCH-02 | Phase 78 (plans 01-05) | 78-05 | Complete |
| TOOLBENCH-03 | Phase 85 | TBD | Pending |
| TOOLBENCH-04 | Phase 85 | TBD | Pending |
| TOOLBENCH-05 | Phase 85 | TBD | Pending |
| TOOLBENCH-06 | Phase 85 | TBD | Pending |
| TOOLBENCH-07 | Phase 85 | TBD | Pending |
| TOOLBENCH-08 | Phase 85 | TBD | Pending |
| TOOLBENCH-09 | Phase 85 | TBD | Pending |
| TOOLBENCH-10 | Phase 78 (plan 01) | 78-05 | Complete |
| ABLATE-01 | Phase 80 | TBD | Pending |
| ABLATE-02 | Phase 76 | 76-02 | Complete |
| ABLATE-03 | Phase 80 | TBD | Pending |
| ABLATE-04 | Phase 83 | TBD | Pending |
| ABLATE-05 | Phase 76 | TBD | Pending |
| ABLATE-06 | Phase 81 | 81-04, 81-05 | Complete |
| ABLATE-07 | Phase 76 | TBD | Pending |
| ABLATE-08 | Phase 76 | TBD | Pending |
| METRIC-01 | Phase 79 | 79-01 | Complete |
| METRIC-02 | Phase 79 | 79-02 | Complete |
| METRIC-03 | Phase 79 | 79-03 | Complete |
| METRIC-04 | Phase 79 | 79-02 | Complete |
| METRIC-05 | Phase 79 | 79-02 | Complete |
| METRIC-06 | Phase 79 | 79-04 | Complete |
| VERIFIED-01 | Phase 87 | TBD | Pending |
| VERIFIED-02 | Phase 87 | TBD | Pending |
| VERIFIED-03 | Phase 86 | TBD | Pending |
| STATS-01 | Phase 82 | 82-01 (producer half) | In progress — producer side done (ExpandMatrix N cells + --runs); aggregator fail-closed N-gate lands in 82-05 |
| STATS-02 | Phase 82 | TBD | Pending |
| STATS-03 | Phase 82 | TBD | Pending |
| STATS-04 | Phase 82 | TBD | Pending |
| COST-01 | Phase 75 | TBD | Pending |
| COST-02 | Phase 82 | TBD | Pending |
| COST-03 | Phase 82 | TBD | Pending |
| ADAPTER-AIDER-01 | Phase 85 | TBD | Pending |
| ADAPTER-CCE-01 | Phase 86 | TBD | Pending |
| ADAPTER-REPO-01 | Phase 86 | TBD | Pending |
| ADAPTER-SWE-01 | Phase 87 | TBD | Pending |
| ADAPTER-MULTI-01 | Phase 88 | TBD | Pending |
| ADAPTER-TERM-01 | Phase 88 | TBD | Pending |
| CONTAINER-01 | Phase 84 | TBD | Pending |
| CONTAINER-02 | Phase 84 | TBD | Pending |
| CONTAINER-03 | Phase 84 | TBD | Pending |
| CONTAINER-04 | Phase 84 | TBD | Pending |
| REPORT-01 | Phase 89 | TBD | Pending |
| REPORT-02 | Phase 89 | TBD | Pending |
| REPORT-03 | Phase 89 | TBD | Pending |
| REPORT-04 | Phase 89 | TBD | Pending |
| REPORT-05 | Phase 89 | TBD | Pending |
| INFRA-01 | Phase 75 | TBD | Pending |
| INFRA-02 | Phase 75 | TBD | Pending |
| INFRA-03 | Phase 75 | TBD | Pending |
| INFRA-04 | Phase 89 | TBD | Pending |
| INFRA-05 | Phase 89 | TBD | Pending |
