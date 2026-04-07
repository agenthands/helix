package daemon

import (
	"context"
	"os"
	"time"
)

// shutdown performs two-phase graceful shutdown (DMN-12).
// Phase 1: Close listeners, drain active sessions.
// Phase 2: (Future) Stop LS workers, flush caches.
func (d *Daemon) shutdown() {
	timeout := time.Duration(d.config.Daemon.ShutdownTimeout) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	d.logger.Info("graceful shutdown starting", "timeout", timeout)

	// Phase 1: Close listeners
	if d.socketListener != nil {
		d.socketListener.Close()
	}
	// Clean up socket file
	if d.config.Daemon.SocketPath != "" {
		os.Remove(d.config.Daemon.SocketPath)
	}

	// Phase 2: Future -- stop LS workers, flush workspace state
	_ = ctx // used in Phase 2 for LS worker shutdown with timeout

	d.logger.Info("graceful shutdown complete")
}
