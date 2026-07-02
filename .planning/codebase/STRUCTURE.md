# Codebase Structure

**Analysis Date:** 2026-07-01

Helix ships as a **single Go binary** (`cmd/helix`, entrypoint `cmd/helix/main.go`)
driving an `internal/**` codebase. Module `github.com/agenthands/helix`, Go 1.25.1,
`CGO_ENABLED=1`. Scale: **85,472 non-test LOC** across 24 `internal/` packages;
595 non-test `.go` files, 590 `*_test.go`; zero TODO/FIXME/HACK in `internal`+`cmd`.
The agent-facing surface is the `helix <verb>` CLI (51 frozen verbs, generated
catalog `internal/cli/verbs_gen.go`; full inventory auto-generated in `README.md`).

## Directory Layout

```
helix/
├── cmd/                                 # 16 binaries (product + generators + gates)
│   ├── helix/                           # THE product; single entrypoint main.go, subcommands in internal/cli
│   ├── docgen/                          # generates README.md tool/language tables from skill init() registry
│   ├── lspgen/                          # generates protocol/gen/*.go from protocol/metaModel.json (LSP 3.17)
│   ├── helix-cligen/                    # generates internal/cli/verbs_gen.go (verbSpecs catalog)
│   ├── helix-refgen/                    # generates internal/cli/skills/helix/reference.md (per-verb reference)
│   ├── helix-eval/                      # Phase 67 evaluation harness (run/report subcommands)
│   ├── helix-bench/                     # Phase 75 provider-independent benchmark harness entrypoint
│   ├── helix-bench-rag/                 # standalone provably-isolated baseline_rag MCP server (ABLATE-04)
│   ├── eval-attestation-check/          # warn-only date-staleness check for eval/EVAL.md attestation
│   ├── vet-noduckdb/                    # go/analysis gate: STORE-06 (no direct duckdb import)
│   ├── vet-nokernel2semantic/           # gate: LIVE-07 #1 (kernel must not import semantic)
│   ├── vet-nosemantic2kernel/           # gate: ENRICH-01 #1 (semantic must not import kernel)
│   ├── vet-compact-uses-store/          # gate: compact→store boundary (belt-and-braces over noduckdb)
│   ├── vet-ablation-leakage/            # gate: ABLATE-08 (bench-runner → disabled-subsystem imports)
│   ├── vet-bench-rag-leakage/           # gate: ABLATE-04 #1c (baseline_rag → kernel/semantic imports)
│   └── vet-tools-quarantine/            # gate: runtime → dev-time tools/ import boundary
├── internal/                            # 24 packages, 85,472 non-test LOC
│   ├── semantic/    (30,814)            # semantic index subsystem (18 subpackages; see below)
│   ├── kernel/      (11,046)            # code-intelligence kernel + 8 subpackages
│   ├── daemon/       (9,025)            # persistent supervisor, rank engine, semantic wiring, middleware install
│   ├── skill/        (7,841)            # Skill/ToolProvider/WorkflowProvider interfaces + 5 subpackages
│   ├── cli/          (5,204)            # cobra command tree, verbSpecs (51 verbs), setup/status/activate
│   ├── mcp/          (2,043)            # SerenaMCPServer wrapper, ToolRegistry, middleware, gRPC transport
│   ├── repomap/      (1,891)            # tree-sitter tag extraction, SQLite tag cache, PageRank, token-budget render
│   ├── obs/          (1,539)            # observability scaffolding (Prometheus RED metrics, OTel tracing)
│   ├── upgrade/      (1,534)            # in-binary self-upgrade subcommand pair
│   ├── guardrails/     (956)            # server-side capability receipts (Phase 66, GUARD-01..07)
│   ├── langregistry/   (747)            # 52-language embedded registry, YAML override, three-tier LS installer
│   ├── forwarder/      (773)            # client-side gRPC StreamMCP dial path (CallTool/OpenSession)
│   ├── fuzzy/          (745)            # pure 4-strategy fuzzy text matcher with ambiguity refusal
│   ├── memory/         (728)            # markdown memory store, SQLite FTS5 index, fsnotify watcher
│   ├── profile/        (728)            # 5 agent profiles, 4 operational modes
│   ├── phasegraph/     (601)            # stdlib-only DAG orchestrator (ordered, validated)
│   ├── config/         (464)            # koanf-backed daemon configuration (4-layer precedence)
│   ├── graph/          (265)            # deterministic generic PageRank engine
│   ├── errors/         (225)            # typed error taxonomy for MCP tools
│   ├── degrade/        (141)            # tool classification + timeout budget lookup
│   ├── treesitter/     (137)            # shared grammar registry (23 grammars)
│   ├── workspace/      (132)            # workspace registry, WorkspaceKey (repo root + lang + toolchain)
│   ├── lint/         (subpkgs)          # 7 go/analysis Analyzers backing the vet-* gate binaries
│   └── eval/           (384)            # top-level runner library for the Phase 67 eval harness
├── api/proto/serena/v1/                 # gRPC IPC proto (ipc.proto/ipc.pb.go/ipc_grpc.pb.go)
│                                        #   dir name "serena/v1" retained as wire-format lineage (Phase 52-03), NOT residue
├── protocol/
│   ├── gen/                             # generated LSP 3.17 types (324 structs, 216 union types) from metaModel.json
│   ├── patch/                           # metaModel patches (compatibility.go, rename_params.go)
│   └── metaModel.json                   # LSP 3.17 machine-readable spec (lspgen input)
├── bench/                               # 291 .go files: milestone bench harness (SWE-bench, Multi-SWE-bench,
│                                        #   Terminal-Bench, aider-polyglot, repobench, crosscodeeval); Podman/Docker auto-detect
├── tools/dspy-tune/                     # dev-time-only Python (DSPy tuning harness); NOT shipped, uv/uvx, .venv git-ignored
├── eval/                                # eval corpus, fixtures, generated tasks, reports, EVAL.md
├── test/                               # cross-package harnesses (bench, harness, integration, oracle)
├── testdata/                            # shared fixtures + profiles
├── deploy/grafana/                      # Grafana dashboards for the obs metrics
└── docs/                                # edge-types.md, type-resolution.md, runbooks/ (CURRENT; refreshed v2.13)
```

