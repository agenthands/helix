# Phase 12: Tracing End-to-End - Context

**Gathered:** 2026-04-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Propagate distributed traces from forwarder through daemon into kernel tool execution, off by default, with OTLP export as an opt-in flag. Wires trace IDs into slog records (Phase 10 `ContextHandler` already supports this via `SpanContext` stub, which Phase 12 reimplements to extract from `trace.SpanContextFromContext`).

</domain>

<decisions>
## Implementation Decisions

### OTel TracerProvider Wiring
- **D-01:** Explicit dependency injection via `obs.Provider.Tracer() trace.Tracer`. NO global `otel.SetTracerProvider()`.
- **D-02:** Rationale: test-safe (multiple daemon instances in the same process each get their own provider), clean dependency graph, matches Phase 10/11 pattern.
- **D-03:** `otelgrpc.NewServerHandler` / `NewClientHandler` and `otelslog.NewHandler` are instantiated with explicit `trace.TracerProvider` argument (not auto-discovery from global).
- **D-04:** Phase 10 `obs.Provider` struct extends with `Tracer()` method. Noop provider returns a noop tracer (so callers always get a non-nil tracer).

### Span Granularity
- **D-05:** Three-span tree per tool call:
  1. `forwarder.tools.call` (forwarder entry)
  2. `daemon.mcp.tools.call` (daemon telemetry middleware)
  3. `kernel.tool.{tool_name}` (kernel tool execution)
- **D-06:** LSP interactions emit **span events** (not child spans) on the kernel span:
  - `name: ls.request`
  - `attributes: {lsp.method, lsp.language, lsp.duration_ms}`
- **D-07:** Standard span attributes per D-05/D-06: `tool_name`, `profile`, `mode`, `language` (matches metrics label contract).
- **D-08:** Rationale: conservative, low overhead, minimal cardinality. LS events give debuggers enough context without 2x span count per call.

