package types

import (
	"context"
	"strings"
	"testing"
)

// TestResolveAccessChain_DepthBound: a 9-token chain exceeds max_chain_depth=8
// and emits an unresolved response with reason citing the depth bound.
func TestResolveAccessChain_DepthBound(t *testing.T) {
	tokens := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	// Resolver that ALWAYS resolves at LSP. With depth bound respected, this
	// is irrelevant — chain.go must short-circuit BEFORE the first hop.
	fake := &fakeResolver{resolveChain: []ChainResponse{{
		Resolved: true, Confidence: ConfidenceLSP, EvidenceKind: EvidenceLSP,
		ValidationState: "validated",
	}}}
	resp, err := ResolveAccessChain(context.Background(), fake, ChainRequest{
		RepoID: "r", Language: "go", ChainTokens: tokens,
	}, 8)
	if err != nil {
		t.Fatalf("ResolveAccessChain err: %v", err)
	}
	if resp.Resolved {
		t.Fatalf("9-token chain should be unresolved, got %+v", resp)
	}
	if resp.Confidence > ConfidenceComment {
		t.Fatalf("over-depth response confidence = %v, must be ≤ %v", resp.Confidence, ConfidenceComment)
	}
	if !strings.Contains(strings.ToLower(resp.Reason), "max") &&
		!strings.Contains(strings.ToLower(resp.Reason), "deep") {
		t.Fatalf("resp.Reason = %q, want max/deep substring", resp.Reason)
	}
}

// TestResolveAccessChain_EarlyExit: per-hop resolver returns Resolved=false at
// the second hop; chain stops and returns the last (failing) response without
// continuing into hops 3+.
func TestResolveAccessChain_EarlyExit(t *testing.T) {
	fake := &fakeResolver{resolveChain: []ChainResponse{
		{Resolved: true, Confidence: ConfidenceLSP, EvidenceKind: EvidenceLSP, Target: 1, ValidationState: "validated"},
		{Resolved: false, Confidence: ConfidenceUnknown, EvidenceKind: EvidenceUnknown, ValidationState: "unresolved"},
	}}
	resp, err := ResolveAccessChain(context.Background(), fake, ChainRequest{
		RepoID: "r", Language: "go", ChainTokens: []string{"a", "b", "c"},
	}, 8)
	if err != nil {
		t.Fatalf("ResolveAccessChain err: %v", err)
	}
	if resp.Resolved {
		t.Fatalf("expected stop on hop 2 unresolved, got %+v", resp)
	}
	if fake.calls.Load() != 2 {
		t.Fatalf("fake.calls = %d, want 2 (hops 1 and 2)", fake.calls.Load())
	}
}

// TestResolveAccessChain_FullResolve: 4 tokens, all hops resolve at LSP →
// final response carries LSP confidence + validated state.
func TestResolveAccessChain_FullResolve(t *testing.T) {
	fake := &fakeResolver{resolveChain: []ChainResponse{{
		Resolved: true, Confidence: ConfidenceLSP, EvidenceKind: EvidenceLSP,
		Target: 42, ValidationState: "validated", Source: "lsp.go.text_document_definition",
	}}}
	resp, err := ResolveAccessChain(context.Background(), fake, ChainRequest{
		RepoID: "r", Language: "go", ChainTokens: []string{"a", "b", "c", "d"},
	}, 8)
	if err != nil {
		t.Fatalf("ResolveAccessChain err: %v", err)
	}
	if !resp.Resolved {
		t.Fatalf("expected resolved, got %+v", resp)
	}
	if resp.Confidence != ConfidenceLSP {
		t.Fatalf("resp.Confidence = %v, want %v", resp.Confidence, ConfidenceLSP)
	}
	if resp.ValidationState != "validated" {
		t.Fatalf("resp.ValidationState = %q, want validated", resp.ValidationState)
	}
	if fake.calls.Load() != 4 {
		t.Fatalf("fake.calls = %d, want 4 (one per token)", fake.calls.Load())
	}
}

// TestResolveAccessChain_EmptyTokens: zero-token chain returns an unresolved
// response (defensive — callers should not invoke with empty tokens).
func TestResolveAccessChain_EmptyTokens(t *testing.T) {
	fake := &fakeResolver{}
	resp, err := ResolveAccessChain(context.Background(), fake, ChainRequest{
		RepoID: "r", Language: "go", ChainTokens: nil,
	}, 8)
	if err != nil {
		t.Fatalf("ResolveAccessChain err: %v", err)
	}
	if resp.Resolved {
		t.Fatalf("empty chain should be unresolved, got %+v", resp)
	}
	if fake.calls.Load() != 0 {
		t.Fatalf("empty chain should not call resolver, calls = %d", fake.calls.Load())
	}
}
