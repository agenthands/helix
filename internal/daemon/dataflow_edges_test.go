package daemon

import (
	"testing"

	"github.com/agenthands/helix/internal/semantic/dataflow"
	"github.com/agenthands/helix/internal/semantic/extract"
	goextract "github.com/agenthands/helix/internal/semantic/extract/golang"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/treesitter"
)

// collectDataFlows returns the DATA_FLOWS edges from an emitted Facts batch.
func collectDataFlows(facts semanticstore.Facts) []semanticstore.EdgeFact {
	var df []semanticstore.EdgeFact
	for _, e := range facts.Edges {
		if e.EdgeKind == "DATA_FLOWS" {
			df = append(df, e)
		}
	}
	return df
}

// TestFactsFromExtracted_E2E_DataFlows drives the real Go provider end-to-end
// through the full Phase 126 pipeline: extraction -> FlowSummary ->
// nodeToParams (emit-order adjacency) -> nameCount (anti-mis-bind) ->
// dataFlowEdges -> DATA_FLOWS edges. A src -> mid -> sink chain must emit two
// directed caller.param -> callee.param edges, and sink must be REACHABLE from
// src by walking the emitted edges (the folded Phase 127 reachability proof —
// the taint substrate payoff).
func TestFactsFromExtracted_E2E_DataFlows(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)

	// src forwards x into mid; mid forwards m into sink. Two DATA_FLOWS hops.
	// sink has an empty body (no flow of its own) — it is a callee only, but it
	// still has a param node for the edge to land on.
	src := goExtract(t, p, "src.go", "package m\nfunc src(x int) { mid(x) }\n")
	mid := goExtract(t, p, "mid.go", "package m\nfunc mid(m int) { sink(m) }\n")
	sink := goExtract(t, p, "sink.go", "package m\nfunc sink(v int) {}\n")

	got := factsFromExtracted([]*extract.ExtractedFile{src, mid, sink}, "r", "", nil, nil)
	df := collectDataFlows(got)

	if len(df) < 2 {
		t.Fatalf("want >=2 DATA_FLOWS edges (src->mid, mid->sink); got %d: %+v", len(df), df)
	}
	for _, e := range df {
		if e.SrcNodeID == 0 || e.DstNodeID == 0 || e.SrcNodeID == e.DstNodeID {
			t.Errorf("DATA_FLOWS endpoints invalid: %d->%d %+v", e.SrcNodeID, e.DstNodeID, e)
		}
		if e.Source != "def_use" {
			t.Errorf("DATA_FLOWS Source = %q, want def_use", e.Source)
		}
		// Distinctness (FLOW-03b): DATA_FLOWS is param-anchored, never the
		// function-anchored SIMILAR_TO/STRUCTURAL_TWIN similarity edges.
		if e.SrcKind != "parameter" || e.DstKind != "parameter" {
			t.Errorf("DATA_FLOWS kinds = %q/%q, want parameter/parameter", e.SrcKind, e.DstKind)
		}
	}

	// Reachability: from src's param, sink's param is reachable.
	srcParam := paramNodeByName(got, "x")
	sinkParam := paramNodeByName(got, "v")
	if srcParam == 0 || sinkParam == 0 {
		t.Fatalf("missing endpoints: src.x=%d sink.v=%d", srcParam, sinkParam)
	}
	if !reachable(df, srcParam, sinkParam) {
		t.Errorf("sink.v not reachable from src.x over DATA_FLOWS edges (substrate broken)")
	}
	if reachable(df, sinkParam, srcParam) {
		t.Errorf("src.x reachable from sink.v — DATA_FLOWS must be directed")
	}
}

// TestFactsFromExtracted_E2E_DataFlows_BrokenHop is the revert-and-fail guard:
// if mid does NOT forward to sink, sink must become unreachable from src.
func TestFactsFromExtracted_E2E_DataFlows_BrokenHop(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)
	src := goExtract(t, p, "src.go", "package m\nfunc src(x int) { mid(x) }\n")
	mid := goExtract(t, p, "mid.go", "package m\nfunc mid(m int) {}\n") // dead param — no forward
	sink := goExtract(t, p, "sink.go", "package m\nfunc sink(v int) {}\n")

	got := factsFromExtracted([]*extract.ExtractedFile{src, mid, sink}, "r", "", nil, nil)
	df := collectDataFlows(got)

	srcParam := paramNodeByName(got, "x")
	sinkParam := paramNodeByName(got, "v")
	if srcParam == 0 || sinkParam == 0 {
		t.Fatal("missing endpoints")
	}
	if reachable(df, srcParam, sinkParam) {
		t.Errorf("sink.v reachable from src.x despite broken mid->sink hop: %+v", df)
	}
}

