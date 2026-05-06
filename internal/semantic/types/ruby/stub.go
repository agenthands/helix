// Package ruby implements the Phase 62 P05 Ruby type-resolver stub.
//
// Per D-12 v1: Ruby has no LSP cascade and no static analyzer wired into
// Helix yet. The stub UNCONDITIONALLY emits confidence=0.20 +
// validation_state="unresolved" — never silently skips. A future v1.10.x
// phase will replace this stub with a full resolver (likely YARD-driven).
//
// PHP/Ruby are explicitly NOT LSP-conditional in v1 (D-12) — they always
// produce a 0.20 row regardless of incoming edge state.
package ruby

import (
	"context"

	"github.com/agenthands/helix/internal/semantic/types"
)

// Stub is the Ruby always-0.20 type-resolver stub.
type Stub struct{}

// NewStub constructs a Ruby stub. Takes no EffectiveReader — D-12 invariant.
func NewStub() *Stub { return &Stub{} }

// ResolveChain unconditionally returns the 0.20 unresolved response.
func (s *Stub) ResolveChain(_ context.Context, _ types.ChainRequest) (types.ChainResponse, error) {
	return types.ChainResponse{
		Resolved:        false,
		Confidence:      types.ConfidenceUnknown,
		EvidenceKind:    types.EvidenceUnknown,
		ValidationState: "unresolved",
		Source:          "stub.ruby",
		Reason:          "ruby: type resolution stub — full resolver deferred to v1.10.x",
	}, nil
}

// ResolveSymbol unconditionally returns the 0.20 unresolved response.
func (s *Stub) ResolveSymbol(_ context.Context, _ types.SymbolRequest) (types.SymbolResponse, error) {
	return types.SymbolResponse{
		Resolved:        false,
		Confidence:      types.ConfidenceUnknown,
		EvidenceKind:    types.EvidenceUnknown,
		ValidationState: "unresolved",
		Source:          "stub.ruby",
		Reason:          "ruby: type resolution stub — full resolver deferred to v1.10.x",
	}, nil
}
