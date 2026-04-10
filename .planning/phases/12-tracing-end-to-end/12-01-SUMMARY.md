---
phase: 12-tracing-end-to-end
plan: 01
subsystem: obs
tags: [tracing, otel, config, spancontext]
dependency_graph:
  requires: []
  provides: [obs.Tracer, obs.TracerProvider, obs.ShutdownTracing, obs.WithTracing, obs.TracingConfig, config.TracingEndpoint, config.TracingSampleRatio, config.ServiceName, spanContextFromContext-real]
  affects: [internal/obs, internal/config]
tech_stack:
  added: [go.opentelemetry.io/otel@v1.43.0, go.opentelemetry.io/otel/sdk@v1.43.0, go.opentelemetry.io/otel/exporters/otlp/otlptrace@v1.43.0, go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.43.0]
  patterns: [degraded-optional-lifecycle, noop-default-tracerProvider, explicit-DI-no-globals]
key_files:
  created:
    - internal/obs/tracing.go
    - internal/obs/tracing_test.go
    - internal/obs/spancontext_test.go
  modified:
    - internal/obs/obs.go
    - internal/obs/spancontext.go
    - internal/obs/handler_test.go
    - internal/config/config.go
    - internal/config/defaults.go
    - go.mod
    - go.sum
decisions:
  - "OTel pinned at v1.43.0 (was v1.39.0 transitive); SDK tracer never used on default path (tracenoop only)"
  - "WithSpanContext helper removed; Phase 12 extractor uses trace.SpanContextFromContext directly"
metrics:
  duration: 5min
  completed: 2026-04-10
---

# Phase 12 Plan 01: OTel TracerProvider Foundation Summary

OTel v1.43.0 TracerProvider with degraded-optional OTLP/gRPC exporter, noop-default Tracer() accessor, real spanContextFromContext extractor, and config schema for TracingEndpoint/TracingSampleRatio/ServiceName.

## What Was Done

### Task 1: Pin OTel modules and scaffold tracing.go
- Upgraded OTel from v1.39.0 (transitive) to v1.43.0 (direct dependency)
- Created `internal/obs/tracing.go` with `TracingConfig`, `newTracerProvider()`, and `WithTracing()` constructor
- Extended `Provider` struct with `tracerProvider trace.TracerProvider` field
- Added `Tracer()`, `TracerProvider()`, `ShutdownTracing()` methods on Provider
- `Noop()` initialises with `tracenoop.NewTracerProvider()` (D-17 hot-path safe)
- No `otel.SetTracerProvider()` anywhere (D-01 enforced)

### Task 2: Replace spanContext stub, add TracingConfig plumbing, unit tests
- Replaced Phase 10 stub `spanContextFromContext` with real OTel extractor using `trace.SpanContextFromContext`
- Added `TracingEndpoint`, `TracingSampleRatio`, `ServiceName` to `ObservabilityConfig`
- Set defaults: endpoint="" (disabled), ratio=0.0 (off), service_name="serena"
- Five tests covering TRACE-04, TRACE-05, D-04, D-15:
  - `TestDefaultSamplerOff` -- proves ParentBased(TraceIDRatioBased(0.0)) produces non-recording spans
  - `TestTracingDegradedFallback` -- proves invalid endpoint falls back to noop with warning
  - `TestNoopTracerNonNil` -- proves Noop path yields functional noop tracer
  - `TestSpanContextRealExtraction` -- proves real trace/span ID extraction from OTel context
  - `TestContextHandlerWithRealSpan` -- proves trace_id/span_id appear in JSON slog output

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 0e2d0709 | Pin OTel v1.43.0, scaffold tracing.go, extend Provider |
| 2 | 431e730b | Replace spanContext stub, add config fields, 5 unit tests |

## Deviations from Plan

### Minor Deviations

**1. otelgrpc not pinned in go.mod**
- **Reason:** `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc@v0.68.0` was installed but `go mod tidy` removed it because no Go source file imports it yet. It will be added automatically when Plan 12-02/03 imports it in forwarder/daemon code.

**2. [Rule 1 - Bug] WithSpanContext helper removed**
- **Found during:** Task 2
- **Issue:** The Phase 10 `WithSpanContext` exported helper (context-value based) is now dead code since `spanContextFromContext` uses `trace.SpanContextFromContext` instead of context-value lookup.
- **Fix:** Removed the helper and its `ctxKey` type. Updated the Phase 10 stub test to test no-span behavior instead.
- **Files modified:** internal/obs/spancontext.go, internal/obs/handler_test.go

## Self-Check: PASSED
