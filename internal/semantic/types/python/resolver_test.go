package python

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
	data, err := os.ReadFile(filepath.Join("testdata", "ladder", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var fx fixture
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
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
		t.Fatalf("Resolved=%v want %v (resp=%+v)", resp.Resolved, fx.Expected.Resolved, resp)
	}
	if resp.Confidence != fx.Expected.Confidence {
		t.Fatalf("Confidence=%v want %v", resp.Confidence, fx.Expected.Confidence)
	}
	if string(resp.EvidenceKind) != fx.Expected.EvidenceKind {
		t.Fatalf("Evidence=%q want %q", resp.EvidenceKind, fx.Expected.EvidenceKind)
	}
	if resp.ValidationState != fx.Expected.ValidationState {
		t.Fatalf("State=%q want %q", resp.ValidationState, fx.Expected.ValidationState)
	}
	if fx.Expected.SourcePrefix != "" && !strings.HasPrefix(resp.Source, fx.Expected.SourcePrefix) {
		t.Fatalf("Source=%q want prefix %q", resp.Source, fx.Expected.SourcePrefix)
	}
}

func TestPyResolver_LadderTier_LSP(t *testing.T)         { runFixture(t, loadFixture(t, "lsp_input.json")) }
func TestPyResolver_LadderTier_Annotation(t *testing.T)  { runFixture(t, loadFixture(t, "annotation_input.json")) }
func TestPyResolver_LadderTier_Constructor(t *testing.T) { runFixture(t, loadFixture(t, "constructor_input.json")) }
func TestPyResolver_LadderTier_Assignment(t *testing.T)  { runFixture(t, loadFixture(t, "assignment_input.json")) }
func TestPyResolver_LadderTier_TypeComment(t *testing.T) { runFixture(t, loadFixture(t, "typecomment_input.json")) }
func TestPyResolver_LadderTier_Heuristic(t *testing.T)   { runFixture(t, loadFixture(t, "heuristic_input.json")) }
func TestPyResolver_LadderTier_Unknown(t *testing.T)     { runFixture(t, loadFixture(t, "unknown_input.json")) }

// TestPyScope_InitWalk: package scope follows the innermost __init__.py.
func TestPyScope_InitWalk(t *testing.T) {
	root := t.TempDir()
	pkgDir := filepath.Join(root, "pkg")
	subDir := filepath.Join(pkgDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(pkgDir, "__init__.py"), filepath.Join(subDir, "__init__.py")} {
		if err := os.WriteFile(p, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := PackageScope(filepath.Join(subDir, "mod.py"))
	if got != subDir {
		t.Fatalf("PackageScope = %q, want %q (innermost __init__.py)", got, subDir)
	}
}

// TestPyScope_NoInitFallsBackToDir: no __init__.py → fallback to file dir.
func TestPyScope_NoInitFallsBackToDir(t *testing.T) {
	root := t.TempDir()
	noPkg := filepath.Join(root, "loose")
	if err := os.MkdirAll(noPkg, 0o755); err != nil {
		t.Fatal(err)
	}
	got := PackageScope(filepath.Join(noPkg, "mod.py"))
	if got != noPkg {
		t.Fatalf("PackageScope = %q, want %q (fallback)", got, noPkg)
	}
}

// TestPyComment_TypeComment: `# type: UserRepository`.
func TestPyComment_TypeComment(t *testing.T) {
	got := ParsePyTypeComment("x = something()  # type: UserRepository")
	if got != "UserRepository" {
		t.Fatalf("got %q want UserRepository", got)
	}
}

// TestPyComment_PEP484Annotation: function signature.
func TestPyComment_PEP484Annotation(t *testing.T) {
	cases := []struct {
		sig  string
		want string
	}{
		{"def f(x: UserRepository) -> Foo:", "Foo"}, // return type wins
		{"x: Foo", "Foo"},
		{"x: int = 0", "int"},
	}
	for _, tc := range cases {
		got := ParsePyAnnotation(tc.sig)
		if got != tc.want {
			t.Fatalf("ParsePyAnnotation(%q) = %q, want %q", tc.sig, got, tc.want)
		}
	}
}
