# Phase 12: Tracing End-to-End - Research

**Researched:** 2026-04-08
**Domain:** OpenTelemetry Go tracing wiring (forwarder ↔ gRPC ↔ daemon ↔ kernel) with degraded-optional OTLP exporter
**Confidence:** HIGH

## Summary

Phase 12 delivers distributed tracing from the forwarder through the daemon gRPC boundary, through the MCP `TelemetryMiddleware` (Phase 11), into per-tool kernel handlers, with LS interactions recorded as span **events** (not child spans). Tracing is **off by default** via `ParentBased(TraceIDRatioBased(0.0))`, the OTLP/gRPC exporter is degraded-optional (bind/construction failures fall back to noop), and the existing Phase 10 `ContextHandler` automatically starts injecting `trace_id`/`span_id` into slog records as soon as the stub extractor is swapped for `trace.SpanContextFromContext`.

All OTel wiring stays confined to `internal/obs/` so the dependency blast radius matches the Phase 11 precedent for `prometheus/client_golang`. Provider injection is explicit — **no `otel.SetTracerProvider` global** — matching the Phase 10/11 pattern and the CONTEXT.md D-01 decision. Three-span tree (`forwarder.tools.call` → `daemon.mcp.tools.call` → `kernel.tool.{name}`) gives debuggers enough context without doubling span count per call.

**Primary recommendation:** Pin OTel at the latest stable (**v1.43.0**, released 2026-04-03 — not v1.38 as CONTEXT.md assumed), use `otelgrpc.NewServerHandler` / `NewClientHandler` with explicit `WithTracerProvider(provider.TracerProvider())`, plumb the tracer through a new `obs.Provider.Tracer()` accessor, extend `TelemetryMiddleware` to wrap `next(ctx, ...)` in `tracer.Start(ctx, "daemon.mcp.tools.call")`, and add per-tool-package span creation via a small `kernel` helper. Exporter lifecycle mirrors `internal/daemon/telemetry.go` (admin listener) with a **dedicated 5s shutdown context** per PITFALLS #4.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**OTel TracerProvider Wiring**
- **D-01:** Explicit dependency injection via `obs.Provider.Tracer() trace.Tracer`. NO global `otel.SetTracerProvider()`.
- **D-02:** Rationale: test-safe (multiple daemon instances in the same process each get their own provider), clean dependency graph, matches Phase 10/11 pattern.
- **D-03:** `otelgrpc.NewServerHandler` / `NewClientHandler` and `otelslog.NewHandler` are instantiated with explicit `trace.TracerProvider` argument (not auto-discovery from global).
- **D-04:** Phase 10 `obs.Provider` struct extends with `Tracer()` method. Noop provider returns a noop tracer (so callers always get a non-nil tracer).

**Span Granularity**
- **D-05:** Three-span tree per tool call:
  1. `forwarder.tools.call` (forwarder entry)
  2. `daemon.mcp.tools.call` (daemon telemetry middleware)
  3. `kernel.tool.{tool_name}` (kernel tool execution)
- **D-06:** LSP interactions emit **span events** (not child spans) on the kernel span: `name: ls.request`, `attributes: {lsp.method, lsp.language, lsp.duration_ms}`
- **D-07:** Standard span attributes per D-05/D-06: `tool_name`, `profile`, `mode`, `language` (matches metrics label contract).
- **D-08:** Rationale: conservative, low overhead, minimal cardinality.