// TestDataFlowEdges_AntiMisBind: a callee name shared by two nodes (overload /
// duplicate) must NOT bind (D5) — no edge for either candidate.
func TestDataFlowEdges_AntiMisBind(t *testing.T) {
	nodes := []fingerprintedNode{
		{nodeID: 1, flow: &dataflow.Summary{Params: []dataflow.ParamFlow{
			{Index: 0, CallArgs: []dataflow.CallArgTarget{{Callee: "g", ArgPos: 0}}},
		}}},
	}
	nodeToParams := map[uint64][]uint64{1: {10}, 100: {200}, 101: {201}}
	nameToNode := map[string]uint64{"g": 100}
	nameCount := map[string]int{"g": 2} // two candidates -> anti-mis-bind
	if edges := dataFlowEdges(nodes, nodeToParams, nameToNode, nameCount); len(edges) != 0 {
		t.Errorf("anti-mis-bind: want 0 edges for duplicate callee name; got %+v", edges)
	}
}

// TestDataFlowEdges_DedupDirected: two call sites forwarding the same param
// pair collapse to ONE directed edge (D7/D8); dedup key is (Src,Dst).
func TestDataFlowEdges_DedupDirected(t *testing.T) {
	nodes := []fingerprintedNode{
		{nodeID: 1, flow: &dataflow.Summary{Params: []dataflow.ParamFlow{
			{Index: 0, CallArgs: []dataflow.CallArgTarget{
				{Callee: "g", ArgPos: 0},
				{Callee: "g", ArgPos: 0},
			}},
		}}},
	}
	nodeToParams := map[uint64][]uint64{1: {10}, 100: {200}}
	nameToNode := map[string]uint64{"g": 100}
	nameCount := map[string]int{"g": 1}
	edges := dataFlowEdges(nodes, nodeToParams, nameToNode, nameCount)
	if len(edges) != 1 {
		t.Fatalf("dedup: want 1 edge; got %d: %+v", len(edges), edges)
	}
	if edges[0].SrcNodeID != 10 || edges[0].DstNodeID != 200 {
		t.Errorf("directed endpoints = %d->%d, want 10->200", edges[0].SrcNodeID, edges[0].DstNodeID)
	}
}

// TestDataFlowEdges_ExternalCalleeNoEdge: a callee not in the batch name index
// (external / unresolved) must produce no fabricated edge (D2 honesty).
func TestDataFlowEdges_ExternalCalleeNoEdge(t *testing.T) {
	nodes := []fingerprintedNode{
		{nodeID: 1, flow: &dataflow.Summary{Params: []dataflow.ParamFlow{
			{Index: 0, CallArgs: []dataflow.CallArgTarget{{Callee: "ext", ArgPos: 0}}},
		}}},
	}
	nodeToParams := map[uint64][]uint64{1: {10}}
	nameToNode := map[string]uint64{} // ext not in repo
	nameCount := map[string]int{}
	if edges := dataFlowEdges(nodes, nodeToParams, nameToNode, nameCount); len(edges) != 0 {
		t.Errorf("external callee: want 0 edges; got %+v", edges)
	}
}

// --- helpers ---

// reachable is a plain BFS over directed edges — the reachability primitive a
// future trace-data-flow verb would consume (Phase 127 folded into 126).
func reachable(edges []semanticstore.EdgeFact, from, to uint64) bool {
	adj := map[uint64][]uint64{}
	for _, e := range edges {
		adj[e.SrcNodeID] = append(adj[e.SrcNodeID], e.DstNodeID)
	}
	seen := map[uint64]bool{from: true}
	frontier := []uint64{from}
	for len(frontier) > 0 {
		n := frontier[0]
		frontier = frontier[1:]
		if n == to {
			return true
		}
		for _, nb := range adj[n] {
			if !seen[nb] {
				seen[nb] = true
				frontier = append(frontier, nb)
			}
		}
	}
	return false
}

// paramNodeByName resolves a parameter symbol's NodeID by name. (The small
// fixtures use unique param names, so a name lookup is unambiguous.)
func paramNodeByName(facts semanticstore.Facts, paramName string) uint64 {
	for _, s := range facts.Symbols {
		if s.Kind == "parameter" && s.Name == paramName {
			return s.NodeID
		}
	}
	return 0
}
