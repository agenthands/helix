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

// functionNodeByName resolves a function/method symbol's NodeID by name.
func functionNodeByName(facts semanticstore.Facts, name string) uint64 {
	for _, s := range facts.Symbols {
		if (s.Kind == "function" || s.Kind == "method") && s.Name == name {
			return s.NodeID
		}
	}
	return 0
}

// collectDataFlowsBySource returns the DATA_FLOWS edges with a specific Source
// marker (def_use / def_use_inbody / def_use_return).
func collectDataFlowsBySource(facts semanticstore.Facts, source string) []semanticstore.EdgeFact {
	var out []semanticstore.EdgeFact
	for _, e := range facts.Edges {
		if e.EdgeKind == "DATA_FLOWS" && e.Source == source {
			out = append(out, e)
		}
	}
	return out
}

// TestFactsFromExtracted_InBodyEmission (FLOW-05a): a caller with an in-body
// producer()->sink(a) flow emits a def_use_inbody edge producer.function ->
// sink.param0.
func TestFactsFromExtracted_InBodyEmission(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)
	caller := goExtract(t, p, "caller.go", "package m\nfunc caller() { a := producer(); sink(a) }\n")
	producer := goExtract(t, p, "producer.go", "package m\nfunc producer() int { return 0 }\n")
	sink := goExtract(t, p, "sink.go", "package m\nfunc sink(v int) {}\n")

	got := factsFromExtracted([]*extract.ExtractedFile{caller, producer, sink}, "r", "", nil, nil)
	inBody := collectDataFlowsBySource(got, "def_use_inbody")
	if len(inBody) == 0 {
		t.Fatalf("want >=1 def_use_inbody edge; got none. all DATA_FLOWS: %+v", collectDataFlows(got))
	}
	prodNode := functionNodeByName(got, "producer")
	sinkParam := paramNodeByName(got, "v")
	if prodNode == 0 || sinkParam == 0 {
		t.Fatalf("missing endpoints: producer.fn=%d sink.v=%d", prodNode, sinkParam)
	}
	var found bool
	for _, e := range inBody {
		if e.SrcNodeID == prodNode && e.DstNodeID == sinkParam {
			found = true
			if e.SrcKind != "function" || e.DstKind != "parameter" {
				t.Errorf("in-body kinds = %q/%q, want function/parameter", e.SrcKind, e.DstKind)
			}
			if e.Confidence != 0.50 || e.Weight != 0.5 {
				t.Errorf("in-body conf/weight = %v/%v, want 0.50/0.5", e.Confidence, e.Weight)
			}
			if e.Reason != "return of producer -> sink param0" {
				t.Errorf("in-body Reason = %q, want %q", e.Reason, "return of producer -> sink param0")
			}
		}
	}
	if !found {
		t.Errorf("no def_use_inbody edge producer.fn(%d) -> sink.v(%d); got %+v", prodNode, sinkParam, inBody)
	}
}

// TestFactsFromExtracted_ReturnBridge (FLOW-05b): a param whose value reaches
// the function's return emits a def_use_return edge param -> enclosing function.
func TestFactsFromExtracted_ReturnBridge(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)
	transform := goExtract(t, p, "transform.go", "package m\nfunc transform(x int) int { return x }\n")

	got := factsFromExtracted([]*extract.ExtractedFile{transform}, "r", "", nil, nil)
	bridge := collectDataFlowsBySource(got, "def_use_return")
	if len(bridge) == 0 {
		t.Fatalf("want >=1 def_use_return edge; got none. all DATA_FLOWS: %+v", collectDataFlows(got))
	}
	fnNode := functionNodeByName(got, "transform")
	xParam := paramNodeByName(got, "x")
	if fnNode == 0 || xParam == 0 {
		t.Fatalf("missing endpoints: transform.fn=%d x=%d", fnNode, xParam)
	}
	var found bool
	for _, e := range bridge {
		if e.SrcNodeID == xParam && e.DstNodeID == fnNode {
			found = true
			if e.SrcKind != "parameter" || e.DstKind != "function" {
				t.Errorf("bridge kinds = %q/%q, want parameter/function", e.SrcKind, e.DstKind)
			}
			if e.Confidence != 0.55 || e.Weight != 0.5 {
				t.Errorf("bridge conf/weight = %v/%v, want 0.55/0.5", e.Confidence, e.Weight)
			}
			if e.Reason != "param0 -> return" {
				t.Errorf("bridge Reason = %q, want %q", e.Reason, "param0 -> return")
			}
		}
	}
	if !found {
		t.Errorf("no def_use_return edge x(%d) -> transform.fn(%d); got %+v", xParam, fnNode, bridge)
	}
}

