# Architecture

**Analysis Date:** 2026-07-01

## Pattern Overview

**Overall:** Helix is a Go-native, CLI-first code-intelligence platform shipped as a
**single Go binary** (`cmd/helix/main.go`) with no runtime Python, Docker, or LSP
dependencies. It runs as a **persistent supervisor daemon** (`internal/daemon/`)
that keeps language servers warm between agent sessions and exposes symbol-level
operations as `helix <verb>` commands. The design is a **4-layer architecture**
(MCP Runtime → Kernel → Skills & Multi-Lang → Profiles & Setup) with a
first-class **semantic-graph index** (`internal/semantic/`, ~30.8k LOC) built
alongside the kernel.

**Key Characteristics:**
- **Single binary, warm daemon**: one process supervises the workspace registry,
  kernel, skills, and listeners; survives client disconnects (`internal/daemon/daemon.go`).
- **CLI-first surface**: agents drive 51 frozen `helix` verbs over gRPC; the MCP Go
  SDK + gRPC IPC are internal plumbing, not an agent-facing surface (`internal/cli/root.go`).
- **Skill-driven tool set**: kernel tools + `init()`-registered skills compose the
  tool registry without touching core (`internal/skill/`).
- **Multi-language via LSP + tree-sitter**: 52-language registry
  (`internal/langregistry/`), 23 tree-sitter grammars (`internal/treesitter/registry.go`),
  11 first-class semantic extractors.
- **Semantic graph**: typed, language-agnostic edge graph over code symbols, backed
  by a DuckDB store (`internal/semantic/store/`).
- **Architectural boundaries enforced by custom vet gates**: kernel↔semantic import
  boundary pinned in both directions (`make vet`).

## Layers

**Layer 0 — MCP Runtime:**
- Purpose: MCP server, middleware stack, persistent daemon supervision, gRPC IPC.
- `internal/mcp/` — MCP server over the official Go SDK (`server.go`,
  `SerenaMCPServer` type — Go identifier retained for internal-API stability,
  Phase 52-03), tool registry (`registry.go`), and the middleware stack (below).
- `internal/daemon/daemon.go` — persistent supervisor (`Daemon` struct); errgroup
  orchestration, signal-first lifecycle, kernel-first shutdown; bootstraps every
  subsystem (see Data Flow).
- `internal/forwarder/` — stdio-to-gRPC proxy with daemon auto-start.
- `api/proto/serena/v1/` — gRPC IPC proto (`ipc.proto`, `ForwarderService`);
  package directory name `serena/v1` retained as a wire-format lineage artifact
  (Phase 52-03), NOT residue.
- Transport: the agent-facing surface is the `helix` CLI dialing the daemon over
  gRPC `StreamMCP` (unix-domain socket by default; opt-in loopback-gated gRPC TCP).
  The stdio MCP forwarder head and the Streamable-HTTP `/mcp` head were removed in
  Phase 94 — only the internal gRPC `StreamMCP` wire remains.

**Layer 1 — Code Intelligence Kernel (`internal/kernel/`):**
- Purpose: warm language-server orchestration and symbol/edit/file/diagnostic tools.
- `internal/kernel/kernel.go` — kernel orchestrator; `workspace.go` — workspace runtime + language detection.
- `internal/kernel/lspool/` — LS worker pool: share-until-dirty (`pool.go`),
  adaptive TTL with reuse scoring, circuit breaking (`circuit.go`), platform-aware
  memory-pressure eviction (`pressure_linux.go`, `pressure_darwin.go`).
- `internal/kernel/symbols/` — 9 symbol tools (definition, references, hover,
  implementations, call/type hierarchy, blast radius) in `tools.go`, `hierarchy.go`, `blast.go`.
- `internal/kernel/edit/` — 6 edit tools with tree-sitter body surgery (`tools.go`, `rename.go`, `replace.go`).
- `internal/kernel/fileops/` — 7 file tools (read/write/list/find/search/replace/fuzzy_edit).
- `internal/kernel/diag/` — 3 diagnostic tools (diagnostics, code actions, formatting).
- `internal/kernel/jsonrpc/` — JSON-RPC 2.0 codec for LS communication.
- `internal/kernel/health/`, `internal/kernel/help/` — `get_health` / `get_tool_help`
  MCP tools, wrapped as skills via each package's `skill_adapter.go`.
