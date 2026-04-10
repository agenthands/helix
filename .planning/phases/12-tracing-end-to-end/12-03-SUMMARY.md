---
phase: 12-tracing-end-to-end
plan: 03
subsystem: forwarder
tags: [tracing, otel, otelgrpc, forwarder, noop-provider]
dependency_graph:
  requires: [obs.Noop, obs.Tracer, obs.TracerProvider]
  provides: [forwarder.tools.call-span, otelgrpc-client-handler, forwarder-obs-provider]
  affects: [internal/forwarder]
tech_stack:
  added: [go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc@v0.68.0]
  patterns: [explicit-TracerProvider-DI, byte-substring-method-detection, noop-default-forwarder]
key_files:
  created: []
  modified:
    - internal/forwarder/forwarder.go
    - internal/forwarder/dial.go
    - internal/forwarder/forwarder_test.go
    - go.mod
    - go.sum
decisions:
  - "Forwarder uses noop obs.Provider only for v1.2 -- no OTLP endpoint config exposed"
  - "TracerProvider threaded as parameter (not struct field) for test ergonomics"
  - "isToolsCall uses byte substring matching to avoid JSON parsing on hot path"
  - "sendWithSpan returns error directly rather than writing to channel for proper goroutine control flow"
metrics:
  duration: 4min
  completed: 2026-04-10
---

# Phase 12 Plan 03: Forwarder OTel Provider + Root Span Summary

Forwarder noop obs.Provider with otelgrpc client stats handler (explicit TracerProvider, D-01) and forwarder.tools.call root span gated to tools/call messages only.

## What Was Done

### Task 1: Construct forwarder obs.Provider + install otelgrpc client handler
- Added `obs.Noop(logger.Handler())` in `RunForwarder` to create forwarder-local noop provider
- Threaded `trace.TracerProvider` parameter through `connectOrStartDaemon`, `tryConnect`, and `waitForDaemon`
- Installed `otelgrpc.NewClientHandler(otelgrpc.WithTracerProvider(tp))` as gRPC dial option in `dial.go`
- Single `grpc.WithStatsHandler` call (Pitfall 5 safe -- no duplicate)
- Zero use of `otel.SetTracerProvider` or `otel.GetTracerProvider` (D-01 enforced)
- Added `otelgrpc@v0.68.0` dependency to go.mod
- Updated existing tests to pass noop TracerProvider

### Task 2: Wrap forwarder tools/call dispatch in forwarder.tools.call root span
- Added `isToolsCall()` helper using `bytes.Contains` on two variants of `"method":"tools/call"` (with/without space)
- Added `sendWithSpan()` that creates `forwarder.tools.call` span, sends message, ends span
- Non-tools/call messages bypass span creation entirely (hot path untouched)
- With noop tracer (default), `tracer.Start` returns non-recording span -- overhead is one function call
- Tests:
  - `TestIsToolsCall` -- table test covering tools/call, initialize, tools/list, empty
  - `TestForwarderRootSpan` -- uses tracetest.InMemoryExporter + AlwaysSample to verify span name
  - `TestForwarderRootSpan_NoopTracer` -- verifies zero-error forwarding with noop tracer

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | c58510b9 | Construct forwarder obs.Provider and install otelgrpc client handler |
| 2 | 8bd876bb | Wrap forwarder tools/call dispatch in root span |

## Deviations from Plan

None -- plan executed exactly as written.

## Self-Check: PASSED
