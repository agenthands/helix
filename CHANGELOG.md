# Changelog

All notable changes to Serena (Go) are documented here.

## v1.7 — Developer Experience & Auto-Setup (2026-04-22)

### Setup CLI
- One-command MCP registration: `serena setup <client>` for 6 clients — Claude Code, VS Code, JetBrains, Claude Desktop, Gemini CLI, and generic MCP clients
- Automatic project language detection in the working directory and pre-installation of available language servers
- Uses client CLIs as subprocess (e.g., `claude mcp add-json`) rather than writing config files directly
- `--global` flag for user-wide registration; `--no-hooks` opt-out for Claude Code hook installation

### Health & Status
- `get_health` MCP tool returning per-workspace language server status (running, crashed, indexing), capabilities, and indexing progress
- `serena status` CLI command with human-readable summary, `--json` machine-readable output, and `--verbose` detail mode
- Error-only defaults — surfaces only actionable failures unless verbose is requested

### Claude Code Hooks
- Automatic hook installation during `serena setup claude-code`:
  - **SessionStart** hook activates the workspace when a Claude Code session begins
  - **PreToolUse** hook nudges agents toward symbolic tools (`find_symbol`, `get_symbols_overview`) when they overuse grep/read
  - **Stop** hook cleans up session data on exit
- `--no-hooks` opt-out for users who manage their own hook config

### Smart Error Responses
- MCP middleware that enriches parameter typos and enum value errors with "Did you mean?" suggestions via Levenshtein distance
- Suggestions only correct parameters within the same tool — never redirect to a different tool
- Implemented as middleware wrapping the existing typed error taxonomy — error kinds unchanged

### Progressive Descriptions
- Tiered brief/detailed tool descriptions — listings show brief under 100 tokens each
- `get_tool_help <tool_name>` MCP tool for comprehensive on-demand docs with parameters, types, and usage examples
- Golden-file regression gating — no description ships without passing behavioral tool-selection tests

### Lazy Workspace Initialization
- `sync.Once` per workspace path — first MCP tool call transparently activates the workspace
- Concurrent first calls from multiple agents safely serialized (no duplicate initialization or races)
- No upfront indexing delay after setup

## v1.6 — Context Intelligence & Resilient Editing (2026-04-20)

### Fuzzy Editing
- 4-strategy cascade in `internal/fuzzy/`: exact match, whitespace-normalized, indentation-flexible, ellipsis-placeholder
- Common-prefix indentation reflow — preserves indentation when rewriting at different nesting levels
- Ambiguity refusal — ambiguous matches return a structured diff with options rather than silently applying
- Ellipsis placeholder support — `...` in the search block matches across any number of lines
- Strategy reporting in the result envelope so agents can see which strategy matched

### Fuzzy Edit Integration
- Standalone `fuzzy_edit` MCP tool for direct fuzzy match-and-replace
- `replace_in_file` falls back to fuzzy matching when exact matching fails
- `replace_symbol_body` gains a `search_body` parameter for fuzzy body matching before tree-sitter surgery

### RepoMap Context Intelligence
- Tree-sitter tag extraction with per-language `.scm` queries and a shared `GrammarRegistry`
- LSP `documentSymbol` fallback extractor for languages without tree-sitter grammars
- SQLite tag cache with mtime-based invalidation — survives daemon restart
- Scope-aware elision renderer preserves enclosing scope when trimming for token budget
- Cross-file reference graph with hand-rolled PageRank (~60 LOC, no third-party dep, supports personalization)
- Token-budgeted tree renderer using binary search to fit any target budget
- `get_repo_map` MCP tool — ranked structural overview of the repository
- `get_context` MCP tool — given relevant files, returns ranked symbols and definitions across the codebase
- LSP enrichment of tag hover/signature information after PageRank ranking

