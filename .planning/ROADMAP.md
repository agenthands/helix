# Roadmap: Serena 2.0

## Milestones

- ✅ **v1.0 MVP** — Phases 1-5 (shipped 2026-04-08)
- ✅ **v1.1 Integration Testing** — Phases 6-8 (shipped 2026-04-09)
- ✅ **v1.2 Performance & Production Hardening** — Phases 9-15 (shipped 2026-04-10)
- 🚧 **v1.3 Documentation Catchup** — Phases 16-17 (in progress)

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

### 🚧 v1.3 Documentation Catchup (In Progress)

**Milestone Goal:** Update all project documentation to accurately reflect v1.2 capabilities (observability, metrics, tracing, graceful degradation, benchmarks).

- [ ] **Phase 16: Core Documentation Update** - Update README, CHANGELOG, and CONTRIBUTING to reflect current project state
- [ ] **Phase 17: Usage & Install Guides** - Add observability/degradation docs to USAGE, overhaul install guide for multi-agent setup

## Phase Details

### Phase 16: Core Documentation Update
**Goal**: Project documentation accurately describes Serena's current capabilities, architecture, and contributor workflow
**Depends on**: Phase 15 (v1.2 shipped)
**Requirements**: README-01, README-02, README-03, CHLOG-01, CONTR-01, CONTR-02
**Success Criteria** (what must be TRUE):
  1. README reflects current tool count (38+), 4-layer architecture, and all v1.2 capabilities (observability, metrics, tracing, graceful degradation)
  2. README install instructions work end-to-end for a fresh user (go install, binary usage, MCP client config)
  3. CHANGELOG v1.2 entry covers all 7 phases (9-15) with key accomplishments matching MILESTONES.md
  4. CONTRIBUTING documents the current build/test/vet/benchmark workflow and explains how to run integration tests and the benchmark harness
**Plans**: TBD

### Phase 17: Usage & Install Guides
**Goal**: Users can configure and tune Serena's production features using USAGE docs, and coding agents can be set up using the install guide
**Depends on**: Phase 16 (README/CHANGELOG provide foundational context)
**Requirements**: USAGE-01, USAGE-02, USAGE-03, INST-01, INST-02
**Success Criteria** (what must be TRUE):
  1. USAGE documents how to enable and configure Prometheus metrics, OTLP tracing, and the admin listener with concrete config examples
  2. USAGE documents graceful degradation tuning (per-class budgets, GOMEMLIMIT, circuit breaker settings) with recommended defaults
  3. USAGE documents performance tuning guidance and how to run/interpret benchmarks
  4. Install guide is renamed from llms-install.md to an agent-focused name and covers setup for Claude Code, Codex, OpenCode, Cursor, Gemini CLI, and Antigravity
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 16 → 17

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
| 16. Core Documentation Update | v1.3 | 0/? | Not started | - |
| 17. Usage & Install Guides | v1.3 | 0/? | Not started | - |
