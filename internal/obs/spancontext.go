package obs

import "context"

// SpanContext carries the minimal trace identifiers that ContextHandler
// will eventually attach to every log record. Kept deliberately small:
// Phase 12 will decide whether to extend (trace flags, baggage, etc.)
// or to replace with direct OTel SpanContext lookup.
type SpanContext struct {
	TraceID string
	SpanID  string
}

// ctxKey is the unexported context key used by WithSpanContext. Using a
// dedicated zero-size struct type avoids collisions with any other package's
// context values (Go's standard idiom for context keys).
type ctxKey struct{}

// WithSpanContext stores a SpanContext on ctx. Exported so Phase 12 tests can
// inject a fake span without needing the real OTel extractor wired up. Phase
// 10 production code does NOT call this — the daemon has no span source yet.
func WithSpanContext(ctx context.Context, sc SpanContext) context.Context {
	return context.WithValue(ctx, ctxKey{}, sc)
}

// spanContextFromContext is the Phase 10 stub extractor used by ContextHandler.
//
// CONTRACT (pinned for OBS-06 / Pitfall #1 in 10-RESEARCH.md): in Phase 10
// this function MUST always return (SpanContext{}, false). That guarantees
// ContextHandler.Handle always takes its early-return fast path, which in turn
// guarantees the wrapper adds zero allocations versus the unwrapped inner
// handler. The benchmark BenchmarkContextHandler_Handle pins this contract.
//
// Phase 12 will replace the body with real OTel span extraction (likely via
// trace.SpanContextFromContext). At that point the signature may change to
// return additional data (sampled flag, etc.), so this function is
// intentionally UNEXPORTED — no other package may grow a dependency on its
// current shape.
func spanContextFromContext(_ context.Context) (SpanContext, bool) {
	return SpanContext{}, false
}
