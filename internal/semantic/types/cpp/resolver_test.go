package cpp

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/types"
)

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
		RepoID: "repo", Language: "cpp", FilePath: "src/a.cpp",
		RefNodeID: 10, RefKind: "RESOLVES_TO",
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	return resp
}

// TestCppResolver_LadderTiers walks each populated C++ tier (2/3/4/6), the LSP
// short-circuit (1), and the always-emit floor (7).
func TestCppResolver_LadderTiers(t *testing.T) {
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
			name:      "T1_LSP",
			sym:       types.SymbolFact{NodeID: 10, Signature: "Foo x"},
			edges:     []types.EdgeFact{{SrcNodeID: 10, DstNodeID: 99, EdgeKind: "RESOLVES_TO", Source: "lsp.textDocument", Confidence: 1.0}},
			wantRes:   true,
			wantConf:  types.ConfidenceLSP,
			wantKind:  types.EvidenceLSP,
			wantState: "validated",
		},
		{
			name:      "T2_annotation_explicit_type",
			sym:       types.SymbolFact{NodeID: 10, Signature: "Foo x"},
			wantRes:   true,
			wantConf:  types.ConfidenceAnnotation,
			wantKind:  types.EvidenceAnnotation,
			wantState: "validated",
		},
		{
			name:      "T2_annotation_brace_init",
			sym:       types.SymbolFact{NodeID: 10, Signature: "Widget w{5}"},
			wantRes:   true,
			wantConf:  types.ConfidenceAnnotation,
			wantKind:  types.EvidenceAnnotation,
			wantState: "validated",
		},
		{
			name:      "T2_annotation_template_qualified",
			sym:       types.SymbolFact{NodeID: 10, Signature: "std::vector<int> v"},
			wantRes:   true,
			wantConf:  types.ConfidenceAnnotation,
			wantKind:  types.EvidenceAnnotation,
			wantState: "validated",
		},
		{
			name:      "T3_constructor_auto_brace",
			sym:       types.SymbolFact{NodeID: 10, Signature: "auto x = Gadget{}"},
			wantRes:   true,
			wantConf:  types.ConfidenceConstructor,
			wantKind:  types.EvidenceConstructor,
			wantState: "validated",
		},
		{
			name:      "T3_constructor_heap_new",
			sym:       types.SymbolFact{NodeID: 10, Signature: "auto p = new Session()"},
			wantRes:   true,
			wantConf:  types.ConfidenceConstructor,
			wantKind:  types.EvidenceConstructor,
			wantState: "validated",
		},
		{
			name:      "T3_constructor_bare_new",
			sym:       types.SymbolFact{NodeID: 10, Signature: "new Thing()"},
			wantRes:   true,
			wantConf:  types.ConfidenceConstructor,
			wantKind:  types.EvidenceConstructor,
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
			name:      "T6_heuristic",
			sym:       types.SymbolFact{NodeID: 10, StableKey: "dbConn"},
			wantRes:   true,
			wantConf:  types.ConfidenceHeuristic,
			wantKind:  types.EvidenceHeuristic,
			wantState: "validated",
		},
		{
			name:      "T7_unknown_always_emit",
			sym:       types.SymbolFact{NodeID: 10, Signature: "", StableKey: "x"},
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

// TestCppResolver_LadderPrecedence: an explicit-type ctor init resolves at
// annotation (T2), not constructor (T3) — the type is written explicitly.
func TestCppResolver_LadderPrecedence(t *testing.T) {
	resp := resolve(t, types.SymbolFact{NodeID: 10, Signature: "Foo x = Bar()"}, nil)
	if resp.EvidenceKind != types.EvidenceAnnotation {
		t.Fatalf("explicit-type-with-ctor-init resolved at %q, want annotation (T2 wins)", resp.EvidenceKind)
	}
}

// TestCppResolver_NormalizeType covers template-arg + namespace-qualifier
// stripping shared across tiers.
func TestCppResolver_NormalizeType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Foo", "Foo"},
		{"ns::Foo", "Foo"},
		{"std::vector<int>", "vector"},
		{"ns::Widget<T, U>", "Widget"},
		{"Foo*", "Foo"},
		{"Foo&", "Foo"},
		{"a::b::c::Deep", "Deep"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeCppType(tc.in); got != tc.want {
			t.Errorf("normalizeCppType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestCppResolver_ParseAnnotation covers the explicit typed-declaration grammar.
func TestCppResolver_ParseAnnotation(t *testing.T) {
	cases := []struct{ sig, want string }{
		{"Foo x", "Foo"},
		{"Foo *x", "Foo"},
		{"Foo* x", "Foo"},
		{"const Foo& r", "Foo"},
		{"std::vector<int> v", "vector"},
		{"Widget w{5}", "Widget"},
		{"static constexpr Bar b", "Bar"},
		{"auto x = Foo{}", ""},   // auto → constructor tier
		{"new Thing()", ""},      // new → constructor tier
		{"x = y", ""},            // bare assignment
		{"lonely", ""},           // single identifier
	}
	for _, tc := range cases {
		if got := parseAnnotation(tc.sig); got != tc.want {
			t.Errorf("parseAnnotation(%q) = %q, want %q", tc.sig, got, tc.want)
		}
	}
}

// TestCppResolver_ParseConstructor covers new/auto-ctor derivation.
func TestCppResolver_ParseConstructor(t *testing.T) {
	cases := []struct{ sig, want string }{
		{"auto x = Gadget{}", "Gadget"},
		{"auto x = Factory(1, 2)", "Factory"},
		{"auto p = new Session()", "Session"},
		{"new Thing()", "Thing"},
		{"new ns::Widget<T>()", "Widget"},
		{"Foo x", ""},   // explicit type — annotation tier
		{"y = x", ""},   // assignment
		{"", ""},
	}
	for _, tc := range cases {
		if got := parseConstructor(tc.sig); got != tc.want {
			t.Errorf("parseConstructor(%q) = %q, want %q", tc.sig, got, tc.want)
		}
	}
}

// TestCppResolver_ParseAssignment covers bare-assignment RHS extraction.
func TestCppResolver_ParseAssignment(t *testing.T) {
	cases := []struct{ sig, want string }{
		{"y = x", "x"},
		{"a = bee", "bee"},
		{"x = f()", ""},           // call RHS
		{"x = Foo{}", ""},         // brace ctor
		{"auto x = y", ""},        // auto decl
		{"p = new Foo()", ""},     // new expression
		{"Foo x = y", ""},         // typed decl (two-token LHS)
		{"", ""},
	}
	for _, tc := range cases {
		if got := parseAssignment(tc.sig); got != tc.want {
			t.Errorf("parseAssignment(%q) = %q, want %q", tc.sig, got, tc.want)
		}
	}
}

// TestCppResolver_Heuristic covers suffix rules + PascalCase self-match +
// template/qualifier normalization of the name.
func TestCppResolver_Heuristic(t *testing.T) {
	cases := []struct{ name, want string }{
		{"dbConn", "DbConnection"},
		{"reqCtx", "ReqContext"},
		{"Widget", "Widget"},          // PascalCase self-match
		{"ns::Widget", "Widget"},      // qualifier stripped, then self-match
		{"MAX", ""},                   // ALL_CAPS macro-style — no lower, no match
		{"lowercase", ""},             // not PascalCase, no suffix
		{"", ""},
	}
	for _, tc := range cases {
		if got := guessFromName(tc.name); got != tc.want {
			t.Errorf("guessFromName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestCppResolver_FlatScope: C++ scope is program-wide (no intra-package cap).
func TestCppResolver_FlatScope(t *testing.T) {
	if !SameScope("src/a.cpp", "src/other/b.cpp") {
		t.Fatalf("SameScope across C++ files = false; C++ TU/header linkage is program-wide (M1)")
	}
}

// TestCppResolver_ResolveSymbol: the symbol path delegates to the same ladder.
func TestCppResolver_ResolveSymbol(t *testing.T) {
	store := newFakeStore()
	store.symsByNode[10] = types.SymbolFact{NodeID: 10, Signature: "auto p = new Session()"}
	r := NewResolver(store)
	resp, err := r.ResolveSymbol(context.Background(), types.SymbolRequest{
		RepoID: "repo", Language: "cpp", FilePath: "src/a.cpp", SymbolNodeID: 10,
	})
	if err != nil {
		t.Fatalf("ResolveSymbol err: %v", err)
	}
	if !resp.Resolved || resp.EvidenceKind != types.EvidenceConstructor {
		t.Fatalf("ResolveSymbol = %+v, want resolved constructor", resp)
	}
}
