# Architecture: v1.12 Bench Stack & Tool Evaluation

**Milestone:** v1.12 (subsequent milestone, builds on v1.10 Phase 67 `eval/` harness)
**Researched:** 2026-06-13
**Scope:** Net-new `bench/` tree, `cmd/helix-bench` binary, 6-mode ablation matrix, 8 Tier-1 ToolBench fixtures, container/sandbox runtime, baseline_rag MCP shim, multi-run + bootstrap CI aggregation, integration with existing 4-layer Helix architecture and v1.10 `eval/` harness.

## Executive Verdict (the four open questions)

1. **`cmd/helix-bench` is a NEW sibling binary to `cmd/helix-eval`.** Do not rewrite Phase 67. The two systems answer different questions, have different SLOs, and have different downstream consumers.
2. **`eval/` is left intact as a v1.10 artifact.** `bench/datasets/internal-toolbench/` is a CLEAN-SLATE rebuild that supersedes `eval/corpus/` semantically; `eval/corpus/` does not migrate.
3. **Container runtime is host Docker via `os/exec` shell-out + a thin `bench/runtime/container/` Go package**, not an embedded Go container library. Terminal-Bench and SWE-bench Verified both ship official Docker images; matching the public harness is required to keep our numbers comparable.
4. **`baseline_rag_agent` lives at `bench/runners/baseline_rag_agent/` as its own stdio MCP server binary (`cmd/helix-bench-rag/`)** — a sibling to `helix daemon`, NOT a Helix profile. It exports a fixed 4-tool surface (`rag_search`, `rag_read_chunk`, `grep`, `read_file`) backed by an on-disk embedding index. The agent (`claude`) connects to it via `claude mcp add-json`. This keeps the RAG control completely independent of Helix's tool registry and prevents accidental contamination of measurements.

## System Diagram (data flow)

```
                                  bench/datasets/
                                  ┌──────────────────────────────┐
                                  │ swebench/   multi-swebench/  │
                                  │ aider-polyglot/  crosscodeeval/
                                  │ repobench/  terminal-bench/  │
                                  │ internal-toolbench/  ◄── source-of-truth tier
                                  └──────────┬───────────────────┘
                                             │ (task spec JSON)
                                             ▼
                                  ┌──────────────────────────────┐
                                  │ cmd/helix-bench (orchestrator)
                                  │  - matrix expander           │
                                  │  - work-stealing scheduler   │
                                  │  - container pool            │
                                  │  - result-cache keyed by     │
                                  │    (task, mode, model, seed) │
                                  └──────────┬───────────────────┘
                                             │ fan-out: N (modes) × M (runs) × T (tasks)
                                             ▼
                       ┌─────────────────────┴────────────────────┐
                       │                                          │
                       ▼                                          ▼
              ┌────────────────────┐                    ┌────────────────────┐
              │ bench/runtime/     │                    │ bench/runtime/     │
              │ container/         │ ──spawns──►        │ subprocess/        │
              │  - docker run      │                    │  - bench-mode      │
              │  - rootless podman │                    │    runners         │
              └────────┬───────────┘                    └────────┬───────────┘
                       │                                         │
                       ▼ (one container per task × mode × run)   │
              ┌────────────────────────────────────────────────────┐
              │  PER-TASK SANDBOX (reused sandbox.Sandbox API)     │
              │  ┌─────────────────────────┐   ┌────────────────┐  │
              │  │ MCP server subprocess   │   │ agent          │  │
              │  │ (mode-specific):        │◄──┤ subprocess     │  │
              │  │   your_agent_*  →       │   │ (claude CLI)   │  │
              │  │     helix daemon        │   │                │  │
              │  │     --profile=<mode>    │   └───────┬────────┘  │
              │  │   baseline_plain →      │           │           │
              │  │     none / shell only   │           │ tools/call│
              │  │   baseline_rag →        │◄──────────┘           │
              │  │     helix-bench-rag     │   (stdio JSON-RPC)    │
              │  └─────────┬───────────────┘                       │
              │            │ daemon.log     claude.stdout/jsonl    │
              │            ▼                ▼                      │
              │       trace.TapDaemonLog  trace.TapCCStream        │
              │            (reused from internal/eval/trace/)      │
              └────────┬────────────────────────────────────────┬──┘
                       │                                        │
                       ▼ merged TaskResult v2                   │
              ┌────────────────────┐                            │
              │ bench/evaluators/  │                            │
              │  test_runner/      │ (per-language: go test,    │
              │  patch_validator/  │   pytest, mvn, dotnet,     │
              │  semantic_oracle/  │   cargo, ctest, jest)      │
              │  token_meter/      │                            │
              │  tool_trace_analyzer/                           │
              │  regression_checker/                            │
              └────────┬───────────┘                            │
                       │                                        │
                       ▼                                        │
              ┌────────────────────┐                            │
              │ result.v2.json     │ ◄──────────────────────────┘
              │ (per-task,         │
              │  per-mode,         │
              │  per-run)          │
              └────────┬───────────┘
                       │ N≥3 runs per (task, mode)
                       ▼
              ┌────────────────────────────┐
              │ bench/aggregator/          │
              │  - bootstrap CI (B=10000)  │
              │  - pass@k                  │
              │  - cost_per_solved_task    │
              │  - per-language rollup     │
              └────────┬───────────────────┘
                       │
                       ▼
              ┌────────────────────────────┐
              │ bench/reports/             │
              │   leaderboard.md           │
              │   per_language.md          │
              │   ablations.md             │
              │   cost_quality.md          │
              └────────────────────────────┘
```

