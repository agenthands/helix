package semantic

import (
	"testing"

	"github.com/agenthands/helix/internal/semantic/integ"
)

// TestShapeEdges_SurfacesSemanticallyRelated is the read-surface proof for
// SEMANTICALLY_RELATED (RELATE-03e). explain_symbol_deep's IncomingEdgesOf /
// OutgoingEdgesOf accessors apply NO edge-kind filter (unlike
// get_change_impact_graph, which is hardcoded to the call_graph projection and
// rewrites every edge Kind to "calls"), so any stored edge kind reaches
// shapeEdges. This asserts shapeEdges passes a SEMANTICALLY_RELATED row through
// to the surface enum unchanged — i.e. the edge is readable from
// `helix explain-symbol-deep` once committed, not silently dropped.
func TestShapeEdges_SurfacesSemanticallyRelated(t *testing.T) {
	rows := []SymbolEdgeRow{
		{From: integ.SymbolID("sym1"), To: integ.SymbolID("sym2"), InternalKind: "SEMANTICALLY_RELATED"},
		{From: integ.SymbolID("sym3"), To: integ.SymbolID("sym2"), InternalKind: "CALLS"},
	}
	out, total := shapeEdges(rows, 100)
	if total != 2 {
		t.Fatalf("total = %d, want 2 (no edge-kind filtering)", total)
	}
	var sawRelated bool
	for _, e := range out {
		if e.InternalKind == "SEMANTICALLY_RELATED" {
			sawRelated = true
			if e.EdgeKind != EdgeKindSemanticallyRelated {
				t.Errorf("EdgeKind = %q, want %q", e.EdgeKind, EdgeKindSemanticallyRelated)
			}
		}
	}
	if !sawRelated {
		t.Fatal("SEMANTICALLY_RELATED row was dropped by shapeEdges — edge would be unreadable from explain-symbol-deep")
	}
}

// TestShapeEdges_SurfacesInBodyAndReturnBridge (FLOW-05f) is the read-surface
// proof for the v2.13 in-body (def_use_inbody) and return-bridge
// (def_use_return) DATA_FLOWS edges. Both carry EdgeKind "DATA_FLOWS" on the
// store (the Source marker distinguishes them at emission), so at the read
// surface each maps to EdgeKindDataFlows via MapInternalKind and survives
// shapeEdges unchanged — readable from `helix explain-symbol-deep`
// edges_outgoing/edges_incoming (IncomingEdgesOf/OutgoingEdgesOf apply no
// kind filter).
func TestShapeEdges_SurfacesInBodyAndReturnBridge(t *testing.T) {
	rows := []SymbolEdgeRow{
		// in-body: producer.function -> consumer.param (outgoing from producer).
		{From: integ.SymbolID("producer"), To: integ.SymbolID("sinkParam"), InternalKind: "DATA_FLOWS"},
		// return-bridge: param -> enclosing function (incoming to the function).
		{From: integ.SymbolID("transformParam"), To: integ.SymbolID("transform"), InternalKind: "DATA_FLOWS"},
	}
	out, total := shapeEdges(rows, 100)
	if total != 2 {
		t.Fatalf("total = %d, want 2 (no edge-kind filtering)", total)
	}
	var dataFlowCount int
	for _, e := range out {
		if e.InternalKind == "DATA_FLOWS" {
			dataFlowCount++
			if e.EdgeKind != EdgeKindDataFlows {
				t.Errorf("EdgeKind = %q, want %q", e.EdgeKind, EdgeKindDataFlows)
			}
		}
	}
	if dataFlowCount != 2 {
		t.Fatalf("want 2 DATA_FLOWS rows surviving shapeEdges (in-body + return-bridge); got %d — new edges would be unreadable from explain-symbol-deep", dataFlowCount)
	}
}
