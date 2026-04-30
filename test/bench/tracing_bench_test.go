// Package bench_test: Phase 12 tracing hot-path benchmark.
//
// BenchmarkTracingOffPath proves TRACE-05 / D-17: the tracing-disabled path
// (default config, no TracingEndpoint) adds <= +2 allocs/op vs the Phase 11
// BenchmarkTelemetryMiddleware baseline. The key difference from the Phase 11
// bench is that Phase 12 TelemetryMiddleware now calls tracer.Start()/span.End()
// on the tools/call path -- with a noop tracer those must be effectively free.
//
// BenchmarkTracingOnPath is informational (NOT gated): captures the real-tracing
// cost with an in-memory exporter for Phase 14 documentation (D-18).
//
// All benchmarks use testing.B.Loop (Go 1.25, BENCH-01) and report allocations.
package bench_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	serenamcp "github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
)

// BenchmarkTracingOffPath exercises TelemetryMiddleware with an obs.Noop
// provider (noop tracer). This is the production-default path when
// TracingEndpoint is empty. The allocs/op must be within +2 of the Phase 11
// BenchmarkTelemetryMiddleware baseline (D-17 budget).
func BenchmarkTracingOffPath(b *testing.B) {
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := &serenamcp.SessionInfo{
		Profile:  "claude-code",
		Mode:     "edit",
		Language: "go",
	}
	getSession := func(ctx context.Context) *serenamcp.SessionInfo { return sess }

	mw := serenamcp.TelemetryMiddleware(provider, getSession, nil, logger)
	wrapped := mw(noopInnerHandler)
	req := &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "find_symbol",
			Arguments: json.RawMessage(`{}`),
		},
	}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = wrapped(ctx, "tools/call", req)
	}
}

// BenchmarkTracingOnPath exercises TelemetryMiddleware with tracing ENABLED
// (AlwaysSample + in-memory exporter). This is informational only -- NOT
// gated against any budget. Captures the real-tracing cost for Phase 14
// documentation (D-18).
func BenchmarkTracingOnPath(b *testing.B) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	provider := obs.NewForTest(tp)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := &serenamcp.SessionInfo{
		Profile:  "claude-code",
		Mode:     "edit",
		Language: "go",
	}
	getSession := func(ctx context.Context) *serenamcp.SessionInfo { return sess }

	mw := serenamcp.TelemetryMiddleware(provider, getSession, nil, logger)
	wrapped := mw(noopInnerHandler)
	req := &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      "find_symbol",
			Arguments: json.RawMessage(`{}`),
		},
	}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = wrapped(ctx, "tools/call", req)
		exporter.Reset() // prevent unbounded memory growth
	}
}
