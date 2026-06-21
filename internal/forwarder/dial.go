package forwarder

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"time"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/gofrs/flock"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
	"github.com/agenthands/helix/internal/obs"
)

// newFlockLocker builds the production gofrs/flock-backed daemonLocker. It is a
// package-level var so the algorithm tests can substitute an in-process locker
// (real OS flock would durably-block synctest — RESEARCH Pitfall 4). Portable:
// Unix flock + Windows LockFileEx, no CGO (RESEARCH Pitfall 6) — never the
// non-portable raw syscall lock.
var newFlockLocker = func(path string) daemonLocker {
	return &flockLocker{fl: flock.New(path)}
}

// ConnectOrStartDaemon connects to a running daemon or starts one (D-03, gopls pattern).
// The tp parameter provides an explicit TracerProvider for the otelgrpc client handler
// (D-01: no global TracerProvider).
func ConnectOrStartDaemon(ctx context.Context, socketPath string, logger *slog.Logger, tp trace.TracerProvider) (serenav1.ForwarderServiceClient, *grpc.ClientConn, error) {
	// Warm-reuse fast path: try connecting to an existing daemon BEFORE taking the
	// startup lock so the common (daemon-already-up) case is lock-free.
	conn, client, err := tryConnect(ctx, socketPath, tp)
	if err == nil {
		logger.Info("connected to existing daemon", "socket", socketPath)
		return client, conn, nil
	}

	// Cold path: guard the spawn+wait window with a per-socket flock + a
	// double-checked connect (CLI-03). startupGuard ensures that under N parallel
	// cold callers exactly one daemon is spawned; lock losers (and TOCTOU peers)
	// reuse the winner's daemon instead of forking their own. The exported
	// signature is unchanged, so every caller (activate.go, RunForwarder) inherits
	// the fix for free.
	logger.Info("daemon not running, acquiring startup lock", "socket", socketPath)
	if err := startupGuard(ctx, socketPath, seams{
		newLocker: newFlockLocker,
		connect:   func(sp string) error { _, _, e := tryConnect(ctx, sp, tp); return e },
		spawn:     startDaemon,
		waitUp:    func(sp string) error { _, _, e := waitForDaemon(ctx, sp, 10*time.Second, tp); return e },
	}); err != nil {
		return nil, nil, fmt.Errorf("starting daemon: %w", err)
	}

	// Guard returned success: the daemon is up (we spawned it, reused a peer's, or
	// won the double-check). Obtain the real connection handles.
	conn, client, err = tryConnect(ctx, socketPath, tp)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting after startup: %w", err)
	}
	return client, conn, nil
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
		// Phase 12 / Phase 58 D-06: otelgrpc client handler for trace
		// propagation (D-01, D-12). obs.ClientStatsHandler centralises the
		// TracerProvider + WithPropagators(TraceContext{}) option set so the
		// client and server sites cannot drift — see internal/obs/grpc.go.
		// Pitfall 5: only one WithStatsHandler call (gRPC silently overwrites
		// duplicates).
		grpc.WithStatsHandler(obs.ClientStatsHandler(tp)),
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

	// Disable the HTTP listener on the auto-started daemon: a forwarder/CLI dial
	// reaches the daemon over the unix SOCKET, never the Streamable-HTTP transport.
	// Leaving the default --http-addr=:8080 made cold-start fail with
	// "listen tcp :8080: bind: address already in use" whenever the port was taken
	// (a second helix daemon on a different socket, or any unrelated service),
	// silently killing the freshly-spawned daemon so the dial timed out after 10s.
	cmd := exec.Command(exe, "--serve", "--socket="+socketPath, "--http-addr=")
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
