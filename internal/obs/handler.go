package obs

import (
	"context"
	"log/slog"
)

// ContextHandler wraps an inner slog.Handler and, when a SpanContext is
// present on ctx, appends trace_id and span_id attributes to each record.
// When ctx carries no span, Handle forwards to the inner handler without
// touching the record — this is the zero-alloc fast path that the Phase 10
// stub extractor always takes. See BenchmarkContextHandler_Handle.
type ContextHandler struct {
	inner slog.Handler
}

// NewContextHandler wraps inner with a trace-aware handler. The returned
// handler is safe to use anywhere a slog.Handler is expected; it delegates
// all decisions (level gating, formatting, output destination) to inner.
func NewContextHandler(inner slog.Handler) slog.Handler {
	return &ContextHandler{inner: inner}
}

// Enabled delegates to the inner handler.
func (h *ContextHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

// Handle injects trace_id/span_id when a span is present on ctx, then
// forwards to the inner handler. The early return is the OBS-06 hot-path
// guarantee: no attr construction, no allocation, no slice growth.
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	// Fast path: no span in ctx — forward unchanged, zero alloc.
	// Phase 10's spanContextFromContext always returns false, so this is
	// the only path exercised until Phase 12 wires real span extraction.
	sc, ok := spanContextFromContext(ctx)
	if !ok {
		return h.inner.Handle(ctx, r)
	}
	// Slow path (Phase 12+): append typed attrs. slog.String avoids the
	// boxing allocation that slog.Any would incur; see PITFALLS.md #3.
	r.AddAttrs(
		slog.String("trace_id", sc.TraceID),
		slog.String("span_id", sc.SpanID),
	)
	return h.inner.Handle(ctx, r)
}

// WithAttrs wraps the inner's WithAttrs result so the trace-injection
// wrapper identity is preserved across derived handlers (slog.Logger.With).
func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{inner: h.inner.WithAttrs(attrs)}
}

// WithGroup wraps the inner's WithGroup result for the same reason as
// WithAttrs — derived handlers must keep the ContextHandler wrapper.
func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{inner: h.inner.WithGroup(name)}
}
