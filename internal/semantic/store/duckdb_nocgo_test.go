//go:build !cgo

package store

import (
	"context"
	"errors"
	"testing"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/semantic"
)

// TestStub_Open_ReturnsUnsupported asserts that under CGO_ENABLED=0 the
// Open function returns an error matching serr.ErrUnsupported. Defense in
// depth — the daemon refuses CGO=0 at step 6a, so this code path is only
// exercised by unit tests built with CGO_ENABLED=0 explicitly.
func TestStub_Open_ReturnsUnsupported(t *testing.T) {
	cfg := semantic.Config{
		Enabled: true,
		Store:   semantic.StoreConfig{Kind: "duckdb", Path: ".helix/semantic.duckdb"},
	}
	_, err := Open(context.Background(), cfg, nil, nil)
	if err == nil {
		t.Fatal("Open under !cgo: want error, got nil")
	}
	if !errors.Is(err, serr.ErrUnsupported) {
		t.Errorf("Open under !cgo: want errors.Is(serr.ErrUnsupported), got %v", err)
	}
}

// TestStub_Available_ReturnsFalse asserts that under CGO_ENABLED=0 the
// Store stub's Available() reports false. Mirrors the
// internal/treesitter.Available const-false-under-!cgo pattern.
func TestStub_Available_ReturnsFalse(t *testing.T) {
	s := &Store{}
	if s.Available() {
		t.Error("(*Store).Available() under !cgo: want false, got true")
	}
}
