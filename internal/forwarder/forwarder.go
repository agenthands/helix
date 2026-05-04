package forwarder

import (
	"bufio"
	"bytes"
	"context"
	cryptoRand "crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel/trace"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
	"github.com/agenthands/helix/internal/obs"
)

// RunForwarder starts the stdio-to-gRPC forwarder (DMN-03).
// It reads JSON-RPC from stdin, sends via gRPC to daemon, and writes responses to stdout.
func RunForwarder(ctx context.Context, socketPath string, logger *slog.Logger) error {
	// Phase 58 D-06: real TracerProvider when OTEL_EXPORTER_OTLP_ENDPOINT
	// is set; falls back to Noop otherwise (degraded-optional per
	// internal/obs/tracing.go:1-13 D-09). Construction never panics —
	// WithTracing logs a warning and returns Noop on exporter init failure.
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	fwdProvider := obs.WithTracing(logger.Handler(), obs.TracingConfig{
		Endpoint:    endpoint,
		ServiceName: "helix-forwarder",
		SampleRatio: 1.0,
	}, logger)
	// Phase 58 D-06 / CR-02: SDK TracerProvider needs ShutdownTracing to
	// flush the batch span processor and tear down the OTLP gRPC
	// connection. The defer is registered IMMEDIATELY after construction
	// (before the daemon dial) so a ConnectOrStartDaemon failure does not
	// leak the SDK goroutines + gRPC conn for the lifetime of the
	// forwarder process. No-op on the Noop path (degraded-optional).
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = fwdProvider.ShutdownTracing(shutdownCtx)
	}()

	client, conn, err := ConnectOrStartDaemon(ctx, socketPath, logger, fwdProvider.TracerProvider())
	if err != nil {
		return fmt.Errorf("connecting to daemon: %w", err)
	}
	defer conn.Close()

	stream, err := client.StreamMCP(ctx)
	if err != nil {
		return fmt.Errorf("opening MCP stream: %w", err)
	}

	// Generate a session ID for this forwarder connection
	sessionID := generateSessionID()
	logger.Info("forwarder session started", "session_id", sessionID)

	errCh := make(chan error, 2)

	tracer := fwdProvider.Tracer()

	// Goroutine: stdin -> gRPC (send client messages to daemon)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		// MCP messages can be large
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			payload := append([]byte(nil), scanner.Bytes()...)
			msg := &serenav1.MCPMessage{
				Payload:   payload,
				SessionId: sessionID,
			}

			// Phase 12: create root span for tools/call only.
			// With noop tracer (default) this is a no-op function call.
			// Non-tools/call methods skip span creation (hot path untouched).
			var sendErr error
			if isToolsCall(payload) {
				sendErr = sendWithSpan(ctx, tracer, stream, msg)
			} else {
				sendErr = stream.Send(msg)
			}
			if sendErr != nil {
				errCh <- fmt.Errorf("sending to daemon: %w", sendErr)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("reading stdin: %w", err)
			return
		}
		// stdin closed -- close send direction
		if err := stream.CloseSend(); err != nil {
			errCh <- fmt.Errorf("closing send: %w", err)
			return
		}
		errCh <- io.EOF
	}()

	// Goroutine: gRPC -> stdout (send daemon responses to client)
	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				errCh <- io.EOF
				return
			}
			if err != nil {
				errCh <- fmt.Errorf("receiving from daemon: %w", err)
				return
			}
			// Write response to stdout followed by newline
			os.Stdout.Write(msg.Payload)
			os.Stdout.Write([]byte("\n"))
		}
	}()

	// Wait for either goroutine to finish
	err = <-errCh
	if err == io.EOF {
		return nil
	}
	return err
}

// generateSessionID creates a unique session identifier using crypto/rand.
func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = io.ReadFull(cryptoRand.Reader, b)
	return fmt.Sprintf("%x", b)
}

// toolsCallMethod is the JSON-RPC method substring used to detect tools/call messages.
// Checking for the byte pattern avoids full JSON parsing on the hot path.
var toolsCallMethod = []byte(`"method":"tools/call"`)

// toolsCallMethodSpaced matches the variant with spaces around the colon.
var toolsCallMethodSpaced = []byte(`"method": "tools/call"`)

// isToolsCall returns true if the JSON-RPC payload is a tools/call request.
// Uses substring matching rather than full JSON parsing to stay allocation-free
// on the forwarding hot path.
func isToolsCall(payload []byte) bool {
	return bytes.Contains(payload, toolsCallMethod) ||
		bytes.Contains(payload, toolsCallMethodSpaced)
}

// sendWithSpan wraps a gRPC Send in a forwarder.tools.call root span.
// The span is the first node in the 3-span tree:
//
//	forwarder.tools.call -> daemon.mcp.tools.call -> kernel.tool.{name}
//
// With a noop tracer (v1.2 default), Start returns a non-recording span and
// End is a no-op — overhead is bounded to one function call per tools/call.
func sendWithSpan(ctx context.Context, tracer trace.Tracer, stream serenav1.ForwarderService_StreamMCPClient, msg *serenav1.MCPMessage) error {
	_, span := tracer.Start(ctx, "forwarder.tools.call")
	err := stream.Send(msg)
	span.End()
	return err
}
