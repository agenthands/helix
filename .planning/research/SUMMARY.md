# Project Research Summary

**Project:** Helix v1.12 — Bench Stack & Tool Evaluation
**Domain:** Agentic-coding benchmark harness — public benchmark adapters + internal ToolBench + 6-mode ablation matrix across 8 Tier-1 languages
**Researched:** 2026-06-13
**Confidence:** HIGH

## Executive Summary

v1.12 ships a net-new `bench/` tree and a sibling `cmd/helix-bench` binary that drives **six controlled ablation modes** (`baseline_plain`, `baseline_rag`, `your_agent_full`, `no_lsp`, `no_semantic`, `no_structured_edit`) against six public benchmarks (Aider Polyglot, CrossCodeEval, RepoBench, SWE-bench Verified, Multi-SWE-bench, Terminal-Bench 2.0) plus an internal 10-capability × 8-language ToolBench that is the *deterministic* source of truth. The load-bearing external claim — *"same model + same budget, with Helix the agent solves more tasks with fewer tokens, fewer files read, fewer destructive edits"* — is only credible if (a) ablations actually hold model + budget + system prompt constant, (b) public-benchmark numbers are statistically sound with N≥3 + BCa bootstrap CI, and (c) the known SWE-bench "tests-pass ≠ correct" hole is addressed via UTBoost-augmented rescoring + multi-oracle `verified_correctness`.

The recommended approach is a strict **subprocess-shellout** posture for the heavy upstream Python harnesses (SWE-bench, Multi-SWE-bench, Terminal-Bench) combined with **dataset-loader-only** adapters for completion-style benchmarks (Aider Polyglot, CrossCodeEval, RepoBench), all backed by a thin Go orchestrator. Container runtime is **`os/exec` to the `docker` CLI** (not the Go Docker SDK — see conflict resolution below) so results are bit-for-bit comparable to upstream harnesses and podman is a drop-in. RAG is a **standalone `cmd/helix-bench-rag` stdio MCP server** exporting 4 fixed tools (`rag_search`, `rag_read_chunk`, `grep`, `read_file`) — never a Helix profile — to prevent contamination of the production tool surface. `eval/` (v1.10 Phase 67) is frozen and stays as the PR-gated wiring smoke; `bench/` is the milestone artifact.

