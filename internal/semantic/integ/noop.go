// noop.go — NoopLookup is the SemanticLookup implementation used as the
// pre-wiring default. Setter post-init wiring (Pattern 1) replaces the
// NoopLookup with a production integSemanticLookup once the daemon's
// semanticBundle finishes constructing.
//
// Contract (RESEARCH Code Example 4):
//   - Available() returns false unconditionally.
//   - Every other method returns ErrIndexErrored.
//
// Returning ErrIndexErrored (rather than ErrNoSnapshot or
// ErrIndexDisabled) is a deliberate choice: NoopLookup means "the
// production lookup has not been wired", which is operationally a wiring
// bug — surfaceable but not the steady-state "config off" path. Consumers
// pair the error with their own SourceTreeSitter / SourceFallback decision.
package integ

import (
	"context"

	"github.com/agenthands/helix/internal/workspace"
)

// NoopLookup is the zero-value SemanticLookup default.
type NoopLookup struct{}

// Available reports false unconditionally.
func (NoopLookup) Available() bool { return false }

// SymbolID returns ("", ErrIndexErrored).
func (NoopLookup) SymbolID(_ context.Context, _ workspace.WorkspaceKey, _ string, _, _ uint32) (SymbolID, error) {
	return SymbolID(""), ErrIndexErrored
}

// RankFiles returns (nil, ErrIndexErrored).
func (NoopLookup) RankFiles(_ context.Context, _ workspace.WorkspaceKey) ([]RankedFile, error) {
	return nil, ErrIndexErrored
}

// RankFromSeeds returns (nil, ErrIndexErrored).
func (NoopLookup) RankFromSeeds(_ context.Context, _ workspace.WorkspaceKey, _ []string) ([]RankedFile, error) {
	return nil, ErrIndexErrored
}

// ExpandFrom returns (nil, ErrIndexErrored).
func (NoopLookup) ExpandFrom(_ context.Context, _ workspace.WorkspaceKey, _ SymbolID, _ int) ([]Impact, error) {
	return nil, ErrIndexErrored
}

// ValidateCriticalEdges returns (nil, ErrIndexErrored).
func (NoopLookup) ValidateCriticalEdges(_ context.Context, _ workspace.WorkspaceKey, _ []Edge) ([]ValidatedEdge, error) {
	return nil, ErrIndexErrored
}

// Status returns (zero-value, ErrIndexErrored). The zero value is safe to
// log; consumers must check the error before consulting fields.
func (NoopLookup) Status(_ context.Context, _ workspace.WorkspaceKey) (SemanticStatus, error) {
	return SemanticStatus{}, ErrIndexErrored
}

// LocateSymbol returns ("", 0, 0, false, ErrIndexErrored). Phase 65 65-12 Task 1
// — the production lookup has not been wired; the kernel-side LSP probe must
// see this as a wiring bug, not a steady-state miss.
func (NoopLookup) LocateSymbol(_ context.Context, _ workspace.WorkspaceKey, _ SymbolID) (string, uint32, uint32, bool, error) {
	return "", 0, 0, false, ErrIndexErrored
}
