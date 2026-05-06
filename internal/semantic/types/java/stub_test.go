package java

import (
	"context"
	"strings"
	"testing"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/types"
)

type fakeStore struct {
	edges map[graph.NodeID][]types.EdgeFact
}

func (f *fakeStore) QueryEffectiveEdges(_ context.Context, q types.EdgeQuery) ([]types.EdgeFact, error) {
	return f.edges[q.SrcNodeID], nil
}
func (f *fakeStore) QueryEffectiveSymbol(_ context.Context, _ string, n graph.NodeID) (types.SymbolFact, error) {
	return types.SymbolFact{NodeID: n}, nil
}

// TestJavaStub_LSPShortCircuit: Phase 61 LSP edge → resolved+1.0+validated.
func TestJavaStub_LSPShortCircuit(t *testing.T) {
	store := &fakeStore{
		edges: map[graph.NodeID][]types.EdgeFact{
			1: {{
				SrcNodeID: 1, DstNodeID: 99, EdgeKind: "RESOLVES_TO",
				Source: "lsp.java.text_document_definition",
				Confidence: 1.0, ValidationState: "validated",
			}},
		},
	}
	s := NewStub(store)
	resp, err := s.ResolveChain(context.Background(), types.ChainRequest{
		RepoID: "r", Language: "java", RefNodeID: 1, RefKind: "RESOLVES_TO",
		ChainTokens: []string{"x"},
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !resp.Resolved || resp.Confidence != 1.0 || resp.ValidationState != "validated" {
		t.Fatalf("resp = %+v, want resolved+1.0+validated", resp)
	}
	if !strings.HasPrefix(resp.Source, "lsp.") {
		t.Fatalf("Source = %q, want lsp.* prefix preserved", resp.Source)
	}
}

// TestJavaStub_NoLSPEmits020: no LSP edges → 0.20 unresolved.
func TestJavaStub_NoLSPEmits020(t *testing.T) {
	s := NewStub(&fakeStore{edges: map[graph.NodeID][]types.EdgeFact{}})
	resp, err := s.ResolveChain(context.Background(), types.ChainRequest{
		RepoID: "r", Language: "java", RefNodeID: 1,
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
	if resp.ValidationState != "unresolved" {
		t.Fatalf("ValidationState = %q, want unresolved", resp.ValidationState)
	}
	if !strings.Contains(strings.ToLower(resp.Reason), "java") ||
		!strings.Contains(strings.ToLower(resp.Reason), "no lsp") {
		t.Fatalf("Reason = %q, want substrings 'java' + 'no LSP'", resp.Reason)
	}
}

// TestJavaStub_ResolveSymbol_LSPShortCircuit: ResolveSymbol mirrors the
// short-circuit.
func TestJavaStub_ResolveSymbol_LSPShortCircuit(t *testing.T) {
	store := &fakeStore{
		edges: map[graph.NodeID][]types.EdgeFact{
			2: {{
				SrcNodeID: 2, DstNodeID: 200, EdgeKind: "RESOLVES_TO",
				Source: "lsp.java.text_document_definition",
				Confidence: 1.0, ValidationState: "validated",
			}},
		},
	}
	s := NewStub(store)
	resp, err := s.ResolveSymbol(context.Background(), types.SymbolRequest{
		RepoID: "r", Language: "java", SymbolNodeID: 2,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !resp.Resolved || resp.Confidence != 1.0 || resp.TypeNodeID != 200 {
		t.Fatalf("resp = %+v, want resolved+1.0 → 200", resp)
	}
}
