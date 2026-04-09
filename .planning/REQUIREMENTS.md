# Requirements: Serena v1.2 Performance & Production Hardening

**Defined:** 2026-04-09
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## v1.2 Requirements

### Benchmarks

- [ ] **BENCH-01**: Benchmark harness in `test/bench/` using `testing.B.Loop` (Go 1.25)
- [ ] **BENCH-02**: Tool response time benchmarks for all 38 tools (p50/p95/p99)
- [ ] **BENCH-03**: LSP indexing throughput benchmarks (cold and warm) for Go fixture
- [ ] **BENCH-04**: Memory profile benchmarks (baseline, per-workspace, per-LS-worker)
- [ ] **BENCH-05**: CI benchstat regression gate (fails on >10% time or >20% allocs at p<0.05)
- [ ] **BENCH-06**: v1.1 baselines committed to `test/bench/baselines/`

### Observability Foundation

- [ ] **OBS-01**: `internal/obs/` package with noop-default provider
- [ ] **OBS-02**: Trace-aware slog handler that injects trace_id/span_id from context
- [ ] **OBS-03**: Dedicated admin listener on loopback (configurable port, default disabled)
- [ ] **OBS-04**: `/healthz` and `/readyz` endpoints on admin listener
- [ ] **OBS-05**: Gated `/debug/pprof/*` endpoints (admin profile only)
- [ ] **OBS-06**: slog hot-path allocation budget ≤ +1 alloc/op vs Phase 9 baseline

### Metrics

- [ ] **METRIC-01**: `/metrics` endpoint on admin listener (Prometheus format)
- [ ] **METRIC-02**: RED histograms per tool (rate, errors, duration) with tuned buckets
- [ ] **METRIC-03**: lspool gauges (workers, evictions, restarts, circuit state)
- [ ] **METRIC-04**: Go runtime collectors (goroutines, GC, memory)
- [ ] **METRIC-05**: Bounded-label contract enforced by CI lint (allowlist: tool_name, profile, mode, language, outcome)

### Tracing

- [ ] **TRACE-01**: `otelgrpc` StatsHandlers on forwarder↔daemon gRPC
- [ ] **TRACE-02**: Telemetry middleware replacing logging middleware; runs before profile filter
- [ ] **TRACE-03**: Per-tool sub-spans for kernel operations
- [ ] **TRACE-04**: Optional OTLP exporter behind config flag
- [ ] **TRACE-05**: Default sampler `ParentBased(TraceIDRatioBased(0.0))` — off by default

### Graceful Degradation

- [ ] **DEGRADE-01**: `internal/degrade/` package with per-class timeout budgets (read 5s / search 15s / edit 10s / index 120s / diagnostics 20s)
- [ ] **DEGRADE-02**: Deadline propagation from forwarder → daemon → kernel → LS
- [ ] **DEGRADE-03**: Typed `lspool.ErrCircuitOpen` error with structured envelope
- [ ] **DEGRADE-04**: Circuit breaker tuning with decorrelated jitter and single-probe half-open
- [ ] **DEGRADE-05**: LS crash recovery with restart budget
- [ ] **DEGRADE-06**: `runtime/debug.SetMemoryLimit` wired from config
- [ ] **DEGRADE-07**: Graceful shutdown integration test (SIGTERM mid-request, spans flushed)

### Documentation

- [ ] **DOC-01**: README.md with project pitch, install instructions, capabilities overview
- [ ] **DOC-02**: README.md includes full 38-tool table (auto-generated from registry)
- [ ] **DOC-03**: README.md includes 52-language table with LS install commands
- [ ] **DOC-04**: README.md includes client configs for Claude Code, Codex, IDE assistants
- [ ] **DOC-05**: USAGE.md with profile/mode reference and config precedence
- [ ] **DOC-06**: USAGE.md with common workflow examples (onboarding, refactoring, code review)
- [ ] **DOC-07**: USAGE.md with troubleshooting guide (LS not starting, cache issues, mode restrictions)
- [ ] **DOC-08**: USAGE.md with observability quickstart (enable metrics, view traces)
- [ ] **DOC-09**: USAGE.md with performance tuning guide (memory limits, worker pool sizing)
- [ ] **DOC-10**: CHANGELOG.md with v1.0, v1.1, v1.2 entries

## Future Requirements

Deferred to v1.3+. Tracked but not in current roadmap.

### Production UX

- **DOCTOR-01**: `serena doctor` CLI command (LS availability, config sanity, perf hints)
- **METRIC-06**: Native histograms (Prometheus 2.40+) for bucket-tuning-free latency
- **OBS-07**: Auth on admin listener (currently loopback-only)

### Typed Errors

- **ERR-01**: Full typed-error migration across kernel tools (v1.2 introduces only `ErrCircuitOpen`)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Mandatory OTel pipeline | Heavy dep graph; tracing must be opt-in |
| Vendor APM clients (Datadog, New Relic) | OTLP is the vendor-neutral path |
| Docs site generator (mkdocs, hugo) | Markdown in repo is sufficient for v1.2 |
| Shipped Grafana dashboards | Operator concern, not platform concern |
| Load test harness | v1.2 focuses on benchmarking, not stress testing |
| Admin web UI | CLI + metrics scrape sufficient |
| Request hedging / retries | Belongs in client, not server |
| Full typed-errors migration | v1.2 introduces only `ErrCircuitOpen`; rest deferred |

## Traceability

Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| | | |

**Coverage:**
- v1.2 requirements: 38 total
- Mapped to phases: 0
- Unmapped: 38 ⚠️

---
*Requirements defined: 2026-04-09*
*Last updated: 2026-04-09 after initial definition*