- Supporting: `internal/fuzzy/` (4-strategy fuzzy cascade), `internal/repomap/`
  (tag extraction, SQLite tag cache, PageRank, token-budget renderer),
  `protocol/gen/` (generated LSP 3.17 types).

**Layer 2 — Skills & Multi-Language (`internal/skill/`):**
- Purpose: extend the tool set without touching core; multi-language registry.
- `internal/skill/skill.go` — `Skill` / `ToolProvider` / `WorkflowProvider`
  interfaces; `registry.go` — Caddy-style `init()` registration.
- `internal/skill/memory/` — 7 memory tools over markdown + SQLite FTS5.
- `internal/skill/workflow/` — onboarding and session-handoff prompts.
- `internal/skill/repomap/` — `get_repo_map` / `get_context` wrapping the RepoMap engine.
- `internal/skill/semantic/` — the semantic-graph MCP tools (11 tools registered by
  `register.go` `RegisterAll`).
- `internal/skill/guardrails/` — guardrail skill surface.
- `internal/memory/` — memory store, FTS5 index (`index.go`), fsnotify watcher.
- `internal/langregistry/` — 52-language embedded registry (`languages.go`), YAML
  override (`registry.go`), three-tier LS installer (`installer.go`).

**Layer 3 — Profiles & Setup (`internal/profile/`, `internal/config/`, `internal/cli/`):**
- Purpose: agent profiles, layered config, client setup.
- `internal/profile/` — 5 agent profiles (claude-code, codex, ide-assistant, ci-bot,
  full) plus ablation profiles (`profiles/*.yaml`) and 4 modes (read/edit/review/admin, `modes/*.yaml`).
- `internal/config/` — 4-layer koanf config: CLI > project `.helix/` > user
  `~/.helix/` > profile defaults (`loader.go` `Load`).
- `internal/cli/setup*.go` — `helix setup <client>` across 8 registrars
  (`setup_clients.go`); `internal/cli/status*.go` — `helix status`.

**Semantic-Graph Subsystem (`internal/semantic/`, first-class, ~30.8k LOC):**
- Purpose: a typed, language-agnostic edge graph over code symbols, built and
  maintained daemon-side; consumed by Layer-2 semantic MCP tools through a
  types-only seam (`internal/semantic/integ/`).
