package daemon

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
)

// buildCTypeFacts runs the REAL C extractor over each src, then composes the
// production Facts via factsFromExtracted WITH the type-resolution driver
// enabled (the same path the production buildFn takes), and stamps the dense
// 1-based EdgeID the buildFn assigns before WriteSnapshotFacts. This is the
// in-process production path for the RESOLVES_TO edge proof (Phase 136 D1).
func buildCTypeFacts(t *testing.T, repoID string, files map[string]string) semanticstore.Facts {
	t.Helper()
	var extracted []*extract.ExtractedFile
	// Deterministic file order for the extractor input (map iteration is not).
	for _, path := range sortedKeys(files) {
		extracted = append(extracted, cExtract(t, path, files[path]))
	}
	facts := factsFromExtracted(extracted, repoID, "", nil, nil,
		withTypeResolution(typeResolveParams{maxFixpoint: 8, minConfidence: 0.45, emitUnresolved: false}))
	for i := range facts.Edges {
		facts.Edges[i].EdgeID = uint64(i + 1)
	}
	return facts
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// tiny insertion sort — deterministic, no import churn.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// nodeByName returns the NodeID of the first committed symbol with the given
// name and (optional) kind. kind=="" matches any kind.
func nodeByName(facts semanticstore.Facts, name, kind string) (uint64, bool) {
	for _, s := range facts.Symbols {
		if s.Name == name && (kind == "" || s.Kind == kind) {
			return s.NodeID, true
		}
	}
	return 0, false
}

// countResolvesTo returns the RESOLVES_TO edges outgoing from src in the
// committed snapshot.
func countResolvesTo(t *testing.T, store *semanticstore.Store, snapshotID, src uint64) []semanticstore.SymbolEdgeRaw {
	t.Helper()
	edges, err := store.QuerySymbolEdgesOutgoing(context.Background(), snapshotID, src)
	if err != nil {
		t.Fatalf("QuerySymbolEdgesOutgoing: %v", err)
	}
	var out []semanticstore.SymbolEdgeRaw
	for _, e := range edges {
		if e.EdgeKind == "RESOLVES_TO" {
			out = append(out, e)
		}
	}
	return out
}

// TestResolveTypeEdges_PositiveCommitsRealEdge is the Phase 136 D1 EXIT GATE
// (SC1, ordered FIRST): a single C file `struct Foo { int a; }; void g(struct
// Foo* p){}` driven through the REAL extractor → factsFromExtracted (type
// driver) → committed snapshot yields EXACTLY ONE RESOLVES_TO edge from p's
// node whose DstNodeID → QueryStableKeyByNodeID == Foo's stable key.
func TestResolveTypeEdges_PositiveCommitsRealEdge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in-process type-edge proof in -short mode (opens DuckDB)")
	}
	src := "struct Foo { int a; };\nvoid g(struct Foo* p){ (void)p; }\n"
	var facts semanticstore.Facts
	store, repoID, cleanup := openDataFlowStore(t, func(r string) semanticstore.Facts {
		facts = buildCTypeFacts(t, r, map[string]string{"a.c": src})
		return facts
	})
	defer cleanup()

	snap, err := store.LatestCommittedSnapshot(context.Background(), repoID)
	if err != nil || snap == 0 {
		t.Fatalf("LatestCommittedSnapshot: snap=%d err=%v", snap, err)
	}

	pNode, ok := nodeByName(facts, "p", string(extract.KindParameter))
	if !ok {
		t.Fatalf("param p not found in committed symbols: %+v", facts.Symbols)
	}
	fooNode, ok := nodeByName(facts, "Foo", string(extract.KindStruct))
	if !ok {
		t.Fatalf("struct Foo not found in committed symbols")
	}

	edges := countResolvesTo(t, store, snap, pNode)
	if len(edges) != 1 {
		t.Fatalf("RESOLVES_TO edges from p = %d, want EXACTLY 1: %+v", len(edges), edges)
	}
	if edges[0].DstNodeID != fooNode {
		t.Errorf("edge dst = %d, want Foo's node %d", edges[0].DstNodeID, fooNode)
	}
	// The DstNodeID must resolve to Foo's committed stable key (real target).
	sk, found, err := store.QueryStableKeyByNodeID(context.Background(), repoID, edges[0].DstNodeID)
	if err != nil || !found {
		t.Fatalf("QueryStableKeyByNodeID(dst=%d): found=%v err=%v", edges[0].DstNodeID, found, err)
	}
	fooSK, foundFoo, err := store.QueryStableKeyByNodeID(context.Background(), repoID, fooNode)
	if err != nil || !foundFoo {
		t.Fatalf("QueryStableKeyByNodeID(Foo=%d): found=%v err=%v", fooNode, foundFoo, err)
	}
	if sk != fooSK {
		t.Errorf("dst stable key = %q, want Foo's stable key %q", sk, fooSK)
	}
}

