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
