package obs

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

// recordingHandler is a minimal slog.Handler that captures the last record
// passed to Handle, and tracks calls to WithAttrs/WithGroup/Enabled.
type recordingHandler struct {
	lastRecord  slog.Record
	handleCount int
	enabledHits int
	withAttrs   int
	withGroup   int
	// attrs captured via WithAttrs (shallow, single level for tests)
	attrs []slog.Attr
}

func (h *recordingHandler) Enabled(_ context.Context, _ slog.Level) bool {
	h.enabledHits++
	return true
}

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.handleCount++
	h.lastRecord = r
	return nil
}

func (h *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.withAttrs++
	nh := &recordingHandler{attrs: append([]slog.Attr{}, h.attrs...)}
	nh.attrs = append(nh.attrs, attrs...)
	return nh
}

func (h *recordingHandler) WithGroup(_ string) slog.Handler {
	h.withGroup++
	return &recordingHandler{}
}

// Test 2: ContextHandler delegates Enabled, WithAttrs, WithGroup to the inner.
func TestContextHandler_Delegates(t *testing.T) {
	rec := &recordingHandler{}
	h := NewContextHandler(rec)

	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("expected Enabled to return true from inner")
	}
	if rec.enabledHits != 1 {
		t.Fatalf("expected 1 Enabled call on inner, got %d", rec.enabledHits)
	}

	h2 := h.WithAttrs([]slog.Attr{slog.String("k", "v")})
	if _, ok := h2.(*ContextHandler); !ok {
		t.Fatalf("expected WithAttrs to return *ContextHandler, got %T", h2)
	}
	if rec.withAttrs != 1 {
		t.Fatalf("expected 1 WithAttrs call on inner, got %d", rec.withAttrs)
	}

	h3 := h.WithGroup("grp")
	if _, ok := h3.(*ContextHandler); !ok {
		t.Fatalf("expected WithGroup to return *ContextHandler, got %T", h3)
	}
	if rec.withGroup != 1 {
		t.Fatalf("expected 1 WithGroup call on inner, got %d", rec.withGroup)
	}
}

// Test 3: ContextHandler.Handle with context.Background() forwards the record
// unchanged — no trace_id or span_id attributes should be appended.
func TestContextHandler_Handle_FastPath_NoSpan(t *testing.T) {
	rec := &recordingHandler{}
	h := NewContextHandler(rec)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	r.AddAttrs(slog.String("preexisting", "value"))
	startAttrs := r.NumAttrs()

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if rec.handleCount != 1 {
		t.Fatalf("expected 1 inner Handle call, got %d", rec.handleCount)
	}
	if rec.lastRecord.NumAttrs() != startAttrs {
		t.Fatalf("expected NumAttrs unchanged (%d), got %d — fast path should not inject attrs", startAttrs, rec.lastRecord.NumAttrs())
	}
	// Walk attrs and verify no trace_id / span_id leaked in.
	rec.lastRecord.Attrs(func(a slog.Attr) bool {
		if a.Key == "trace_id" || a.Key == "span_id" {
			t.Fatalf("unexpected %q attr in fast path", a.Key)
		}
		return true
	})
}

// Test 4: Benchmark the hot path with b.ReportAllocs() to guard OBS-06
// (≤ +1 alloc/op budget). Phase 10 stub makes this the *always* path.
func BenchmarkContextHandler_Handle(b *testing.B) {
	inner := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})
	h := NewContextHandler(inner)
	ctx := context.Background()
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "bench", 0)
	r.AddAttrs(slog.String("k", "v"))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Handle(ctx, r)
	}
}

// Test 5: spanContextFromContext(Background) returns (zero, false) — this
// documents the Phase 10 stub so Phase 12 can safely flip it.
func TestSpanContextFromContext_Phase10Stub(t *testing.T) {
	sc, ok := spanContextFromContext(context.Background())
	if ok {
		t.Fatalf("expected Phase 10 stub to always return false, got ok=true with %+v", sc)
	}
	if sc != (SpanContext{}) {
		t.Fatalf("expected zero SpanContext, got %+v", sc)
	}

	// Even with a SpanContext set via the exported helper, Phase 10 stub
	// should *still* return false (the stub is intentionally blind). This
	// pins the contract so Phase 12's replacement is a visible diff.
	ctx := WithSpanContext(context.Background(), SpanContext{TraceID: "t", SpanID: "s"})
	sc2, ok2 := spanContextFromContext(ctx)
	if ok2 {
		t.Fatalf("Phase 10 stub must always return false, got ok=true with %+v", sc2)
	}
}
