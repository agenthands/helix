# Serena

## What This Is

A Go-native code intelligence platform for MCP: universal LSP gateway at the core, agent skills as plugins. Single binary, persistent daemon, 35+ callable MCP tools, 52-language support. Targets coding agents (Claude Code, Codex, IDE assistants) that need semantic code operations — symbol-level retrieval, editing, refactoring — backed by real language servers with warm persistent caching.

## Core Value

Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## Requirements

### Validated

- ✓ Go rewrite with daemon skeleton, MCP runtime, gRPC forwarder, HTTP transport — v1.0 Phase 1
- ✓ Single binary distribution via `go build` — v1.0 Phase 1
- ✓ Python Serena moved to `legacy/` as reference — v1.0 Phase 1
- ✓ LSP 3.17 metamodel codegen (324 structs, 216 union types) — v1.0 Phase 2
- ✓ JSON-RPC codec for LS communication — v1.0 Phase 2
- ✓ LS worker pool with adaptive TTL, circuit breaking, pressure eviction — v1.0 Phase 2
- ✓ 9 symbol retrieval tools (definition, references, overview, search, hover, implementations, call/type hierarchy, blast radius) — v1.0 Phase 2
- ✓ 6 symbol editing tools with tree-sitter body surgery — v1.0 Phase 2
- ✓ 6 file operation tools — v1.0 Phase 2
- ✓ 3 diagnostic tools (diagnostics, code actions, formatting) — v1.0 Phase 2
- ✓ 52-language embedded registry with YAML override — v1.0 Phase 3
- ✓ Three-tier LS installer (PATH/download/error) — v1.0 Phase 3
- ✓ Memory system (markdown + SQLite FTS5) — v1.0 Phase 3
- ✓ Skill interface (Skill/ToolProvider/WorkflowProvider) — v1.0 Phase 3
- ✓ 7 memory MCP tools + onboarding/handoff workflows — v1.0 Phase 3
- ✓ 5 agent profiles (claude-code, codex, ide-assistant, ci-bot, full) — v1.0 Phase 4
- ✓ 4 modes (read/edit/review/admin) with switch_mode tool — v1.0 Phase 4
- ✓ Token budget reporting via get_token_budget — v1.0 Phase 4
- ✓ Layered config precedence (CLI > project > user > profile) — v1.0 Phase 4
- ✓ Dynamic tool registry with pluggable skill packs — v1.0
- ✓ Centralized daemon bootstrap with 38+ callable MCP tools — v1.0 Phase 5
- ✓ Kernel tool skill adapters for uniform composition model — v1.0 Phase 5
- ✓ Integration test harness — end-to-end MCP round-trips (InMemory + HTTP) — v1.1 Phase 6
- ✓ Go dogfooding suite — all 38 tools tested against own codebase — v1.1 Phase 6
- ✓ Multi-language test fixtures — Python, TypeScript, Java, Rust — v1.1 Phase 7
- ✓ Symbol editing round-trip tests — all 6 edit tools — v1.1 Phase 7
- ✓ Profile/mode contract tests — golden file pattern with 19 goldens — v1.1 Phase 8
- ✓ Three-tier concurrency tests — scenarios + fan-out + testing/synctest — v1.1 Phase 8
- ✓ Three-band error path coverage — 30 cases across categories and destructive tools — v1.1 Phase 8

- ✓ README.md with capabilities, install, auto-generated tool/language tables — v1.2 Phase 14
- ✓ USAGE.md with profiles, workflows, troubleshooting, observability, performance tuning — v1.2 Phase 14
- ✓ CHANGELOG.md with v1.0, v1.1, v1.2 entries — v1.2 Phase 14
- ✓ Benchmark harness with testing.B.Loop, CI benchstat gate, baseline capture workflow — v1.2 Phase 9, 15
- ✓ Tool response time benchmarks (p50/p95/p99) for all 38 tools — v1.2 Phase 9
- ✓ LSP indexing throughput benchmarks (cold/warm) — v1.2 Phase 9
- ✓ Memory profiles (4 scenarios with dual Go/kernel RSS + pprof) — v1.2 Phase 9
- ✓ Observability foundation (internal/obs/, trace-aware slog, admin listener) — v1.2 Phase 10
- ✓ Prometheus /metrics with RED histograms, lspool gauges, bounded labels — v1.2 Phase 11
- ✓ End-to-end tracing (otelgrpc, telemetry middleware, per-tool spans, OTLP exporter) — v1.2 Phase 12
- ✓ Graceful degradation (per-class budgets, deadline propagation, ErrCircuitOpen, GOMEMLIMIT) — v1.2 Phase 13

