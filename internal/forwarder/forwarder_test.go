package forwarder

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
)

func TestTryConnect_NoSocket(t *testing.T) {
	conn, client, err := tryConnect(context.Background(), "/tmp/helix-test-nonexistent.sock", "", tracenoop.NewTracerProvider())
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "socket not found")
}

func TestStartDaemon_ExecutableLookup(t *testing.T) {
	// Verify we can find the current executable (basic sanity check)
	exe, err := os.Executable()
	require.NoError(t, err)
	assert.NotEmpty(t, exe)
}

func TestWaitForDaemon_Timeout(t *testing.T) {
	ctx := context.Background()
	// Use a nonexistent socket path with a very short timeout
	client, conn, err := waitForDaemon(ctx, "/tmp/helix-test-timeout-"+generateSessionID()+".sock", 200*time.Millisecond, tracenoop.NewTracerProvider())
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "daemon did not start")
}

func TestWaitForDaemon_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	client, conn, err := waitForDaemon(ctx, "/tmp/helix-test-cancel.sock", 5*time.Second, tracenoop.NewTracerProvider())
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Nil(t, conn)
}

func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()
	assert.Len(t, id1, 32) // 16 bytes = 32 hex chars
	assert.Len(t, id2, 32)
	assert.NotEqual(t, id1, id2)
}

func TestIsToolsCall(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    bool
	}{
		{
			name:    "tools/call without space",
			payload: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_symbols_overview"}}`,
			want:    true,
		},
		{
			name:    "tools/call with space after colon",
			payload: `{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "get_symbols_overview"}}`,
			want:    true,
		},
		{
			name:    "initialize method",
			payload: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
			want:    false,
		},
		{
			name:    "tools/list method",
			payload: `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
			want:    false,
		},
		{
			name:    "empty payload",
			payload: ``,
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isToolsCall([]byte(tt.payload)))
		})
	}
}

// fakeStream implements just the Send method of ForwarderService_StreamMCPClient
// for testing sendWithSpan.
type fakeStream struct {
	serenav1.ForwarderService_StreamMCPClient
	sent []*serenav1.MCPMessage
}

func (f *fakeStream) Send(msg *serenav1.MCPMessage) error {
	f.sent = append(f.sent, msg)
	return nil
}

func TestForwarderRootSpan(t *testing.T) {
	// Set up an in-memory span exporter with AlwaysSample so we can verify
	// that sendWithSpan actually creates a "forwarder.tools.call" span.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test")
	stream := &fakeStream{}
	msg := &serenav1.MCPMessage{
		Payload:   []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}`),
		SessionId: "test-session",
	}

	err := sendWithSpan(context.Background(), tracer, stream, msg)
	require.NoError(t, err)

	// Force flush
	_ = tp.ForceFlush(context.Background())

	spans := exporter.GetSpans()
	require.Len(t, spans, 1, "expected exactly one span")
	assert.Equal(t, "forwarder.tools.call", spans[0].Name)

	// Verify the message was actually sent through the stream
	require.Len(t, stream.sent, 1)
	assert.Equal(t, msg.Payload, stream.sent[0].Payload)
}

func TestForwarderRootSpan_NoopTracer(t *testing.T) {
	// With the noop tracer, sendWithSpan should still forward the message
	// without error — just no span recorded.
	tracer := tracenoop.NewTracerProvider().Tracer("test")
	stream := &fakeStream{}
	msg := &serenav1.MCPMessage{
		Payload:   []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{}}`),
		SessionId: "test-session",
	}

	err := sendWithSpan(context.Background(), tracer, stream, msg)
	require.NoError(t, err)
	require.Len(t, stream.sent, 1)
}
