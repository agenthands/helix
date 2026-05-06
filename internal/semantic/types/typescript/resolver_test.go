package typescript

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
		t.Fatalf("read fixture %s: %v", name, err)
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

func TestTSResolver_LadderTier_LSP(t *testing.T)         { runFixture(t, loadFixture(t, "lsp_input.json")) }
func TestTSResolver_LadderTier_Annotation(t *testing.T)  { runFixture(t, loadFixture(t, "annotation_input.json")) }
func TestTSResolver_LadderTier_Constructor(t *testing.T) { runFixture(t, loadFixture(t, "constructor_input.json")) }
func TestTSResolver_LadderTier_Assignment(t *testing.T)  { runFixture(t, loadFixture(t, "assignment_input.json")) }
func TestTSResolver_LadderTier_TSDoc(t *testing.T)       { runFixture(t, loadFixture(t, "tsdoc_input.json")) }
func TestTSResolver_LadderTier_Heuristic(t *testing.T)   { runFixture(t, loadFixture(t, "heuristic_input.json")) }
func TestTSResolver_LadderTier_Unknown(t *testing.T)     { runFixture(t, loadFixture(t, "unknown_input.json")) }

// TestTSScope_TsconfigBoundary writes a tsconfig.json into a temp dir tree
// and asserts PackageScope walks UP from a child file to the directory
// containing tsconfig.json.
func TestTSScope_TsconfigBoundary(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "src", "foo")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tsconfig := filepath.Join(root, "tsconfig.json")
	if err := os.WriteFile(tsconfig, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := PackageScope(filepath.Join(srcDir, "a.ts"))
	if got != root {
		t.Fatalf("PackageScope = %q, want %q", got, root)
	}
}

// TestTSScope_NoTsconfigFallsBackToDir: when no tsconfig is found anywhere,
// fall back to file's containing directory.
func TestTSScope_NoTsconfigFallsBackToDir(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "src", "bar")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	got := PackageScope(filepath.Join(srcDir, "a.ts"))
	if got != srcDir {
		t.Fatalf("PackageScope = %q, want %q (fallback to file dir)", got, srcDir)
	}
}

func TestTSComment_TSDocType(t *testing.T) {
	got := ParseTSDocType("/** @type {UserRepository} */")
	if got != "UserRepository" {
		t.Fatalf("ParseTSDocType @type = %q, want UserRepository", got)
	}
}

func TestTSComment_JSDocParam(t *testing.T) {
	got := ParseTSDocType("/** @param {Foo} bar – description */")
	if got != "Foo" {
		t.Fatalf("ParseTSDocType @param = %q, want Foo", got)
	}
}

func TestTSComment_JSDocReturns(t *testing.T) {
	got := ParseTSDocType("/** @returns {Baz} */")
	if got != "Baz" {
		t.Fatalf("ParseTSDocType @returns = %q, want Baz", got)
	}
}
