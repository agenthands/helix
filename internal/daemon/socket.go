package daemon

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"
)

// ensureSocket checks for stale socket files and cleans up if needed (DMN-13).
// Returns nil if socket path is available, error if daemon is already running.
func ensureSocket(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // No socket file, path is available
	}

	// Socket file exists -- try connecting to check if daemon is alive
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		// Connection refused or timeout = stale socket from crashed daemon
		slog.Info("removing stale socket file", "path", path)
		return os.Remove(path)
	}
	conn.Close()
	return fmt.Errorf("daemon already running at %s", path)
}

// createSocketDir ensures the directory for the socket file exists with 0700 permissions.
func createSocketDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0700)
}
