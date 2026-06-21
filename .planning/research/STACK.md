# STACK.md — v1.12 Bench Stack & Tool Evaluation (Additions)

**Project:** Helix v1.12 Bench Stack & Tool Evaluation
**Researched:** 2026-06-13
**Scope:** NEW dependencies / runtime requirements for the new `bench/` tree only. The existing v1.11 stack (Go 1.25.1, MCP SDK, koanf v2, modernc.org/sqlite, duckdb-go v2.10502.0, go-tree-sitter + 23 grammars, gRPC, Prometheus, OTel, cobra, fsnotify v1.9.0, anthropic-sdk-go v1.35.0, openai-go v1.12.0, tiktoken-go/tokenizer **already chosen for `eval/` at v1.10**, bluekeyes/go-gitdiff **already chosen for `eval/` at v1.10**, sigstore-go, gonum test-only) is fixed and not re-evaluated.

**Mandatory upstream-tool tag for every adapter row in this doc:**
- `subprocess-shellout` — run upstream's reference harness as-is via `os/exec` (lowest effort, highest fidelity)
- `dataset-loader-only` — pull dataset, run our own scorer (medium effort, full control over modes)
- `go-native-rewrite` — reimplement the harness in Go (highest effort; only if the upstream harness is unusable from outside Python or we need deep ablation hooks)

---

## TL;DR — Recommended Additions

| Component | Library / Tool | Version | CGO | Runtime needed | Confidence |
|-----------|----------------|---------|-----|----------------|-----------|
| Docker SDK (container orchestration for SWE-bench / Multi-SWE-bench / Terminal-Bench) | `github.com/docker/docker` (engine-api `client`) | v27.x | no (Unix socket / TCP) | Docker Engine ≥ 24 on the host | HIGH |
| Container test ergonomics (optional, dev-time only) | `github.com/testcontainers/testcontainers-go` | v0.36.x (Apr 2026) | no | Docker Engine | MEDIUM (only if Go-side container reuse becomes painful) |
| SWE-bench harness | upstream `swebench` PyPI | v4.x (2026) | n/a — `subprocess-shellout` | Python 3.11+, Docker Engine | HIGH |
| Multi-SWE-bench harness | upstream `multi-swe-bench` (github) | main (no PyPI release) | n/a — `subprocess-shellout` | Python 3.11+, Docker Engine | HIGH |
| Terminal-Bench 2.0 + Harbor | upstream `terminal-bench` (`tb` CLI) | 2.x (Nov 2025) | n/a — `subprocess-shellout` | Python (uv/pipx), Docker Engine | HIGH |
| Aider Polyglot dataset | `Aider-AI/polyglot-benchmark` repo | shallow git clone, pin sha | n/a — `dataset-loader-only` | per-language toolchains | HIGH |
| CrossCodeEval dataset | HF `crosscodeeval` (jsonl) | v1 | n/a — `dataset-loader-only` | none (completion-only scoring) | HIGH |
| RepoBench dataset | HF `tianyang/repobench_python_v1.1` / `_java_v1.1` | v1.1 | n/a — `dataset-loader-only` | none (completion-only scoring) | HIGH |
| HF dataset/parquet fetcher | `github.com/gomlx/go-huggingface` | v0.x | no | none | MEDIUM |
| Parquet reader (lower-level fallback) | `github.com/apache/arrow-go/v18` (already indirect in go.sum) | v18.5.1 | no | none | HIGH |
| RAG baseline — embedding provider | OpenAI `text-embedding-3-small` via existing `openai-go` v1.12.0 (primary); Ollama `nomic-embed-text` over HTTP for offline (secondary) | n/a | no | network OR local Ollama daemon | HIGH |
| RAG baseline — in-process vector store | `github.com/philippgille/chromem-go` | v0.7.x | no (zero deps) | none | HIGH |
| Bootstrap CIs, percentiles, BCa | `gonum.org/v1/gonum/stat` (promoted from test-only to runtime for bench/) + ~40 LOC BCa helper in `bench/evaluators/statx` | v0.16.x | no | none | HIGH |
| Per-language test runner shellouts | stdlib `os/exec` + per-language toolchain (already runs in CI) | n/a | no | go, python, node, java (jdk), dotnet, msvc/clang/cmake, cargo, rustc | HIGH |
| Cost table format | static YAML in `bench/datasets/cost-table.yaml`, parsed via koanf v2 (existing) | n/a | no | none | HIGH |
| Trace merging | reuse existing OTel pipeline + Phase 67's `internal/eval/trace` merger; no new lib | n/a | no | none | HIGH |
| Token counting | `github.com/tiktoken-go/tokenizer` (already in eval/) v0.6.x **+** `anthropic-sdk-go` server-side count | v0.6.0 | no | none / network for exact Claude | HIGH |
| Patch apply (SWE-bench-style prediction patches) | `github.com/bluekeyes/go-gitdiff` (already in eval/) v0.8.x | v0.8.0 | no | none | HIGH |

