package c_sharp

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/types"
)

type nilReader struct{}

func (nilReader) QueryEffectiveEdges(context.Context, types.EdgeQuery) ([]types.EdgeFact, error) {
	return nil, nil
}
func (nilReader) QueryEffectiveSymbol(_ context.Context, _ string, _ graph.NodeID) (types.SymbolFact, error) {
	return types.SymbolFact{}, nil
}

func TestStub_UnresolvedWithoutLSP(t *testing.T) {
	s := NewStub(nilReader{})
	resp, err := s.ResolveChain(context.Background(), types.ChainRequest{
		Language:  "c_sharp",
		RefNodeID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Resolved {
		t.Error("expected unresolved without LSP facts")
	}
	if resp.Confidence != types.ConfidenceUnknown {
		t.Errorf("confidence = %v, want %v", resp.Confidence, types.ConfidenceUnknown)
	}
	if resp.ValidationState != "unresolved" {
		t.Errorf("validation = %q, want unresolved", resp.ValidationState)
	}
}

func TestStub_ResolveSymbolUnresolved(t *testing.T) {
	s := NewStub(nilReader{})
	resp, err := s.ResolveSymbol(context.Background(), types.SymbolRequest{
		Language:     "c_sharp",
		SymbolNodeID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Resolved {
		t.Error("expected unresolved without LSP facts")
	}
}
