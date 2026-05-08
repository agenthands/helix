// Tests for the SemanticLookup interface and NoopLookup default. Phase 65 D-03.
package integ

import (
	"context"
	"errors"
	"testing"

	"github.com/agenthands/helix/internal/workspace"
)

// TestSemanticLookup_InterfaceShape is a compile-time assertion: NoopLookup
// must satisfy SemanticLookup. If a method is added or renamed on the
// interface and NoopLookup is not updated, this fails at build time.
func TestSemanticLookup_InterfaceShape(t *testing.T) {
	var _ SemanticLookup = NoopLookup{}
}

// TestNoopLookup_AvailableFalse asserts the documented NoopLookup contract:
// Available()=false; every other method returns ErrIndexErrored.
func TestNoopLookup_AvailableFalse(t *testing.T) {
	var l SemanticLookup = NoopLookup{}
	ctx := context.Background()
	ws := workspace.WorkspaceKey{}

	if l.Available() {
		t.Fatal("NoopLookup.Available() should be false")
	}

	if _, err := l.SymbolID(ctx, ws, "x.go", 1, 1); !errors.Is(err, ErrIndexErrored) {
		t.Errorf("SymbolID: want ErrIndexErrored, got %v", err)
	}
	if _, err := l.RankFiles(ctx, ws); !errors.Is(err, ErrIndexErrored) {
		t.Errorf("RankFiles: want ErrIndexErrored, got %v", err)
	}
	if _, err := l.RankFromSeeds(ctx, ws, []string{"a"}); !errors.Is(err, ErrIndexErrored) {
		t.Errorf("RankFromSeeds: want ErrIndexErrored, got %v", err)
	}
	if _, err := l.ExpandFrom(ctx, ws, SymbolID("s"), 2); !errors.Is(err, ErrIndexErrored) {
		t.Errorf("ExpandFrom: want ErrIndexErrored, got %v", err)
	}
	if _, err := l.ValidateCriticalEdges(ctx, ws, []Edge{{From: "a", To: "b"}}); !errors.Is(err, ErrIndexErrored) {
		t.Errorf("ValidateCriticalEdges: want ErrIndexErrored, got %v", err)
	}
	if _, err := l.Status(ctx, ws); !errors.Is(err, ErrIndexErrored) {
		t.Errorf("Status: want ErrIndexErrored, got %v", err)
	}
	if _, _, _, _, err := l.LocateSymbol(ctx, ws, SymbolID("s")); !errors.Is(err, ErrIndexErrored) {
		t.Errorf("LocateSymbol: want ErrIndexErrored, got %v", err)
	}
}

// TestNoopLookup_LocateSymbol_ReturnsErrIndexErrored pins Phase 65 65-12 Task 1
// — the NoopLookup default surfaces an ErrIndexErrored on LocateSymbol so the
// kernel-side analyze_blast_radius Pass-2 probe can recognize the
// "production lookup never wired" wiring bug and fall through to LSP fallback.
// The (path, line, col, ok) return must all be zero-valued on the
// ErrIndexErrored path so callers cannot accidentally consume a stale
// location.
func TestNoopLookup_LocateSymbol_ReturnsErrIndexErrored(t *testing.T) {
	var l SemanticLookup = NoopLookup{}
	path, line, col, ok, err := l.LocateSymbol(context.Background(), workspace.WorkspaceKey{}, SymbolID("anything"))
	if !errors.Is(err, ErrIndexErrored) {
		t.Errorf("err: want ErrIndexErrored, got %v", err)
	}
	if path != "" {
		t.Errorf("path: want empty, got %q", path)
	}
	if line != 0 {
		t.Errorf("line: want 0, got %d", line)
	}
	if col != 0 {
		t.Errorf("col: want 0, got %d", col)
	}
	if ok {
		t.Errorf("ok: want false, got true")
	}
}
