package mcp

import (
	"context"
	"errors"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// stubSkillExecutor is a minimal SkillToolExecutor used by the span tests.
type stubSkillExecutor struct {
	result string
	err    error
}

func (s *stubSkillExecutor) ExecuteTool(_ string, _ map[string]interface{}) (string, error) {
	return s.result, s.err
}

func newSkillSpanTracer(exp *tracetest.InMemoryExporter) trace.Tracer {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	return tp.Tracer("mcp-test")
}

// TestAddSkillToolEmitsChildSpan asserts that the wrapped skill handler emits
// a `skill.tool.{name}` child span parented to the caller's active span (in
// production this is `daemon.mcp.tools.call` from TelemetryMiddleware).
func TestAddSkillToolEmitsChildSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newSkillSpanTracer(exp)
	executor := &stubSkillExecutor{result: "ok"}

	handler := wrapSkillToolHandler(tracer, "memory_write", executor)

	parentCtx, parent := tracer.Start(context.Background(), "daemon.mcp.tools.call")
	res, _, err := handler(parentCtx, &mcpsdk.CallToolRequest{}, map[string]any{"key": "v"})
	parent.End()
	require.NoError(t, err)
	require.NotNil(t, res)
	require.False(t, res.IsError, "success path: IsError must be false")

	spans := exp.GetSpans()
	require.Len(t, spans, 2)

	var child, parentSnap tracetest.SpanStub
	for _, s := range spans {
		switch s.Name {
		case "skill.tool.memory_write":
			child = s
		case "daemon.mcp.tools.call":
			parentSnap = s
		}
	}
	require.Equal(t, "skill.tool.memory_write", child.Name, "child span must be named exactly 'skill.tool.{name}'")
	require.Equal(t, "daemon.mcp.tools.call", parentSnap.Name)
	assert.Equal(t, parentSnap.SpanContext.SpanID(), child.Parent.SpanID(), "skill.tool span must be parented to daemon.mcp.tools.call")
}

// TestAddSkillToolRecordsError asserts that on executor failure, the child
// span has Status.Code == codes.Error AND the MCP CallToolResult preserves
// IsError=true (existing public behavior must not regress).
func TestAddSkillToolRecordsError(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newSkillSpanTracer(exp)
	wantErr := errors.New("memory store unavailable")
	executor := &stubSkillExecutor{err: wantErr}

	handler := wrapSkillToolHandler(tracer, "memory_write", executor)

	res, _, err := handler(context.Background(), &mcpsdk.CallToolRequest{}, nil)
	require.NoError(t, err, "transport-level error must remain nil; tool failure surfaces via IsError")
	require.NotNil(t, res)
	assert.True(t, res.IsError, "executor failure must preserve IsError=true on the MCP result")

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	span := spans[0]
	assert.Equal(t, "skill.tool.memory_write", span.Name)
	assert.Equal(t, codes.Error, span.Status.Code, "skill.tool span must record Error status on executor failure")
	assert.Contains(t, span.Status.Description, "memory store unavailable")

	hasException := false
	for _, ev := range span.Events {
		if ev.Name == "exception" {
			hasException = true
			break
		}
	}
	assert.True(t, hasException, "skill.tool span must have an exception event from RecordError")
}

// TestAddSkillToolNoAttributesOnChild asserts D-07: the per-tool child span
// MUST carry zero attributes — the parent middleware span already carries
// tool_name, profile, mode, language, outcome.
func TestAddSkillToolNoAttributesOnChild(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tracer := newSkillSpanTracer(exp)
	executor := &stubSkillExecutor{result: "ok"}

	handler := wrapSkillToolHandler(tracer, "memory_read", executor)
	_, _, err := handler(context.Background(), &mcpsdk.CallToolRequest{}, nil)
	require.NoError(t, err)

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	span := spans[0]
	assert.Equal(t, "skill.tool.memory_read", span.Name)
	assert.Empty(t, span.Attributes, "skill.tool child span must carry zero attributes (D-07 — parent has them)")
}

// TestSetTracer_NilSafe asserts the SetTracer setter accepts nil and falls
// back to a noop tracer (the constructor invariant must survive).
func TestSetTracer_NilSafe(t *testing.T) {
	// Reuse one of the existing constructors to get a server.
	srv := &SerenaMCPServer{}
	srv.SetTracer(nil)
	require.NotNil(t, srv.Tracer(), "SetTracer(nil) must fall back to a non-nil noop tracer")
}
