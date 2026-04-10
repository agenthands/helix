# Changelog

All notable changes to Serena (Go) are documented here.

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
