// Phase 62 P03 RED gate — failing tests for InvalidationsConsumer
// (the "derived path" replay consumer).
package graph

import (
	"context"
	"testing"
)

// fakeInvalidationsStore satisfies SchedulerStore + a small extra
// PendingTombstones method that the consumer reads to derive GraphRepair.
// This mirrors the production read path against the overlay tombstones
// table (RESEARCH OQ3: derived from overlay diff, no separate
// semantic_invalidations table).
type fakeInvalidationsStore struct {
	*fakeSchedulerStore
	tombstoneSymbolFileIDs []uint64
	tombstoneEdgeNodeIDs   []NodeID
}

// PendingOverlayTombstones returns the lists of file_ids (deleted symbols)
// and node_ids (deleted edge endpoints) that have been tombstoned since
// the last ApplyRepair commit. Empty slices when nothing is pending.
func (s *fakeInvalidationsStore) PendingOverlayTombstones(_ context.Context, _ string) (symbolFileIDs []uint64, edgeNodeIDs []NodeID, err error) {
	return s.tombstoneSymbolFileIDs, s.tombstoneEdgeNodeIDs, nil
}

func TestInvalidationsConsumer_DerivesRepairFromOverlay(t *testing.T) {
	store := &fakeInvalidationsStore{
		fakeSchedulerStore:     newFakeSchedulerStore(),
		tombstoneSymbolFileIDs: []uint64{101, 102},
		tombstoneEdgeNodeIDs:   []NodeID{200, 201},
	}
	c := NewInvalidationsConsumer(store)
	repairs, err := c.ConsumePending(context.Background(), "ws")
	if err != nil {
		t.Fatalf("ConsumePending: %v", err)
	}
	if len(repairs) != 1 {
		t.Fatalf("repairs=%d, want 1 (single coalesced repair)", len(repairs))
	}
	r := repairs[0]
	if len(r.RemovedNodes) == 0 {
		t.Errorf("RemovedNodes empty; want symbols [101, 102]")
	}
	// Edge tombstones contribute DirtyNodes (matching the ComputeGraphRepair
	// rule for RemovedEdges → endpoints become dirty).
	if len(r.DirtyNodes) == 0 {
		t.Errorf("DirtyNodes empty; want edge endpoints [200, 201]")
	}
}

func TestInvalidationsConsumer_EmptyOverlayReturnsNothing(t *testing.T) {
	store := &fakeInvalidationsStore{
		fakeSchedulerStore: newFakeSchedulerStore(),
	}
	c := NewInvalidationsConsumer(store)
	repairs, err := c.ConsumePending(context.Background(), "ws")
	if err != nil {
		t.Fatalf("ConsumePending: %v", err)
	}
	if len(repairs) != 0 {
		t.Errorf("repairs=%d, want 0 (no tombstones)", len(repairs))
	}
}