### `internal/semantic/` subtree (30,814 LOC — the largest subsystem)

18 subpackages emitting a semantic graph (edges DEFINES, RESOLVES_TO/has_type,
DATA_FLOWS, SEMANTICALLY_RELATED, SIMILAR_TO, STRUCTURAL_TWIN; read surface +
ledger in `docs/edge-types.md`). LOC per subpackage (non-test):

```
internal/semantic/
├── extract/     (8,410)  # per-language tree-sitter symbol extraction (11 langs w/ testdata: go/ts/java/csharp/kotlin/php/python/ruby/rust/c/cpp)
├── store/       (5,211)  # duckdb-backed graph store + overlay write API
├── types/       (3,373)  # type resolvers (C-family tiered resolvers), RESOLVES_TO/has_type
├── live/        (3,087)  # live index / fsnotify-driven FileFactDiff populator
├── lspenrich/   (2,706)  # async LSP enrichment of tree-sitter-extracted facts
├── graph/       (1,628)  # graph_version advance machinery, weak-component wiring
├── compact/       (913)  # graph compaction (store-backed; enforced by vet-compact-uses-store)
├── retrieval/     (881)  # bleve-backed full-text retrieval engine
├── integ/         (744)  # types-only seam between semantic subsystem and consumers
├── scheduler/     (624)  # extraction lifecycle (initial-walk on workspace activation)
├── classifier/    (584)  # per-language name-based edge classification
├── relatedidx/    (507)  # Random Indexing over function bodies → SEMANTICALLY_RELATED edges
├── dataflow/      (485)  # case-1 + in-body intraprocedural flow summary → DATA_FLOWS edges
├── cluster/       (297)  # deterministic weak-component clustering (GRAPH-06)
├── crossrepo/     (268)  # pure resolution logic behind CROSS_* edges
├── minhash/       (227)  # MinHash + LSH fingerprinting → SIMILAR_TO near-clone edges
├── bench/         (187)  # minimal probe binary pulling in bleve for the linker
└── cochange/      (186)  # mines git history for file co-change relationships
```

The **intraprocedural data-flow** path spans two subsystems: `semantic/dataflow`
computes a per-function case-1 flow summary (leaf package, stdlib + tree-sitter
only) invoked from the shared `extract.FingerprintBody` seam so all 11 language
providers inherit it with no per-provider edit; the daemon then turns those
summaries into `DATA_FLOWS` edges in `internal/daemon/semantic_similarity_edges.go`
(`dataFlowEdges` def_use, `inBodyDataFlowEdges` def_use_inbody, `returnBridgeEdges`
def_use_return), wired in `semantic_wiring.go` strictly after `dataFlowEdges`.

The semantic subsystem is walled off by four `vet-*` gates: `vet-nokernel2semantic`
(kernel↛semantic), `vet-nosemantic2kernel` (semantic↛kernel), `vet-compact-uses-store`
(compact→store), `vet-noduckdb` (no direct duckdb outside store).

### `internal/kernel/` and `internal/skill/` subtrees

```
internal/kernel/            internal/skill/
├── symbols/  (9 symbol tools)   ├── memory/     (7 memory tools; markdown + FTS5)
├── edit/     (6 edit tools)     ├── workflow/   (onboarding, session handoff)
├── fileops/  (7 file tools)     ├── repomap/    (get_repo_map, get_context)
├── diag/     (3 diag tools)     ├── semantic/   (Phase 64 semantic MCP tools)
├── lspool/   (LS worker pool)   └── guardrails/ (capability-receipt tools)
├── jsonrpc/  (JSON-RPC 2.0 codec)
├── health/   (get_health tool)
└── help/     (get_tool_help tool)
```

