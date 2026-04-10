---
phase: 12-tracing-end-to-end
plan: 05
subsystem: test, bench
tags: [tracing, benchmark, integration-test, phase-gate]
dependency_graph:
  requires: [12-02, 12-03, 12-04]
  provides: [tracing-off-benchmark, phase12-baseline, trace-propagation-test, trace-shutdown-test]
  affects: [test/bench, test/integration, internal/daemon]
tech_stack:
  added: []
  patterns: [InMemoryExporter-span-assertion, NewWithObsProvider-test-injection, benchstat-delta-gate]
key_files:
  created:
    - test/bench/tracing_bench_test.go
    - test/bench/baselines/v1.2-phase12-github-hosted.txt
    - test/integration/trace_propagation_test.go
    - test/integration/trace_shutdown_test.go
  modified:
    - internal/daemon/daemon.go
decisions:
  - "D-17 budget met: +1 alloc/op vs Phase 11 (within +2 limit); extra alloc from noop tracer.Start context.WithValue"
  - "Forwarder-to-daemon trace propagation tested via unit tests (forwarder_test.go) not integration (InMemoryTransports lack gRPC metadata)"
  - "daemon.NewWithObsProvider added for test TracerProvider injection without bloating production constructor"
  - "BenchmarkTracingOnPath captured at 10 allocs/op 2660 B/op for D-18 documentation"
metrics:
  duration_seconds: 765
  completed: "2026-04-10T07:06:19Z"
  tasks_completed: 2
  tasks_total: 2
  files_created: 4
  files_modified: 1
---

# Phase 12 Plan 05: Phase Gate -- Benchmarks + Integration Tests Summary

Hot-path tracing-off benchmark at +1 alloc/op vs Phase 11 (within D-17 +2 budget), Phase 12 baseline committed for Phase 13 delta gate, integration tests proving daemon+kernel 2-span tree with trace ID continuity and shutdown flush with dedicated context (PITFALLS #4).

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 9d4b3ef8 | Hot-path tracing benchmark + Phase 12 baseline capture |
| 2 | 2196ec7d | Integration tests for trace propagation + shutdown flush |

## Implementation Details

### Task 1: Hot-path Benchmark + Phase 12 Baseline

**BenchmarkTracingOffPath** exercises the full TelemetryMiddleware path with `obs.Noop` (noop tracer). Results:
- Phase 11 BenchmarkTelemetryMiddleware: 96 B/op, 2 allocs/op
- Phase 12 BenchmarkTracingOffPath: 144 B/op, 3 allocs/op
- **Delta: +1 alloc/op, +48 B/op** -- within the +2 allocs/op budget (D-17)

The extra allocation comes from `tracer.Start(ctx, "daemon.mcp.tools.call")` which creates a new context via `context.WithValue` even with a noop tracer. This is unavoidable given the tracer.Start call site exists in the middleware (added by Plan 02). The noop tracer elides attribute allocation and span recording.

**BenchmarkTracingOnPath** (informational, NOT gated): 2660 B/op, 10 allocs/op with AlwaysSample + InMemoryExporter. Captured for D-18 Phase 14 documentation.

Phase 12 baseline committed at `test/bench/baselines/v1.2-phase12-github-hosted.txt` with 10-count runs of all benchmark families.

### Task 2: Integration Tests

**TestTraceparentPropagation**: Starts a daemon with tracetest.InMemoryExporter via `daemon.NewWithObsProvider`. Activates a project, calls `read_file`, then asserts:
1. daemon.mcp.tools.call and kernel.tool.read_file spans both recorded
2. Both share the same TraceID (in-process context propagation)
3. kernel span's parent == daemon span (parent-child linkage)
4. daemon span has tool_name, profile, mode, outcome attributes (D-07)
5. kernel span has ZERO attributes (D-07 cardinality contract)

**TestForwarderSpanCreation**: Verifies forwarder.tools.call span is recorded with valid TraceID/SpanID. Forwarder-to-daemon propagation via otelgrpc is tested in forwarder unit tests (InMemoryTransports don't carry gRPC metadata).

**TestTraceFlushOnShutdown**: Proves PITFALLS #4 -- shutdown with a fresh `context.Background()` context succeeds, verifying the daemon's dedicated 5s flush context works. Also asserts double-shutdown doesn't panic (Pitfall 6).

**daemon.NewWithObsProvider**: New test-friendly constructor that accepts a pre-built `*obs.Provider`, enabling TracerProvider injection without modifying the production `New()` path. `ObsProvider()` accessor added for shutdown flush test assertions.

## Deviations from Plan

### Minor Deviations

**1. 2-span tree instead of 3-span tree in integration test**
- **Reason:** InMemoryTransports (used by the integration harness) don't propagate gRPC metadata, so the forwarder span's trace context doesn't reach the daemon. The forwarder.tools.call span is tested separately in `TestForwarderSpanCreation` and in `internal/forwarder/forwarder_test.go`. In production, otelgrpc bridges the gap via gRPC metadata automatically.
- **Impact:** The full 3-span tree is proven by composition: forwarder span (unit tested) + daemon+kernel tree (integration tested) + gRPC otelgrpc propagation (Plan 02/03).

**2. [Rule 2 - Missing] daemon.NewWithObsProvider constructor added**
- **Found during:** Task 2
- **Issue:** No way to inject a test TracerProvider into the daemon without modifying config
- **Fix:** Extracted shared `newDaemon()` helper, added `NewWithObsProvider` public constructor and `ObsProvider()` accessor
- **Files modified:** internal/daemon/daemon.go

## Verification Evidence

- `go vet ./...` exits 0
- `go build ./cmd/serena` exits 0
- `go test ./internal/...` all pass
- Integration tests (non-Java) all pass
- BenchmarkTracingOffPath: 3 allocs/op (delta +1 vs Phase 11's 2 allocs/op)
- Phase 12 baseline file exists with 10-count benchmark data

## Self-Check: PASSED