// TestFactsFromExtracted_InBodyAntiMisBind (FLOW-05c): two producers sharing the
// name "producer" (nameCount==2) => NO def_use_inbody edge (anti-mis-bind D5).
func TestFactsFromExtracted_InBodyAntiMisBind(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)
	caller := goExtract(t, p, "caller.go", "package m\nfunc caller() { a := producer(); sink(a) }\n")
	prod1 := goExtract(t, p, "prod1.go", "package a\nfunc producer() int { return 0 }\n")
	prod2 := goExtract(t, p, "prod2.go", "package b\nfunc producer() int { return 1 }\n")
	sink := goExtract(t, p, "sink.go", "package m\nfunc sink(v int) {}\n")

	got := factsFromExtracted([]*extract.ExtractedFile{caller, prod1, prod2, sink}, "r", "", nil, nil)
	if inBody := collectDataFlowsBySource(got, "def_use_inbody"); len(inBody) != 0 {
		t.Errorf("anti-mis-bind: want 0 def_use_inbody edges for duplicate producer name; got %+v", inBody)
	}
}

// TestFactsFromExtracted_InBodyExternalConsumerNoEdge (FLOW-05c): an unresolved
// consumer (not in the batch name index) => no fabricated in-body edge.
func TestFactsFromExtracted_InBodyExternalConsumerNoEdge(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)
	// caller feeds producer()'s return into an external sink not defined in batch.
	caller := goExtract(t, p, "caller.go", "package m\nfunc caller() { a := producer(); extsink(a) }\n")
	producer := goExtract(t, p, "producer.go", "package m\nfunc producer() int { return 0 }\n")

	got := factsFromExtracted([]*extract.ExtractedFile{caller, producer}, "r", "", nil, nil)
	if inBody := collectDataFlowsBySource(got, "def_use_inbody"); len(inBody) != 0 {
		t.Errorf("external consumer: want 0 def_use_inbody edges; got %+v", inBody)
	}
}

// TestFactsFromExtracted_Distinctness (FLOW-05d, mutation-confirmed): an
// in-body-only body emits def_use_inbody + ZERO def_use param->param; a
// param-only body emits def_use + ZERO def_use_inbody.
func TestFactsFromExtracted_Distinctness(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)

	// (1) in-body-only: caller has no params; the only flow is producer()->sink(a).
	caller := goExtract(t, p, "caller.go", "package m\nfunc caller() { a := producer(); sink(a) }\n")
	producer := goExtract(t, p, "producer.go", "package m\nfunc producer() int { return 0 }\n")
	sink := goExtract(t, p, "sink.go", "package m\nfunc sink(v int) {}\n")
	gotIn := factsFromExtracted([]*extract.ExtractedFile{caller, producer, sink}, "r", "", nil, nil)
	if n := len(collectDataFlowsBySource(gotIn, "def_use_inbody")); n == 0 {
		t.Errorf("in-body-only: want >=1 def_use_inbody edge; got 0")
	}
	if defuse := collectDataFlowsBySource(gotIn, "def_use"); len(defuse) != 0 {
		t.Errorf("in-body-only: want 0 def_use param->param edges; got %+v", defuse)
	}

	// (2) param-only: f(x){ sink(x) } — a v2.9 param->param flow, no in-body call.
	f := goExtract(t, p, "f.go", "package m\nfunc f(x int) { sink(x) }\n")
	sink2 := goExtract(t, p, "sink2.go", "package m\nfunc sink(v int) {}\n")
	gotParam := factsFromExtracted([]*extract.ExtractedFile{f, sink2}, "r", "", nil, nil)
	if n := len(collectDataFlowsBySource(gotParam, "def_use")); n == 0 {
		t.Errorf("param-only: want >=1 def_use param->param edge; got 0")
	}
	if inBody := collectDataFlowsBySource(gotParam, "def_use_inbody"); len(inBody) != 0 {
		t.Errorf("param-only: want 0 def_use_inbody edges; got %+v", inBody)
	}
}

