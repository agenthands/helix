// Package daemon admin listener (Phase 10 OBS-03/04/05).
//
// This file implements the dedicated loopback admin surface: /healthz, /readyz,
// and conditionally /debug/pprof/*. The listener is a non-fatal errgroup
// goroutine in daemon.Run — bind failure is logged and the daemon continues.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	nhpprof "net/http/pprof"
	"sync/atomic"
	"time"
)

// ready is flipped to 1 at the end of Daemon.Run setup (after kernel.Run
// spawned, listeners up, profile resolved). /readyz reads this.
// Pitfall #3 mitigation: prevents /readyz from returning 200 before the
// daemon is actually ready to serve traffic.
var ready atomic.Uint32

// adminListenerAddr is a TEST-ONLY hook that publishes the bound listener
// address once net.Listen succeeds, so lifecycle tests can probe the
// admin listener without racing against the Serve loop. It is nil whenever
// no admin listener is running.
var adminListenerAddr atomic.Pointer[string]

// listenAdmin runs the loopback admin HTTP listener until ctx is cancelled.
// It is a no-op when AdminAddr == "" and returns a descriptive error for
// non-loopback addresses (referencing the v1.3 auth roadmap).
//
// The caller in daemon.Run wraps this in an errgroup goroutine that swallows
// errors per D-04 — admin is instrumentation, not product.
func (d *Daemon) listenAdmin(ctx context.Context) error {
	addr := d.config.Observability.AdminAddr
	if addr == "" {
		return nil // disabled
	}
	if err := validateAdminAddr(addr); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen admin %s: %w", addr, err)
	}

	// Publish bound address for test hook, and ensure it is cleared on exit.
	bound := ln.Addr().String()
	adminListenerAddr.Store(&bound)
	defer adminListenerAddr.Store(nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", d.handleHealthz)
	mux.HandleFunc("/readyz", d.handleReadyz)
	if d.config.Observability.EnablePprof {
		registerPprof(mux)
		d.logger.Info("pprof endpoints enabled", "addr", bound)
	}

	server := &http.Server{Handler: mux}
	d.logger.Info("admin listener started",
		"addr", bound,
		"pprof", d.config.Observability.EnablePprof,
	)

	// Mirror listenHTTP's graceful shutdown pattern (Pitfall #2).
	serveDone := make(chan error, 1)
	go func() {
		if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveDone <- err
		}
		close(serveDone)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-serveDone:
		return err
	}
}

// validateAdminAddr ensures the configured admin address is loopback-only.
// Non-loopback addresses are refused with an error that references the v1.3
// auth roadmap (Pitfall #6 contract).
func validateAdminAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid admin addr %q: %w", addr, err)
	}
	switch host {
	case "", "localhost", "127.0.0.1", "::1":
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("admin addr must be loopback, got %q (v1.3 will add auth for non-loopback)", host)
}

// registerPprof wires the net/http/pprof handlers onto the admin mux.
// Called ONLY when cfg.Observability.EnablePprof == true (D-10, D-12).
// Importing net/http/pprof has the side effect of registering handlers on
// http.DefaultServeMux, but the daemon never uses DefaultServeMux so this is
// invisible and harmless.
func registerPprof(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", nhpprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", nhpprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", nhpprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", nhpprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", nhpprof.Trace)
}

// handleHealthz always responds 200 when the listener is up (D-13).
// The body is the literal {"status":"ok"} — no hostname, version, or internal
// state exposed (threat T-10-09 accept).
func (d *Daemon) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleReadyz returns 200 only when the ready atomic has been flipped at
// the end of Daemon.Run setup; 503 otherwise (D-14, Pitfall #3).
func (d *Daemon) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if ready.Load() == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "starting"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}
