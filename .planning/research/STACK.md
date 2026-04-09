# Technology Stack — v1.2 Performance & Production Hardening

**Project:** Serena (Go MCP code intelligence platform)
**Milestone:** v1.2 — Benchmarks, observability, graceful degradation, documentation
**Researched:** 2026-04-08
**Go version:** 1.25.1

## Philosophy

Serena v1.0/v1.1 shipped a lean, opinionated dependency set (MCP SDK, koanf, modernc/sqlite, tree-sitter, gRPC). v1.2 adds **observability and benchmark tooling only** — no new frameworks, no runtime abstractions, no logging libraries that compete with `log/slog`. Every addition must justify itself against "use stdlib instead."

**Hard rules:**
- No new logging framework. `log/slog` (stdlib, since Go 1.21) is the logger.
- No new benchmark runner. `testing.B` (stdlib) is the runner.
- No APM SaaS clients. OTLP-only for tracing/metrics export; users BYO collector.
- No doc generators for README/USAGE — human-authored Markdown.

---

## Recommended Stack Additions

### 1. Benchmarks & CI Regression Detection

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `testing` (stdlib) | Go 1.25 | Benchmark runner via `testing.B` | Built-in, no dep. Already used for tests. |
| `golang.org/x/perf/cmd/benchstat` | latest | A/B statistical comparison of benchmark runs | Canonical Go benchmark tool. Confidence intervals, multi-run median, projections/filtering (2023 rewrite). Used by Go core team. |
| GitHub Actions workflow | n/a | CI gate: run benchmarks, compare with `benchstat`, fail on regression | No third-party action dependency. Shell + `benchstat` only. |

**Integration point:** New directory `internal/kernel/bench/` and/or `test/bench/` for benchmark files following Go convention (`*_test.go` with `func BenchmarkXxx(b *testing.B)`). Benchmarks exercise the same public kernel APIs the integration tests use — no separate harness.

**CI pattern:**
```bash
# On PR: run benchmarks on base and head, compare
go test -bench=. -benchmem -count=10 -run=^$ ./... > head.txt
git checkout main && go test -bench=. -benchmem -count=10 -run=^$ ./... > base.txt
benchstat base.txt head.txt
# Gate: fail if any benchmark regresses >10% at p<0.05 (parse benchstat output)
```

**Rejected alternatives:**

| Alternative | Why not |
|-------------|---------|
| `bobheadxi/gobenchdata` | Adds webapp + data store overhead. We want CI gate only. |
| `knqyf263/cob` | Less maintained. `benchstat` handles comparison natively. |
| `benchmark-action/github-action-benchmark` | Third-party action, stores data in gh-pages. Too much ceremony for a CI gate. |
| Custom regression scripts | Reinventing `benchstat`'s statistics (Mann-Whitney U, confidence intervals). |

---

### 2. Observability: Metrics (Prometheus-compatible)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/prometheus/client_golang/prometheus` | v1.23.x (latest stable) | Metric types (Counter, Histogram, Gauge), registry | De facto Go Prometheus client. Exposition format is the *standard* — grafana, alertmanager, datadog, victoriametrics all consume it. |
| `github.com/prometheus/client_golang/prometheus/promhttp` | same module | `/metrics` HTTP handler | Native integration with existing HTTP transport. Single line to wire. |
| `github.com/prometheus/client_golang/prometheus/collectors` | same module | `NewGoCollector`, `NewProcessCollector` | Free runtime + process metrics (goroutines, GC, RSS, fds). Mandatory for "memory profiles" requirement. |

**Integration point:** New package `internal/observ/metrics/` exposes a `Registry` constructed in `daemon.Bootstrap`. Kernel/lspool/memory instrument their hot paths via injected `*prometheus.Registry` (no globals). Exposed on Streamable HTTP transport at `/metrics` (already serving HTTP) and via optional sidecar listener for stdio mode.

