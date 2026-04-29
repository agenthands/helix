package forwarder

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	serenav1 "github.com/postfix/serena/api/proto/serena/v1"
)

// ConnectOrStartDaemon connects to a running daemon or starts one (D-03, gopls pattern).
// The tp parameter provides an explicit TracerProvider for the otelgrpc client handler
// (D-01: no global TracerProvider).
func ConnectOrStartDaemon(ctx context.Context, socketPath string, logger *slog.Logger, tp trace.TracerProvider) (serenav1.ForwarderServiceClient, *grpc.ClientConn, error) {
	// Try connecting to existing daemon
	conn, client, err := tryConnect(ctx, socketPath, tp)
	if err == nil {
		logger.Info("connected to existing daemon", "socket", socketPath)
		return client, conn, nil
	}

	// Daemon not running -- start it
	logger.Info("daemon not running, starting", "socket", socketPath)
	if err := startDaemon(socketPath); err != nil {
		return nil, nil, fmt.Errorf("starting daemon: %w", err)
	}

	// Poll for daemon readiness (up to 10 seconds)
	return waitForDaemon(ctx, socketPath, 10*time.Second, tp)
}

// tryConnect attempts to connect to a daemon at the given socket path.
// The tp parameter provides an explicit TracerProvider for the otelgrpc stats handler.
func tryConnect(_ context.Context, socketPath string, tp trace.TracerProvider) (*grpc.ClientConn, serenav1.ForwarderServiceClient, error) {
	// Check socket exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("socket not found: %s", socketPath)
	}

	// Verify the socket is alive by attempting a dial
	testConn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("socket not responding: %w", err)
	}
	testConn.Close()

	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// Pitfall 1: Configure keepalive to detect dead daemon
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		// Phase 12: otelgrpc client handler for trace propagation (D-01, D-12).
		// WithTracerProvider is MANDATORY — without it otelgrpc falls back to the
		// OTel global, violating the no-global rule. Pitfall 5: only one
		// WithStatsHandler call (gRPC silently overwrites duplicates).
		grpc.WithStatsHandler(otelgrpc.NewClientHandler(
			otelgrpc.WithTracerProvider(tp),
		)),
	)
	if err != nil {
		return nil, nil, err
	}

	client := serenav1.NewForwarderServiceClient(conn)
	return conn, client, nil
}

// startDaemon starts a new daemon process in the background.
func startDaemon(socketPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	cmd := exec.Command(exe, "--serve", "--socket="+socketPath)
	// Detach daemon from forwarder process group (Unix only; no-op on Windows).
	detachFromProcessGroup(cmd)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting daemon process: %w", err)
	}
	// Release -- don't wait for daemon (it's a long-running process)
	return cmd.Process.Release()
}

// waitForDaemon polls for daemon readiness up to the given timeout.
func waitForDaemon(ctx context.Context, socketPath string, timeout time.Duration, tp trace.TracerProvider) (serenav1.ForwarderServiceClient, *grpc.ClientConn, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		// Check if socket file appeared
		if _, err := os.Stat(socketPath); err == nil {
			// Try connecting
			conn, client, err := tryConnect(ctx, socketPath, tp)
			if err == nil {
				return client, conn, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, nil, fmt.Errorf("daemon did not start within %s", timeout)
}
