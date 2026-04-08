package daemon

import (
	"context"
	"os"
	"time"
)

// shutdown performs two-phase graceful shutdown (DMN-12).
// Phase 1: Stop kernel (drain workers, close LS processes) per D-10.
// Phase 2: Close listeners, clean up socket.
func (d *Daemon) shutdown() {
	timeout := time.Duration(d.config.Daemon.ShutdownTimeout) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	d.logger.Info("graceful shutdown starting", "timeout", timeout)

	// Phase 1: Stop kernel (drain workers, close LS processes).
	if d.kernel != nil {
		if err := d.kernel.Shutdown(ctx); err != nil {
			d.logger.Warn("kernel shutdown error", "error", err)
		}
	}

	// Phase 2: Close listeners.
	if d.socketListener != nil {
		d.socketListener.Close()
	}
	// Clean up socket file.
	if d.config.Daemon.SocketPath != "" {
		os.Remove(d.config.Daemon.SocketPath)
	}

	d.logger.Info("graceful shutdown complete")
}
