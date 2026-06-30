// Phase 53 D-08: forwarder stdio session-lifecycle metric emission.
//
// Verifies that forwarderServiceHandler.StreamMCP emits
// helix_session_lifecycle_total{transport="stdio"} at the documented
// transitions:
//
//   - (started, stdio) on first message received
//   - (ended,   stdio) on clean session.Wait() return
//   - (error,   stdio) on Connect() failure or non-nil session.Wait()
//
// Regression guard: if stream.Recv() fails BEFORE firstMsg is read, NO
// lifecycle event is emitted (no session_id is known yet). This guards
// against count drift from inflated "started" counters without matching
// "ended" / "error".
package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"google.golang.org/grpc/metadata"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
	"github.com/agenthands/helix/internal/obs"
)

// fakeForwarderStream is a minimal serenav1.ForwarderService_StreamMCPServer
// (= grpc.BidiStreamingServer[MCPMessage, MCPMessage]) for testing
// StreamMCP's lifecycle emission. Only Recv() and Context() are exercised
// by the SUT; the rest of the gRPC ServerStream surface is no-op.
type fakeForwarderStream struct {
	mu      sync.Mutex
	ctx     context.Context
	queue   []recvResult
	sent    []*serenav1.MCPMessage
	recvIdx int
}

type recvResult struct {
	msg *serenav1.MCPMessage
	err error
}

func newFakeStream(t *testing.T, results ...recvResult) *fakeForwarderStream {
	t.Helper()
	return &fakeForwarderStream{
		ctx:   context.Background(),
		queue: results,
	}
}

func (f *fakeForwarderStream) Recv() (*serenav1.MCPMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recvIdx >= len(f.queue) {
		return nil, io.EOF
	}
	r := f.queue[f.recvIdx]
	f.recvIdx++
	return r.msg, r.err
}

func (f *fakeForwarderStream) Send(msg *serenav1.MCPMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, msg)
	return nil
}

// grpc.ServerStream embedded methods — all no-ops for tests.
func (f *fakeForwarderStream) SetHeader(metadata.MD) error  { return nil }
func (f *fakeForwarderStream) SendHeader(metadata.MD) error { return nil }
func (f *fakeForwarderStream) SetTrailer(metadata.MD)       {}
func (f *fakeForwarderStream) Context() context.Context     { return f.ctx }
func (f *fakeForwarderStream) SendMsg(m any) error          { return nil }
func (f *fakeForwarderStream) RecvMsg(m any) error          { return nil }

// gatherSessionLifecycle returns a map of "phase|transport" → count for the
// helix_session_lifecycle_total family, gathered from the obs.Metrics
// owned registry.
func gatherSessionLifecycle(t *testing.T, m *obs.Metrics) map[string]int {
	t.Helper()
	out := make(map[string]int)
	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != "helix_session_lifecycle_total" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			var phase, transport string
			for _, lp := range metric.GetLabel() {
				switch lp.GetName() {
				case "phase":
					phase = lp.GetValue()
				case "transport":
					transport = lp.GetValue()
				}
			}
			out[phase+"|"+transport] += int(metric.GetCounter().GetValue())
		}
	}
	return out
}

// newTestHandler constructs a forwarderServiceHandler suitable for
// StreamMCP unit tests. The serveSession seam lets the test stub the
// MCP-runtime portion of StreamMCP without spinning up a full server.
func newTestHandler(t *testing.T, metrics *obs.Metrics, serveSession sessionRunner) *forwarderServiceHandler {
	t.Helper()
	return &forwarderServiceHandler{
		mcpServer:    nil, // not exercised when serveSession seam is non-nil
		kernel:       nil,
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		metrics:      metrics,
		serveSession: serveSession,
	}
}

