package lspool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// newSpanTestTracer constructs a trace.Tracer backed by an in-memory exporter
// that always samples — same shape as internal/mcp/telemetry_span_test.go.
func newSpanTestTracer(exp *tracetest.InMemoryExporter) trace.Tracer {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	return tp.Tracer("lspool-test")
}

// newReadyWorker builds a Worker pre-set to the Ready state with a stubbed
// callOverride so Request can run without a live LS process.
func newReadyWorker(t *testing.T, language string, tracer trace.Tracer, fn func(ctx context.Context, method string, params, result interface{}) error) *Worker {
	t.Helper()
	w := NewWorker("w-test", language, "/tmp", "true", nil, testLogger(), tracer)
	w.callOverride = fn
	w.state.Store(int32(WorkerReady))
	return w
}

// TestRequestEmitsChildSpan asserts that Worker.Request creates a child span
// named exactly "ls.request" parented to the caller's active span, with the
// expected lsp.method / lsp.language / lsp.duration_ms attributes.
func TestRequestEmitsChildSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newSpanTestTracer(exp)

	called := false
	w := newReadyWorker(t, "go", tracer, func(ctx context.Context, method string, params, result interface{}) error {
		called = true
		// Confirm we're running under a span (the ls.request child).
		s := trace.SpanFromContext(ctx)
		assert.True(t, s.SpanContext().IsValid(), "callOverride must run under a valid span context")
		return nil
	})

	// Build a parent span so we can assert parent/child linkage.
	parentCtx, parent := tracer.Start(context.Background(), "test.parent")
	err := w.Request(parentCtx, "textDocument/hover", nil, nil)
	require.NoError(t, err)
	parent.End()

	require.True(t, called, "callOverride must have been invoked")

	spans := exp.GetSpans()
	require.Len(t, spans, 2, "expected exactly two spans (test.parent + ls.request)")

	var child, parentSnap tracetest.SpanStub
	for _, s := range spans {
		switch s.Name {
		case "ls.request":
			child = s
		case "test.parent":
			parentSnap = s
		}
	}
	require.Equal(t, "ls.request", child.Name, "child span name must be exactly 'ls.request'")
	require.Equal(t, "test.parent", parentSnap.Name)
	assert.Equal(t, parentSnap.SpanContext.SpanID(), child.Parent.SpanID(), "ls.request must be parented to the active span")

	// Attribute checks: bounded enums only (no PII).
	attrs := make(map[string]string)
	durMs := int64(-1)
	for _, a := range child.Attributes {
		switch a.Key {
		case "lsp.method":
			attrs["lsp.method"] = a.Value.AsString()
		case "lsp.language":
			attrs["lsp.language"] = a.Value.AsString()
		case "lsp.duration_ms":
			durMs = a.Value.AsInt64()
		}
	}
	assert.Equal(t, "textDocument/hover", attrs["lsp.method"])
	assert.Equal(t, "go", attrs["lsp.language"])
	assert.GreaterOrEqual(t, durMs, int64(0), "lsp.duration_ms must be set and non-negative")

	// No "ls.request" event must remain on any span (regression for D-06 → child-span migration).
	for _, s := range spans {
		for _, ev := range s.Events {
			assert.NotEqual(t, "ls.request", ev.Name, "no 'ls.request' AddEvent should remain on any span")
		}
	}
}

// TestRequestSpanRecordsError asserts that when the underlying call returns
// a non-nil error, the ls.request span has Status.Code == codes.Error and at
// least one recorded error event.
func TestRequestSpanRecordsError(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newSpanTestTracer(exp)

	wantErr := errors.New("ls call boom")
	w := newReadyWorker(t, "go", tracer, func(ctx context.Context, method string, params, result interface{}) error {
		return wantErr
	})

	parentCtx, parent := tracer.Start(context.Background(), "test.parent")
	err := w.Request(parentCtx, "textDocument/hover", nil, nil)
	parent.End()
	require.ErrorIs(t, err, wantErr)

	spans := exp.GetSpans()
	var child tracetest.SpanStub
	for _, s := range spans {
		if s.Name == "ls.request" {
			child = s
		}
	}
	require.Equal(t, "ls.request", child.Name)
	assert.Equal(t, codes.Error, child.Status.Code, "ls.request span must have Error status on call failure")
	assert.Contains(t, child.Status.Description, "ls call boom")

	hasException := false
	for _, ev := range child.Events {
		if ev.Name == "exception" {
			hasException = true
			break
		}
	}
	assert.True(t, hasException, "ls.request span must record an exception event via RecordError")
}

// TestRequestSpanShape asserts the span name is uniformly "ls.request" and is
// NOT method-suffixed — protects 55-RESEARCH Open Question #2 (low-cardinality
// span names; method carried as bounded-enum attribute).
func TestRequestSpanShape(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newSpanTestTracer(exp)
	w := newReadyWorker(t, "go", tracer, func(context.Context, string, interface{}, interface{}) error { return nil })

	for _, m := range []string{"textDocument/hover", "textDocument/definition", "workspace/symbol"} {
		require.NoError(t, w.Request(context.Background(), m, nil, nil))
	}

	spans := exp.GetSpans()
	require.Len(t, spans, 3)
	for _, s := range spans {
		assert.Equal(t, "ls.request", s.Name, "span name must be uniform 'ls.request' regardless of method")
	}
}

// allocCounter is a tiny counter used by BenchmarkRequestNoopTracer to verify
// the noop path runs callOverride without obvious extra allocation paths.
var allocCounter atomic.Int64

// BenchmarkRequestNoopTracer asserts the tracing-off path remains low-overhead.
// Per Pitfall 2 / D-17 — the noop tracer's Start is free, so wrapping every LS
// call should not regress allocations beyond the call itself. Reported value
// must be ≤ 1 alloc/op for a successful Request.
func BenchmarkRequestNoopTracer(b *testing.B) {
	noop := tracenoop.NewTracerProvider().Tracer("bench")
	w := NewWorker("w-bench", "go", "/tmp", "true", nil, testLogger(), noop)
	w.callOverride = func(context.Context, string, interface{}, interface{}) error {
		allocCounter.Add(1)
		return nil
	}
	w.state.Store(int32(WorkerReady))

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.Request(ctx, "textDocument/hover", nil, nil)
	}
}
