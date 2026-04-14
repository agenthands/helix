# Roadmap: Serena 2.0

## Milestones

- ✅ **v1.0 MVP** — Phases 1-5 (shipped 2026-04-08)
- ✅ **v1.1 Integration Testing** — Phases 6-8 (shipped 2026-04-09)
- ✅ **v1.2 Performance & Production Hardening** — Phases 9-15 (shipped 2026-04-10)
- ✅ **v1.3 Documentation Catchup** — Phases 16-17 (shipped 2026-04-11)
- ✅ **v1.4 Integration Testing v2** — Phases 18-21 (shipped 2026-04-14)
- **v1.5 Typed Errors & Hardening** — Phases 22-24 (in progress)

## Phases

<details>
<summary>v1.0 MVP (Phases 1-5) -- SHIPPED 2026-04-08</summary>

- [x] Phase 1: Foundation (3/3 plans) -- Daemon, MCP runtime, gRPC forwarder, HTTP transport
- [x] Phase 2: Code Intelligence Kernel (6/6 plans) -- LSP codegen, worker pool, 9 symbol + 6 edit + 6 file + 3 diag tools
- [x] Phase 3: Multi-Language and Skills (5/5 plans) -- 52 languages, memory system, skill interface
- [x] Phase 4: Agent Profiles and Configuration (3/3 plans) -- 5 profiles, 4 modes, token budget
- [x] Phase 5: Daemon Bootstrap Integration (3/3 plans) -- Full daemon wiring, 38+ callable MCP tools

**Full details:** `.planning/milestones/v1.0-ROADMAP.md`

</details>

<details>
<summary>v1.1 Integration Testing (Phases 6-8) -- SHIPPED 2026-04-09</summary>

- [x] Phase 6: Test Harness + Go Dogfooding (4/4 plans) -- MCP round-trip harness, 38-tool dogfooding, Language field gap closure
- [x] Phase 7: Symbol Editing + Multi-Language Fixtures (3/3 plans) -- Edit round-trips, Python/TypeScript/Java/Rust fixtures
- [x] Phase 8: Advanced Testing (4/4 plans) -- Profile/mode golden contracts, three-tier concurrency, three-band errors

**Full details:** `.planning/milestones/v1.1-ROADMAP.md`

</details>

<details>
<summary>v1.2 Performance & Production Hardening (Phases 9-15) -- SHIPPED 2026-04-10</summary>

- [x] Phase 9: Benchmark Harness & v1.1 Baseline (6/6 plans) -- testing.B.Loop benchmarks, CI benchstat gate, v1.1 baseline
- [x] Phase 10: Observability Foundation (3/3 plans) -- internal/obs/, trace-aware slog, admin listener, health endpoints, pprof
- [x] Phase 11: Metrics (4/4 plans) -- Prometheus /metrics, RED histograms, lspool gauges, bounded-label contract
- [x] Phase 12: Tracing End-to-End (5/5 plans) -- otelgrpc propagation, telemetry middleware, per-tool spans, OTLP exporter
- [x] Phase 13: Graceful Degradation (3/3 plans) -- Per-class budgets, deadline propagation, ErrCircuitOpen, GOMEMLIMIT
- [x] Phase 14: Documentation (3/3 plans) -- README.md, USAGE.md, CHANGELOG.md with auto-generated tables
- [x] Phase 15: Benchmark Gate Hardening (1/1 plans) -- capture-baseline.yml workflow, blocking benchstat gate

**Full details:** `.planning/milestones/v1.2-ROADMAP.md`

</details>

<details>
<summary>v1.3 Documentation Catchup (Phases 16-17) -- SHIPPED 2026-04-11</summary>

- [x] Phase 16: Core Documentation Update (2/2 plans) -- README Production & Observability section, Go-native CONTRIBUTING, CHANGELOG v1.2 gap fill
- [x] Phase 17: Usage & Install Guides (2/2 plans) -- USAGE.md accuracy fixes + benchmarks, INSTALL.md with 6-agent setup

**Full details:** `.planning/milestones/v1.3-ROADMAP.md`

