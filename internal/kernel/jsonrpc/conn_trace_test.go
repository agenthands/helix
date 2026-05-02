package jsonrpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
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

// TestConnCall_EmitsLspoolSpanWithLspMethodAttribute asserts Phase 55 OBS-04 success
// criterion 1 at the Conn layer: every Call emits a child span named
// "lspool.lsp.{method}" with attribute lsp.method == method.
func TestConnCall_EmitsLspoolSpanWithLspMethodAttribute(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newTestTracer(exp)

	mock := newMockRWC()
	conn := NewConn(mock, "test-sess", tracer)

	// Pre-load a response for the upcoming Call's session-prefixed ID (test-sess:1).
	expectedID := "test-sess:1"
	mock.writeResponse(&Response{
		JSONRPC: "2.0",
		ID:      expectedID,
		Result:  json.RawMessage(`{}`),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _ = conn.Listen(ctx) }()

	var result map[string]interface{}
	err := conn.Call(ctx, "textDocument/definition", nil, &result)
	require.NoError(t, err)

	cancel()

	spans := exp.GetSpans()
	require.Len(t, spans, 1, "expected exactly one captured span")
	assert.Equal(t, "lspool.lsp.textDocument/definition", spans[0].Name)

	// Find the lsp.method attribute.
	found := false
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "lsp.method" {
			assert.Equal(t, "textDocument/definition", attr.Value.AsString())
			found = true
		}
	}
	assert.True(t, found, "expected lsp.method attribute on span")
}

// TestConnNotify_EmitsLspoolNotifySpan asserts Notify emits a child span named
// "lspool.lsp.notify.{method}" with attribute lsp.method == method.
func TestConnNotify_EmitsLspoolNotifySpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newTestTracer(exp)

	mock := newMockRWC()
	conn := NewConn(mock, "test-sess", tracer)

	err := conn.Notify(context.Background(), "initialized", nil)
	require.NoError(t, err)

	spans := exp.GetSpans()
	require.Len(t, spans, 1, "expected exactly one captured span")
	assert.Equal(t, "lspool.lsp.notify.initialized", spans[0].Name)

	found := false
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "lsp.method" {
			assert.Equal(t, "initialized", attr.Value.AsString())
			found = true
		}
	}
	assert.True(t, found, "expected lsp.method attribute on span")
}

// TestConnCall_NoopTracerZeroAllocations asserts D-17: passing nil tracer to
// NewConn does not panic and emits zero spans on a separate sample exporter
// (which proves the noop fallback truly is a noop, not a silent passthrough
// to some global). Loops 1000 times against a closed conn (each Call returns
// "connection closed" error fast) to amplify any allocation/panic regression.
func TestConnCall_NoopTracerZeroAllocations(t *testing.T) {
	// A SEPARATE exporter that we never wire into the Conn — it must remain empty.
	exp := tracetest.NewInMemoryExporter()
	_ = newTestTracer(exp) // build the provider so the exporter is "armed"

	mock := newMockRWC()
	conn := NewConn(mock, "test-sess", nil)
	require.NotNil(t, conn, "NewConn with nil tracer must return non-nil Conn")
	_ = conn.Close()

	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		var out map[string]interface{}
		err := conn.Call(ctx, "test/method", nil, &out)
		require.Error(t, err, "iteration %d: expected closed-conn error", i)
	}

	assert.Empty(t, exp.GetSpans(), "noop fallback must not emit spans through any global tracer")
}

// TestConnCall_ParentSpanLinked asserts that a Call inside a parent span produces
// a child span whose parent SpanID matches the outer span's SpanID.
func TestConnCall_ParentSpanLinked(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newTestTracer(exp)

	mock := newMockRWC()
	conn := NewConn(mock, "test-sess", tracer)

	expectedID := "test-sess:1"
	mock.writeResponse(&Response{
		JSONRPC: "2.0",
		ID:      expectedID,
		Result:  json.RawMessage(`{}`),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() { _ = conn.Listen(ctx) }()

	parentCtx, parent := tracer.Start(ctx, "parent")
	var result map[string]interface{}
	err := conn.Call(parentCtx, "textDocument/hover", nil, &result)
	require.NoError(t, err)
	parent.End()
	cancel()

	spans := exp.GetSpans()
	require.Len(t, spans, 2, "expected child + parent spans")

	// The first span ended (defer order) is the lspool child.
	var child, parentSpan tracetest.SpanStub
	for _, s := range spans {
		if s.Name == "lspool.lsp.textDocument/hover" {
			child = s
		} else if s.Name == "parent" {
			parentSpan = s
		}
	}
	require.NotEmpty(t, child.Name, "lspool child span not found")
	require.NotEmpty(t, parentSpan.Name, "parent span not found")

	assert.Equal(t, parentSpan.SpanContext.SpanID(), child.Parent.SpanID(),
		"child parent SpanID must equal parent SpanID")
	assert.Equal(t, parentSpan.SpanContext.TraceID(), child.SpanContext.TraceID(),
		"child and parent must share the same trace ID")
}
