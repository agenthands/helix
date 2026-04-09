# Feature Landscape: v1.2 Performance & Production Hardening

**Domain:** Production MCP/LSP gateway — benchmarks, observability, graceful degradation, user docs
**Researched:** 2026-04-08
**Scope:** Only features needed for the v1.2 milestone. Existing v1.0/v1.1 capabilities (38 tools, 52 languages, worker pool with circuit breaker/pressure eviction, integration test harness) are the baseline; this milestone adds the observability/perf/docs layer on top.

## Table Stakes

Features users (agent runtimes, platform teams deploying Serena) expect. Missing these makes the product feel unfinished for a "v1.2 production hardening" release.

### Benchmarks

| Feature | Why Expected | Complexity | Depends On |
|---------|--------------|------------|------------|
| Tool response latency bench (p50/p95/p99) for all 38 tools | Primary SLO unit for MCP clients; agents time out on slow tools | Medium | v1.1 test harness (`test/` package), InMemory transport |
| LSP indexing throughput (LOC/sec, files/sec, time-to-first-query) | Warm-up cost is the #1 user complaint for LSP gateways (see gopls issues) | Medium | Worker pool, language registry, multi-lang fixtures from v1.1 |
| Memory profile: baseline (daemon idle), per-workspace, per-LS-worker | LSP servers are the dominant memory cost; users need sizing guidance | Medium | `runtime/metrics`, pressure eviction telemetry |
| Go `testing.B` benchmarks checked into repo | Standard Go practice; enables `go test -bench` and benchstat locally | Low | Existing test infrastructure |
| `benchstat`-based regression comparison (HEAD vs base) | Benchmarks without diffing are decorative | Low | `golang.org/x/perf/cmd/benchstat` |
| CI benchmark gate with threshold alerting | Prevents perf regressions from landing unnoticed | Medium | GitHub Actions, benchstat or `benchmark-action/github-action-benchmark` |
| Cold-start vs warm-start separation in indexing benches | Share-until-dirty claims need to be measured, not asserted | Medium | Worker pool metrics |

### Observability

| Feature | Why Expected | Complexity | Depends On |
|---------|--------------|------------|------------|
| `log/slog` structured logging (stdlib, JSON handler) | Go 1.21+ standard; replaces whatever ad-hoc logging exists today | Low | Go stdlib only |
| Request/trace ID propagation through `context.Context` | Correlating a single MCP tool call across kernel -> pool -> LS is impossible without it | Medium | MCP middleware, kernel call sites |
| Per-tool span timing (even without full OTel) | `tool=find_symbol duration_ms=142 workspace=X` is the minimum viable trace | Medium | Middleware in `internal/mcp/` |
| Prometheus `/metrics` endpoint on HTTP transport | De facto standard; everyone scrapes Prometheus | Low | `prometheus/client_golang`, existing HTTP server |
| Core RED metrics: Rate, Errors, Duration per tool (histograms) | Prometheus table stakes for any RPC-style service | Low | client_golang histogram |
| LS worker pool gauges: workers alive, idle, busy, evictions, restarts, circuit state | Operators need visibility into the thing most likely to misbehave | Medium | Instrument existing pool in `internal/kernel/lspool/` |
| Process/runtime metrics (goroutines, heap, GC pause, open FDs) | `client_golang` provides these out of the box via `collectors.NewGoCollector` | Low | client_golang |
| Readiness and liveness endpoints (`/healthz`, `/readyz`) | Kubernetes and systemd expect these | Low | HTTP transport |
| Log-trace correlation: slog attrs include trace_id/span_id | Enables jumping from a log line to the full request trace | Low | slog handler wrapper |

### Graceful Degradation

| Feature | Why Expected | Complexity | Depends On |
|---------|--------------|------------|------------|
| Per-tool timeout budgets (default + per-tool overrides) | Agents enforce their own budgets; the server must respect them so it can't wedge | Medium | Config layer, kernel dispatch |
| Context deadline propagation from MCP call -> LSP request | Cancelling a slow `find_references` must actually cancel the LSP RPC | Medium | JSON-RPC codec, pool call paths |
| Circuit breaker telemetry + tuning knobs exposed in config | Circuit breaker already exists; v1.2 makes it observable and tunable | Low | Existing `internal/kernel/lspool/` |
| LS crash recovery: auto-restart with exponential backoff + restart budget | Share-until-dirty workers die; pool must not thrash respawning them | Medium | Worker lifecycle code, backoff jitter |
| OOM/pressure degraded mode: shed to fewer workers, refuse new workspaces with clear error | Pressure eviction exists; degraded mode makes it a first-class state instead of an eviction loop | Medium | Pressure eviction code in pool |
| Structured error responses distinguishing timeout / circuit open / LS crash / pressure | MCP clients need to know whether to retry, back off, or surface to the user | Medium | MCP error envelope, kernel error types |
| `GOMEMLIMIT` soft memory limit support (Go 1.19+) | Pairs with pressure eviction; prevents runaway heap before OS kills the process | Low | Go runtime + docs |
| Graceful shutdown: drain in-flight, refuse new, close LS workers in order | Signal-first lifecycle already exists — verify and document for production operators | Low | Existing daemon lifecycle |

