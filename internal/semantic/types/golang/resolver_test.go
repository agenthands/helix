package golang

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/types"
)

// fakeStore is a minimal types.EffectiveReader used by the Go resolver tests.
// It records the last query and returns canned edge / symbol facts indexed
// by the request's RefNodeID (or symbol NodeID).
type fakeStore struct {
	edgesByNode  map[graph.NodeID][]types.EdgeFact
	symsByNode   map[graph.NodeID]types.SymbolFact
	edgesQueries []types.EdgeQuery
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		edgesByNode: map[graph.NodeID][]types.EdgeFact{},
		symsByNode:  map[graph.NodeID]types.SymbolFact{},
	}
}

func (f *fakeStore) QueryEffectiveEdges(_ context.Context, q types.EdgeQuery) ([]types.EdgeFact, error) {
	f.edgesQueries = append(f.edgesQueries, q)
	return f.edgesByNode[q.SrcNodeID], nil
}

func (f *fakeStore) QueryEffectiveSymbol(_ context.Context, _ string, n graph.NodeID) (types.SymbolFact, error) {
	if s, ok := f.symsByNode[n]; ok {
		return s, nil
	}
	return types.SymbolFact{NodeID: n}, nil
}

// fixture is the JSON shape used by testdata/ladder/<tier>_input.json.
type fixture struct {
	Name     string                      `json:"name"`
	Symbols  map[uint64]types.SymbolFact `json:"symbols"`
	Edges    map[uint64][]types.EdgeFact `json:"edges"`
	Request  types.ChainRequest          `json:"request"`
	Expected struct {
		Resolved        bool    `json:"resolved"`
		Confidence      float64 `json:"confidence"`
		EvidenceKind    string  `json:"evidence_kind"`
		ValidationState string  `json:"validation_state"`
		SourcePrefix    string  `json:"source_prefix"`
	} `json:"expected"`
}

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	path := filepath.Join("testdata", "ladder", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var fx fixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return fx
}

func runFixture(t *testing.T, fx fixture) {
	t.Helper()
	store := newFakeStore()
	for k, v := range fx.Symbols {
		store.symsByNode[graph.NodeID(k)] = v
	}
	for k, v := range fx.Edges {
		store.edgesByNode[graph.NodeID(k)] = v
	}
	r := NewResolver(store)
	resp, err := r.ResolveChain(context.Background(), fx.Request)
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	if resp.Resolved != fx.Expected.Resolved {
		t.Fatalf("Resolved = %v, want %v (resp=%+v)", resp.Resolved, fx.Expected.Resolved, resp)
	}
	if resp.Confidence != fx.Expected.Confidence {
		t.Fatalf("Confidence = %v, want %v (resp=%+v)", resp.Confidence, fx.Expected.Confidence, resp)
	}
	if string(resp.EvidenceKind) != fx.Expected.EvidenceKind {
		t.Fatalf("EvidenceKind = %q, want %q", resp.EvidenceKind, fx.Expected.EvidenceKind)
	}
	if resp.ValidationState != fx.Expected.ValidationState {
		t.Fatalf("ValidationState = %q, want %q", resp.ValidationState, fx.Expected.ValidationState)
	}
	if fx.Expected.SourcePrefix != "" && !strings.HasPrefix(resp.Source, fx.Expected.SourcePrefix) {
		t.Fatalf("Source = %q, want prefix %q", resp.Source, fx.Expected.SourcePrefix)
	}
}

func TestGoResolver_LadderTier_LSP(t *testing.T) { runFixture(t, loadFixture(t, "lsp_input.json")) }
func TestGoResolver_LadderTier_Annotation(t *testing.T) {
	runFixture(t, loadFixture(t, "annotation_input.json"))
}
func TestGoResolver_LadderTier_Constructor(t *testing.T) {
	runFixture(t, loadFixture(t, "constructor_input.json"))
}
func TestGoResolver_LadderTier_Assignment(t *testing.T) {
	runFixture(t, loadFixture(t, "assignment_input.json"))
}
func TestGoResolver_LadderTier_GoDoc(t *testing.T) { runFixture(t, loadFixture(t, "godoc_input.json")) }
func TestGoResolver_LadderTier_Heuristic(t *testing.T) {
	runFixture(t, loadFixture(t, "heuristic_input.json"))
}
func TestGoResolver_LadderTier_Unknown(t *testing.T) {
	runFixture(t, loadFixture(t, "unknown_input.json"))
}

