// Phase 62 P03 RED gate — failing tests for ComputeFrontier.
//
// These tests reference the production type ComputeFrontier which does not
// yet exist. The package will not build until Task 2 lands frontier.go +
// util.go.
package graph

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"testing"
)

func TestComputeFrontier_OneHopUnion(t *testing.T) {
	// Edges: A→B, A→C, B→D. Changed=[A] → 1-hop frontier should include
	// A's outgoing neighbors B, C (and A itself); D is 2-hops out, excluded.
	const A, B, C, D NodeID = 1, 2, 3, 4
	out := map[NodeID]map[NodeID]float64{
		A: {B: 1, C: 1},
		B: {D: 1},
	}
	in := map[NodeID]map[NodeID]float64{
		B: {A: 1},
		C: {A: 1},
		D: {B: 1},
	}
	frontier, overflow := ComputeFrontier([]NodeID{A}, out, in, 100)
	if overflow {
		t.Fatalf("overflow=true unexpectedly")
	}
	want := []NodeID{A, B, C}
	if !equalNodeIDs(frontier, want) {
		t.Errorf("frontier=%v, want %v", frontier, want)
	}
}

func TestComputeFrontier_IncludesIncoming(t *testing.T) {
	// Graph: X→A, Y→A, A→B. Changed=[A] → frontier=[A, B, X, Y]
	// (A's incoming + outgoing one-hop neighbors, plus A itself).
	const A, B, X, Y NodeID = 10, 11, 12, 13
	out := map[NodeID]map[NodeID]float64{
		X: {A: 1},
		Y: {A: 1},
		A: {B: 1},
	}
	in := map[NodeID]map[NodeID]float64{
		A: {X: 1, Y: 1},
		B: {A: 1},
	}
	frontier, overflow := ComputeFrontier([]NodeID{A}, out, in, 100)
	if overflow {
		t.Fatalf("overflow=true unexpectedly")
	}
	want := []NodeID{A, B, X, Y}
	if !equalNodeIDs(frontier, want) {
		t.Errorf("frontier=%v, want %v", frontier, want)
	}
}

func TestComputeFrontier_OverflowReturnsTrue(t *testing.T) {
	// Build a hub with 6000 outgoing leaves. Changed=[hub] → frontier
	// should overflow when maxNodes=5000.
	const hub NodeID = 1
	out := map[NodeID]map[NodeID]float64{hub: {}}
	for i := 2; i <= 6001; i++ {
		out[hub][NodeID(i)] = 1
	}
	in := map[NodeID]map[NodeID]float64{}
	for i := 2; i <= 6001; i++ {
		in[NodeID(i)] = map[NodeID]float64{hub: 1}
	}
	frontier, overflow := ComputeFrontier([]NodeID{hub}, out, in, 5000)
	if !overflow {
		t.Fatalf("overflow=false; want true (frontier size > maxNodes)")
	}
	if frontier != nil {
		t.Errorf("frontier=%v, want nil on overflow", frontier)
	}
}

func TestComputeFrontier_DeterministicAcrossRuns(t *testing.T) {
	// Build a 100-node graph with hash-derived edges to ensure map iteration
	// order would scramble the result if the algorithm relied on it.
	const N = 100
	out := map[NodeID]map[NodeID]float64{}
	in := map[NodeID]map[NodeID]float64{}
	for i := 1; i <= N; i++ {
		src := NodeID(i)
		out[src] = map[NodeID]float64{}
		// Add a few pseudo-random outgoing edges seeded by hash(i).
		h := sha256.Sum256([]byte(fmt.Sprintf("seed-%d", i)))
		for k := 0; k < 5; k++ {
			dst := NodeID((binary.BigEndian.Uint64(h[k*4:k*4+8]) % N) + 1)
			if dst == src {
				continue
			}
			out[src][dst] = 1
			if in[dst] == nil {
				in[dst] = map[NodeID]float64{}
			}
			in[dst][src] = 1
		}
	}
	changed := []NodeID{}
	for i := 1; i <= 20; i++ {
		changed = append(changed, NodeID(i))
	}
	first, _ := ComputeFrontier(changed, out, in, 10000)
	for run := 0; run < 100; run++ {
		got, overflow := ComputeFrontier(changed, out, in, 10000)
		if overflow {
			t.Fatalf("run #%d: overflow=true unexpectedly", run)
		}
		if !equalNodeIDs(got, first) {
			t.Fatalf("run #%d: frontier diverged from run 0\n got %v\n  vs %v",
				run, got, first)
		}
	}
	// Sanity: frontier must be sorted ascending.
	if !sort.SliceIsSorted(first, func(i, j int) bool { return first[i] < first[j] }) {
		t.Errorf("frontier not sorted ascending: %v", first)
	}
}

func equalNodeIDs(a, b []NodeID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