Key risks cluster around four BLOCKER classes: **training-data contamination** (mitigate via Verified-only headlines + canary-emission probe + delta-only reporting), **patch-validation false positives** (mitigate via run-all-tests override + UTBoost-augmented suite + multi-oracle `verified_correctness ≠ tests_pass`), **ablation leakage** (LSP / semantic / structured-edit subsystems have back-channels through `analyze_blast_radius`, RepoMap enrichment, OnEdit hooks — mitigate via hard kernel-level `disable_*_subsystem` flags + `vet-ablation-leakage` analyzer + zero-`lsp.*`-spans trace assertion), and **fairness drift** (temperature, retry, cache, system-prompt, dated model snapshot — mitigate via a single `fairness_contract.go` struct that all modes load from). Secondary HIGH risks: token-counting attribution (provider `usage` block, not Helix's MCP counter), static cost-table drift (hard `valid_until` expiration), bootstrap CI math (BCa not percentile, N≥10,000), per-language toolchain hermeticity (pre-baked images + `--network=none`), Docker disk-explosion (189 GB+ unoptimized → 30 GB via logicstar mirror + cosign-signed GHCR copy).

## Key Findings

### Recommended Stack

The v1.10 stack is fixed; v1.12 adds operator-side runtime requirements (Python 3.11+, Docker Engine, per-language toolchains) for `cmd/helix-bench` *only*. The shipping `helix` daemon binary is unchanged — single-binary distribution survives. See `.planning/research/STACK.md` for the full additions table.

**Core technologies (new at v1.12):**
- **Upstream Python harnesses via `subprocess-shellout`** — `swebench` PyPI v4.x, `multi-swe-bench` (main), `terminal-bench` `tb` CLI — reproducing published numbers exactly without reimplementing 6+ months of container infra.
- **Docker via `os/exec` to the `docker` CLI** (with podman drop-in) — chosen over Go Docker SDK per ARCHITECTURE.md verdict; bit-exact match to upstream harnesses; zero new heavy deps.
- **`github.com/philippgille/chromem-go` v0.7.x** — zero-deps in-process vector store for `baseline_rag`; no FAISS, no Weaviate, no external service.
- **OpenAI `text-embedding-3-small` (primary) / Ollama `nomic-embed-text` (offline fallback)** — embedding providers for RAG baseline; document choice in `bench/runners/baseline_rag_agent/EMBED-CHOICE.md`.
- **`github.com/gomlx/go-huggingface` v0.x + `arrow-go/v18` parquet fallback** — HF dataset fetcher with `$HELIX_CACHE_DIR/bench-datasets/<sha>/` content-hashed cache.
- **`gonum.org/v1/gonum/stat`** (promoted from test-only to runtime for bench/) — base for bootstrap; ~40 LOC BCa helper on top; pass@k closed-form per HumanEval paper.
- **Reused from v1.10 Phase 67:** `tiktoken-go/tokenizer` v0.6, `bluekeyes/go-gitdiff` v0.8, `anthropic-sdk-go` v1.35, `openai-go` v1.12, `koanf/v2`, modernc.org/sqlite, OTel + propagation.TraceContext.
- **Per-language test runners via stdlib `os/exec`** — no per-language Go bindings; `go test`, `pytest --json-report`, `npx jest/vitest`, `mvn test`, `dotnet test`, `cargo test`, `ctest`.

### Expected Features

See `.planning/research/FEATURES.md`. Helix is the only entry in the surveyed peer set that holds model fixed across a *layered subtractive* ablation while reporting raw + UTBoost-augmented SWE-bench pass-rates side-by-side.

**Must have (table stakes):**
- `bench/` tree skeleton + `cmd/helix-bench` + `cmd/helix-bench-rag` sibling binaries
- Internal ToolBench: 10 capabilities × 8 Tier-1 langs, deterministic, <60s wall, runs in `make bench-quick` + `make test`
- 6-mode ablation matrix with same-model-same-budget invariant enforced by `fairness_contract.go`
- Aider Polyglot adapter (cheapest, 6 langs, first external number)
- CrossCodeEval adapter (only public coverage for C#)
- SWE-bench Verified adapter (headline external benchmark)
- Normalized per-task `result.v2.json` with all 12+ PROJECT.md metrics
- `pass@1` + `pass@k=3` + 95% BCa bootstrap CI (N_resamples ≥ 10,000)
- `cost_per_solved_task` from `bench/datasets/cost-table.yaml` with hard `valid_until` expiration
- UTBoost-augmented SWE-bench pass-rate reported alongside raw
- `leaderboard.md`, `per_language.md`, `ablations.md`, `cost_quality.md` auto-generated
- `make bench` / `bench-quick` / `bench-<suite>` targets; CI gates on ToolBench only
- Per-instance Docker image cache with pinned sha256 digests + cosign-signed GHCR mirror

**Should have (Helix-specific differentiators):**
- Layered subtractive ablations (`no_lsp`, `no_semantic`, `no_structured_edit`) as headline attribution mechanism — no peer publishes this
- Edit-locality + regression-rate as first-class headline metrics (conditioned on success to prevent gaming)
- Cost-per-solve broken out per ablation mode
- Multi-SWE-bench adapter (7 langs, scale story, covers Tier-1 gaps)
- RepoBench adapter (clean retrieval-only story for RepoMap attribution)
- Terminal-Bench 2.0 adapter (long-horizon control)

**Defer (v2+):**
- Public leaderboard submission infra
- Continuous benchmarking dashboard
- Tier-2 / Tier-3 language ToolBench fixtures
- Cross-vendor head-to-head vs Cursor / Cody / Continue

**Anti-features (explicitly NOT to ship):**
- HumanEval / MBPP as primary scoring (smoke-only via MultiPL-E if at all)
- LLM-judge as CI gate or `verified_correctness` contributor (EVAL-07 holds; judge stays in `informational_quality_score`)
- Black-box product comparisons
- Single-run results in any external report
- Helix tools exposed to the RAG baseline arm (contamination)

### Architecture Approach

See `.planning/research/ARCHITECTURE.md`. Five architectural verdicts drive everything else:

1. **`cmd/helix-bench` is a NEW sibling binary** to `cmd/helix-eval`, not an extension — different SLOs.
2. **Container runtime is `os/exec` shell-out to `docker` (with podman fallback)** — bit-exact match to public harnesses.
3. **`baseline_rag` is `cmd/helix-bench-rag` as its own stdio MCP server** — 4 fixed tools, never a Helix profile.
4. **Five of six ablation modes are profile YAMLs** (`bench-full`, `bench-no-lsp`, `bench-no-semantic`, `bench-no-structured-edit`, plus reused `baseline.yaml` for `baseline_plain`). `no_semantic` also requires `semantic_index.bench_disabled: true` config-key gate at daemon bootstrap to un-wire Phase 65 strangler-fig.
5. **Per-language test runners live in `bench/languages/<L>/testrunner.go`** — one package per language; `bench/evaluators/test_runner/dispatch.go` is a 30-LOC switch.

**Major components:**
1. **`cmd/helix-bench`** — orchestrator, matrix expander, work-stealing scheduler, container pool, content-hash result cache
2. **`cmd/helix-bench-rag`** — standalone stdio MCP server for RAG baseline
3. **`bench/runtime/{container,subprocess,sandbox}/`** — Docker/podman shellout, daemon+agent+rag subprocess control, per-cell sandbox wrapping `internal/eval/sandbox/`
4. **`bench/datasets/<X>/adapter.go`** — one adapter per public benchmark + `internal-toolbench/`
5. **`bench/runners/<mode>/runner.go`** — six mode runners
6. **`bench/languages/<L>/{contract_test.go,testrunner.go}`** — per-language assertions + native test runner
7. **`bench/evaluators/{test_runner,patch_validator,semantic_oracle,token_meter,tool_trace_analyzer,regression_checker}/`** — cross-language graders
8. **`bench/aggregator/`** — N≥3 multi-run rollup, BCa bootstrap CI, pass@k, cost rollup
9. **`bench/reports/`** — generated markdown
10. **`internal/profile/profiles/bench-*.yaml`** — four new ablation profile YAMLs
11. **`semantic_index.bench_disabled` config key** — only new code path inside the daemon

**Conflict resolution (STACK.md vs ARCHITECTURE.md — Docker SDK):** STACK.md proposes `github.com/docker/docker` v27 Go client; ARCHITECTURE.md rejects it in favor of `os/exec` to the `docker` CLI. **ARCHITECTURE.md wins.** Rationale: bench invokes Docker ≤ once per (task,mode,run) so call latency is irrelevant against tens-of-seconds LLM latency; shell-out is the only way to demonstrably match the public harnesses; podman compatibility comes free; the Docker Go SDK pulls a ~8MB compiled / ~500MB-source dep tree. STACK.md's projected `go.mod` diff drops the `docker/docker` line.

### Critical Pitfalls

See `.planning/research/PITFALLS.md`. Top 5 (all BLOCKER except #5):

1. **Training-data contamination (BLOCKER)** — frontier models have seen public benchmarks. Mitigate via SWE-bench Verified as *only* SWE-family headline; canary-emission probe on every benchmark; **delta-only reporting** (Helix − baseline_plain on same model; contamination affects both arms equally); per-dataset `CANARY.txt`.
2. **Patch-validation false positives (BLOCKER)** — SWE-bench's per-instance harness only re-runs PR-modified tests; UTBoost shows ~31% of "solved" Verified patches are semantically wrong. Mitigate via **run-all-tests override**; **multi-oracle `verified_correctness`** (tests-pass AND ≥1 of {gold-patch-diff-overlap, no-new-diagnostics, mutation-survival}); never collapse `verified_correctness ≡ tests_pass` (Phase 67 F-09 lesson).
3. **Ablation-mode leakage (BLOCKER)** — `no_lsp` profile filters `tools/list` (visibility) but doesn't stop `analyze_blast_radius` from calling `lspProbeForEdges`, RepoMap's `SetEnrichFn`, or Phase 60 OnEdit hooks. Mitigate via hard kernel-level `disable_*_subsystem` flags plumbed to `kernel.NewWorkspace`; `vet-ablation-leakage` static analyzer; escape-path audit tests asserting zero LS workers + zero `lsp.*` spans for `no_lsp`.
4. **Same-model fairness drift (BLOCKER)** — trivial temperature/retry/cache/system-prompt asymmetries invalidate the headline. Mitigate via single `bench/runners/fairness_contract.go` struct pinning dated model snapshot (`claude-sonnet-4-5-20260520`), temperature, max_tokens, system_prompt_hash, retry policy, cache policy; CI gate refuses mode overrides without explicit `WaiverReason`; disable Anthropic prompt caching globally for headline runs; deprecation-calendar gate.
5. **Token-counting attribution boundary (HIGH)** — Helix's MCP-level counter ≠ what the provider bills; headline must source from provider `usage` block with `tokens_input_uncached`/`tokens_input_cached_read`/`tokens_input_cache_write`/`tokens_output` reported separately.

PITFALLS.md ships 19 pitfalls plus a 24-item "Looks Done But Isn't" checklist and a complete pitfall-to-phase mapping table the roadmapper should consume directly.

## Implications for Roadmap

The architecture's 18-step build order (ARCHITECTURE.md §"Phase Build Order") is the canonical dependency graph; suggestions below collapse into ~15 roadmap phases. Strictly bottom-up.

### Phase 1: Schema, fairness contract, and tree skeleton
**Rationale:** Every later phase writes `result.v2.json` or reads `bench.yaml` or loads `fairness_contract.go`. Schema-first lets every subsequent phase be schema-validated; fairness-contract-first means no benchmark adapter ever defines its own model config.
**Delivers:** `bench/` tree skeleton; `bench/schema/result.v2.schema.json`; `bench/bench.yaml` shape; `bench/runners/fairness_contract.go`; content-hash cache key derivation; `bench/datasets/cost-table.yaml` with `valid_until`; `bench/providers/tos_attestation.yaml` + `make verify-tos` gate; per-dataset `CANARY.txt` slots.
**Avoids:** Pitfalls 4, 6, 7, 1 (canary infra).
**Research flag:** None — patterns from Phase 67.

### Phase 2: Ablation profile YAMLs + kernel-level disable flags
**Rationale:** The `disable_*_subsystem` kernel flags + `semantic_index.bench_disabled` config gate are the *only* invasive code paths inside the daemon — land them early so they're stable.
**Delivers:** Four new `bench-*.yaml`; kernel disable flags plumbed to `kernel.NewWorkspace`; LS pool / RepoMap `SetEnrichFn` / `lspProbeForEdges` no-op paths; `vet-ablation-leakage` analyzer; escape-path audit tests.
**Avoids:** Pitfall 3 (central correctness invariant for the milestone).
**Research flag:** **NEEDS RESEARCH** — strangler-fig (Phase 65) and OnEdit hooks (Phase 60) have non-obvious back-channels. Roadmapper should add a research pass enumerating every LSP/semantic consumer in the kernel.

### Phase 3: Bench runtime (sandbox + subprocess, no container) + first E2E smoke
**Rationale:** Reuses `internal/eval/sandbox/` via thin wrapper; subprocess control without containers gets us first E2E smoke (one task, one mode) and forces the orchestrator shape.
**Delivers:** `bench/runtime/sandbox/`; `bench/runtime/subprocess/{daemon.go,agent.go}`; `cmd/helix-bench run` skeleton + matrix expander; `bench/runners/your_agent_full/runner.go`; first smoke test.
**Uses:** `internal/eval/sandbox/`, `internal/eval/trace/`, `internal/eval/budget/`.

### Phase 4: Internal ToolBench — Go first
**Rationale:** Go is Helix's own language; tightest feedback loop; no container; validates per-language test-runner pattern before generalizing.
**Delivers:** `bench/languages/go/`; `bench/datasets/internal-toolbench/go/` first 3 capability fixtures; `capabilities.yaml` schema.

### Phase 5: Evaluators (test_runner dispatch + patch_validator + token_meter + tool_trace_analyzer + regression_checker)
**Rationale:** Minimum graders to produce meaningful `result.v2.json`. Introduces two-counter `tokens_to_model` + `tokens_through_daemon` schema sourced from provider `usage`.
**Delivers:** All evaluators; per-language dispatch interface; cost computation from `cost-table.yaml`.
**Avoids:** Pitfalls 5, 10 (`edit_locality_given_solved` as headline), 16 (judge in `informational_quality_score`).

### Phase 6: Remaining ablation runners (baseline_plain + no_lsp + no_structured_edit)
**Rationale:** No new infra. First three ablation comparisons unlock attribution story on Go ToolBench alone.
**Delivers:** Three runner packages; first ablation rows in result schema.
**Decision required:** Native agent edit tools (`Edit`, `Read`) enabled across all modes? Recommendation: yes (matches `claude-code.yaml`). Document in `bench/BENCH.md`.

### Phase 7: `no_semantic` mode + E2E config-gate test
**Rationale:** Trickiest mode — strangler-fig in `get_repo_map`/`get_context` cannot be un-wired by profile YAML alone. Gets a dedicated regression test asserting `get_repo_map` returns v1.9 tree-sitter path (`source == "tree_sitter"`) when `bench_disabled` is set.
**Delivers:** `bench/runners/your_agent_no_semantic/`; plumbing E2E test; vet-style boundary guard.
**Avoids:** Pitfall 3 (semantic leakage variant).

### Phase 8: Multi-run aggregator + statistical rigor + first leaderboard
**Rationale:** First externally-publishable artifact. Tests N≥3 multi-run, BCa bootstrap, pass@k, cost rollup. Output is internal-ToolBench-Go-only.
**Delivers:** `bench/aggregator/`; `cmd/helix-bench aggregate`; first `bench/reports/leaderboard.md`; power-analysis precondition gate.
**Avoids:** Pitfalls 11 (BCa, N≥10,000), 12 (task randomization, seed pinning).

### Phase 9: `cmd/helix-bench-rag` + baseline_rag runner + embedding-index builder
**Rationale:** Self-contained; benefits from soaking before public benchmarks land. Embedding choice + index build pipeline + per-corpus cache.
**Delivers:** `cmd/helix-bench-rag/`; `bench/runners/baseline_rag_agent/`; OpenAI + Ollama embedding providers; chromem-go vector store; `EMBED-CHOICE.md`.
**Avoids:** Pitfall 4 (RAG isn't a Helix profile — clean separation).
**Research flag:** **NEEDS RESEARCH** — specific embedder + chunking strategy has direct credibility implications.

### Phase 10: Container runtime + warm-pool + cosign-signed GHCR image mirror
**Rationale:** Container infra needed only for public benchmarks. Internal ToolBench + RAG + Aider Polyglot (subset) run container-free up to here. Adopt logicstar.ai optimized image set; mirror to `ghcr.io/agenthands/helix-bench-*` with cosign keyless via Phase 58 infra; pre-flight disk check.
**Delivers:** `bench/runtime/container/` with Docker + Podman implementations; image-pull manifest; `make bench-setup` pre-pull; disk-space pre-flight; arch-mismatch refusal.
**Avoids:** Pitfall 8 (disk explosion + Docker Hub rate limits).

### Phase 11: Aider Polyglot adapter + 7 remaining per-language test runners
**Rationale:** Cheapest public benchmark first (225 tasks); drives the order of remaining `bench/languages/` runners by Aider's 6-lang coverage. Per-language toolchain images pre-baked + hermetic `--network=none`.
**Delivers:** `bench/datasets/aider-polyglot/adapter.go`; 7 per-language testrunners; 2-attempt-with-feedback pattern; pre-baked toolchain images.
**Avoids:** Pitfalls 9, 14 (per-track Exercism license audit).
**Research flag:** **NEEDS LICENSE AUDIT** before any mirroring.

### Phase 12: CrossCodeEval + RepoBench adapters
**Rationale:** Mid-size completion-only public benchmarks; cover C#. Pure `dataset-loader-only`; EM + ES + identifier-match + acc@k metrics.
**Delivers:** Both adapters; completion-style scorer; HF dataset fetcher.
**Uses:** `gomlx/go-huggingface` + arrow-go fallback.

### Phase 13: SWE-bench Verified adapter + UTBoost rescorer + multi-oracle `verified_correctness`
**Rationale:** First containerized large-scale benchmark and the *headline* external number. Requires Phase 10 + Python toolchain image (Phase 11). UTBoost-augmented suite consumed; raw + augmented pass-rates reported side-by-side.
**Delivers:** SWE-bench adapter; `bench/evaluators/swebench/differential.go`; UTBoost ingestion; multi-oracle `verified_correctness`; run-all-tests harness override.
**Avoids:** Pitfalls 2 (load-bearing mitigation), 1 (canary-probe column).
**Research flag:** **NEEDS RESEARCH** — UTBoost suite location + format + adapter shape needs concrete validation.

### Phase 14: Multi-SWE-bench + Terminal-Bench 2.0 adapters
**Rationale:** Final public coverage. Multi-SWE-bench: 7 langs × ~230 tasks. Terminal-Bench 2.0: long-horizon control with `tb` CLI, some tasks > 1 day wall.
**Delivers:** Both adapters; long-wall scheduler accommodations; per-language slicing in reporter.
**Research flag:** **NEEDS LICENSE RESOLUTION** for Multi-SWE-bench — not surfaced on HF card; defer to v1.13 if unresolved.

### Phase 15: Reports finalize + CI policy + docs + eval↔bench separation note
**Rationale:** Reports come after every input that feeds them is stable. CI-policy split (`bench-quick` ≤ 5 min on PR; `bench` only on release-tag with maintainer label) protects the $1K–$10K cost ceiling. `eval/EVAL.md` clarification preserves Phase 67 compatibility.
**Delivers:** Remaining report generators; `make bench` cost-guard (`BENCH_FULL=1` or interactive); `--resume` per-cell checkpointing; smoke-bench daily drift detector ($20/day cap); `bench/BENCH.md`; `eval/EVAL.md` migration note; `PHASE67_CROSSWALK.md`.
**Avoids:** Pitfalls 15, 17, 18 (F-07 parametric across modes), UX pitfalls.

### Phase Ordering Rationale

- **Schema and fairness first (Phase 1):** every downstream phase writes a result row or pins a model config.
- **Profile YAMLs + kernel disable flags second (Phase 2):** only invasive daemon-side change in the milestone; stable target for everything that follows.
- **Go ToolBench third (Phase 4):** dogfooding our own language for tightest debug loop.
- **Evaluators before more runners (Phase 5 before 6):** unscored results are useless.
- **Aggregator before public benchmarks (Phase 8 before 11):** first leaderboard renders against ToolBench-only data; public-benchmark phases write into a stable consumer.
- **RAG before container (Phase 9 before 10):** soak the cheaper subsystem first.
- **Container before SWE-bench (Phase 10 before 13):** runtime hardened against Aider Polyglot Python slice first.
- **SWE-bench before Multi-SWE-bench and Terminal-Bench (Phase 13 before 14):** Verified is the canonical reference.
- **Reports last (Phase 15):** stabilize only once inputs are stable.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 2 (ablation kernel flags):** complete LSP/semantic consumer enumeration in kernel
- **Phase 9 (baseline_rag embedder choice):** 2026 RAG baselines move fast; reviewer-facing
- **Phase 11 (Aider Polyglot license audit):** per-Exercism-track licensing
- **Phase 13 (UTBoost integration):** concrete suite + format + adapter shape
- **Phase 14 (Multi-SWE-bench license):** not surfaced on HF card; resolve or defer

Phases with standard patterns (skip research-phase):
- Phase 1 (schema/contract — Phase 67 shape proven)
- Phase 3 (subprocess sandbox — direct extension)
- Phase 4 (Go ToolBench — Helix owns Go LS + tools)
- Phase 5 (evaluators — wrappers over existing eval/score + gitdiff + tiktoken)
- Phase 8 (aggregator + BCa — gonum + 40 LOC well-defined)
- Phase 10 (Docker shellout — trivial os/exec; cosign mirror reuses Phase 58)
- Phase 12 (CrossCodeEval + RepoBench — completion-only adapters)
- Phase 15 (reports + CI — markdown gen + Makefile policy)

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Every addition has documented Go-native equivalent + version pin; only ambiguity is Docker SDK vs CLI (resolved in favor of CLI). One MEDIUM: gomlx/go-huggingface API stability not field-tested in-tree. |
| Features | HIGH | Public-benchmark mechanics + peer conventions corroborated by multiple sources per benchmark; UTBoost is canonical SWE-bench-correctness citation; ToolBench-style internal contract suites have few public exemplars but Phase 67 patterns extrapolate cleanly (MEDIUM there). |
| Architecture | HIGH | Five verdicts grounded in concrete v1.10/v1.11 source-code patterns; explicit "new vs modified" table; explicit anti-patterns; explicit build-order DAG. |
| Pitfalls | HIGH | Every pitfall grounded in Phase 67 audit findings or published evidence from the specific benchmarks v1.12 adopts. |

**Overall confidence:** HIGH

### Gaps to Address

- **UTBoost adapter shape (Phase 13):** consume published augmented suite as-is vs re-derive — concrete API/format check needed before phase plan.
- **Multi-SWE-bench license (Phase 14):** not surfaced on HF dataset card. Resolve upstream or defer to v1.13.
- **`baseline_rag` embedder model pin:** OpenAI `text-embedding-3-small` is industry-default but reviewer-sensitive; consider also reporting against `nomic-embed-text` to pre-empt "weak embedder" critique.
- **Native agent-CLI tool surface across modes (Phase 6 decision):** Recommendation yes (matches `claude-code.yaml`); needs explicit doc in `bench/BENCH.md`.
- **`gomlx/go-huggingface` field-test (Phase 12):** API stability not exercised in-tree; have early exploratory check with `arrow-go/v18` fallback ready.
- **Per-language toolchain CI install matrix (Phase 11):** Maven, .NET SDK, CMake+clang, cargo, pnpm — define version pins before Phase 11 starts.

## Sources

### Primary (HIGH confidence)

Internal research files:
- `.planning/research/STACK.md` — 12 stack additions, alternatives, CGO posture, projected `go.mod` diff
- `.planning/research/FEATURES.md` — feature landscape, 6 public benchmarks, 6-mode matrix, 12+ metrics, competitor analysis
- `.planning/research/ARCHITECTURE.md` — four executive verdicts, canonical `bench/` tree, component table, 18-step build order, 6 BLOCKER-level risks
- `.planning/research/PITFALLS.md` — 19 pitfalls + 24-item completeness checklist + pitfall-to-phase mapping
- `.planning/PROJECT.md` lines 147–187 — v1.12 milestone scope

Helix-internal references:
- `eval/EVAL.md`, `internal/eval/{sandbox,trace,score}/` — Phase 67 v1.10 patterns reused verbatim
- `cmd/helix-eval/`, `cmd/helix-eval/run_cmd_test.go` — EVAL-07 invariants
- `internal/profile/profiles/{baseline,claude-code}.yaml` — reference shape for `bench-*.yaml`
- `.planning/v1.10-MILESTONE-AUDIT.md` F-01/F-05/F-07/F-08/F-09 findings + commits

Public-benchmark research (canonical sources):
- SWE-bench Verified docs + dataset card + Epoch AI 1-hour guide + UTBoost paper (arxiv 2506.09289)
- Multi-SWE-bench paper (arxiv 2504.02605) + HF dataset + GitHub
- Aider Polyglot launch post + leaderboard + polyglot-benchmark repo
- CrossCodeEval NeurIPS 2023 paper + project site
- RepoBench paper (arxiv 2306.03091) + Leolty/repobench
- Terminal-Bench 2.0 (Snorkel announcement, VentureBeat, ICLR 2026 paper)
- logicstar.ai SWE-bench image-optimization writeup (684 GiB → 67 GiB)

### Secondary (MEDIUM confidence)

- `gomlx/go-huggingface` README — parquet iterator API; not field-tested in-tree
- `philippgille/chromem-go` README + pkg.go.dev
- 2026 cost-per-task leaderboard pages (morphllm, SSOJet, costgoat)
- BCa bootstrap notes (SAS blog, Sebastian Schöner, arch.readthedocs)

### Tertiary (LOW confidence)

- ByteDance-Seed Multi-SWE-bench license clause — not surfaced; needs upstream confirmation
- UTBoost augmented-suite consumption format for non-Verified splits — integration shape needs validation

---
*Research completed: 2026-06-13*
*Ready for roadmap: yes*