### Documentation (README.md and USAGE.md)

| Feature | Why Expected | Complexity | Depends On |
|---------|--------------|------------|------------|
| README: one-paragraph pitch, capability bullets, install, quickstart | First-impression file; if it's bad, nothing else matters | Low | — |
| README: supported languages table (52) with LS installer tier | Users immediately ask "is my language supported?" | Low | `internal/langregistry/` |
| README: supported MCP clients with copy-pasteable config blocks | Claude Code, Codex, Zed, Cursor, generic stdio — each has a different config schema | Low | Profile docs |
| README: single-binary install (go install, release binaries, homebrew later) | Zero-friction install is the bar set by other Go tools | Low | GoReleaser or manual release |
| README: architecture diagram (forwarder -> daemon -> pool -> LS) | One picture answers half the "how does this work" questions | Low | — |
| USAGE: client setup for Claude Code (stdio + HTTP) | Primary target audience | Low | Profile configs |
| USAGE: client setup for Codex | Second target audience | Low | Profile configs |
| USAGE: client setup for generic IDE assistants / `ide-assistant` profile | Third target audience | Low | Profile configs |
| USAGE: profile + mode reference (what each profile exposes, how modes work) | Unique to Serena; not obvious from tool listings | Low | `internal/profile/` |
| USAGE: config precedence walkthrough (CLI > project > user > profile) | Four layers is one more than most tools; confusing without examples | Low | `internal/config/` |
| USAGE: onboarding/handoff workflow guide | Differentiator features, poorly discoverable without docs | Low | Workflow skills |
| USAGE: troubleshooting — LS install failures, timeout errors, permission issues | The top 5 support questions, answered once in docs | Low | — |
| USAGE: observability quickstart (Prometheus scrape, log format, trace IDs) | Pairs with the new v1.2 observability features | Low | v1.2 observability work |
| USAGE: performance tuning (pool sizing, TTL, `GOMEMLIMIT`) | Pairs with new benchmarks/degradation features | Low | v1.2 perf work |
| CHANGELOG.md with v1.0/v1.1/v1.2 entries | Standard OSS hygiene; users need to know what's new | Low | — |

## Differentiators

Not expected, but elevate Serena above "another MCP server."

| Feature | Value Proposition | Complexity | Depends On |
|---------|-------------------|------------|------------|
| Per-workspace metric labels (without high cardinality explosion) | Operators running multi-tenant Serena need to attribute cost; most MCP servers are single-workspace so this is uncommon | Medium | Label allowlist, workspace ID hashing |
| `serena doctor` CLI command: checks LS binaries, permissions, config, port binds, memory headroom | Turns "it doesn't work" bug reports into self-diagnosis; rare in MCP ecosystem | Medium | `internal/langregistry/installer`, config validation |
| Benchmark results published to repo (`bench/results/`) with history graph | Public perf claims beat private ones; github-action-benchmark renders this for free | Low | CI + gh-pages |
| Tool latency budgets declared in profile YAML (`max_duration_ms`) | Lets platform teams set SLOs declaratively per profile | Medium | Profile schema, middleware |
| Built-in pprof endpoints (`/debug/pprof/`) gated by admin mode | Go-native perf debugging; trivial to add, high value during incidents | Low | `net/http/pprof` |
| Trace export to OTLP (optional, off by default) | For teams already running OpenTelemetry collectors; keeps core dep-light | Medium | `go.opentelemetry.io/otel` (optional build tag or dep) |
| USAGE: a real "day in the life" example — Claude Code onboarding a repo, editing, handing off | Narrative docs outperform reference docs for adoption | Low | — |
| Degraded-mode banner in tool responses when pool is under pressure | Clients see `"status": "degraded", "reason": "memory_pressure"` in metadata, can adapt | Medium | MCP response envelope |
| Memory sizing calculator in USAGE.md (per language, per workspace) | Based on real v1.2 bench data; answers "how much RAM do I need?" | Low | v1.2 memory profiles |

## Anti-Features

