package rust

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
		Language:  "rust",
		RefNodeID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Resolved {
		t.Error("expected unresolved without LSP facts")
	}
}
