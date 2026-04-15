# Roadmap: Serena 2.0

## Milestones

- ✅ **v1.0 MVP** — Phases 1-5 (shipped 2026-04-08)
- ✅ **v1.1 Integration Testing** — Phases 6-8 (shipped 2026-04-09)
- ✅ **v1.2 Performance & Production Hardening** — Phases 9-15 (shipped 2026-04-10)
- ✅ **v1.3 Documentation Catchup** — Phases 16-17 (shipped 2026-04-11)
- ✅ **v1.4 Integration Testing v2** — Phases 18-21 (shipped 2026-04-14)
- ✅ **v1.5 Typed Errors & Hardening** — Phases 22-24 (shipped 2026-04-15)

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

<details>
<summary>✅ v1.4 Integration Testing v2 (Phases 18-21) — SHIPPED 2026-04-14</summary>

- [x] Phase 18: Harness Extraction & Foundation (2/2 plans) — Importable test/harness/, build tag taxonomy
- [x] Phase 19: Protocol & Contract Oracles (3/3 plans) — MCP handshake, 23 goldens, schema validation, error contracts
- [x] Phase 20: Scenarios & Runtime (4/4 plans) — 14+ agent workflow scenarios, pool stress, degraded injection
- [x] Phase 21: LLM Behavioral & Judge (2/2 plans) — Tool selection, disambiguation, interpretation, judge scoring

**Full details:** `.planning/milestones/v1.4-ROADMAP.md`

</details>

<details>
<summary>✅ v1.5 Typed Errors & Hardening (Phases 22-24) — SHIPPED 2026-04-15</summary>

- [x] Phase 22: Error Taxonomy (2/2 plans) — internal/errors/ package with 7 error kinds, builder pattern, cause-chain wrapping
- [x] Phase 23: Tool Migration (8/8 plans) — All 38+ tools migrated from raw strings to typed errors across 10 packages
- [x] Phase 24: Validation & Testing (2/2 plans) — Inline input validation on 24 kernel tools, Kind-level test assertions, typed golden files

**Full details:** `.planning/milestones/v1.5-ROADMAP.md`

</details>

## Progress

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
| 18. Harness Extraction & Foundation | v1.4 | 2/2 | Complete | 2026-04-11 |
| 19. Protocol & Contract Oracles | v1.4 | 3/3 | Complete | 2026-04-11 |
| 20. Scenarios & Runtime | v1.4 | 4/4 | Complete | 2026-04-12 |
| 21. LLM Behavioral & Judge | v1.4 | 2/2 | Complete | 2026-04-12 |
| 22. Error Taxonomy | v1.5 | 2/2 | Complete | 2026-04-15 |
| 23. Tool Migration | v1.5 | 8/8 | Complete | 2026-04-15 |
| 24. Validation & Testing | v1.5 | 2/2 | Complete | 2026-04-15 |
