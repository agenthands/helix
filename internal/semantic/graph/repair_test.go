// Phase 62 P02 RED gate — failing tests for ComputeGraphRepair (D-05/D-06).
//
// The production package internal/semantic/graph does not exist at the time
// these tests are written; they MUST fail to compile until Task 3 lands the
// production code (see 62-02-PLAN.md tasks 1 → 3).
//
// Each test pins one cell of the SymbolDiff matrix:
//   - body-only edits → IsEmpty() == true (D-06 short-circuit)
//   - signature/exported/kind/stable_key changes → IsEmpty() == false
//   - edge add/remove → endpoints land in DirtyNodes
//   - removed symbol → RemovedNodes + InvalidatedIncoming + InvalidatedOutgoing
package graph

import "testing"

func TestComputeGraphRepair_BodyOnlyIsEmpty(t *testing.T) {
	diff := FileFactDiff{
		ChangedSymbols: []SymbolDiff{{NodeID: 42, BodyOnlyChanged: true}},
	}
	repair := ComputeGraphRepair(diff)
	if !repair.IsEmpty() {
		t.Fatalf("body-only diff: IsEmpty()=false, want true (D-06 short-circuit). repair=%+v", repair)
	}
}

func TestComputeGraphRepair_SignatureChangeNotEmpty(t *testing.T) {
	diff := FileFactDiff{
		ChangedSymbols: []SymbolDiff{{NodeID: 42, SignatureChanged: true}},
	}
	repair := ComputeGraphRepair(diff)
	if repair.IsEmpty() {
		t.Fatal("signature-changed diff: IsEmpty()=true, want false")
	}
}

func TestComputeGraphRepair_ExportedChange(t *testing.T) {
	diff := FileFactDiff{
		ChangedSymbols: []SymbolDiff{{NodeID: 1, ExportedChanged: true}},
	}
	if ComputeGraphRepair(diff).IsEmpty() {
		t.Fatal("exported-changed: IsEmpty()=true, want false")
	}
}

func TestComputeGraphRepair_KindChange(t *testing.T) {
	diff := FileFactDiff{
		ChangedSymbols: []SymbolDiff{{NodeID: 1, KindChanged: true}},
	}
	if ComputeGraphRepair(diff).IsEmpty() {
		t.Fatal("kind-changed: IsEmpty()=true, want false")
	}
}

func TestComputeGraphRepair_StableKeyChange(t *testing.T) {
	diff := FileFactDiff{
		ChangedSymbols: []SymbolDiff{{NodeID: 1, StableKeyChanged: true}},
	}
	if ComputeGraphRepair(diff).IsEmpty() {
		t.Fatal("stable_key-changed: IsEmpty()=true, want false")
	}
}

func TestComputeGraphRepair_AddedEdge(t *testing.T) {
	diff := FileFactDiff{
		AddedEdges: []GraphEdge{{SrcNodeID: 1, DstNodeID: 2, EdgeKind: "CALLS"}},
	}
	repair := ComputeGraphRepair(diff)
	if repair.IsEmpty() {
		t.Fatal("added-edge: IsEmpty()=true, want false")
	}
	// Both endpoints must land in DirtyNodes.
	wantDirty := map[NodeID]bool{1: false, 2: false}
	for _, n := range repair.DirtyNodes {
		if _, ok := wantDirty[n]; ok {
			wantDirty[n] = true
		}
	}
	for n, seen := range wantDirty {
		if !seen {
			t.Errorf("added-edge: endpoint %d missing from DirtyNodes (got %v)", n, repair.DirtyNodes)
		}
	}
}

func TestComputeGraphRepair_RemovedSymbol(t *testing.T) {
	diff := FileFactDiff{
		RemovedSymbols: []SymbolDiff{{NodeID: 7}},
	}
	repair := ComputeGraphRepair(diff)
	if repair.IsEmpty() {
		t.Fatal("removed-symbol: IsEmpty()=true, want false")
	}
	if !contains(repair.RemovedNodes, 7) {
		t.Errorf("removed-symbol: 7 missing from RemovedNodes (got %v)", repair.RemovedNodes)
	}
	if !contains(repair.InvalidatedIncoming, 7) {
		t.Errorf("removed-symbol: 7 missing from InvalidatedIncoming (got %v)", repair.InvalidatedIncoming)
	}
	if !contains(repair.InvalidatedOutgoing, 7) {
		t.Errorf("removed-symbol: 7 missing from InvalidatedOutgoing (got %v)", repair.InvalidatedOutgoing)
	}
}

func contains(s []NodeID, v NodeID) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
