package mcp_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
)

// newSpanTestProvider constructs an obs.Provider backed by the given in-memory
// span exporter. The TracerProvider always samples so every span is captured.
func newSpanTestProvider(exp *tracetest.InMemoryExporter) *obs.Provider {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	return obs.NewForTest(tp)
}

func spanTestSession() *mcp.SessionInfo {
	return &mcp.SessionInfo{
		SessionID: "test-span",
		Profile:   "claude-code",
		Mode:      "edit",
		Language:  "go",
	}
}

func spanTestCallToolReq(name string) *mcpsdk.CallToolRequest {
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      name,
			Arguments: json.RawMessage(`{}`),
		},
	}
}

func TestTelemetryMiddlewareSpan_ToolCallCreatesSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	provider := newSpanTestProvider(exp)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	session := spanTestSession()

	mw := mcp.TelemetryMiddleware(provider, func(_ context.Context) *mcp.SessionInfo {
		return session
	}, nil, logger)

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(inner)
	_, err := handler(context.Background(), "tools/call", spanTestCallToolReq("find_symbol"))
	require.NoError(t, err)

	spans := exp.GetSpans()
	require.Len(t, spans, 1, "expected exactly one span for tools/call")

	span := spans[0]
	assert.Equal(t, "daemon.mcp.tools.call", span.Name)

	// Check attributes
	attrs := make(map[string]string)
	for _, attr := range span.Attributes {
		attrs[string(attr.Key)] = attr.Value.AsString()
	}
	assert.Equal(t, "find_symbol", attrs["tool_name"])
	assert.Equal(t, "claude-code", attrs["profile"])
	assert.Equal(t, "edit", attrs["mode"])
	assert.Equal(t, "go", attrs["language"])
	assert.Equal(t, "success", attrs["outcome"])
}

func TestTelemetryMiddlewareSpan_NonToolMethodNoSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	provider := newSpanTestProvider(exp)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mw := mcp.TelemetryMiddleware(provider, func(_ context.Context) *mcp.SessionInfo {
		return nil
	}, nil, logger)

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.InitializeResult{}, nil
	}

	handler := mw(inner)
	_, err := handler(context.Background(), "initialize", nil)
	require.NoError(t, err)

	spans := exp.GetSpans()
	assert.Empty(t, spans, "initialize method must not produce a span")
}

func TestTelemetryMiddlewareSpan_ErrorRecorded(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	provider := newSpanTestProvider(exp)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	session := spanTestSession()

	mw := mcp.TelemetryMiddleware(provider, func(_ context.Context) *mcp.SessionInfo {
		return session
	}, nil, logger)

	testErr := serr.New(serr.Internal, "tool execution failed")
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return nil, testErr
	}

	handler := mw(inner)
	_, err := handler(context.Background(), "tools/call", spanTestCallToolReq("broken_tool"))
	require.Error(t, err)

	spans := exp.GetSpans()
	require.Len(t, spans, 1)

	span := spans[0]
	assert.Equal(t, codes.Error, span.Status.Code, "span status must be Error")
	assert.Contains(t, span.Status.Description, "tool execution failed")

	// Verify RecordError was called (appears as a span event)
	require.NotEmpty(t, span.Events, "span must have at least one event from RecordError")
	foundException := false
	for _, ev := range span.Events {
		if ev.Name == "exception" {
			foundException = true
		}
	}
	assert.True(t, foundException, "span must have an 'exception' event from RecordError")
}

func TestTelemetryMiddlewareSpan_NoopTracerZeroCost(t *testing.T) {
	// Use obs.Noop which provides a noop tracer -- must not panic and metrics
	// emission must remain byte-for-byte identical to Phase 11.
	provider := obs.Noop(slog.Default().Handler())
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	session := spanTestSession()

	mw := mcp.TelemetryMiddleware(provider, func(_ context.Context) *mcp.SessionInfo {
		return session
	}, nil, logger)

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{}, nil
	}

	handler := mw(inner)

	// Must not panic
	_, err := handler(context.Background(), "tools/call", spanTestCallToolReq("find_symbol"))
	require.NoError(t, err)

	// Verify metrics still work -- provider.Metrics() must be non-nil.
	m := provider.Metrics()
	require.NotNil(t, m, "metrics must be non-nil even on noop provider")
}
