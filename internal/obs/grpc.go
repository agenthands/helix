// Phase 58 D-06: centralized constructors for the otelgrpc stats handlers
// used by the forwarder client (internal/forwarder/dial.go) and the daemon
// server (internal/daemon/daemon.go). Both sites need (a) an explicit
// TracerProvider (no-global D-01) and (b) the W3C TraceContext propagator
// wired per-handler (the propagator extension of the no-global rule).
//
// Centralising the helpers here means the client and server propagator
// option set cannot drift — a future contributor cannot accidentally land a
// dial site that omits WithPropagators while the server site retains it
// (or vice versa), breaking trace continuity asymmetrically.
//
// Design rules:
//
//   - No call to otel.SetTextMapPropagator in this file or anywhere else
//     in internal/obs (D-01 extends to the propagator).
//   - Both helpers take an explicit trace.TracerProvider; nil callers are
//     not supported (use obs.Provider.TracerProvider() which is non-nil
//     by Noop construction).
//   - Return type is google.golang.org/grpc/stats.Handler so call sites
//     pass the result directly to grpc.WithStatsHandler / grpc.StatsHandler.
package obs

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/stats"
)

// ClientStatsHandler returns the standard otelgrpc client handler with
// TraceContext propagator pre-wired. Phase 58 D-06: keeps propagator
// wiring in one place so dial sites cannot drift from server sites.
func ClientStatsHandler(tp trace.TracerProvider) stats.Handler {
	return otelgrpc.NewClientHandler(
		otelgrpc.WithTracerProvider(tp),
		otelgrpc.WithPropagators(propagation.TraceContext{}),
	)
}

// ServerStatsHandler is the symmetric server-side helper. Use this in
// grpc.NewServer(grpc.StatsHandler(...)) to keep traceparent extraction
// consistent with the client side.
func ServerStatsHandler(tp trace.TracerProvider) stats.Handler {
	return otelgrpc.NewServerHandler(
		otelgrpc.WithTracerProvider(tp),
		otelgrpc.WithPropagators(propagation.TraceContext{}),
	)
}