## Directory Purposes

Layout maps onto the 4-layer architecture (see `ARCHITECTURE.md`):

- **Layer 0 — MCP Runtime.** `internal/mcp/` (MCP server `SerenaMCPServer`, tool
  registry, middleware stack, gRPC transport), `internal/daemon/` (persistent
  supervisor, errgroup orchestration, signal-first lifecycle), `internal/forwarder/`
  (client-side gRPC `StreamMCP` dial path), `api/proto/serena/v1/` (gRPC IPC wire).
- **Layer 1 — Code-Intelligence Kernel.** `internal/kernel/` (orchestrator,
  workspace runtime, language detection) + subpackages `lspool` (share-until-dirty
  LS pool, adaptive TTL, circuit breaking, pressure eviction), `symbols`, `edit`,
  `fileops`, `diag`, `jsonrpc`, `health`, `help`; `internal/fuzzy/` (4-strategy
  cascade), `internal/repomap/` (tag cache + PageRank + budgeted renderer),
  `protocol/gen/`.
- **Layer 2 — Skills & Multi-Language.** `internal/skill/` (Skill/ToolProvider/
  WorkflowProvider interfaces, Caddy-style `init()` registration) + subpackages
  `memory`, `workflow`, `repomap`, `semantic`, `guardrails`; `internal/memory/`,
  `internal/langregistry/` (52-lang embedded registry, three-tier LS installer).
- **Layer 3 — Profiles & Setup.** `internal/profile/` (5 profiles, 4 modes),
  `internal/config/` (4-layer koanf: CLI > project `.helix/` > user `~/.helix/` >
  profile defaults), `internal/cli/setup*.go` and `status*.go`.

The **semantic index** (`internal/semantic/`) is an independent subsystem consumed
by Layer 2's `skill/semantic` and wired by `internal/daemon/`; it does not sit on
the kernel critical path and is import-isolated from the kernel by vet gates.

## Key File Locations

**Entry Points:**
- `cmd/helix/main.go`: the sole product binary; delegates to `internal/cli/root.go`
  (cobra command tree). `version` injected via `-ldflags`.
- `internal/cli/verbs_gen.go`: generated `verbSpecs` catalog (51 verbs); one
  `helix <verb>` subcommand per callable tool. Regenerate via `cmd/helix-cligen`.
- `internal/daemon/daemon.go`: daemon bootstrap — registry, kernel+pool,
  GrammarRegistry (23 grammars), TagCache, skill `InitAll`, middleware install
  (steps 14/14b/14c), profile resolution.
- `internal/mcp/server.go`: `SerenaMCPServer` / `NewSerenaMCPServer`, the MCP
  server wrapper (Go identifier retained per Phase 52-03; user-facing name `helix`).

**Configuration:**
- `internal/config/`: koanf loader with 4-layer precedence.
- `internal/profile/`: profile/mode YAMLs (embedded).
- `internal/langregistry/languages.go`, `registry.go`, `entry.go`, `installer.go`:
  embedded language registry + LS installer.

**Core Logic:**
- `internal/kernel/`: kernel orchestrator + workspace runtime; symbol/edit/file/diag
  tools in the named subpackages.
- `internal/semantic/`: extraction → store → enrichment → graph pipeline (18 subpkgs).
- `internal/repomap/`: RepoMap tag extraction, cache, PageRank, renderer.
- `internal/fuzzy/`: fuzzy match cascade used by edit tools.

**Tool Registration:**
- Kernel tools: `RegisterTools(server *mcp.SerenaMCPServer, ...)` in each
  `internal/kernel/<group>/tools.go`.
- Skill tools: `ToolProvider.Tools()` returning `[]*mcp.ToolDef`; registered
  centrally by `internal/daemon/`.

**Generated / Committed Artifacts:**
- `internal/cli/verbs_gen.go` (helix-cligen), `README.md` tool table (docgen),
  `internal/cli/skills/helix/reference.md` (helix-refgen), `protocol/gen/*.go` (lspgen).
  All guarded by drift gates (`verify-cligen`, `verify-docs`, `verify-reference`).

**Testing:**
- `*_test.go` colocated per package (590 files); `testdata/` fixtures per package.
- Real-binary E2E: `internal/cli/*_e2e_test.go` (driven by `HELIX_BIN`).
- Cross-package harnesses under `test/` (bench, harness, integration, oracle).

## Naming Conventions

