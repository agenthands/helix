# Roadmap: Serena 2.0

## Milestones

- ✅ **v1.0 MVP** — Phases 1-5 (shipped 2026-04-08)
- ✅ **v1.1 Integration Testing** — Phases 6-8 (shipped 2026-04-09)
- ✅ **v1.2 Performance & Production Hardening** — Phases 9-15 (shipped 2026-04-10)
- ✅ **v1.3 Documentation Catchup** — Phases 16-17 (shipped 2026-04-11)
- 🚧 **v1.4 Integration Testing v2** — Phases 18-21 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-5) — SHIPPED 2026-04-08</summary>

- [x] Phase 1: Foundation (3/3 plans) — Daemon, MCP runtime, gRPC forwarder, HTTP transport
- [x] Phase 2: Code Intelligence Kernel (6/6 plans) — LSP codegen, worker pool, 9 symbol + 6 edit + 6 file + 3 diag tools
- [x] Phase 3: Multi-Language and Skills (5/5 plans) — 52 languages, memory system, skill interface
- [x] Phase 4: Agent Profiles and Configuration (3/3 plans) — 5 profiles, 4 modes, token budget
- [x] Phase 5: Daemon Bootstrap Integration (3/3 plans) — Full daemon wiring, 38+ callable MCP tools

**Full details:** `.planning/milestones/v1.0-ROADMAP.md`

</details>

<details>
<summary>✅ v1.1 Integration Testing (Phases 6-8) — SHIPPED 2026-04-09</summary>

- [x] Phase 6: Test Harness + Go Dogfooding (4/4 plans) — MCP round-trip harness, 38-tool dogfooding, Language field gap closure
- [x] Phase 7: Symbol Editing + Multi-Language Fixtures (3/3 plans) — Edit round-trips, Python/TypeScript/Java/Rust fixtures
- [x] Phase 8: Advanced Testing (4/4 plans) — Profile/mode golden contracts, three-tier concurrency, three-band errors

**Full details:** `.planning/milestones/v1.1-ROADMAP.md`

</details>

<details>
<summary>✅ v1.2 Performance & Production Hardening (Phases 9-15) — SHIPPED 2026-04-10</summary>

- [x] Phase 9: Benchmark Harness & v1.1 Baseline (6/6 plans) — testing.B.Loop benchmarks, CI benchstat gate, v1.1 baseline
- [x] Phase 10: Observability Foundation (3/3 plans) — internal/obs/, trace-aware slog, admin listener, health endpoints, pprof
- [x] Phase 11: Metrics (4/4 plans) — Prometheus /metrics, RED histograms, lspool gauges, bounded-label contract
- [x] Phase 12: Tracing End-to-End (5/5 plans) — otelgrpc propagation, telemetry middleware, per-tool spans, OTLP exporter
- [x] Phase 13: Graceful Degradation (3/3 plans) — Per-class budgets, deadline propagation, ErrCircuitOpen, GOMEMLIMIT
- [x] Phase 14: Documentation (3/3 plans) — README.md, USAGE.md, CHANGELOG.md with auto-generated tables
- [x] Phase 15: Benchmark Gate Hardening (1/1 plans) — capture-baseline.yml workflow, blocking benchstat gate

**Full details:** `.planning/milestones/v1.2-ROADMAP.md`

</details>

<details>
<summary>✅ v1.3 Documentation Catchup (Phases 16-17) — SHIPPED 2026-04-11</summary>

- [x] Phase 16: Core Documentation Update (2/2 plans) — README Production & Observability section, Go-native CONTRIBUTING, CHANGELOG v1.2 gap fill
- [x] Phase 17: Usage & Install Guides (2/2 plans) — USAGE.md accuracy fixes + benchmarks, INSTALL.md with 6-agent setup

**Full details:** `.planning/milestones/v1.3-ROADMAP.md`

</details>

### 🚧 v1.4 Integration Testing v2 (In Progress)