**Runtime posture impact (single most important issue):**
v1.12 introduces three **non-Go runtime dependencies on the bench host**: Python 3.11+ (for upstream SWE-bench / Multi-SWE-bench / Terminal-Bench harnesses), Docker Engine (for those same harnesses' per-instance environment images), and per-language toolchains (Go/Python/Node/JDK/dotnet/clang+cmake/cargo) for ToolBench test execution. None of these enter the `helix` shipping binary — they are **operator-side** requirements for the `helix-bench` binary's `run` subcommand. This must be called out in `bench/BENCH.md` the way `eval/EVAL.md` calls out ZDR. The single-binary distribution rule of the `helix` daemon is **not** violated; the bench harness is an out-of-band benchmarking tool.

---

## 1. Container Orchestration (SWE-bench, Multi-SWE-bench, Terminal-Bench 2.0)

### Recommended: subprocess-shellout to upstream Python harnesses + Docker Engine via `docker/docker` client

**Why subprocess-shellout, not Go-native rewrite:**

- SWE-bench's evaluation harness is its own quality gate — it ships a 3-layer Docker image hierarchy (base → environment → instance) covering ~60 Python repo environments with exact dep pins. Reimplementing this in Go would be a multi-month project that reproduces, but does not improve, the upstream behavior. Worse, results would not be comparable to other leaderboard entries unless we bit-for-bit reproduce the upstream's harness behavior — which is itself the canonical definition.
- The upstream harness is invoked as `python -m swebench.harness.run_evaluation --dataset_name ... --predictions_path ... --run_id ...`. This is well-defined and stable.
- Multi-SWE-bench uses essentially the same pattern: `python -m multi_swe_bench.harness.run_evaluation --config <config.json>` and produces `final_report.json`. It is a Multi-SWE-bench-org fork of the SWE-bench harness with Java/TS/JS/Go/Rust/C/C++ environment images.
- Terminal-Bench 2.0 ships its own `tb` CLI (`uv tool install terminal-bench` per upstream) and the new Harbor container-test framework. Same shellout pattern.

**Helix's role in the loop:** the agent (Helix-enabled or baseline) produces `predictions.jsonl` (one prediction per task, where each prediction is a unified diff against the instance's base commit). The bench harness then shells out to the upstream Python harness, which spins the per-instance Docker container, applies the predicted patch, runs the test patch, and produces a results JSON. Helix's contribution is on the **prediction generation** side — and that is where the 6-mode ablation matrix lives.

**Docker SDK choice:** for our own container ops (Terminal-Bench task spawn, ToolBench per-language test isolation if we choose container isolation, bench-side cleanup), use the official Docker engine API client `github.com/docker/docker/client`. Tagged release v27.x corresponds to Docker 27.x. This is the same client used by goreleaser internally and is the canonical Go SDK. **Do not** pull testcontainers-go into the runtime path — its strength is dev-time test ergonomics with reapers, networks, and waitstrategies; for our hands-on container orchestration it adds abstraction we don't need. Keep it as a **test-time-only** dependency if at all (e.g., to spin a per-test postgres for harness-of-the-harness unit tests).

**ToolBench per-language isolation:**
- **Default:** no container — run the per-language test runner directly in a temp dir. Same model as Phase 67's sandbox isolation (`internal/eval/sandbox`).
- **Optional `--container-isolate`:** wrap each (task × mode) in a thin Docker container using a curated per-language base image. Only flip on for hostile-task corpora; default off for speed.

