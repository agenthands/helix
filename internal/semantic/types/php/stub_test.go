package php

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/types"
)

// TestPHPStub_AlwaysEmits020: any input → 0.20 unresolved.
func TestPHPStub_AlwaysEmits020(t *testing.T) {
	s := NewStub()
	resp, err := s.ResolveChain(context.Background(), types.ChainRequest{
		RepoID: "r", Language: "php", RefNodeID: 1,
		ChainTokens: []string{"x"},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Resolved {
		t.Fatalf("Resolved = true, want false")
	}
	if resp.Confidence != types.ConfidenceUnknown {
		t.Fatalf("Confidence = %v, want 0.20", resp.Confidence)
	}
	if resp.EvidenceKind != types.EvidenceUnknown {
		t.Fatalf("EvidenceKind = %q, want unknown", resp.EvidenceKind)
	}
	if resp.ValidationState != "unresolved" {
		t.Fatalf("ValidationState = %q, want unresolved", resp.ValidationState)
	}
}

// TestPHPStub_ResolveSymbolEmits020: same shape on ResolveSymbol.
func TestPHPStub_ResolveSymbolEmits020(t *testing.T) {
	s := NewStub()
	resp, err := s.ResolveSymbol(context.Background(), types.SymbolRequest{
		RepoID: "r", Language: "php", SymbolNodeID: 1,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Resolved {
		t.Fatalf("Resolved = true, want false")
	}
	if resp.Confidence != types.ConfidenceUnknown {
		t.Fatalf("Confidence = %v, want 0.20", resp.Confidence)
	}
	if resp.ValidationState != "unresolved" {
		t.Fatalf("ValidationState = %q, want unresolved", resp.ValidationState)
	}
}