// TestResolveTypeEdges_NegativeNoEdge is the differential anti-vacuity NEGATIVE
// (SC-neg): a primitive-typed param `void h(int n){}` yields ZERO RESOLVES_TO
// edges (no named type = no link = no fabricated edge).
func TestResolveTypeEdges_NegativeNoEdge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in-process type-edge proof in -short mode (opens DuckDB)")
	}
	src := "void h(int n){ (void)n; }\n"
	var facts semanticstore.Facts
	store, repoID, cleanup := openDataFlowStore(t, func(r string) semanticstore.Facts {
		facts = buildCTypeFacts(t, r, map[string]string{"b.c": src})
		return facts
	})
	defer cleanup()

	snap, err := store.LatestCommittedSnapshot(context.Background(), repoID)
	if err != nil || snap == 0 {
		t.Fatalf("LatestCommittedSnapshot: snap=%d err=%v", snap, err)
	}
	nNode, ok := nodeByName(facts, "n", string(extract.KindParameter))
	if !ok {
		// Param may still be extracted; if not present at all, zero edges
		// trivially holds. Assert the whole snapshot carries no RESOLVES_TO.
		for _, e := range facts.Edges {
			if e.EdgeKind == "RESOLVES_TO" {
				t.Fatalf("unexpected RESOLVES_TO edge for primitive-only fixture: %+v", e)
			}
		}
		return
	}
	if edges := countResolvesTo(t, store, snap, nNode); len(edges) != 0 {
		t.Fatalf("RESOLVES_TO edges from primitive param n = %d, want 0: %+v", len(edges), edges)
	}
}

// TestResolveTypeEdges_Deterministic (SC5): repeated in-process build over the
// same fixture yields byte-identical out.Edges (the sorted driver + no
// Go-map iteration in the emit path).
func TestResolveTypeEdges_Deterministic(t *testing.T) {
	src := "struct Foo { int x; };\nstruct Bar { int y; };\n" +
		"int f(struct Foo* p, struct Bar* q, int n){ return n; }\n"
	build := func() []semanticstore.EdgeFact {
		ef := cExtract(t, "e.c", src)
		facts := factsFromExtracted([]*extract.ExtractedFile{ef}, "r", "", nil, nil,
			withTypeResolution(typeResolveParams{maxFixpoint: 8, minConfidence: 0.45}))
		var rt []semanticstore.EdgeFact
		for _, e := range facts.Edges {
			if e.EdgeKind == "RESOLVES_TO" {
				rt = append(rt, e)
			}
		}
		return rt
	}
	a := build()
	b := build()
	if len(a) == 0 {
		t.Fatalf("determinism fixture emitted 0 RESOLVES_TO edges — anti-vacuity: nothing to compare")
	}
	if len(a) != len(b) {
		t.Fatalf("edge count differs across builds: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("RESOLVES_TO edge %d differs across builds:\n a=%+v\n b=%+v", i, a[i], b[i])
		}
	}
}

// TestResolveTypeEdges_AntiMisBind (SC3): two same-named struct types
// (nameCount>1) leave the type name out of the index, so a param typed by that
// name yields NO edge (no fabricated target).
func TestResolveTypeEdges_AntiMisBind(t *testing.T) {
	// Two DISTINCT struct Foo definitions in different files (different
	// StableKey/SymbolID → both survive dedup → nameCount["Foo"]==2).
	files := map[string]string{
		"one.c": "struct Foo { int a; };\nvoid use1(struct Foo* p){ (void)p; }\n",
		"two.c": "struct Foo { long b; long c; };\n",
	}
	var extracted []*extract.ExtractedFile
	for _, path := range sortedKeys(files) {
		extracted = append(extracted, cExtract(t, path, files[path]))
	}
	facts := factsFromExtracted(extracted, "r", "", nil, nil,
		withTypeResolution(typeResolveParams{maxFixpoint: 8, minConfidence: 0.45}))
	for _, e := range facts.Edges {
		if e.EdgeKind == "RESOLVES_TO" {
			t.Fatalf("anti-mis-bind violated: ambiguous same-named type produced a RESOLVES_TO edge: %+v", e)
		}
	}
}
