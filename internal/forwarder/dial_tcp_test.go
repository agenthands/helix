//go:build !windows
// +build !windows

package forwarder

// Phase 94 RETIRE-04: the CLI dial path can target a loopback gRPC TCP endpoint
// instead of the unix socket. This test proves tryConnect's tcp branch dials a
// gRPC ForwarderService over tcp and round-trips a StreamMCP message identically
// to the unix path, and that the unix default is unchanged when no TCP endpoint
// is configured.
//
// It stands up a minimal in-package gRPC server (a fake ForwarderService whose
// StreamMCP echoes the first message) bound on BOTH a unix socket and a loopback
// tcp listener, so the test exercises the real grpc.NewClient target selection
// without importing internal/daemon (which would create an import cycle).

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
)

// echoForwarderServer is a minimal ForwarderService whose StreamMCP echoes back
// the first message it receives, then returns. It is enough to prove the dial
// path round-trips over the chosen network.
type echoForwarderServer struct {
	serenav1.UnimplementedForwarderServiceServer
}

func (s *echoForwarderServer) StreamMCP(stream serenav1.ForwarderService_StreamMCPServer) error {
	msg, err := stream.Recv()
	if err != nil {
		return err
	}
	return stream.Send(&serenav1.MCPMessage{SessionId: msg.SessionId, Payload: msg.Payload})
}

// serveEcho registers the echo server on ln and serves until the test ends.
func serveEcho(t *testing.T, ln net.Listener) *grpc.Server {
	t.Helper()
	srv := grpc.NewServer()
	serenav1.RegisterForwarderServiceServer(srv, &echoForwarderServer{})
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(srv.GracefulStop)
	return srv
}

// roundTrip dials via tryConnect with the given (socketPath, tcpAddr) and drives
// one StreamMCP echo, returning the echoed payload.
func roundTrip(t *testing.T, socketPath, tcpAddr string) []byte {
	t.Helper()
	conn, client, err := tryConnect(context.Background(), socketPath, tcpAddr, noop.NewTracerProvider())
	if err != nil {
		t.Fatalf("tryConnect(socket=%q tcp=%q): %v", socketPath, tcpAddr, err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stream, err := client.StreamMCP(ctx)
	if err != nil {
		t.Fatalf("StreamMCP: %v", err)
	}
	if err := stream.Send(&serenav1.MCPMessage{SessionId: "tcp-parity", Payload: []byte(`{"ping":1}`)}); err != nil {
		t.Fatalf("send: %v", err)
	}
	resp, err := stream.Recv()
	if err != nil && err != io.EOF {
		t.Fatalf("recv: %v", err)
	}
	if resp == nil {
		t.Fatal("nil response from echo server")
	}
	return resp.Payload
}

func TestTryConnect_TCPRoundTripMatchesUnix(t *testing.T) {
	// Unix listener.
	dir, err := os.MkdirTemp("/tmp", "helix-dialtcp-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath := filepath.Join(dir, "d.sock")
	unixLn, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	serveEcho(t, unixLn)

	// Loopback tcp listener.
	tcpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	serveEcho(t, tcpLn)
	tcpAddr := tcpLn.Addr().String()

	// Unix dial (default: tcpAddr empty).
	unixPayload := roundTrip(t, socketPath, "")
	// TCP dial (tcpAddr set — unix probe skipped).
	tcpPayload := roundTrip(t, socketPath, tcpAddr)

	if string(unixPayload) != string(tcpPayload) {
		t.Fatalf("tcp payload %q != unix payload %q", tcpPayload, unixPayload)
	}
	if string(tcpPayload) != `{"ping":1}` {
		t.Fatalf("unexpected echo payload: %q", tcpPayload)
	}
}

func TestTryConnect_TCPSkipsUnixProbe(t *testing.T) {
	// With a TCP endpoint set, tryConnect must NOT require the unix socket file
	// to exist (the unix-file liveness probe is skipped). Point socketPath at a
	// nonexistent path and confirm the tcp dial still succeeds.
	tcpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	serveEcho(t, tcpLn)

	got := roundTrip(t, "/nonexistent/never.sock", tcpLn.Addr().String())
	if string(got) != `{"ping":1}` {
		t.Fatalf("tcp dial with absent socket failed; payload=%q", got)
	}
}
