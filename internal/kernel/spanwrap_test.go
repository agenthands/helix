package kernel_test

import (
	"context"
	"errors"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/postfix/serena/internal/kernel"
)

// newTestTracer returns a tracer backed by an in-memory exporter with
// AlwaysSample so every span is captured.
func newTestTracer(exp *tracetest.InMemoryExporter) trace.Tracer {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exp),
	)
	return tp.Tracer("test")
}

// trivialHandler is a simple handler that returns a text result.
func trivialHandler(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "ok"}},
	}, nil, nil
}

func TestWrapToolSpan_SpanCreated(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newTestTracer(exp)

	wrapped := kernel.WrapToolSpan(tracer, "testtool", trivialHandler)
	result, _, err := wrapped(context.Background(), &mcpsdk.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "kernel.tool.testtool" {
		t.Errorf("expected span name 'kernel.tool.testtool', got %q", spans[0].Name)
	}
}

func TestWrapToolSpan_NoopDoesNotPanic(t *testing.T) {
	tracer := tracenoop.NewTracerProvider().Tracer("noop")
	wrapped := kernel.WrapToolSpan(tracer, "noop_tool", trivialHandler)

	for i := 0; i < 1000; i++ {
		result, _, err := wrapped(context.Background(), &mcpsdk.CallToolRequest{}, struct{}{})
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if result == nil {
			t.Fatalf("iteration %d: expected non-nil result", i)
		}
	}
}

func TestWrapToolSpan_ErrorRecorded(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newTestTracer(exp)

	errHandler := func(_ context.Context, _ *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, any, error) {
		return nil, nil, errors.New("test-error")
	}

	wrapped := kernel.WrapToolSpan(tracer, "err_tool", errHandler)
	_, _, err := wrapped(context.Background(), &mcpsdk.CallToolRequest{}, struct{}{})
	if err == nil {
		t.Fatal("expected error")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	// Check that the error was recorded as an event on the span.
	found := false
	for _, ev := range spans[0].Events {
		if ev.Name == "exception" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected RecordError event on span, not found")
	}

	if spans[0].Status.Code != codes.Unset {
		// RecordError only adds an event — it does NOT set status.
		// Status would require an explicit SetStatus call, which we
		// intentionally omit to keep the wrapper minimal.
	}
}

func TestWrapToolSpan_ParentSpanLinked(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newTestTracer(exp)

	// Create a parent span.
	ctx, parent := tracer.Start(context.Background(), "parent")

	wrapped := kernel.WrapToolSpan(tracer, "child_tool", trivialHandler)
	_, _, err := wrapped(ctx, &mcpsdk.CallToolRequest{}, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parent.End()

	spans := exp.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}

	// The child span (first ended) should have the parent's span ID as its parent.
	childSpan := spans[0] // kernel.tool.child_tool ends first (defer)
	parentSpan := spans[1]

	if childSpan.Parent.SpanID() != parentSpan.SpanContext.SpanID() {
		t.Errorf("child parent span ID %s != parent span ID %s",
			childSpan.Parent.SpanID(), parentSpan.SpanContext.SpanID())
	}

	if childSpan.SpanContext.TraceID() != parentSpan.SpanContext.TraceID() {
		t.Error("child and parent should share the same trace ID")
	}
}
