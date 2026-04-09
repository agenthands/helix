# Architecture: Observability, Graceful Degradation, Benchmarks

**Milestone:** v1.2 Performance & Production Hardening
**Scope:** How NEW features (metrics, logs, traces, graceful degradation, benchmarks) integrate with the existing 4-layer Go MCP platform.
**Researched:** 2026-04-08
**Confidence:** HIGH for Go stdlib/OTel/Prom patterns; MEDIUM for MCP SDK middleware specifics (verified via existing code).

---

## 1. Guiding Principles

1. **Single binary, single process.** No sidecars. Observability runs inside the daemon.
2. **One context, one trace.** `context.Context` is already threaded daemon -> MCP -> kernel -> lspool. Trace IDs ride that context; do not invent a parallel carrier.
3. **Slog stays the ingress.** Existing `slog.Logger` is passed everywhere. Do NOT rip it out — wrap its handler so every log line gets trace/span IDs for free.
4. **Additive, not invasive.** No kernel/skill signatures change. New wiring happens in `internal/daemon/daemon.go` bootstrap and via new middleware / handler decorators.
5. **Cheap-by-default.** Metrics are always-on (sub-percent overhead). Tracing is sampled (default 0% remote, on-demand). Profiling endpoints gated behind admin profile.
6. **Never break error paths.** Telemetry failures are swallowed and logged. A dropped span must not propagate to tool callers.

---

## 2. Current-State Recap (what we're integrating with)

Verified from code (`internal/daemon/daemon.go`, `internal/mcp/middleware.go`, `internal/kernel/lspool/`):

| Layer | Existing surface | Relevant for v1.2 |
|-------|------------------|-------------------|
| Forwarder | `internal/forwarder/dial.go` uses `grpc.NewClient` | Client-side gRPC interceptor injection point |
| Daemon gRPC | `grpc.NewServer()` in `daemon.listenSocket` | Server-side gRPC interceptor injection point |
| HTTP transport | `http.ServeMux` at `/mcp` in `daemon.listenHTTP` | Add new `/metrics`, `/debug/pprof/*`, `/healthz` routes OR new mux on new listener |
| MCP server | `mcpsdk.AddReceivingMiddleware` used for logging + profile filter | Single chain point for request-level telemetry |
| Errgroup | `daemon.Run` orchestrates kernel + socket + HTTP | Add metrics listener as new `g.Go(...)` sibling |
| Worker pool | `lspool.Pool.AcquireLease`, circuit breaker in `lspool/circuit.go` | Already has degradation primitives; v1.2 wires metrics to them, adds tuning knobs |
| Logger | `slog.Logger` passed by value from daemon to every subsystem | Replace the root `slog.Handler` with a trace-aware wrapper |

Current logging middleware (`mcp/middleware.go:88-110`) logs method + duration + error. It is the obvious seed for the telemetry middleware; v1.2 expands it in place rather than adding a second chain.

---

## 3. New Components

### 3.1 `internal/obs/` (new package)

Small, dependency-light shim over OpenTelemetry + Prometheus. All subsystems import from here; nothing else imports OTel/Prom directly. This keeps vendoring swap-out cheap and prevents a metrics rewrite from touching 30 files.