- Pipeline stages (**extract → store → enrich → emit**):
  - **extract** (`internal/semantic/extract/`, ~8.4k LOC): per-language tree-sitter
    symbol/reference/import/type/heritage extraction; 11 first-class extractors
    (Go, TypeScript/JavaScript, Python, Java, C#, Rust, C, C++, Kotlin, PHP, Ruby),
    each `extract/<lang>/` with a `queries.scm`.
  - **store** (`internal/semantic/store/`, ~5.2k LOC): DuckDB-backed graph store;
    SOLE owner of the `duckdb-go` import (`duckdb.go`, D-12); snapshot ⊕ overlay −
    tombstone effective-read model; three-tier open (open / quarantine+rebuild / hard-fail).
  - **enrich** (`internal/semantic/lspenrich/`, ~2.7k LOC): LSP enrichment of
    extracted facts via warm LS leases (`worker.go`), budget-bounded cascade.
  - **types** (`internal/semantic/types/`, ~3.4k LOC): per-language type resolvers
    (C-family tiered resolvers, fixpoint chains).
  - **emit / graph**: `graph/` (graph_version advance, read-time score_status),
    `compact/` (snapshot compaction), `relatedidx/` (Random Indexing →
    SEMANTICALLY_RELATED), `dataflow/` (case-1 + in-body flow → DATA_FLOWS),
    `minhash/` (near-clone → SIMILAR_TO), `classifier/` (call classification),
    `retrieval/`, `scheduler/`, `live/` (fsnotify-driven live index), `cluster/`,
    `crossrepo/`, `cochange/`.
- Edge kinds emitted: DEFINES/contains/imports/implements/extends,
  calls/references/has_type/uses_type, DATA_FLOWS (def_use / def_use_inbody /
  def_use_return), SEMANTICALLY_RELATED, SIMILAR_TO, STRUCTURAL_TWIN, and call
  classifications (http_calls/async_calls/emits/listens_on/...). Read surface and
  full ledger: `docs/edge-types.md`.
- Boundary: the kernel may import only `internal/semantic/integ/` (types-only);
  enforced statically by `vet-nokernel2semantic` / `vet-nosemantic2kernel`, and the
  duckdb import is confined by `vet-noduckdb` / `vet-compact-uses-store`.

**MCP Middleware Stack (`internal/mcp/`, installed in `internal/daemon/daemon.go`):**
- `TelemetryMiddleware` (`middleware.go`) — RED metrics on `tools/call`, per-tool
  deadlines via `BudgetFunc`, outcome classification (success / timeout /
  circuit_open / internal).
- `ProfileFilterMiddleware` (`middleware.go`) — filters `tools/list` by active
  profile and applies brief + profile-specific descriptions on the same pass.
- `SuggestionMiddleware` (`suggest.go`) — enriches parameter-typo / enum errors with
  Levenshtein "Did you mean?" suggestions; never redirects.
- `GuardrailMiddleware` (`guardrail_middleware.go`) — gates destructive tool calls on
  guardrail receipts.
- `ProfileEnforcementMiddleware` (`profile_enforce.go`) — refuses out-of-profile
  `tools/call` with a typed `PermissionDenied` error (Phase 91 SEC-01).
- `LazyInitMiddleware` (`lazy_init.go`) — `sync.Once` per workspace path; activates
  the workspace on first tool call; installed LAST so it runs FIRST.

## Data Flow

**Daemon bootstrap (`internal/daemon/daemon.go` `newDaemon`):**
1. Build config-derived subsystems: language registry + three-tier installer, kernel
   with LS pool, `GrammarRegistry` (23 grammars), SQLite `TagCache`.
2. Open the semantic store when `cfg.SemanticIndex.Enabled` (three-tier open); wire
   the semantic bundle, scheduler, and live index.
3. Blank-import all skill packages (`imports.go`) for `init()` registration; call
   `skill.InitAll(deps)`; register kernel tools + skill tools centrally with the MCP SDK.
4. Post-init wiring: `SetEnrichFn` (RepoMap LSP enrichment), workspace activation
   callback, fallback extractor for languages without tree-sitter coverage,
   `semantic.RegisterAll` for the semantic MCP tools.
5. Install middleware in order (step 14 `InstallMiddleware` → 14b Suggestion →
   14b.5 Guardrail → 14b.6 ProfileEnforce → 14c LazyInit); resolve active profile.
6. Fail-fast for core subsystems (D-06); degrade gracefully for optional providers (D-07).

**Middleware execution (LIFO):** `AddReceivingMiddleware` composes in LIFO order, so
the on-request execution order is the reverse of the install order:

`LazyInitMiddleware` → `ProfileEnforcementMiddleware` → `GuardrailMiddleware` →
`SuggestionMiddleware` → `ProfileFilterMiddleware` (applies brief descriptions on
`tools/list`) → `TelemetryMiddleware` → tool handler.

LazyInit MUST run first so the workspace is activated before `TelemetryMiddleware`
applies its per-tool deadline (install-order comment at
`internal/mcp/lazy_init.go:106-112`); ProfileEnforce runs before Guardrail so an
out-of-profile call is refused before receipt evaluation
(`internal/mcp/profile_enforce.go:16-23`). Any change to this order must preserve
both invariants.

**Verb execution:** an agent runs `helix <verb>` → `internal/cli/root.go` maps the
verb (via generated `verbSpecs`, `verbs_gen.go`) to a tool name and args → dials the
daemon over gRPC `StreamMCP` (`forwarder.CallTool`) → the daemon's MCP server routes
through the middleware chain to the tool handler → typed result string returned to
the CLI, which prints it `relpath:line:col`-anchored.

**Session lifecycle (`forwarderServiceHandler.StreamMCP`):** on first message the
daemon emits `(started, stdio)`, wraps the gRPC stream in a `GRPCTransport`, calls
`mcpServer.SDK().Connect`, and waits; emits `(ended)` on clean close or `(error)` on
Connect/Wait failure.

**Semantic index build:** on workspace activation the scheduler
(`internal/semantic/scheduler/`) runs the initial extraction walk (RepoMap PageRank
priority); file changes drive incremental extraction via the live pipeline
(`internal/semantic/live/`, fsnotify + kernel `EditNotifier`); facts land in the
DuckDB overlay, are enriched by LSP, and compacted into snapshots. Consumers read
through `integ.SemanticLookup` (read-only seam) which returns effective facts
(snapshot ⊕ overlay − tombstones).

## Key Abstractions

**Daemon (`internal/daemon/daemon.go`):**
- Purpose: the persistent supervisor owning registry, MCP server, kernel, skills, listeners.
- Pattern: `New`/`NewWithObsProvider` → `Run(ctx)` blocks until shutdown; signal
  handlers registered first; kernel-first shutdown ordering.

**Kernel (`internal/kernel/kernel.go`):**
- Purpose: workspace-runtime orchestrator over the warm LS worker pool.
- Pattern: `ActivateWorkspace(ctx, path)` returns a runtime; tools acquire warm LS
  leases from `lspool`.

**Skill / ToolProvider / WorkflowProvider (`internal/skill/skill.go`):**
- Purpose: reusable capability packages contributing MCP tools and/or prompts.
- Pattern: register via `init()` → `skill.Register(&MySkill{})`; the daemon calls
  `skill.InitAll(deps)`, then `skill.ToolProviders()` / `skill.WorkflowProviders()`.
- Kernel-resident tools (`health/`, `help/`) are exposed as skills via thin
  `skill_adapter.go` wrappers so the ToolProvider surface stays uniform.

**LS Worker Pool (`internal/kernel/lspool/pool.go`):**
- Purpose: keep language servers warm across sessions.
- Pattern: share-until-dirty (clean sessions share a warm worker), adaptive TTL with
  reuse scoring, circuit breaking with backoff for crashy workers, platform-aware
  memory-pressure eviction.

**Semantic Store (`internal/semantic/store/duckdb.go`):**
- Purpose: durable, per-workspace semantic graph in DuckDB.
- Pattern: sole `duckdb-go` owner; three-tier open (open / quarantine+rebuild /
  hard-fail); effective reads = snapshot ⊕ overlay − tombstones; per-snapshot dense
  EdgeIDs.

**SemanticLookup seam (`internal/semantic/integ/`):**
- Purpose: types-only, read-only boundary between the daemon-resident engine and MCP
  consumers; keeps kernel free of DuckDB and `internal/semantic/*` concretions.
- Pattern: `SemanticLookup` interface + `NoopLookup{}` default; every error classified
  by `ClassifyLookupErr` before reaching the wire.

**Language Registry + Installer (`internal/langregistry/`):**
- Purpose: 52-language LS catalog with YAML overrides and a three-tier installer.
- Pattern: `NewRegistry(overridePaths...)` layers embedded `defaultEntries` under YAML
  overrides; `Installer.Resolve` tries PATH → managed download (npm/pip/cargo/gem/dotnet/binary)
  → helpful error.

## Entry Points

**CLI (`cmd/helix/main.go` + `internal/cli/root.go`):**
- Trigger: user/agent runs `helix ...`.
- `main.go` threads the ldflag-injected version into the CLI and MCP identity, builds
  the cobra root (`NewRootCommand`), and maps typed exit codes via `ExitCodeForError`.
- `root.go` mounts all subcommands (setup, status, activate, deactivate, nudge,
  daemon `--serve`) and the generated per-verb catalog (`verbs_gen.go`).

**Daemon (`internal/cli/root.go` `runDaemon` → `internal/daemon/daemon.go` `Run`):**
- Trigger: `helix --serve` (or auto-start via the forwarder).
- Responsibilities: load layered config, build the structured logger, bootstrap all
  subsystems, listen on a unix socket, serve `ForwarderService` over gRPC.

**gRPC `ForwarderService` (`api/proto/serena/v1/ipc.proto`):**
- Trigger: a `helix` verb dials the daemon.
- RPCs: `StreamMCP` (bidirectional MCP JSON-RPC), `GetStatus`, `ActivateWorkspace`, `DeactivateWorkspace`.

## Error Handling

**Strategy:** typed error taxonomy for programmatic matching by agents, with graceful
degradation across subsystems.

**Patterns:**
- **Typed errors (`internal/errors/`):** `Error{Kind, Message, Tool, Detail, cause}`
  with a closed `Kind` enum (`not_found`, `invalid_args`, `no_workspace`,
  `unsupported`, `internal`, `circuit_open`, `timeout`, `permission_denied`,
  `guardrail_violation`). Callers match with `errors.Is(err, serr.ErrNotFound)`;
  imported under alias `serr`. `MarshalJSON` renders errors onto the MCP wire.
- **Guardrail violations:** `NewGuardrailViolation` wraps `*Error` with a typed
  `GuardrailViolationDetail` (required receipt classes + `see_also` remediation),
  recoverable via `AsGuardrailViolation` / `errors.As`.
- **Exit codes:** `cli.ExitCodeForError` parses the typed `<kind>: msg` prefix into a
  frozen per-kind exit code (`cmd/helix/main.go`).
- **Subsystem disable / ablation:** disabled subsystems return `Unsupported` with a
  greppable `subsystem_disabled:` message prefix (`internal/errors/kinds.go`).
- **LS worker crashes:** `lspool` circuit-breaks crashy workers with exponential
  backoff and evicts under memory pressure rather than crashing the daemon.
- **Semantic store corruption:** three-tier open quarantines a corrupt DB
  (`<path>.corrupt.<ts>`) and rebuilds; a rebuild failure is a hard-fail that refuses
  daemon start (`internal/semantic/store/doc.go`).
- **Bootstrap:** fail-fast for core subsystems (D-06); optional providers degrade to a
  usable subset (D-07).

## Cross-Cutting Concerns

**Observability (`internal/obs/`):**
- `Provider` is the single home for `prometheus/client_golang` and
  `go.opentelemetry.io/otel` imports; `Noop` returns a non-nil provider with a real
  metrics sink and a no-op tracer, so call sites never nil-check.
- RED metrics on `tools/call` via `TelemetryMiddleware`; OTLP/gRPC trace export when
  configured; `ContextHandler` injects `trace_id`/`span_id` into slog records.

**Logging:**
- `log/slog` structured logging; `internal/cli/root.go` `newLogger` builds a
  text/JSON handler to stderr wrapped in `obs.NewContextHandler` for trace correlation.
  Level configurable via config; no third-party logging framework in the shipped path.

**Guardrails (`internal/guardrails/`):**
- Rule engine (`rules/` — g001 rename-by-grep, g002 delete-without-refs, g004
  large-fuzzy-edit, g005 security-sensitive) issuing receipts; enforced by
  `GuardrailMiddleware`; per-language catalogs (`catalogs/*.yaml`).

**Degradation (`internal/degrade/`):**
- Token/output budgeting (`budget.go`) so oversized tool results degrade gracefully
  rather than overflow.

**Concurrency:**
- Daemon runs subsystems under an errgroup with signal-first lifecycle; LS workers run
  as isolated subprocesses; `LazyInitMiddleware` serializes concurrent first-calls per
  workspace via `sync.Once`.

**Architectural boundary enforcement (`make vet`):**
- 7 custom `cmd/vet-*` analyzers pin import boundaries (kernel↔semantic both
  directions, duckdb confinement, compact→store, ablation/bench leakage, tools
  quarantine) on every vet run.

---

*Architecture analysis: 2026-07-01*