**Metric taxonomy (minimum):**
- `serena_lsp_request_duration_seconds{lang,method}` — histogram (tool response times p50/p95/p99)
- `serena_lsp_indexing_loc_total{lang}` — counter (for LOC/sec derivation)
- `serena_worker_pool_active{lang}` — gauge (worker lifecycle)
- `serena_worker_pool_evictions_total{reason}` — counter (pressure/ttl/crash)
- `serena_circuit_breaker_state{worker}` — gauge (0/1/2 closed/open/half)
- `serena_mcp_tool_calls_total{tool,profile,status}` — counter
- Plus default Go/process collectors.

**Rejected alternatives:**

| Alternative | Why not |
|-------------|---------|
| OpenTelemetry metrics (`go.opentelemetry.io/otel/metric`) | Requirement wording is **Prometheus-compatible export**. OTel metrics requires either a collector or a Prometheus exporter bridge — added hops for zero benefit. Prom client is direct and battle-tested. |
| `go.opentelemetry.io/otel/exporters/prometheus` bridge | Two APIs to learn, same output. Unjustified indirection. |
| `expvar` (stdlib) | Not Prometheus format. No histograms. Not consumable by standard tooling. |
| `rcrowley/go-metrics` | Unmaintained; inferior to client_golang. |

**Note on OTel vs Prom for metrics:** If OTel metrics *also* lands later, Prometheus can still scrape via the OTel collector's Prometheus receiver. The reverse is also true. Starting with `client_golang` is the lower-risk path for v1.2 and does not foreclose future OTel adoption.

---

### 3. Observability: Structured Logging & Tracing

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `log/slog` (stdlib) | Go 1.25 | Structured logging, JSON/text handlers, levels, context propagation | Stdlib since Go 1.21. Zero-allocation hot path. Already the Go ecosystem default. No third-party alternative is justifiable in 2026. |
| `go.opentelemetry.io/otel` | v1.38.x (stable) | Tracing API + span context | Stable v1. Industry standard. Backend-agnostic. |
| `go.opentelemetry.io/otel/sdk` | v1.38.x | SDK: tracer provider, samplers, batch processor | Stable v1. |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | v1.38.x | OTLP/gRPC trace exporter | Single export format. Users point at any OTLP collector (Jaeger, Tempo, Honeycomb, Datadog, etc.). Reuses existing gRPC dep. |
| `go.opentelemetry.io/contrib/bridges/otelslog` | v0.13.x | slog → OTel trace context auto-injection | Official OTel bridge. Auto-adds `trace_id`/`span_id` to every log record when `InfoContext(ctx, ...)` is used. Enables log↔trace correlation with zero per-call code. |

**Integration point:** New package `internal/observ/` with two submodules:
- `internal/observ/log/` — slog handler factory (JSON for stdio/http, text for dev), level from config, wraps `otelslog` when tracing is enabled.
- `internal/observ/trace/` — OTel tracer provider init, OTLP gRPC exporter, `ParentBasedTraceIDRatio` sampler, shutdown hook registered in daemon lifecycle.

**Request ID / trace flow:**
1. MCP handler (in `internal/mcp/`) starts a span per tool call → injects `trace_id` into `context.Context`.
2. Kernel, lspool, memory all use `slog.InfoContext(ctx, ...)` — `otelslog` handler automatically stamps trace/span IDs.
3. LSP request timing via `prometheus` histogram + OTel span events.
4. Log output is JSON by default; `trace_id` field enables jumping from logs → traces in any backend.

**Config additions (koanf):**
```yaml
observability:
  log:
    level: info          # debug|info|warn|error
    format: json         # json|text
  metrics:
    enabled: true
    listen: ":9090"      # separate port or reuse http transport
  tracing:
    enabled: false       # off by default
    endpoint: ""         # OTLP gRPC endpoint (e.g. localhost:4317)
    sample_ratio: 0.01   # 1% default when enabled
    insecure: true
```

**Rejected alternatives:**

| Alternative | Why not |
|-------------|---------|
| `uber-go/zap` | Predates slog. No reason to take a new dep in 2026. slog matches zap perf for our workload. |
| `rs/zerolog` | Same. slog is stdlib. |
| `sirupsen/logrus` | Slower, in maintenance mode. |
| Direct OTel log bridge (`go.opentelemetry.io/otel/log`) | Still recent; slog + `otelslog` is the mature path. Logs-over-OTLP can be added later without changing application code. |
| Jaeger-native client (`jaegertracing/jaeger-client-go`) | Deprecated in favor of OTel. |
| DataDog tracer | Vendor lock-in. OTLP is the vendor-neutral answer. |

