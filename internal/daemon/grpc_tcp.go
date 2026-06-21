// Package daemon gated gRPC TCP listener (Phase 94 RETIRE-04).
//
// This file adds an OPTIONAL, loopback-gated gRPC TCP listener for split-host
// CLI↔daemon use. It is the network-transparent topology that the deleted HTTP
// `/mcp` head used to cover (94-02 deletes that head). The default is the local
// unix socket (daemon.grpc_addr empty → no-op).
//
// Security shape (mirrors the v1.2 admin-addr pattern, telemetry.go:40-121):
// validateGRPCAddr refuses non-loopback addresses BEFORE net.Listen, pointing
// at REMOTE-01 for the deferred remote/auth scope. The listener reuses the SAME
// ForwarderService.StreamMCP RPC as listenSocket (no proto change), so both the
// tcp and unix paths dispatch into the identical mcpServer.SDK() with all 6
// middlewares attached.
package daemon

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	"google.golang.org/grpc"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
	"github.com/agenthands/helix/internal/obs"
)

// grpcTCPListenerAddr is a TEST-ONLY hook that publishes the bound listener
// address once net.Listen succeeds, so lifecycle tests can dial the gRPC TCP
// listener without racing the Serve loop. It is nil whenever no gRPC TCP
// listener is running (mirrors adminListenerAddr in telemetry.go).
var grpcTCPListenerAddr atomic.Pointer[string]

// newForwarderServiceHandler builds the ForwarderService handler registered on
// every gRPC server the daemon serves (unix socket AND the optional tcp
// listener). Factoring it here keeps the two transports byte-for-byte identical:
// both register the same handler, differing only in the network they listen on.
//
// The testServeSession seam (nil in production) lets a daemon test stub the
// MCP-runtime portion of StreamMCP without spinning up a full MCP server.
func (d *Daemon) newForwarderServiceHandler() *forwarderServiceHandler {
	return &forwarderServiceHandler{
		mcpServer: d.mcpServer,
		kernel:    d.kernel,
		logger:    d.logger,
		// Phase 53 D-17: direct call to *obs.Metrics for stdio session
		// lifecycle emission. observability.Metrics() is never nil per the
		// Noop-default invariant — no nil guard needed inside the handler.
		metrics: d.obs.Metrics(),
		// Phase 61 P03 (B2): forward DeactivateWorkspace to the live
		// bundle so cached enrichment leases for the workspace are
		// released promptly per CONTEXT lines 492-494.
		live: d.live,
		// Phase 94 RETIRE-04: test-only StreamMCP session runner seam.
		serveSession: d.testServeSession,
	}
}

// validateGRPCAddr ensures the configured gRPC TCP address is loopback-only.
// Empty is valid (disabled). Unlike the pre-existing validateAdminAddr
// (telemetry.go:108), an EMPTY host (":9099" / ":0") is REFUSED here: that form
// makes net.Listen("tcp", addr) bind ALL interfaces (0.0.0.0 + ::), which would
// expose the full ForwarderService.StreamMCP MCP tool surface — read/edit/
// refactor tools behind all middlewares — plaintext and unauthenticated on the
// LAN (RETIRE-04: "no unauthenticated non-loopback exposure by default").
// net.IP.IsLoopback() is the source of truth for explicit hosts; wildcard and
// non-loopback addresses are refused, pointing at REMOTE-01 (the deferred
// remote/auth scope recorded in REMOTE-SCOPE-ADR.md).
//
// NOTE: validateAdminAddr has the SAME empty-host gap, but the admin path only
// exposes instrumentation endpoints (/healthz, /metrics, opt-in pprof). The gap
// is materially worse on this surface (the full MCP runtime), so it is closed
// here regardless of the admin precedent. See REVIEW-FIX.md (CR-01) for the
// admin-sibling note.
// ValidateGRPCAddr is the exported wrapper over validateGRPCAddr for the CLI
// composition root (WR-02). It lets runDaemon refuse a misconfigured insecure
// bind early — before any subsystem starts — so the operator gets an actionable
// "that address is not allowed" message instead of an opaque late "daemon
// exited" once the listener goroutine runs. The listener still re-validates
// internally (defense in depth); this is purely a fail-fast front door.
func ValidateGRPCAddr(addr string) error {
	return validateGRPCAddr(addr)
}

func validateGRPCAddr(addr string) error {
	if addr == "" {
		return nil // disabled is valid (unix-socket only — the default)
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid grpc addr %q: %w", addr, err)
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return nil
	case "":
		// ":9099" / ":0" → net.Listen binds ALL interfaces. Refuse: the
		// loopback gate must name an explicit loopback host.
		return fmt.Errorf("grpc addr must name an explicit loopback host, got wildcard %q "+
			"(use 127.0.0.1:PORT or [::1]:PORT; non-loopback gRPC bind is deferred to REMOTE-01)", addr)
	}
	// IsUnspecified() defends against 0.0.0.0 / :: should the explicit branch
	// ever be reordered (those are already rejected as non-loopback today, but
	// the guard documents the intent: never accept a wildcard bind).
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() && !ip.IsUnspecified() {
		return nil
	}
	return fmt.Errorf("grpc addr must be loopback, got %q (non-loopback gRPC bind is deferred to REMOTE-01)", host)
}

// listenGRPCTCP runs the OPTIONAL loopback gRPC TCP listener until ctx is
// cancelled. It is a no-op when daemon.grpc_addr == "" (the default), and
// returns a descriptive error for non-loopback addresses (validateGRPCAddr).
//
// It mirrors listenAdmin's empty-addr no-op + validate gate (telemetry.go:40),
// but serves the SAME ForwarderService over tcp using the listenSocket gRPC
// lifecycle (RegisterForwarderServiceServer + GracefulStop on ctx.Done —
// daemon.go listenSocket). NOT the HTTP server.Shutdown form: this is gRPC.
func (d *Daemon) listenGRPCTCP(ctx context.Context) error {
	addr := d.config.Daemon.GRPCAddr
	if addr == "" {
		return nil // disabled (default) — unix-socket only
	}
	if err := validateGRPCAddr(addr); err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen grpc tcp %s: %w", addr, err)
	}

	// Publish bound address for the test hook, and ensure it is cleared on exit.
	bound := ln.Addr().String()
	grpcTCPListenerAddr.Store(&bound)
	defer grpcTCPListenerAddr.Store(nil)

	// Reuse the SAME stats handler the unix listener uses so the tcp path is
	// byte-for-byte the unix path with a different network (listenSocket).
	srv := grpc.NewServer(
		grpc.StatsHandler(obs.ServerStatsHandler(d.obs.TracerProvider())),
	)
	serenav1.RegisterForwarderServiceServer(srv, d.newForwarderServiceHandler())

	d.logger.Info("grpc tcp listener started", "addr", bound)

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		srv.GracefulStop()
		return ctx.Err()
	case err := <-serveDone:
		return err
	}
}