## The Proposed `bench/` Tree (canonical layout)

```
bench/
  datasets/                    # task corpora — read-mostly, large
    swebench/                  # 500 Python tasks (Verified split)
      tasks.jsonl              # canonical pull from upstream
      images/                  # per-task Docker image refs
      adapter.go               # SWE-bench official-harness shim
    multi-swebench/            # 1,632 instances × 7 langs
      tasks.jsonl
      adapter.go
    aider-polyglot/            # 225 Exercism tasks × 6 langs
      tasks/
      adapter.go
    crosscodeeval/             # cross-file completion × 4 langs
    repobench/                 # retrieval + completion × 2 langs
    internal-toolbench/        # ◄── NEW source-of-truth tier
      go/                      # 10 capabilities × N fixtures
      python/
      typescript/
      java/
      csharp/
      cpp/
      rust/
      javascript/
      capabilities.yaml        # 10-capability schema
    terminal-bench/            # 89 containerized tasks
  runners/                     # one subdir per ablation MODE
    baseline_plain_agent/      # shell + grep + read + edit + test
      runner.go                # implements bench.ModeRunner
      tools.yaml               # exposed tool surface (NONE from helix)
    baseline_rag_agent/        # grep + embeddings + chunk RAG
      runner.go                # spawns cmd/helix-bench-rag MCP server
      index/                   # embedding-index builder + cache
      mcp_server/              # stdio MCP shim (the 4 RAG tools)
    your_agent_full/           # all helix tools + admin mode
      runner.go                # spawns helix daemon --profile=bench-full
    your_agent_no_lsp/         # strip LSP-backed tools
      runner.go                # spawns helix daemon --profile=bench-no-lsp
    your_agent_no_semantic/    # strip semantic-index tools
      runner.go                # spawns helix daemon --profile=bench-no-semantic
    your_agent_no_structured_edit/
      runner.go                # spawns helix daemon --profile=bench-no-structured-edit
  languages/                   # per-language CONTRACT tests + shared rigging
    go/
      contract_test.go         # 10-capability assertions
      testrunner.go            # `go test ./...` invocation + parse
      tools_under_test.yaml    # which helix tools cover which capability
    python/
      contract_test.go
      testrunner.go            # `pytest --json-report` invocation + parse
      tools_under_test.yaml
    typescript/                # `npm test` / `vitest --reporter=json`
    javascript/                # ditto
    java/                      # `mvn test -B -DforkCount=1` / `gradle test`
    csharp/                    # `dotnet test --logger:trx`
    cpp/                       # `cmake --build && ctest --output-junit`
    rust/                      # `cargo test --message-format=json`
  evaluators/                  # CROSS-LANGUAGE judges
    test_runner/               # dispatches to bench/languages/<L>/testrunner.go
      dispatch.go
    patch_validator/           # `git apply --check`, AST parse-ok, ...
    semantic_oracle/           # rename-completeness, dead-code, ...
    token_meter/               # provider price tables × token counts
    tool_trace_analyzer/       # MUST_CALL / MUST_NOT_CALL rules (reuse score.Rules)
    regression_checker/        # repo-state diff before vs after
  runtime/                     # NEW infrastructure layer
    container/                 # docker/podman shell-out
      docker.go
      pool.go                  # warm container pool
    subprocess/                # daemon + agent + bench-rag subprocess control
      daemon.go                # specialization of internal/eval/sandbox
      agent.go                 # claude CLI control
      rag_server.go            # bench-rag stdio server control
    sandbox/                   # thin wrapper around internal/eval/sandbox
  schema/                      # versioned result/trace schemas
    result.v2.schema.json      # bumped from internal/eval/report v1
    trace.v2.schema.json
    capabilities.schema.json
  aggregator/                  # multi-run statistical layer
    bootstrap.go               # bootstrap CI computation (B=10000)
    passatk.go                 # pass@1 / pass@k
    cost.go                    # dollar-cost rollup
    cli/                       # `bench aggregate --runs <glob>`
  reports/                     # GENERATED markdown
    leaderboard.md
    per_language.md
    ablations.md
    cost_quality.md
  bench.yaml                   # top-level matrix definition (modes × benchmarks × langs × N)

cmd/
  helix-bench/                 # NEW orchestrator binary (sibling to helix-eval)
    main.go
    run_cmd.go                 # `helix-bench run`
    aggregate_cmd.go           # `helix-bench aggregate`
    list_cmd.go                # `helix-bench list` (debug)
  helix-bench-rag/             # NEW RAG baseline MCP server binary
    main.go                    # spawned only as a subprocess by the rag runner

internal/profile/profiles/     # MODIFIED: new YAMLs added
  bench-full.yaml              # NEW — full Helix tools
  bench-no-lsp.yaml            # NEW — strips LSP-backed tools
  bench-no-semantic.yaml       # NEW — strips semantic-index tools
  bench-no-structured-edit.yaml # NEW — strips replace_symbol_body / fuzzy_edit / etc.
  # NOTE: existing baseline.yaml is reused for baseline_plain (already strips everything)
```

### New vs Modified (explicit)