---

### 4. Graceful Degradation

**No new libraries required.** This is an architectural/tuning milestone leveraging what's already shipping:

| Existing component | Role in v1.2 |
|--------------------|--------------|
| `internal/kernel/lspool/` circuit breaker | Tune thresholds, expose `serena_circuit_breaker_state` metric, log state transitions |
| `internal/kernel/lspool/` pressure eviction | Instrument evictions, expose `serena_worker_pool_evictions_total{reason=...}` |
| `golang.org/x/sync/errgroup` (already used) | Daemon shutdown ordering (already correct) |
| `context` (stdlib) | Timeout budgets per tool call — pass ctx with deadline from MCP layer through kernel |
| `runtime/debug.SetMemoryLimit` (stdlib, Go 1.19+) | Soft memory ceiling for OOM avoidance. Verify current wiring in daemon and expose via config. |

**What NOT to add:**
- `sony/gobreaker`, `afex/hystrix-go` — we already have a circuit breaker in lspool.
- `uber-go/ratelimit` — no rate limiting requirement for v1.2.
- `cenkalti/backoff` — existing backoff implementation in lspool.

---

### 5. Documentation (README.md, USAGE.md)

**No tooling stack.** Hand-authored Markdown. Rejected:

| Alternative | Why not |
|-------------|---------|
| `mkdocs`, `docusaurus`, `hugo` | Adds a build step + JS/Python dep for two files. Overkill. GitHub renders Markdown natively. |
| `godoc`/`pkgsite` auto-gen | Already works for Go API docs. README/USAGE are *user-facing*, not API-facing. |
| `terraform-docs`-style generators | No schema to generate from. |

The only tooling question is whether `USAGE.md` needs **generated** tool reference tables. Recommendation:
- Write a small internal helper (`cmd/serena tools --format=markdown`) that dumps the tool registry with descriptions — zero new deps, reuses existing registry. Pipe its output into a `<!-- BEGIN:tools -->` block in USAGE.md and verify in CI that the block is current (simple `diff` gate). This keeps the tool inventory honest without a doc framework.

---

## Complete go.mod Delta

```bash
# Metrics
go get github.com/prometheus/client_golang@latest

# Tracing
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/sdk@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@latest
go get go.opentelemetry.io/contrib/bridges/otelslog@latest

# Benchmark comparison (CI only, not a runtime dep — install in CI)
go install golang.org/x/perf/cmd/benchstat@latest
```

**Net dependency additions at runtime:** 1 (prometheus/client_golang) + 4 (OTel modules, all from the same monorepo and versioned together). `benchstat` is a CI-only binary, not a module import.

**Transitive weight:** `client_golang` pulls `prometheus/common`, `prometheus/procfs`, `cespare/xxhash`, `beorn7/perks`, `golang/protobuf` — all well-maintained, small, no CGO. OTel pulls `go.opentelemetry.io/proto/otlp` and reuses existing gRPC/protobuf.

---

## Integration with Existing Stack

| Existing | Interaction |
|----------|-------------|
| MCP Go SDK v1.5.0 | Tool middleware wraps calls with OTel spans + metrics counters. No SDK changes. |
| koanf v2 config | New `observability:` section, resolved in `internal/config/` with profile-level overrides. |
| gRPC (forwarder↔daemon) | Reuse gRPC infrastructure; OTLP exporter uses its own client to keep control-plane and telemetry cleanly separated. |
| `internal/daemon/` bootstrap | New `InitObservability(cfg)` step before skill InitAll, shutdown hooked via existing errgroup pattern. |
| `internal/kernel/lspool/` | Instrument at pool boundaries; no API change to callers. |
| `internal/mcp/` profile middleware | Add tracing middleware *after* profile filter so filtered tools don't show up as spans. |
| stretchr/testify | Benchmarks don't need testify — use `testing.B` idioms. |