### Multi-Language Grammar Expansion
- Tree-sitter grammar coverage expanded from 5 to **23 languages** (full aider parity): Java, C, C++, C#, Ruby, PHP, JavaScript, Kotlin, Scala, Bash, Haskell, Julia, OCaml, Lua, Zig, HCL, plus Swift and R via locally vendored bindings
- Both tag and body queries defined per language
- Best-of-breed query merging from aider community collections
- Unit, integration, and Wave 1 LS fixture tests

### Verification & Wiring
- Cache persistence verified end-to-end (daemon restart returns identical output from SQLite cache without re-extraction)
- FallbackExtractor wired through `walkAndExtract` and the daemon post-init path for languages without tree-sitter coverage

## v1.5 — Typed Errors & Hardening (2026-04-15)

### Error Taxonomy
- `internal/errors/` package with 7 error kinds: `NotFound`, `InvalidArgs`, `NoWorkspace`, `Unsupported`, `Internal`, `CircuitOpen`, `Timeout`
- Builder pattern API with `serr.New` / `serr.Wrap` and JSON serialization
- Cause-chain wrapping compatible with `errors.Is` / `errors.As`
- Unified sentinels: `ErrCircuitOpen`, `ErrSessionExpired`, `ErrLSCrashed` migrated into the taxonomy (backward-compatible re-exports, later removed)

### Tool Migration
- All 38+ MCP tools across 10 packages (symbols, edit, fileops, diag, memory, workflow, profile, MCP core) migrated from raw `fmt.Errorf` strings to typed `serr.New` / `serr.Wrap`
- All 24 kernel tool handlers gained inline input validation — empty-string checks on required fields before any workspace or LS work

### Testing
- Three-band error tests upgraded with `extractKind` helper and `expectedKind` struct field for Kind-level assertions
- 4 typed error golden files: `invalid_args`, `no_workspace`, `not_found`, `unsupported`

## v1.4 — Integration Testing v2 (2026-04-14)

### Test Harness
- Importable `test/harness/` package — Runner, golden store, fixture helpers
- Build tag taxonomy: `integration`, `llm`, `llmjudge`

### Oracle Tests
- **Protocol oracle:** MCP handshake, `tools/list` validation, session isolation, reconnect resilience
- **Contract oracle:** JSON Schema Draft 2020-12 validation, 23 golden output files, 6 error category contracts, tool selectability heuristics
- **Scenario oracle:** 14+ full-cycle agent workflow tests across Go, Python, TypeScript, C++, Swift, Zig, JavaScript, PHP, SQL, Markdown, polyglot, unsupported, collision, and degraded fixtures

### LLM Behavioral Tests
- Tool selection across 33 tools, 11 disambiguation pairs, output interpretation
- Multi-provider support (Anthropic + DeepSeek)
- Judge scoring with 5-dimension rubrics, transcript pipeline, aggregate reporting — never blocks merge

## v1.3 — Documentation Catchup (2026-04-11)

### README & Observability Docs
- README **Production & Observability** section documenting metrics, tracing, admin endpoints, and graceful degradation
- Go-native CONTRIBUTING.md replacing the legacy Python contribution guide

### Install & Usage Guides
- USAGE.md accuracy fixes plus a benchmarks subsection
- INSTALL.md with per-agent MCP config sections for Claude Code, Codex, OpenCode, Cursor, Gemini CLI, Antigravity, and HTTP mode

### Changelog
- CHANGELOG.md v1.2 gap fill (items missed during the v1.2 writeup)

## v1.2 — Performance & Production Hardening (2026-04-10)

### Benchmark Harness
- Benchmark suite in `test/bench/` using `testing.B.Loop` for all MCP tools
- Benchstat CI regression gate with tiered thresholds (PR: >15% time / >25% allocs; release: >10% / >20%)
- Committed v1.1 baselines for p50/p95/p99 tool latency, LSP indexing, and memory profiles