**OTLP Exporter Lifecycle**
- **D-09:** Daemon-managed lifecycle (degraded-optional pattern from Phase 10 admin listener); construction errors log warning and fall back to noop; shutdown uses a dedicated 5s shutdown context (not the cancelled errgroup context — per PITFALLS #4 flush race).
- **D-10:** Default sampler: `ParentBased(TraceIDRatioBased(0.0))` — tracing is zero-cost unless explicitly turned on via config.
- **D-11:** Config fields added: `cfg.Observability.TracingEndpoint string`, `cfg.Observability.TracingSampleRatio float64` (default 0.0), `cfg.Observability.ServiceName string` (default "serena").

**Context Propagation**
- **D-12:** `otelgrpc.NewServerHandler` on daemon gRPC server (IPC), `otelgrpc.NewClientHandler` on forwarder gRPC client. Traceparent flows via gRPC metadata automatically.
- **D-13:** `TelemetryMiddleware` creates the `daemon.mcp.tools.call` span, extracting parent from ctx.
- **D-14:** Kernel tools add sub-spans inside their handlers via `tracer.Start(ctx, "kernel.tool.foo")`. Minimal API: plumb tracer through kernel constructor.

**Phase 10 SpanContext Stub Replacement**
- **D-15:** `internal/obs/spancontext.go` — `spanContextFromContext(ctx)` now extracts from `trace.SpanContextFromContext(ctx)` instead of returning false.
- **D-16:** Phase 10 `ContextHandler` unchanged — zero-change wire-up.

**Hot-Path Budget**
- **D-17:** Tracing-off baseline: ≤ +2 allocs/op vs Phase 11 baseline (`v1.2-phase11-github-hosted.txt`).
- **D-18:** Tracing-on (1% sampling) is NOT gated against the budget — benchmark captures the number for Phase 14 docs.

### Claude's Discretion

- Exact package layout (`internal/obs/tracing.go` vs extending `obs.go`)
- Exporter initialization order (before or after kernel start)
- Span attribute naming (OTel semconv vs Serena-native)
- How to plumb the tracer into kernel — constructor parameter vs obs.Provider accessor vs context-stashed

### Deferred Ideas (OUT OF SCOPE)

- Per-LS-request child spans (v1.3 behind `--trace-ls`)
- Histogram exemplars linking trace IDs (needs Prometheus 2.40+ scrape side)
- Smart sampling based on tool_name or outcome (v1.3+)
- Tracing over HTTP transport (v1.2 only instruments gRPC)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| TRACE-01 | `otelgrpc` StatsHandlers on forwarder↔daemon gRPC | "Context Propagation" + "Integration Points" sections; verified API signatures for `NewServerHandler` / `NewClientHandler`; install points in `forwarder/dial.go` and `daemon.listenSocket` |
| TRACE-02 | Telemetry middleware replacing logging middleware; runs before profile filter | Already landed in Phase 11 (`internal/mcp/middleware.go` → `TelemetryMiddleware`); Phase 12 extends it with `tracer.Start(ctx, "daemon.mcp.tools.call")` |
| TRACE-03 | Per-tool sub-spans for kernel operations | "Architecture Patterns" → kernel helper + span attribute contract |
| TRACE-04 | Optional OTLP exporter behind config flag | "OTLP Exporter Lifecycle" + code example for degraded-optional construction |
| TRACE-05 | Default sampler `ParentBased(TraceIDRatioBased(0.0))` — off by default | Verified `sdktrace.ParentBased` + `sdktrace.TraceIDRatioBased` API; tracer-off budget gate |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Language:** Go only. Every Phase 12 artifact is Go code. Python (`legacy/`) is untouched.
- **Build gates:** `go vet ./...` and `go test ./...` before completing any task. `gofmt -w .` required.
- **MCP:** primary interface — Phase 12 instruments the MCP request path via middleware + kernel tool wrappers.
- **No JetBrains / proprietary LSP backends:** irrelevant to tracing but confirms LS span events are the correct abstraction (JSON-RPC, not OTel-aware).
- **GSD workflow:** all file edits go through a GSD command.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `go.opentelemetry.io/otel` | **v1.43.0** | Tracing API (`trace.Tracer`, `trace.SpanContextFromContext`, `noop.NewTracerProvider`) | `[VERIFIED: pkg.go.dev release history]` Stable v1, industry standard. Note: **CONTEXT.md assumed v1.38.x; actual current is v1.43.0 (2026-04-03)** `[CITED: github.com/open-telemetry/opentelemetry-go/releases]` |
| `go.opentelemetry.io/otel/sdk` | v1.43.0 | `sdktrace.TracerProvider`, `sdktrace.WithBatcher`, `sdktrace.WithSampler`, `sdktrace.WithResource`, samplers (`ParentBased`, `TraceIDRatioBased`) | `[VERIFIED: pkg.go.dev/go.opentelemetry.io/otel/sdk/trace]` SDK pairs with API same version. |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace` | v1.43.0 | Transport-agnostic OTLP trace exporter wrapper | `[CITED: pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace]` |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | v1.43.0 | OTLP/gRPC trace exporter (default endpoint `localhost:4317`) | `[CITED: pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc]` |
| `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` | **v0.68.0** | Auto-propagates W3C traceparent through gRPC metadata via `stats.Handler` | `[VERIFIED: pkg.go.dev; 2026-04-07]` contrib package tracks OTel core (0.x = 1.x-1) |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go.opentelemetry.io/otel/sdk/resource` | v1.43.0 | Resource attributes (`service.name`) | Construct once in `obs.newTracing(cfg)`; attached via `sdktrace.WithResource` |
| `go.opentelemetry.io/otel/semconv/v1.26.0` | pinned | Semantic convention keys (`service.name`, `rpc.service`) | Only for the resource — spans use Serena-native attributes per the D-07 label contract (matches metrics allowlist) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `otelgrpc` StatsHandler | `otelgrpc.UnaryClientInterceptor` / `StreamClientInterceptor` | **Deprecated** `[CITED: pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc]`. StatsHandler captures connection-level events interceptors miss. Not an option. |
| `otelslog` bridge | Keep current `obs.ContextHandler` | STACK.md originally recommended `otelslog`, but Phase 10 already shipped a hand-rolled `ContextHandler` that injects `trace_id`/`span_id` via the stub extractor. Switching to `otelslog` would delete working alloc-budgeted code for no functional gain. **Do NOT introduce `otelslog` in Phase 12.** |
| OTLP/HTTP exporter | `otlptracehttp` | CONTEXT.md locked gRPC; HTTP is a v1.3+ option. |
| Global `otel.SetTracerProvider` | Explicit injection | D-01 locks explicit injection. Global provider breaks parallel tests and couples subsystems to import order. |

**Installation:**
```bash
go get go.opentelemetry.io/otel@v1.43.0
go get go.opentelemetry.io/otel/sdk@v1.43.0
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.43.0
go get go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc@v0.68.0
```

**Version verification:** Planner must confirm at Wave 0 via `go list -m -versions go.opentelemetry.io/otel` that v1.43.0 is still current at merge time; pin exact versions in go.mod (no floating pseudo-versions).

## Architecture Patterns

### Recommended Package Layout

```
internal/obs/
├── obs.go           # Provider extended with TracerProvider + Tracer() accessor
├── tracing.go       # NEW — exporter construction, sampler wiring, Shutdown(ctx)
├── spancontext.go   # spanContextFromContext() replaced with trace.SpanContextFromContext
├── handler.go       # UNCHANGED — ContextHandler starts injecting once extractor returns true
├── metrics.go       # UNCHANGED
└── tracing_test.go  # NEW — in-memory span recorder tests
```

**Rationale for separate `tracing.go`:** `obs.go` is already a thin facade. All OTel imports (core, sdk, otlptracegrpc) live in `tracing.go`, keeping `obs.go` free of third-party clutter and matching the Phase 11 `metrics.go` split.

### Pattern 1: Provider Construction

```go
// Source: go.opentelemetry.io/otel/sdk/trace docs, v1.43.0
// Called from daemon.New() AFTER logger + BEFORE kernel creation.

// noop path (default, tracingEndpoint == "")
func Noop(inner slog.Handler) *Provider {
    return &Provider{
        slogHandler:    NewContextHandler(inner),
        metrics:        newMetrics(),
        tracerProvider: tracenoop.NewTracerProvider(), // noop.NewTracerProvider
    }
}

// degraded-optional construction (tracingEndpoint != "")
func WithTracing(inner slog.Handler, cfg TracingConfig, logger *slog.Logger) *Provider {
    p := Noop(inner) // start from noop — on any error we return this unchanged

    exporter, err := otlptracegrpc.New(context.Background(),
        otlptracegrpc.WithEndpoint(cfg.Endpoint),
        otlptracegrpc.WithInsecure(), // cfg.Insecure — TLS is v1.3
    )
    if err != nil {
        logger.Warn("otlp trace exporter construction failed, tracing disabled", "error", err)
        return p // noop tracer — daemon keeps starting
    }

    res, _ := resource.Merge(resource.Default(), resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceName(cfg.ServiceName),
    ))

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
        sdktrace.WithResource(res),
    )
    p.tracerProvider = tp
    return p
}
```

### Pattern 2: Provider Accessors (D-04)

```go
func (p *Provider) TracerProvider() trace.TracerProvider { return p.tracerProvider }
func (p *Provider) Tracer() trace.Tracer {
    return p.tracerProvider.Tracer("github.com/postfix/serena",
        trace.WithInstrumentationVersion("v1.2"))
}
```

Single named tracer per instrumentation scope keeps span aggregation coherent across subsystems. Kernel and middleware share the same instance.

### Pattern 3: gRPC StatsHandlers (D-12, TRACE-01)

```go
// daemon side — internal/daemon/daemon.go listenSocket
d.grpcServer = grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler(
        otelgrpc.WithTracerProvider(d.obs.TracerProvider()),
    )),
)

// forwarder side — internal/forwarder/dial.go tryConnect
conn, err := grpc.NewClient("unix://"+socketPath,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithStatsHandler(otelgrpc.NewClientHandler(
        otelgrpc.WithTracerProvider(fwdProvider.TracerProvider()),
    )),
    grpc.WithKeepaliveParams(...),
)
```

**CRITICAL:** `WithTracerProvider` is non-optional for Serena. Omitting it falls back to the OTel global provider, which D-01 forbids.

**Forwarder provider:** the forwarder is a separate process from the daemon and needs its own `obs.Provider`. Phase 10 landed the daemon-side provider; Phase 12 must also construct a forwarder-side provider (noop by default, with its own config resolution). Without it the `forwarder.tools.call` span cannot exist.

### Pattern 4: Telemetry Middleware Span (D-13, TRACE-02)

```go
// Source: go.opentelemetry.io/otel/trace v1.43.0 + existing mcp/middleware.go
func TelemetryMiddleware(provider *obs.Provider, getSession func(ctx context.Context) *SessionInfo, logger *slog.Logger) mcpsdk.Middleware {
    m := provider.Metrics()
    tracer := provider.Tracer() // noop when tracing is off
    return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
        return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
            // Only spanify tools/call to match metric-emission scope.
            if method != "tools/call" {
                return next(ctx, method, req) // ... existing log path unchanged
            }
            ctx, span := tracer.Start(ctx, "daemon.mcp.tools.call")
            defer span.End()

            start := time.Now()
            result, err := next(ctx, method, req)
            duration := time.Since(start)

            // Attributes pulled from session snapshot (same as metrics labels).
            toolName := extractToolName(req)
            outcome := classifyOutcome(result, err)
            var profile, mode, language string
            if sess := getSession(ctx); sess != nil {
                snap := sess.Snapshot()
                profile, mode, language = snap.Profile, snap.Mode, snap.Language
            }
            span.SetAttributes(
                attribute.String("tool_name", toolName),
                attribute.String("profile", profile),
                attribute.String("mode", mode),
                attribute.String("language", language),
                attribute.String("outcome", outcome),
            )
            if err != nil {
                span.RecordError(err)
                span.SetStatus(codes.Error, err.Error())
            }

            // ...existing metrics + logging emission preserved byte-for-byte...
            _ = duration
            return result, err
        }
    }
}
```

**Invariant:** `classifyOutcome` and metrics emission must remain byte-for-byte identical — Phase 11 tests pin the exact labels.

### Pattern 5: Kernel Tool Sub-Spans (D-14, TRACE-03)

**Minimum-churn approach:** add a `tracer trace.Tracer` field to kernel `Kernel` struct, plumbed through `NewKernel(..., tracer trace.Tracer)`. Kernel tool handlers call:

```go
// Source: internal/kernel/symbols/tools.go, new wrapper
mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
    Name: "find_references",
    ...
}, func(ctx context.Context, req *mcpsdk.CallToolRequest, args FindReferencesArgs) (*mcpsdk.CallToolResult, any, error) {
    ctx, span := k.Tracer().Start(ctx, "kernel.tool.find_references")
    defer span.End()
    // ... existing body ...
})
```

**Discretion call:** CONTEXT.md leaves plumbing method to discretion. Recommended: **kernel constructor parameter** (over context-stashed or accessor on pool). Rationale: tests construct the kernel directly and can pass a `tracenoop.NewTracerProvider().Tracer("test")`; no global state; no runtime `ctx.Value` cost.

**Helper to avoid 38-call repetition:** small `kernel.SpanToolHandler(tracer, name, fn)` wrapper. Not mandatory but reduces churn.

### Pattern 6: LS Span Events (D-06)

At the JSON-RPC request boundary inside `lspool.Worker.Request`, if a span is active on ctx:

```go
span := trace.SpanFromContext(ctx) // returns noop if not active — no nil check needed
if span.IsRecording() {
    span.AddEvent("ls.request", trace.WithAttributes(
        attribute.String("lsp.method", method),
        attribute.String("lsp.language", workerLang),
        attribute.Int64("lsp.duration_ms", duration.Milliseconds()),
    ))
}
```

`IsRecording()` gate avoids attribute-allocation cost in the tracing-off path, critical for D-17 budget.

### Pattern 7: Shutdown (PITFALLS #4)

```go
// internal/daemon/shutdown.go addition
// Phase 1: kernel shutdown (existing, unchanged).
// Phase 1.5 (NEW): flush tracing with a dedicated context — NOT the cancelled errgroup ctx.
if d.obs != nil {
    flushCtx, flushCancel := context.WithTimeout(context.Background(), 5*time.Second)
    if err := d.obs.ShutdownTracing(flushCtx); err != nil {
        d.logger.Warn("trace exporter shutdown error", "error", err)
    }
    flushCancel()
}
// Phase 2: close listeners (existing).
```

`obs.Provider.ShutdownTracing(ctx)` type-asserts `p.tracerProvider` to `*sdktrace.TracerProvider` and calls `Shutdown`. Noop path is a no-op.

### Anti-Patterns to Avoid

- **`otel.SetTracerProvider(tp)`** — breaks D-01; poisons parallel tests.
- **`otel.GetTracerProvider()`** in middleware — same reason; always capture `provider.Tracer()` in the closure.
- **Starting spans in hot helpers (`pathToURI`, `acquireLease`)** — PITFALLS #7 overhead. Spans belong at tool-dispatch granularity only.
- **Passing the cancelled `gctx` to `tracerProvider.Shutdown`** — PITFALLS #4. Use fresh `context.Background()` + 5s timeout.
- **`ctx.Value` stash for tracer** — works but adds per-call lookup cost. Constructor injection is free.
- **Custom sampler with I/O** — `Sampler.ShouldSample` is synchronous per span. `ParentBased(TraceIDRatioBased(x))` is built for this.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| gRPC trace context propagation | Custom metadata injection | `otelgrpc.NewServerHandler` + `NewClientHandler` | StatsHandler captures connection-level events interceptors miss; handles W3C `traceparent` + `tracestate` correctly |
| Batch span processing | Goroutine + channel + ticker | `sdktrace.WithBatcher(exporter)` | Backpressure, shutdown flush, retry all handled |
| Sampling logic | Per-span random check | `sdktrace.ParentBased(sdktrace.TraceIDRatioBased(r))` | Respects parent decision; hash-based consistency across services |
| Resource attribute schema | String keys | `semconv.ServiceName(...)` | W3C/OTel convention; tooling relies on exact keys |
| Noop tracer for tests | `if tracer != nil` branches | `tracenoop.NewTracerProvider()` | Real `trace.Tracer` interface; zero-cost; no nil checks at call sites |
| Trace ID extraction from ctx | Custom context key | `trace.SpanContextFromContext(ctx)` | Direct replacement for Phase 10 stub (D-15) |

**Key insight:** every piece of this phase is boilerplate wiring. The only Serena-specific code is the 3-span hierarchy contract and the attribute set (matching metrics labels). Everything else comes from OTel SDK.

## Runtime State Inventory

Phase 12 is a greenfield feature addition — no rename/refactor. Section intentionally omitted.

## Common Pitfalls

### Pitfall 1: Flush Race on SIGTERM (PITFALLS #4)
**What goes wrong:** `tracerProvider.Shutdown(gctx)` is called with the already-cancelled errgroup context; the batch span processor sees `ctx.Err() != nil` and drops in-flight batches. The final minute of traces — including shutdown-time spans — vanishes.
**Why it happens:** `daemon.Run` cancels `gctx` on signal; `shutdown()` naively reuses it.
**How to avoid:** dedicated `context.WithTimeout(context.Background(), 5*time.Second)` in `shutdown()` before calling `ShutdownTracing`. See Pattern 7.
**Warning signs:** integration test sends SIGTERM mid-request; collector receives zero spans.

### Pitfall 2: Tracing Allocations in the Off Path (D-17)
**What goes wrong:** Adding `tracer.Start(ctx, "...")` unconditionally allocates a span struct even when the sampler is 0.0, blowing the D-17 ≤ +2 allocs/op budget.
**Why it happens:** Noop `trace.Tracer` **still returns a real (but trivial) span**. SDK tracer with zero-sample sampler allocates attribute slices eagerly.
**How to avoid:**
- Default provider is **noop tracer** (not SDK tracer with 0% sampler) when `cfg.TracingEndpoint == ""` — `tracenoop.NewTracerProvider()`, not `sdktrace.NewTracerProvider`.
- When tracing IS configured, trust `ParentBased(TraceIDRatioBased(0.0))` to short-circuit — but this path is outside the D-17 budget.
- Inside kernel tool handlers gate expensive attribute-building behind `span.IsRecording()`.

**Warning signs:** benchmark `allocs/op` for `BenchmarkTelemetryMiddleware_ToolsCall` rises >2 vs Phase 11 baseline even with `TracingEndpoint=""`.

### Pitfall 3: Forwarder Has No Provider (D-05 #1)
**What goes wrong:** `forwarder.tools.call` is the root span in the 3-span tree. Without a provider in the forwarder process, `otelgrpc.NewClientHandler()` falls back to the OTel global (which D-01 forbids being set) and produces noop spans — the downstream daemon span becomes the root, breaking the trace tree.
**Why it happens:** Phase 10 landed the daemon provider only; the forwarder is a separate process.
**How to avoid:** extend forwarder `main.go` / bootstrap to construct its own `obs.Provider` (noop by default). Explicitly pass via `otelgrpc.WithTracerProvider`.
**Warning signs:** Jaeger shows 2-level trace (daemon + kernel) instead of 3.

### Pitfall 4: otelgrpc / OTel Core Version Skew
**What goes wrong:** `otelgrpc v0.68.0` depends on specific `go.opentelemetry.io/otel` minor versions. Pulling `otel@latest` later can break the build.
**Why it happens:** contrib packages track core `v1.x-1` by convention, but the relationship isn't enforced at module graph level.
**How to avoid:** pin all five OTel modules to exact versions in go.mod; run `go vet ./...` + integration test as Wave 0 gate.
**Warning signs:** compile error `type stats.Handler mismatch`.

### Pitfall 5: `stats.Handler` Ordering with Keepalive (from existing forwarder code)
**What goes wrong:** `forwarder/dial.go` already uses `grpc.WithKeepaliveParams` + `grpc.WithTransportCredentials`. Adding `grpc.WithStatsHandler` is order-independent but passing the handler twice (e.g., if future code adds a metrics stats handler) silently overwrites — gRPC keeps only the last one.
**How to avoid:** use `otelgrpc`'s combined handler; if Phase 13+ adds more stats handlers, use `grpc.WithStatsHandler` once with a composite.
**Warning signs:** span traces appear but other telemetry disappears.

### Pitfall 6: Thread-Safety of `sdktrace.TracerProvider.Shutdown`
**What goes wrong:** calling `Shutdown` twice panics or returns already-shutdown error.
**How to avoid:** guard with `sync.Once` in `obs.Provider.ShutdownTracing`, or ensure `shutdown()` is only called from `Run()`'s `defer`.

### Pitfall 7: Span Attribute Cardinality (PITFALLS #2, #7)
**What goes wrong:** adding unbounded attributes (`file.path`, `symbol.name`, `request.id`) to spans — unlike metrics, OTel spans tolerate high cardinality, but the D-07 contract ties span attributes to the metrics allowlist for consistency.
**How to avoid:** only set `tool_name`, `profile`, `mode`, `language`, `outcome` on the middleware span. Tool-level spans add nothing extra beyond the span name.

## Code Examples

### Extending `obs.Provider` (D-04, D-15)

```go
// internal/obs/obs.go (modification)
import (
    "go.opentelemetry.io/otel/trace"
    tracenoop "go.opentelemetry.io/otel/trace/noop"
)

type Provider struct {
    slogHandler    slog.Handler
    metrics        *Metrics
    tracerProvider trace.TracerProvider // NEW
}

func Noop(inner slog.Handler) *Provider {
    return &Provider{
        slogHandler:    NewContextHandler(inner),
        metrics:        newMetrics(),
        tracerProvider: tracenoop.NewTracerProvider(), // NEW
    }
}

func (p *Provider) TracerProvider() trace.TracerProvider { return p.tracerProvider }
func (p *Provider) Tracer() trace.Tracer {
    return p.tracerProvider.Tracer("github.com/postfix/serena")
}
func (p *Provider) ShutdownTracing(ctx context.Context) error {
    sdk, ok := p.tracerProvider.(interface{ Shutdown(context.Context) error })
    if !ok {
        return nil
    }
    return sdk.Shutdown(ctx)
}
```

### Replacing the Stub Extractor (D-15)

```go
// internal/obs/spancontext.go
// Source: go.opentelemetry.io/otel/trace v1.43.0
import "go.opentelemetry.io/otel/trace"

func spanContextFromContext(ctx context.Context) (SpanContext, bool) {
    sc := trace.SpanContextFromContext(ctx)
    if !sc.IsValid() {
        return SpanContext{}, false
    }
    return SpanContext{
        TraceID: sc.TraceID().String(),
        SpanID:  sc.SpanID().String(),
    }, true
}
```

`trace.SpanContext.IsValid()` is the canonical guard — same contract Phase 10 handler relies on.

### Exporter Construction

```go
// internal/obs/tracing.go (new file)
// Source: pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.43.0
func newTracerProvider(ctx context.Context, cfg TracingConfig) (*sdktrace.TracerProvider, error) {
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(cfg.Endpoint),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return nil, fmt.Errorf("otlp exporter: %w", err)
    }
    res, err := resource.Merge(resource.Default(),
        resource.NewWithAttributes(semconv.SchemaURL,
            semconv.ServiceName(cfg.ServiceName),
        ))
    if err != nil {
        return nil, fmt.Errorf("resource: %w", err)
    }
    return sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
        sdktrace.WithResource(res),
    ), nil
}
```

### Ctx-Propagation Audit Targets

All these already accept `ctx context.Context` as the first parameter (verified by grep); Phase 12 adds span creation without signature changes:

1. `internal/mcp/middleware.go` — `TelemetryMiddleware` closure — Pattern 4
2. `internal/kernel/symbols/tools.go` — 9 registered handlers (`go_to_definition`, `find_references`, `get_symbol_overview`, `search_symbols`, `get_hover_info`, `find_implementations`, `get_call_hierarchy`, `get_type_hierarchy`, `analyze_blast_radius`)
3. `internal/kernel/edit/*.go` — 6 edit tools
4. `internal/kernel/fileops/*.go` — 6 file operation tools
5. `internal/kernel/diag/*.go` — 3 diagnostic tools
6. `internal/kernel/lspool/worker.go` — JSON-RPC `Request` (LS span event emission)

Total: ~24 kernel handler wraps + 1 middleware extension + 1 LS-event emission point. All mechanical; a `kernel.WrapWithSpan` helper shrinks this to a single wrapper applied at `RegisterTools` time.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `otelgrpc.UnaryClientInterceptor` / `StreamServerInterceptor` | `NewServerHandler` / `NewClientHandler` (StatsHandler) | contrib v0.43+ (~2023) | Captures connection-level events interceptors miss; simpler install |
| `otel.SetTracerProvider` global | Explicit `WithTracerProvider(tp)` injection | Best practice in otel-go since v1.0 | Test-safe, parallel-safe |
| CONTEXT.md assumed OTel **v1.38.x** | Current stable is **v1.43.0** (2026-04-03) | Last ~6 months | Correction required in task package-pin |
| `otelslog` (STACK.md recommendation) | Stay on hand-rolled `ContextHandler` | Phase 10 already shipped | Do not regress working alloc-budgeted code |

**Deprecated / outdated:**
- Interceptor-based otelgrpc: `[CITED: pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc]` explicitly recommends StatsHandler.
- `go.opentelemetry.io/otel/semconv/v1.4.0` and older: use v1.26.0+ constants.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build | ✓ | 1.25.1 (go.mod) | — |
| `go.opentelemetry.io/otel` modules | runtime (new) | ✗ | — | `go get` at start of Wave 0 |
| `golang.org/x/perf/cmd/benchstat` | benchmark regression gate | ✓ (via `golang.org/x/perf` in go.mod) | indirect | — |
| Running OTLP collector | **integration test only** | ✗ (none needed) | — | Use in-memory span recorder (`tracetest.NewInMemoryExporter`) |

**Missing dependencies with no fallback:** none — all OTel modules are `go get`-able from the Go proxy.

**Missing dependencies with fallback:** OTLP collector for end-to-end trace visual verification. Use `go.opentelemetry.io/otel/sdk/trace/tracetest.NewInMemoryExporter()` in Go tests to assert traceparent continuity without running Jaeger/Tempo.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `stretchr/testify` (already in go.mod) |
| Config file | none (Go convention) |
| Quick run command | `go test ./internal/obs/... ./internal/mcp/... ./internal/kernel/... -run TestPhase12 -count=1` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TRACE-01 | otelgrpc StatsHandler installed on both sides; traceparent propagates | integration | `go test ./test/integration/ -run TestTraceparentPropagation -count=1` | ❌ Wave 0 |
| TRACE-02 | Telemetry middleware creates `daemon.mcp.tools.call` span with correct attrs | unit | `go test ./internal/mcp/ -run TestTelemetryMiddlewareSpan -count=1` | ❌ Wave 0 |
| TRACE-03 | Each kernel tool emits a `kernel.tool.{name}` child span | unit | `go test ./internal/kernel/symbols/ -run TestToolSpans -count=1` | ❌ Wave 0 |
| TRACE-04 | Exporter construction failure degrades to noop (daemon still starts) | unit | `go test ./internal/obs/ -run TestTracingDegradedFallback -count=1` | ❌ Wave 0 |
| TRACE-05 | Default sampler is `ParentBased(TraceIDRatioBased(0.0))` — zero spans recorded without parent context | unit | `go test ./internal/obs/ -run TestDefaultSamplerOff -count=1` | ❌ Wave 0 |
| D-15 regression | `spanContextFromContext` now extracts real trace IDs when span is active on ctx | unit | `go test ./internal/obs/ -run TestContextHandlerWithRealSpan -count=1` | ❌ Wave 0 |
| D-17 budget | Tracing-off path ≤ +2 allocs/op vs Phase 11 baseline | benchmark | `go test ./test/bench/ -bench BenchmarkMiddlewareToolsCall -benchmem -count=10` + `benchstat v1.2-phase11-github-hosted.txt new.txt` | Benchmark exists; new comparison baseline |
| PITFALLS #4 | Shutdown flushes in-flight spans when SIGTERM arrives mid-request | integration | `go test ./test/integration/ -run TestTraceFlushOnShutdown -count=1` | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go vet ./... && go test ./internal/obs/... ./internal/mcp/... -count=1`
- **Per wave merge:** `go test ./... -count=1` (full suite) + `go test ./test/bench/ -bench BenchmarkMiddlewareToolsCall -benchmem -count=10`
- **Phase gate:** Full suite green + `benchstat` shows tracing-off delta ≤ +2 allocs/op vs `v1.2-phase11-github-hosted.txt`.

### Wave 0 Gaps

- [ ] `internal/obs/tracing_test.go` — covers TRACE-04, TRACE-05, D-15
- [ ] `internal/mcp/telemetry_span_test.go` — covers TRACE-02 (span creation, attributes, error status)
- [ ] `internal/kernel/symbols/tools_span_test.go` — covers TRACE-03 (one representative tool; rest via helper)
- [ ] `test/integration/trace_propagation_test.go` — covers TRACE-01 end-to-end via in-memory exporter + real gRPC loopback
- [ ] `test/integration/trace_shutdown_test.go` — covers PITFALLS #4
- [ ] `test/bench/` — extend existing `BenchmarkMiddlewareToolsCall` to report allocs/op against the Phase 11 baseline file
- [ ] Framework install: `go get` the five OTel modules at exact pinned versions

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | OTLP exporter authenticates to collector if configured (v1.3 — out of scope for v1.2) |
| V3 Session Management | no | — |
| V4 Access Control | no | Admin listener already loopback-only (Phase 10) |
| V5 Input Validation | yes (indirect) | Validate `cfg.Observability.TracingEndpoint` format (host:port), reject on malformed |
| V6 Cryptography | deferred | `otelgrpc.WithInsecure()` is fine for localhost loopback collector; TLS config is v1.3 |
| V8 Data Protection | **yes** | Spans and LS events MUST NOT contain file contents, symbol bodies, workspace paths — see Pitfall 7 + PITFALLS #12 (logging sensitive data) |

### Known Threat Patterns for this Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Proprietary source code leaked via span attributes | Information Disclosure | D-07 attribute allowlist (`tool_name`, `profile`, `mode`, `language`, `outcome`); no file path / symbol name attributes |
| High-cardinality attributes on tool-level span | Denial of Service (collector) | Span-attribute contract aligned with metrics allowlist (bounded cardinality) |
| OTLP endpoint pointing at attacker-controlled host | Information Disclosure | Document in USAGE.md; v1.2 trusts operator config (loopback default) |
| `WithInsecure()` on non-loopback endpoint | Info Disclosure in transit | Phase 12 keeps `WithInsecure`; Phase 14 docs warn; v1.3 adds TLS config |
| Shutdown race leaks in-flight spans | Availability of observability | Dedicated 5s flush context (Pattern 7) |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `otelgrpc v0.68.0` is compatible with `otel v1.43.0` | Standard Stack | Build break; fix by pinning the version pair from a single release matrix. Low risk — contrib matrix tracks core. |
| A2 | Forwarder needs its own `obs.Provider` | Pattern 3 + Pitfall 3 | If wrong (e.g., forwarder already has one from Phase 10), one planned task is redundant. Verified via `internal/forwarder/dial.go` read — no provider present; Phase 10 only wired the daemon. Low risk. |
| A3 | Discretion call — kernel tracer via constructor parameter is the best plumbing method | Pattern 5 | If a context-value approach is preferred, the diff is small but touches different call sites. Medium risk — user may have opinions. |
| A4 | `tracenoop.NewTracerProvider()` produces a tracer that allocates ≤ +2 allocs/op when spans are created and discarded | Pitfall 2 + D-17 | If noop still allocates, the D-17 budget is missed and Wave N adds allocation-avoidance gates. Verified by OTel docs calling it "zero-overhead" but NOT benchmarked in-repo yet — **benchmark is a Wave 0 gate**. Medium risk. |
| A5 | Phase 11 `v1.2-phase11-github-hosted.txt` baseline file exists under `.planning/phases/baselines/` | Validation Architecture | Confirmed by `ls` — file present. No risk. |
| A6 | CONTEXT.md v1.38.x was a drafting assumption, not a user-locked pin | Standard Stack | If user specifically wants v1.38, upgrade plan shifts backward. Low risk — CONTEXT.md's "v1.38.x" language came from STACK.md synthesis, not user input. Planner should confirm with user before pinning v1.43.0. |

## Open Questions

**STATUS: ALL RESOLVED** (2026-04-09, planner handoff)

1. **OTel version pin confirmation** — RESOLVED: pin v1.43.0 / otelgrpc v0.68.0. CONTEXT.md corrected.
   - What we know: CONTEXT.md references "v1.38.x"; actual current is v1.43.0 (2026-04-03).
   - What's unclear: whether the CONTEXT.md version was load-bearing or a best-guess from milestone research.
   - Recommendation: plan uses v1.43.0 (current stable); planner surfaces the version delta to user at start of Phase 12 discuss/plan handoff.

2. **Forwarder provider bootstrapping** — RESOLVED: noop-only provider in forwarder for v1.2; independent OTLP endpoint deferred to v1.3. Plan 03 implements.
   - What we know: Phase 10 only constructed the daemon provider; the forwarder has no observability scaffolding.
   - What's unclear: whether the forwarder should support its own `cfg.Observability.TracingEndpoint` (independent OTLP endpoint) or inherit the daemon's config somehow.
   - Recommendation: keep forwarder provider **noop-only** for v1.2. The forwarder span is cheap even with a noop provider, and avoiding a config duplication keeps scope tight. Deferred forwarder OTLP push is a v1.3 item.

3. **Tracer plumbing discretion (A3)** — RESOLVED: constructor parameter NewKernel(..., tracer trace.Tracer). Plan 04 implements.
   - Options:
     1. Constructor parameter on `NewKernel(..., tracer trace.Tracer)` — recommended
     2. Accessor on `obs.Provider` stashed in a kernel field
     3. Context value `tracer := trace.NewTracer(ctx)` at each handler entry
   - Recommendation: option 1 — matches existing `metrics lspool.MetricsSink` plumbing, zero runtime cost, test-ergonomic.

4. **Span helper vs per-tool wrapping** — RESOLVED: kernel.WrapToolSpan(tracer, toolName, handlerFn) helper applied at RegisterTools time. Plan 04 implements.
   - What we know: 24+ kernel tool handlers would benefit from a shared wrapper.
   - What's unclear: whether the helper lives in `internal/kernel` or inline in each tool package.
   - Recommendation: `kernel.WrapToolSpan(tracer, toolName, handlerFn)` in `internal/kernel`, called at `RegisterTools` time to wrap each handler exactly once. Minimal churn, centralized span contract.

## Sources

### Primary (HIGH confidence)
- `[VERIFIED]` `pkg.go.dev/go.opentelemetry.io/otel/sdk/trace` — `NewTracerProvider`, `WithBatcher`, `WithSampler`, `WithResource`, `ParentBased`, `TraceIDRatioBased` signatures (v1.43.0, 2026-04-03)
- `[VERIFIED]` `pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` — `NewServerHandler`, `NewClientHandler`, `WithTracerProvider` option list (v0.68.0, 2026-04-07)
- `[VERIFIED]` `pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` — `New(ctx, opts...)`, `WithEndpoint`, `WithInsecure`
- `[VERIFIED]` `github.com/open-telemetry/opentelemetry-go/releases` — v1.43.0 released 2026-04-03
- `[VERIFIED]` Existing Serena code — `internal/obs/obs.go`, `internal/obs/handler.go`, `internal/obs/spancontext.go`, `internal/mcp/middleware.go`, `internal/daemon/daemon.go`, `internal/daemon/shutdown.go`, `internal/forwarder/dial.go`, `internal/kernel/kernel.go`, `internal/kernel/symbols/tools.go`, `go.mod`
- `[VERIFIED]` `.planning/research/STACK.md`, `.planning/research/ARCHITECTURE.md`, `.planning/research/PITFALLS.md`, `.planning/research/SUMMARY.md`, `.planning/phases/12-tracing-end-to-end/12-CONTEXT.md`, `.planning/REQUIREMENTS.md`

### Secondary (MEDIUM confidence)
- `[CITED]` `opentelemetry.io/docs/languages/go/sampling/` — ParentBased + TraceIDRatioBased recommendation
- `[CITED]` `oneuptime.com/blog/post/2026-02-06-otel-statshandler-grpc-go/view` — StatsHandler superiority over interceptors (independent confirmation, 2026)

### Tertiary (LOW confidence)
- None. All load-bearing claims are verified against pkg.go.dev or the repo itself.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all versions verified against pkg.go.dev within 7 days
- Architecture: HIGH — matches Phase 10/11 precedent directly; all integration points read from source
- Pitfalls: HIGH — PITFALLS.md #4/#7 are repo-verified; new pitfalls (forwarder provider, version skew) identified from code read
- Security: MEDIUM-HIGH — D-07 attribute allowlist is pinned; TLS deferred to v1.3 is a deliberate scope call

**Research date:** 2026-04-08
**Valid until:** 2026-05-08 (30 days — OTel core v1 is stable, but contrib tracks 0.x and could ship a breaking change within the window; re-verify at Phase 12 Wave 0)
