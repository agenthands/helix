package jsonrpc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadMessage_Valid(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	r := bufio.NewReader(strings.NewReader(input))

	msg, err := ReadMessage(r)
	require.NoError(t, err)
	assert.JSONEq(t, body, string(msg))
}

func TestReadMessage_MultipleHeaders(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1}`
	input := fmt.Sprintf("Content-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
	r := bufio.NewReader(strings.NewReader(input))

	msg, err := ReadMessage(r)
	require.NoError(t, err)
	assert.JSONEq(t, body, string(msg))
}

func TestReadMessage_MissingContentLength(t *testing.T) {
	input := "Content-Type: application/json\r\n\r\n{}"
	r := bufio.NewReader(strings.NewReader(input))

	_, err := ReadMessage(r)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing Content-Length")
}

func TestWriteMessage_Roundtrip(t *testing.T) {
	body := json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":null}`)
	var buf bytes.Buffer

	err := WriteMessage(&buf, body)
	require.NoError(t, err)

	r := bufio.NewReader(&buf)
	msg, err := ReadMessage(r)
	require.NoError(t, err)
	assert.JSONEq(t, string(body), string(msg))
}

func TestRequest_JSON(t *testing.T) {
	req, err := NewRequest("sess-1:1", "initialize", map[string]string{"rootUri": "file:///tmp"})
	require.NoError(t, err)

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded Request
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "2.0", decoded.JSONRPC)
	assert.Equal(t, "sess-1:1", decoded.ID)
	assert.Equal(t, "initialize", decoded.Method)
}

func TestResponse_JSON(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      "sess-1:1",
		Result:  json.RawMessage(`{"capabilities":{}}`),
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded Response
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "2.0", decoded.JSONRPC)
	assert.Nil(t, decoded.Error)
	assert.NotNil(t, decoded.Result)
}

func TestResponse_Error(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      "sess-1:1",
		Error: &ResponseError{
			Code:    -32601,
			Message: "method not found",
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded Response
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.NotNil(t, decoded.Error)
	assert.Equal(t, -32601, decoded.Error.Code)
	assert.Equal(t, "method not found", decoded.Error.Error())
}

func TestNotification_JSON(t *testing.T) {
	notif, err := NewNotification("textDocument/didOpen", map[string]string{"uri": "file:///tmp/test.go"})
	require.NoError(t, err)

	data, err := json.Marshal(notif)
	require.NoError(t, err)

	var decoded Notification
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "2.0", decoded.JSONRPC)
	assert.Equal(t, "textDocument/didOpen", decoded.Method)
}

// mockRWC is a mock ReadWriteCloser for testing Conn.
type mockRWC struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	mu       sync.Mutex
	closed   bool
}

func newMockRWC() *mockRWC {
	return &mockRWC{
		readBuf:  &bytes.Buffer{},
		writeBuf: &bytes.Buffer{},
	}
}

func (m *mockRWC) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.EOF
	}
	return m.readBuf.Read(p)
}

func (m *mockRWC) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.writeBuf.Write(p)
}

func (m *mockRWC) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockRWC) writeResponse(resp *Response) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, _ := json.Marshal(resp)
	fmt.Fprintf(m.readBuf, "Content-Length: %d\r\n\r\n%s", len(data), data)
}

func (m *mockRWC) writeNotification(method string, params interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	notif, _ := NewNotification(method, params)
	data, _ := json.Marshal(notif)
	fmt.Fprintf(m.readBuf, "Content-Length: %d\r\n\r\n%s", len(data), data)
}

func TestConn_Call(t *testing.T) {
	mock := newMockRWC()
	conn := NewConn(mock, "test-sess")

	// Pre-load a response with the expected ID.
	expectedID := "test-sess:1"
	result := json.RawMessage(`{"capabilities":{}}`)
	mock.writeResponse(&Response{
		JSONRPC: "2.0",
		ID:      expectedID,
		Result:  result,
	})

	// Start listener in background.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var listenErr error
	go func() {
		listenErr = conn.Listen(ctx)
	}()

	var caps map[string]interface{}
	err := conn.Call(ctx, "initialize", map[string]string{"rootUri": "file:///tmp"}, &caps)
	require.NoError(t, err)
	assert.NotNil(t, caps)

	cancel()
	// Listener should exit on context cancel.
	_ = listenErr
}