### Alternatives Considered

| Alternative | Why Not |
|-------------|---------|
| Go-native SWE-bench harness | 6+ months of work to reproduce environment images, would not be comparable to upstream-published numbers, and locks us into maintaining 60+ environment fingerprints forever. |
| `testcontainers-go` in runtime path | Adds ryuk reaper, network-attach magic, waitstrategy abstractions we don't need for batch runs. Strictly more code in prod hot path. |
| podman / containerd direct | Most upstream harnesses assume `docker` socket; mismatched container runtime is a foot-gun. Operators wanting podman can use the podman-docker shim. |
| `nerdctl` | Same as above. |
| `swebench` PyPI 2.0.2 (pinned) | Older releases (`pip install swebench==2.0.2`) work but lack post-Verified curation/harness improvements; pin to current v4.x stable. |

---

## 2. Public Benchmark Adapters — Per-Benchmark Disposition

| Benchmark | Tasks | Languages | Disposition | Helix's adapter does |
|-----------|-------|-----------|-------------|----------------------|
| **SWE-bench Verified** | 500 | Python | `subprocess-shellout` | (a) load HF `SWE-bench/SWE-bench_Verified` via `gomlx/go-huggingface` parquet reader; (b) drive Helix-enabled or baseline agent to produce `predictions.jsonl`; (c) shell out to upstream `python -m swebench.harness.run_evaluation`; (d) ingest `<run_id>.json` results into bench result schema. |
| **Multi-SWE-bench** | 1,632 (full) / 400 (mini) | Java, TS, JS, Go, Rust, C, C++ (full); +Python in mini | `subprocess-shellout` | Same loop as SWE-bench but with `python -m multi_swe_bench.harness.run_evaluation --config <config.json>`. Per-language ablation slicing built into our reporter, not the upstream harness. |
| **Terminal-Bench 2.0** | 89 | shell / polyglot | `subprocess-shellout` | Drive agent through Harbor's `tb run` CLI, which spawns containers and applies the agent's terminal commands. Score from `tb`'s emitted JSON. |
| **Aider Polyglot** | 225 | C++, Go, Java, JS, Python, Rust | `dataset-loader-only` | Shallow clone `Aider-AI/polyglot-benchmark` at a pinned sha, treat each Exercism task as a (problem.md, stub source, hidden test) triple. Agent produces edits; we run the per-language test command ourselves (Go: `go test`, Python: `pytest`, etc.). 2-attempt protocol baked into our runner (re-prompt with test stderr on fail). |
| **CrossCodeEval** | ~10k examples (filtered) | Python, Java, TS, C# | `dataset-loader-only` | JSONL completion task. Score is **EM** + **edit similarity** + **identifier match** (per CCE paper). No test execution required — this is line-completion, not patch-apply. Adapter purely fetches dataset and runs our own scorer. |
| **RepoBench** | 1,075 Python + 594 Java test instances | Python, Java | `dataset-loader-only` | Three sub-tasks (RepoBench-R retrieval / RepoBench-C completion / RepoBench-P pipeline). EM + edit-similarity scoring, no test execution. Load via HF `tianyang/repobench_python_v1.1` and `_java_v1.1`. |
| **MultiPL-E / HumanEval-X / McEval** (smoke only) | varies | many | `dataset-loader-only` | Per PROJECT.md "Out of scope for v1.12 primary scoring" — kept as health-check smoke only. |

**Practical effort budget the roadmapper should plan around:**

- `subprocess-shellout` adapter: ≈ 1 wave (1–2 weeks) per benchmark. Mostly dataset wiring, predictions.jsonl shaping, results ingestion, error handling on upstream's Docker layer.
- `dataset-loader-only` adapter: ≈ 1–2 waves per benchmark. The work is in the per-task **runner** (must invoke real per-language test commands), not in the loader. Aider Polyglot is the biggest because of 6-language test runners.

---

## 3. Embedding + Vector RAG (the `baseline_rag` Mode)

