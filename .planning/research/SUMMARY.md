# Research Synthesis: Serena v1.2 Performance & Production Hardening

**Researched:** 2026-04-09
**Confidence:** HIGH

## Executive Summary

Serena v1.0/v1.1 already shipped the hard parts — 4-layer platform, worker pool with circuit breaker/adaptive TTL/pressure eviction, 38 tools, 52 languages, integration harness. **v1.2 is a hardening milestone, not a feature milestone**: observability, benchmarking, degradation tuning, and user docs. All four research files converge on stdlib-first (`log/slog`, `testing.B`, `benchstat`) plus one Prom client and one OTel SDK, funneled through a new `internal/obs/` shim.

The approach is a thin additive delta: `prometheus/client_golang` + 4 OTel modules, a new `internal/obs/` package holding the trace-aware slog handler and pre-registered metric vectors, a **dedicated loopback admin listener** (`/metrics`, `/healthz`, `/readyz`, gated pprof) separate from the MCP transport, and an `internal/degrade/` package centralizing per-class timeout budgets. Existing lspool circuit breaker, pressure eviction, and errgroup shutdown are **instrumented and tuned**, not rewritten.

The dominant risk is a meta-pitfall: **observing the thing you are benchmarking taints the benchmark**. This single concern drives the phase ordering decision below.

## Key Findings

### Stack Choices
- **`log/slog`** (stdlib) — use `slog.LogAttrs` on hot paths to avoid boxing
- **`prometheus/client_golang` v1.23.x** — chosen over OTel metrics SDK (requirement says "Prometheus-compatible")
- **`go.opentelemetry.io/otel` v1.38.x + OTLP/gRPC exporter** — tracing off by default
- **`otelslog` bridge** — auto-injects trace_id/span_id into slog records
- **`otelgrpc` StatsHandler** — forwarder↔daemon traceparent propagation
- **`testing.B` + `benchstat`** — use Go 1.24+ `testing.B.Loop` to prevent elision

Net runtime additions: 1 Prom + 4 OTel modules. No CGO.

### Feature Landscape

**Table stakes:**
- Benchmarks: p50/p95/p99 for 38 tools, LSP indexing throughput, memory profiles, benchstat CI gate
- Structured logging: slog JSON, trace-ID propagation, log↔trace correlation
- Metrics: `/metrics`, RED per tool, pool gauges, Go runtime collectors, `/healthz`/`/readyz`
- Degradation: per-tool timeout budgets, deadline propagation, structured error taxonomy, LS crash recovery, `GOMEMLIMIT`, graceful drain
- Docs: README (pitch, install, 52-lang table, client configs), USAGE.md (client setup, profile/mode reference, troubleshooting, observability quickstart), CHANGELOG

**Differentiators:** `serena doctor` CLI, pprof admin endpoints, optional OTLP exporter, memory sizing calculator

**Anti-features:** mandatory OTel pipeline, vendor APM, docs site generator, shipped Grafana dashboards, load-test harness, admin web UI, full typed-errors migration (v1.2 introduces exactly one: `lspool.ErrCircuitOpen`)

### Architecture Approach

Additive, not restructured. No kernel/skill signature changes.

1. **`internal/obs/`** (new) — shim over OTel + Prom. Noop by default.
2. **Dedicated admin listener** (`internal/daemon/telemetry.go`) — separate net.Listener on `127.0.0.1:0`, hosts `/metrics`, `/healthz`, `/readyz`, gated pprof. **Not shared with MCP mux.** Bind failure is non-fatal.
3. **Telemetry middleware** replaces logging middleware; runs **before** profile filter.
4. **`internal/degrade/`** (new) — per-class budgets (read 5s / search 15s / edit 10s / index 120s / diagnostics 20s); applied at tool handler entry.
5. **lspool instrumentation** — lease wait/hold histograms, eviction reason labels, circuit state gauge, crash counters, RSS gauge.
6. **Trace propagation** — `otelgrpc` StatsHandlers on forwarder↔daemon gRPC; kernel tools add sub-spans.
7. **`test/bench/`** — macro benches as `package bench_test`; micro benches co-located.

### Critical Pitfalls

1. **Meta-pitfall: observing the benchmarked thing** — mitigate via strict ordering (benchmarks first) + per-phase delta reports
2. **Prometheus cardinality explosion** — bounded-label contract in the same PR as the first metric
3. **Benchmark compiler elision** — mandate `testing.B.Loop`, `-count=10`, benchstat at p<0.05
4. **Flush lost on SIGTERM** — separate shutdown context (5s) for exporters
5. **Circuit breaker thundering herd** — decorrelated jitter on probes, exactly one probe in half-open
6. **Timeout budgets double-count** — propagate deadlines, not durations
7. **OTel overhead** (~20-35% CPU reported) — mitigate with low default sampler
8. **slog hot-path allocations** — enforce `slog.LogAttrs` with typed attrs
9. **Docs rot** — executable examples in CI, code-generated tool/profile tables

## Implications for Roadmap

### Phase Ordering — Resolved Tension

**Tension:** FEATURES recommended logging → metrics → benchmarks → degradation → docs. PITFALLS recommended benchmarks → logging → metrics → tracing → degradation → docs.

**Recommendation: follow PITFALLS ordering** — the meta-pitfall is the only one-way door in v1.2. Once observability lands, you cannot retroactively measure the pre-instrumentation baseline.

### Suggested Phase Structure (6 phases)

**Phase 9 — Benchmark Harness & v1.1 Baseline** (must be first)
Delivers: `test/bench/` + micro benches, `testing.B.Loop`, baselines committed, CI benchstat gate.

**Phase 10 — Observability Foundation**
Delivers: `internal/obs/` package, trace-aware slog handler, admin listener with `/healthz`/`/readyz`/gated pprof.
Delta gate: slog hot-path ≤ +1 alloc/op vs Phase 9 baseline.

**Phase 11 — Metrics**
Delivers: `/metrics` on admin listener, RED histograms per tool with SLO-tuned buckets, lspool gauges, bounded-label contract + CI lint.

**Phase 12 — Tracing End-to-End**
Delivers: `otelgrpc` StatsHandlers, telemetry middleware, per-tool-package sub-spans, `ParentBased(TraceIDRatioBased(0.0))` default, optional OTLP exporter.

**Phase 13 — Graceful Degradation**
Delivers: `internal/degrade/` with per-class budgets, deadline-propagation audit, circuit breaker tuning, typed `lspool.ErrCircuitOpen`, `GOMEMLIMIT`, chaos test.

**Phase 14 — Documentation** (drafting can parallelize)
Delivers: README.md (capabilities, install, 52-lang table, client configs), USAGE.md (client setup, profile/mode reference, troubleshooting, observability quickstart, perf tuning), CHANGELOG.md, executable example smoke tests.

## Confidence Assessment

| Area | Confidence |
|---|---|
| Stack | HIGH |
| Features | HIGH |
| Architecture | HIGH |
| Pitfalls | MEDIUM-HIGH |

**Overall: HIGH**

### Gaps to Address
- Phase 9: benchmark runner strategy (self-hosted vs GitHub-hosted)
- Phase 10: admin profile gating for pprof/metrics
- Phase 12: otelgrpc import path verification; ctx-propagation audit
- Phase 11: cardinality headroom review

---

*Synthesized: 2026-04-09*
