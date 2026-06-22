# Milestones

## v2.0 v2.0 (Shipped: 2026-06-22)

**Phases completed:** 7 phases, 20 plans, 41 tasks

**Key accomplishments:**

- Per-socket gofrs/flock startup lock with double-checked tryConnect in `ConnectOrStartDaemon`, proven by a synctest fan-out test to spawn exactly one daemon under N concurrent cold callers (CLI-03).
- Task 1 (TDD): No-arg helix → grouped help, exit 0, no stdio session (CLI-04)
- `helix call <verb> --flag=val` issues a single MCP `tools/call` through the warm daemon over the EXISTING gRPC `StreamMCP` wire via a client-side transport mirror + the MCP SDK client (initialize handshake, not hand-framed JSON-RPC), with race-safe cold auto-start (90-01), ordered clean teardown, and zero proto changes (CLI-01/CLI-02).
- 1. [Rule 1 - Bug] `search` verb mapped to a non-existent tool
- A go/packages+AST generator (`cmd/helix-cligen`) that emits a committed `internal/cli/verbs_gen.go` of 50 capability-grouped `helix <verb>` subcommands — one per live-registry tool — drift-gated by `helix-cligen --check`, plus the exported `VerbToolNames()` seam for 91-03.
- Server-side `tools/call` authz (ProfileEnforcementMiddleware) that refuses any tool outside the session's resolved AllowedTools with a typed `serr.PermissionDenied`, installed between Guardrail and LazyInit so LazyInit-first LIFO is preserved.
- 1. [Rule 3 - Blocking] Flat verb name and flag differed from the plan's assumptions
- Three pure, dependency-free foundation units for the Phase 92 terse renderer: a 50-verb render-class map, a locus parse + path-normalization + sort/dedup core, and the 9-kind serr.Kind → frozen exit-code + stderr-prefix mapper — all built TDD with no cobra/network dependency.
- Wired the 92-01 foundation (render class, locus core, exit-code mapper) into the live verb path: `renderResultFor` now emits sorted+deduped `relpath:line:col<TAB>payload` for locus-list verbs with a workspace-clamped CLI-side snippet for bare nav loci, passes tree/opaque through verbatim, gates color up front, adds persistent `--color`/`--abs`/`--json` flags every generated verb inherits, preserves the daemon's typed `<kind>:` error so `main.go` exits with the per-kind code, and adds color/abs to the cligen denylist — the output shape FREEZES here.
- Re-pointed the TEST-02 contract oracle from MCP `TextContent` goldens to REAL `helix <verb> --flags` subprocess stdout — per-verb goldens now freeze the terse `relpath:line:col<TAB>payload` shape (plus `--abs` absolute and `--color=never` zero-ANSI variants) against the binary; the MCP schema meta-validation is replaced by an untagged default-suite typed-args→cobra-flags parity test; and a behavioral chain proves a nav locus feeds a downstream verb verbatim (OUT-04) with a self-contained snippet (OUT-03).
- 1. [Rule 3 - Blocking] Repaired a corrupt local Go module cache (environmental, not code)
- Repurposed the PreToolUse nudge from a generic 5-call "use find_symbol" tip into a per-call advisory that steers grep/sed/cat/find over positively-identified CODE targets toward the frozen Phase 92 `helix` verbs via `hookSpecificOutput.additionalContext` JSON — fail-open and exit-0 on prose/log/config, unparseable, and no-operand commands.
- Task 1 — SKILL-04 dependency-free idle-cost bound + filled token-note
- Landed the CLI-is-sole-sufficient-surface proof (TestCLI_DualRunParity) and the loopback-gated gRPC-TCP opt-in that replaces the to-be-deleted HTTP /mcp head's network reach — both green with BOTH agent-facing heads still alive, zero new deps, zero proto change.
- Deleted both agent-facing MCP heads — the stdio forwarder (RunForwarder) and the HTTP /mcp listener (listenHTTP/HTTPHandler/httpSessionMiddleware) — plus the dead RunStdio and the entire --http-addr thread, leaving the CLI's per-verb gRPC dial path as the sole agent surface; the retained E2E + RETIRE-03 parity tests stay GREEN post-deletion, with zero proto change and zero new deps.
- Re-keyed the auto-generated README tool table to `helix <verb>` names and closed the v1.12 docgen-drift hole by wiring `go run ./cmd/docgen --check` into both `make verify-docs` and CI.
- Rewrote Helix's identity from MCP-primary to CLI-first across README/CLAUDE.md/PROJECT.md and added a Helix-CLI tool-routing decision matrix to CLAUDE.md citing the 50 real frozen verbs, with the external SMTC matrix left byte-for-byte intact.
- Closed the four non-blocking v2.0 audit items: hardened the admin listener against wildcard binds, removed the dead mergeJSONConfig helper, stopped the Bash classifier from treating a grep PATTERN as a file, and reworded get_tool_help docs CLI-first with a regenerated README.

---

## v1.11 Semantic Index Completion & P1 MCP Tools (Shipped: 2026-06-07)

**Phases completed:** 7 phases, 38 plans, 19 tasks

**Key accomplishments:**