---

## What NOT to Add

This list exists to protect the dependency budget during v1.2:

- **No new logger.** slog only.
- **No service mesh / proxy libs** (envoy, linkerd clients).
- **No health-check frameworks.** A single `/healthz` handler in the HTTP transport suffices.
- **No feature-flag libraries.** Config already does this.
- **No profiler UIs.** `net/http/pprof` (stdlib) is sufficient — expose behind the HTTP transport gated by admin profile.
- **No error-tracking SaaS clients** (sentry, rollbar). Errors go to logs → collector.
- **No YAML/JSON schema validators for docs.** Markdown is Markdown.
- **No benchmark dashboards.** CI gate + artifact storage in GitHub Actions is enough for v1.2.

---

## Confidence Assessment

| Decision | Confidence | Basis |
|----------|------------|-------|
| `prometheus/client_golang` for metrics | HIGH | Official Prometheus project, stable v1.x, direct match for requirement wording ("Prometheus-compatible"). |
| `log/slog` for logging | HIGH | Stdlib since Go 1.21; Go 1.25 in use. No competitor justified. |
| OTel Go SDK v1.38.x for tracing | HIGH | Trace SDK is stable v1. Official. OTLP is vendor-neutral. |
| `otelslog` bridge for log/trace correlation | HIGH | Official OTel contrib bridge. Minimal surface. |
| `benchstat` for CI regression | HIGH | Canonical Go tool, used by Go core. 2023 rewrite is mature. |
| `testing.B` for benchmarks | HIGH | Stdlib. Already used ecosystem-wide. |
| Rejecting OTel metrics in favor of Prom client | MEDIUM | Defensible given requirement wording; revisit if v1.3 adds full OTel pipeline. |
| No doc framework | HIGH | Two files; Markdown on GitHub renders natively. |
| Tool-registry-dump helper for USAGE.md | MEDIUM | Pattern is sound but requires small design work; could defer to hand-maintained tables. |

---

## Pre-Implementation Checklist

Before Phase 1 of v1.2 starts, verify:

- [ ] `runtime/debug.SetMemoryLimit` — confirm current wiring in `internal/daemon/`
- [ ] HTTP transport port policy — reuse for `/metrics` and `/healthz` or separate listener?
- [ ] Profile gating — does `admin` profile gate pprof, `/metrics`, and `/healthz`? Document in FEATURES.md.
- [ ] Context propagation audit — does every kernel tool accept and forward `ctx`? (Required for trace propagation to work.)
- [ ] Benchmark fixture strategy — reuse `test/` integration fixtures or create `test/bench/` with larger corpora?

---

## Sources

- [prometheus/client_golang on pkg.go.dev](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus)
- [prometheus/client_golang releases](https://github.com/prometheus/client_golang/releases)
- [Instrumenting a Go application for Prometheus](https://prometheus.io/docs/guides/go-application/)
- [benchstat on pkg.go.dev](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)
- [Leveraging benchstat Projections — bwplotka, 2024](https://www.bwplotka.dev/2024/go-microbenchmarks-benchstat/)
- [Continuous benchmarking with Go and GitHub Actions](https://dev.to/vearutop/continuous-benchmarking-with-go-and-github-actions-41ok)
- [opentelemetry-go releases](https://github.com/open-telemetry/opentelemetry-go/releases)
- [OpenTelemetry Go documentation](https://opentelemetry.io/docs/languages/go/)
- [otelslog bridge on pkg.go.dev](https://pkg.go.dev/go.opentelemetry.io/contrib/bridges/otelslog)
- [OpenTelemetry Slog setup — Uptrace](https://uptrace.dev/guides/opentelemetry-slog)
- [Distributed Tracing with OpenTelemetry in Go (2026)](https://dev.to/young_gao/distributed-tracing-with-opentelemetry-a-practical-guide-for-go-services-pep)
- [Go structured logging with OpenTelemetry (2026)](https://oneuptime.com/blog/post/2026-01-07-go-structured-logging-opentelemetry/view)