### OTLP Exporter Lifecycle
- **D-09:** Daemon-managed lifecycle (degraded-optional pattern from Phase 10 admin listener):
  - `daemon.New()` constructs OTLP exporter + `sdktrace.TracerProvider` if `cfg.Observability.TracingEndpoint` is non-empty
  - Construction errors log warning + fall back to noop (does NOT fail daemon startup)
  - `daemon.Shutdown()` flushes + closes exporter within a dedicated 5s shutdown context (not the cancelled errgroup context — per PITFALLS #4 flush race)
- **D-10:** Default sampler: `ParentBased(TraceIDRatioBased(0.0))` — tracing is zero-cost unless explicitly turned on via config.
- **D-11:** Config fields added:
  - `cfg.Observability.TracingEndpoint string` — OTLP/gRPC endpoint (empty = tracing disabled)
  - `cfg.Observability.TracingSampleRatio float64` — 0.0 default, 1.0 max
  - `cfg.Observability.ServiceName string` — resource attribute (default "serena")

### Context Propagation
- **D-12:** `otelgrpc.NewServerHandler` on daemon gRPC server (IPC), `otelgrpc.NewClientHandler` on forwarder gRPC client. Traceparent flows via gRPC metadata automatically.
- **D-13:** MCP SDK middleware chain (telemetry middleware from Phase 11) creates the `daemon.mcp.tools.call` span, extracting parent from ctx.
- **D-14:** Kernel tools add sub-spans inside their handlers via `tracer.Start(ctx, "kernel.tool.foo")`. Minimal API: plumb tracer through kernel constructor.

### Phase 10 SpanContext Stub Replacement
- **D-15:** `internal/obs/spancontext.go` — `spanContextFromContext(ctx)` now extracts from `trace.SpanContextFromContext(ctx)` instead of returning false. `SpanContext{TraceID, SpanID string}` populated from `otel/trace.SpanContext` hex strings.
- **D-16:** Phase 10 `ContextHandler` unchanged — it already reads from `spanContextFromContext`. This is the zero-change wire-up.

### Hot-Path Budget
- **D-17:** Tracing-off baseline (default, `TracingEndpoint == ""`): ≤ +2 allocs/op vs Phase 11 baseline (`v1.2-phase11-github-hosted.txt`). Achieved via noop tracer and sampler dropping spans before attribute allocation.
- **D-18:** Tracing-on (1% sampling) is NOT gated against the budget — overhead is acceptable when operator opts in. Benchmark captures the number for Phase 14 docs.

### Claude's Discretion
- Exact package layout (`internal/obs/tracing.go` vs extending `obs.go`)
- Exporter initialization order (before or after kernel start)
- Span attribute naming (OTel semconv vs Serena-native)
- How to plumb the tracer into kernel — constructor parameter vs obs.Provider accessor vs context-stashed

</decisions>

<canonical_refs>
## Canonical References

### Milestone Research
- `.planning/research/STACK.md` — `go.opentelemetry.io/otel` v1.43.0 (Research correction 2026-04-08: actual current stable; was v1.38.x in draft) + OTLP/gRPC + otelgrpc StatsHandler + otelslog bridge
- `.planning/research/ARCHITECTURE.md` — Explicit provider wiring, 3-span tree, degraded-optional lifecycle
- `.planning/research/PITFALLS.md` — #4 flush race (separate shutdown ctx), #7 OTel overhead (low sampler default), ctx propagation audit

### Phase 10 Foundation
- `internal/obs/obs.go` — Provider struct (needs Tracer() extension)
- `internal/obs/spancontext.go` — stub extractor to replace
- `internal/obs/handler.go` — ContextHandler reads from SpanContext (unchanged)
- `internal/daemon/telemetry.go` — admin listener pattern (precedent for degraded-optional lifecycle)

### Phase 11 Integration
- `internal/mcp/middleware.go` — `TelemetryMiddleware` (Phase 12 adds span creation here)
- `internal/obs/metrics.go` — `obs.Metrics` unchanged (Phase 12 adds tracer to sibling)

### Existing gRPC
- `api/proto/serena/v1/` — gRPC IPC (forwarder ↔ daemon)
- `internal/forwarder/dial.go` — where `otelgrpc.NewClientHandler` goes
- `internal/daemon/daemon.go` — where `otelgrpc.NewServerHandler` goes

### External
- [opentelemetry-go releases](https://github.com/open-telemetry/opentelemetry-go/releases) — v1.43.0 (Research correction 2026-04-08: actual current stable; was v1.38.x in draft) pin
- [otelgrpc StatsHandler](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc)
- [OTel Go sampling](https://opentelemetry.io/docs/languages/go/sampling/)
- ~~otelslog bridge~~ (NOT used — Phase 10 ContextHandler is more efficient; replacing spanContextFromContext stub achieves the same outcome at zero alloc cost)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `obs.Provider` struct (Phase 10) — extend with `Tracer()` method and `tracerProvider *sdktrace.TracerProvider` field
- `obs.SpanContext` stub (Phase 10) — reimplement extractor
- `ContextHandler` (Phase 10) — zero changes, automatically starts injecting trace IDs once extractor returns true
- `TelemetryMiddleware` (Phase 11) — add span start/end around tool call
- `internal/daemon/telemetry.go` (Phase 10) — degraded-optional pattern to mirror for OTLP exporter lifecycle
- v1.2-phase11 baseline — Phase 12 delta gate target

### Established Patterns
- Noop-default providers
- Degraded-optional startup (admin listener, now OTLP exporter)
- Bounded config in `cfg.Observability`
- Explicit ctx flow through middleware chain

### Integration Points
- New file `internal/obs/tracing.go` — TracerProvider construction, exporter lifecycle
- Modified `internal/obs/obs.go` — add `Tracer()` method + `tracerProvider` field
- Modified `internal/obs/spancontext.go` — replace stub extractor
- Modified `internal/mcp/middleware.go` — add span start/end in TelemetryMiddleware
- Modified `internal/kernel/kernel.go` (or tool handlers) — sub-span creation
- Modified `internal/forwarder/dial.go` — add otelgrpc.NewClientHandler
- Modified `internal/daemon/daemon.go` — add otelgrpc.NewServerHandler, exporter lifecycle
- Modified `internal/config/config.go` — add TracingEndpoint, TracingSampleRatio, ServiceName
- New benchmark: extend `test/bench/metrics_bench_test.go` or add `tracing_bench_test.go`

</code_context>

<specifics>
## Specific Ideas

- `otelgrpc` at gRPC layer (automatic propagation), tracer plumbed explicitly through Go call chain
- 3-span tree with LS events (not child spans) keeps overhead bounded
- Default sampler zero rate — tracing is strictly opt-in
- Flush uses dedicated 5s shutdown context (Pitfall #4 fix)

</specifics>

<deferred>
## Deferred Ideas

- Per-LS-request child spans (could land in v1.3 behind `--trace-ls` flag)
- Histogram exemplars linking trace IDs (requires Prometheus 2.40+ scrape side)
- Trace sampling based on tool_name or outcome (smart sampling) — v1.3+
- Tracing over HTTP transport (v1.2 only instruments gRPC; HTTP is forwarder-internal)

</deferred>

---

*Phase: 12-tracing-end-to-end*
*Context gathered: 2026-04-09*