### Observability
- `internal/obs/` package with noop-default provider (zero-cost when disabled)
- Trace-aware slog handler: log records carry `trace_id` and `span_id` from context
- Dedicated loopback admin listener with `/healthz`, `/readyz`, and gated `/debug/pprof/*`
- Prometheus `/metrics` endpoint with RED histograms per tool and lspool gauges
- Bounded-label contract enforced by CI lint (allowlist: tool_name, profile, mode, language, outcome)

### Tracing
- End-to-end trace propagation: forwarder -> daemon -> kernel -> LS via `otelgrpc`
- Telemetry middleware with per-tool sub-spans
- Optional OTLP/gRPC exporter behind config flag
- Default sampler `ParentBased(TraceIDRatioBased(0.0))` — off by default

### Graceful Degradation
- Per-class timeout budgets (read 5s / search 15s / edit 10s / index 120s / diagnostics 20s)
- Deadline propagation from forwarder through daemon and kernel to language server
- Typed `lspool.ErrCircuitOpen` with structured error envelope
- Circuit breaker tuning: decorrelated jitter backoff, single-probe half-open
- LS crash recovery with configurable restart budget
- `runtime/debug.SetMemoryLimit` wired from config
- Graceful shutdown: SIGTERM drains in-flight calls, flushes telemetry within 5s

### Documentation
- Auto-generated tool and language tables in README.md via `cmd/docgen`
- USAGE.md: profiles, modes, tutorials, troubleshooting, observability quickstart, performance tuning
- This changelog

### Benchmark Gate Hardening
- `capture-baseline.yml` workflow for on-demand baseline capture on GitHub-hosted runners
- Benchstat CI gate enforces blocking thresholds (removed `--warn-only` flag)
- Real ubuntu-latest baseline numbers committed to `test/bench/baselines/`

## v1.1 — Integration Testing (2026-04-09)

### Test Harness & Dogfooding
- MCP round-trip integration harness (`test/integration/`)
- All 38 tools exercised via stdio transport against live Go fixture
- Language field gap closure for multi-language routing
- 10 real production bugs discovered and fixed during dogfooding

### Symbol Editing
- Edit round-trip integration tests (replace_symbol_body, insert_before/after, rename, safe_delete)
- Python, TypeScript, Java, and Rust multi-language fixtures
- Two-phase lease pattern for edit operations (clean for planning, dirty for mutation)

### Advanced Testing
- Profile/mode golden contract tests (5 profiles x 4 modes, 19 golden files)
- Three-tier concurrency coverage: t.Parallel(), errgroup fan-out, testing/synctest
- Three-band error classification tests (30 cases across categories and destructive tools)
- SessionInfo data race fix with RWMutex + Snapshot pattern
- ProfileFilterMiddleware access control gap fix for initial sessions

## v1.0 — MVP (2026-04-08)

Initial release of Serena as a Go-native code intelligence platform for MCP.

### Core
- Single Go binary with persistent daemon, gRPC forwarder, and stdio/HTTP transports
- MCP server with official Go SDK, tool registry, and profile filtering middleware
- Signal-first lifecycle with errgroup orchestration

### Code Intelligence
- LSP 3.17 protocol support with generated types (324 structs, 216 union types)
- Worker pool with share-until-dirty semantics, adaptive TTL, circuit breaking, and memory pressure eviction
- 9 symbol retrieval tools (definition, references, hover, implementations, call/type hierarchy, blast radius)
- 6 symbol editing tools with tree-sitter body surgery and post-edit diagnostic verification
- 6 file operation tools (read, write, list, find, search, replace)
- 3 diagnostic tools (diagnostics, code actions, formatting)

### Skills & Multi-Language
- 52-language embedded registry with YAML override and three-tier LS installer
- 7 memory tools with markdown storage and SQLite FTS5 search
- Workflow tools (onboarding, session handoff)
- Caddy-style skill plugin system with init() registration

### Agent Profiles
- 5 pre-built profiles: claude-code, codex, ide-assistant, ci-bot, full
- 4 operational modes: read, edit, review, admin
- 4-layer config precedence: CLI > project > user > profile defaults
- Token budget awareness for context optimization