**Milestone Goal:** Prove Serena-Go is protocol-correct, contract-stable, runtime-safe, and genuinely usable by LLM clients through a multi-oracle test harness.

- [x] **Phase 18: Harness Extraction & Foundation** - Extract importable test harness and establish build tag taxonomy (completed 2026-04-11)
- [x] **Phase 19: Protocol & Contract Oracles** - Validate MCP protocol compliance and per-tool contract stability (completed 2026-04-11)
- [x] **Phase 20: Scenarios & Runtime** - Multi-step agent workflows, polyglot honesty, profile/mode behavior, and runtime stress (completed 2026-04-11)
- [ ] **Phase 21: LLM Behavioral & Judge** - LLM tool selection accuracy, disambiguation, output interpretation, and judge scoring

## Phase Details

### Phase 18: Harness Extraction & Foundation
**Goal**: New oracle packages can import shared test infrastructure and run under correct build tags
**Depends on**: Phase 17 (v1.3 complete)
**Requirements**: FOUND-01, FOUND-02
**Success Criteria** (what must be TRUE):
  1. A new test file in `test/oracle/protocol/` can import `test/harness` and call `StartTestDaemon`, `PrepareFixture`, `callTool`, and golden helpers without compilation errors
  2. Running `go test ./...` (no tags) skips all integration/llm/llmjudge tests; running with `-tags integration` includes oracle tests but not LLM tests
  3. Existing v1.1 tests in `test/integration/` continue to pass unchanged
**Plans:** 2/2 plans complete

Plans:
- [x] 18-01-PLAN.md — Extract test/harness/ package with evolved API and self-tests
- [x] 18-02-PLAN.md — Create oracle directory stubs and smoke test for build tag taxonomy

### Phase 19: Protocol & Contract Oracles
**Goal**: Every MCP protocol interaction and every exposed tool has deterministic correctness assertions
**Depends on**: Phase 18
**Requirements**: PROTO-01, PROTO-02, PROTO-03, PROTO-04, CONT-01, CONT-02, CONT-03, CONT-04
**Success Criteria** (what must be TRUE):
  1. MCP initialize/shutdown handshake succeeds on both InMemory and HTTP transports with correct capabilities, protocol version, and server info
  2. `tools/list` returns tools with unique names, non-empty descriptions, and schemas that pass JSON Schema Draft 2020-12 validation
  3. Two concurrent sessions with different workspaces and modes do not observe each other's state or side effects
  4. A disconnected client can reconnect and resume without worker leakage or stale state
  5. Every exposed MCP tool has a golden output file and error responses assert stable error class/code per category
**Plans:** 3/3 plans complete

Plans:
- [x] 19-01-PLAN.md — Protocol oracle: handshake, tools/list, session isolation, reconnect
- [x] 19-02-PLAN.md — Contract oracle: schema meta-validation and selectability heuristics
- [x] 19-03-PLAN.md — Contract oracle: golden output files and error category contracts

### Phase 20: Scenarios & Runtime
**Goal**: Realistic agent workflows pass across diverse repository shapes, and the runtime survives stress and degraded conditions
**Depends on**: Phase 19
**Requirements**: SCEN-01, SCEN-02, SCEN-03, SCEN-04, RUNT-01, RUNT-02, RUNT-03
**Success Criteria** (what must be TRUE):
  1. Multi-step agent workflows (activate, search, read, edit, verify) pass against Go, Python, TypeScript, polyglot monorepo, unsupported language, degraded capability, and name collision fixtures
  2. Polyglot scenarios produce no fake cross-language symbol links, no silent omissions, and unsupported languages fail with clear errors
  3. Read mode blocks edit tool calls, admin mode grants all tools, and mode switching updates tool visibility correctly across concurrent sessions
  4. Worker pool survives sustained load with circuit breaker trips, pressure eviction, and share-until-dirty under concurrent edits; clean shutdown drains work with no goroutine leaks
  5. Selective LS/memory/skill failure injection causes degraded startup (not crash), and affected tools report status honestly