- 1. [Rule 1 — Cycle] PriorFileFact.Symbols cannot be []extract.SymbolFact
- Go
- [Rule 2 - Missing critical functionality] Added label-allowlist carve-outs + vector priming.
- Duplicated
- Test:
- None significant.
- One-liner:
- [Minor] Stamp logic factored into stampLastCompactAt helper + PublicStampLastCompactAtForTest seam.
- 1. [Rule 2] Wire RetrievalStatus into the get_semantic_graph_status envelope
- Migration (v5 → v6)
- `QuerySymbolByName(ctx, repoID, path, name) ([]string, error)`
- Files:
- Issue:
- Production wiring of `RetrievalAccessor` + `ClusterMembershipAccessor` for `find_related_symbols`:
- Branch B
- cluster_id.go
- Types:
- 1. [Rule 2 - Design clarification] MemberCount uses all-rows pre-cap
- TestConn_Call race condition in `internal/kernel/jsonrpc/codec_test.go`
- Task 1:
- Task 1:
- Task 1 — Expand getChangeImpactGraphHelp
- Task 1 — SC#3 Static Wrapper-Consistency Gate (wrapper_consistency_test.go)
- Added WiredAccessorsBoolMap struct and WiredAccessorsForTest function to export_p1_test.go, providing a reflection-free test seam for all 10 P1 accessor nil-checks callable from daemon-package bootstrap tests.
- semP1SymbolEdgesAdapter (SymbolEdgesAccessor) and semP1ClusterMembershipAdapter (ClusterMembershipAccessor) added to semantic_wiring.go with supporting Store helper methods in effective_graph.go, closing BLOCKER-1 for the D-01a FOLD accessors.
- One-liner:
- One-liner:

---

## v1.9 Polish & Infra (Shipped: 2026-05-03)

**Phases completed:** 12 phases (46–56, including emergent 51.1), 51 plans
**Files changed:** 517 files, +57,759 / -3,120 lines (326 commits)
**Timeline:** 7 days (2026-04-24 → 2026-05-01; close 2026-05-03)
**Tag:** v1.9

**Key accomplishments:**