- ✓ README.md updated with Production & Observability section, corrected tool count to 35+ — v1.3 Phase 16
- ✓ CONTRIBUTING.md rewritten as Go-native contributor guide — v1.3 Phase 16
- ✓ CHANGELOG.md v1.2 entry completed (Phase 15 gap filled) — v1.3 Phase 16
- ✓ USAGE.md accuracy gaps fixed (6 metrics, service_name config, PromQL examples, benchmarks subsection) — v1.3 Phase 17
- ✓ INSTALL.md created with per-agent MCP config for 6 coding agents + HTTP mode — v1.3 Phase 17
- ✓ Test harness extraction (test/harness/ package with Runner, tools, golden, fixture helpers) — v1.4 Phase 18
- ✓ Protocol oracle tests (handshake, tools/list, session isolation, reconnect) — v1.4 Phase 19
- ✓ Contract oracle tests (schema meta-validation, selectability heuristics, golden outputs, error contracts) — v1.4 Phase 19
- ✓ Scenario oracle tests (multi-language runtime correctness across 14+ languages/fixtures) — v1.4 Phase 20
- ✓ LLM behavioral tests (tool selection, disambiguation, output interpretation) + judge scoring infrastructure — v1.4 Phase 21
- ✓ Multi-provider LLM support (Anthropic + DeepSeek) for behavioral tests — v1.4
- ✓ Extension-based language detection fallback for marker-free languages (Markdown) — v1.4
- ✓ Marksman quirk adapter for Markdown LSP support — v1.4

### Active

## Current Milestone: v1.5 Typed Errors & Hardening

**Goal:** Every MCP tool returns typed, structured errors — replacing raw strings with a consistent error taxonomy that enables reliable error handling by agents.

**Target features:**
- Typed error taxonomy (error kinds, categories, structured fields)
- Migrate all 38+ tools from raw error strings to typed errors
- Consistent error contracts across kernel, skills, and MCP layer
- Input validation at tool boundaries (parameter checking before execution)
- Error path test coverage (extend three-band error tests to use typed assertions)

### Out of Scope

- JetBrains plugin backend — dropped, LSP-only going forward
- Python compatibility layer — native Go, no Python interop
- Mobile/embedded targets — server-side only
- Custom language server implementations — wrap existing LSP servers, don't reimplement
- Knowledge graphs — CodeGraphContext/GitNexus own this space
- Vector/embedding search — Augment Context Engine does this better
- Git operations — GitHub MCP Server handles git comprehensively

## Context

Shipped v1.0 (25,779 LOC, 35 MCP tools, 52 languages), v1.1 Integration Testing (~12K additional LOC), v1.2 Performance & Production Hardening (35.7K total Go LOC), v1.3 Documentation Catchup, and v1.4 Integration Testing v2 (4,850 LOC oracle tests). Single binary, 4-layer architecture, persistent daemon. All documentation current as of v1.2 capabilities. Multi-oracle test harness covers protocol, contract, scenario (14+ language fixtures), LLM behavioral (multi-provider: Anthropic + DeepSeek), and judge scoring. Extension-based language detection fallback enables marker-free languages like Markdown.

Tech stack: Go 1.25, official MCP Go SDK, koanf v2, modernc.org/sqlite, go-tree-sitter, gRPC, prometheus/client_golang, OpenTelemetry (otelgrpc + otlptrace).

Architecture: 4-layer (MCP runtime → Code intelligence kernel → Skills → Agent profiles). Persistent daemon with stdio/HTTP edge adapters. Worker pool with share-until-dirty, adaptive TTL, platform-aware pressure eviction. All layers wired via centralized daemon bootstrap with fail-fast core / degraded-optional startup. Observability: noop-default provider with opt-in Prometheus metrics and OTLP tracing. Dedicated admin listener for /healthz, /readyz, /metrics, /debug/pprof.

**v1.2 known tech debt:** Benchmark baselines captured locally (darwin/arm64) instead of CI ubuntu-latest due to gopls v0.17.1 incompatibility with Go 1.25 on linux/amd64. `capture-baseline.yml` workflow ready for re-capture once resolved. benchstat@latest unpinned.

