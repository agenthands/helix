---
phase: 12-tracing-end-to-end
reviewed: 2026-04-10T12:00:00Z
depth: standard
files_reviewed: 28
files_reviewed_list:
  - go.mod
  - go.sum
  - internal/config/config.go
  - internal/config/defaults.go
  - internal/daemon/daemon.go
  - internal/daemon/shutdown.go
  - internal/forwarder/dial.go
  - internal/forwarder/forwarder_test.go
  - internal/forwarder/forwarder.go
  - internal/kernel/diag/tools.go
  - internal/kernel/edit/tools.go
  - internal/kernel/fileops/tools.go
  - internal/kernel/kernel.go
  - internal/kernel/lspool/worker.go
  - internal/kernel/spanwrap_test.go
  - internal/kernel/spanwrap.go
  - internal/kernel/symbols/tools.go
  - internal/mcp/middleware.go
  - internal/mcp/telemetry_span_test.go
  - internal/obs/handler_test.go
  - internal/obs/obs.go
  - internal/obs/spancontext_test.go
  - internal/obs/spancontext.go
  - internal/obs/tracing_test.go
  - internal/obs/tracing.go
  - test/bench/baselines/v1.2-phase12-github-hosted.txt
  - test/bench/tracing_bench_test.go
  - test/integration/trace_propagation_test.go
  - test/integration/trace_shutdown_test.go
findings:
  critical: 0
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 12: Code Review Report

**Reviewed:** 2026-04-10T12:00:00Z
**Depth:** standard
**Files Reviewed:** 28
**Status:** issues_found

## Summary

Phase 12 introduces end-to-end distributed tracing using OpenTelemetry across the forwarder, daemon MCP middleware, kernel tool handlers, and LS worker pool. The implementation is well-structured with clear separation of concerns: `obs.Provider` is the single entry point, `WrapToolSpan` provides a clean generic decorator, `TelemetryMiddleware` creates the daemon-level span, and `spanContextFromContext` bridges OTel trace IDs into slog records.

The code follows the project's design rules consistently -- no global TracerProvider usage (D-01), noop fallback everywhere (D-04), degraded-optional pattern for exporter failures (D-09), and IsRecording() gating for hot-path budget protection (D-17). Test coverage is thorough, with unit tests for every new component and integration tests verifying the full span tree.

Three warnings and four info items were identified. No critical security or correctness issues were found.

## Warnings

### WR-01: OTLP exporter always uses WithInsecure

**File:** `internal/obs/tracing.go:43-45`
**Issue:** `newTracerProvider` unconditionally passes `otlptracegrpc.WithInsecure()` to the OTLP/gRPC exporter. When `TracingEndpoint` is configured to point at a remote collector (not localhost), trace data -- including tool names, profile names, and span metadata -- is transmitted in plaintext. While the config comment says "v1.3 adds auth," operators may deploy this in v1.2 with a remote endpoint, unaware that TLS is disabled.
**Fix:** At minimum, add a `TracingInsecure bool` field to `TracingConfig` (defaulting to `true` for backward compatibility) and gate `WithInsecure()` on it. Document the security implication in the config struct comment:
```go
// TracingInsecure disables TLS for the OTLP/gRPC exporter. Default true
// for localhost collectors. Set to false when using a remote endpoint.
TracingInsecure bool
```
Then conditionally apply:
```go
opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
if cfg.TracingInsecure {
    opts = append(opts, otlptracegrpc.WithInsecure())
}
exporter, err := otlptracegrpc.New(ctx, opts...)
```

### WR-02: isToolsCall substring matching could false-positive on payload data

