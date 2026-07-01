package c_sharp

import (
	"context"
	"strings"
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
		RepoID: "repo", Language: "c_sharp", FilePath: "src/a/f.cs",
		RefNodeID: 10, RefKind: "RESOLVES_TO",
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	return resp
}

// TestCSharpResolver_LadderTiers walks each populated C# tier (2/3/4/6), LSP
// (1), and the always-emit floor (7). Note (red-team B3): this file REPLACES
// the removed stub_test.go — the T7 case preserves its no-LSP-unresolved
// coverage.
func TestCSharpResolver_LadderTiers(t *testing.T) {
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
			name:      "T2_annotation_typed_decl",
			sym:       types.SymbolFact{NodeID: 10, Signature: "private readonly Foo _x"},
			wantRes:   true,
			wantConf:  types.ConfidenceAnnotation,
			wantKind:  types.EvidenceAnnotation,
			wantState: "validated",
		},
		{
			name:      "T2_annotation_attribute_stripped",
			sym:       types.SymbolFact{NodeID: 10, Signature: "[XmlAttribute] public Widget W"},
			wantRes:   true,
			wantConf:  types.ConfidenceAnnotation,
			wantKind:  types.EvidenceAnnotation,
			wantState: "validated",
		},
		{
			name:      "T2_annotation_generic",
			sym:       types.SymbolFact{NodeID: 10, Signature: "List<Widget> items"},
			wantRes:   true,
			wantConf:  types.ConfidenceAnnotation,
			wantKind:  types.EvidenceAnnotation,
			wantState: "validated",
		},
		{
			name:      "T3_constructor_new",
			sym:       types.SymbolFact{NodeID: 10, Signature: "var x = new Repository()"},
			wantRes:   true,
			wantConf:  types.ConfidenceConstructor,
			wantKind:  types.EvidenceConstructor,
			wantState: "validated",
		},
		{
			name:      "T3_constructor_object_initializer",
			sym:       types.SymbolFact{NodeID: 10, Signature: "new Session { Id = 1 }"},
			wantRes:   true,
			wantConf:  types.ConfidenceConstructor,
			wantKind:  types.EvidenceConstructor,
			wantState: "validated",
		},
		{
			name:      "T4_assignment_var",
			sym:       types.SymbolFact{NodeID: 10, Signature: "var x = y"},
			wantRes:   true,
			wantConf:  types.ConfidenceAssignment,
			wantKind:  types.EvidenceAssignment,
			wantState: "validated",
		},
		{
			name:      "T6_heuristic_iprefix",
			sym:       types.SymbolFact{NodeID: 10, StableKey: "IRepository"},
			wantRes:   true,
			wantConf:  types.ConfidenceHeuristic,
			wantKind:  types.EvidenceHeuristic,
			wantState: "validated",
		},
		{
			name:      "T7_unknown_no_lsp",
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

// TestCSharpResolver_CrossNamespaceCaps: a target type in a different namespace
// (directory) caps at ≤0.60 + unresolved (M1: C# resolves intra-namespace only).
func TestCSharpResolver_CrossNamespaceCaps(t *testing.T) {
	store := newFakeStore()
	store.symsByNode[100] = types.SymbolFact{NodeID: 100, Signature: "Other x", FilePath: "src/a/f.cs"}
	store.symsByNode[200] = types.SymbolFact{NodeID: 200, Kind: "class", StableKey: "Other", FilePath: "src/b/g.cs"}
	r := NewResolver(store)
	r.typeIndex = map[string]graph.NodeID{"Other": 200}
	resp, err := r.ResolveChain(context.Background(), types.ChainRequest{
		RepoID: "repo", Language: "c_sharp", FilePath: "src/a/f.cs",
		RefNodeID: 100, RefKind: "RESOLVES_TO",
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	if resp.Confidence > 0.60 {
		t.Errorf("Confidence = %v, want ≤ 0.60 (cross-namespace cap)", resp.Confidence)
	}
	if resp.ValidationState != "unresolved" {
		t.Errorf("ValidationState = %q, want unresolved", resp.ValidationState)
	}
	if !strings.Contains(strings.ToLower(resp.Reason), "cross-namespace") {
		t.Errorf("Reason = %q, want substring 'cross-namespace'", resp.Reason)
	}
}

// TestCSharpResolver_IntraNamespaceResolves: same directory → validated at tier.
func TestCSharpResolver_IntraNamespaceResolves(t *testing.T) {
	store := newFakeStore()
	store.symsByNode[100] = types.SymbolFact{NodeID: 100, Signature: "Sibling x", FilePath: "src/a/f.cs"}
	store.symsByNode[200] = types.SymbolFact{NodeID: 200, Kind: "class", StableKey: "Sibling", FilePath: "src/a/g.cs"}
	r := NewResolver(store)
	r.typeIndex = map[string]graph.NodeID{"Sibling": 200}
	resp, _ := r.ResolveChain(context.Background(), types.ChainRequest{
		RepoID: "repo", Language: "c_sharp", FilePath: "src/a/f.cs",
		RefNodeID: 100, RefKind: "RESOLVES_TO",
	})
	if !resp.Resolved || resp.Confidence != types.ConfidenceAnnotation {
		t.Fatalf("intra-namespace = %+v, want resolved annotation 0.90", resp)
	}
}

// TestCSharpResolver_ParseAnnotation covers the typed-declaration grammar.
func TestCSharpResolver_ParseAnnotation(t *testing.T) {
	cases := []struct{ sig, want string }{
		{"Foo x", "Foo"},
		{"private readonly Foo _x", "Foo"},
		{"[XmlAttribute] public Widget W", "Widget"},
		{"[Required][Range(0,9)] internal Bar b", "Bar"},
		{"List<Widget> items", "List"},
		{"Foo[] arr", "Foo"},
		{"System.String s", "String"},
		{"Foo? maybe", "Foo"},
		{"public Foo Bar { get; set; }", "Foo"},
		{"var x = new Foo()", ""}, // var → assignment/ctor tier
		{"new Foo()", ""},         // new → constructor tier
		{"x = y", ""},             // bare assignment
		{"lonely", ""},
	}
	for _, tc := range cases {
		if got := parseAnnotation(tc.sig); got != tc.want {
			t.Errorf("parseAnnotation(%q) = %q, want %q", tc.sig, got, tc.want)
		}
	}
}

// TestCSharpResolver_ParseConstructor covers new/record construction.
func TestCSharpResolver_ParseConstructor(t *testing.T) {
	cases := []struct{ sig, want string }{
		{"new Foo()", "Foo"},
		{"var x = new Repository()", "Repository"},
		{"new Session { Id = 1 }", "Session"},
		{"new List<Widget>()", "List"},
		{"new ns.Deep.Point(1, 2)", "Point"},
		{"Foo x", ""},
		{"x = y", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := parseConstructor(tc.sig); got != tc.want {
			t.Errorf("parseConstructor(%q) = %q, want %q", tc.sig, got, tc.want)
		}
	}
}

// TestCSharpResolver_ParseAssignment covers var/bare assignment RHS.
func TestCSharpResolver_ParseAssignment(t *testing.T) {
	cases := []struct{ sig, want string }{
		{"var x = y", "y"},
		{"x = other", "other"},
		{"var x = new Foo()", ""}, // ctor
		{"x = f()", ""},           // call
		{"Foo x = y", ""},         // typed decl
		{"", ""},
	}
	for _, tc := range cases {
		if got := parseAssignment(tc.sig); got != tc.want {
			t.Errorf("parseAssignment(%q) = %q, want %q", tc.sig, got, tc.want)
		}
	}
}

// TestCSharpResolver_Heuristic covers suffix rules + PascalCase/I-prefix.
func TestCSharpResolver_Heuristic(t *testing.T) {
	cases := []struct{ name, want string }{
		{"userSvc", "UserService"},
		{"orderCtrl", "OrderController"},
		{"IRepository", "IRepository"}, // I-prefix interface self-match
		{"Widget", "Widget"},           // PascalCase self-match
		{"lowercase", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := guessFromName(tc.name); got != tc.want {
			t.Errorf("guessFromName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestCSharpResolver_NamespaceScope: directory-based namespace approximation.
func TestCSharpResolver_NamespaceScope(t *testing.T) {
	if !SameNamespace("src/App/Models/A.cs", "src/App/Models/B.cs") {
		t.Fatalf("SameNamespace within a dir = false, want true")
	}
	if SameNamespace("src/App/Models/A.cs", "src/App/Services/B.cs") {
		t.Fatalf("SameNamespace across dirs = true, want false")
	}
}

// TestCSharpResolver_ResolveSymbol: symbol path delegates to the ladder.
func TestCSharpResolver_ResolveSymbol(t *testing.T) {
	store := newFakeStore()
	store.symsByNode[10] = types.SymbolFact{NodeID: 10, Signature: "var x = new Repository()"}
	r := NewResolver(store)
	resp, err := r.ResolveSymbol(context.Background(), types.SymbolRequest{
		RepoID: "repo", Language: "c_sharp", FilePath: "src/a/f.cs", SymbolNodeID: 10,
	})
	if err != nil {
		t.Fatalf("ResolveSymbol err: %v", err)
	}
	if !resp.Resolved || resp.EvidenceKind != types.EvidenceConstructor {
		t.Fatalf("ResolveSymbol = %+v, want resolved constructor", resp)
	}
}
