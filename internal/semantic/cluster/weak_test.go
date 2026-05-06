// Phase 62 P04 — weak-component determinism + hex-digest tests (Task 1: RED).
//
// These tests are written BEFORE weak.go exists (TDD RED). They will compile-
// fail / fail until Task 2 lands the algorithm and the golden hex digests are
// pinned.
//
// GRAPH-06 byte-equality contract:
//   - Same input → byte-identical Cluster slices across runs.
//   - Cluster IDs are the smallest member NodeID (sorted-key tiebreak).
//   - Members within a cluster are sorted ascending.
//   - The output Cluster slice is sorted by ID ascending.

package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenthands/helix/internal/semantic/graph"
)

// digest serializes a Cluster slice canonically and returns its sha256 hex.
// Format: "<cluster_id>:<m1>,<m2>,...\n" repeated, concatenated, hashed.
// Members are written in slice order — the algorithm must already have
// sorted them ascending, so the order is part of the determinism contract.
func digest(clusters []Cluster) string {
	var b strings.Builder
	for _, c := range clusters {
		fmt.Fprintf(&b, "%d:", uint64(c.ID))
		for i, m := range c.Members {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "%d", uint64(m))
		}
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// readGolden loads testdata/<name> and trims whitespace.
func readGolden(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("testdata", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read golden %s: %v", p, err)
	}
	return strings.TrimSpace(string(b))
}

// threeComponents builds three disconnected triangles:
//
//	Triangle A: 1-2-3
//	Triangle B: 10-11-12
//	Triangle C: 20-21-22
func threeComponents() (nodes []graph.NodeID, edges map[graph.NodeID]map[graph.NodeID]float64) {
	nodes = []graph.NodeID{1, 2, 3, 10, 11, 12, 20, 21, 22}
	edges = map[graph.NodeID]map[graph.NodeID]float64{
		1:  {2: 1.0, 3: 1.0},
		2:  {3: 1.0},
		10: {11: 1.0, 12: 1.0},
		11: {12: 1.0},
		20: {21: 1.0, 22: 1.0},
		21: {22: 1.0},
	}
	return
}

// singleComponentRing builds a 5-node directed ring 5→7→11→13→17→5.
// All nodes share one weak component.
func singleComponentRing() (nodes []graph.NodeID, edges map[graph.NodeID]map[graph.NodeID]float64) {
	nodes = []graph.NodeID{5, 7, 11, 13, 17}
	edges = map[graph.NodeID]map[graph.NodeID]float64{
		5:  {7: 1.0},
		7:  {11: 1.0},
		11: {13: 1.0},
		13: {17: 1.0},
		17: {5: 1.0},
	}
	return
}

// isolatedNodes builds 4 nodes with no edges → 4 single-member clusters.
func isolatedNodes() (nodes []graph.NodeID, edges map[graph.NodeID]map[graph.NodeID]float64) {
	nodes = []graph.NodeID{4, 8, 15, 16}
	edges = map[graph.NodeID]map[graph.NodeID]float64{}
	return
}

// TestWeakComponents_ThreeComponents asserts that three disconnected
// triangles produce three clusters of size 3, sorted by ID, with members
// sorted ascending.
func TestWeakComponents_ThreeComponents(t *testing.T) {
	nodes, edges := threeComponents()
	cs := WeakComponents(nodes, edges)
	if len(cs) != 3 {
		t.Fatalf("len: got %d, want 3", len(cs))
	}
	want := []Cluster{
		{ID: 1, Members: []graph.NodeID{1, 2, 3}},
		{ID: 10, Members: []graph.NodeID{10, 11, 12}},
		{ID: 20, Members: []graph.NodeID{20, 21, 22}},
	}
	for i, w := range want {
		if cs[i].ID != w.ID {
			t.Errorf("cluster[%d].ID: got %d, want %d", i, cs[i].ID, w.ID)
		}
		if len(cs[i].Members) != len(w.Members) {
			t.Errorf("cluster[%d].Members len: got %d, want %d", i, len(cs[i].Members), len(w.Members))
			continue
		}
		for j := range w.Members {
			if cs[i].Members[j] != w.Members[j] {
				t.Errorf("cluster[%d].Members[%d]: got %d, want %d", i, j, cs[i].Members[j], w.Members[j])
			}
		}
	}
}

// TestWeakComponents_SingleComponent asserts that a 5-node ring becomes one
// cluster, IDed by the smallest member.
func TestWeakComponents_SingleComponent(t *testing.T) {
	nodes, edges := singleComponentRing()
	cs := WeakComponents(nodes, edges)
	if len(cs) != 1 {
		t.Fatalf("len: got %d, want 1", len(cs))
	}
	if cs[0].ID != 5 {
		t.Errorf("ID: got %d, want 5 (smallest member)", cs[0].ID)
	}
	wantMembers := []graph.NodeID{5, 7, 11, 13, 17}
	if len(cs[0].Members) != len(wantMembers) {
		t.Fatalf("Members len: got %d, want %d", len(cs[0].Members), len(wantMembers))
	}
	for i, m := range wantMembers {
		if cs[0].Members[i] != m {
			t.Errorf("Members[%d]: got %d, want %d", i, cs[0].Members[i], m)
		}
	}
}

// TestWeakComponents_IsolatedNodes asserts that nodes with no edges each
// form their own single-member cluster.
func TestWeakComponents_IsolatedNodes(t *testing.T) {
	nodes, edges := isolatedNodes()
	cs := WeakComponents(nodes, edges)
	if len(cs) != 4 {
		t.Fatalf("len: got %d, want 4", len(cs))
	}
	wantIDs := []graph.NodeID{4, 8, 15, 16}
	for i, w := range wantIDs {
		if cs[i].ID != w {
			t.Errorf("cluster[%d].ID: got %d, want %d", i, cs[i].ID, w)
		}
		if len(cs[i].Members) != 1 || cs[i].Members[0] != w {
			t.Errorf("cluster[%d].Members: got %v, want [%d]", i, cs[i].Members, w)
		}
	}
}

// TestWeakComponents_DirectedTreatedAsUndirected asserts that A→B + C→B
// (no A→C) still produces a single weak component containing all three.
func TestWeakComponents_DirectedTreatedAsUndirected(t *testing.T) {
	nodes := []graph.NodeID{1, 2, 3} // A=1, B=2, C=3
	edges := map[graph.NodeID]map[graph.NodeID]float64{
		1: {2: 1.0}, // A→B
		3: {2: 1.0}, // C→B
	}
	cs := WeakComponents(nodes, edges)
	if len(cs) != 1 {
		t.Fatalf("len: got %d, want 1 (weak component must merge A,B,C)", len(cs))
	}
	if cs[0].ID != 1 {
		t.Errorf("ID: got %d, want 1 (smallest member)", cs[0].ID)
	}
	if len(cs[0].Members) != 3 {
		t.Errorf("Members len: got %d, want 3", len(cs[0].Members))
	}
}

// TestWeakComponents_HexDigest asserts byte-equal serialization against the
// pinned testdata goldens for the three canonical fixtures.
func TestWeakComponents_HexDigest(t *testing.T) {
	cases := []struct {
		name   string
		golden string
		build  func() ([]graph.NodeID, map[graph.NodeID]map[graph.NodeID]float64)
	}{
		{"three_components", "golden_three_components.txt", threeComponents},
		{"single_component", "golden_single_component.txt", singleComponentRing},
		{"isolated_nodes", "golden_isolated_nodes.txt", isolatedNodes},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			nodes, edges := tc.build()
			got := digest(WeakComponents(nodes, edges))
			want := readGolden(t, tc.golden)
			if want == "PLACEHOLDER" {
				t.Fatalf("golden %s is still PLACEHOLDER — Task 2 must pin the real digest", tc.golden)
			}
			if got != want {
				t.Errorf("digest mismatch:\n got:  %s\n want: %s", got, want)
			}
		})
	}
}