The `baseline_rag` ablation is essential — it answers "does Helix beat a competent grep + embedding-RAG baseline, not just a grep-only baseline?" Without it, every "we win" claim is suspect. This means the bench harness must **ship a real RAG baseline** that we are comfortable shipping with the binary.

### Recommended primary: OpenAI `text-embedding-3-small` via existing `openai-go` v1.12.0

**Why:**
- `openai-go` is already in `go.mod`. No new dep, no new auth surface area.
- `text-embedding-3-small` (1536-dim, $0.02/1M tokens as of 2026) is the cost-effective default and is what every "RAG baseline" in the literature uses.
- Real, comparable to what an agent integrator would actually wire up.

### Recommended offline fallback: Ollama `nomic-embed-text` via stdlib `net/http`

**Why:**
- No new dep — Ollama exposes `/api/embed`; a 30-LOC client in `bench/runners/embed/ollama.go` suffices.
- Lets the bench harness run airgapped (operator must have Ollama installed, but that's a documented prereq, not a Helix install requirement).
- 768-dim, runs on CPU, "good enough" embedding for RAG-baseline purposes.

### Recommended in-process vector store: `philippgille/chromem-go` v0.7.x

**Why:**
- Embeddable, **zero third-party dependencies** (matches our "be careful what enters go.mod" discipline).
- In-memory with optional persistence — exactly the shape we want (per-bench-run index, throwaway).
- Chroma-like API surface makes it familiar.
- No CGO. Single binary preserved.

### Alternatives Considered

| Alternative | Why Not |
|-------------|---------|
| FAISS / `bleve` semantic vectors | bleve is already in go.sum (semantic store v1.10). Reusing it for the **RAG baseline** would let our `baseline_rag` mode read our own production indexes — that taints the comparison. The whole point of `baseline_rag` is to be a *naive* baseline. Keep it separate. |
| `weaviate-go-client` | Weaviate is not embeddable in Go; would require operator to stand up a Weaviate server. Inflates the bench host requirement; we already have Docker as a hard prereq from SWE-bench. |
| `milvus` / pinecone | External vector DB; same problem. |
| Pure-Go `all-MiniLM-L6-v2` via `clems4ever/all-minilm-l6-v2-go` | Tempting (no network, no Ollama). But the project is single-maintainer, pre-1.0, and embedding quality differs from the standard baselines. Operators wanting fully offline can use Ollama. |
| `gomlx/onnx-gomlx` + sentence-transformers ONNX | Promising but immature for production embeddings as of 2026; reopen path captured for v1.13+. |

**Decision:** OpenAI primary, Ollama secondary, chromem-go as the vector store. RAG mode docs note the precise embedding model + index params so the baseline is reproducible.

---

## 4. Statistics — Bootstrap CIs, pass@k, BCa

### Recommended: `gonum.org/v1/gonum/stat` + ~40 LOC BCa helper

**Promotion from test-only to runtime:**
v1.10 STACK explicitly scoped gonum to test-only oracle use. For v1.12, the **bench harness** (a new binary, `cmd/helix-bench`) needs `gonum/stat` in runtime for percentile, mean, variance, and as the base layer for bootstrap. This does **not** affect the main `helix` daemon binary — `cmd/helix` does not import `cmd/helix-bench`'s packages. The gonum-not-in-prod rule (ADR-005 from v1.10) was about the kernel/semantic graph hot path; the bench evaluator is not a hot path and is a separate binary.

**What we hand-roll on top:**
- **Bootstrap percentile CI**: ~20 LOC over `math/rand/v2` + `gonum/stat.Quantile`.
- **BCa (bias-corrected accelerated)**: ~40 LOC. Needed for skewed metrics (cost-per-solved, edit-distance) where percentile CI is biased.
- **pass@k**: closed-form from `(c, n, k)` per the HumanEval paper: `pass@k = 1 - C(n-c, k)/C(n, k)`. ~10 LOC.
- **N≥3 per task** is enforced in the matrix runner, not the stat library.

### Alternatives Considered

| Alternative | Why Not |
|-------------|---------|
| Pure stdlib | Would have to reimplement quantile, mean-variance one-pass, etc. Gonum is the canonical Go scientific lib for this; the cost of bringing it in is ~10 MB of go.sum noise, no runtime weight. |
| Port Python `scipy.stats.bootstrap` | Translation toil with no upside. |
| `aclements/go-moremath/stats` | Excellent for benchmark stats (it backs `benchstat`), but its bootstrap surface is narrow — UTests, not arbitrary metric resamples. Use gonum. |

---

## 5. Per-Language Test Runners (ToolBench, 8 Tier-1 languages)

ToolBench is the deterministic core. Each language needs a real test invocation per capability test. The bench runner shells out via stdlib `os/exec` — **no per-language Go bindings**.

| Language | Test invocation | Toolchain prereq | Helix's adapter detects via |
|----------|----------------|-----------------|------------------------------|
| Go | `go test ./...` with `-json` | go 1.25+ | `go.mod` |
| Python | `pytest -q --json-report` (`pytest-json-report`) | python 3.11+, pip | `pyproject.toml` / `setup.py` / `requirements*.txt` |
| TypeScript | `npx jest --json` or `npx vitest --reporter=json` | node 20+, npm/pnpm | `package.json` + `tsconfig.json` |
| JavaScript | `npx jest --json` or `npx mocha --reporter json` | node 20+, npm/pnpm | `package.json` (no tsconfig) |
| Java | `mvn -q test -Dsurefire.useFile=false` or `gradle test --console=plain` (parse Surefire XML) | JDK 17+, Maven 3.9+ or Gradle 8+ | `pom.xml` / `build.gradle` |
| C# | `dotnet test --logger "trx;LogFileName=test-results.trx"` | .NET 8 SDK | `*.csproj` / `*.sln` |
| C++ | `cmake -S . -B build && cmake --build build && ctest --output-on-failure -T Test` (parse CTest XML) | CMake 3.25+, clang/gcc/msvc | `CMakeLists.txt` |
| Rust | `cargo test --message-format=json` | rustc 1.80+, cargo | `Cargo.toml` |

**Pattern:** one Go adapter per language in `bench/languages/<lang>/runner.go`, each implementing a `LanguageRunner` interface:

```go
type LanguageRunner interface {
    Detect(repoRoot string) bool
    Setup(ctx context.Context, repoRoot string) error      // install deps, e.g. `go mod download`
    RunTests(ctx context.Context, repoRoot string) (TestResult, error)
    Capabilities() []CapabilityKind                         // which ToolBench capabilities this language supports
}
```

**Output normalization:** each adapter parses its native runner's JSON/XML into a common `TestResult{Passed, Failed, Skipped, Errors, RawJSON}`. The bench harness's scorer is language-agnostic.

**No new Go deps required.** All toolchains are operator-side prereqs documented in `bench/BENCH.md`. The `helix setup` command's language-detection plumbing (`internal/cli/setup_detect.go`) can be reused for detection — same extension-scan logic.

---

## 6. HuggingFace Dataset Loading

### Recommended: `github.com/gomlx/go-huggingface` v0.x for the top-level fetch + iterate API; fall back to `apache/arrow-go/v18` (already in go.sum) for raw parquet.

**Why:**
- `go-huggingface` provides `IterParquetFromDataset` and handles HF's `refs/convert/parquet` branch resolution. Saves us from re-implementing HF's URL convention.
- Pure Go, no CGO. Single binary preserved.
- For datasets that don't fit the parquet convention (rare — most v1.12 benchmarks publish parquet), fall back to direct `http.Get` of the raw JSONL on the HF CDN, or DuckDB's HF integration (we already have DuckDB linked).

**Caching:** stash downloaded datasets under `$HELIX_CACHE_DIR/bench-datasets/<hf-repo>/<sha>/` so repeat runs are offline. xxhash of dataset content for verification.

### Alternatives Considered

| Alternative | Why Not |
|-------------|---------|
| `huggingface-hub` (Python) shellout | We already have Python as a prereq for SWE-bench. Adding it for dataset download would mean coordinating two Python virtualenvs (upstream harness's vs ours). Cleaner to keep Go-native fetching. |
| DuckDB `read_parquet('hf://...')` | DuckDB does support HF URIs as of 1.x, but pulling it into the bench runner for *download* (it's already loaded for semantic store, but in a different binary) doubles the surface area we'd test. Use go-huggingface. |
| Hand-rolled HF API client | The HF "list parquet files" + "convert dataset to parquet branch" logic is non-trivial. go-huggingface already wraps it. |

---

## 7. Cost Table

### Recommended: static YAML in `bench/datasets/cost-table.yaml`, parsed via koanf v2

**Why:**
- koanf is the established config lib for Helix (4-layer precedence already).
- A static price table doesn't need provider-API discovery — pricing pages move slowly enough that updating a YAML on release is fine.

**Format:**

```yaml
# bench/datasets/cost-table.yaml
# Last updated: 2026-06-13 — verify against provider pricing pages quarterly.
providers:
  anthropic:
    models:
      claude-opus-4-7: { input_per_mtok: 15.00, output_per_mtok: 75.00, currency: USD }
      claude-sonnet-4-5: { input_per_mtok: 3.00, output_per_mtok: 15.00, currency: USD }
  openai:
    models:
      gpt-5: { input_per_mtok: 1.25, output_per_mtok: 10.00, currency: USD }
      gpt-5-mini: { input_per_mtok: 0.25, output_per_mtok: 2.00, currency: USD }
      o3: { input_per_mtok: 15.00, output_per_mtok: 60.00, currency: USD }
  deepseek:
    models:
      deepseek-chat: { input_per_mtok: 0.27, output_per_mtok: 1.10, currency: USD }
      deepseek-reasoner: { input_per_mtok: 0.55, output_per_mtok: 2.19, currency: USD }
last_verified: "2026-06-13"
```

**Cost computation:** `cost_per_solved_task = sum(model.input_tokens * provider.input_per_mtok / 1e6) + ... / count(solved_tasks)`. Token totals come from the existing eval harness's token-counting layer (tiktoken + Anthropic API).

---

## 8. Trace Merging

**Recommended:** reuse existing OTel pipeline + Phase 67's `internal/eval/trace` merger. No new dep.

The Helix daemon already emits OTel traces via otelgrpc/otlptrace (v1.43.0). Phase 67 added a tap that merges Claude CLI's tool-call trace with the daemon's tool-call span graph (via the forwarder→daemon trace continuity wired at v1.10 Phase 58 REL-06). Bench's per-(task,mode) runs ride the same rails — they get a unique `bench.run_id` span attribute injected at the matrix-runner boundary, then the existing exporter handles the rest.

---

## 9. Token Counting & Patch Apply — Already Decided

These were already evaluated and chosen at v1.10 (Phase 67):

- `github.com/tiktoken-go/tokenizer` v0.6.x — pure Go, embedded vocab, no network on first use.
- `github.com/anthropics/anthropic-sdk-go` v1.35.0 — server-side `messages.CountTokens` for exact Claude counts.
- `github.com/bluekeyes/go-gitdiff` v0.8.x — pure Go, parses + applies git-style and unified diffs.

**v1.12 reuses all three as-is.** The bench harness imports them from `internal/eval/...` via shared sub-packages OR (if we want strict separation) copies the usage pattern. The `legacy eval/ tree stays as v1.10` rule from the milestone description means bench's `internal/bench/patch` and `internal/bench/tokens` will be **separate packages** that happen to depend on the same external libs.

---

## 10. CGO Posture & Distribution Rule

**Recap of constraints from CLAUDE.md / EMBED-AUDIT.md:**
- Source tree is single-mode `CGO_ENABLED=1` (Phase 59.1 removed the CGO=0 stub apparatus).
- The `helix` daemon binary ships from `cmd/helix` — that single binary is the distribution unit.
- `cmd/helix-eval` (Phase 67) is built but not part of the user-facing release archive.
- `cmd/helix-bench` (v1.12, **new**) is in the same camp as `cmd/helix-eval` — built from the same module, but **not shipped in the goreleaser archives**. It is operator-side tooling.

**v1.12 specifically:**
- No new CGO dependency. duckdb is the only CGO dep and bench may or may not need it (probably not — the bench database, if any, is small enough for modernc.org/sqlite, which we already have).
- chromem-go is zero-deps, pure Go, no CGO.
- go-huggingface, gonum, docker/docker client — all pure Go.
- All operator-side runtime prereqs (Python, Docker Engine, per-language toolchains) are **out of the binary**.

**Embed audit impact:** chromem-go's vocab/index format is computed at runtime, not embedded. No new `embed.FS` declarations. `EMBED-AUDIT.md` does not need a new row for v1.12 bench code unless we embed the cost table (we should — it's static and small) — add one row `bench/datasets/cost-table.yaml → embedded into helix-bench`.

---

## 11. Installation / go.mod Diff (Projected)

```diff
require (
+   github.com/docker/docker v27.4.0
+   github.com/gomlx/go-huggingface v0.5.0   // or current stable
+   github.com/philippgille/chromem-go v0.7.0
+   gonum.org/v1/gonum v0.16.0                // promoted from test-only to runtime (bench-side)

    // already present, reused:
    github.com/tiktoken-go/tokenizer v0.6.0      // (added in v1.10 for eval)
    github.com/bluekeyes/go-gitdiff v0.8.0       // (added in v1.10 for eval)
    github.com/anthropics/anthropic-sdk-go v1.35.0
    github.com/openai/openai-go v1.12.0
    github.com/knadh/koanf/v2 v2.3.4
    github.com/spf13/cobra v1.10.2
    github.com/cespare/xxhash/v2 v2.3.0
)

require (
    // test-only additions: none — testcontainers-go intentionally omitted.
)
```

**Binary size impact:** docker/docker client pulls a non-trivial dep tree (containerd protos, etc.) — ~8 MB additional compiled. Acceptable for a benchmarking binary; would be a red flag for `helix` itself.

---

## 12. New Binary Surface: `cmd/helix-bench`

Lives alongside `cmd/helix-eval`. Shape (informed by `helix-eval/main.go`):

```
helix-bench
├── run                    — execute the matrix (--benchmarks, --modes, --languages, --tasks)
├── fetch-datasets         — pre-populate $HELIX_CACHE_DIR/bench-datasets
├── doctor                 — verify operator prereqs (python, docker, per-lang toolchains, embedding key)
├── report                 — re-render reports from a previous run-id
└── validate-cost-table    — sanity-check the embedded cost YAML
```

cobra v1.10 — same plumbing as the daemon.

---

## What NOT to Pull In

| Library | Why we don't want it |
|---------|----------------------|
| `testcontainers-go` (runtime) | Adds reaper/network/waitstrategy abstractions we don't need; use Docker engine API directly. |
| Weaviate / Milvus / Pinecone clients | Bench host must not require a long-lived vector DB; chromem-go is the right shape. |
| FAISS Go bindings | CGO + native libs; chromem-go covers our needs. |
| Custom Python bridge (gopy, go-python) | "Out of scope: native Go, no Python interop" — Python stays at the subprocess boundary. |
| `sourcegraph/go-diff` | Already rejected at v1.10 — parser only. |
| `pkoukk/tiktoken-go` | Already rejected at v1.10 — downloads vocab. |
| Replacement for sigstore (cosign) | Bench binary signing rides existing release pipeline. |
| A second OTel tracer | Reuse `internal/obs/`. |
| `cobra/viper` (viper specifically) | We use koanf. |
| Reimplemented SWE-bench harness | 6 months for zero comparability gain. |

---

## Confidence Assessment

| Claim | Confidence | Source |
|-------|------------|--------|
| SWE-bench Verified = 500 tasks, Python only, golden-patch + test-patch oracle, requires Docker | HIGH | Upstream SWE-bench docs + HF dataset card |
| Multi-SWE-bench = 1,632 instances × 7 langs (Java/TS/JS/Go/Rust/C/C++), Bytedance Seed, has `python -m multi_swe_bench.harness.run_evaluation` | HIGH | Multi-SWE-bench paper + GitHub repo + HF dataset card |
| Aider Polyglot = 225 Exercism tasks × 6 langs, 2-attempt protocol, dataset is a single git repo | HIGH | aider.chat blog + Aider-AI/polyglot-benchmark repo |
| CrossCodeEval = Python/Java/TS/C#, completion-only, no test execution, scored via EM + identifier match | HIGH | CCE NeurIPS 2023 paper + project site |
| RepoBench = Python + Java, 1,075 + 594 test instances, three sub-tasks (R/C/P), completion-only | HIGH | RepoBench paper + Leolty/repobench README + HF datasets |
| Terminal-Bench 2.0 = 89 containerized tasks, ships `tb` CLI + Harbor, Nov 2025 release | HIGH | Terminal-Bench docs + VentureBeat coverage |
| `docker/docker` engine-API client is the right Go SDK | HIGH | Canonical, used by goreleaser, k8s, etc. |
| `chromem-go` is zero-deps embeddable vector DB suitable for `baseline_rag` | HIGH | Project README + pkg.go.dev |
| `gomlx/go-huggingface` covers parquet iteration of HF datasets | MEDIUM | Project README; specific API stability not field-tested in our repo yet |
| `gonum/stat` is the right base for bootstrap + BCa, BCa needs ~40 LOC on top | HIGH | gonum docs + BCa is a well-defined algorithm |
| OpenAI text-embedding-3-small or Ollama nomic-embed-text are the right embedding choices for `baseline_rag` | HIGH | Industry-standard baselines; cited in every RAG paper of last 24 months |
| Per-language test runners can stay as `os/exec` shellouts without per-language Go bindings | HIGH | Pattern works in CI today across 8+ languages; no upside to native bindings |
| Helix's single-binary distribution rule survives v1.12 | HIGH | helix-bench is not in the goreleaser archive matrix |

---

## Sources

- [SWE-bench docs — Docker Setup](https://www.swebench.com/SWE-bench/guides/docker_setup/)
- [SWE-bench docs — Evaluation Harness](https://www.swebench.com/SWE-bench/guides/evaluation/)
- [swe-bench/SWE-bench GitHub](https://github.com/swe-bench/SWE-bench)
- [HF dataset: SWE-bench/SWE-bench_Verified](https://huggingface.co/datasets/SWE-bench/SWE-bench_Verified)
- [Multi-SWE-bench GitHub](https://github.com/multi-swe-bench/multi-swe-bench)
- [HF dataset: ByteDance-Seed/Multi-SWE-bench](https://huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench)
- [HF dataset: ByteDance-Seed/Multi-SWE-bench_mini](https://huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench_mini)
- [Multi-SWE-bench paper (arxiv 2504.02605)](https://arxiv.org/pdf/2504.02605)
- [Aider-AI/polyglot-benchmark GitHub](https://github.com/Aider-AI/polyglot-benchmark)
- [Aider Polyglot launch post](https://aider.chat/2024/12/21/polyglot.html)
- [Aider Leaderboards](https://aider.chat/docs/leaderboards/)
- [CrossCodeEval project site](https://crosscodeeval.github.io/)
- [CrossCodeEval NeurIPS 2023 paper](https://proceedings.neurips.cc/paper_files/paper/2023/file/920f2dced7d32ab2ba2f1970bc306af6-Paper-Datasets_and_Benchmarks.pdf)
- [Leolty/repobench GitHub](https://github.com/Leolty/repobench)
- [RepoBench paper (arxiv 2306.03091)](https://arxiv.org/pdf/2306.03091)
- [Terminal-Bench 2.0 announcement (VentureBeat)](https://venturebeat.com/ai/terminal-bench-2-0-launches-alongside-harbor-a-new-framework-for-testing)
- [Terminal-Bench paper](https://arxiv.org/html/2601.11868v1)
- [testcontainers-go GitHub releases](https://github.com/testcontainers/testcontainers-go/releases)
- [philippgille/chromem-go GitHub](https://github.com/philippgille/chromem-go)
- [gomlx/go-huggingface GitHub](https://github.com/gomlx/go-huggingface)
- [gonum.org/v1/gonum/stat docs](https://pkg.go.dev/gonum.org/v1/gonum/stat)
- [tiktoken-go/tokenizer](https://github.com/tiktoken-go/tokenizer)
- [bluekeyes/go-gitdiff](https://github.com/bluekeyes/go-gitdiff)
