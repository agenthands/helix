package c

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/types"
)

// fakeStore is a minimal types.EffectiveReader for the C resolver tests: it
// returns canned edge / symbol facts indexed by RefNodeID.
type fakeStore struct {
	edgesByNode map[graph.NodeID][]types.EdgeFact
	symsByNode  map[graph.NodeID]types.SymbolFact
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		edgesByNode: map[graph.NodeID][]types.EdgeFact{},
		symsByNode:  map[graph.NodeID]types.SymbolFact{},
	}
}

func (f *fakeStore) QueryEffectiveEdges(_ context.Context, q types.EdgeQuery) ([]types.EdgeFact, error) {
	return f.edgesByNode[q.SrcNodeID], nil
}

func (f *fakeStore) QueryEffectiveSymbol(_ context.Context, _ string, n graph.NodeID) (types.SymbolFact, error) {
	if s, ok := f.symsByNode[n]; ok {
		return s, nil
	}
	return types.SymbolFact{NodeID: n}, nil
}

func resolve(t *testing.T, sym types.SymbolFact, edges []types.EdgeFact) types.ChainResponse {
	t.Helper()
	store := newFakeStore()
	store.symsByNode[10] = sym
	if edges != nil {
		store.edgesByNode[10] = edges
	}
	r := NewResolver(store)
	resp, err := r.ResolveChain(context.Background(), types.ChainRequest{
		RepoID: "repo", Language: "c", FilePath: "src/a.c",
		RefNodeID: 10, RefKind: "RESOLVES_TO",
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	return resp
}

// TestCResolver_LadderTiers walks each populated C tier (2/4/6), the LSP
// short-circuit (1), and the always-emit floor (7).
func TestCResolver_LadderTiers(t *testing.T) {
	cases := []struct {
		name      string
		sym       types.SymbolFact
		edges     []types.EdgeFact
		wantRes   bool
		wantConf  float64
		wantKind  types.EvidenceKind
		wantState string
	}{
		{
			name:      "T1_LSP_short_circuit",
			sym:       types.SymbolFact{NodeID: 10, Signature: "struct Foo x"},
			edges:     []types.EdgeFact{{SrcNodeID: 10, DstNodeID: 99, EdgeKind: "RESOLVES_TO", Source: "lsp.textDocument", Confidence: 1.0}},
			wantRes:   true,
			wantConf:  types.ConfidenceLSP,
			wantKind:  types.EvidenceLSP,
			wantState: "validated",
		},
		{
			name:      "T2_annotation_struct_tag",
			sym:       types.SymbolFact{NodeID: 10, Signature: "struct Foo x"},
			wantRes:   true,
			wantConf:  types.ConfidenceAnnotation,
			wantKind:  types.EvidenceAnnotation,
			wantState: "validated",
		},
		{
			name:      "T2_annotation_pointer",
			sym:       types.SymbolFact{NodeID: 10, Signature: "Widget *w"},
			wantRes:   true,
			wantConf:  types.ConfidenceAnnotation,
			wantKind:  types.EvidenceAnnotation,
			wantState: "validated",
		},
		{
			name:      "T2_annotation_qualified_with_init",
			sym:       types.SymbolFact{NodeID: 10, Signature: "const Session s = make()"},
			wantRes:   true,
			wantConf:  types.ConfidenceAnnotation,
			wantKind:  types.EvidenceAnnotation,
			wantState: "validated",
		},
		{
			name:      "T4_assignment",
			sym:       types.SymbolFact{NodeID: 10, Signature: "y = x"},
			wantRes:   true,
			wantConf:  types.ConfidenceAssignment,
			wantKind:  types.EvidenceAssignment,
			wantState: "validated",
		},
		{
			name:      "T6_heuristic_name_shape",
			sym:       types.SymbolFact{NodeID: 10, StableKey: "userCfg"},
			wantRes:   true,
			wantConf:  types.ConfidenceHeuristic,
			wantKind:  types.EvidenceHeuristic,
			wantState: "validated",
		},
		{
			name:      "T7_unknown_always_emit",
			sym:       types.SymbolFact{NodeID: 10, Signature: "", StableKey: "opaque_thing"},
			wantRes:   false,
			wantConf:  types.ConfidenceUnknown,
			wantKind:  types.EvidenceUnknown,
			wantState: "unresolved",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := resolve(t, tc.sym, tc.edges)
			if resp.Resolved != tc.wantRes {
				t.Errorf("Resolved = %v, want %v (%+v)", resp.Resolved, tc.wantRes, resp)
			}
			if resp.Confidence != tc.wantConf {
				t.Errorf("Confidence = %v, want %v", resp.Confidence, tc.wantConf)
			}
			if resp.EvidenceKind != tc.wantKind {
				t.Errorf("EvidenceKind = %q, want %q", resp.EvidenceKind, tc.wantKind)
			}
			if resp.ValidationState != tc.wantState {
				t.Errorf("ValidationState = %q, want %q", resp.ValidationState, tc.wantState)
			}
		})
	}
}

// TestCResolver_NoConstructorTier proves C has NO constructor tier: a
// `NewFoo()`-style initializer (which Go would resolve at Tier 3) falls
// through to Tier 7 unresolved for C.
func TestCResolver_NoConstructorTier(t *testing.T) {
	resp := resolve(t, types.SymbolFact{NodeID: 10, Signature: "x = NewFoo()"}, nil)
	if resp.Resolved {
		t.Fatalf("C resolved a constructor-style init (%+v); C has no constructor tier — want Tier 7", resp)
	}
	if resp.EvidenceKind != types.EvidenceUnknown {
		t.Errorf("EvidenceKind = %q, want unknown (no ctor tier in C)", resp.EvidenceKind)
	}
}

// TestCResolver_LadderPrecedence: a typed declaration with an assignment
// resolves at annotation (T2), NOT assignment (T4) — the ladder is top-down.
func TestCResolver_LadderPrecedence(t *testing.T) {
	resp := resolve(t, types.SymbolFact{NodeID: 10, Signature: "Gadget g = other"}, nil)
	if resp.EvidenceKind != types.EvidenceAnnotation {
		t.Fatalf("typed-decl-with-init resolved at %q, want annotation (T2 wins over T4)", resp.EvidenceKind)
	}
}

// TestCResolver_ParseAnnotation is a pure-parser table for the C declarator
// grammar the annotation tier recognises.
func TestCResolver_ParseAnnotation(t *testing.T) {
	cases := []struct{ sig, want string }{
		{"struct Foo x", "Foo"},
		{"union U u", "U"},
		{"enum Color c", "Color"},
		{"Widget *w", "Widget"},
		{"const Session s", "Session"},
		{"static struct Node *n", "Node"},
		{"Gadget g = init", "Gadget"},
		{"int x", "int"},
		{"", ""},
		{"x = y", ""},        // bare assignment — not a typed decl
		{"lonely", ""},       // single identifier — no declarator
		{"x = NewFoo()", ""}, // constructor-style — not a typed decl
	}
	for _, tc := range cases {
		if got := parseAnnotation(tc.sig); got != tc.want {
			t.Errorf("parseAnnotation(%q) = %q, want %q", tc.sig, got, tc.want)
		}
	}
}

// TestCResolver_ParseAssignment covers the bare-assignment RHS extraction.
func TestCResolver_ParseAssignment(t *testing.T) {
	cases := []struct{ sig, want string }{
		{"y = x", "x"},
		{"a = bee", "bee"},
		{"x = f()", ""},   // function call RHS — rejected
		{"Foo x = y", ""}, // typed decl (two-token LHS) — not a bare assign
		{"noequals", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := parseAssignment(tc.sig); got != tc.want {
			t.Errorf("parseAssignment(%q) = %q, want %q", tc.sig, got, tc.want)
		}
	}
}

// TestCResolver_Heuristic covers the name-shape suffix rules + determinism.
func TestCResolver_Heuristic(t *testing.T) {
	cases := []struct{ name, want string }{
		{"userCfg", "UserConfig"},
		{"dbConn", "DbConnection"},
		{"reqCtx", "ReqContext"},
		{"Ctx", ""}, // bare suffix, no prefix → no match (needs prefixed ident)
		{"plainname", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := guessFromName(tc.name); got != tc.want {
			t.Errorf("guessFromName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestCResolver_FlatScope: C scope is program-wide (no intra-package cap).
func TestCResolver_FlatScope(t *testing.T) {
	if !SameScope("src/a.c", "src/other/b.c") {
		t.Fatalf("SameScope across C files = false; C linkage is program-wide (M1)")
	}
	if TranslationUnitScope("anything.c") != "" {
		t.Fatalf("TranslationUnitScope != flat empty")
	}
}

// TestCResolver_ResolveSymbol: the symbol path delegates to the same ladder.
func TestCResolver_ResolveSymbol(t *testing.T) {
	store := newFakeStore()
	store.symsByNode[10] = types.SymbolFact{NodeID: 10, Signature: "struct Foo x"}
	r := NewResolver(store)
	resp, err := r.ResolveSymbol(context.Background(), types.SymbolRequest{
		RepoID: "repo", Language: "c", FilePath: "src/a.c", SymbolNodeID: 10,
	})
	if err != nil {
		t.Fatalf("ResolveSymbol err: %v", err)
	}
	if !resp.Resolved || resp.EvidenceKind != types.EvidenceAnnotation {
		t.Fatalf("ResolveSymbol = %+v, want resolved annotation", resp)
	}
}

// TestCResolver_ChainTokensAnnotation (v2.12 Phase 136 A1): a ChainRequest
// with ChainTokens:["Foo"] + a typeIndex mapping "Foo"→N resolves at the
// annotation tier to Target=N WITHOUT relying on the Signature (the co-driver
// supplies the declared type). This is the production producer's path.
func TestCResolver_ChainTokensAnnotation(t *testing.T) {
	store := newFakeStore()
	// The referencing symbol carries a BARE signature (no parseable type) —
	// only ChainTokens can supply "Foo".
	store.symsByNode[10] = types.SymbolFact{NodeID: 10, Signature: "p", FilePath: "src/a.c"}
	store.symsByNode[42] = types.SymbolFact{NodeID: 42, Kind: "struct", StableKey: "Foo", FilePath: "src/a.c"}
	r := NewResolverWithIndex(store, map[string]graph.NodeID{"Foo": 42})
	resp, err := r.ResolveChain(context.Background(), types.ChainRequest{
		RepoID: "repo", Language: "c", FilePath: "src/a.c",
		RefNodeID: 10, RefKind: "RESOLVES_TO", ChainTokens: []string{"Foo"},
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	if !resp.Resolved {
		t.Fatalf("ChainTokens annotation did not resolve: %+v", resp)
	}
	if resp.EvidenceKind != types.EvidenceAnnotation || resp.Confidence != types.ConfidenceAnnotation {
		t.Errorf("tier = %q/%v, want annotation/0.90", resp.EvidenceKind, resp.Confidence)
	}
	if resp.Target != 42 {
		t.Errorf("Target = %d, want 42 (via typeIndex, not Signature)", resp.Target)
	}
}

// TestCResolver_NewResolverWithIndex (A2): the constructor seam populates the
// index and a resolve uses it for Target.
func TestCResolver_NewResolverWithIndex(t *testing.T) {
	store := newFakeStore()
	store.symsByNode[10] = types.SymbolFact{NodeID: 10, Signature: "struct Bar b", FilePath: "src/a.c"}
	store.symsByNode[7] = types.SymbolFact{NodeID: 7, Kind: "struct", StableKey: "Bar", FilePath: "src/a.c"}
	r := NewResolverWithIndex(store, map[string]graph.NodeID{"Bar": 7})
	// No ChainTokens → Signature fallback ("struct Bar b" → "Bar") + index target.
	resp, err := r.ResolveChain(context.Background(), types.ChainRequest{
		RepoID: "repo", Language: "c", FilePath: "src/a.c",
		RefNodeID: 10, RefKind: "RESOLVES_TO",
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	if !resp.Resolved || resp.Target != 7 {
		t.Fatalf("NewResolverWithIndex resolve = %+v, want resolved Target=7", resp)
	}
}
