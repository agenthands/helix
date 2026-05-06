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

	// Phase 0: stop live-update + enrichment manager (Phase 61 P03 / B2
	// safety-net) so cached enrichment leases are released BEFORE the
	// kernel tears down its worker pool.
	if d.live != nil {
		d.live.Stop()
	}

	// Phase 1: Stop kernel (drain workers, close LS processes).
	if d.kernel != nil {
		if err := d.kernel.Shutdown(ctx); err != nil {
			d.logger.Warn("kernel shutdown error", "error", err)
		}
	}

	// Phase 1.5: Flush tracing BEFORE listener close, with a DEDICATED
	// context (NOT the cancelled errgroup ctx — PITFALLS #4 flush race).
	// sdktrace.TracerProvider.Shutdown panics on double-call (Pitfall 6);
	// this block runs exactly once per daemon lifecycle via the single-shot
	// shutdown() call from Daemon.Run.
	if d.obs != nil {
		flushCtx, flushCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := d.obs.ShutdownTracing(flushCtx); err != nil {
			d.logger.Warn("trace exporter shutdown error", "error", err)
		}
		flushCancel()
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
