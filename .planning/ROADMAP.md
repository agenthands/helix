# Roadmap: Serena 2.0

## Milestones

- ✅ **v1.0 MVP** — Phases 1-5 (shipped 2026-04-08)
- ✅ **v1.1 Integration Testing** — Phases 6-8 (shipped 2026-04-09)
- 🚧 **v1.2 Performance & Production Hardening** — Phases 9-14 (in progress)

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

### 🚧 v1.2 Performance & Production Hardening (Phases 9-14)

**Goal:** Make Serena production-ready with measurable performance, observability, and graceful failure handling — plus comprehensive user-facing documentation.

**Granularity:** coarse · **Phases:** 6 · **Requirements mapped:** 38/38

**Ordering Constraint (Non-Negotiable):** Phases execute strictly sequentially. The critical meta-pitfall is that observing the thing you are benchmarking taints the benchmark. Phase 9 must publish the pre-instrumentation baseline before any observability code lands. Each subsequent phase publishes a benchstat delta vs. the previous phase's baseline as a delta gate (>10% time or >20% allocs at p<0.05 blocks close-out).

- [ ] **Phase 9: Benchmark Harness & v1.1 Baseline** — Capture pre-instrumentation performance baseline with `testing.B.Loop`, benchstat CI gate, and committed v1.1 numbers
- [x] **Phase 10: Observability Foundation** — `internal/obs/` shim, trace-aware slog handler, dedicated loopback admin listener with health endpoints and gated pprof (completed 2026-04-09)
- [x] **Phase 11: Metrics** — Prometheus `/metrics`, RED histograms per tool, lspool gauges, bounded-label contract enforced by CI lint (completed 2026-04-09)
- [x] **Phase 12: Tracing End-to-End** — `otelgrpc` gRPC propagation, telemetry middleware, per-tool sub-spans, off-by-default sampler, optional OTLP exporter (completed 2026-04-10)
- [ ] **Phase 13: Graceful Degradation** — `internal/degrade/` with per-class budgets, deadline propagation, circuit tuning, typed `ErrCircuitOpen`, `GOMEMLIMIT`
- [ ] **Phase 14: Documentation** — README.md, USAGE.md, CHANGELOG.md with auto-generated tool/language tables and executable examples

## Phase Details

### Phase 9: Benchmark Harness & v1.1 Baseline
**Goal**: Capture the pre-instrumentation performance baseline so every subsequent phase can publish a measurable delta
**Depends on**: Nothing (first phase of v1.2 — one-way door; must ship before any observability code)
**Requirements**: BENCH-01, BENCH-02, BENCH-03, BENCH-04, BENCH-05, BENCH-06
**Success Criteria** (what must be TRUE):
  1. A developer can run `go test -bench=. ./test/bench/...` and get reproducible p50/p95/p99 numbers for all 38 tools, cold/warm LSP indexing throughput, and memory profiles
  2. The v1.1 baseline numbers are committed under `test/bench/baselines/` and serve as the reference point for all v1.2 delta gates
  3. A CI job runs benchstat against the committed baseline and fails any PR that regresses >10% time or >20% allocs at p<0.05
  4. All benchmarks use `testing.B.Loop` so the Go compiler cannot elide the hot path
**Plans**: 6 plans
- [x] 09-01-PLAN.md — Refactor test/integration harness.go + helpers.go to testing.TB (Wave 0 blocker; unblocks bench reuse)
- [x] 09-02-PLAN.md — Scaffold test/bench/ package (rss, bench_helpers, TestMain, 38-tool manifest)
- [x] 09-03-PLAN.md — BenchmarkTools: all 38 MCP tools with b.Loop + 3-call warmup (BENCH-02)
- [x] 09-04-PLAN.md — LSP indexing cold/warm benches + full-repo smoke (BENCH-03)
- [x] 09-05-PLAN.md — BenchmarkMemory 4 D-06 scenarios + pprof heap snapshots + dual RSS (BENCH-04)
- [x] 09-06-PLAN.md — benchgate wrapper + v1.1 baseline + GitHub Actions workflow + Make targets (BENCH-05, BENCH-06)

