// Package obs holds the noop-default observability scaffolding for Serena.
// Phase 10: no metrics, no tracing, no exporters — only the slog ContextHandler
// wrapper and the Provider surface area that Phases 11/12 will extend.
//
// Design rules for this package (pin them early so later phases don't drift):
//
//   - Provider is a struct, never an interface, so Phases 11/12 can add methods
//     without breaking call sites.
//   - Noop always returns a non-nil *Provider with non-nil internal fields;
//     call sites never branch on `provider == nil`.
//   - Zero third-party dependencies. Stdlib only. No go.opentelemetry.io/*,
//     no github.com/prometheus/*. Phase 12 is the first place OTel appears.
package obs

import "log/slog"

// Provider is the single entry point for all observability wiring. Phase 10
// exposes only SlogHandler; Phase 11 will add a Meter accessor and Phase 12
// a Tracer accessor on this same type.
type Provider struct {
	// slogHandler is the wrapped handler installed into the daemon logger.
	slogHandler slog.Handler
}

// Noop returns a Provider that does nothing beyond wrapping the supplied
// inner slog.Handler with the trace-aware ContextHandler. In Phase 10 the
// wrapper's Handle path is a pure forward (spanContextFromContext is a
// stub that always returns false), so the noop provider is zero-cost on
// the log hot path while still reserving the injection point Phase 12 needs.
func Noop(inner slog.Handler) *Provider {
	return &Provider{
		slogHandler: NewContextHandler(inner),
	}
}

// SlogHandler returns the slog.Handler that should be passed to slog.New()
// at daemon and forwarder startup. Never returns nil.
func (p *Provider) SlogHandler() slog.Handler { return p.slogHandler }
