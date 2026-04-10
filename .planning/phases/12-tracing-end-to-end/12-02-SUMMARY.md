---
phase: 12-tracing-end-to-end
plan: 02
subsystem: daemon, mcp
tags: [tracing, otelgrpc, middleware, span, shutdown]
dependency_graph:
  requires: [obs.Tracer, obs.TracerProvider, obs.ShutdownTracing, obs.WithTracing, obs.TracingConfig, config.TracingEndpoint]
  provides: [daemon-otelgrpc-server-handler, daemon.mcp.tools.call-span, tracing-shutdown-flush]
  affects: [internal/daemon, internal/mcp, internal/obs]
tech_stack:
  added: [go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc@v0.68.0]
  patterns: [explicit-TracerProvider-injection, span-IsRecording-gate, dedicated-shutdown-context]
key_files:
  created:
    - internal/mcp/telemetry_span_test.go
  modified:
    - internal/daemon/daemon.go
    - internal/daemon/shutdown.go
    - internal/mcp/middleware.go
    - internal/obs/obs.go
    - go.mod
    - go.sum
decisions:
  - "otelgrpc.NewServerHandler uses explicit WithTracerProvider (D-01: no global)"
  - "Tracing flush uses context.Background()+5s, ordered before listener close (PITFALLS #4)"
  - "provider.Tracer() captured once outside inner closure for hot-path efficiency"
  - "span.IsRecording() gate prevents attribute allocation when tracing off (D-17)"
  - "obs.NewForTest helper added for test TracerProvider injection"
metrics:
  duration: 5min
  completed: 2026-04-10
---

# Phase 12 Plan 02: Daemon gRPC Handler + TelemetryMiddleware Span Summary

otelgrpc.NewServerHandler on daemon gRPC with explicit TracerProvider, conditional obs.WithTracing/Noop construction, TelemetryMiddleware extended with daemon.mcp.tools.call span (D-07 attributes), and shutdown flush with dedicated 5s context before listener close.

## What Was Done

### Task 1: Wire daemon gRPC server handler + obs construction + shutdown flush

- Replaced unconditional `obs.Noop(handler)` with conditional construction: `obs.WithTracing` when `cfg.Observability.TracingEndpoint != ""`, else `obs.Noop`
- Added `otelgrpc.NewServerHandler(otelgrpc.WithTracerProvider(d.obs.TracerProvider()))` to `grpc.NewServer()` in `listenSocket` -- explicit provider injection per D-01
- Inserted tracing flush in `shutdown.go` between kernel shutdown (Phase 1) and listener close (Phase 2): `d.obs.ShutdownTracing(flushCtx)` with `context.WithTimeout(context.Background(), 5*time.Second)` -- NOT the cancelled errgroup ctx (PITFALLS #4)
- Flush block runs exactly once per daemon lifecycle via the single-shot `shutdown()` call (Pitfall 6: double-call panic prevention)

### Task 2: Extend TelemetryMiddleware with span creation (TDD)

**RED:** 4 tests written in `telemetry_span_test.go`:
- `TestTelemetryMiddlewareSpan_ToolCallCreatesSpan` -- verifies span name + 5 D-07 attributes
- `TestTelemetryMiddlewareSpan_NonToolMethodNoSpan` -- verifies initialize bypasses span
- `TestTelemetryMiddlewareSpan_ErrorRecorded` -- verifies codes.Error status + RecordError event
- `TestTelemetryMiddlewareSpan_NoopTracerZeroCost` -- verifies noop path doesn't panic, metrics work

**GREEN:** Extended `TelemetryMiddleware` in middleware.go:
- `tracer := provider.Tracer()` captured once outside inner closure (not per-request)
- Non-tool methods take early return with log-only path -- no span created
- `tools/call` path: `tracer.Start(ctx, "daemon.mcp.tools.call")` + `defer span.End()`
- Attributes set behind `span.IsRecording()` gate: tool_name, profile, mode, language, outcome
- Error path: `span.RecordError(err)` + `span.SetStatus(codes.Error, err.Error())`
- Phase 11 metrics emission (ToolCalls.Inc, ToolDuration.Observe) preserved byte-for-byte with identical label order

**Regression:** All 9 Phase 11 TelemetryMiddleware tests pass unchanged.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 2f34f667 | Wire daemon gRPC otelgrpc handler, obs construction, shutdown flush |
| 2 (RED) | 3711cd46 | Add failing span tests for TelemetryMiddleware |
| 2 (GREEN) | 0b82363b | Extend TelemetryMiddleware with daemon.mcp.tools.call span |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Forwarder otelgrpc changes from Plan 01**
- **Found during:** Task 1
- **Issue:** Plan 01 left uncommitted forwarder changes (dial.go, forwarder.go, forwarder_test.go) that added TracerProvider parameter to connectOrStartDaemon/tryConnect/waitForDaemon and otelgrpc.NewClientHandler. These were already in the working tree from Plan 01's worktree merge but not explicitly part of Plan 02's scope.
- **Fix:** Included in Task 1 commit since they were necessary for compilation.
- **Files modified:** internal/forwarder/dial.go, internal/forwarder/forwarder.go, internal/forwarder/forwarder_test.go (already present, no new changes needed)

## Verification Evidence

- `go vet ./internal/... ./cmd/...` exits 0
- `go build ./cmd/serena` exits 0
- `go test ./internal/daemon/... ./internal/mcp/... -count=1` all pass
- No `otel.SetTracerProvider` or `otel.GetTracerProvider` in internal/ (excluding comments)
- Shutdown ordering: ShutdownTracing at line 37, socketListener.Close at line 45
- Phase 11 metrics tests (9 tests) all pass unchanged

## Self-Check: PASSED