| Component | Status | Rationale |
|---|---|---|
| `bench/` tree (entire) | NEW | Net-new per milestone scope |
| `cmd/helix-bench` | NEW | Sibling to helix-eval, different SLO |
| `cmd/helix-bench-rag` | NEW | Standalone MCP server for RAG control |
| `internal/profile/profiles/bench-*.yaml` (4 files) | NEW | Ablation matrix mode definitions |
| `internal/profile/profiles/baseline.yaml` | UNCHANGED, REUSED | Already strips all tools (Phase 67 D-02) |
| `internal/eval/sandbox/` | UNCHANGED, IMPORTED | Re-exported via `bench/runtime/sandbox/` thin wrapper |
| `internal/eval/trace/` | UNCHANGED, IMPORTED | Reused for daemon + CC tap merging |
| `internal/eval/budget/` | UNCHANGED, IMPORTED | Reused for per-task budget loading |
| `internal/eval/score/` | UNCHANGED, IMPORTED | Reused for MUST_CALL / MUST_NOT_CALL rules |
| `internal/eval/judge/` | UNCHANGED, OPTIONAL | Bench may consume informationally (default: off) |
| `internal/eval/runner/` | UNCHANGED, NOT REUSED | Different orchestration shape; bench/runtime/ replaces it for bench/ tasks |
| `internal/phasegraph/` | UNCHANGED, REUSED | New `BuildBenchPhases()` constructor declared in `internal/phasegraph/pipelines/` |
| `cmd/helix-eval/` | UNCHANGED, FROZEN | v1.10 artifact, `eval/corpus/` continues to be its source-of-truth corpus |
| `eval/corpus/` | UNCHANGED, FROZEN | Does NOT migrate; bench/internal-toolbench is a fresh design |
| `internal/profile/profile.go` | UNCHANGED | No new mode-strip API needed; existing YAML mechanism is sufficient |
| `internal/mcp/` middleware | UNCHANGED | ProfileFilterMiddleware already supports new YAMLs transparently |

## Why `cmd/helix-bench` (not extend `cmd/helix-eval`)

| Dimension | `cmd/helix-eval` (v1.10 Phase 67) | `cmd/helix-bench` (v1.12) |
|---|---|---|
| Corpus shape | ~20 synthetic tasks, `task.md` + `repo/` + `verify.sh` + `expected_tools.yaml` | 6 public benchmarks (thousands of tasks) + 8-language internal ToolBench |
| Default `MaxParallel` | 1 (wall-time predictability per D-05) | ≥4, scales to host cores (must hit hours, not days) |
| Container runtime | None — direct subprocess | Required — SWE-bench Verified and Terminal-Bench ship Docker images |
| Per-task cost | $0 (no LLM in eval-quick; cheap in real-eval) | Real LLM cost across thousands of tasks × N runs — caching is mandatory |
| Multi-run | Single run | N ≥ 3 + bootstrap CI |
| Reports | 6-file fixed schema (`eval_report.{json,md}`, `cost_summary.json`, `tool_behavior.json`, `safety_compliance.json`, `run_metadata.json`) | 4-file flat markdown leaderboard + per-task `result.v2.json` |
| Result schema | `internal/eval/report.EvalResult` v1 | `bench/schema/result.v2.schema.json` v2 (superset of v1) |
| Exit-code contract | Non-zero on any task failure (EVAL-07 carves out judge) | Non-zero only on infra failure; per-task pass/fail is data, not exit signal |

These differences are not tunable knobs — they are fundamentally different SLOs. Rewriting helix-eval to absorb bench would:

- Break the v1.10 EVAL-* contracts that the audit is built on
- Force eval-quick (<30s) and a 12-hour SWE-bench Verified run to share one CLI surface
- Couple the eval `eval_report.md` schema to the bench leaderboard schema
- Force the `--quick` in-process scripted-agent path to coexist with a container pool

Cleaner separation: ship `cmd/helix-bench` as a sibling, and where they share infra (sandbox, trace merge, profile YAMLs, phasegraph) import the shared `internal/*` packages. `eval/` stays usable for fast PR-gated harness wiring checks; `bench/` is for milestone numbers.

## Why container runtime is `os/exec` Docker, not a Go library

SWE-bench's official harness publishes per-task Docker images (`sweb.eval.x86_64.{instance_id}`). Terminal-Bench ships task containers and a Python harness that drives `docker run`. Aider Polyglot and CrossCodeEval are simpler but most still ship Dockerfiles. The decision is whether `bench/runtime/container/` shells out to the `docker` CLI or links against `github.com/docker/docker/client`.

**Decision: shell-out to `docker` CLI (with podman as drop-in alternative).**

| Approach | Pro | Con |
|---|---|---|
| Shell `docker run` | Bit-exact match to upstream harnesses; rootless podman works the same; trivial to swap runtimes; zero CGO/dep weight | Slightly higher latency per call; no streaming events without `docker events --since` |
| Go Docker client | Streaming events; typed API; no shell parsing | Heavy dep tree; CGO not required but build weight is large; behaviour can diverge subtly from `docker run` flags that the public harnesses use; podman compatibility surface is partial |

The bench harness invokes Docker at most once per (task, mode, run) — call latency is irrelevant compared to LLM latency (~tens of seconds per task). Shell-out is also the only way to demonstrably match the public harness's container behaviour. Helix's CGO=1 build pipeline (Phase 59.1) is already heavy; adding the Docker client dep is unnecessary cost.

`bench/runtime/container/` interface:

```go
type Runtime interface {
    PullImage(ctx context.Context, ref string) error
    Run(ctx context.Context, spec RunSpec) (RunResult, error)  // calls `docker run`
    Exec(ctx context.Context, containerID string, cmd []string) (ExecResult, error)
    Stop(ctx context.Context, containerID string) error
}
```

Implementations: `DockerRuntime`, `PodmanRuntime`. Selection via `bench.yaml` (default: docker; fall back to podman if `docker` not on PATH).