func TestConn_Notify(t *testing.T) {
	mock := newMockRWC()
	conn := NewConn(mock, "test-sess")

	ctx := context.Background()
	err := conn.Notify(ctx, "textDocument/didOpen", map[string]string{"uri": "file:///tmp/test.go"})
	require.NoError(t, err)

	// Verify notification was written.
	written := mock.writeBuf.String()
	assert.Contains(t, written, "Content-Length:")
	assert.Contains(t, written, "textDocument/didOpen")
	// Notification should not have an "id" field.
	assert.NotContains(t, written, `"id"`)
}

func TestConn_Notification_Handler(t *testing.T) {
	mock := newMockRWC()
	conn := NewConn(mock, "test-sess")

	// Pre-load a notification.
	mock.writeNotification("textDocument/publishDiagnostics", map[string]string{"uri": "file:///tmp/test.go"})

	var received string
	var receivedParams json.RawMessage
	conn.OnNotification = func(method string, params json.RawMessage) {
		received = method
		receivedParams = params
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Listen will read the notification then hit EOF.
	_ = conn.Listen(ctx)

	assert.Equal(t, "textDocument/publishDiagnostics", received)
	assert.NotNil(t, receivedParams)
}

// Note: panic recovery for notification handlers is the Worker dispatcher's
// responsibility (lspool.buildDispatcher); see TestBuildDispatcher_PanicRecovers
// in internal/kernel/lspool/worker_test.go. jsonrpc.Conn intentionally does
// not recover (Phase 56 D-05 architectural placement).

// TestConn_NotificationUnknownMethod (Phase 56 LSDISP-04b / D-12) asserts that
// Conn.Listen forwards ALL notifications to OnNotification regardless of method
// name; method allow-listing is the dispatcher's job (Phase 56 D-04 lives in
// lspool.buildDispatcher, not here). This test guards against regressions
// where Listen accidentally adds a method allow-list at the Conn layer.
func TestConn_NotificationUnknownMethod(t *testing.T) {
	mock := newMockRWC()
	conn := NewConn(mock, "test-sess")

	// Pre-load a notification with a method name that no real LSP server
	// would ever emit and that no quirk handler would ever register for.
	mock.writeNotification("totally/unregistered", map[string]int{"x": 1})

	var gotMethod string
	var gotParams json.RawMessage
	conn.OnNotification = func(method string, params json.RawMessage) {
		gotMethod = method
		gotParams = params
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Listen will read the notification then hit EOF.
	_ = conn.Listen(ctx)

	assert.Equal(t, "totally/unregistered", gotMethod)
	require.NotNil(t, gotParams)
	assert.JSONEq(t, `{"x":1}`, string(gotParams))
}

func TestConn_SessionPrefixedIDs(t *testing.T) {
	mock1 := newMockRWC()
	mock2 := newMockRWC()
	conn1 := NewConn(mock1, "sess-abc")
	conn2 := NewConn(mock2, "sess-xyz")

	ctx := context.Background()

	id1, err := conn1.Send(ctx, "test/method", nil)
	require.NoError(t, err)
	id2, err := conn2.Send(ctx, "test/method", nil)
	require.NoError(t, err)

	// IDs should have different session prefixes.
	assert.True(t, strings.HasPrefix(id1, "sess-abc:"))
	assert.True(t, strings.HasPrefix(id2, "sess-xyz:"))
	// IDs should not collide.
	assert.NotEqual(t, id1, id2)
}

func TestConn_ClosedConnection(t *testing.T) {
	mock := newMockRWC()
	conn := NewConn(mock, "test-sess")
	_ = conn.Close()

	ctx := context.Background()
	err := conn.Notify(ctx, "test/method", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection closed")
}