// TestGoResolver_CrossPackageStopsAtLastInPackage: ref in /repo/foo/a.go
// resolves to a target whose file is /repo/bar/b.go. Even though annotation
// would normally yield 0.90, the cross-package guard caps at ≤0.60 with
// validation_state="unresolved" + Reason mentions "cross-package" (D-13).
func TestGoResolver_CrossPackageStopsAtLastInPackage(t *testing.T) {
	store := newFakeStore()
	// Ref NodeID 100: typed as "OtherType" via signature; lives in /repo/foo/a.go.
	store.symsByNode[100] = types.SymbolFact{
		NodeID:    100,
		Language:  "go",
		Kind:      "variable",
		StableKey: "x",
		FilePath:  "/repo/foo/a.go",
		Signature: "var x OtherType",
	}
	// The type "OtherType" is declared as NodeID 200 in /repo/bar/b.go (different package).
	store.symsByNode[200] = types.SymbolFact{
		NodeID:    200,
		Language:  "go",
		Kind:      "type",
		StableKey: "OtherType",
		FilePath:  "/repo/bar/b.go",
	}
	r := NewResolver(store)
	r.typeIndex = map[string]graph.NodeID{"OtherType": 200} // test-only seam
	resp, err := r.ResolveChain(context.Background(), types.ChainRequest{
		RepoID:    "r",
		Language:  "go",
		FilePath:  "/repo/foo/a.go",
		RefNodeID: 100,
		RefKind:   "RESOLVES_TO",
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	if resp.Confidence > 0.60 {
		t.Fatalf("Confidence = %v, want ≤ 0.60 (cross-package cap)", resp.Confidence)
	}
	if resp.ValidationState != "unresolved" {
		t.Fatalf("ValidationState = %q, want unresolved", resp.ValidationState)
	}
	if !strings.Contains(strings.ToLower(resp.Reason), "cross-package") &&
		!strings.Contains(strings.ToLower(resp.Reason), "cross_package") {
		t.Fatalf("Reason = %q, want substring 'cross-package'", resp.Reason)
	}
}

// TestGoResolver_ScopeDirectoryDetection: PackageScope returns filepath.Dir.
func TestGoResolver_ScopeDirectoryDetection(t *testing.T) {
	got := PackageScope("/repo/foo/bar/baz.go")
	want := "/repo/foo/bar"
	if got != want {
		t.Fatalf("PackageScope = %q, want %q", got, want)
	}
	if !SamePackage("/r/x/a.go", "/r/x/b.go") {
		t.Fatalf("SamePackage(a,b) within /r/x = false, want true")
	}
	if SamePackage("/r/x/a.go", "/r/y/b.go") {
		t.Fatalf("SamePackage across dirs = true, want false")
	}
}

// TestGoComment_ParsesGoDocReturns: pure parser test.
func TestGoComment_ParsesGoDocReturns(t *testing.T) {
	cases := []struct {
		doc  string
		want string
	}{
		{"// Foo returns a *UserRepository for the given user.", "UserRepository"},
		{"// Bar returns the *Foo it found.", "Foo"},
		{"// Baz returns an Error if anything fails.", "Error"},
	}
	for _, tc := range cases {
		got := ParseGoDocType(tc.doc)
		if got != tc.want {
			t.Fatalf("ParseGoDocType(%q) = %q, want %q", tc.doc, got, tc.want)
		}
	}
}

// TestGoComment_NoMatchReturnsEmpty: no recognised pattern → empty.
func TestGoComment_NoMatchReturnsEmpty(t *testing.T) {
	cases := []string{
		"",
		"// just a doc comment with no return clause",
		"// counts the number of widgets",
	}
	for _, doc := range cases {
		if got := ParseGoDocType(doc); got != "" {
			t.Fatalf("ParseGoDocType(%q) = %q, want empty", doc, got)
		}
	}
}

// TestGoResolver_ChainTokensAnnotation (v2.12 Phase 136 A1): a ChainRequest
// with ChainTokens:["Foo"] + a typeIndex mapping "Foo"→N resolves at the
// annotation tier to Target=N WITHOUT relying on the Signature.
func TestGoResolver_ChainTokensAnnotation(t *testing.T) {
	store := newFakeStore()
	// Bare signature — only ChainTokens supplies "Foo".
	store.symsByNode[10] = types.SymbolFact{NodeID: 10, Language: "go", Kind: "variable", StableKey: "p", FilePath: "/repo/pkg/a.go", Signature: "p"}
	store.symsByNode[42] = types.SymbolFact{NodeID: 42, Language: "go", Kind: "type", StableKey: "Foo", FilePath: "/repo/pkg/b.go"}
	r := NewResolverWithIndex(store, map[string]graph.NodeID{"Foo": 42})
	resp, err := r.ResolveChain(context.Background(), types.ChainRequest{
		RepoID: "r", Language: "go", FilePath: "/repo/pkg/a.go",
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

// TestGoResolver_NewResolverWithIndex (A2): the constructor seam populates the
// index and a Signature-fallback resolve uses it for Target.
func TestGoResolver_NewResolverWithIndex(t *testing.T) {
	store := newFakeStore()
	store.symsByNode[10] = types.SymbolFact{NodeID: 10, Language: "go", Kind: "variable", StableKey: "b", FilePath: "/repo/pkg/a.go", Signature: "var b Bar"}
	store.symsByNode[7] = types.SymbolFact{NodeID: 7, Language: "go", Kind: "type", StableKey: "Bar", FilePath: "/repo/pkg/a.go"}
	r := NewResolverWithIndex(store, map[string]graph.NodeID{"Bar": 7})
	resp, err := r.ResolveChain(context.Background(), types.ChainRequest{
		RepoID: "r", Language: "go", FilePath: "/repo/pkg/a.go",
		RefNodeID: 10, RefKind: "RESOLVES_TO",
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	if !resp.Resolved || resp.Target != 7 {
		t.Fatalf("NewResolverWithIndex resolve = %+v, want resolved Target=7", resp)
	}
}
