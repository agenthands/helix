// Package bench_test: Phase 11 TelemetryMiddleware hot-path benchmark.
//
// Proves METRIC-02 budget: wrapping a tools/call pass-through with
// TelemetryMiddleware adds <= +3 allocs/op vs an identically-shaped
// no-op middleware baseline measured in the SAME run (Case B from
// 11-04-PLAN.md, because the v1.2-phase10 baseline contains only the
// BenchmarkSlogHotPath family — no middleware analog).
//
// The in-run baseline BenchmarkBaselineMiddleware eliminates
// environmental drift between the Phase 10 capture and this Phase 11
// capture; the delta is computed directly from the two benchmark names
// in the single Phase 11 baseline file.
//
// All benchmarks use testing.B.Loop (Go 1.25, BENCH-01) and report
// allocations. Request construction happens OUTSIDE the hot loop so
// only middleware + inner-handler cost is measured.
package bench_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	helixmcp "github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
)

// noopInnerHandler is the innermost MethodHandler — returns a zero
// CallToolResult and nil error. It is the "do nothing" reference the
// middleware wraps.
func noopInnerHandler(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
	return &mcpsdk.CallToolResult{}, nil
}

// passthroughMiddleware is the minimal mcpsdk.Middleware shape: it
// only calls next(ctx, method, req). This establishes the in-run
// "delta origin" for BenchmarkTelemetryMiddleware — any allocations
// above this baseline are attributable to TelemetryMiddleware's
// metric-emission work (classifyOutcome, extractToolName, Snapshot,
// WithLabelValues).
func passthroughMiddleware(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
	return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return next(ctx, method, req)
	}
}

// newBenchCallToolReq builds a *mcpsdk.CallToolRequest once, outside
// the hot loop, so request construction cost does NOT pollute the
// middleware measurement.
func newBenchCallToolReq(name string) *mcpsdk.CallToolRequest {
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      name,
			Arguments: json.RawMessage(`{}`),
		},
	}
}

// BenchmarkBaselineMiddleware measures a no-op pass-through middleware
// wrapping the noop inner handler. This is the Case B in-run reference
// for BenchmarkTelemetryMiddleware (11-04-PLAN.md Task 1). Expected to
// be extremely cheap — one function-call indirection.
func BenchmarkBaselineMiddleware(b *testing.B) {
	wrapped := passthroughMiddleware(noopInnerHandler)
	req := newBenchCallToolReq("test_tool")
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = wrapped(ctx, "tools/call", req)
	}
}

// BenchmarkTelemetryMiddleware measures the real Phase 11 middleware
// wrapping the same noop inner handler. The allocs/op delta vs
// BenchmarkBaselineMiddleware MUST be <= +3 (METRIC-02, D-09 budget,
// A4 resolution).
//
// Setup constructs a real *obs.Provider via obs.Noop — Metrics() is
// non-nil and backed by a genuine Prometheus registry, matching the
// production shape. The session closure returns a pre-built
// *SessionInfo so Snapshot() exercises the RLock path the hot path
// actually takes.
func BenchmarkTelemetryMiddleware(b *testing.B) {
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := &helixmcp.SessionInfo{
		Profile:  "claude-code",
		Mode:     "edit",
		Language: "go",
	}
	getSession := func(ctx context.Context) *helixmcp.SessionInfo { return sess }

	mw := helixmcp.TelemetryMiddleware(provider, getSession, nil, logger)
	wrapped := mw(noopInnerHandler)
	req := newBenchCallToolReq("find_symbol")
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = wrapped(ctx, "tools/call", req)
	}
}

// BenchmarkTelemetryMiddleware_ToolsList asserts the early-return fast
// path for non-tools/call methods. No metric emission occurs; only the
// pass-through + structured-log branch runs. Expected to be near
// identical to BenchmarkBaselineMiddleware (slog.Info to io.Discard
// still costs ~1 call, but no Snapshot, no WithLabelValues).
func BenchmarkTelemetryMiddleware_ToolsList(b *testing.B) {
	provider := obs.Noop(slog.NewTextHandler(io.Discard, nil))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess := &helixmcp.SessionInfo{
		Profile:  "claude-code",
		Mode:     "edit",
		Language: "go",
	}
	getSession := func(ctx context.Context) *helixmcp.SessionInfo { return sess }

	mw := helixmcp.TelemetryMiddleware(provider, getSession, nil, logger)
	wrapped := mw(noopInnerHandler)
	req := newBenchCallToolReq("find_symbol")
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = wrapped(ctx, "tools/list", req)
	}
}