// TestWeakComponents_DeterministicAcrossRuns runs the algorithm 10 times in
// the same process and asserts the byte-identical Cluster slice each time.
// `go test -count=10` adds a second multiplier on top.
func TestWeakComponents_DeterministicAcrossRuns(t *testing.T) {
	nodes, edges := threeComponents()
	first := digest(WeakComponents(nodes, edges))
	for i := 1; i < 10; i++ {
		got := digest(WeakComponents(nodes, edges))
		if got != first {
			t.Fatalf("run #%d digest drift: got %s, want %s", i, got, first)
		}
	}
}

// TestWeakComponents_EmptyInput asserts the empty-slice and single-isolate
// edge cases.
func TestWeakComponents_EmptyInput(t *testing.T) {
	cs := WeakComponents(nil, nil)
	if cs != nil {
		t.Errorf("nil input: got %v, want nil", cs)
	}
	cs = WeakComponents([]graph.NodeID{42}, nil)
	if len(cs) != 1 {
		t.Fatalf("single node: len=%d, want 1", len(cs))
	}
	if cs[0].ID != 42 || len(cs[0].Members) != 1 || cs[0].Members[0] != 42 {
		t.Errorf("single node: got %+v, want {ID:42, Members:[42]}", cs[0])
	}
}

// TestWeakComponents_SortedMembersWithinCluster asserts that even when the
// input nodes are unsorted, the resulting Members slice is sorted ascending
// AND the cluster ID is the smallest (= 7, not the first-appearing 42).
func TestWeakComponents_SortedMembersWithinCluster(t *testing.T) {
	nodes := []graph.NodeID{42, 7, 99, 13}
	// 4-node clique
	edges := map[graph.NodeID]map[graph.NodeID]float64{
		42: {7: 1.0, 99: 1.0, 13: 1.0},
		7:  {99: 1.0, 13: 1.0},
		99: {13: 1.0},
	}
	cs := WeakComponents(nodes, edges)
	if len(cs) != 1 {
		t.Fatalf("len: got %d, want 1", len(cs))
	}
	if cs[0].ID != 7 {
		t.Errorf("ID: got %d, want 7 (smallest, regardless of input order)", cs[0].ID)
	}
	want := []graph.NodeID{7, 13, 42, 99}
	for i, w := range want {
		if cs[0].Members[i] != w {
			t.Errorf("Members[%d]: got %d, want %d", i, cs[0].Members[i], w)
		}
	}
}