// TestFactsFromExtracted_DefUseSetNonRegression (FLOW-05e): on a PURE-param Go
// forward (src->mid->sink, no in-body call-return, so D-COMPOSE never fires),
// the Source=="def_use" param->param edge SET (by content tuple) is EXACTLY the
// v2.9 set — the new passes must not mutate it. Go is one of the 6 stable langs
// (138 ledger); ruby/kotlin/python/c/cpp are excluded (wholesale/partially-new).
func TestFactsFromExtracted_DefUseSetNonRegression(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)
	src := goExtract(t, p, "src.go", "package m\nfunc src(x int) { mid(x) }\n")
	mid := goExtract(t, p, "mid.go", "package m\nfunc mid(m int) { sink(m) }\n")
	sink := goExtract(t, p, "sink.go", "package m\nfunc sink(v int) {}\n")

	got := factsFromExtracted([]*extract.ExtractedFile{src, mid, sink}, "r", "", nil, nil)
	defuse := collectDataFlowsBySource(got, "def_use")

	// v2.9 content tuple: (Src,Dst,SrcKind,DstKind,Source,Confidence,Weight,Reason).
	type tup struct {
		src, dst           uint64
		sk, dk, so, reason string
		conf, wt           float64
	}
	srcX := paramNodeByName(got, "x")
	midM := paramNodeByName(got, "m")
	sinkV := paramNodeByName(got, "v")
	if srcX == 0 || midM == 0 || sinkV == 0 {
		t.Fatalf("missing endpoints: x=%d m=%d v=%d", srcX, midM, sinkV)
	}
	want := map[tup]struct{}{
		{srcX, midM, "parameter", "parameter", "def_use", "arg0 -> callee param0", 0.55, 0.5}: {},
		{midM, sinkV, "parameter", "parameter", "def_use", "arg0 -> callee param0", 0.55, 0.5}: {},
	}
	got0 := map[tup]struct{}{}
	for _, e := range defuse {
		got0[tup{e.SrcNodeID, e.DstNodeID, e.SrcKind, e.DstKind, e.Source, e.Reason, e.Confidence, e.Weight}] = struct{}{}
	}
	if len(got0) != len(want) {
		t.Fatalf("def_use SET size = %d, want %d (v2.9 set drift). got: %+v", len(got0), len(want), defuse)
	}
	for w := range want {
		if _, ok := got0[w]; !ok {
			t.Errorf("def_use SET missing v2.9 edge %+v; got %+v", w, defuse)
		}
	}
}

// TestFactsFromExtracted_MultiHop (FLOW-05g): src() -> transform -> sink over
// real Go. sink.param must be reachable from src's FUNCTION node via
// in-body(src->transform.param) -> return-bridge(transform.param->transform) ->
// in-body(transform->sink.param).
func TestFactsFromExtracted_MultiHop(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)
	src := goExtract(t, p, "src.go", "package m\nfunc src() int { return 0 }\n")
	transform := goExtract(t, p, "transform.go", "package m\nfunc transform(x int) int { return x }\n")
	sink := goExtract(t, p, "sink.go", "package m\nfunc sink(v int) {}\n")
	caller := goExtract(t, p, "caller.go", "package m\nfunc caller() { a := src(); b := transform(a); sink(b) }\n")

	got := factsFromExtracted([]*extract.ExtractedFile{src, transform, sink, caller}, "r", "", nil, nil)
	df := collectDataFlows(got)

	srcFn := functionNodeByName(got, "src")
	sinkV := paramNodeByName(got, "v")
	if srcFn == 0 || sinkV == 0 {
		t.Fatalf("missing endpoints: src.fn=%d sink.v=%d", srcFn, sinkV)
	}
	if !reachable(df, srcFn, sinkV) {
		t.Errorf("sink.v(%d) not reachable from src.fn(%d) over multi-hop DATA_FLOWS; edges: %+v", sinkV, srcFn, df)
	}
}

// TestFactsFromExtracted_MultiHop_BrokenHop (FLOW-05g revert-and-fail): if
// transform drops the return (return-bridge hop severed), sink.v is no longer
// reachable from src's function node.
func TestFactsFromExtracted_MultiHop_BrokenHop(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)
	src := goExtract(t, p, "src.go", "package m\nfunc src() int { return 0 }\n")
	// transform no longer returns its param — the return-bridge hop disappears.
	transform := goExtract(t, p, "transform.go", "package m\nfunc transform(x int) int { return 0 }\n")
	sink := goExtract(t, p, "sink.go", "package m\nfunc sink(v int) {}\n")
	caller := goExtract(t, p, "caller.go", "package m\nfunc caller() { a := src(); b := transform(a); sink(b) }\n")

	got := factsFromExtracted([]*extract.ExtractedFile{src, transform, sink, caller}, "r", "", nil, nil)
	df := collectDataFlows(got)

	srcFn := functionNodeByName(got, "src")
	sinkV := paramNodeByName(got, "v")
	if srcFn == 0 || sinkV == 0 {
		t.Fatal("missing endpoints")
	}
	if reachable(df, srcFn, sinkV) {
		t.Errorf("sink.v reachable from src.fn despite severed return-bridge hop: %+v", df)
	}
}