**Packages / Directories:**
- Lowercase single-word package names: `kernel`, `semantic`, `repomap`, `langregistry`.
- Subsystem subpackages by role: `kernel/lspool`, `kernel/symbols`, `semantic/extract`,
  `semantic/store`, `skill/memory`.
- Command binaries: product is `cmd/helix`; generators are verb-descriptive
  (`cmd/docgen`, `cmd/lspgen`, `cmd/helix-cligen`); architectural gates are
  `cmd/vet-<invariant>` wrapping a matching `internal/lint/<invariant>` Analyzer.

**Files:**
- Snake_case Go filenames: `semantic_wiring.go`, `lazy_init.go`, `setup_hooks.go`.
- Tool registration files: `tools.go` (kernel) / `tools_<name>.go` (skill/semantic).
- Generated files carry `// Code generated by <gen>; DO NOT EDIT.` and end `_gen.go`
  or live under `protocol/gen/`.
- Tests: `<file>_test.go`; real-binary end-to-end: `<name>_e2e_test.go`.

**Types / Identifiers:**
- CamelCase exported types: `SerenaMCPServer` (retained lineage identifier),
  `GrammarRegistry`, `WorkspaceKey`, `ToolRegistry`.
- Middleware types suffixed `Middleware`: `TelemetryMiddleware`, `ProfileFilterMiddleware`,
  `SuggestionMiddleware`, `LazyInitMiddleware`.
- Frozen CLI verbs: kebab-case (`go-to-definition`, `analyze-blast-radius`) = the
  tool name with `_`→`-`; defined only in `verbs_gen.go`.

## Where to Add New Code

**New `helix` verb / tool:**
- Kernel-resident: add the handler under the relevant `internal/kernel/<group>/`,
  register in that group's `tools.go` via `RegisterTools`.
- Skill-resident: add under `internal/skill/<skill>/`, expose through
  `ToolProvider.Tools()`; the skill self-registers via Caddy-style `init()`.
- Regenerate the verb catalog: `go run ./cmd/helix-cligen` (updates `verbs_gen.go`),
  then `go run ./cmd/docgen` and `go run ./cmd/helix-refgen`; the drift gates
  (`verify-cligen`, `verify-docs`, `verify-reference`) enforce the regen.

**New language support:**
- Grammar: add the tree-sitter binding to `internal/treesitter/registry.go`
  `NewGrammarRegistry`.
- Language server: add an entry to `internal/langregistry/` (embedded registry +
  installer wiring).
- Extraction: add a per-language provider under `internal/semantic/extract/` with
  `testdata/` fixtures.

**New semantic edge / analysis:**
- Add a leaf subpackage under `internal/semantic/` (stdlib + tree-sitter only for
  leaf boundaries, mirroring `minhash`/`relatedidx`/`dataflow`), emit facts through
  the extract seam, then wire edge emission in `internal/daemon/semantic_wiring.go`.
- Respect the import boundaries enforced by the `vet-*` gates.

**New architectural invariant:**
- Author a `go/analysis` Analyzer under `internal/lint/<name>/`, wrap it in a
  `cmd/vet-<name>/` singlechecker binary, and wire it into `make vet`.

**Configuration / profiles:**
- Config keys: `internal/config/`. Profile/mode definitions: `internal/profile/`.

## Special Directories

**`api/proto/serena/v1/`:**
- Purpose: gRPC IPC proto between forwarder client and daemon (`StreamMCP`).
- Lineage: the `serena/v1` package-directory name is a **retained wire-format
  lineage artifact** (Phase 52-03), NOT residue — renaming it would break the wire.
- Committed: yes (`ipc.proto` + generated `ipc.pb.go`, `ipc_grpc.pb.go`).

**`protocol/gen/`:**
- Purpose: generated LSP 3.17 Go types (324 structs, 216 union types).
- Generated: by `cmd/lspgen` from `protocol/metaModel.json` + `protocol/patch/`.
- Committed: yes; do not hand-edit.

**`tools/dspy-tune/`:**
- Purpose: dev-time-only Python DSPy tuning harness.
- Shipped: NO — not in the binary, not on the runtime path; uses `uv`/`uvx`,
  `.venv/` git-ignored. Import-isolated from runtime by `vet-tools-quarantine`.

**`bench/`:**
- Purpose: milestone benchmark harness (291 `.go` files) — SWE-bench,
  Multi-SWE-bench, Terminal-Bench, aider-polyglot, repobench, crosscodeeval.
- Container-backed: engine-agnostic, auto-detects Podman or Docker.
- Committed: yes; exercised via `cmd/helix-bench`.

**`docs/`:**
- Purpose: CURRENT reference docs (`edge-types.md`, `type-resolution.md`,
  `runbooks/`), refreshed in v2.13.
- Out of scope for this map; reference only.

**`deploy/grafana/`:**
- Purpose: Grafana dashboards for the `internal/obs` RED metrics.
- Committed: yes.
