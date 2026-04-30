// Package bench_test: Phase 10 slog hot-path allocation benchmark.
//
// Proves OBS-06: obs.ContextHandler adds <= +1 alloc/op vs an unwrapped
// slog.JSONHandler on the no-span fast path. The Phase 10 stub
// spanContextFromContext always returns false, so this benchmark exercises
// the early-return path that must remain effectively zero-overhead until
// Phase 12 wires real span extraction.
//
// This file is strictly additive — it does not touch any existing bench
// fixture, helper, or baseline file.
package bench_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/agenthands/helix/internal/obs"
)

// newDiscardJSONHandler returns the same base handler shape the daemon
// installs in cli/root.go runDaemon — JSON output to io.Discard so IO cost
// is zero and allocations reflect only slog + wrapper work.
func newDiscardJSONHandler() slog.Handler {
	return slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})
}

// BenchmarkSlogHotPath_Baseline measures a bare slog.NewJSONHandler with no
// wrapping. This is the Phase 9 reference shape used as the OBS-06 anchor.
func BenchmarkSlogHotPath_Baseline(b *testing.B) {
	logger := slog.New(newDiscardJSONHandler())
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		logger.LogAttrs(ctx, slog.LevelInfo, "tool_call",
			slog.String("tool", "find_symbol"),
			slog.String("workspace", "bench"),
			slog.Int("latency_ms", 42),
		)
	}
}

// BenchmarkSlogHotPath_WithContextHandler measures the same emission path
// through obs.NewContextHandler. The Phase 10 stub always takes the no-span
// fast path, so the allocation delta vs the baseline MUST be <= +1 alloc/op
// (OBS-06 contract).
func BenchmarkSlogHotPath_WithContextHandler(b *testing.B) {
	logger := slog.New(obs.NewContextHandler(newDiscardJSONHandler()))
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		logger.LogAttrs(ctx, slog.LevelInfo, "tool_call",
			slog.String("tool", "find_symbol"),
			slog.String("workspace", "bench"),
			slog.Int("latency_ms", 42),
		)
	}
}
