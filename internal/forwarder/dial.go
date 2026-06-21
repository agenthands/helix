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
//
// Phase 94 RETIRE-04: when tcpAddr is non-empty the dial targets a loopback gRPC
// TCP endpoint (split-host topology) instead of the unix socket. In that mode
// there is NO cold-start auto-spawn — a remote/TCP daemon is operator-managed, so
// the dial either connects or fails fast; the lock+spawn cold path is unix-only.
func ConnectOrStartDaemon(ctx context.Context, socketPath, tcpAddr string, logger *slog.Logger, tp trace.TracerProvider) (serenav1.ForwarderServiceClient, *grpc.ClientConn, error) {
	// TCP endpoint: no auto-start. Dial the operator-managed gRPC TCP daemon
	// directly; the unix cold-start lock+spawn machinery does not apply to a
	// split-host endpoint.
	if tcpAddr != "" {
		conn, client, err := tryConnect(ctx, socketPath, tcpAddr, tp)
		if err != nil {
			return nil, nil, fmt.Errorf("connecting to grpc tcp daemon %s: %w", tcpAddr, err)
		}
		logger.Info("connected to grpc tcp daemon", "addr", tcpAddr)
		return client, conn, nil
	}

	// Warm-reuse fast path: try connecting to an existing daemon BEFORE taking the
	// startup lock so the common (daemon-already-up) case is lock-free.
	conn, client, err := tryConnect(ctx, socketPath, "", tp)
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
		// WR-01/WR-02: the double-check and wait probes build a live
		// *grpc.ClientConn on success. The guard only consumes the
		// success/failure signal, so close the probe conn here — otherwise the
		// lock-loser / TOCTOU-peer / spawn paths leak its resolver+keepalive
		// goroutines and FD for the life of the process (ConnectOrStartDaemon
		// re-dials the single real conn at the tryConnect below).
		connect: func(sp string) error {
			c, _, e := tryConnect(ctx, sp, "", tp)
			if c != nil {
				_ = c.Close()
			}
			return e
		},
		spawn: startDaemon,
		waitUp: func(sp string) error {
			// NB: waitForDaemon returns (client, conn, err) — conn is the SECOND
			// value, unlike tryConnect which returns conn first.
			_, c, e := waitForDaemon(ctx, sp, 10*time.Second, tp)
			if c != nil {
				_ = c.Close()
			}
			return e
		},
	}); err != nil {
		return nil, nil, fmt.Errorf("starting daemon: %w", err)
	}

	// Guard returned success: the daemon is up (we spawned it, reused a peer's, or
	// won the double-check). Obtain the real connection handles.
	conn, client, err = tryConnect(ctx, socketPath, "", tp)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting after startup: %w", err)
	}
	return client, conn, nil
}

// tryConnect attempts to connect to a daemon at the given socket path, or — when
// tcpAddr is non-empty — at the given loopback gRPC TCP endpoint (Phase 94
// RETIRE-04). The tp parameter provides an explicit TracerProvider for the
// otelgrpc stats handler.
//
// The two paths differ ONLY in the network: the TCP path skips the unix-file
// liveness probe (os.Stat + unix net.DialTimeout, which are unix-socket-specific)
// and targets `passthrough:///host:port` instead of `unix://path`. "tcp://" is
// NOT a valid gRPC target scheme — passthrough:/// dials the address verbatim,
// the direct analog of the unix:// form. All other dial options (keepalive +
// obs.ClientStatsHandler) are byte-for-byte identical so the two transports cannot
// drift.
func tryConnect(_ context.Context, socketPath, tcpAddr string, tp trace.TracerProvider) (*grpc.ClientConn, serenav1.ForwarderServiceClient, error) {
	target := "unix://" + socketPath
	if tcpAddr != "" {
		// TCP endpoint: skip the unix-file liveness probe (unix-socket-specific)
		// and dial the address verbatim via the passthrough resolver.
		target = "passthrough:///" + tcpAddr
	} else {
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
	}

	conn, err := grpc.NewClient(
		target,
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

// daemonLogPath derives the per-socket log file the auto-started daemon's
// stdout/stderr are redirected to (WR-05). Like lockfilePath it is a plain
// regular file beside the socket, inheriting the per-uid 0700 socket dir
// permissions, and is never bind()ed so the AF_UNIX sun_path length limit does
// not apply.
func daemonLogPath(socketPath string) string {
	return socketPath + ".daemon.log"
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
	// WR-05: capture the auto-started daemon's stderr to a log file beside the
	// socket so cold-start failures (bad config, unwritable socket dir, a port
	// bind that kills the daemon) are diagnosable instead of surfacing only as an
	// opaque 10s waitForDaemon timeout. The file lives in the existing per-uid
	// 0700 socket dir, so it inherits those permissions (no new permission code).
	// Best-effort: if the log can't be opened, fall back to discarding output
	// rather than failing the spawn. The fd is intentionally NOT closed here — the
	// detached child keeps writing to it for its lifetime; the OS reclaims it when
	// this short-lived CLI process exits.
	if logFile, lerr := os.OpenFile(daemonLogPath(socketPath), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); lerr == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		cmd.Stdout = nil
		cmd.Stderr = nil
	}

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
			// Try connecting (cold-start is unix-only; tcpAddr empty)
			conn, client, err := tryConnect(ctx, socketPath, "", tp)
			if err == nil {
				return client, conn, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, nil, fmt.Errorf("daemon did not start within %s", timeout)
}