Features to explicitly NOT build in this milestone. Either out of scope per PROJECT.md or premature for v1.2.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Full OpenTelemetry as a mandatory dependency | Heavy dep graph (`otel-sdk` pulls ~20 modules); most users don't need distributed tracing | Optional exporter behind build tag or config flag; default to slog + Prometheus only |
| Custom metrics DSL or framework | Reinventing Prometheus | Use `prometheus/client_golang` directly |
| Jaeger/Zipkin native exporters | OTLP is the standard; vendors ingest OTLP | If exporting traces at all, export OTLP only |
| APM agent integration (Datadog, New Relic SDKs) | Vendor lock-in; bloats binary | Prometheus scrape + OTLP exporter covers these vendors |
| In-process distributed tracing UI | Not Serena's job | Link to Jaeger/Tempo/Grafana in USAGE |
| Grafana dashboard JSON shipped as core artifact | Maintenance burden; Grafana versions drift | Ship a single example dashboard as optional contrib; document the metrics schema so anyone can build their own |
| Alert rules (PromQL) shipped in repo | Same reason — every org has different SLOs | Document the metrics; let users write their own alerts |
| Load testing harness / traffic replay tool | Benchmarks use `testing.B` and fixtures; load testing is a different tool (`k6`, `vegeta`) | Document "how to load test Serena with k6" in USAGE |
| Auto-scaling / horizontal pod autoscaler logic | Single-binary daemon; scaling is the operator's concern | Document memory/CPU characteristics so HPA can be configured externally |
| A dedicated "admin web UI" for metrics | Adds web framework, auth, templating — scope explosion | `/metrics` + pprof + logs; use Grafana for visualization |
| Benchmark against other MCP servers | Apples-to-oranges; political | Benchmark against self over time (regression gate) |
| Per-request hedging | Cuts p99 but doubles LS load; worker pool is already the bottleneck | Defer; revisit if benches show p99 >> p95 after v1.2 |
| Configurable retry policies inside the daemon | Clients own retries; server owns timeouts and circuit breakers | Document that clients should retry idempotent tools |
| Video tutorials / GIF-heavy docs | Rots fast, hard to maintain | Text-first docs with copy-pasteable commands |
| Docs site generator (Docusaurus, mkdocs) | README.md + USAGE.md in-repo is enough for v1.2 | Markdown in repo, rendered by GitHub |
| Localized docs (i18n) | Zero demand signal; massive maintenance | English-only |
| Knowledge graphs, vector search, git operations | Explicitly Out of Scope in PROJECT.md | — |

## Feature Dependencies

```
slog structured logging
    +-> trace ID context propagation
            +-> per-tool span timing (middleware)
                    +-> log-trace correlation in slog attrs
                    +-> OTLP trace export (optional differentiator)

Prometheus client_golang
    +-> /metrics endpoint on HTTP transport
    +-> RED metrics per tool (histograms)
    +-> LS pool gauges
    +-> Go runtime collector
    +-> per-workspace labels (differentiator)

testing.B benchmarks
    +-> tool latency p50/p95/p99
    +-> indexing throughput (cold vs warm)
    +-> memory profiles (runtime/metrics)
    +-> benchstat regression
            +-> CI gate (GitHub Actions)
                    +-> published bench history (differentiator)

Timeout budgets
    +-> context deadline propagation to LSP
    +-> structured timeout errors
    +-> per-profile max_duration_ms (differentiator)

Circuit breaker (existing)
    +-> telemetry (gauges above)
    +-> config-exposed tuning knobs
    +-> structured "circuit_open" error responses

Pressure eviction (existing)
    +-> GOMEMLIMIT integration
    +-> degraded-mode state machine
    +-> degraded-mode response banner (differentiator)

README.md
    +-> USAGE.md
            +-> client setup pages
            +-> profile/mode reference
            +-> troubleshooting
            +-> observability quickstart (depends on obs work landing first)
            +-> perf tuning (depends on bench work landing first)
```

**Key ordering constraint:** Observability instrumentation must land before or alongside benchmarks — you want benches producing the same metrics the prod server emits so regression triage is frictionless. Docs for obs/perf should be the last thing written, after the features stabilize.

## MVP Recommendation

If v1.2 had to ship in minimum viable form, prioritize in this order:

1. **slog + trace IDs + per-tool span timing** (foundational; unlocks everything else)
2. **Prometheus `/metrics` with RED + pool gauges + Go runtime collector** (standard observability contract)
3. **Tool latency benchmarks (p50/p95/p99) + indexing throughput + memory profiles** (measurable performance claims)
4. **benchstat + CI regression gate** (keep gains, prevent regressions)
5. **Timeout budgets + deadline propagation + structured error taxonomy** (degradation behavior)
6. **LS crash recovery with restart budget + OOM degraded mode** (fault tolerance)
7. **README.md** (install, capabilities, client configs, supported languages)
8. **USAGE.md** (client setup, profiles/modes, troubleshooting, obs/perf tuning)
9. **CHANGELOG.md** (standard hygiene)