**File:** `internal/forwarder/forwarder.go:127-130`
**Issue:** `isToolsCall` uses `bytes.Contains` to match `"method":"tools/call"` and the spaced variant. If any tool argument value (or user-supplied string in a `params` field) happens to contain this exact substring, the function will return a false positive, causing an unnecessary span to be created. While this is not a correctness bug (an extra noop span is harmless), it represents a fragile detection mechanism that could mask real issues in diagnostic scenarios.
**Fix:** Consider also checking for `"jsonrpc"` at the start of the payload, or matching only within the first N bytes (method fields appear early in JSON-RPC messages). For a more robust approach, at minimum anchor the match to avoid matching inside string values:
```go
// Only match if "method" appears before any "params" key
methodIdx := bytes.Index(payload, toolsCallMethod)
if methodIdx < 0 {
    methodIdx = bytes.Index(payload, toolsCallMethodSpaced)
}
paramsIdx := bytes.Index(payload, []byte(`"params"`))
return methodIdx >= 0 && (paramsIdx < 0 || methodIdx < paramsIdx)
```

### WR-03: TracerProvider shutdown called without protection against concurrent use

**File:** `internal/daemon/shutdown.go:36-39`
**Issue:** `d.obs.ShutdownTracing(flushCtx)` is called during `shutdown()`, but there is no synchronization to ensure that in-flight tool calls have finished using the TracerProvider's Tracer before shutdown is called. If a tool call is actively executing `tracer.Start()` or `span.End()` concurrently with `TracerProvider.Shutdown()`, the OTel SDK documentation notes this can result in dropped spans or undefined behavior. The errgroup context cancellation signals goroutines to stop, but there is no `WaitGroup` or similar mechanism to confirm they have drained before `ShutdownTracing` is called.
**Fix:** Consider adding a brief grace period or a wait mechanism between errgroup context cancellation and the tracing shutdown call. The existing `kernel.Shutdown(ctx)` call provides partial protection, but MCP handlers running on in-flight gRPC streams could still race. A simple mitigation:
```go
// Phase 1: Stop kernel (drain workers).
if d.kernel != nil {
    if err := d.kernel.Shutdown(ctx); err != nil {
        d.logger.Warn("kernel shutdown error", "error", err)
    }
}
// Brief grace for in-flight MCP handlers to complete.
time.Sleep(100 * time.Millisecond)
// Phase 1.5: Flush tracing.
```

## Info

### IN-01: TODO comments for v1.3 outcome wiring

**File:** `internal/mcp/middleware.go:51-54`
**Issue:** Three TODO comments mark outcome enum values (`invalid_args`, `not_found`, `ls_crash`) as deferred to v1.3. These are intentional design placeholders, not forgotten work, but they will accumulate if not tracked.
**Fix:** Ensure these are captured in a v1.3 planning artifact or roadmap issue so they do not become stale TODOs.

### IN-02: Unused import of `trace` package in `forwarder.go`

**File:** `internal/forwarder/forwarder.go:13`
**Issue:** The `go.opentelemetry.io/otel/trace` import is used only for the `trace.Tracer` type in `sendWithSpan`. The forwarder constructs a noop provider and immediately extracts a tracer from it. This is fine architecturally but the trace import could be avoided by accepting a `trace.Tracer` directly in `RunForwarder` rather than building a provider internally.
**Fix:** Minor refactoring opportunity for cleaner dependency injection. Low priority.

### IN-03: Benchmark baseline file notes "placeholder capture pending" for GitHub-hosted runner

**File:** `test/bench/baselines/v1.2-phase12-github-hosted.txt:6`
**Issue:** The baseline file header states it was captured on "local dev (darwin/arm64)" and notes a GitHub-hosted CI run is pending. The filename says "github-hosted" but the data is from local dev. This naming inconsistency could confuse future CI comparisons.
**Fix:** Either rename the file to reflect it is a local capture (e.g., `v1.2-phase12-local-dev.txt`) or update it once the CI run completes.

### IN-04: `Sprintf` with string-only format in blast radius output

**File:** `internal/kernel/symbols/tools.go:411`
**Issue:** `fmt.Sprintf("\nCallers:\n")` is used where a plain string literal would suffice. This is a minor inefficiency and code style issue.
**Fix:** Replace with string concatenation or direct `WriteString`:
```go
sb.WriteString("\nCallers:\n")
```

---

_Reviewed: 2026-04-10T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
