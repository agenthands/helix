package java

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
		RepoID: "repo", Language: "java", FilePath: "src/com/app/F.java",
		RefNodeID: 10, RefKind: "RESOLVES_TO",
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	return resp
}

// TestJavaResolver_LadderTiers walks each populated Java tier (2/3/4/6), LSP
// (1), and the always-emit floor (7). This file REPLACES the removed
// stub_test.go (red-team B3); T1 + T7 preserve its short-circuit + no-LSP
// coverage.
func TestJavaResolver_LadderTiers(t *testing.T) {
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
			sym:       types.SymbolFact{NodeID: 10, Signature: "Foo x"},
			edges:     []types.EdgeFact{{SrcNodeID: 10, DstNodeID: 99, EdgeKind: "RESOLVES_TO", Source: "lsp.java.text_document_definition", Confidence: 1.0}},
			wantRes:   true,
			wantConf:  types.ConfidenceLSP,
			wantKind:  types.EvidenceLSP,
			wantState: "validated",
		},
		{
			name:      "T2_annotation_typed_decl",
			sym:       types.SymbolFact{NodeID: 10, Signature: "final Foo x"},
			wantRes:   true,
			wantConf:  types.ConfidenceAnnotation,
			wantKind:  types.EvidenceAnnotation,
			wantState: "validated",
		},
		{
			name:      "T2_annotation_java_annot_stripped",
			sym:       types.SymbolFact{NodeID: 10, Signature: "@Inject private Widget widget"},
			wantRes:   true,
			wantConf:  types.ConfidenceAnnotation,
			wantKind:  types.EvidenceAnnotation,
			wantState: "validated",
		},
		{
			name:      "T2_annotation_generic",
			sym:       types.SymbolFact{NodeID: 10, Signature: "List<String> items"},
			wantRes:   true,
			wantConf:  types.ConfidenceAnnotation,
			wantKind:  types.EvidenceAnnotation,
			wantState: "validated",
		},
		{
			name:      "T3_constructor_diamond",
			sym:       types.SymbolFact{NodeID: 10, Signature: "var x = new ArrayList<>()"},
			wantRes:   true,
			wantConf:  types.ConfidenceConstructor,
			wantKind:  types.EvidenceConstructor,
			wantState: "validated",
		},
		{
			name:      "T4_assignment",
			sym:       types.SymbolFact{NodeID: 10, Signature: "var x = y"},
			wantRes:   true,
			wantConf:  types.ConfidenceAssignment,
			wantKind:  types.EvidenceAssignment,
			wantState: "validated",
		},
		{
			name:      "T6_heuristic_impl",
			sym:       types.SymbolFact{NodeID: 10, StableKey: "UserServiceImpl"},
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

// TestJavaResolver_CrossPackageCaps: a target in a different package (dir) caps
// at ≤0.60 + unresolved (M1: Java resolves intra-package only).
func TestJavaResolver_CrossPackageCaps(t *testing.T) {
	store := newFakeStore()
	store.symsByNode[100] = types.SymbolFact{NodeID: 100, Signature: "Other x", FilePath: "src/com/app/F.java"}
	store.symsByNode[200] = types.SymbolFact{NodeID: 200, Kind: "class", StableKey: "Other", FilePath: "src/com/other/G.java"}
	r := NewResolver(store)
	r.typeIndex = map[string]graph.NodeID{"Other": 200}
	resp, err := r.ResolveChain(context.Background(), types.ChainRequest{
		RepoID: "repo", Language: "java", FilePath: "src/com/app/F.java",
		RefNodeID: 100, RefKind: "RESOLVES_TO",
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	if resp.Confidence > 0.60 {
		t.Errorf("Confidence = %v, want ≤ 0.60 (cross-package cap)", resp.Confidence)
	}
	if resp.ValidationState != "unresolved" {
		t.Errorf("ValidationState = %q, want unresolved", resp.ValidationState)
	}
	if !strings.Contains(strings.ToLower(resp.Reason), "cross-package") {
		t.Errorf("Reason = %q, want substring 'cross-package'", resp.Reason)
	}
}

// TestJavaResolver_ParseAnnotation covers the typed-declaration grammar.
func TestJavaResolver_ParseAnnotation(t *testing.T) {
	cases := []struct{ sig, want string }{
		{"Foo x", "Foo"},
		{"final Foo x", "Foo"},
		{"@Override public String name", "String"},
		{"@Inject private Widget widget", "Widget"},
		{"@RequestMapping(\"/x\") Handler h", "Handler"},
		{"List<String> items", "List"},
		{"java.util.Map<K,V> m", "Map"},
		{"int[] nums", "int"},
		{"var x = new Foo()", ""}, // var → ctor/assign tier
		{"new Foo()", ""},         // new → constructor tier
		{"x = y", ""},
		{"lonely", ""},
	}
	for _, tc := range cases {
		if got := parseAnnotation(tc.sig); got != tc.want {
			t.Errorf("parseAnnotation(%q) = %q, want %q", tc.sig, got, tc.want)
		}
	}
}

// TestJavaResolver_ParseConstructor covers new / diamond construction.
func TestJavaResolver_ParseConstructor(t *testing.T) {
	cases := []struct{ sig, want string }{
		{"new Foo()", "Foo"},
		{"var x = new ArrayList<>()", "ArrayList"},
		{"new java.util.HashMap<K,V>()", "HashMap"},
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

// TestJavaResolver_Heuristic covers *Impl / Abstract* / get* / suffix / Pascal.
func TestJavaResolver_Heuristic(t *testing.T) {
	cases := []struct{ name, want string }{
		{"UserServiceImpl", "UserService"}, // *Impl → interface
		{"AbstractHandler", "Handler"},      // Abstract* → base
		{"getUser", "User"},                 // get* → property type
		{"setName", "Name"},                 // set* → property type
		{"orderRepo", "OrderRepository"},    // suffix rule
		{"Widget", "Widget"},                // PascalCase self-match
		{"lowercase", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := guessFromName(tc.name); got != tc.want {
			t.Errorf("guessFromName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestJavaResolver_PackageScope: directory-proxy package scope.
func TestJavaResolver_PackageScope(t *testing.T) {
	if !SamePackage("src/com/app/A.java", "src/com/app/B.java") {
		t.Fatalf("SamePackage within a package dir = false, want true")
	}
	if SamePackage("src/com/app/A.java", "src/com/other/B.java") {
		t.Fatalf("SamePackage across package dirs = true, want false")
	}
}

// TestJavaResolver_ResolveSymbol: symbol path delegates to the ladder + still
// short-circuits on an LSP fact (preserves stub_test's ResolveSymbol coverage).
func TestJavaResolver_ResolveSymbol(t *testing.T) {
	store := newFakeStore()
	store.symsByNode[2] = types.SymbolFact{NodeID: 2}
	store.edgesByNode[2] = []types.EdgeFact{{SrcNodeID: 2, DstNodeID: 200, EdgeKind: "RESOLVES_TO", Source: "lsp.java.text_document_definition", Confidence: 1.0}}
	r := NewResolver(store)
	resp, err := r.ResolveSymbol(context.Background(), types.SymbolRequest{
		RepoID: "repo", Language: "java", FilePath: "src/com/app/F.java", SymbolNodeID: 2,
	})
	if err != nil {
		t.Fatalf("ResolveSymbol err: %v", err)
	}
	if !resp.Resolved || resp.Confidence != 1.0 || resp.TypeNodeID != 200 {
		t.Fatalf("ResolveSymbol = %+v, want resolved+1.0 → 200 (LSP)", resp)
	}
}
