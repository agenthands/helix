package obs

import (
	"io"
	"log/slog"
	"testing"
)

// Test 1: obs.Noop returns a non-nil Provider whose SlogHandler() is non-nil
// and wraps the inner handler.
func TestNoop_ReturnsNonNilProvider(t *testing.T) {
	inner := slog.NewTextHandler(io.Discard, nil)
	p := Noop(inner)
	if p == nil {
		t.Fatal("expected non-nil Provider")
	}
	if p.SlogHandler() == nil {
		t.Fatal("expected non-nil SlogHandler")
	}
	// SlogHandler should wrap inner in a ContextHandler (not be the inner itself)
	if _, ok := p.SlogHandler().(*ContextHandler); !ok {
		t.Fatalf("expected *ContextHandler, got %T", p.SlogHandler())
	}
}
