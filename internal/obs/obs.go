// Package obs holds the observability scaffolding for Serena.
//
// Phase 10 shipped the noop slog ContextHandler wrapper. Phase 11 adds
// Prometheus metrics (see metrics.go) — the obs package is now the single
// home for all prometheus/client_golang imports so the blast radius of that
// dependency stays contained.
//
// Design rules for this package (pin them early so later phases don't drift):
//
//   - Provider is a struct, never an interface, so future phases can add
//     methods without breaking call sites.
//   - Noop always returns a non-nil *Provider with non-nil internal fields;
//     call sites never branch on `provider == nil`. This extends to Metrics()
//     — the noop path also holds a real *Metrics sink.
//   - Third-party deps live HERE only. Phase 11: prometheus/client_golang.
//     Phase 12 will add go.opentelemetry.io/* in the same package.
package obs

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Provider is the single entry point for all observability wiring. Phase 10
// exposed SlogHandler; Phase 11 adds Metrics; Phase 12 will add a Tracer
// accessor on this same type.
type Provider struct {
	// slogHandler is the wrapped handler installed into the daemon logger.
	slogHandler slog.Handler
	// metrics holds the Prometheus vectors + registry. Never nil once the
	// Provider has been constructed via Noop.
	metrics *Metrics
	// tracerProvider holds the OTel TracerProvider. Noop initialises this to
	// tracenoop.NewTracerProvider(); WithTracing replaces it with an SDK
	// provider backed by an OTLP/gRPC exporter. Never nil (D-04).
	tracerProvider trace.TracerProvider
}

// Noop returns a Provider that does nothing beyond wrapping the supplied
// inner slog.Handler with the trace-aware ContextHandler and pre-constructing
// a Metrics sink on an owned registry. The log hot path remains zero-cost
// (spanContextFromContext is a stub); the metrics sink is unscraped unless
// the admin listener mounts /metrics (plan 11-01 task 3).
func Noop(inner slog.Handler) *Provider {
	return &Provider{
		slogHandler:    NewContextHandler(inner),
		metrics:        newMetrics(),
		tracerProvider: tracenoop.NewTracerProvider(),
	}
}

// NewForTest constructs a Provider with the given TracerProvider. Intended for
// test code that needs to inject a tracetest-backed provider. The slog handler
// and metrics sink are noop-initialised.
func NewForTest(tp trace.TracerProvider) *Provider {
	p := Noop(slog.Default().Handler())
	p.tracerProvider = tp
	return p
}

// SlogHandler returns the slog.Handler that should be passed to slog.New()
// at daemon and forwarder startup. Never returns nil.
func (p *Provider) SlogHandler() slog.Handler { return p.slogHandler }

// Metrics returns the Prometheus metrics sink. Never returns nil for a
// Provider constructed via Noop — downstream code is contractually allowed
// to call methods on the result without nil checks.
func (p *Provider) Metrics() *Metrics { return p.metrics }

// TracerProvider returns the underlying trace.TracerProvider. Never nil —
// Noop sets it to tracenoop.NewTracerProvider(), WithTracing may replace it
// with an SDK provider. Callers that need an explicit provider (e.g.
// otelgrpc.WithTracerProvider) use this accessor.
func (p *Provider) TracerProvider() trace.TracerProvider { return p.tracerProvider }

// Tracer returns a trace.Tracer scoped to the Serena module. Never returns
// nil (D-04): even the noop path yields a functional (no-op) tracer.
func (p *Provider) Tracer() trace.Tracer {
	return p.tracerProvider.Tracer("github.com/postfix/serena", trace.WithInstrumentationVersion("v1.2"))
}

// ShutdownTracing flushes and shuts down the TracerProvider if it implements
// Shutdown (i.e. it is an SDK TracerProvider, not the noop). No-op for the
// noop path. The caller should pass a context with a deadline (e.g. 5s) to
// bound the flush time (PITFALLS #4).
func (p *Provider) ShutdownTracing(ctx context.Context) error {
	sdk, ok := p.tracerProvider.(interface{ Shutdown(context.Context) error })
	if !ok {
		return nil
	}
	return sdk.Shutdown(ctx)
}