### Phase 10: Observability Foundation
**Goal**: Stand up the observability plumbing (package, logging, admin surface) with near-zero hot-path cost so later phases can plug metrics and tracing into a single shim
**Depends on**: Phase 9 (baseline required for the allocation delta gate)
**Requirements**: OBS-01, OBS-02, OBS-03, OBS-04, OBS-05, OBS-06
**Success Criteria** (what must be TRUE):
  1. An operator can start the daemon with observability disabled and see no behavioral or performance change — the noop-default provider costs ≤ +1 alloc/op vs the Phase 9 baseline
  2. An operator can enable the loopback admin listener on a configured port and hit `/healthz` and `/readyz` to check daemon liveness and readiness
  3. Log records emitted during a traced request carry `trace_id` and `span_id` fields injected from context without manual plumbing at call sites
  4. `/debug/pprof/*` endpoints are reachable on the admin listener only when the active profile grants admin scope
**Plans**: 3 plans
- [x] 10-01-PLAN.md — Create internal/obs/ package (Noop provider, ContextHandler, SpanContext stub) + ObservabilityConfig schema + --admin-addr CLI flag (OBS-01, OBS-02)
- [x] 10-02-PLAN.md — Admin listener: telemetry.go with loopback validation, /healthz + /readyz + gated /debug/pprof/*, non-fatal errgroup wiring, ready atomic (OBS-03, OBS-04, OBS-05)
- [x] 10-03-PLAN.md — OBS-06 benchmark proof: slog hot-path bench + v1.2-phase10 baseline committed, benchgate delta gate against Phase 9 baseline (OBS-06)

### Phase 11: Metrics
**Goal**: Expose Prometheus-scrapeable RED metrics for tools and lspool health with a bounded-label contract that cannot silently explode cardinality
**Depends on**: Phase 10 (admin listener and obs shim)
**Requirements**: METRIC-01, METRIC-02, METRIC-03, METRIC-04, METRIC-05
**Success Criteria** (what must be TRUE):
  1. An operator can scrape `/metrics` on the admin listener and see RED histograms (rate, errors, duration) per tool, lspool gauges (workers, evictions, restarts, circuit state), and Go runtime collectors
  2. Tool latency histograms use SLO-tuned buckets that resolve p50/p95/p99 for the actual response-time distribution captured in Phase 9
  3. A CI lint step rejects any new metric whose label set falls outside the allowlist (`tool_name`, `profile`, `mode`, `language`, `outcome`)
  4. Enabling metrics produces no regression beyond the Phase 10 delta gate on the benchmark suite
**Plans**: 4 plans
- [x] 11-01-PLAN.md — obs.Metrics scaffold + label allowlist CI lint + /metrics handler on admin listener (METRIC-01, METRIC-04, METRIC-05)
- [x] 11-02-PLAN.md — TelemetryMiddleware (RED metrics) before ProfileFilterMiddleware + SessionInfo.Language (METRIC-02, resolves A1/A2)
- [x] 11-03-PLAN.md — lspool MetricsSink interface + pool/circuit hook wiring (METRIC-03)
- [x] 11-04-PLAN.md — BenchmarkTelemetryMiddleware + Phase 11 baseline + benchgate (≤ +3 allocs/op budget, resolves A4)

### Phase 12: Tracing End-to-End
**Goal**: Propagate traces from forwarder through daemon into kernel tool execution, off by default, with OTLP export as an opt-in flag
**Depends on**: Phase 11 (telemetry middleware slots in beside metrics)
**Requirements**: TRACE-01, TRACE-02, TRACE-03, TRACE-04, TRACE-05
**Success Criteria** (what must be TRUE):
  1. With sampling enabled, a single tool call produces a trace starting at the forwarder, crossing the gRPC boundary via `otelgrpc`, and showing per-tool kernel sub-spans in the exporter
  2. The default sampler is `ParentBased(TraceIDRatioBased(0.0))` — tracing is zero-cost unless explicitly turned on
  3. An operator can point Serena at an OTLP/gRPC collector via a config flag and see traces arrive without code changes
  4. The telemetry middleware runs before the profile filter so denied calls are still observable
**Plans**: 5 plans
- [x] 12-01-PLAN.md — obs package tracing foundation: Provider.Tracer(), WithTracing degraded-optional, real spanContextFromContext, config fields, TRACE-04/05 unit tests (Wave 1)
- [x] 12-02-PLAN.md — Daemon wiring: otelgrpc.NewServerHandler, TelemetryMiddleware span, dedicated 5s shutdown flush (Wave 2, TRACE-01 server / TRACE-02)
- [x] 12-03-PLAN.md — Forwarder provider + otelgrpc.NewClientHandler + forwarder.tools.call root span (Wave 2, TRACE-01 client)
- [x] 12-04-PLAN.md — Kernel tracer plumbing, WrapToolSpan helper across 24 tools, ls.request span events in lspool worker (Wave 3, TRACE-03)
- [x] 12-05-PLAN.md — Hot-path bench (D-17 ≤ +2 allocs/op), end-to-end integration tests (3-span tree + shutdown flush), Phase 12 baseline commit (Wave 3)

### Phase 13: Graceful Degradation
**Goal**: Make Serena survive slow LS workers, crashes, and memory pressure with typed errors, bounded budgets, and clean shutdown
**Depends on**: Phase 12 (uses tracing to verify deadline propagation end-to-end)
**Requirements**: DEGRADE-01, DEGRADE-02, DEGRADE-03, DEGRADE-04, DEGRADE-05, DEGRADE-06, DEGRADE-07
**Success Criteria** (what must be TRUE):
  1. A tool call exceeds its per-class budget (read 5s / search 15s / edit 10s / index 120s / diagnostics 20s) and returns a structured timeout error instead of hanging — the deadline is observable across forwarder, daemon, kernel, and LS spans
  2. When the lspool circuit is open, callers receive a typed `lspool.ErrCircuitOpen` with a structured envelope, and the half-open state admits exactly one probe with decorrelated jitter backoff
  3. A crashed language server is restarted within its restart budget; repeated crashes trip the circuit rather than looping
  4. Sending SIGTERM mid-request drains in-flight calls, flushes telemetry exporters within a separate 5s shutdown context, and exits cleanly — validated by the graceful shutdown integration test
**Plans**: TBD

### Phase 14: Documentation
**Goal**: Ship the user-facing docs (README, USAGE, CHANGELOG) so external users can install, configure, operate, and troubleshoot Serena without reading source
**Depends on**: Phase 13 (docs reference final metric names, profiles, and degradation behavior)
**Requirements**: DOC-01, DOC-02, DOC-03, DOC-04, DOC-05, DOC-06, DOC-07, DOC-08, DOC-09, DOC-10
**Success Criteria** (what must be TRUE):
  1. A new user can land on README.md, understand what Serena is, install it, wire it into Claude Code / Codex / an IDE assistant, and run a first tool call using only the documented commands
  2. USAGE.md gives an operator a complete reference for profiles, modes, config precedence, workflow examples, troubleshooting, observability quickstart, and performance tuning
  3. The 38-tool table in README and the 52-language table are generated from the registry, so they cannot drift from the code
  4. CHANGELOG.md records v1.0, v1.1, and v1.2 with dated entries and links to the milestone summaries
**Plans**: TBD

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
| 9. Benchmark Harness & v1.1 Baseline | v1.2 | 0/6 | Not started | - |
| 10. Observability Foundation | v1.2 | 3/3 | Complete    | 2026-04-09 |
| 11. Metrics | v1.2 | 4/4 | Complete    | 2026-04-09 |
| 12. Tracing End-to-End | v1.2 | 5/5 | Complete   | 2026-04-10 |
| 13. Graceful Degradation | v1.2 | 0/? | Not started | - |
| 14. Documentation | v1.2 | 0/? | Not started | - |