**Plans:** 4/4 plans complete

Plans:
- [x] 20-01-PLAN.md — Create fixture matrix (polyglot, unsupported, collision) and runtime oracle stub
- [x] 20-02-PLAN.md — Multi-step agent workflow scenarios (Go, Python, TS, polyglot, unsupported, collision, degraded)
- [x] 20-03-PLAN.md — Profile/mode behavior tests and deferred CONT-03 error categories
- [x] 20-04-PLAN.md — Runtime stress (pool, shutdown, degraded subsystem injection)

### Phase 21: LLM Behavioral & Judge
**Goal**: An LLM client can correctly select, disambiguate, and interpret results from every Serena tool
**Depends on**: Phase 20
**Requirements**: LLM-01, LLM-02, LLM-03, LLM-04
**Success Criteria** (what must be TRUE):
  1. Given a task description for every exposed tool, Claude selects the correct tool based on tool descriptions alone
  2. Claude distinguishes similar tool pairs (search_symbols vs find_references, get_symbol_overview vs explain_symbol) and selects appropriately based on context
  3. Claude correctly interprets tool results -- distinguishes success from failure, does not hallucinate capabilities the tool output does not support
  4. LLM judge scores transcripts via structured rubrics and runs only on manual trigger, never blocking merge
**Plans:** 2 plans

Plans:
- [ ] 21-01-PLAN.md — Shared LLM infrastructure + tool selection and disambiguation tests
- [ ] 21-02-PLAN.md — Output interpretation tests + judge scoring infrastructure

## Progress

**Execution Order:**
Phases execute in numeric order: 18 → 19 → 20 → 21

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | Complete | 2026-04-07 |
| 2. Code Intelligence Kernel | v1.0 | 6/6 | Complete | 2026-04-08 |
| 3. Multi-Language and Skills | v1.0 | 5/5 | Complete | 2026-04-08 |
| 4. Agent Profiles and Configuration | v1.0 | 3/3 | Complete | 2026-04-08 |
| 5. Daemon Bootstrap Integration | v1.0 | 3/3 | Complete | 2026-04-08 |
| 6. Test Harness + Go Dogfooding | v1.1 | 4/4 | Complete | 2026-04-09 |
| 7. Symbol Editing + Multi-Language Fixtures | v1.1 | 3/3 | Complete | 2026-04-09 |
| 8. Advanced Testing | v1.1 | 4/4 | Complete | 2026-04-09 |
| 9. Benchmark Harness & v1.1 Baseline | v1.2 | 6/6 | Complete | 2026-04-09 |
| 10. Observability Foundation | v1.2 | 3/3 | Complete | 2026-04-09 |
| 11. Metrics | v1.2 | 4/4 | Complete | 2026-04-09 |
| 12. Tracing End-to-End | v1.2 | 5/5 | Complete | 2026-04-10 |
| 13. Graceful Degradation | v1.2 | 3/3 | Complete | 2026-04-10 |
| 14. Documentation | v1.2 | 3/3 | Complete | 2026-04-10 |
| 15. Benchmark Gate Hardening | v1.2 | 1/1 | Complete | 2026-04-10 |
| 16. Core Documentation Update | v1.3 | 2/2 | Complete | 2026-04-11 |
| 17. Usage & Install Guides | v1.3 | 2/2 | Complete | 2026-04-11 |
| 18. Harness Extraction & Foundation | v1.4 | 2/2 | Complete    | 2026-04-11 |
| 19. Protocol & Contract Oracles | v1.4 | 3/3 | Complete    | 2026-04-11 |
| 20. Scenarios & Runtime | v1.4 | 4/4 | Complete    | 2026-04-12 |
| 21. LLM Behavioral & Judge | v1.4 | 0/2 | Not started | - |
