package forwarder

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
)

// fakeClientStream is an in-memory GRPCClientStream for transport tests. It
// records every payload Sent by the client transport and serves a queue of
// canned MCPMessages on Recv (returning recvErr once the queue is drained).
type fakeClientStream struct {
	mu sync.Mutex

	// recv queue: messages handed to the SDK client reader.
	recvQueue []*serenav1.MCPMessage
	recvErr   error
	recvIdx   int
	recvGate  chan struct{} // closed to release a blocking Recv after the queue drains

	// sent records every MCPMessage the transport pushed toward the daemon.
	sent []*serenav1.MCPMessage
}

func newFakeClientStream(recv []*serenav1.MCPMessage, recvErr error) *fakeClientStream {
	return &fakeClientStream{
		recvQueue: recv,
		recvErr:   recvErr,
		recvGate:  make(chan struct{}),
	}
}

func (f *fakeClientStream) Recv() (*serenav1.MCPMessage, error) {
	f.mu.Lock()
	if f.recvIdx < len(f.recvQueue) {
		msg := f.recvQueue[f.recvIdx]
		f.recvIdx++
		f.mu.Unlock()
		return msg, nil
	}
	f.mu.Unlock()

	// Queue drained: if a terminal error is configured, return it; otherwise
	// block until the test releases the gate (mimics a stream with no more
	// traffic) and then return EOF.
	if f.recvErr != nil {
		return nil, f.recvErr
	}
	<-f.recvGate
	return nil, io.EOF
}

func (f *fakeClientStream) Send(msg *serenav1.MCPMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Defensive copy of the payload (the transport reuses its read buffer).
	cp := append([]byte(nil), msg.Payload...)
	f.sent = append(f.sent, &serenav1.MCPMessage{Payload: cp, SessionId: msg.SessionId})
	return nil
}

func (f *fakeClientStream) sentMessages() []*serenav1.MCPMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*serenav1.MCPMessage(nil), f.sent...)
}

// encodeReq is a small helper that marshals a JSON-RPC request to its wire
// bytes so tests can push realistic payloads through the stream.
func encodeReq(t *testing.T, id int64, method string) []byte {
	t.Helper()
	rpcID, err := jsonrpc.MakeID(id)
	if err != nil {
		t.Fatalf("MakeID: %v", err)
	}
	msg, err := jsonrpc.EncodeMessage(&jsonrpc.Request{ID: rpcID, Method: method})
	if err != nil {
		t.Fatalf("EncodeMessage: %v", err)
	}
	return msg
}

// Test 1: round-trip framing. A JSON-RPC message written by the SDK client is
// Sent as exactly one MCPMessage payload with NO trailing newline, and a
// payload Recv'd from the stream is delivered to the SDK client reader intact.
func TestClientTransport_RoundTripFraming(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	recvPayload := encodeReq(t, 1, "ping")
	stream := newFakeClientStream([]*serenav1.MCPMessage{{Payload: recvPayload}}, nil)

	tr := NewGRPCClientTransport(stream, "sess-1")
	conn, err := tr.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	// Recv direction: the queued payload should surface via the Connection Read.
	gotMsg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	gotBytes, err := jsonrpc.EncodeMessage(gotMsg)
	if err != nil {
		t.Fatalf("re-encode read message: %v", err)
	}
	if string(gotBytes) != string(recvPayload) {
		t.Fatalf("Read payload mismatch:\n got: %s\nwant: %s", gotBytes, recvPayload)
	}

	// Send direction: writing a message should produce exactly one MCPMessage,
	// with the configured sessionID and no embedded newline.
	outID, _ := jsonrpc.MakeID(int64(2))
	if err := conn.Write(ctx, &jsonrpc.Request{ID: outID, Method: "pong"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	var sent []*serenav1.MCPMessage
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sent = stream.sentMessages()
		if len(sent) >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(sent) != 1 {
		t.Fatalf("expected exactly 1 sent MCPMessage, got %d", len(sent))
	}
	if sent[0].SessionId != "sess-1" {
		t.Fatalf("sent SessionId = %q, want sess-1", sent[0].SessionId)
	}
	for _, b := range sent[0].Payload {
		if b == '\n' {
			t.Fatalf("sent payload contains an embedded newline: %q", sent[0].Payload)
		}
	}
}

// Test 2: multi-line drain. Two JSON-RPC messages written back-to-back are Sent
// as two separate MCPMessages split on '\n' (mirrors the server-side drain).
func TestClientTransport_MultiLineDrain(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream := newFakeClientStream(nil, nil)
	tr := NewGRPCClientTransport(stream, "sess-2")
	conn, err := tr.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	id1, _ := jsonrpc.MakeID(int64(10))
	id2, _ := jsonrpc.MakeID(int64(11))
	if err := conn.Write(ctx, &jsonrpc.Request{ID: id1, Method: "a"}); err != nil {
		t.Fatalf("Write 1: %v", err)
	}
	if err := conn.Write(ctx, &jsonrpc.Request{ID: id2, Method: "b"}); err != nil {
		t.Fatalf("Write 2: %v", err)
	}

	var sent []*serenav1.MCPMessage
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sent = stream.sentMessages()
		if len(sent) >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(sent) != 2 {
		t.Fatalf("expected 2 sent MCPMessages (split on newline), got %d", len(sent))
	}
	for i, m := range sent {
		if _, derr := jsonrpc.DecodeMessage(m.Payload); derr != nil {
			t.Fatalf("sent[%d] is not a decodable JSON-RPC message: %v (payload=%s)", i, derr, m.Payload)
		}
	}
}

// Test 3: no firstMsg replay. The client transport must NOT pre-write any first
// message into the reader before the first stream Recv — the very first message
// the SDK client reads must be the FIRST item the stream serves, not a replayed
// pre-consumed message.
func TestClientTransport_NoFirstMsgReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	first := encodeReq(t, 100, "first")
	stream := newFakeClientStream([]*serenav1.MCPMessage{{Payload: first}}, nil)

	tr := NewGRPCClientTransport(stream, "sess-3")
	conn, err := tr.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	gotMsg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	gotBytes, err := jsonrpc.EncodeMessage(gotMsg)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if string(gotBytes) != string(first) {
		t.Fatalf("first Read should be the stream's first message (no replay):\n got: %s\nwant: %s", gotBytes, first)
	}

	// A second Read must NOT return another copy of the same message (no replay
	// duplicate); it should block until EOF since the queue is drained.
	stream.mu.Lock()
	close(stream.recvGate)
	stream.mu.Unlock()
	if _, err := conn.Read(ctx); err == nil {
		t.Fatalf("second Read should fail (stream drained), got a message — indicates a replayed/duplicated first message")
	}
}

// Test 4: clean close on stream EOF. When the fake stream's Recv returns a
// terminal error, the reader side is closed so the SDK client sees a clean end
// (the Connection's Read surfaces an error) and no goroutine leaks.
func TestClientTransport_CleanCloseOnStreamEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	streamErr := errors.New("stream closed")
	stream := newFakeClientStream(nil, streamErr)

	tr := NewGRPCClientTransport(stream, "sess-4")
	conn, err := tr.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	// Recv returns the terminal error immediately (empty queue + recvErr), so
	// the reader pipe is closed and Read must surface an error rather than hang.
	done := make(chan error, 1)
	go func() {
		_, rerr := conn.Read(ctx)
		done <- rerr
	}()
	select {
	case rerr := <-done:
		if rerr == nil {
			t.Fatalf("Read should surface an error after stream end, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Read did not return after stream end — reader pipe was not closed (goroutine leak)")
	}
}
