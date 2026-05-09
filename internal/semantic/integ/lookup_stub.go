// lookup_stub.go — UnsupportedSemanticLookup mixin (Phase 66 Plan 02).
//
// UnsupportedSemanticLookup implements only the Phase 66 methods added to
// SemanticLookup (Visibility, IsEntrypointReachable) returning the typed
// ErrUnsupported sentinel. Existing test fakes embed this struct so they
// gain the new interface methods without being hand-edited.
//
// Usage in test fakes:
//
//	type myFakeLookup struct {
//	    integ.UnsupportedSemanticLookup
//	    // existing fake fields ...
//	}
//
// Production adapters (integSemanticLookup in internal/daemon/semantic_wiring.go)
// implement Visibility and IsEntrypointReachable directly using the semantic store.
package integ

import (
	"context"

	"github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/workspace"
)

// UnsupportedSemanticLookup is a zero-value embeddable struct that provides
// stub implementations of the Phase 66 SemanticLookup extension methods.
// Both methods return the typed ErrUnsupported sentinel so callers can detect
// the degraded state via errors.Is(err, serr.ErrUnsupported).
//
// Existing test doubles that embed this struct automatically satisfy the
// extended SemanticLookup interface without source changes.
type UnsupportedSemanticLookup struct{}

// Visibility returns (VisUnknown, ErrUnsupported).
// Callers should treat VisUnknown as a conservative-warn signal (D-19).
func (UnsupportedSemanticLookup) Visibility(_ context.Context, _ workspace.WorkspaceKey, _ SymbolID) (Visibility, error) {
	return VisUnknown, errors.ErrUnsupported
}

// IsEntrypointReachable returns (false, ErrUnsupported).
// Callers should treat false-with-error as a degraded-mode signal (D-19).
func (UnsupportedSemanticLookup) IsEntrypointReachable(_ context.Context, _ workspace.WorkspaceKey, _ SymbolID) (bool, error) {
	return false, errors.ErrUnsupported
}
