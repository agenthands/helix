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

	// Phase 1.6: emit the semantic-store read total on a single shutdown log
	// line (Phase 81 ABLATE-06, Task 0 path A). The bench daemon runs
	// HTTP-disabled over a Unix socket (D-06), so the Prometheus /metrics scrape
	// is unreachable; the bench cell instead scrapes this line from daemon.log to
	// assert helix_semantic_store_reads_total == 0 on the no_semantic arm (D-05).
	// The line is parseable by the cell's scrapeSemanticReadsTotal: msg matches
	// "semantic store reads total" and the count is the integer counter value.
	// On the no_semantic arm the kernel gate (Plan 04) forces NoopLookup, so this
	// count is 0 — a non-zero value here means a semantic read survived the gate
	// and the bench cell fails hard.
	if d.obs != nil {
		d.logger.Info("semantic store reads total", "count", d.obs.Metrics().SemanticStoreReadsValue())
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
