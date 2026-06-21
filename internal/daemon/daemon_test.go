package daemon

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/config"
)

func TestEnsureSocket_NoFile(t *testing.T) {
	err := ensureSocket("/tmp/helix-test-nonexistent.sock")
	assert.NoError(t, err)
}

func TestEnsureSocket_StaleFile(t *testing.T) {
	// Create a stale socket file (no listener)
	path := filepath.Join(t.TempDir(), "stale.sock")
	os.WriteFile(path, []byte{}, 0600)
	err := ensureSocket(path)
	assert.NoError(t, err)
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "stale socket should be removed")
}

// shortSocketPath returns a short path for Unix sockets (macOS has 104-char limit).
func shortSocketPath(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join("/tmp", "helix-test-"+name)
	os.MkdirAll(dir, 0700)
	path := filepath.Join(dir, "s.sock")
	t.Cleanup(func() { os.RemoveAll(dir) })
	return path
}

func TestEnsureSocket_ActiveDaemon(t *testing.T) {
	// Start a real listener to simulate active daemon
	path := shortSocketPath(t, "active")
	ln, err := net.Listen("unix", path)
	require.NoError(t, err)
	defer ln.Close()

	err = ensureSocket(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "daemon already running")
}

func TestDaemon_StartsAndStops(t *testing.T) {
	socketPath := shortSocketPath(t, "lifecycle")
	cfg := &config.SerenaConfig{}
	cfg.Daemon.SocketPath = socketPath
	cfg.Daemon.ShutdownTimeout = 2

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d, err := New(cfg, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for socket to appear
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify socket exists
	_, statErr := os.Stat(socketPath)
	assert.NoError(t, statErr, "socket file should exist")

	// Cancel context triggers shutdown
	cancel()
	err = <-errCh
	// context.Canceled is expected
	assert.ErrorIs(t, err, context.Canceled)
}