## Why `baseline_rag` is its own MCP server binary

The proposed milestone scope describes baseline_rag as "grep + embeddings + chunk RAG presented as MCP tools the agent can call." This is fundamentally not a Helix profile — Helix has no embedding/vector layer at the MCP tool level, and the semantic index (Phase 60) is a different abstraction (it's a code-graph + ranked-file index, not chunk-level embedding retrieval).

Trying to express baseline_rag as a `bench-rag.yaml` Helix profile would force adding new tools to Helix purely to satisfy the control arm — contaminating the production tool surface for a measurement artifact. Worse, the "RAG baseline" would silently inherit any Helix-side improvement (e.g., a future fuzzy-search improvement that bleeds into grep). Both directions are bad.

**Decision: `cmd/helix-bench-rag` is a standalone Go binary that exports a stdio MCP server with exactly 4 tools:**

| Tool | Purpose |
|---|---|
| `rag_search(query, top_k)` | Embedding-search over the pre-built per-task index, returns ranked chunk IDs |
| `rag_read_chunk(chunk_id)` | Materializes a chunk back to file:line+text |
| `grep(pattern, path)` | Plain ripgrep wrapper (regex, glob) |
| `read_file(path, range?)` | Plain file read |

The embedding index lives at `bench/runners/baseline_rag_agent/index/` and is built once per (corpus, model_family) and cached. The agent (`claude`) is told about this server via `claude mcp add-json` exactly the same way it's told about `helix daemon` in the other modes. From the agent's point of view both modes look identical (one stdio MCP server); the difference is only which tools are exposed.

Embedding backend choice (deferred to phase plan): a small local embedder (e.g., `nomic-embed-text-v1.5` via ollama-style HTTP, or a Go-native option) keeps baseline_rag offline and reproducible. Avoid charging the RAG control arm for OpenAI/Anthropic embedding API calls — the control should not appear "more expensive than the agent" purely from index-construction cost.

## Per-language test runner location (verdict)

There were three plausible homes; only one keeps things sane.

| Option | Verdict | Rationale |
|---|---|---|
| `bench/evaluators/test_runner/<lang>.go` | NO | Forces a single Go package to know all 8 build systems; explodes import graph (cgo for nothing); blocks per-language contributors |
| `bench/runners/test_runner/` (shared) | NO | Confuses "runner" (mode dispatcher) with "test runner" (build-system invoker); not a mode |
| `bench/languages/<lang>/testrunner.go` | **YES** | One package per language, each owning its own build-system shell-out + parser. `bench/evaluators/test_runner/dispatch.go` is a 30-line switch keyed on language. |

This co-locates the test-runner with the per-language contract tests (`bench/languages/<lang>/contract_test.go`) that need to call them anyway. It also matches the existing Helix pattern: `internal/kernel/jsonrpc/`, `internal/skill/<name>/`, etc., are package-per-concern.

The `evaluators/test_runner/dispatch.go` interface is:

```go
type Result struct {
    Passed   int
    Failed   int
    Skipped  int
    DurationMs int64
    Junit    []byte    // canonical JUnit XML for cross-lang rollup
    FailLog  string
}
type Runner interface {
    Run(ctx context.Context, repoDir string) (Result, error)
}
```

Each `bench/languages/<lang>/testrunner.go` implements `Runner`. Dispatch picks one based on the task's `language` field.

## Ablation modes mapped to Helix profiles

Five of the six modes are Helix-side profile YAMLs. The sixth (`baseline_rag`) is the separate MCP server above.

| Bench mode | MCP server spawned | Profile / surface |
|---|---|---|
| `baseline_plain_agent` | none (shell + bash + write tools agent-native only) | n/a — `bench/runners/baseline_plain_agent/` exposes nothing |
| `baseline_rag_agent` | `cmd/helix-bench-rag` | 4 fixed tools |
| `your_agent_full` | `helix daemon` | `bench-full` profile — superset of `claude-code.yaml`, admin mode, includes everything kernel + skills + semantic |
| `your_agent_no_lsp` | `helix daemon` | `bench-no-lsp` — exclude `find_references`, `goto_definition`, `find_implementations`, `get_diagnostics`, `get_call_hierarchy`, `get_type_hierarchy`, `rename_symbol`, `format_file`, `get_code_actions` (the 9 LSP-backed tools) |
| `your_agent_no_semantic` | `helix daemon` | `bench-no-semantic` — exclude the 10-tool SemanticSkill surface added in v1.11 Phase 73 (`explain_symbol_deep`, `find_related_symbols`, `validate_graph_edge`, `get_cluster_map`, `explain_cluster`, `get_change_impact_graph`, plus the 4 P0 semantic tools) and the strangler-fig branch of `get_repo_map`/`get_context` (via env-var feature flag or a `bench-no-semantic.yml` config override that disables `SetSemanticLookup`) |
| `your_agent_no_structured_edit` | `helix daemon` | `bench-no-structured-edit` — exclude `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `fuzzy_edit`, and force `replace_in_file` to exact-match only (forces agent to write raw patches) |

All four `bench-*.yaml` profiles inherit the existing `claude-code.yaml` mode-transition table and guardrails; they only differ in their `skills:` / `exclude_tools:` lists. This is the same mechanism the existing 5 profiles already use — zero new YAML schema.

**Architectural risk (semantic ablation):** `no_semantic` is the only ablation that cannot be expressed cleanly through `skills:` / `exclude_tools:` alone. The strangler-fig integration in v1.10 Phase 65 wires the semantic lookup into `get_repo_map`/`get_context` at daemon-bootstrap time via `RepoMapSkill.SetSemanticLookup`. The mode YAML cannot un-wire that. Two options:

1. **Config-key gate (recommended):** add a single config key `semantic_index.bench_disabled: true` that, when set, makes the daemon skip `SetSemanticLookup` at bootstrap. The bench runner sets this in the mode-specific `helix_config.yml`. Zero new code paths inside `repomap`/`semantic`; the rest of Helix can't tell the difference from a workspace with no semantic store.
2. Build a v2 strangler-fig flag inside `internal/semantic/integ` that lets the lookup pretend to be unbuilt. More invasive, no benefit.

Go with option 1. Document the config key in `bench/runners/your_agent_no_semantic/CONFIG.md` and put a vet-style guard somewhere obvious so a future contributor doesn't accidentally rip it out.

## Reuse from v1.10 Phase 67 (explicit)

| Phase 67 component | Reuse strategy |
|---|---|
| `internal/eval/sandbox/` | Re-export from `bench/runtime/sandbox/` (thin wrapper that adds container-aware Prepare). The path-traversal hardening, `/tmp` short-path, lstat-symlink rejection (T-67-01) all carry forward. |
| `internal/eval/trace/MergeInput` + `Merge` | Direct import. Daemon-tap + CC-tap merging is identical between eval and bench; the merged trace.json is the basis for the `result.v2.json` schema. |
| `internal/eval/budget/` | Direct import. `bench.yaml` ships a global budget; per-task `budget.yaml` override is honored. |
| `internal/eval/score.Rules` (`expected_tools.yaml` DSL) | Direct import. ToolBench fixtures ship `expected_tools.yaml` files using the existing DSL; `bench/evaluators/tool_trace_analyzer/` is a wrapper around `score.Apply`. |
| `internal/eval/judge/` | Imported but **off by default in bench** (per milestone scope: judge stays informational, EVAL-07 holds). Bench can produce a `tool_behavior_judge.json` sidecar if `--judge` is passed. |
| `internal/eval/report.EvalResult` v1 schema | NOT used directly — bench writes `bench/schema/result.v2.schema.json`. v2 is a superset: every v1 field is present (so a downstream eval consumer keeps working) plus the new bench fields (per-run array, bootstrap CI, dollar-cost, container ID, container exit code, JUnit XML blob). |
| `internal/phasegraph/pipelines/` | Add `BuildBenchPhases(...)` constructor alongside the existing `BuildEvalPhases`. Bench has more phases (container start, test-runner dispatch by language, multi-run loop) but the validator and runner are unchanged. |
| `eval/EVAL.md` ZDR attestation | Inherited verbatim by `bench/BENCH.md`; the same ZDR gate (`HELIX_EVAL_ZDR_VERIFIED=1`, renamed `HELIX_BENCH_ZDR_VERIFIED=1` for bench, both checked) blocks bench against non-synthetic corpora without enterprise attestation. |

## Components (responsibility table)

| Component | Responsibility | Communicates with |
|---|---|---|
| `cmd/helix-bench` | CLI surface, matrix expansion, work-stealing scheduler, result aggregation | `bench/runtime/*`, `bench/aggregator/`, `bench/evaluators/*` |
| `cmd/helix-bench-rag` | Stdio MCP server exporting 4 RAG tools | `claude` (over stdio); reads `bench/runners/baseline_rag_agent/index/` |
| `bench/runtime/container/` | `docker`/`podman` shell-out + warm container pool | `bench/runtime/subprocess/` (inside a container) |
| `bench/runtime/subprocess/` | Spawn helix daemon + claude CLI + helix-bench-rag with the right env / args / sockets | `bench/runtime/sandbox/`, `internal/eval/trace/` |
| `bench/runtime/sandbox/` | Filesystem isolation per (task, mode, run) | wraps `internal/eval/sandbox/` |
| `bench/datasets/<X>/adapter.go` | Convert upstream task format → canonical `bench.Task` JSON | `bench/runtime/*` |
| `bench/runners/<mode>/runner.go` | Knows which MCP server (if any) this mode spawns, which profile to pass | `bench/runtime/*` |
| `bench/languages/<L>/contract_test.go` | 10-capability ToolBench assertions for language L | `bench/languages/<L>/testrunner.go` |
| `bench/languages/<L>/testrunner.go` | Shell out to the language's native test runner and parse JUnit XML | language-native test runner (`go test`, `pytest`, etc.) |
| `bench/evaluators/*` | Cross-language graders | `bench/schema/result.v2` |
| `bench/aggregator/` | Bootstrap CI, pass@k, cost rollup across N runs | reads `result.v2.json` files |
| `bench/reports/` | Generated markdown (artifact) | n/a |

## Data Flow Sequence (per task, per mode, per run)

1. **Matrix expansion** — `cmd/helix-bench run --bench=swebench --mode=your_agent_full --runs=3` expands to `len(tasks) × len(modes) × runs` work items.
2. **Cache probe** — for each item, hash `(task_id, mode, model_id, prompt_template_sha, helix_sha, runs_seed)` and consult `bench/.cache/` for a prior `result.v2.json`. If hit, skip; if miss, queue.
3. **Container start** (if benchmark requires) — `bench/runtime/container/` pulls and starts the per-task image; mounts the per-(task, mode, run) sandbox dir from `bench/runtime/sandbox/`.
4. **MCP server start** — mode-specific runner from `bench/runners/<mode>/runner.go` spawns:
   - `your_agent_*` → `helix daemon --profile=<bench-*>`
   - `baseline_rag_agent` → `helix-bench-rag --index=<path>`
   - `baseline_plain_agent` → none
5. **Agent start** — `claude` subprocess; given the MCP server config + task prompt + sandbox `repoDir`. Output captured to `claude.stdout` and `claude.jsonl`.
6. **Trace tap & merge** — on agent exit, `trace.TapDaemonLog(daemon.log, pid)` + `trace.TapCCStream(claude.jsonl)` → `trace.Merge(...)` → `trace.json`.
7. **Evaluators** — in order:
   - `patch_validator` → `git apply --check`
   - `test_runner/dispatch.go` → calls `bench/languages/<L>/testrunner.go.Run(ctx, repoDir)` → JUnit XML
   - `semantic_oracle` → optional rename-completeness / dead-code checks
   - `token_meter` → `(InputTokens, OutputTokens) × provider_price` → `cost_usd`
   - `tool_trace_analyzer` → `score.Apply(merged, rules)`
   - `regression_checker` → diff between `git stash before` and `git stash after`
8. **Result write** — `result.v2.json` written to `bench/results/<run_id>/<task_id>/<mode>/run_<k>/result.v2.json`. Schema-validated against `bench/schema/result.v2.schema.json` on write.
9. **Container stop** — `docker rm -f`; sandbox dir kept for 24h then GC'd.
10. **Aggregation** (deferred, separate command) — `cmd/helix-bench aggregate --runs <glob>` walks `result.v2.json` files, computes bootstrap CIs and pass@k, writes `bench/reports/*.md`.

## Patterns to Follow

### Pattern 1: Mode-as-MCP-server-configuration
**What:** Each ablation mode is defined by which MCP server the agent connects to and which tools that server exposes. Never by injecting flags into Helix's tool handlers.

**When:** Always for ablation arms. If a mode needs different behaviour inside a tool, that's a design smell — split the surface instead.

**Example:**
```go
// bench/runners/your_agent_no_lsp/runner.go
func (r *Runner) MCPServerCommand(sb *sandbox.Sandbox, task bench.Task) *exec.Cmd {
    return exec.Command(r.helixBin, "daemon",
        "--socket", sb.SocketFor(task.ID, "no_lsp"),
        "--profile", "bench-no-lsp",
        "--config", sb.ConfigFor(task.ID, "no_lsp"))
}
```

### Pattern 2: Result schema is a versioned, validated artifact
**What:** `result.v2.json` ships with a published JSON Schema. Every writer validates on write; the aggregator validates on read. Schema version is part of the cache key.

**When:** Every result write. No exceptions.

### Pattern 3: Multi-run bootstrap, not single-run reporting
**What:** A single `run_<k>` directory is not a publishable result. All claims come from `bench/aggregator/` consuming N ≥ 3 runs and producing `mean ± bootstrap_95ci`.

**When:** Every external claim. Internal CI gates on aggregated values, not on single runs.

### Pattern 4: Caching is keyed by content, not by name
**What:** Skip a (task, mode) only if the hash `(task_id, mode, model_id, prompt_sha, helix_sha, seed)` matches. Bench-side changes that affect the runner code force re-run because `helix_sha` changes.

### Pattern 5: Container is per-task, not per-suite
**What:** One container per (task × mode × run). No shared-state long-lived container. This matches SWE-bench Verified harness behaviour and prevents cross-task leakage.

## Anti-Patterns to Avoid

### Anti-Pattern 1: Profile-as-mode-of-Helix-internals
**What:** Wiring an `--ablation=no_semantic` flag into `internal/skill/semantic/` so tools change behaviour based on a CLI flag.

**Why bad:** Pollutes production code with bench-only knobs; future refactors silently break the ablation; mode is no longer a clean "the agent sees a different tool surface" — it becomes "tools secretly behave differently in bench."

**Instead:** Mode = profile YAML + (rare) config-key gate at daemon bootstrap. The tools themselves never know about modes.

### Anti-Pattern 2: Sharing one container across tasks
**What:** Spinning up one Ubuntu container, then `docker exec`'ing each task into it.

**Why bad:** SWE-bench Verified uses per-task images (`sweb.eval.x86_64.<instance_id>`); sharing breaks comparability. Cross-task filesystem leakage taints `regression_rate` measurements. Long-lived containers accumulate state that biases later tasks.

**Instead:** One container per (task × mode × run). Warm-pool the *image*, not the running container.

### Anti-Pattern 3: Single-run results in reports
**What:** Generating `leaderboard.md` from one `runs=1` matrix and publishing it.

**Why bad:** LLM nondeterminism + container scheduling noise will swing results by 5–15% between runs. Any single-run delta is noise. Reviewers will reject the claim.

**Instead:** `--runs ≥ 3` for any number that leaves the repo. Aggregator refuses to write `reports/` if any (task, mode) cell has fewer than the configured minimum.

### Anti-Pattern 4: Reusing `eval/corpus/` as bench input
**What:** Pointing `cmd/helix-bench --corpus=eval/corpus`.

**Why bad:** `eval/corpus/` is 20 synthetic tasks for harness-wiring validation; it does not stress the 10-capability ToolBench schema; mixing it into bench reports contaminates them with a different SLO's data.

**Instead:** `bench/datasets/internal-toolbench/` is built fresh against the 10-capability schema. `eval/corpus/` remains the v1.10 harness-validation corpus.

### Anti-Pattern 5: Linking the Docker Go client
**What:** Importing `github.com/docker/docker/client` for "type safety."

**Why bad:** ~500MB dep tree, slower builds, no measurable benefit; behaviour diverges from public harnesses that use the CLI.

**Instead:** `os/exec` + structured stdout parsing. Drop-in podman support comes free.

## Scalability Considerations

| Concern | At 100 tasks (smoke) | At 1,000 tasks (Aider + ToolBench) | At 10,000 tasks (full Multi-SWE-bench × N) |
|---|---|---|---|
| Wall-time | ~30 min, 1 host | 4–8 hours, 1 host with `--parallel=4` | Distributed across hosts; cache + resume essential |
| Disk | <10 GB | 100 GB (per-task containers + sandboxes) | TB-class; `bench/.cache/` GC policy required |
| LLM cost (per run × N=3) | ~$5 | ~$200 | ~$2,000 — caching keyed by `(task_sha, mode, model)` is non-negotiable |
| Container concurrency | 4 | 8–16, bounded by host RAM | Cluster-scheduled; out of scope for v1.12 |
| Memory | helix daemon ~200MB × parallelism | Same × parallelism + ~1GB per container | Need pressure eviction in the container pool |

## Phase Build Order (with dependency rationale)

Strictly bottom-up; nothing earlier depends on anything later.

| Order | Phase | Why it must come this early |
|---|---|---|
| 1 | **Tree skeleton + `result.v2` schema + `bench.yaml` shape** | Every later phase writes results or reads `bench.yaml`. Schema landing first lets all subsequent phases be schema-validated. |
| 2 | **Mode YAMLs (`bench-full`, `bench-no-lsp`, `bench-no-semantic`, `bench-no-structured-edit`) + `semantic_index.bench_disabled` config key** | Profile-filter middleware is exercised by golden tests as soon as YAMLs land; no need to wait for runners. The `bench_disabled` config-key gate is the one architectural change inside daemon bootstrap — land it early so it's stable when downstream phases need it. |
| 3 | **`bench/runtime/sandbox/` thin wrapper + `bench/runtime/subprocess/` for `helix daemon`-only modes** | Reuses `internal/eval/sandbox/`. Required by all `your_agent_*` modes. No container yet. |
| 4 | **`cmd/helix-bench run` skeleton + matrix expander + `bench/runners/your_agent_full/runner.go`** | First end-to-end smoke: one task, one mode, no container, no eval. Forces the orchestrator shape to materialise. |
| 5 | **`bench/languages/go/` (contract_test + testrunner) + `bench/datasets/internal-toolbench/go/` first 3 fixtures** | Go is Helix's own language; tightest feedback loop; no container needed; no external corpus dependency. Validates the per-language test-runner pattern before we generalize. |
| 6 | **`bench/evaluators/test_runner/dispatch.go` + `bench/evaluators/patch_validator/` + `bench/evaluators/token_meter/` + `bench/evaluators/tool_trace_analyzer/`** | Minimum graders to produce a meaningful `result.v2.json`. `tool_trace_analyzer` is a wrapper over existing `internal/eval/score`. |
| 7 | **`bench/runners/baseline_plain_agent/` + `bench/runners/your_agent_no_lsp/` + `bench/runners/your_agent_no_structured_edit/`** | First three ablation comparisons. No new infra needed (no container, no embedding index). `no_semantic` deferred to after the config-key plumbing has soaked. |
| 8 | **`bench/runners/your_agent_no_semantic/` + the `semantic_index.bench_disabled` plumbing E2E test** | Trickiest mode; gets a phase to itself. |
| 9 | **`bench/aggregator/` with bootstrap CI + pass@k + first cut of `bench/reports/leaderboard.md`** | The first end-to-end externally-publishable artifact. Tests the multi-run loop end to end. Output is internal-ToolBench-only for now — no public benchmarks. |
| 10 | **`cmd/helix-bench-rag/` + `bench/runners/baseline_rag_agent/` + embedding-index builder** | Self-contained subsystem; needs to ship as its own binary; benefits from soaking before public benchmarks land (otherwise its results will be noise on the leaderboard). |
| 11 | **`bench/runtime/container/` + Docker runtime + warm-pool** | Container infra needed only for public benchmarks. Internal ToolBench runs container-free up to here. |
| 12 | **`bench/datasets/aider-polyglot/` adapter** | Cheapest public benchmark first (225 tasks, no container in most languages, simple verify). First non-Helix corpus through the harness. |
| 13 | **`bench/languages/{python,typescript,javascript,rust,cpp,java,csharp}/`** in priority order driven by Aider Polyglot coverage | Adds the other 7 testrunners. Order can match what the team has installed locally; CI install matrix grows accordingly. |
| 14 | **`bench/datasets/crosscodeeval/` + `bench/datasets/repobench/`** | Mid-size public benchmarks. |
| 15 | **`bench/datasets/swebench/` (Verified split, 500 Python tasks)** | First containerized large-scale benchmark. Requires container runtime + per-task image pull + the `internal-toolbench/python/` work to be solid. |
| 16 | **`bench/datasets/multi-swebench/` + `bench/datasets/terminal-bench/`** | Final public coverage. Multi-SWE-bench exercises 7 languages × ~230 tasks each; Terminal-Bench 2.0 is the long-horizon control. |
| 17 | **`bench/reports/{per_language,ablations,cost_quality}.md` generators + final leaderboard pass** | Reports are the publication artifact; they come after every input that feeds them is stable. |
| 18 | **Documentation + migration note in `eval/EVAL.md` clarifying eval ↔ bench separation** | Wraps the milestone. |

Critical-path test: a phase numbered N is allowed to depend on any phase numbered < N and is forbidden from depending on a phase numbered ≥ N. Each row above satisfies this.

## Architectural Risks (BLOCKER-level questions to resolve before phases land)

1. **`no_semantic` mode purity.** If `semantic_index.bench_disabled` accidentally still allows the persisted semantic store to leak through `get_repo_map`'s strangler-fig branch, the ablation is meaningless. **Mitigation:** add a regression test in v1.12 phase 2 that asserts `get_repo_map` returns the v1.9 tree-sitter path (not the persisted-graph path) when `bench_disabled` is set, with the `source` envelope field equal to `"tree_sitter"` rather than `"semantic"`. Make this a vet-style boundary the way `vet-nokernel2semantic` works.

2. **Container reproducibility on CI vs maintainer laptop.** SWE-bench Verified images are amd64; Helix maintainers may be on arm64 (Apple Silicon). Running through Rosetta or qemu-user gives unreliable timing. **Mitigation:** bench harness refuses to run SWE-bench Verified on a host arch that doesn't match the image arch unless `BENCH_ARCH_MISMATCH_OK=1` is set; otherwise prints a one-line remediation. Mirror the v1.10 ZDR gate's pattern.

3. **LLM cost containment.** A full bench run is thousands of LLM calls. Without the content-hash cache there's nothing stopping an accidental re-run from charging $2,000. **Mitigation:** the cache directory and key derivation lands in phase 1 (with the `result.v2` schema). Aggregation in phase 9 surfaces "$X spent" prominently. The CI gate (if any) caps total spend per invocation.

4. **RAG baseline embedding choice as a confounder.** If `baseline_rag` is run against a much weaker embedder than is industry-standard, the headline claim becomes "Helix beats a bad RAG baseline," which is uninteresting. **Mitigation:** document the embedding model choice in `bench/runners/baseline_rag_agent/EMBED-CHOICE.md` and pin it; baseline_rag results must include the embedder ID in `result.v2.json`. Roadmap a phase for "embedder swap study" if reviewer pushback materializes.

5. **`eval/` deprecation timing.** The milestone scope says "eval/ left as legacy; synthetic corpus may migrate later but is not the source of truth." But `cmd/helix-eval` continues to be a PR-gated wiring test (eval-quick, <30s). If we delete eval/ prematurely we lose that gate. **Mitigation:** explicit project decision (record in PROJECT.md Key Decisions): `eval/` is frozen at v1.10 shape, continues to back PR-gate wiring tests, and is not extended in v1.12. A v1.13+ phase may unify them once `bench/` has proven shape — not now.

6. **`baseline_plain_agent` tool surface.** The "plain" baseline says "shell+grep+read+edit+test." If we let the agent use its own native edit tool (Claude Code's `Edit` tool) vs. forcing it through a stdio MCP server we expose, results across modes are not apples-to-apples (other modes use agent-native tools too). **Mitigation:** explicit decision needed before phase 7: do all modes run with the agent's native edit/read tools enabled in addition to the mode-specific MCP server? Recommendation: **yes** — `claude-code.yaml` already excludes Helix's `read_file`/`create_text_file` because the agent has its own, so this matches the existing pattern. Document the decision in `bench/BENCH.md` so reviewers can audit it.

## Sources

| Source | Confidence | Notes |
|---|---|---|
| `.planning/PROJECT.md` (lines 147–187) | HIGH (curated) | v1.12 milestone scope, target features, out-of-scope |
| `internal/profile/profiles/baseline.yaml` | HIGH (source) | Phase 67 D-02 mechanism: empty `skills:` + `tools:` strips everything |
| `internal/profile/profiles/claude-code.yaml` | HIGH (source) | Reference for `skills:` / `exclude_tools:` shape of new bench-*.yaml |
| `cmd/helix-eval/main.go` | HIGH (source) | 6-file report schema, `--quick` carve-out, `--judge` opt-in pattern |
| `cmd/helix-eval/run_cmd_test.go` | HIGH (source) | EVAL-07 "judge cannot affect exit code" invariant — bench will mirror this |
| `internal/eval/runner/runner.go` | HIGH (source) | 10-step per-(task, mode) pipeline shape that bench will generalize |
| `internal/eval/sandbox/sandbox.go` | HIGH (source) | `/tmp` short-path requirement (UDS 104-byte limit on macOS) — bench/runtime/sandbox/ inherits this |
| `internal/phasegraph/phase.go` + `run.go` | HIGH (source) | `BuildBenchPhases` will sit alongside `BuildEvalPhases` |
| `eval/EVAL.md` | HIGH (curated) | ZDR attestation pattern, eval-quick vs eval distinction — both inherited by bench |
| `eval/corpus/` directory listing | HIGH (source) | 20 synthetic tasks, mostly Go + Python — confirms internal-toolbench needs to be a fresh design, not a migration |
| SWE-bench Verified harness | MEDIUM (general knowledge) | Per-task Docker images, `sweb.eval.x86_64.<instance>` naming, `docker run` reference flow |
| Terminal-Bench 2.0 design | MEDIUM (general knowledge) | Containerized, long-horizon, 89 tasks |
| Aider Polyglot | MEDIUM (general knowledge) | 225 Exercism tasks, 6 languages, repo-level evaluation, smaller harness |
| MCP Go SDK behaviour (claude mcp add-json) | HIGH (in-tree usage via `internal/cli/setup_clients.go`) | Confirms that `cmd/helix-bench-rag` can be registered with Claude via the same CLI invocation Helix already uses |
