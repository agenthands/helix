package forwarder

import (
	"context"
	"fmt"
	"log/slog"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace/noop"
)

// CallTool issues a SINGLE MCP tools/call against the warm daemon and returns
// the tool result (CLI-01/CLI-02).
//
// It rides the EXISTING gRPC StreamMCP bidirectional wire — no proto change —
// by wrapping the stream in GRPCClientTransport and driving it with the MCP SDK
// client (which performs the standard initialize handshake the daemon's SDK
// server requires). Hand-framing JSON-RPC over stream.Send is deliberately NOT
// used (RESEARCH Anti-Pattern: it skips the handshake and the daemon rejects it).
//
// version is the CLI binary version (cli.CurrentVersion()); it is passed in as a
// parameter rather than imported because internal/cli imports internal/forwarder,
// so forwarder importing cli would create an import cycle.
//
// Teardown is ordered (RESEARCH Pitfall 3): session.Close() flushes the SDK
// shutdown over the stream, then stream.CloseSend() half-closes the send
// direction so the daemon's stream.Recv() observes a clean io.EOF, then
// conn.Close() (deferred earliest, runs last) drops the gRPC connection.
// Without the explicit CloseSend the daemon would see a non-EOF RST when the
// conn drops and record the session with an `error` outcome, so
// helix_session_lifecycle{outcome="error"} would climb per CLI call (WR-03).
func CallTool(
	ctx context.Context,
	socketPath string,
	logger *slog.Logger,
	version string,
	name string,
	args map[string]any,
) (*mcpsdk.CallToolResult, error) {
	// Race-safe dial (90-01): warm-reuse fast path, or a single guarded cold
	// auto-start. A CLI command needs no OTel, so pass a noop TracerProvider.
	client, conn, err := ConnectOrStartDaemon(ctx, socketPath, logger, noop.NewTracerProvider())
	if err != nil {
		return nil, fmt.Errorf("connecting to daemon: %w", err)
	}
	defer conn.Close()

	stream, err := client.StreamMCP(ctx)
	if err != nil {
		return nil, fmt.Errorf("opening MCP stream: %w", err)
	}

	sessionID := generateSessionID()
	transport := NewGRPCClientTransport(stream, sessionID)

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "helix-cli",
		Version: version,
	}, nil)

	session, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting MCP session: %w", err)
	}
	// Ordered teardown (RESEARCH Pitfall 3): session.Close() first so the SDK
	// flushes its shutdown over the stream, THEN stream.CloseSend() so the
	// daemon's stream.Recv() observes a clean io.EOF and records the session with
	// outcome="ended" — NOT the non-EOF RST abort it would see from conn.Close()
	// alone, which inflates helix_session_lifecycle{outcome="error"} per CLI call
	// (WR-03). This mirrors the long-lived forwarder's CloseSend at
	// forwarder.go:95. conn.Close() (deferred above) still runs LAST.
	defer func() {
		session.Close()
		// Half-close the send direction so the daemon sees clean EOF. Best-effort:
		// a CloseSend error on an already-aborted stream is not actionable here.
		_ = stream.CloseSend()
	}()

	res, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("calling tool %q: %w", name, err)
	}
	return res, nil
}
