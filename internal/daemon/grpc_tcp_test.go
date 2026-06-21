// Unit and lifecycle tests for the gated gRPC TCP listener (Phase 94 RETIRE-04).
//
// validateGRPCAddr is a line-for-line analog of validateAdminAddr
// (telemetry.go:108) — loopback-only, non-loopback refused with an error that
// points at REMOTE-01 (not the v1.3 admin roadmap). listenGRPCTCP mirrors
// listenAdmin's empty-addr no-op + validate gate, but serves the SAME
// ForwarderService over tcp using the listenSocket gRPC lifecycle
// (RegisterForwarderServiceServer + GracefulStop on ctx.Done).
package daemon

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/obs"
)

func TestValidateGRPCAddr(t *testing.T) {
	cases := []struct {
		name    string
		addr    string
		wantErr bool
		errHas  string
	}{
		{name: "empty", addr: "", wantErr: false}, // empty = disabled, valid
		{name: "loopback_v4", addr: "127.0.0.1:9099", wantErr: false},
		{name: "localhost", addr: "localhost:9099", wantErr: false},
		{name: "loopback_v6", addr: "[::1]:9099", wantErr: false},
		{name: "auto_port", addr: "127.0.0.1:0", wantErr: false},
		{name: "zero_bind", addr: "0.0.0.0:9099", wantErr: true, errHas: "REMOTE-01"},
		{name: "lan_bind", addr: "192.168.1.5:9099", wantErr: true, errHas: "REMOTE-01"},
		{name: "dns_host", addr: "example.com:9099", wantErr: true, errHas: "REMOTE-01"},
		{name: "garbage", addr: "garbage", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGRPCAddr(tc.addr)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.addr)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.addr, err)
			}
			if tc.errHas != "" && err != nil && !strings.Contains(err.Error(), tc.errHas) {
				t.Fatalf("error for %q missing %q: %v", tc.addr, tc.errHas, err)
			}
		})
	}
}

// grpcTCPDaemon constructs a Daemon with only the fields the gRPC TCP listener
// needs. The testServeSession seam stubs the MCP-runtime portion of StreamMCP so
// the lifecycle test can drive a real tcp round-trip without spinning up a full
// MCP server (mirrors forwarder_test.go's newTestHandler seam usage).
func grpcTCPDaemon(t *testing.T, addr string, serveSession sessionRunner) *Daemon {
	t.Helper()
	return &Daemon{
		config: &config.SerenaConfig{Daemon: config.DaemonConfig{GRPCAddr: addr}},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		obs:    obs.Noop(nil),
		// mcpServer/kernel are nil — not exercised when testServeSession is set.
		testServeSession: serveSession,
	}
}

func TestListenGRPCTCP_Disabled(t *testing.T) {
	d := grpcTCPDaemon(t, "", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := d.listenGRPCTCP(ctx); err != nil {
		t.Fatalf("listenGRPCTCP disabled should return nil, got %v", err)
	}
	if grpcTCPListenerAddr.Load() != nil {
		t.Fatal("grpcTCPListenerAddr should be nil when listener disabled")
	}
}

func TestListenGRPCTCP_NonLoopbackRejected(t *testing.T) {
	d := grpcTCPDaemon(t, "0.0.0.0:0", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err := d.listenGRPCTCP(ctx)
	if err == nil {
		t.Fatal("expected error for non-loopback addr, got nil")
	}
	if !strings.Contains(err.Error(), "REMOTE-01") {
		t.Fatalf("error missing REMOTE-01: %v", err)
	}
}

// waitForGRPCTCPAddr polls the grpcTCPListenerAddr test hook until set or timeout.
func waitForGRPCTCPAddr(t *testing.T, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if p := grpcTCPListenerAddr.Load(); p != nil {
			return *p
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("grpc tcp listener did not publish address within %s", timeout)
	return ""
}

func TestListenGRPCTCP_LoopbackLifecycle(t *testing.T) {
	// serveSession returns nil → a clean StreamMCP round-trip. This proves the
	// TCP listener serves the SAME ForwarderService over tcp the unix path does.
	served := make(chan struct{}, 1)
	runner := func(ctx context.Context, stream serenav1.ForwarderService_StreamMCPServer, firstMsg *serenav1.MCPMessage) error {
		// Echo the first message back so the client observes a round-trip, then
		// return nil for the (ended, stdio) clean path.
		_ = stream.Send(&serenav1.MCPMessage{SessionId: firstMsg.SessionId, Payload: firstMsg.Payload})
		select {
		case served <- struct{}{}:
		default:
		}
		return nil
	}

	d := grpcTCPDaemon(t, "127.0.0.1:0", runner)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- d.listenGRPCTCP(ctx) }()

	addr := waitForGRPCTCPAddr(t, 3*time.Second)

	// Dial the bound tcp address with a gRPC client and drive one StreamMCP
	// round-trip, asserting a non-error result.
	// passthrough:/// dials the tcp address verbatim (the gRPC analog of the
	// unix:// form) — "tcp://" is NOT a valid gRPC target scheme.
	conn, err := grpc.NewClient("passthrough:///"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient tcp://%s: %v", addr, err)
	}
	defer conn.Close()

	client := serenav1.NewForwarderServiceClient(conn)
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer dialCancel()
	stream, err := client.StreamMCP(dialCtx)
	if err != nil {
		t.Fatalf("StreamMCP over tcp: %v", err)
	}
	if err := stream.Send(&serenav1.MCPMessage{SessionId: "tcp-lifecycle", Payload: []byte("{}")}); err != nil {
		t.Fatalf("send over tcp stream: %v", err)
	}
	if _, err := stream.Recv(); err != nil && err != io.EOF {
		t.Fatalf("recv over tcp stream: %v", err)
	}

	select {
	case <-served:
	case <-time.After(3 * time.Second):
		t.Fatal("server-side StreamMCP runner was never invoked over tcp")
	}

	// Cancel ctx and confirm graceful stop + hook cleared.
	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Logf("listenGRPCTCP returned: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("listenGRPCTCP did not exit within 5s of ctx cancel")
	}
	if grpcTCPListenerAddr.Load() != nil {
		t.Fatal("grpcTCPListenerAddr should be nil after exit")
	}
}
