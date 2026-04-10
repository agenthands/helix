# Milestones

## v1.2 Performance & Production Hardening (Shipped: 2026-04-10)

**Phases completed:** 15 phases, 56 plans, 78 tasks

**Key accomplishments:**

- Python code migrated to legacy/, Go module initialized with cobra CLI skeleton producing single serena binary
- 1. [Rule 1 - Bug] Fixed Unix socket path length on macOS
- MCP server with official SDK, dummy tools (ping/echo/activate_project), gRPC forwarder-daemon IPC, stdio forwarder with auto-start, and Streamable HTTP endpoint
- Full LSP 3.17 type generation from metaModel.json (324 structs, 216 union types) plus Content-Length framed JSON-RPC 2.0 codec with session-prefixed ID routing
- 6 pure Go file operation tools (read, write, list, find, search, replace) with symlink-aware path security and MCP registration
- Multi-LS worker pool with share-until-dirty policy, adaptive TTL, circuit breaking, pressure eviction, and workspace-scoped language detection
- 1. [Rule 2] Added LeaseProvider abstraction
- Tree-sitter body extraction for 4 languages with 6 symbol editing MCP tools and automatic post-edit diagnostic verification
- 52-language embedded registry with YAML deep-merge overlay and three-tier LS installer (PATH/download/error)
- Markdown-based memory CRUD with SQLite FTS5 search, project/global scoping, and fsnotify auto-reindex
- Go skill/plugin interfaces with init()-based registry and YAML-driven context/mode composition for tool filtering
- QuirkAdapter interface replacing hardcoded LanguageQuirks with per-language behavioral hooks, wired to language registry for LS resolution
- Memory skill wrapping MemoryStore as 7 MCP tools and workflow skill with onboarding project analysis and session handoff via Caddy-style init() registration
- Profile/Mode types extending skill specs, 5 agent profile YAMLs and 4 mode YAMLs embedded via go:embed, loader with override merging and 6 tests
- switch_mode and get_token_budget MCP tools via profile skill with per-session mode tracking and transition validation
- Profile selection wired through 4-layer koanf config precedence with ProfileFilterMiddleware applying tool filtering and description overrides on MCP tools/list responses
- 1. [Rule 3 - Blocking] daemon.New signature change required caller updates
- 10 integration tests proving daemon bootstrap registers 38 tools, initializes 7 skills, resolves profiles, and all 6 E2E flows work end-to-end
- In-process daemon test harness with MCP InMemoryTransports, Go fixture project (8 symbols), and build-tag-gated self-tests
- Commit:
- 1. [Rule 1 - Bug] Corrected tool parameter names
- 1. [Rule 1 - Bug] Worker process killed when request context completes
- 1. [Rule 3 - Blocking] Edit tools did not resolve relative paths against workspace root
- 1. [Rule 3 - Blocking] Added requireLS helper inline
- 1. [Rule 2 - Missing] Added requireLS to helpers.go instead of inline
- 1. [Rule 3 — Blocker] switch_mode arg name mismatch in harness (08-01 inherited)
- 1. [Rule 1 - Bug / T-08-08 mitigation] Data race on SessionInfo under concurrent switch_mode + tool invocation
- File:
- 1. [Rule 3 - Blocking] Parity assertion source: session.ListTools → Registry().Names()
- 1. [Rule 3 - Blocking] Bumped worker pool cap in `bench_helpers_test.go`
- Task 1 — Cold + warm LSP indexing benchmarks (`test/bench/lsp_index_bench_test.go`)
- BenchmarkMemory walks 4 D-06 scenarios with dual Go/kernel RSS reporting and per-scenario pprof heap snapshots, locking the v1.1 memory baseline for BENCH-04.
- Commit:
- Stdlib-only obs package with noop Provider, zero-alloc trace-aware slog ContextHandler, ObservabilityConfig schema, and --admin-addr CLI flag wired through runDaemon and runForwarder.
- OBS-03 (loopback admin listener)
- OBS-06 measured and locked: obs.ContextHandler adds 0 alloc/op over a bare slog.JSONHandler on the no-span fast path, with a committed v1.2 baseline for Phase 11 to measure against.
- `github.com/prometheus/client_golang v1.23.2`
- 1. [Rule 3 - Blocking] Added go.sum entry for prometheus testutil dependency
- 1. [Rule 3 - Blocking] daemon.go observability construction order
- 1. otelgrpc not pinned in go.mod
- RED:
- 1. [Rule 3 - Blocking] fileops/diag tracer parameter approach
- BenchmarkTracingOffPath
- 1. [Rule 3 - Blocking] DegradationConfig struct created in Task 1 instead of Task 2
- 1. [Rule 3 - Blocking] Bench tests missing BudgetFunc parameter
- Go-only CHANGELOG.md with v1.0 MVP, v1.1 Integration Testing, and v1.2 Performance & Production Hardening entries dated from milestone data
- Commit:

---

## v1.1 Integration Testing (Shipped: 2026-04-09)

**Phases completed:** 3 phases (6-8), 11 plans, 64 commits
**Lines changed:** 12,615 insertions, 1,149 deletions across 116 files
**Timeline:** 2 days (2026-04-08 → 2026-04-09)

**Key accomplishments:**

- Integration test harness with MCP `NewInMemoryTransports()` and HTTP `StreamableClientTransport` — in-process daemon spawning, LS readiness polling, build-tag-gated separation (`//go:build integration`)
- Go dogfooding suite exercising all 38 MCP tools against Serena's own 25,779 LOC codebase with strict structural assertions
- Edit round-trip testing for all 6 symbol editing tools (replace body, insert before/after, rename, safe delete, verify edit) with tree-sitter body surgery validated end-to-end
- Multi-language fixture projects for Python, TypeScript, Java, and Rust with known symbols, cross-file reference chains, and per-language LS skip logic via `requireLS`
- Profile/mode contract tests using golden file pattern (19 `.tools.golden` files) with `-update` flag and diff-reviewable tool visibility assertions
- Three-tier concurrency testing: `t.Parallel()` scenario stress + `errgroup` fan-out on worker pool + `testing/synctest` deterministic unit tests for `WorkerMetrics` decay
- Three-band error path coverage: category matrix (shared harness) + exhaustive per-tool for 9 destructive tools + thin smoke for read-only tools (30 cases total)
- **10 production bugs found and fixed during dogfooding** — including `ProfileFilterMiddleware` unfiltered initial sessions (access control), `SessionInfo` data race on concurrent mode switches (fixed with `sync.RWMutex` + Snapshot pattern), Language field routing causing circuit breaker failures, and 5 edit tool bugs (path resolution, DocumentSymbol ranges, rename DocumentChanges, SafeDelete SelectionRange, two-phase lease pattern)
- `make test-stress` target for elevated-count concurrency fan-out in CI

---

## v1.0 MVP (Shipped: 2026-04-08)

**Phases completed:** 5 phases, 20 plans, 91 commits
**Lines of Go:** 25,779 across 21 packages
**Timeline:** 2 days (2026-04-07 - 2026-04-08)

**Key accomplishments:**

- Go rewrite with daemon skeleton, MCP runtime, gRPC forwarder, and Streamable HTTP transport — single binary distribution
- Full LSP 3.17 type generation from metaModel.json (324 structs, 216 union types) plus JSON-RPC 2.0 codec with session-prefixed ID routing
- 24 MCP tools for symbol retrieval (9), symbol editing (6), file operations (6), and diagnostics (3) — backed by live language servers with tree-sitter body surgery
- Multi-LS worker pool with share-until-dirty policy, adaptive TTL, circuit breaking, and platform-aware pressure eviction
- 52-language embedded registry with YAML deep-merge overlay and three-tier LS installer (PATH/download/error)
- Markdown-based memory system with SQLite FTS5 search, 7 MCP tools, and fsnotify auto-reindex
- Skill/plugin interfaces with Caddy-style init() registration, onboarding workflow, and session handoff
- 5 agent profiles (Claude Code, Codex, IDE assistant, CI bot, full), 4 modes (read/edit/review/admin), token budget awareness, profile filtering middleware
- Full daemon bootstrap wiring: 38+ callable MCP tools, centralized registration, fail-fast startup, kernel-first shutdown

---