```
internal/obs/
|-- obs.go          # Provider: holds tracer, meter, prom registry, slog handler
|-- handler.go      # slog.Handler wrapper that injects trace_id/span_id from ctx
|-- meter.go        # Typed metric constructors (counters/histograms) + registry
|-- tracer.go       # Tracer factory + noop fallback
|-- config.go       # ObsConfig (port, sampler, exporter type, enabled flags)
`-- testing.go      # In-memory exporter for unit tests
```

**Key decisions:**

- **Tracing library:** `go.opentelemetry.io/otel` + `otel/sdk/trace`. Industry standard, MCP/LSP have no alternative. HIGH confidence.
- **Metrics library:** Prometheus native (`prometheus/client_golang`) directly — NOT via OTel metrics bridge. Rationale: OTel metrics SDK is still heavier and noisier than prom client; our target is a `/metrics` endpoint, not OTLP metrics push. We already own the Prom endpoint. MEDIUM confidence — revisit if we need OTLP metrics push later.
- **No global state.** `obs.Provider` is constructed in `daemon.New()` and passed explicitly, same pattern as `logger`. Avoids the OTel `otel.SetTracerProvider` global footgun which breaks tests that run in parallel.
- **Noop by default.** If `cfg.Observability.Enabled == false` or init fails, `Provider` returns noop tracers/meters. Downstream code never branches on `if provider != nil`.

### 3.2 `internal/daemon/telemetry.go` (new file)

Thin wiring file. Creates the obs provider, registers runtime/process collectors (Go GC stats, goroutine count, RSS), exposes `/metrics` + `/healthz` + `/debug/pprof/*` on a **dedicated admin listener** (see section 4.1).

### 3.3 `internal/mcp/telemetry_middleware.go` (new file, or fold into `middleware.go`)

Replaces `loggingMiddleware` with `telemetryMiddleware(provider)`. Single pass: start span -> record duration histogram -> increment counter -> log. Existing `InstallMiddleware` signature grows one parameter or accepts a `Provider` struct.

### 3.4 `internal/kernel/lspool/metrics.go` (new file)

Pool-internal Prom metrics: lease acquire latency, lease queue depth, worker startup time, circuit breaker state gauge, crashes total, pressure evictions total, RSS per worker. Pool already holds a logger; now also holds a `*obs.Meter`. No external API change.

### 3.5 `test/bench/` (new directory, top-level, public-API only)

Benchmarks follow the existing `test/integration/` black-box precedent — external package (`package bench_test`) driving the daemon through its public API via the in-process MCP harness. See section 6.

### 3.6 `internal/degrade/` (new package, small)

Home for timeout budgets and deadline propagation helpers. Separates policy (what timeout for what operation class) from mechanism (`context.WithTimeout`). Lets us tune budgets in one file without grepping the codebase.

```
internal/degrade/
|-- budget.go       # Per-operation-class timeouts (read / edit / search / index)
|-- deadline.go     # EnsureBudget(ctx, class) helper — applies budget if none set
`-- classify.go     # Map tool names -> operation classes
```

Existing circuit breaker stays in `lspool/circuit.go` — v1.2 just exposes its state via metrics and adds config-driven tuning knobs.

---

## 4. Component Diagram (v1.2 additions highlighted with *)

```
                       +-------------------------------+
 mcp client --stdio--> |   internal/forwarder          |
                       |   + gRPC client interceptor * |  (trace propagation)
                       +---------------+---------------+
                                       | gRPC (unix socket)
                       +---------------v---------------+
                       |   internal/daemon             |
                       |                               |
                       |   errgroup:                   |
                       |   |- kernel.Run               |
                       |   |- listenSocket (gRPC)      |
                       |   |  + server interceptor *   |
                       |   |- listenHTTP  (MCP)        |
                       |   |- listenAdmin *  <-- NEW   |-- /metrics, /healthz, /readyz, /debug/pprof/*
                       |   `- telemetry.Shutdown *     |
                       |                               |
                       |   +-------------------------+ |
                       |   | MCP server              | |
                       |   |  middleware chain:      | |
                       |   |   1. telemetry * (was   | |
                       |   |      logging)           | |
                       |   |   2. profile filter     | |
                       |   +----------+--------------+ |
                       |              | ctx w/ span    |
                       |   +----------v--------------+ |
                       |   | kernel / skills         | |
                       |   |  + tool-scoped spans *  | |
                       |   |  + degrade.EnsureBudget*| |
                       |   +----------+--------------+ |
                       |              |                |
                       |   +----------v--------------+ |
                       |   | lspool                  | |
                       |   |  + metrics *            | |
                       |   |  + breaker gauge *      | |
                       |   +----------+--------------+ |
                       +--------------+----------------+
                                      | JSON-RPC/stdio
                                      v
                              language servers
```

### 4.1 Metrics endpoint placement — **dedicated listener, not shared**

Three options were considered:

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| A. Share MCP HTTP mux (`/metrics` alongside `/mcp`) | Zero new config, one port | MCP transport and admin surface share auth/ACL/rate limits; scraping noise competes with tool traffic; any panic on `/metrics` takes down the MCP handler's ServeMux; profile filter middleware shouldn't apply to Prom scraping | no |
| B. Separate `net.Listener` on dedicated admin addr | Clean separation, distinct bind address means ops can firewall it internally, pprof/healthz live with metrics naturally, cannot be scraped by random MCP clients, no middleware cross-talk | One extra config field | **chosen** |
| C. Push-only (OTLP/Pushgateway) | No listener | Requires external infra, breaks single-binary story, hurts local-dev onboarding | no |

**Decision:** New `cfg.Observability.AdminAddr` (default `127.0.0.1:0` — off unless configured, bound to loopback when enabled). New `g.Go(d.listenAdmin)` sibling in `Daemon.Run`. Hosts:

- `/metrics` — Prom exposition
- `/healthz` — liveness (always 200 if process alive)
- `/readyz` — readiness (200 once kernel + profile resolved + at least one listener up)
- `/debug/pprof/*` — gated: only mounted if `cfg.Observability.Pprof == true` AND active profile has admin mode available

Rationale for loopback default: avoids accidental public exposure; operators who want remote scraping opt in explicitly. Matches gopls/etcd/CockroachDB conventions. HIGH confidence — this is standard Go service hygiene.

### 4.2 Trace propagation forwarder -> daemon -> kernel -> LS

**Forwarder -> daemon (gRPC):**

Add interceptors to the existing `grpc.NewServer()` in `daemon.listenSocket` and `grpc.NewClient` in `forwarder/dial.go`:

```go
// daemon side
d.grpcServer = grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler()),
)
// forwarder side
conn, err := grpc.NewClient(addr,
    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
    ...,
)
```

`otelgrpc` (from `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`) auto-extracts W3C traceparent from gRPC metadata and establishes a span on the server side. The MCP SDK's stream handler runs under that span's context, so everything downstream inherits it without code changes. HIGH confidence — `otelgrpc` is the canonical integration and the correct API surface is `StatsHandler` (not the older deprecated `UnaryInterceptor`/`StreamInterceptor`). Verify exact import path via Context7 at implementation time.

**Daemon MCP middleware -> kernel:**

The telemetry middleware starts a child span keyed on `method` (e.g., `mcp.tools/call`) and, for `tools/call`, also annotates with `tool.name`. Kernel tools that do meaningful work create sub-spans using `provider.Tracer("serena.kernel").Start(ctx, "symbols.find_references")`. Attributes: `workspace.lang`, `workspace.root` (hash, not path — PII), `result.size`, `cache.hit`.

**Kernel -> LS (JSON-RPC):**

LS processes do not speak OTel. Tracing stops at the JSON-RPC boundary. Instead, we record a **span event** `"ls.request"` with attributes `ls.method`, `ls.duration_ms`, `ls.language`, `ls.worker_id`. This gives us LS latency visibility without pretending LS is OTel-aware. If someday LSP gains trace context extensions, we extend the JSON-RPC codec — until then, span events are the right abstraction.

### 4.3 Logs <-> traces correlation

Wrap the existing `slog.Handler` once in `daemon.New()`:

```go
baseHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
handler := obs.NewContextHandler(baseHandler) // injects trace_id, span_id from ctx
logger := slog.New(handler)
```

`obs.NewContextHandler` implements `slog.Handler` and overrides `Handle(ctx, r)` to pull the active span from `ctx` and append `trace_id` / `span_id` attributes. Zero changes at call sites — every existing `logger.Info("...", ...)` that receives a ctx-bearing call path (via the middleware) automatically gains correlation IDs. HIGH confidence — this is the documented slog extension pattern.

**Caveat:** many existing log calls in lspool/kernel use `logger.Info(...)` without a ctx. The handler just omits the IDs in those cases — no panic, no error. We do NOT do a big sweep to pipe ctx into every log call. Incremental: high-value paths (request lifecycle, tool execution, worker lease) get ctx-aware logging; background chatter stays ctx-free.

### 4.4 Error paths — non-negotiable invariants

1. **Span recording failures are swallowed.** `otelgrpc` and the SDK's exporter handle this; obs provider uses `otel.SetErrorHandler` to route internal OTel errors to slog at `Warn` level, never propagated.
2. **Metric increment is allocation-free on hot path.** Use pre-registered labeled instruments. No `WithLabelValues` on hot path without pre-computed vectors.
3. **Telemetry middleware never swallows tool errors.** Record the error on the span (`span.RecordError(err); span.SetStatus(codes.Error, ...)`), then return it unchanged. Existing error behavior at `mcp/middleware.go:95-100` is preserved byte-for-byte; only the log line grows attributes.
4. **Admin listener failure is non-fatal.** If `/metrics` listener fails to bind, log ERROR and continue. `g.Go` would otherwise cancel the errgroup and take down the MCP daemon — unacceptable. Wrap the admin goroutine so its return value is always nil:

```go
g.Go(func() error {
    if err := d.listenAdmin(gctx); err != nil && !errors.Is(err, context.Canceled) {
        d.logger.Error("admin listener failed, continuing without metrics", "error", err)
    }
    return nil
})
```

This is a deliberate deviation from the kernel/socket/HTTP listeners which MUST fail the group. Rationale: observability is instrumentation, not product.

---

## 5. Graceful Degradation Integration

### 5.1 Timeout budgets

New `internal/degrade` package defines per-tool-class budgets:

| Class | Default | Rationale |
|-------|---------|-----------|
| read (hover, definition, symbol overview) | 5s | LSP reads are usually <500ms; 5s catches pathological gopls indexing |
| search (find_symbol, search_for_pattern) | 15s | Ripgrep-style fan-out |
| edit (all 6 edit tools) | 10s | Includes tree-sitter + LS validate roundtrip |
| index (activate_project) | 120s | Cold jdtls/rust-analyzer are slow; must exceed LS bootstrap |
| diagnostics | 20s | Includes publishDiagnostics settle time |

`degrade.EnsureBudget(ctx, class)` returns a derived context with the budget applied if none is already set. Called at tool handler entry. If caller already set a deadline, respect the shorter.

**Integration point:** kernel tool `RegisterTools` functions (in `internal/kernel/symbols`, `edit`, `fileops`, `diag`) wrap each handler. Alternative considered: put it in the MCP middleware. Rejected because middleware doesn't know the operation class without a classifier table — and a classifier table in middleware reintroduces coupling the kernel tool packages were designed to avoid. Doing it at tool registration keeps class info colocated with the tool definition. MEDIUM confidence on placement — this is a judgment call, will revisit after implementation.

### 5.2 Circuit breaker tuning

Existing `lspool/circuit.go` has the breaker. v1.2 additions:

1. Expose state via metric: `serena_lspool_circuit_state{lang,reason}` gauge (0=closed, 1=half-open, 2=open).
2. Config-driven thresholds in `cfg.WorkerPool.Circuit` (new struct): `FailureThreshold`, `CooldownDuration`, `HalfOpenProbes`.
3. New counter: `serena_lspool_circuit_trips_total{lang,reason}`.
4. Breaker-open errors become a typed error `lspool.ErrCircuitOpen` so the telemetry middleware can tag spans with `breaker_open=true` without string matching. Feeds into the deferred typed-errors TODO(#typed-errors) noted in v1.1.

No behavioral change in the breaker algorithm itself — v1.2 is observability + tuning.

### 5.3 OOM / crash recovery

Pool already does pressure eviction (`pressure_{linux,darwin}.go`). New work:

1. **Crash detection metric:** `serena_lspool_worker_crashes_total{lang,signal}` — incremented in the existing worker supervision goroutine when the LS process exits abnormally.
2. **Eviction reason labels:** `serena_lspool_evictions_total{reason=pressure|ttl|circuit|shutdown}`. The pool's existing eviction paths each get a call-site constant. No logic change; just labeling.
3. **RSS gauge:** per-worker RSS sampled on the existing pressure check tick. Emitted as `serena_lspool_worker_rss_bytes{lang,worker_id}`.
4. **Daemon-level watchdog:** a `runtime.ReadMemStats`-fed gauge exposed alongside Go runtime collector (`prometheus/client_golang/prometheus/collectors`).

No new goroutines — piggyback on existing pressure tick to avoid adding scheduler noise.

### 5.4 LS crash recovery

Already handled by worker supervision; v1.2 adds:

- Structured log event `lspool.worker_crashed` with `signal`, `exit_code`, `stderr_tail` (last 4KB).
- Automatic restart still bounded by circuit breaker — if the breaker opens, no restart storm.
- New `serena_lspool_restarts_total` counter.

---

## 6. Benchmarks: access pattern decision

**Question:** where do benchmarks live and how do they see internal APIs?

**Options:**

| Option | Location | Package | Access |
|--------|----------|---------|--------|
| A. Co-located `_test.go` under `internal/...` | `internal/kernel/lspool/pool_bench_test.go` | `package lspool` | Full internal access |
| B. Top-level `test/bench/` | `test/bench/` | `package bench_test` | Public API only |
| C. `benchmark/` top-level with internal imports | `benchmark/` | various | Allowed to import `internal/*` because it's same module |
| D. Hybrid | A + B | — | Micro-benches internal; macro-benches black-box |

**Chosen: D (hybrid), weighted toward B.**

Rationale:

- **Integration precedent** (v1.1) put macro-scale tests in top-level `test/integration/` as `package *_test` to force clean public-API exports. Benchmarks that measure "tool response times p50/p95/p99" and "indexing throughput" are exactly the same shape: they must exercise the full daemon, which is the v1.2 benchmarking target from PROJECT.md. These live in `test/bench/` and reuse the existing harness (`test/integration/harness.go`).
- **Micro-benchmarks** for hot-path primitives (JSON-RPC codec decode, tree-sitter body extraction, FTS5 query) benefit from internal access and need to be in the package under test. These live as `*_bench_test.go` files alongside the code. Go's `go test -bench` convention handles this natively.
- **Memory profiles** (PROJECT.md line 51) are macro: they need a full workspace activation. `test/bench/memory_bench_test.go` using `testing.B.ReportAllocs()` + manual `runtime.ReadMemStats` snapshots around phases.
- **CI regression gate:** CI runs `go test -bench=. -benchmem -run=^$ ./test/bench/... ./internal/...` and pipes through `benchstat` against a baseline committed at `test/bench/baselines/`. A failing comparison fails CI.

**Why not C (`benchmark/` importing `internal/*`):** same-module internal imports are legal but defeat the point of `internal`. The v1.1 decision explicitly moved integration tests to top-level to force public-API hygiene. Benchmarks are a downstream consumer of the same API contract — same rule applies. HIGH confidence — consistent with existing repo conventions.

**New directory layout:**

```
test/bench/
|-- harness.go                # Re-exports or thin wrapper over test/integration harness
|-- tools_bench_test.go       # p50/p95/p99 per tool, all 38 tools
|-- indexing_bench_test.go    # LOC/sec against testdata/python/go/ts/rust/java fixtures
|-- memory_bench_test.go      # baseline / per-workspace / per-worker RSS snapshots
|-- baselines/
|   |-- tools.txt             # benchstat input — committed
|   `-- indexing.txt
`-- fixtures/                 # link to test/integration/testdata where possible
```

Micro-benchmarks stay alongside source:
- `internal/kernel/jsonrpc/codec_bench_test.go`
- `internal/kernel/edit/body_extract_bench_test.go`
- `internal/memory/fts_bench_test.go`
- `internal/kernel/lspool/pool_bench_test.go`

---

## 7. Data Flow: a traced `find_references` call

```
claude-code
   `-> stdio -> forwarder
         |       `- new OTel span "forwarder.stream"
         |          traceparent -> gRPC metadata
         `-> gRPC (unix socket)
                `-> daemon.grpcServer (otelgrpc StatsHandler extracts traceparent)
                      `-> MCP SDK stream handler (ctx carries span)
                            `-> telemetry middleware
                                  | span "mcp.tools/call" attrs={tool="find_references"}
                                  | hist serena_mcp_request_duration_seconds{method,tool}
                                  | counter serena_mcp_requests_total{method,tool,status}
                                  `-> profile filter middleware (passthrough for tools/call)
                                        `-> symbols.findReferencesHandler
                                              | degrade.EnsureBudget(ctx, "read") -> ctx w/ 5s deadline
                                              | span "symbols.find_references" attrs={lang,sym_count_est}
                                              `-> kernel.Pool.AcquireLease
                                                    | hist serena_lspool_lease_wait_seconds
                                                    | hist serena_lspool_lease_hold_seconds
                                                    | counter serena_lspool_lease_acquired_total{lang,reused}
                                                    `-> LSWorker.Request (textDocument/references)
                                                          span event "ls.request" attrs={method,duration_ms}
                                                          (no span — LSP doesn't speak OTel)
```

Every log line emitted during this path automatically carries `trace_id`/`span_id` via the context slog handler. Operators can pivot from a slow-request log in Loki/ES to the full span in Jaeger/Tempo with zero copy-paste.

---

## 8. Integration Points Summary

| Component | Change type | File(s) |
|-----------|------------|---------|
| `internal/obs/` | NEW package | all files new |
| `internal/degrade/` | NEW package | all files new |
| `internal/daemon/daemon.go` | MODIFY | construct obs provider, install trace-aware slog, add admin listener goroutine, pass provider to MCP + kernel |
| `internal/daemon/telemetry.go` | NEW | admin listener, health, pprof gating, runtime collector registration |
| `internal/mcp/middleware.go` | MODIFY | telemetryMiddleware replaces loggingMiddleware (same install call, new param) |
| `internal/mcp/server.go` | MODIFY (minimal) | accept optional provider in `NewSerenaMCPServer` — pass-through to middleware install |
| `internal/forwarder/dial.go` | MODIFY | add `otelgrpc` client StatsHandler |
| `internal/kernel/lspool/pool.go` | MODIFY | accept `*obs.Meter`, instrument lease acquire / breaker / eviction |
| `internal/kernel/lspool/circuit.go` | MODIFY | state gauge emission, new typed error `ErrCircuitOpen` |
| `internal/kernel/lspool/metrics.go` | NEW | metric definitions for pool |
| `internal/kernel/symbols,edit,fileops,diag` | MODIFY (mechanical) | wrap handlers with `degrade.EnsureBudget` + span start |
| `internal/config` | MODIFY | add `ObservabilityConfig`, `WorkerPool.Circuit`, `WorkerPool.Budgets` |
| `test/bench/` | NEW | macro benchmarks + baselines |
| `internal/**/*_bench_test.go` | NEW (selective) | hot-path micro-benchmarks |
| `legacy/` | UNTOUCHED | — |

Files explicitly NOT changed:

- `internal/skill/*` — skills receive a ctx and logger like today. If a skill wants a span it can ask the logger's handler (via obs helper), but we do not force an API break on the skill interface. The session handoff and onboarding skills have no hot-path concerns.
- `protocol/gen/*` — generated LSP code stays untouched.
- `api/proto/serena/v1/*` — proto schema unchanged. Traceparent rides gRPC metadata, not our proto.

---

## 9. Suggested Build Order

Phases are sized so each delivers standalone value and is independently testable. Dependencies are strict; later phases assume earlier phases are in.

**Phase A — Observability foundation** *(prereq for everything else)*
1. Create `internal/obs/` package with provider, noop defaults, slog context handler, in-memory test exporter.
2. Wire `obs.Provider` construction into `daemon.New`; wrap existing slog handler.
3. Add `cfg.Observability` koanf schema with safe defaults (admin addr loopback, metrics off unless addr set, tracing sampler=never).
4. Unit tests for the context slog handler (trace ID injection) and noop provider.

**Phase B — Metrics + admin listener**
1. Add `internal/daemon/telemetry.go` with admin listener, `/healthz`, `/readyz`, `/metrics`, gated pprof.
2. Add listener to `daemon.Run` errgroup with non-fatal error wrapper.
3. Register Go runtime collectors + process collector.
4. Instrument MCP middleware: request counter + duration histogram.
5. Instrument `lspool`: lease wait/hold, worker count, restarts.
6. Integration test: hit `/metrics`, assert expected metric names present.

**Phase C — Tracing end-to-end**
1. Add `otelgrpc` StatsHandlers to forwarder client + daemon gRPC server.
2. Replace logging middleware with telemetry middleware (spans + metrics + log).
3. Add kernel-level sub-spans in symbol/edit handlers (one per tool package, mechanical).
4. Add span events at LS request boundary in `lspool/worker.go`.
5. Integration test with in-memory exporter asserting trace continuity across forwarder->daemon->kernel.

**Phase D — Graceful degradation**
1. Create `internal/degrade/` with budget table and `EnsureBudget`.
2. Apply at all 38 tool handler entries.
3. Add `cfg.WorkerPool.Circuit` tuning knobs, wire into existing breaker.
4. Add `lspool.ErrCircuitOpen` typed error; update middleware to tag spans.
5. Add crash/eviction reason labels on existing metrics.
6. Chaos test: kill LS workers, assert breaker opens, metrics update, no tool calls hang past budget.

**Phase E — Benchmarks + CI gate**
1. Create `test/bench/` harness reusing integration harness.
2. Tool response time benchmarks (all 38 tools).
3. Indexing throughput benchmarks (per-language fixtures).
4. Memory profile benchmarks.
5. Micro-benchmarks in internal packages for JSON-RPC codec, tree-sitter extract, FTS5 query.
6. Commit baselines to `test/bench/baselines/`.
7. CI job: `benchstat` comparison with configurable regression threshold (suggest 10% for time, 20% for allocs).

**Phase F — Documentation** *(can run in parallel with E)*
1. README.md: capabilities, install, quick start, link to USAGE.
2. USAGE.md: client setup (Claude Code, Codex, IDE), profiles, workflows, troubleshooting, metrics reference, trace interpretation guide.

**Phase ordering constraints:**
- A before B, C, D (everyone imports obs).
- B before C (metrics listener must exist before we produce interesting span data to visualize alongside).
- C before D only weakly — D can proceed on B, but typed errors in D are nicer to wire once spans exist to tag.
- E depends on B at minimum (benchmarks want to capture Go runtime metrics to correlate with p99 spikes).
- F depends on all — operator docs can't describe metrics/traces/degradation that don't exist.

**Parallelization opportunities:**
- Phase D and Phase E can proceed in parallel after C lands.
- Phase F can start drafting during any phase, final sections filled as features land.

---

## 10. Open Questions / Flags for Implementation

1. **OTel contrib version pinning.** `otelgrpc` tracks both OTel core and gRPC versions; pin explicitly and verify via Context7 when implementing Phase C. MEDIUM confidence on exact import path.
2. **Prom client cardinality.** `tool` label has 38 values, `lang` has up to 52. 38x52 = ~2K series per histogram — acceptable but watch for multiplication when we add `status`. Decision rule: any label that multiplies existing cardinality >5x needs a design review before merging.
3. **Sampler default.** Recommend `ParentBased(TraceIDRatioBased(0.0))` — respects parent decision, samples nothing on its own. Operators enable via config. Avoids log-spam from v1.1 integration test suite turning into trace spam.
4. **Admin listener auth.** v1.2 relies on loopback binding. If operators want remote scraping, they expose via reverse proxy. Do NOT add a built-in auth layer in v1.2 — that's a v1.3+ concern and every minute spent on it is a minute not spent on actual observability coverage.
5. **Typed errors scope.** v1.1 deferred typed errors (TODO(#typed-errors)). v1.2 introduces exactly one: `lspool.ErrCircuitOpen`. We do NOT attempt a full typed-error sweep; that remains a separate milestone. Rationale: scope discipline.

---

## Sources

- Existing code: `internal/daemon/daemon.go`, `internal/mcp/middleware.go`, `internal/kernel/lspool/`, `internal/forwarder/dial.go` (HIGH — direct read).
- Go stdlib `log/slog` Handler extension pattern (HIGH — stdlib docs).
- `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` StatsHandler API (MEDIUM — verify exact import path at implementation time via Context7).
- `prometheus/client_golang` best practices for pre-registered labeled vectors (HIGH — library docs).
- Go service convention for loopback-default admin endpoints: gopls, etcd, CockroachDB (HIGH — cross-project pattern).
- v1.1 repo convention: top-level `test/` package for black-box public-API testing (HIGH — PROJECT.md key decisions table).
