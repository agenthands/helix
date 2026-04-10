package forwarder

import (
	"bufio"
	"context"
	cryptoRand "crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"os"

	serenav1 "github.com/postfix/serena/api/proto/serena/v1"
	"github.com/postfix/serena/internal/obs"
)

// RunForwarder starts the stdio-to-gRPC forwarder (DMN-03).
// It reads JSON-RPC from stdin, sends via gRPC to daemon, and writes responses to stdout.
func RunForwarder(ctx context.Context, socketPath string, logger *slog.Logger) error {
	// Phase 12: forwarder provider -- noop-only for v1.2.
	// The daemon's SDK TracerProvider catches spans via otelgrpc traceparent
	// propagation; the forwarder does NOT need its own OTLP endpoint.
	fwdProvider := obs.Noop(logger.Handler())

	client, conn, err := connectOrStartDaemon(ctx, socketPath, logger, fwdProvider.TracerProvider())
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

	// Goroutine: stdin -> gRPC (send client messages to daemon)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		// MCP messages can be large
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
		for scanner.Scan() {
			msg := &serenav1.MCPMessage{
				Payload:   append([]byte(nil), scanner.Bytes()...),
				SessionId: sessionID,
			}
			if err := stream.Send(msg); err != nil {
				errCh <- fmt.Errorf("sending to daemon: %w", err)
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
