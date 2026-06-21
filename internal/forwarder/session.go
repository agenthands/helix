package forwarder

import (
	"context"
	"fmt"
	"log/slog"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace/noop"
)

// Session is a long-lived MCP client session over the retained gRPC StreamMCP
// wire. It is the multi-call analog of the one-shot CallTool (oneshot.go): where
// CallTool opens a stream, performs the initialize handshake, issues a SINGLE
// tools/call, and tears the stream down, Session keeps ONE stream + ONE MCP SDK
// session open so a caller can issue a SEQUENCE of tools/call operations over it
// — exactly the shape the internal bench/eval harnesses need now that the
// agent-facing stdio forwarder head (`helix --mode=stdio`) is deleted (Phase 94).
//
// Like CallTool it rides the EXISTING gRPC StreamMCP bidirectional wire (no proto
// change) by wrapping the stream in GRPCClientTransport and driving it with the
// MCP SDK client, which performs the standard initialize handshake the daemon's
// SDK server requires. Hand-framing JSON-RPC over stream.Send is deliberately NOT
// used — it skips the handshake and the daemon rejects it.
//
// Session is an INTERNAL test/bench driver over the retained wire. It does NOT
// reintroduce any stdio MCP server path, any --mode=stdio route, or any
// agent-facing head: it is a Go API the harnesses call in-process, dialing the
// same unix socket the daemon already listens on (SC2: no stdio MCP server code
// path remains reachable).
//
// Concurrency: Session is single-consumer, mirroring the in-process drive loop it
// replaces (the bench respDispatcher is single-consumer too). Each CallTool waits
// for its own response before returning; callers issue calls sequentially.
type Session struct {
	conn    interface{ Close() error }
	stream  interface{ CloseSend() error }
	session *mcpsdk.ClientSession
}

// OpenSession dials the warm daemon (or cold-starts one, exactly as CallTool
// does) and establishes a single MCP SDK session over a fresh gRPC StreamMCP
// stream. The caller MUST Close() the returned Session to flush the SDK shutdown
// and half-close the stream so the daemon records the session with
// outcome="ended" rather than the non-EOF RST abort it would otherwise see (the
// same ordered-teardown invariant CallTool documents at oneshot.go:69-81).
//
// tcpAddr mirrors CallTool: empty selects the unix-socket default (with
// auto-start); non-empty dials the operator-managed loopback gRPC TCP endpoint
// (no auto-start). version is the client implementation version stamped onto the
// initialize handshake (analogous to CallTool's version param).
func OpenSession(
	ctx context.Context,
	socketPath string,
	tcpAddr string,
	logger *slog.Logger,
	version string,
) (*Session, error) {
	// Race-safe dial (90-01): warm-reuse fast path, or a single guarded cold
	// auto-start. A harness driver needs no OTel, so pass a noop TracerProvider —
	// identical to CallTool.
	client, conn, err := ConnectOrStartDaemon(ctx, socketPath, tcpAddr, logger, noop.NewTracerProvider())
	if err != nil {
		return nil, fmt.Errorf("connecting to daemon: %w", err)
	}

	stream, err := client.StreamMCP(ctx)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("opening MCP stream: %w", err)
	}

	sessionID := generateSessionID()
	transport := NewGRPCClientTransport(stream, sessionID)

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "helix-harness",
		Version: version,
	}, nil)

	// Connect performs the initialize handshake the daemon's SDK server requires
	// (the same handshake CallTool relies on). After this returns the session is
	// ready for a sequence of CallTool operations.
	mcpSession, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		_ = stream.CloseSend()
		_ = conn.Close()
		return nil, fmt.Errorf("connecting MCP session: %w", err)
	}

	return &Session{
		conn:    conn,
		stream:  stream,
		session: mcpSession,
	}, nil
}

// CallTool issues one tools/call over the open session and returns the SDK
// result. Unlike forwarder.CallTool it does NOT open or tear down a stream — it
// reuses the single session, so a sequence of calls shares one daemon-side
// session (one initialize, one session-isolation key). A transport-level failure
// is returned as the error; a tool-level error surfaces as res.IsError on a
// non-nil result.
func (s *Session) CallTool(ctx context.Context, name string, args map[string]any) (*mcpsdk.CallToolResult, error) {
	res, err := s.session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("calling tool %q: %w", name, err)
	}
	return res, nil
}

// Close tears the session down in the same order CallTool uses (oneshot.go:69-81):
// session.Close() first so the SDK flushes its shutdown over the stream, THEN
// stream.CloseSend() so the daemon's stream.Recv() observes a clean io.EOF and
// records the session with outcome="ended" — NOT the non-EOF RST abort it would
// see from conn.Close() alone, which would inflate
// helix_session_lifecycle{outcome="error"}. conn.Close() runs LAST.
//
// Under gRPC the deleted stdio head's delicate "do not close stdin until all
// responses are read" race (drive.go:47-51, integration_test:139-143) is moot:
// there is no stdin pipe to EOF. The equivalent invariant — half-close the SEND
// direction only AFTER every call has completed — is satisfied structurally
// because Close() runs after the caller's last CallTool returns, and CloseSend
// here only half-closes send (the recv direction the SDK already drained on
// session.Close()).
func (s *Session) Close() {
	if s.session != nil {
		s.session.Close()
	}
	if s.stream != nil {
		// Best-effort: a CloseSend error on an already-aborted stream is not
		// actionable here (mirrors oneshot.go:80).
		_ = s.stream.CloseSend()
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
}
