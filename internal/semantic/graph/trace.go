package graph

import "context"

// traceApplyRepair returns (ctx, end) where end finalises the
// "semantic.graph.apply_repair" span. Phase 62 P02 ships the seam without
// otel wiring; P03 may swap in a real opentelemetry.SpanFromContext call.
// The end function is safe to call with a nil err.
//
// Keeping the seam in place now means ApplyRepair can call it later
// without a breaking signature change.
func traceApplyRepair(ctx context.Context, repoID string) (context.Context, func(error)) {
	_ = repoID
	return ctx, func(error) {}
}
