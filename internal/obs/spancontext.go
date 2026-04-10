package obs

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// SpanContext carries the minimal trace identifiers that ContextHandler
// attaches to every log record when a span is active on the context.
type SpanContext struct {
	TraceID string
	SpanID  string
}

// spanContextFromContext extracts trace/span IDs from the OTel span carried
// on ctx. Returns (populated, true) when the span context is valid, or
// (zero, false) when there is no active span — preserving the Phase 10
// ContextHandler fast-path contract (zero alloc when no span).
//
// This replaces the Phase 10 stub that always returned false. The function
// is intentionally UNEXPORTED — no other package may depend on its shape.
func spanContextFromContext(ctx context.Context) (SpanContext, bool) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return SpanContext{}, false
	}
	return SpanContext{
		TraceID: sc.TraceID().String(),
		SpanID:  sc.SpanID().String(),
	}, true
}
