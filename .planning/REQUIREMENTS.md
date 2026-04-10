# Requirements: Serena v1.2 Performance & Production Hardening

**Defined:** 2026-04-09
**Core Value:** Rock-solid LSP-backed MCP runtime that survives client disconnects, shares warm caches across sessions, and exposes semantic code operations as tools.

## v1.2 Requirements

### Benchmarks

- [x] **BENCH-01**: Benchmark harness in `test/bench/` using `testing.B.Loop` (Go 1.25)
- [x] **BENCH-02**: Tool response time benchmarks for all 38 tools (p50/p95/p99)
- [x] **BENCH-03**: LSP indexing throughput benchmarks (cold and warm) for Go fixture
- [x] **BENCH-04**: Memory profile benchmarks (baseline, per-workspace, per-LS-worker)
- [x] **BENCH-05**: CI benchstat regression gate with tiered thresholds per 09-CONTEXT.md D-01 — PR tier (GitHub-hosted, relaxed): >15% time / >25% allocs at p<0.05; release tier (self-hosted, tight): >10% time / >20% allocs at p<0.05 (partial: gate works, baseline platform mismatch — darwin not ubuntu)
- [x] **BENCH-06**: v1.1 baselines committed to `test/bench/baselines/` (partial: real numbers but captured locally, not from CI ubuntu-latest)

### Observability Foundation

- [x] **OBS-01**: `internal/obs/` package with noop-default provider
- [x] **OBS-02**: Trace-aware slog handler that injects trace_id/span_id from context
- [x] **OBS-03**: Dedicated admin listener on loopback (configurable port, default disabled)
- [x] **OBS-04**: `/healthz` and `/readyz` endpoints on admin listener
- [x] **OBS-05**: Gated `/debug/pprof/*` endpoints (admin profile only)
- [x] **OBS-06**: slog hot-path allocation budget ≤ +1 alloc/op vs Phase 9 baseline

### Metrics

- [x] **METRIC-01**: `/metrics` endpoint on admin listener (Prometheus format)
- [x] **METRIC-02**: RED histograms per tool (rate, errors, duration) with tuned buckets
- [x] **METRIC-03**: lspool gauges (workers, evictions, restarts, circuit state)
- [x] **METRIC-04**: Go runtime collectors (goroutines, GC, memory)
- [x] **METRIC-05**: Bounded-label contract enforced by CI lint (allowlist: tool_name, profile, mode, language, outcome)

### Tracing

- [x] **TRACE-01**: `otelgrpc` StatsHandlers on forwarder↔daemon gRPC
- [x] **TRACE-02**: Telemetry middleware replacing logging middleware; runs before profile filter
- [x] **TRACE-03**: Per-tool sub-spans for kernel operations
- [x] **TRACE-04**: Optional OTLP exporter behind config flag
- [x] **TRACE-05**: Default sampler `ParentBased(TraceIDRatioBased(0.0))` — off by default

### Graceful Degradation

- [x] **DEGRADE-01**: `internal/degrade/` package with per-class timeout budgets (read 5s / search 15s / edit 10s / index 120s / diagnostics 20s)
- [x] **DEGRADE-02**: Deadline propagation from forwarder → daemon → kernel → LS
- [x] **DEGRADE-03**: Typed `lspool.ErrCircuitOpen` error with structured envelope
- [x] **DEGRADE-04**: Circuit breaker tuning with decorrelated jitter and single-probe half-open
- [x] **DEGRADE-05**: LS crash recovery with restart budget
- [x] **DEGRADE-06**: `runtime/debug.SetMemoryLimit` wired from config
- [x] **DEGRADE-07**: Graceful shutdown integration test (SIGTERM mid-request, spans flushed)

### Documentation

- [x] **DOC-01**: README.md with project pitch, install instructions, capabilities overview
- [x] **DOC-02**: README.md includes full 38-tool table (auto-generated from registry)
- [x] **DOC-03**: README.md includes 52-language table with LS install commands
- [x] **DOC-04**: README.md includes client configs for Claude Code, Codex, IDE assistants
- [x] **DOC-05**: USAGE.md with profile/mode reference and config precedence
- [x] **DOC-06**: USAGE.md with common workflow examples (onboarding, refactoring, code review)
- [x] **DOC-07**: USAGE.md with troubleshooting guide (LS not starting, cache issues, mode restrictions)
- [x] **DOC-08**: USAGE.md with observability quickstart (enable metrics, view traces)
- [x] **DOC-09**: USAGE.md with performance tuning guide (memory limits, worker pool sizing)
- [x] **DOC-10**: CHANGELOG.md with v1.0, v1.1, v1.2 entries

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

| Requirement | Phase | Status |
|-------------|-------|--------|
| BENCH-01 | Phase 9 | Complete |
| BENCH-02 | Phase 9 | Complete |
| BENCH-03 | Phase 9 | Complete |
| BENCH-04 | Phase 9 | Complete |
| BENCH-05 | Phase 15 | Complete (partial — baseline platform mismatch) |
| BENCH-06 | Phase 15 | Complete (partial — local capture, not CI) |
| OBS-01 | Phase 10 | Complete |
| OBS-02 | Phase 10 | Complete |
| OBS-03 | Phase 10 | Complete |
| OBS-04 | Phase 10 | Complete |
| OBS-05 | Phase 10 | Complete |
| OBS-06 | Phase 10 | Complete |
| METRIC-01 | Phase 11 | Complete |
| METRIC-02 | Phase 11 | Complete |
| METRIC-03 | Phase 11 | Complete |
| METRIC-04 | Phase 11 | Complete |
| METRIC-05 | Phase 11 | Complete |
| TRACE-01 | Phase 12 | Complete |
| TRACE-02 | Phase 12 | Complete |
| TRACE-03 | Phase 12 | Complete |
| TRACE-04 | Phase 12 | Complete |
| TRACE-05 | Phase 12 | Complete |
| DEGRADE-01 | Phase 13 | Complete |
| DEGRADE-02 | Phase 13 | Complete |
| DEGRADE-03 | Phase 13 | Complete |
| DEGRADE-04 | Phase 13 | Complete |
| DEGRADE-05 | Phase 13 | Complete |
| DEGRADE-06 | Phase 13 | Complete |
| DEGRADE-07 | Phase 13 | Complete |
| DOC-01 | Phase 14 | Complete |
| DOC-02 | Phase 14 | Complete |
| DOC-03 | Phase 14 | Complete |
| DOC-04 | Phase 14 | Complete |
| DOC-05 | Phase 14 | Complete |
| DOC-06 | Phase 14 | Complete |
| DOC-07 | Phase 14 | Complete |
| DOC-08 | Phase 14 | Complete |
| DOC-09 | Phase 14 | Complete |
| DOC-10 | Phase 14 | Complete |

**Coverage:**
- v1.2 requirements: 38 total
- Complete: 36 (BENCH-01..04, OBS-01..06, METRIC-01..05, TRACE-01..05, DEGRADE-01..07, DOC-01..10)
- Partial: 2 (BENCH-05, BENCH-06 — gate works, baseline platform mismatch)
- Mapped to phases: 38
- Unmapped: 0

---
*Requirements defined: 2026-04-09*
*Last updated: 2026-04-10 — gap closure phases assigned, 15 stale checkboxes fixed*
