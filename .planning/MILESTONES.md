# Milestones

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
