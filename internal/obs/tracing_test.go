package obs

import (
	"context"
	"io"
	"log/slog"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestDefaultSamplerOff (TRACE-05): WithTracing + SampleRatio=0.0 + a root
// ctx (no parent) must produce a non-recording span, and the tracetest
// recorder must observe zero completed spans.
func TestDefaultSamplerOff(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.0))),
	)

	// Build a Provider manually with the test TracerProvider.
	p := Noop(slog.NewTextHandler(io.Discard, nil))
	p.tracerProvider = tp

	tracer := p.Tracer()
	ctx, span := tracer.Start(context.Background(), "test-op")
	_ = ctx

	if span.IsRecording() {
		t.Fatal("expected non-recording span with 0.0 sample ratio and no parent")
	}
	span.End()

	// Force flush to ensure any spans are exported.
	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush failed: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 0 {
		t.Fatalf("expected zero completed spans, got %d", len(spans))
	}
}

// TestTracingDegradedFallback (TRACE-04): WithTracing with an empty endpoint
// must return a noop Provider (the degraded path). The real degraded-optional
// code path is exercised by newTracerProvider returning an error — but the
// gRPC exporter only fails lazily on export, not at construction. So we test
// the documented contract: empty endpoint → noop, non-empty → real provider.
func TestTracingDegradedFallback(t *testing.T) {
	// Empty endpoint: WithTracing returns early with noop.
	p := WithTracing(slog.NewTextHandler(io.Discard, nil), TracingConfig{
		Endpoint:    "",
		ServiceName: "test-helix",
		SampleRatio: 1.0,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if p == nil {
		t.Fatal("expected non-nil Provider for empty endpoint")
	}

	tracer := p.Tracer()
	if tracer == nil {
		t.Fatal("expected non-nil Tracer from noop Provider")
	}

	// Noop tracer should produce non-recording spans.
	_, span := tracer.Start(context.Background(), "should-be-noop")
	if span.IsRecording() {
		t.Fatal("expected non-recording span from noop provider")
	}
	span.End()

	// Non-empty endpoint with unreachable host: construction succeeds (gRPC
	// connects lazily), but the returned provider is a real SDK provider.
	p2 := WithTracing(slog.NewTextHandler(io.Discard, nil), TracingConfig{
		Endpoint:    "localhost:0",
		ServiceName: "test-helix",
		SampleRatio: 1.0,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if p2 == nil {
		t.Fatal("expected non-nil Provider for non-empty endpoint")
	}

	tracer2 := p2.Tracer()
	_, span2 := tracer2.Start(context.Background(), "real-provider-op")
	if !span2.IsRecording() {
		t.Fatal("expected recording span from real SDK provider with SampleRatio=1.0")
	}
	span2.End()

	// Clean up SDK provider.
	p2.ShutdownTracing(context.Background())
}

// TestNoopTracerNonNil (D-04): Noop(handler).Tracer() returns non-nil;
// Start does not panic; returned span's IsRecording() is false.
func TestNoopTracerNonNil(t *testing.T) {
	p := Noop(slog.NewTextHandler(io.Discard, nil))

	tracer := p.Tracer()
	if tracer == nil {
		t.Fatal("expected non-nil Tracer from Noop Provider")
	}

	_, span := tracer.Start(context.Background(), "noop-op")
	if span.IsRecording() {
		t.Fatal("expected noop span to not be recording")
	}
	span.End() // must not panic
}
