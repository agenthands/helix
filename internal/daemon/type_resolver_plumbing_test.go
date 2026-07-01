package daemon

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/graph"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/semantic/types"
)

// TestTypeResolverPlumbing_RealAdapterUnblocksTiers is the v2.11 Phase 130
// proof (red-team B1): with QueryEffectiveSymbolFact wired, the REAL go resolver
// driven through the REAL daemon adapter against the REAL store produces a
// tier-2 (annotation) edge for a committed typed-decl symbol — NOT the Tier-7
// collapse the empty-adapter stub caused. Before Phase 130 this returned
// 0.20/unresolved (the adapter returned SymbolFact{NodeID} with no Signature).
func TestTypeResolverPlumbing_RealAdapterUnblocksTiers(t *testing.T) {
	const sigNode = uint64(0x4000_0000_0000_0001) & 0x7FFFFFFFFFFFFFFF
	factsFn := func(repoID string) semanticstore.Facts {
		files := []semanticstore.FileFact{{FileID: 1, RepoID: repoID, Path: "src/f.go", Language: "go", ContentHash: "h", SizeBytes: 50, LineCount: 5}}
		symbols := []semanticstore.SymbolFact{{
			SymbolID: sigNode, NodeID: sigNode, FileID: 1, Language: "go",
			Kind: "variable", Name: "x", QualifiedName: "x", StableKey: "sk:x",
			Signature: "var x Foo", StartLine: 1, StartCol: 1, EndLine: 1, EndCol: 12,
			Visibility: "private", Confidence: 1.0,
		}}
		return semanticstore.Facts{Files: files, Symbols: symbols}
	}
	store, repoID, cleanup := openDataFlowStore(t, factsFn)
	defer cleanup()

	// First assert the PLUMBING directly: the adapter now returns the real signature.
	adapter := newTypeStoreAdapter(store)
	sf, err := adapter.QueryEffectiveSymbol(context.Background(), repoID, graph.NodeID(sigNode))
	if err != nil {
		t.Fatalf("adapter.QueryEffectiveSymbol: %v", err)
	}
	if sf.Signature != "var x Foo" {
		t.Fatalf("Phase 130 plumbing failed: adapter returned Signature=%q (empty = the B1 stub still in place); want \"var x Foo\"", sf.Signature)
	}

	// Then the END-TO-END proof: the real go resolver, wired to the real adapter,
	// resolves the typed decl at Tier 2 (annotation), not Tier 7.
	resolver := goTypeResolver(adapter)
	resp, err := resolver.ResolveChain(context.Background(), types.ChainRequest{
		RepoID:      repoID,
		Language:    "go",
		FilePath:    "src/f.go",
		RefNodeID:   graph.NodeID(sigNode),
		RefKind:     "RESOLVES_TO",
		ChainTokens: []string{"x"},
	})
	if err != nil {
		t.Fatalf("ResolveChain: %v", err)
	}
	if !resp.Resolved {
		t.Fatalf("Phase 130: go resolver returned unresolved (Tier 7) for a typed-decl symbol with a real signature — the tier ladder is still starved: %+v", resp)
	}
	if resp.EvidenceKind != types.EvidenceAnnotation {
		t.Errorf("EvidenceKind = %q, want %q (annotation; the typed-decl tier)", resp.EvidenceKind, types.EvidenceAnnotation)
	}
	if resp.Confidence != types.ConfidenceAnnotation {
		t.Errorf("Confidence = %v, want %v (annotation tier)", resp.Confidence, types.ConfidenceAnnotation)
	}
}

// TestTypeResolverPlumbing_UnknownNodeDegrades asserts the D-12 invariant still
// holds: a node with no committed symbol degrades to NodeID-only (Tier 7), not
// an error — the resolver still emits (worst case 0.20 unresolved).
func TestTypeResolverPlumbing_UnknownNodeDegrades(t *testing.T) {
	store, repoID, cleanup := openDataFlowStore(t, func(string) semanticstore.Facts {
		return semanticstore.Facts{} // empty — no symbols
	})
	defer cleanup()
	adapter := newTypeStoreAdapter(store)
	sf, err := adapter.QueryEffectiveSymbol(context.Background(), repoID, graph.NodeID(999))
	if err != nil {
		t.Fatalf("unknown node: error %v (must degrade, not error)", err)
	}
	if sf.Signature != "" {
		t.Errorf("unknown node: Signature=%q, want empty (no row)", sf.Signature)
	}
	resolver := goTypeResolver(adapter)
	resp, _ := resolver.ResolveChain(context.Background(), types.ChainRequest{
		RepoID: repoID, Language: "go", FilePath: "src/f.go",
		RefNodeID: graph.NodeID(999), RefKind: "RESOLVES_TO", ChainTokens: []string{"x"},
	})
	if resp.Resolved {
		t.Errorf("unknown node: resolved=true, want Tier-7 unresolved: %+v", resp)
	}
	if resp.ValidationState != "unresolved" {
		t.Errorf("unknown node: ValidationState=%q, want unresolved", resp.ValidationState)
	}
}
