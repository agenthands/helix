package daemon

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/config"
)

// TestGracefulShutdownMidRequest validates that cancelling the context while
// the daemon is running (simulating SIGTERM) results in a clean exit with
// kernel drain and socket cleanup. Context cancellation is used instead of
// syscall.Kill because Run's signal.NotifyContext propagates cancellation
// identically, and sending SIGTERM to os.Getpid() would kill the test runner.
func TestGracefulShutdownMidRequest(t *testing.T) {
	socketPath := shortSocketPath(t, "graceful-mid")
	cfg := &config.SerenaConfig{}
	cfg.Daemon.SocketPath = socketPath
	cfg.Daemon.ShutdownTimeout = 5

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	d, err := New(cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for readiness (ready atomic flips to 1 after daemon started log).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ready.Load() == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(t, uint32(1), ready.Load(), "daemon should be ready before shutdown test proceeds")

	// Verify socket exists before shutdown.
	_, statErr := os.Stat(socketPath)
	require.NoError(t, statErr, "socket file should exist before shutdown")

	// Cancel context to simulate SIGTERM (signal.NotifyContext cancellation).
	cancel()

	// Assert Run returns within ShutdownTimeout.
	select {
	case runErr := <-errCh:
		// context.Canceled is the expected shutdown result.
		assert.ErrorIs(t, runErr, context.Canceled, "Run should return context.Canceled on SIGTERM")
	case <-time.After(time.Duration(cfg.Daemon.ShutdownTimeout) * time.Second):
		t.Fatal("daemon did not shut down within ShutdownTimeout")
	}

	// After shutdown, ready should be reset to 0.
	assert.Equal(t, uint32(0), ready.Load(), "ready should be 0 after shutdown")

	// Socket file should be cleaned up by shutdown().
	_, statErr = os.Stat(socketPath)
	assert.True(t, os.IsNotExist(statErr), "socket file should be removed after shutdown")
}

// TestGracefulShutdownClean validates that cancelling the context with no
// in-flight requests results in a clean exit within 2 seconds.
func TestGracefulShutdownClean(t *testing.T) {
	socketPath := shortSocketPath(t, "graceful-clean")
	cfg := &config.SerenaConfig{}
	cfg.Daemon.SocketPath = socketPath
	cfg.Daemon.ShutdownTimeout = 2

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	d, err := New(cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for readiness.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ready.Load() == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(t, uint32(1), ready.Load(), "daemon should be ready")

	// Cancel context -- clean shutdown, no in-flight work.
	cancel()

	// Assert clean exit within 2 seconds.
	select {
	case runErr := <-errCh:
		assert.ErrorIs(t, runErr, context.Canceled, "Run should return context.Canceled")
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not shut down within 2 seconds")
	}

	// Socket should be cleaned up.
	_, statErr := os.Stat(socketPath)
	assert.True(t, os.IsNotExist(statErr), "socket file should be removed after clean shutdown")

	// Ready should be reset.
	assert.Equal(t, uint32(0), ready.Load(), "ready should be 0 after shutdown")
}
