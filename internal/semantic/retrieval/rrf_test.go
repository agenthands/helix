package retrieval

import (
	"encoding/json"
	"testing"
)

// rrf_test.go — RED-gate failing tests for retrieval.Fuse.
// Stub Fuse panics; these tests compile (exercising the typed signature) and
// fail at runtime until Task 1 GREEN supplies the real implementation.

// gvAll returns a closure that yields the same graph_version for every
// symbolID — the single-snapshot tiebreak case.
func gvAll(v uint64) func(string) uint64 {
	return func(string) uint64 { return v }
}

// fusedIDs extracts the SymbolID order from a []FusedCandidate so the test
// assertions can compare ranking order without dragging in score arithmetic.
func fusedIDs(fs []FusedCandidate) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.SymbolID)
	}
	return out
}

// TestRRF_EqualWeights_OrderByScore: text=[A,B,C], graph=[C,B,A], K=60,
// equal weights. Hand calculation:
//   A: 1/61 + 1/63 = 0.016393 + 0.015873 = 0.032266
//   B: 1/62 + 1/62 = 0.016129 + 0.016129 = 0.032258
//   C: 1/63 + 1/61 = 0.015873 + 0.016393 = 0.032266
//
// Score-tie between A and C; tiebreak (graph_version desc, symbol_id asc)
// places A before C. So expected order is [A, C, B].
func TestRRF_EqualWeights_OrderByScore(t *testing.T) {
	text := []TextRank{{SymbolID: "A"}, {SymbolID: "B"}, {SymbolID: "C"}}
	graph := []GraphRank{{SymbolID: "C"}, {SymbolID: "B"}, {SymbolID: "A"}}
	got := Fuse(text, graph, DefaultRRFConfig(), gvAll(7))
	want := []string{"A", "C", "B"}
	gotIDs := fusedIDs(got)
	if len(gotIDs) != len(want) {
		t.Fatalf("len: got %d, want %d (got=%v)", len(gotIDs), len(want), gotIDs)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("order[%d]: got %q, want %q (full=%v)", i, gotIDs[i], want[i], gotIDs)
		}
	}
}

// TestRRF_TextOnly_NoGraph: empty graph ranking; the order MUST follow the
// text ranking exactly because graph contributes nothing.
func TestRRF_TextOnly_NoGraph(t *testing.T) {
	text := []TextRank{{SymbolID: "X"}, {SymbolID: "Y"}, {SymbolID: "Z"}}
	graph := []GraphRank(nil)
	got := Fuse(text, graph, DefaultRRFConfig(), gvAll(0))
	want := []string{"X", "Y", "Z"}
	gotIDs := fusedIDs(got)
	for i := range want {
		if i >= len(gotIDs) || gotIDs[i] != want[i] {
			t.Fatalf("text-only order: got %v, want %v", gotIDs, want)
		}
	}
}

// TestRRF_GraphOnly_NoText: empty text ranking; order follows graph ranking.
func TestRRF_GraphOnly_NoText(t *testing.T) {
	text := []TextRank(nil)
	graph := []GraphRank{{SymbolID: "M"}, {SymbolID: "N"}, {SymbolID: "O"}}
	got := Fuse(text, graph, DefaultRRFConfig(), gvAll(0))
	want := []string{"M", "N", "O"}
	gotIDs := fusedIDs(got)
	for i := range want {
		if i >= len(gotIDs) || gotIDs[i] != want[i] {
			t.Fatalf("graph-only order: got %v, want %v", gotIDs, want)
		}
	}
}

// TestRRF_Determinism_TieScore: two symbols with equal RRF scores; tiebreak
// goes (graph_version desc, then symbol_id asc). Run Fuse 10 times and assert
// identical output across all 10 runs (catches non-deterministic map
// iteration / score-tie ordering bugs — Phase 62 sort-before-iterate doctrine
// + CONTEXT.md acceptance test #4 / #8).
func TestRRF_Determinism_TieScore(t *testing.T) {
	// Two symbols at exactly the same rank position in both rankings have
	// identical scores. Set their graph_version to differ so the second
	// tiebreak (graph_version desc) decides — D second, then symbol_id asc.
	text := []TextRank{{SymbolID: "alpha"}, {SymbolID: "beta"}}
	graph := []GraphRank{{SymbolID: "alpha"}, {SymbolID: "beta"}}
	gv := func(id string) uint64 {
		switch id {
		case "alpha":
			return 5
		case "beta":
			return 9
		}
		return 0
	}

	var snapshots [][]byte
	for i := 0; i < 10; i++ {
		out := Fuse(text, graph, DefaultRRFConfig(), gv)
		blob, err := json.Marshal(out)
		if err != nil {
			t.Fatalf("marshal run %d: %v", i, err)
		}
		snapshots = append(snapshots, blob)
	}
	for i := 1; i < len(snapshots); i++ {
		if string(snapshots[i]) != string(snapshots[0]) {
			t.Fatalf("non-deterministic Fuse output: run 0 = %s ; run %d = %s",
				snapshots[0], i, snapshots[i])
		}
	}
	// Pin the tiebreak: beta has graph_version=9 > alpha's 5, so beta MUST
	// rank ahead of alpha (desc) — even though "alpha" < "beta" alphabetically.
	out := Fuse(text, graph, DefaultRRFConfig(), gv)
	if len(out) != 2 || out[0].SymbolID != "beta" || out[1].SymbolID != "alpha" {
		t.Fatalf("graph_version-desc tiebreak: got %v, want [beta, alpha]", fusedIDs(out))
	}
}

// TestRRF_WeightedSplit: WText=2.0, WGraph=0.5. A is text-rank #1 (strong text)
// + graph-rank #5 (weak graph); B is text-rank #5 (weak text) + graph-rank #1
// (strong graph). Hand calculation with K=60:
//   A: 2.0 * (1 / (60+1)) + 0.5 * (1 / (60+5)) = 0.032787 + 0.007692 = 0.040479
//   B: 2.0 * (1 / (60+5)) + 0.5 * (1 / (60+1)) = 0.030769 + 0.008197 = 0.038966
//
// A > B → text-strong outranks graph-strong under WText=2 / WGraph=0.5.
func TestRRF_WeightedSplit(t *testing.T) {
	text := []TextRank{{SymbolID: "A"}, {SymbolID: "p2"}, {SymbolID: "p3"}, {SymbolID: "p4"}, {SymbolID: "B"}}
	graph := []GraphRank{{SymbolID: "B"}, {SymbolID: "p2"}, {SymbolID: "p3"}, {SymbolID: "p4"}, {SymbolID: "A"}}
	cfg := RRFConfig{K: 60, WText: 2.0, WGraph: 0.5}
	got := Fuse(text, graph, cfg, gvAll(0))
	if len(got) == 0 {
		t.Fatalf("no output")
	}
	// Find positions of A and B in the result.
	posA, posB := -1, -1
	for i, c := range got {
		switch c.SymbolID {
		case "A":
			posA = i
		case "B":
			posB = i
		}
	}
	if posA < 0 || posB < 0 {
		t.Fatalf("missing A/B in output (got=%v)", fusedIDs(got))
	}
	if posA >= posB {
		t.Fatalf("text-strong A (pos=%d) must outrank graph-strong B (pos=%d) under WText=2/WGraph=0.5; got=%v",
			posA, posB, fusedIDs(got))
	}
}