func TestForwarderHandler_SessionLifecycle(t *testing.T) {
	t.Run("started_ended_on_clean_exit", func(t *testing.T) {
		provider := obs.Noop(nil)
		metrics := provider.Metrics()
		// serveSession returns nil → (ended, stdio) path.
		runner := func(ctx context.Context, stream serenav1.ForwarderService_StreamMCPServer, firstMsg *serenav1.MCPMessage) error {
			return nil
		}
		handler := newTestHandler(t, metrics, runner)
		stream := newFakeStream(t, recvResult{msg: &serenav1.MCPMessage{SessionId: "session-clean", Payload: []byte("{}")}})

		err := handler.StreamMCP(stream)
		if err != nil {
			t.Fatalf("StreamMCP: %v", err)
		}
		got := gatherSessionLifecycle(t, metrics)
		if got["started|stdio"] != 1 {
			t.Errorf("started|stdio: got %d, want 1 (counts: %v)", got["started|stdio"], got)
		}
		if got["ended|stdio"] != 1 {
			t.Errorf("ended|stdio: got %d, want 1 (counts: %v)", got["ended|stdio"], got)
		}
		if got["error|stdio"] != 0 {
			t.Errorf("error|stdio: got %d, want 0 (counts: %v)", got["error|stdio"], got)
		}
	})

	t.Run("started_error_on_connect_fail", func(t *testing.T) {
		provider := obs.Noop(nil)
		metrics := provider.Metrics()
		// Simulate Connect() failing inside the serveSession seam by
		// returning a connect error. The handler must classify this as
		// (error, stdio).
		connectErr := errors.New("connect failure")
		runner := func(ctx context.Context, stream serenav1.ForwarderService_StreamMCPServer, firstMsg *serenav1.MCPMessage) error {
			return connectErr
		}
		handler := newTestHandler(t, metrics, runner)
		stream := newFakeStream(t, recvResult{msg: &serenav1.MCPMessage{SessionId: "session-cf", Payload: []byte("{}")}})

		err := handler.StreamMCP(stream)
		if err == nil {
			t.Fatal("StreamMCP: expected error, got nil")
		}
		got := gatherSessionLifecycle(t, metrics)
		if got["started|stdio"] != 1 {
			t.Errorf("started|stdio: got %d, want 1 (counts: %v)", got["started|stdio"], got)
		}
		if got["error|stdio"] != 1 {
			t.Errorf("error|stdio: got %d, want 1 (counts: %v)", got["error|stdio"], got)
		}
		if got["ended|stdio"] != 0 {
			t.Errorf("ended|stdio: got %d, want 0 (counts: %v)", got["ended|stdio"], got)
		}
	})

	t.Run("started_error_on_session_wait_fail", func(t *testing.T) {
		provider := obs.Noop(nil)
		metrics := provider.Metrics()
		// Simulate session.Wait() failing — the seam returns a non-nil
		// error AFTER what would have been a successful Connect.
		waitErr := errors.New("session wait failure")
		runner := func(ctx context.Context, stream serenav1.ForwarderService_StreamMCPServer, firstMsg *serenav1.MCPMessage) error {
			return waitErr
		}
		handler := newTestHandler(t, metrics, runner)
		stream := newFakeStream(t, recvResult{msg: &serenav1.MCPMessage{SessionId: "session-wf", Payload: []byte("{}")}})

		err := handler.StreamMCP(stream)
		if err == nil {
			t.Fatal("StreamMCP: expected error, got nil")
		}
		got := gatherSessionLifecycle(t, metrics)
		if got["started|stdio"] != 1 {
			t.Errorf("started|stdio: got %d, want 1 (counts: %v)", got["started|stdio"], got)
		}
		if got["error|stdio"] != 1 {
			t.Errorf("error|stdio: got %d, want 1 (counts: %v)", got["error|stdio"], got)
		}
		if got["ended|stdio"] != 0 {
			t.Errorf("ended|stdio: got %d, want 0 (counts: %v)", got["ended|stdio"], got)
		}
	})

	t.Run("no_emit_when_recv_fails_before_firstmsg", func(t *testing.T) {
		// Regression guard: if Recv() fails before firstMsg is read,
		// no session_id is known yet — emitting (started, stdio) here
		// would inflate the started counter without a matching
		// (ended, stdio) / (error, stdio).
		provider := obs.Noop(nil)
		metrics := provider.Metrics()
		runnerCalled := false
		runner := func(ctx context.Context, stream serenav1.ForwarderService_StreamMCPServer, firstMsg *serenav1.MCPMessage) error {
			runnerCalled = true
			return nil
		}
		handler := newTestHandler(t, metrics, runner)
		stream := newFakeStream(t, recvResult{err: errors.New("recv failed before firstmsg")})

		err := handler.StreamMCP(stream)
		if err == nil {
			t.Fatal("StreamMCP: expected error, got nil")
		}
		if runnerCalled {
			t.Error("serveSession runner must not be invoked when Recv() fails before firstMsg")
		}
		got := gatherSessionLifecycle(t, metrics)
		if got["started|stdio"] != 0 {
			t.Errorf("started|stdio: got %d, want 0 (no emission expected; counts: %v)", got["started|stdio"], got)
		}
		if got["ended|stdio"] != 0 || got["error|stdio"] != 0 {
			t.Errorf("ended/error must also be 0 (counts: %v)", got)
		}
	})
}
