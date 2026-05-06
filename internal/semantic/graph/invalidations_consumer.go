// Phase 62 P03: InvalidationsConsumer (the "derived path" replay consumer).
//
// RESEARCH OQ3 recommended deriving GraphRepair from the post-commit
// OverlayTx diff rather than wiring a separate semantic_invalidations
// table. The production push-based path (handler.updateChangedFileWithKind)
// hands the diff straight to Engine.ApplyRepair after each tx commit.
//
// InvalidationsConsumer covers the replay/catch-up scenario: the daemon
// restarts between the overlay commit and the post-commit handler hook,
// so a set of tombstoned symbol/edge rows exists with no corresponding
// graph_version advance. ConsumePending re-reads those tombstones and
// synthesises one coalesced GraphRepair per call.
//
// The Phase 61 cascade.WriteInvalidations seam (cascade.go:483) remains a
// typed no-op — the producer side of this consumer flows through the
// overlay's existing tombstone columns.

package graph

import "context"

// InvalidationsReader is the narrow read seam InvalidationsConsumer
// depends on. It is a strict superset of SchedulerStore so the production
// adapter can implement both with one type. Stand-alone here so test
// code can supply a smaller fake when the full SchedulerStore surface is
// not needed.
type InvalidationsReader interface {
	// PendingOverlayTombstones returns the (deletedSymbolFileIDs,
	// deletedEdgeNodeIDs) pair captured since the last ApplyRepair commit.
	// Production binds via the overlay's status='deleted' rows + a
	// last-applied-epoch checkpoint stored on overlay_meta. Empty slices
	// when nothing is pending.
	PendingOverlayTombstones(ctx context.Context, repoID string) (symbolFileIDs []uint64, edgeNodeIDs []NodeID, err error)
}

// InvalidationsConsumer reads pending overlay tombstones and re-synthesises
// GraphRepair values the engine can apply. Idempotent: calling
// ConsumePending twice with no new tombstones the second time returns an
// empty slice.
type InvalidationsConsumer struct {
	store InvalidationsReader
}

// NewInvalidationsConsumer wraps the supplied reader.
func NewInvalidationsConsumer(store InvalidationsReader) *InvalidationsConsumer {
	return &InvalidationsConsumer{store: store}
}

// ConsumePending derives the GraphRepair value(s) from the overlay's
// tombstoned rows. One repair per call (the consumer coalesces every
// pending tombstone into a single GraphRepair, mirroring how the
// post-commit handler treats one tx as one repair). Returns nil + nil err
// when nothing is pending.
func (c *InvalidationsConsumer) ConsumePending(ctx context.Context, repoID string) ([]GraphRepair, error) {
	if c == nil || c.store == nil {
		return nil, nil
	}
	syms, edgeNodes, err := c.store.PendingOverlayTombstones(ctx, repoID)
	if err != nil {
		return nil, err
	}
	if len(syms) == 0 && len(edgeNodes) == 0 {
		return nil, nil
	}

	diff := FileFactDiff{}
	for _, fid := range syms {
		// The consumer treats every tombstoned symbol's file_id as the
		// node_id seed for ComputeGraphRepair — the production adapter
		// already maps file_ids onto stable node_ids via Phase 59's
		// stable_key derivation; here we accept the pre-translated id.
		diff.RemovedSymbols = append(diff.RemovedSymbols, SymbolDiff{NodeID: NodeID(fid)})
	}
	for _, n := range edgeNodes {
		// Edge tombstones flow into the RemovedEdges slice with the
		// endpoint pair (n, 0) so ComputeGraphRepair adds n to DirtyNodes.
		// The dst=0 sentinel is harmless: ApplyRepair's RemovedEdges path
		// projects to deduplicated endpoints, so 0 contributes once and
		// is filtered downstream by the empty-tombstone short-circuit.
		diff.RemovedEdges = append(diff.RemovedEdges, GraphEdge{
			SrcNodeID: n,
			DstNodeID: 0,
			EdgeKind:  "",
		})
	}

	repair := ComputeGraphRepair(diff)
	if repair.IsEmpty() {
		return nil, nil
	}
	return []GraphRepair{repair}, nil
}