The `legacy/` directory contains the original Python-based prototype as a reference.

## Constraints

- **Language**: Go — single binary, native concurrency
- **Protocol**: MCP (Model Context Protocol) — primary interface
- **LSP only**: No JetBrains or proprietary backends

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go implementation | Runtime/lifecycle correctness is the foundation problem | ✓ Good — clean single binary, native concurrency |
| Daemon + edge adapters (gopls pattern) | Warm cache, multi-client, reconnect | ✓ Good — worker pool with adaptive TTL |
| Drop JetBrains backend | Simplify to LSP-only | ✓ Good — 52 languages via LSP |
| 4-layer architecture | Prevents monolith; skills as plugins | ✓ Good — clean separation |
| gRPC internal IPC | Typed APIs between forwarder and daemon | ✓ Good — clean proto contract |
| Tree-sitter for body surgery | LSP-only editing was fragile (Serena pain point) | ✓ Good — precise body extraction for 4 languages |
| Full LSP metamodel codegen | Forward-sync with spec, no hand-maintained types | ✓ Good — 324 structs, 216 unions generated |
| Markdown + SQLite FTS5 for memory | Humans own content, engine owns search | ✓ Good — rebuildable index |
| Caddy-style skill registration | Compiled-in plugins without go-plugin overhead | ✓ Good — clean init() pattern |
| Centralized daemon bootstrap | Daemon owns all MCP registration; skills describe, daemon binds | ✓ Good — 38 tools registered, profile filtering, clean shutdown |
| Kernel tools as skill adapters | Uniform ToolProvider interface for profiles and modes | ✓ Good — 4 thin adapters, consistent composition model |
| Integration tests in top-level `test/` package | Black-box testing through public API only | ✓ Good — forced daemon accessor exports, cleaner API |
| Both InMemory + HTTP transports in test harness | InMemory for speed, HTTP smoke for serialization path | ✓ Good — catches both protocol and wire bugs |
| Tiered Go fixture + codebase smoke | Small testdata for deterministic assertions, own codebase for scale | ✓ Good — both layers found real bugs |
| Golden file pattern for profile/mode contracts | Independent oracle from production YAML prevents self-approving bad changes | ✓ Good — 19 goldens, diff-reviewable updates |
| Three-tier concurrency (scenarios + fan-out + synctest) | Layered coverage: realistic + targeted + deterministic | ✓ Good — caught SessionInfo data race |
| Three-band error coverage (category + destructive + read-only) | Risk-weighted testing per Google/OWASP guidance | ✓ Good — 30 cases with low duplication |
| Structured IsError oracle (defer typed errors) | Kernel lacks typed errors yet, tracked as TODO(#typed-errors) | ⚠ Revisit — acceptable short-term, needs upgrade path |
| Multi-oracle test architecture | Five oracle layers as separate packages under test/oracle/ | ✓ Good — clean separation of deterministic vs LLM tests |
| Extension-based language detection fallback | Marker-free languages (Markdown) need file extension scanning | ✓ Good — enables any language with registered extensions |
| Multi-provider LLM tests (Anthropic + DeepSeek) | DeepSeek as fallback when Anthropic credits unavailable | ✓ Good — 97.4% pass rate with DeepSeek, proves provider-agnostic tool descriptions |
| LLM tests never block merge | Build-tag gated, informational judge scoring only | ✓ Good — avoids flaky CI from LLM nondeterminism |
| Benchmarks-first ordering | Observing-the-benchmarked-thing taints results; Phase 9 baseline before any obs code | ✓ Good — clean pre-instrumentation baseline |
| Noop-default observability | Metrics/tracing opt-in, zero overhead when disabled | ✓ Good — 0 allocs/op on fast path |
| Tiered benchmark thresholds | PR tier (15%/25%) relaxed for CI noise; release tier (10%/20%) tight | ✓ Good — reduces false positives |
| Dedicated admin listener | Separate from MCP mux; bind failure non-fatal | ✓ Good — clean separation of concerns |
| Decoupled MetricsSink interface | lspool has zero imports of internal/obs | ✓ Good — compile-time assertion enforces |
| Single typed error (ErrCircuitOpen) | Full migration deferred to v1.3+ | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition:**
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone:**
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-14 after v1.5 milestone started — Typed Errors & Hardening*