Differentiators to consider pulling in if schedule allows: `serena doctor` CLI (high UX value), pprof endpoints (near-free), published bench history (near-free via github-action-benchmark), memory sizing calculator in USAGE (reuses bench data).

**Defer:** OTLP trace export (optional differentiator, behind a flag), per-workspace label cardinality work (only matters at multi-tenant scale), hedging (wait for bench data first).

## Dependencies on Existing v1.0/v1.1 Capabilities

- **v1.1 integration test harness** (`test/` package, InMemory + HTTP transports, multi-language fixtures) -> reused wholesale as the benchmark substrate. `testing.B` functions live alongside the existing `testing.T` functions, share fixture setup.
- **Centralized daemon bootstrap** (`internal/daemon/daemon.go`) -> the single place where slog handler, Prometheus registry, and middleware get wired. Already the registration choke point, extending it is the natural path.
- **ProfileFilterMiddleware** -> add a sibling `ObservabilityMiddleware` that wraps every tool call with start/stop timing, error classification, and trace-ID injection. Same pattern, same layer.
- **Worker pool** (`internal/kernel/lspool/`) -> already has circuit breaker, adaptive TTL, pressure eviction. v1.2 adds telemetry taps, restart budgets, degraded-mode state, and config-exposed tuning knobs. No structural rewrite.
- **JSON-RPC codec** (`internal/kernel/jsonrpc/`) -> needs to honor `ctx.Done()` on request cancellation so deadline propagation actually cancels in-flight LSP calls. Small but critical change.
- **MCP error envelope** -> needs structured error codes (`timeout`, `circuit_open`, `ls_crash`, `pressure`). Today errors are likely untyped per the `TODO(#typed-errors)` note in PROJECT.md Key Decisions — v1.2 is a good moment to partially address that for degradation taxonomy without doing the full typed-error migration.
- **Language registry + installer** -> already has the data for the README supported-languages table and for `serena doctor`. Just needs a formatter.
- **Memory subsystem (SQLite FTS5)** -> not touched by v1.2 observability except for exposing a gauge on index size.

## Sources

- [Scaling gopls for the growing Go ecosystem](https://go.dev/blog/gopls-scalability) — indexing/memory patterns, "per-package index" model, HIGH confidence
- [github-action-benchmark](https://github.com/benchmark-action/github-action-benchmark) — CI regression gate pattern, HIGH confidence
- [cob: Continuous Benchmark for Go](https://github.com/knqyf263/cob) — HEAD vs HEAD~1 benchstat pattern, MEDIUM confidence
- [Continuous benchmarking with Go and GitHub Actions](https://dev.to/vearutop/continuous-benchmarking-with-go-and-github-actions-41ok) — practical benchstat + CI walkthrough, MEDIUM confidence
- [Statistics Behind Latency Metrics: p90/p95/p99](https://medium.com/tuanhdotnet/statistics-behind-latency-metrics-understanding-p90-p95-and-p99-dc87420d505d) — percentile guidance (p50/p95 as budgets, p99 as warning), MEDIUM confidence
- [How to Define and Enforce Performance Budgets Using OpenTelemetry P50/P95/P99](https://oneuptime.com/blog/post/2026-02-06-otel-performance-budgets-latency-histograms/view) — hard vs soft budget split, MEDIUM confidence
- [Structured Logging in Go with slog for Observability and Alerting](https://dev.to/rosgluk/structured-logging-in-go-with-slog-for-observability-and-alerting-3fnm) — stdlib slog as baseline, HIGH confidence
- [OpenTelemetry Slog [otelslog]: Go Bridge](https://uptrace.dev/guides/opentelemetry-slog) — log-trace correlation via slog handler, MEDIUM confidence
- [How to Build Fault-Tolerant Services with Graceful Degradation in Go](https://oneuptime.com/blog/post/2026-01-25-fault-tolerant-graceful-degradation-go/view) — timeout budgets, circuit breaker, fallback patterns in Go, MEDIUM confidence
- [API Gateway Resilience and Fault Tolerance (Zuplo)](https://zuplo.com/learning-center/api-gateway-resilience-fault-tolerance) — layered resilience pattern stack, MEDIUM confidence
- [Circuit Breaker Pattern — Azure Architecture Center](https://learn.microsoft.com/en-us/azure/architecture/patterns/circuit-breaker) — canonical three-state model, HIGH confidence
- [github/github-mcp-server installation guides](https://github.com/github/github-mcp-server/blob/main/docs/installation-guides/README.md) — reference for multi-client install doc structure, HIGH confidence
- [modelcontextprotocol/servers README](https://github.com/modelcontextprotocol/servers) — ecosystem conventions for MCP server READMEs, HIGH confidence
