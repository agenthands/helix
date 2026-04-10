package obs

import (
	"bytes"
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

// TestTracingDegradedFallback (TRACE-04): WithTracing with an unreachable
// endpoint must return a non-nil Provider whose Tracer() works as noop,
// and a warning must be logged.
func TestTracingDegradedFallback(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	cfg := TracingConfig{
		Endpoint:    "invalid://this-will-fail:0",
		ServiceName: "test-serena",
		SampleRatio: 1.0,
	}

	p := WithTracing(slog.NewTextHandler(io.Discard, nil), cfg, logger)
	if p == nil {
		t.Fatal("expected non-nil Provider even on exporter failure")
	}

	tracer := p.Tracer()
	if tracer == nil {
		t.Fatal("expected non-nil Tracer from degraded Provider")
	}

	// Start should not panic on the noop tracer.
	_, span := tracer.Start(context.Background(), "should-be-noop")
	span.End()

	// Check that a warning was logged.
	logOutput := buf.String()
	if !bytes.Contains([]byte(logOutput), []byte("tracing exporter construction failed")) {
		t.Fatalf("expected warning log about exporter failure, got: %s", logOutput)
	}
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