</details>

<details>
<summary>v1.4 Integration Testing v2 (Phases 18-21) -- SHIPPED 2026-04-14</summary>

- [x] Phase 18: Harness Extraction & Foundation (2/2 plans) -- Importable test/harness/, build tag taxonomy
- [x] Phase 19: Protocol & Contract Oracles (3/3 plans) -- MCP handshake, 23 goldens, schema validation, error contracts
- [x] Phase 20: Scenarios & Runtime (4/4 plans) -- 14+ agent workflow scenarios, pool stress, degraded injection
- [x] Phase 21: LLM Behavioral & Judge (2/2 plans) -- Tool selection, disambiguation, interpretation, judge scoring

**Full details:** `.planning/milestones/v1.4-ROADMAP.md`

</details>

### v1.5 Typed Errors & Hardening (In Progress)

**Milestone Goal:** Every MCP tool returns typed, structured errors -- replacing raw strings with a consistent error taxonomy that enables reliable error handling by agents.

- [ ] **Phase 22: Error Taxonomy** - Define typed error kinds, structured fields, and cause-chain wrapping
- [ ] **Phase 23: Tool Migration** - Migrate all 38+ tools from raw error strings to typed errors
- [ ] **Phase 24: Validation & Testing** - Input validation at tool boundaries and typed error test coverage

## Phase Details

### Phase 22: Error Taxonomy
**Goal**: Every tool failure produces a typed, structured error with a known kind and preserved cause chain
**Depends on**: Phase 21 (v1.4 complete)
**Requirements**: ERR-01, ERR-02, ERR-03
**Success Criteria** (what must be TRUE):
  1. A package exports error kinds (NotFound, InvalidArgs, NoWorkspace, Unsupported, Internal, CircuitOpen, Timeout) that any tool can return
  2. Every typed error carries structured fields (Kind, Message, Tool, Detail) that serialize to JSON for agent consumption
  3. Wrapping a lower-level error (LSP, filesystem, tree-sitter) in a typed error preserves the full cause chain via errors.Is/As
  4. The existing ErrCircuitOpen is migrated into the new taxonomy without breaking current behavior
**Plans**: TBD

### Phase 23: Tool Migration
**Goal**: All 38+ MCP tools return typed errors instead of raw strings, providing agents with consistent programmatic error handling
**Depends on**: Phase 22
**Requirements**: MIG-01, MIG-02, MIG-03, MIG-04, MIG-05, MIG-06, MIG-07, MIG-08
**Success Criteria** (what must be TRUE):
  1. All 9 symbol retrieval tools return typed errors with appropriate kinds for each failure mode (symbol not found, no workspace, LS unavailable)
  2. All 6 symbol editing tools return typed errors with appropriate kinds (invalid args, tree-sitter parse failure, diagnostic verification failure)
  3. All 6 file operation tools return typed errors with appropriate kinds (file not found, permission denied, path security violation)
  4. All 3 diagnostic tools, all 7 memory tools, all 2 workflow tools, all 2 profile tools, and all 3 MCP core tools return typed errors
  5. No tool in the codebase returns a raw error string -- every error path goes through the typed error constructors
**Plans**: TBD

### Phase 24: Validation & Testing
**Goal**: Tools validate inputs before execution, and the test suite asserts error types rather than string matching
**Depends on**: Phase 23
**Requirements**: VAL-01, VAL-02, VAL-03
**Success Criteria** (what must be TRUE):
  1. Passing invalid parameters to any tool (missing required fields, wrong types, empty strings where non-empty required) returns an InvalidArgs typed error before any work begins
  2. The existing three-band error tests assert on error Kind (e.g., errors.Is checks or Kind field comparisons) instead of substring matching on error messages
  3. Golden files capture the full error response shape per error kind, detecting regressions in error structure across releases
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 22 -> 23 -> 24

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
| 22. Error Taxonomy | v1.5 | 0/0 | Not started | - |
| 23. Tool Migration | v1.5 | 0/0 | Not started | - |
| 24. Validation & Testing | v1.5 | 0/0 | Not started | - |