- Closed all 4 known LSP/tooling bugs (BUG-01..BUG-04): repomap PageRank starvation on polyglot workspaces, rust-analyzer rename via experimental/serverStatus readiness + RenameOverride QuirkAdapter, jdtls warm cache for Java integration tests in default `go test ./...`, single canonical `GrammarRegistry` shared across all consumers
- Resolved Go 1.25 + gopls linux/amd64 incompatibility on `ubuntu-latest` and converted the benchmark harness to local-only (per the project's local-only bench rule) — removed `bench.yml`, `capture-baseline.yml`, `*-github-hosted.txt` baselines
- Shipped reproducible multi-arch signed release pipeline via goreleaser (6 archives × darwin/linux/windows × amd64/arm64 with minisign signing); CGO=0 build path preserved via Phase 51.1 `//go:build cgo` stubs across treesitter/repomap/edit
- **Renamed product `serena → helix` as a hard-cut breaking change at v1.9** — binary, module path (`github.com/agenthands/helix`), env vars (`SERENA_* → HELIX_*`), config dir (`~/.serena/ → ~/.helix/`), MCP server registration name. Shipped in-binary self-upgrade (`helix update` / `helix upgrade`) with minisign verify + atomic swap + downgrade refusal + daemon-aware re-launch. EMBED-AUDIT.md manifest classifies every runtime asset
- Closed v1.2 observability gaps: 5 new Prometheus metric families (cache hit-rate lspool+repomap, repomap extract latency histogram, session lifecycle, edit outcomes — all bounded labels), 2 Grafana dashboards (`helix-overview.json`, `helix-engine.json`) with registry-driven PromQL validator, 4 runbooks (ErrCircuitOpen, deadline-timeouts, ls-crash-restart, memory-pressure-eviction), full per-MCP-tool + per-LS-call trace coverage with TRACE-AUDIT.md hygiene review and real Jaeger smoke capture
- **Phase 56 (emergent)** — surfaced and fixed that `jsonrpc.Conn.OnNotification` was never wired in production; QuirkAdapter notification handlers were silently dropped. Wired dispatch in `Worker.Start` with regression assertion + shipped `JdtlsAdapter.WaitUntilJavaReady(ctx)` deterministic gate

**Audit result:** `tech_debt` — 14/14 in-scope requirements satisfied (PKG-01 SC-3 deployment-gated on maintainer keypair + first v* tag); 25/25 cross-phase integration wires verified; 4/4 E2E flows wired.

**Deferred to v1.10 / future milestones:**

- PKG-DEFER-03/04/05: Homebrew tap, Scoop bucket, native Linux package (deb/rpm/AUR) — Phase 52 rescoped from these to self-contained-binary + in-binary self-upgrade
- Phase 51 reproducibility gate scope (snapshot-vs-snapshot, not real-release-vs-Pass-3)
- Phase 55 forwarder.tools.call span via Noop tracer (pre-v1.2 architectural limitation; application chain daemon → kernel → lspool fully verified)

**Known deferred items at close:** 5 (see STATE.md Deferred Items)

---

## v1.8 Documentation Overhaul (Shipped: 2026-04-23)

**Phases completed:** 8 phases, 19 plans, 11 tasks

**Key accomplishments:**

- 1. [Rule 3 - Blocking] Created skill adapters for health and help packages
- Commit:
- 1. [Rule 3 - Blocking] rust-analyzer entry not found
- Commit:
- INSTALL.md
- 1. [Rule 1 — Plan acceptance criterion off-by-one] `serena setup opencode` count is 1, not ≥2
- One-liner:
- 1. [Rule 3 - Blocking] Sentence insertion position adjusted to preserve rust-analyzer block byte-position

---

## v1.7 Developer Experience & Auto-Setup (Shipped: 2026-04-22)

**Phases completed:** 5 phases (34-38), 11 plans, 61 files changed, ~7,000 LOC
**Timeline:** 2 days (2026-04-21 → 2026-04-22)

**Key accomplishments:**

- One-command MCP registration (`serena setup <client>`) for 6 clients (Claude Code, VS Code, JetBrains, Claude Desktop, Gemini CLI, generic) with automatic language detection and LS pre-installation
- `get_health` MCP tool and `serena status` CLI for workspace health inspection with error-only defaults and verbose mode
- Claude Code hook auto-installation (SessionStart activation, PreToolUse nudge toward symbolic tools, Stop cleanup) with `--no-hooks` opt-out
- Smart error responses: MCP middleware enriches parameter typos and enum value errors with "Did you mean" suggestions using Levenshtein distance matching
- Progressive tool descriptions: tiered brief/detailed descriptions, `get_tool_help` for comprehensive on-demand docs, golden-file regression gating
- Lazy workspace initialization: sync.Once per workspace path, transparent activation on first MCP tool call

**Known deferred items at close:** 6 (see STATE.md Deferred Items) — all require live external environments (Claude CLI, VS Code, JetBrains) not available in CI

---

## v1.5 Typed Errors & Hardening (Shipped: 2026-04-15)

**Phases completed:** 3 phases (22-24), 12 plans
**Files changed:** 95 files, +7,199 / -534 lines
**Timeline:** 1 day (2026-04-14 → 2026-04-15)

**Key accomplishments:**

- Created `internal/errors/` package with 7 error kinds (NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout), builder pattern, JSON serialization, and cause-chain wrapping via errors.Is/As
- Migrated existing sentinels (ErrCircuitOpen, ErrSessionExpired, ErrLSCrashed) into unified error taxonomy with backward-compatible re-exports, then removed all deprecated bridge aliases
- Migrated all 38+ MCP tools across 10 packages (symbols, edit, fileops, diag, memory, workflow, profile, MCP core) from raw `fmt.Errorf` strings to typed `serr.New`/`serr.Wrap` errors
- Added inline input validation to all 24 kernel tool handlers — empty-string checks on required fields before any workspace or LS work begins
- Upgraded three-band error tests with `extractKind` helper and `expectedKind` struct field for Kind-level assertions, plus 4 typed error golden files (invalid_args, no_workspace, not_found, unsupported)

**Tech debt accepted:**

- 3 golden files deferred (circuit_open, timeout, internal runtime) — cannot trigger deterministically without live LS
- unsupported.golden captures raw error (lspool not yet using serr.Unsupported)
- 2 internal flow-control fmt.Errorf in fileops (errLimitReached, never reaches MCP)

---

## v1.4 Integration Testing v2 (Shipped: 2026-04-14)

**Phases completed:** 4 phases (18-21), 11 plans
**Lines changed:** +5,615 / -51 across 128 files
**Timeline:** 4 days (2026-04-11 → 2026-04-14)

**Key accomplishments:**

- Extracted importable test harness (`test/harness/`) with Runner, golden store, fixture helpers, and build tag taxonomy (`integration`, `llm`, `llmjudge`)
- Protocol oracle tests — MCP handshake, tools/list validation, session isolation, reconnect resilience
- Contract oracle tests — JSON Schema Draft 2020-12 validation, 23 golden output files, 6 error category contracts, tool selectability heuristics
- Scenario oracle matrix — 14+ full-cycle agent workflow tests across Go, Python, TypeScript, C++, Swift, Zig, JavaScript, PHP, SQL, Markdown, polyglot, unsupported, collision, and degraded fixtures
- LLM behavioral tests — tool selection (33 tools), disambiguation (11 pairs), output interpretation with multi-provider support (Anthropic + DeepSeek)
- Judge scoring infrastructure — structured rubrics (5 dimensions), transcript pipeline, aggregate reporting, never blocks merge

---

## v1.3 Documentation Catchup (Shipped: 2026-04-11)

**Phases completed:** 2 phases, 4 plans, 7 tasks

**Key accomplishments:**

- 1. [Rule 1 - Bug] Fixed inaccurate tool count claim
- Commit:
- 1. [Rule 1 - Bug] Corrected serena_tool_duration_seconds label documentation
- Go binary install guide with per-agent MCP config sections for Claude Code, Codex, OpenCode, Cursor, Gemini CLI, Antigravity, and HTTP mode

---

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
