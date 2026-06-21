# Feature Research — v1.12 Bench Stack & Tool Evaluation

**Domain:** Agentic coding benchmark harness — public benchmark adapters + internal ToolBench + ablation matrix
**Researched:** 2026-06-13
**Confidence:** HIGH for public-benchmark mechanics and peer reporting conventions (multiple corroborating sources per benchmark); HIGH for SWE-bench "verified correctness" weakness (UTBoost is the canonical citation); MEDIUM for ToolBench-style internal contract suites (very few public exemplars — CodeAgent-style ablations and Aider's own polyglot harness are the closest analogues).

---

## Headline framing for downstream roadmap

> "Same model + same budget — with Helix the agent solves more tasks, with fewer tokens, fewer files read, and fewer destructive edits." (PROJECT.md line 149)

To land that claim externally and credibly, three things must all be true at once:

1. **The benchmark menu must overlap with what peers publish.** Aider Polyglot, SWE-bench Verified, Multi-SWE-bench, CrossCodeEval, RepoBench, Terminal-Bench 2.0 are the *de facto* 2026 menu. Hitting any single one is dismissible; hitting the union is hard to dismiss.
2. **The ablations must be in-house controlled.** No black-box "we beat Claude Code." Same base model, six modes, headline number reported on `your_agent_full` vs `baseline_plain` and `baseline_rag` at fixed budget.
3. **The verified-correctness story must address the known SWE-bench test-coverage hole.** UTBoost (2026) showed 24.4 % of SWE-bench Verified rankings shift under augmented tests — peers will ask this question, and the table-stakes answer is "we report both raw-tests pass-rate AND UTBoost-augmented pass-rate side-by-side."

Everything below feeds those three requirements.

---

## Feature Landscape

### Table Stakes (peers ship these — Helix must match to be credible)

#### Category 1 — Public benchmark adapters

| Feature | Why expected | Complexity | Notes |
|---|---|---|---|
| **SWE-bench Verified adapter** (500 Python tasks, Docker per instance, fail_to_pass + pass_to_pass) | Industry-default headline. Every coding-agent paper since 2024-08 reports this number. | XL | Reference harness in `swebench/harness`; each instance ships its own Docker image; 500 × ~5 min wall ≈ 40 h single-machine, parallelizable. Docker per-instance is non-negotiable for repro. |
| **Multi-SWE-bench adapter** (1,632 instances × 7 langs: Java, TS, JS, Go, Rust, C, C++) | Only credible multilingual SWE-bench-style benchmark; covers 7 of 8 Tier-1 langs (gap: C#). | XL | Driven by `multi_swe_bench.harness.run_evaluation --config`; produces `final_report.json` with resolved/unresolved counts. Per-instance Dockerfiles same shape as SWE-bench. |
| **Aider Polyglot adapter** (225 hardest Exercism tasks × 6 langs: C++, Go, Java, JS, Py, Rust) | The peer benchmark every coding model gets compared on; covers 6 of 8 Tier-1 langs (gap: TS, C#). Cheap and fast. | M | Two attempts per task with test-error feedback after attempt 1. Two metrics: `percent_correct` and `correct_edit_format`. Edit-format is a separate axis Helix should track. |
| **CrossCodeEval adapter** (cross-file completion × Py, Java, TS, C#) | Covers cross-file completion as a separate skill from issue-fixing; covers 4 of 8 Tier-1 langs including C# (only public benchmark in our menu that does). | M | Static-analysis-validated tasks that *require* cross-file context. Metrics: EM, ES on code; identifier-match for API names. |
| **RepoBench adapter** (RepoBench-R/C/P × Py, Java) | Forces retrieval to be measured separately from generation — a clean win condition for Helix's RepoMap. | M | Three sub-tasks: R (retrieval `acc@k`), C (next-line `EM`/`ES`), P (pipeline). Python @ 12k token context, Java @ 24k. |
| **Terminal-Bench 2.0 adapter** (89 hard containerized long-horizon tasks) | Snorkel's frontier agentic benchmark; the only one in the menu that tests "do real engineering work in a shell," not "fix this bug." | XL | Each task = (instruction, Dockerfile, pytest verifier, oracle script). Completion judged purely by terminal-state pytest. Frontier models < 65 %. Long wall (some tasks > 1 day). |
| **Per-benchmark Docker isolation** | All four issue-fixing benchmarks (SWE-bench, Multi-SWE-bench, Terminal-Bench, parts of Aider) require per-instance container isolation; "I tried to run it on bare host" is an instant disqualifier. | L | One shared `bench/runners/docker.go` with image-pull, cgroup limits, log capture, cleanup. |
| **Patch capture + replay** | Every benchmark above scores agents on the patch they emit, not on transcript inspection. Patches must be persisted per task per run. | M | Already partially in `eval/` from Phase 67; needs to extend to multi-language and multi-attempt (Aider). |
| **Deterministic task ordering + seedable shuffles** | Required for any cross-run statistical claim. | S | — |

#### Category 2 — Internal ToolBench

| Feature | Why expected | Complexity | Notes |
|---|---|---|---|
| **10-capability tool-contract suite** (semantic view, LSP diagnostics, rename safety, fuzzy search, call graph, dependency graph, patch apply, context minimization, incremental update, failure handling) | This is the *deterministic* source of truth — public benchmarks have variance; ToolBench should be exact-equality, < 60s wall. Mirrors the role of Aider's own polyglot harness within Aider's repo. | L | Per-capability, per-Tier-1-language test matrix (10 × 8 = 80 deterministic checks). Direct MCP calls — no LLM in the loop. |
| **Tool contract = (input fixture, MCP call, golden output)** | Standard Helix oracle pattern from v1.4 Phase 19/20 — already in place for `test/oracle/contract` and `test/oracle/scenario`. ToolBench is a domain-specific extension. | M | Reuse `test/harness/` Runner; goldens diff-reviewable. |
| **Per-language Tier-1 fixtures** (Py, TS, JS, Go, Java, C#, C++, Rust) | Required to claim 8-language coverage. | L | Eight per-capability mini-repos. C# (omnisharp or csharp-language-server) and C++ (clangd) need new LS fixtures; Java/Go/Rust/Py/TS/JS already exist from v1.4–v1.6. |
| **Ablation-aware** (each ToolBench check must run in `your_agent_full` AND `no_lsp` / `no_semantic` / `no_structured_edit`) | Without per-mode ToolBench data, the ablation matrix collapses to a single binary "did the public benchmark pass?". Per-capability per-mode data is where the *attribution* claim lives. | L | Each capability check labels which mode-bits it depends on; runner asserts expected differential. |

#### Category 3 — Ablation modes (6-mode matrix)

| Feature | Why expected | Complexity | Notes |
|---|---|---|---|
| **`baseline_plain`** mode (shell + grep + read + edit + test only) | The "no-tools" floor. Every coding-agent paper reports against an unaided agent. | M | Same base model as `your_agent_full`; tool set restricted to a fixed minimal MCP profile. |
| **`baseline_rag`** mode (grep + embeddings + chunk RAG) | The "RAG agent" comparison. SWE-bench's own original baseline scored 1.96 % with this exact shape (chunk → embed → top-k → patch). Modern peers (Cody, Continue's `@codebase`, older Cursor) all sit on this shape. Helix must beat it. | L | Needs an embeddings backend (local `bge-small-en-v1.5` via onnxruntime-go or external; both are off-tree dependencies). Chunking strategy: standard 500-token windows. |
| **`your_agent_full`** mode (semantic + LSP + fuzzy + symbol graph + structured edits + diagnostics) | The Helix product. Mode definition = current full profile. | S | Already exists — it's the production daemon. |
| **`no_lsp`** mode | Attribution: how much of the win comes from LSP-backed answers vs tree-sitter alone? | M | Disable workers in `internal/kernel/lspool/`; structural ops fall through to tree-sitter; goto-def returns `Unsupported`. |
| **`no_semantic`** mode | Attribution: how much of the win comes from RepoMap PageRank + semantic store? | M | Disable `internal/repomap/`, semantic store reads return `NoopLookup`. |
| **`no_structured_edit`** mode | Attribution: how much of the win comes from `replace_symbol_body` / `replace_in_file` / tree-sitter body surgery vs whole-file rewrites? | M | Restrict edit tools to `write_file` + `read_file`; fuzzy + symbol edits return `Unsupported`. |
| **Same-model, same-budget invariant** | The headline claim *requires* model and budget to be held constant across all 6 modes. Single-mode-changes-model results are uninterpretable. | S | Runner config enforces `model_id` and `max_tokens` are identical across the mode sweep; refuses to start otherwise. |

#### Category 4 — Metrics + statistics

| Feature | Why expected | Complexity | Notes |
|---|---|---|---|
| **12+ normalized per-task result schema** (`task_success`, `verified_correctness`, `tokens_in/out`, `tool_calls`, `wall_time_s`, `files_read`, `bytes_read`, `files_modified`, `edit_locality`, `regression_rate`, `lsp_diagnostics_used`, `semantic_tool_calls`, `edit_distance_patch`, `retry_count`) | PROJECT.md mandates this list. Without normalized per-task records, no cross-benchmark aggregation is possible. | M | One JSON-line per task per mode per run; runner-agnostic. |
| **`pass@1`, `pass@k`** (Codex/HumanEval convention: unbiased estimator `1 - C(n-c,k)/C(n,k)`) | Industry default; every coding paper reports both. | S | k=3 default with N≥3 attempts per task. |
| **Bootstrap CI** (BCa or percentile, 1000 resamples default) | Single-run results are *not publishable* per PROJECT.md. Bootstrap CI is the standard convention. | S | Reuse `gonum.org/v1/gonum/stat`. Report 95 % CI on aggregated pass@1. |
| **N ≥ 3 runs per task default** | Industry default for nondeterministic agents; SWE-bench papers typically report n=3–5. | S | Config-overridable; CI runs at n=1 for speed, nightly at n=3. |
| **`cost_per_solved_task`** ($) — reopens Phase 67 deferred item | Cost-per-task is now a standard 2026 leaderboard column (morphllm, ssojet, SWE-bench Pro all report it). Without it, the "fewer tokens" half of the headline claim is unfalsifiable. | M | Static price table per provider × model (`bench/pricing.yaml`); recompute on every run. |
| **`edit_locality`** (fraction of edits that fall inside the intended target symbol/file vs spill outside) | Direct measurement of one Helix differentiator (structured edits don't spill). | M | Diff parse → symbol-range overlap via tree-sitter. |
| **`regression_rate`** (pass_to_pass tests that flip from pass to fail) | SWE-bench already requires `pass_to_pass`; reporting it as a *separate* metric — not just as a precondition for pass@1 — exposes "looked like a solve but broke other things." | S | Already in SWE-bench harness output; surface in normalized record. |
| **Verified correctness — *augmented* tests** (UTBoost-style or equivalent) | The user explicitly called this out. SWE-bench Verified has documented 24.4 % ranking shift under UTBoost-augmented tests (Kang 2026); 31 % of "passed" patches rely on insufficient tests. Peers will ask. | XL | Either (a) consume UTBoost's published augmented test suite (preferred — already public for SWE-bench Verified and Lite), or (b) generate our own augmented tests with the runner's LLM. Report both raw and augmented pass-rates side-by-side. |
| **EM, ES, identifier-match** (for CrossCodeEval, RepoBench-C) | Standard for completion benchmarks. EM = exact match on the held-out span; ES = char-level edit similarity ratio. | S | `internal/eval/metrics/em_es.go`; pure Go. |
| **`acc@k`** (for RepoBench-R) | Standard for retrieval-eval. | S | — |

#### Category 5 — Reports

| Feature | Why expected | Complexity | Notes |
|---|---|---|---|
| **`leaderboard.md`** — top-line aggregate (mode × pass@1 with CI, $/solve, tokens, files-read) | PROJECT.md mandates. Mirrors Aider's `docs/leaderboards/edit.html` and Epoch AI's benchmark pages. | M | Auto-generated from per-run JSON. |
| **`per_language.md`** — breakdown by Tier-1 language × benchmark | PROJECT.md mandates. C# and TS will be the most-watched rows (least covered by public benchmarks). | M | — |
| **`ablations.md`** — 6-mode × benchmark matrix; this is *the* attribution doc | PROJECT.md mandates. This is the load-bearing report for the headline claim. | M | — |
| **`cost_quality.md`** — Pareto frontier ($/solve vs pass@1) | PROJECT.md mandates. Cost-quality plots are the modern way to publish coding-agent results (morphllm 2026, SSOJet 2026, Galileo 2026). | M | — |
| **JSON-line raw output** alongside Markdown | Re-aggregation, third-party analysis, internal historical tracking. | S | Already in eval/ shape. |
| **Provider TOS attestation** carried forward from Phase 67 | Required to publish externally. | S | Already done in `eval/EVAL.md`. |

#### Category 6 — Developer experience

| Feature | Why expected | Complexity | Notes |
|---|---|---|---|
| **`make bench`** — full run, all benchmarks, all modes, all langs, nightly-scale | The expected entrypoint, per peer convention. | S | — |
| **`make bench-quick`** — synthetic + ToolBench only, <5 min | Required to keep CI honest without running 40 h SWE-bench in PR. | S | Mirrors existing `make eval-quick`. |
| **`make bench-<suite>`** — single benchmark (`bench-swe`, `bench-aider`, `bench-crosscodeeval`, `bench-repobench`, `bench-multi-swe`, `bench-terminal`, `bench-toolbench`) | Required for iterating on a single adapter. | S | — |
| **Per-suite registries with shared task schema** | Adapters must report into a *single* normalized format for `leaderboard.md` to aggregate. | M | `bench/datasets/`, `bench/runners/`, `bench/evaluators/`. |
| **Hermetic Docker pulls + image cache** | Public benchmarks ship as ~500–2000 per-instance Docker images. Pulling them fresh per run is impossible (hours of network + ToS issues). Image cache + pinned digests are table-stakes. | L | SWE-bench official images: `ghcr.io/swebench/sweb.eval.x86_64.<instance_id>`. |
| **CI gate on ToolBench only** | Per existing project rule "benchmarks local-only" (Phase 50 decision, v1.9). Public benchmarks run nightly, never PR-gating. | S | Existing convention; preserve. |

---

### Differentiators (Helix-specific wins — net-new, peers don't ship)

| Feature | Value proposition | Complexity | Notes |
|---|---|---|---|
| **6-mode ablation matrix with same-model invariant** | No peer publishes a 6-mode breakdown holding model + budget constant. CodeAgent (2024) does single-tool ablations, Aider does edit-format ablations, neither holds model fixed across a *layered* tool subtractive sweep. This is the load-bearing differentiator for the headline claim. | M | The matrix itself is logic+config; the win is committing to it in writing. |
| **Per-capability ToolBench (10 × 8 Tier-1 langs)** | The deterministic ground truth that public-benchmark variance can't drown out. Mirrors the role of unit-tests in code: when SWE-bench Verified is too noisy to attribute a 1.2 % delta, ToolBench tells you with zero variance whether `rename_symbol` regressed on Java. | L | Helix is uniquely positioned to ship this because Helix already *owns* the tools being tested. |
| **`no_lsp` / `no_semantic` / `no_structured_edit` controlled subtractives** | The peer ablations in the literature (CodeAgent, HyperAgent) drop *one tool at a time*. Helix's ablation drops *whole capability layers*, which directly maps subsystem → benchmark delta. This is the headline-claim attribution mechanism. | M | (Same as the modes in Category 3 above — here counted as a *combined differentiator* because the *combination* is what's net-new.) |
| **Raw + UTBoost-augmented SWE-bench pass-rates reported side-by-side** | Pre-empts the #1 critique of any SWE-bench result in 2026: "your tests are weak." Almost no published agent has done this explicitly. | L | Consume UTBoost's published augmented suite (preferred); regenerate only if missing. |
| **Edit-locality + regression-rate as first-class metrics** | The "fewer destructive edits" half of the headline claim. No public benchmark reports these as primary metrics today — most report pass@1 and stop. | M | Tree-sitter range overlap + p2p flip count. |
| **Cost-per-solve at fixed budget, comparable across modes** | Modern leaderboards (morphllm 2026, SWE-bench Pro) report $/task, but none publish per-ablation-mode breakdowns. "$/solve falls 60 % between `baseline_rag` and `your_agent_full`" is a defensible external claim. | M | Phase 67 deferred this; v1.12 reopens. |
| **Single canonical bench/ tree separate from legacy eval/** | The Phase 67 `eval/` corpus is synthetic. Mixing it with the public-benchmark numbers would muddy the report. Clean tree = clean external story. | S | PROJECT.md already mandates `bench/datasets/`, `bench/runners/`, `bench/languages/`, `bench/evaluators/`, `bench/reports/`. |

---

### Anti-features (commonly requested but problematic)

| Feature | Why requested | Why problematic | Alternative |
|---|---|---|---|
| **HumanEval / MBPP / MultiPL-E as primary score** | "It's fast, it's famous, it has Tier-1 langs." | Toy single-function generation; doesn't exercise *any* Helix capability (no cross-file, no LSP, no editing). Reporting it as primary signals "we benchmark on the easy stuff." PROJECT.md already excludes it. | Smoke-only via MultiPL-E / HumanEval-X / McEval if needed; never primary. |
| **Comparing Helix-agent vs Claude Code / Cursor as black boxes** | "Most striking marketing claim." | Different base model, different scaffolding, different prompts → confounded. The result is uninterpretable. PROJECT.md already excludes it. | Controlled baselines using the *same* base model only (`baseline_plain`, `baseline_rag`). |
| **LLM-judge as CI gate / leaderboard ranker** | "Judges catch quality issues tests don't." | Nondeterministic; per Phase 67 EVAL-07 we already learned this the hard way. Four-layer mitigation already in place. | Judge stays informational; never CI-actionable. Report judge output in a separate file with explicit `INFORMATIONAL` banner. |
| **Submitting to public leaderboards as the primary deliverable** | "External credibility." | Leaderboard submission infra is its own deferred milestone (per PROJECT.md). Mixing it with the harness milestone explodes scope (submission validation, anonymization, repro packaging, paper-style report). | Generate locally; defer public submission to a later milestone. Publish results in our own `leaderboard.md`. |
| **Multi-attempt with unbounded retry budget** | "Maximize pass@1." | Confounds quality with budget. Aider's polyglot is 2 attempts; SWE-bench is conventionally 1; mixing budgets across modes invalidates the same-budget invariant. | Fixed per-suite attempt count from the upstream benchmark spec; never override. |
| **Tier-2 / Tier-3 languages in v1.12** | "Helix supports 52 languages, let's show that." | The story is "Helix improves Tier-1 agent work." Adding PHP/Ruby/Kotlin/Swift fixtures dilutes the per-language signal, doubles the matrix, and most public benchmarks don't cover them anyway. PROJECT.md already excludes them. | Future milestones; v1.12 explicitly Tier-1 only. |
| **Custom internal benchmark replacing public benchmarks** | "Public benchmarks are noisy / leaked / gamed." | True, but a pure-internal-benchmark result is dismissible — peers can't reproduce it. ToolBench is the deterministic *complement*, not a replacement. | Run both: public for external comparability, ToolBench for internal attribution. |
| **Reporting only `pass@1` without `pass@k`, CI, or cost** | "Cleaner leaderboard." | Modern peers (Aider, Epoch AI, vals.ai, morphllm) all report multi-axis. Single-axis pass@1 in 2026 reads as either lazy or hiding variance. | Always: pass@1, pass@k, 95% CI, $/solve, tokens/solve. |

---

## Feature Dependencies

```
Same-model, same-budget invariant
    └──requires──> 6-mode ablation matrix (baseline_plain, baseline_rag,
                                           your_agent_full, no_lsp,
                                           no_semantic, no_structured_edit)
                       └──requires──> Normalized per-task result schema
                                            └──requires──> Patch capture + replay
                                            └──requires──> Token/cost accounting
                                            └──requires──> Trace merging (Helix daemon + agent CLI)

baseline_rag mode
    └──requires──> Embedding backend (off-tree dep — onnxruntime-go + bge-small)
                                            
SWE-bench Verified adapter
    └──requires──> Per-instance Docker image cache
    └──requires──> Patch capture + replay
    └──enhanced-by──> UTBoost-augmented test suite (verified-correctness story)

Multi-SWE-bench adapter
    └──requires──> Per-instance Docker image cache (shared with SWE-bench)
    └──requires──> 7-language LS fixtures (Java/TS/JS/Go/Rust/C/C++)

Terminal-Bench 2.0 adapter
    └──requires──> Per-instance Docker image cache (heaviest — long-horizon)
    └──requires──> Wall-time budgets (some tasks expected > 1 day)

Aider Polyglot adapter
    └──requires──> 6-language Exercism task corpus (C++, Go, Java, JS, Py, Rust)
    └──requires──> 2-attempt-with-feedback runner pattern

CrossCodeEval adapter
    └──requires──> Static-analysis-validated task corpus (Py, Java, TS, C#)
    └──requires──> EM, ES, identifier-match metrics

RepoBench adapter
    └──requires──> Retrieval-task corpus (R/C/P × Py @ 12k, Java @ 24k)
    └──requires──> acc@k, EM, ES metrics

Internal ToolBench
    └──requires──> 8 × Tier-1-language fixtures (one per capability per lang)
    └──requires──> Direct-MCP-call runner (no LLM in loop)
    └──enhances──> All public-benchmark results (deterministic attribution complement)

leaderboard.md / per_language.md / ablations.md / cost_quality.md
    └──requires──> All of: SWE-bench, Multi-SWE-bench, Aider Polyglot,
                  CrossCodeEval, RepoBench, Terminal-Bench, ToolBench results
                  in normalized per-task JSON

Cost-per-solve
    └──requires──> Static pricing table per (provider × model)
    └──requires──> Token accounting in normalized record
```

### Dependency notes

- **`baseline_rag` requires an embedding backend** that doesn't currently exist in Helix. Options: `onnxruntime-go` + `bge-small-en-v1.5` (off-tree CGO dep), or a tiny external Python sidecar at bench time (per-bench, never in prod daemon). The product decision out of this milestone is: *only* used during `baseline_rag` runs, never in the daemon. Helix itself does not become a vector-search product (PROJECT.md "Out of Scope" preserved).
- **UTBoost-augmented tests** are public for SWE-bench Verified and Lite — consume, don't regenerate.
- **Docker image cache is the single biggest infra dependency.** SWE-bench (500) + Multi-SWE-bench (1,632) + Terminal-Bench (89) = 2,221 images. At ~1 GB each, that's ~2 TB; in practice ~200–400 GB after dedup. Pinned `--digest` only; no `:latest`.
- **C# coverage** comes from CrossCodeEval only. There is no public SWE-bench-style benchmark for C# as of 2026-Q2 (SWE-Sharp-Bench is announced but not at scale). ToolBench is therefore *the* C# story.
- **TS coverage** comes from CrossCodeEval + Multi-SWE-bench. (Aider Polyglot is JS, not TS.)
- **Phase 67 cost-conversion deferred item** is a hard dependency for `cost_quality.md` and the headline claim's "fewer tokens" half.

---

## MVP Definition

### Launch with (v1.12 — must ship to land the headline claim)

- [ ] `bench/` tree skeleton (`datasets/`, `runners/`, `languages/`, `evaluators/`, `reports/`) — clean break from legacy `eval/`
- [ ] **Internal ToolBench** — 10 capabilities × 8 Tier-1 langs, deterministic, < 60 s wall, runs in `make bench-quick` and `make test`
- [ ] **6-mode ablation matrix** — all six modes operational, same-model-same-budget invariant enforced
- [ ] **`baseline_plain` and `baseline_rag` modes** — both controlled baselines functional
- [ ] **Aider Polyglot adapter** — cheapest public benchmark, 6 of 8 Tier-1 langs, fastest path to first external number
- [ ] **CrossCodeEval adapter** — covers C# (the language gap in every issue-fixing benchmark) and TS
- [ ] **SWE-bench Verified adapter** — the headline external benchmark
- [ ] **Normalized per-task result schema** — JSON-line per (task, mode, run); all metrics from PROJECT.md present
- [ ] **`pass@1`, `pass@k` (k=3), 95 % bootstrap CI** — single-run results not publishable
- [ ] **`cost_per_solved_task`** — reopens Phase 67 deferral; pricing table per provider × model
- [ ] **UTBoost-augmented SWE-bench pass-rate** reported alongside raw pass-rate
- [ ] `leaderboard.md`, `per_language.md`, `ablations.md`, `cost_quality.md` — all four auto-generated from JSON
- [ ] `make bench` (full) / `make bench-quick` (ToolBench + smoke) / `make bench-<suite>` (single adapter)
- [ ] Per-instance Docker image cache with pinned digests
- [ ] CI gate on ToolBench only (existing "benchmarks local-only" rule preserved)

### Add after validation (v1.12.x or v1.13)

- [ ] **Multi-SWE-bench adapter** — 1,632 instances, 7 langs; expensive (~80 h) but covers the remaining Tier-1 gaps (Java/Go/Rust/TS/JS/C/C++) at scale
- [ ] **RepoBench adapter** — clean retrieval-only story; valuable for RepoMap attribution but RepoMap already has tests
- [ ] **Terminal-Bench 2.0 adapter** — long-horizon work; the most expensive adapter; runs nightly only
- [ ] **Public leaderboard submissions** — submission packaging, repro Docker images, anonymized identity flow
- [ ] **Continuous benchmarking dashboard** — track per-commit deltas on ToolBench + Aider Polyglot

### Future consideration (v2+)

- [ ] **Tier-2 / Tier-3 language ToolBench fixtures** (PHP, Ruby, Kotlin, Swift, C, Scala, …)
- [ ] **Custom Helix-authored benchmark** focused on semantic-tool stress tests (only after public-benchmark story is solid)
- [ ] **Cross-vendor head-to-head** (Helix-agent vs Cursor / Cody / Continue) — only with explicit-same-base-model and explicit-same-task-set guarantees, otherwise stays out of scope

---

## Feature Prioritization Matrix

| Feature | User value | Implementation cost | Priority |
|---|---|---|---|
| `bench/` tree skeleton | HIGH | LOW | **P1** |
| Internal ToolBench (10 × 8) | HIGH | MEDIUM | **P1** |
| 6-mode ablation matrix | HIGH | MEDIUM | **P1** |
| `baseline_plain` mode | HIGH | LOW | **P1** |
| `baseline_rag` mode | HIGH | MEDIUM (embedding dep) | **P1** |
| `no_lsp` / `no_semantic` / `no_structured_edit` modes | HIGH | MEDIUM | **P1** |
| Aider Polyglot adapter | HIGH | MEDIUM | **P1** |
| CrossCodeEval adapter | HIGH | MEDIUM | **P1** |
| SWE-bench Verified adapter | HIGH | HIGH (Docker, scale) | **P1** |
| Normalized per-task result schema | HIGH | LOW | **P1** |
| `pass@1` + `pass@k` + bootstrap CI | HIGH | LOW | **P1** |
| `cost_per_solved_task` | HIGH | LOW | **P1** |
| UTBoost-augmented SWE-bench rescoring | HIGH | MEDIUM | **P1** |
| Four reports (`leaderboard`, `per_language`, `ablations`, `cost_quality`) | HIGH | MEDIUM | **P1** |
| `make bench` / `bench-quick` / `bench-<suite>` | HIGH | LOW | **P1** |
| Docker image cache | HIGH | MEDIUM | **P1** |
| Multi-SWE-bench adapter | HIGH | HIGH (scale) | **P2** |
| RepoBench adapter | MEDIUM | MEDIUM | **P2** |
| Terminal-Bench 2.0 adapter | MEDIUM | HIGH (long wall) | **P2** |
| Edit-locality metric | HIGH | MEDIUM | **P1** |
| Regression-rate metric | HIGH | LOW | **P1** |
| Public leaderboard submission infra | LOW | HIGH | **P3** |
| Continuous benchmarking dashboard | MEDIUM | HIGH | **P3** |

---

## Competitor Feature Analysis

| Feature | Aider's own polyglot | SWE-bench official harness | Multi-SWE-bench | Cody / Continue / Cursor (black-box) | CodeAgent (paper) | Helix v1.12 |
|---|---|---|---|---|---|---|
| Public-benchmark adapter | 1 (their own) | 1 (SWE-bench) | 1 (Multi-SWE-bench) | unknown (proprietary) | 1 (CodeAgentBench) | **6** (Aider, CrossCodeEval, SWE-bench, Multi-SWE-bench, RepoBench, Terminal-Bench) + ToolBench |
| Languages covered | 6 (C++, Go, Java, JS, Py, Rust) | 1 (Py) | 7 (Java, TS, JS, Go, Rust, C, C++) | usually unreported | 1 (Py) | **8 Tier-1** (Py, TS, JS, Go, Java, C#, C++, Rust) |
| Ablation depth | Edit-format only | None | None | None (black-box) | One tool at a time | **6-mode layered subtractive** |
| Same-model invariant | Per row | Per submission | Per submission | Not enforced (black-box) | Yes | **Yes — runner-enforced** |
| pass@k + bootstrap CI | pass@1, pass@2, no CI | pass@1 (resolved %) | pass@1 (resolved %) | not reported | pass@1 | **pass@1, pass@3, 95 % bootstrap CI** |
| Cost per solve | Yes (per row) | Not in official report | Not in official report | Sometimes (morphllm, ssojet aggregate) | Not | **Yes (`cost_quality.md`)** |
| Verified-correctness via augmented tests | No | No (UTBoost is third-party) | No | No | No | **Yes (raw + UTBoost-augmented side-by-side)** |
| Edit-locality / regression-rate | No | p2p tests gate pass; not reported separately | p2p tests gate pass | No | No | **Yes — separate metrics** |
| Internal tool-contract suite | Implicit (their own polyglot runner) | N/A | N/A | Not public | Implicit | **Explicit — 10 capabilities × 8 langs deterministic** |
| Reports | Markdown table + cost ranking | `final_report.json` | `final_report.json` + leaderboard | Marketing pages | Paper tables | **4 markdown reports + JSON-line raw** |

The differentiator pattern: **Helix v1.12 is the *only* harness in the table that holds model fixed across a layered subtractive ablation of its own tool surface, and reports both raw and augmented-test pass-rates.** That combination is the load-bearing external claim.

---

## Sources

### Public benchmark mechanics (HIGH confidence — multiple corroborating sources per benchmark)

- [SWE-bench official Harness reference](https://www.swebench.com/SWE-bench/reference/harness/) — canonical evaluation methodology
- [SWE-bench Verified — OpenAI announcement](https://openai.com/index/introducing-swe-bench-verified/) — 500-instance human-validated subset
- [Epoch AI — How to run SWE-bench Verified in one hour on one machine](https://epoch.ai/blog/swebench-docker) — Docker harness operational notes
- [vals.ai — SWE-bench Verified leaderboard](https://www.vals.ai/benchmarks/swebench) — reporting conventions
- [BenchmarkingAgents — SWE-bench Verified Explained: 2026 Methodology, Tiers, Caveats](https://benchmarkingagents.com/swe-bench/) — 2026 methodology summary
- [Aider — Code editing leaderboard](https://aider.chat/docs/leaderboards/edit.html) — reporting convention
- [Aider — Polyglot leaderboard intro](https://aider.chat/2024/12/21/polyglot.html) — 225-task selection method (problems solved by ≤ 3 of 7 top models)
- [Epoch AI — Aider Polyglot](https://epoch.ai/benchmarks/aider-polyglot) — independent leaderboard
- [Aider — Edit formats](https://aider.chat/docs/more/edit-formats.html) — whole / diff / udiff / SEARCH-REPLACE
- [Aider — Benchmark notes](https://aider.chat/docs/leaderboards/notes.html) — operational notes
- [CrossCodeEval project page](https://crosscodeeval.github.io/) — EM, ES, identifier-match metrics
- [CrossCodeEval paper (arXiv 2310.11248)](https://arxiv.org/pdf/2310.11248) — static-analysis task validation methodology
- [RepoBench paper (arXiv 2306.03091)](https://arxiv.org/abs/2306.03091) — RepoBench-R/C/P, acc@k / EM / ES
- [RepoBench Leaderboard](https://llm-stats.com/benchmarks/repobench) — token thresholds (Python 12k, Java 24k)
- [Multi-SWE-bench GitHub](https://github.com/multi-swe-bench/multi-swe-bench) — 1,632 instances, 7 langs, `multi_swe_bench.harness.run_evaluation`
- [Multi-SWE-bench paper (arXiv 2504.02605)](https://arxiv.org/pdf/2504.02605) — methodology
- [Multi-SWE-bench HF dataset](https://huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench) — final_report.json shape
- [Terminal-Bench 2.0 — Snorkel announcement](https://snorkel.ai/blog/terminal-bench-2-0-raising-the-bar-for-ai-agent-evaluation/) — 89 tasks, 10 domains, frontier < 65 %
- [Terminal-Bench paper (arXiv 2601.11868)](https://arxiv.org/abs/2601.11868) — container methodology
- [VentureBeat — Terminal-Bench 2.0 launch + Harbor framework](https://venturebeat.com/ai/terminal-bench-2-0-launches-alongside-harbor-a-new-framework-for-testing) — frontier results

### Verified-correctness weakness (HIGH confidence — UTBoost is the canonical 2026 citation)

- [UTBoost: Rigorous Evaluation of Coding Agents on SWE-Bench (arXiv 2506.09289)](https://arxiv.org/pdf/2506.09289) — 24.4 % ranking shift, 26 instances with insufficient tests, parser-error correction
- [Daniel Kang — SWE-bench Verified is Flawed Despite Expert Review (Medium)](https://medium.com/@danieldkang/swe-bench-verified-is-flawed-despite-expert-review-utboost-exposes-gaps-in-test-coverage-4b75c6b940c6) — 31.08 % of passed patches rely on insufficient tests; 33.04 % have direct solution leaks
- [Establishing Best Practices for Building Rigorous Agentic Benchmarks (arXiv 2507.02825)](https://arxiv.org/pdf/2507.02825) — agentic benchmark methodology

### Reporting conventions (HIGH confidence)

- [How pass@k is used to evaluate LLM coding performance (Medium)](https://medium.com/@ggfincke/how-pass-k-is-used-to-evaluate-llm-coding-performance-296e5c4565bc) — unbiased estimator formulation
- [IBM — What Is HumanEval?](https://www.ibm.com/think/topics/humaneval) — pass@k as functional-correctness metric
- [Holistic Agent Leaderboard (arXiv 2510.11977)](https://arxiv.org/pdf/2510.11977) — modern multi-axis leaderboard infrastructure
- [Cost-Per-Successful-Task: A New AI Evaluation Metric — Digital Applied](https://www.digitalapplied.com/blog/cost-per-successful-task-new-ai-evaluation-metric) — cost-per-task definition
- [morphllm — Best AI Model for Coding (June 2026): SWE-bench Pro + cost per task](https://www.morphllm.com/best-ai-model-for-coding) — 2026 leaderboard conventions
- [SSOJet — 8 AI Coding Models Ranked by Cost-per-Task](https://ssojet.com/blog/cheapest-ai-coding-models) — cost-per-task reporting

### Ablation methodology (MEDIUM-HIGH confidence)

- [CodeAgent (arXiv 2401.07339)](https://arxiv.org/html/2401.07339v2) — per-tool ablation methodology; semantic-retrieval removal drops resolution 25.58 % → 19.37 %
- [HyperAgent (arXiv 2409.16299)](https://arxiv.org/pdf/2409.16299) — multi-agent ablation by role replacement
- [FeatBench (arXiv 2509.22237)](https://arxiv.org/pdf/2509.22237) — RT % (Regression Test pass rate) and FV % (Feature Validation) as primary metrics
- [Anthropic — Demystifying evals for AI agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents) — eval design principles

### Baseline RAG patterns (HIGH confidence)

- [Agentic, Semantic, or Both? Notes from the Code Search Debate (2026-05)](https://wowelec.wordpress.com/2026/05/18/agentic-semantic-or-both-notes-from-the-code-search-debate/) — 2026 grep-vs-embeddings landscape
- [AI Agents Don't Need Vector Search Anymore (Medium, 2026-05)](https://buzzgrewal.medium.com/ai-agents-dont-need-vector-search-anymore-inside-the-agentic-search-stack-replacing-rag-in-2026-58efcabe4f6f) — 2026 paradigm shift; SWE-grep, Windsurf
- [Aider — File editing problems](https://aider.chat/docs/troubleshooting/edit-errors.html) — edit-format failure modes

### Internal Helix references (HIGH confidence — read in-line)

- `/.planning/PROJECT.md` (v1.12 milestone section, lines 147–187) — milestone scope and explicit out-of-scope list
- `/eval/EVAL.md` — Phase 67 v1.10 baseline (provider TOS, eval-quick vs eval distinction, EVAL-07 judge mitigation)
- `/SPEC-DRAFT.md` (ADR-007, §391 `internal/eval/` layout, §2585 eval config, §3250 Phase 10 Evaluation Harness) — original eval-harness shape from spec draft

---
*Feature research for: agentic coding benchmark harness — v1.12 Bench Stack & Tool Evaluation*
*Researched: 2026-06-13*
