package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestSpanContextRealExtraction (D-15): given ctx carrying a tracetest-generated
// span, spanContextFromContext returns (populated, true) with non-empty hex
// TraceID (32 chars) and SpanID (16 chars).
func TestSpanContextRealExtraction(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "extract-test")
	defer span.End()

	sc, ok := spanContextFromContext(ctx)
	if !ok {
		t.Fatal("expected spanContextFromContext to return true for active span")
	}
	if len(sc.TraceID) != 32 {
		t.Fatalf("expected 32-char hex TraceID, got %q (len=%d)", sc.TraceID, len(sc.TraceID))
	}
	if len(sc.SpanID) != 16 {
		t.Fatalf("expected 16-char hex SpanID, got %q (len=%d)", sc.SpanID, len(sc.SpanID))
	}
	if sc.TraceID == "00000000000000000000000000000000" {
		t.Fatal("TraceID must not be all zeros")
	}
	if sc.SpanID == "0000000000000000" {
		t.Fatal("SpanID must not be all zeros")
	}
}

// TestContextHandlerWithRealSpan: given a ctx with an active OTel span,
// slog records emitted via Provider.Handler() must contain trace_id and
// span_id matching the active span.
func TestContextHandlerWithRealSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "handler-test")
	defer span.End()

	// Capture JSON log output.
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := NewContextHandler(inner)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	if err := handler.Handle(ctx, r); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Parse the JSON output.
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse JSON log: %v\nraw: %s", err, buf.String())
	}

	traceID, ok := logEntry["trace_id"].(string)
	if !ok || traceID == "" {
		t.Fatalf("expected trace_id in log output, got: %v", logEntry)
	}
	spanID, ok := logEntry["span_id"].(string)
	if !ok || spanID == "" {
		t.Fatalf("expected span_id in log output, got: %v", logEntry)
	}

	// Verify they match the active span.
	otelSC := span.SpanContext()
	if traceID != otelSC.TraceID().String() {
		t.Fatalf("trace_id mismatch: log=%q otel=%q", traceID, otelSC.TraceID().String())
	}
	if spanID != otelSC.SpanID().String() {
		t.Fatalf("span_id mismatch: log=%q otel=%q", spanID, otelSC.SpanID().String())
	}
}
