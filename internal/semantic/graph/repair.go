package graph

// NodeID is the semantic node identifier the rank engine keys on. It is a
// raw uint64 alias rather than a typed wrapper so this package can build on
// the existing semantic store schema without an adapter layer (the store
// also keys on uint64).
type NodeID = uint64

// GraphEdge is the typed edge value carried by GraphRepair. Phase 62 P03
// (RankScheduler) and P05 (type resolver) build on this shape.
type GraphEdge struct {
	SrcNodeID, DstNodeID NodeID
	EdgeKind             string  // e.g., "CALLS", "RESOLVES_TO", "USES_TYPE"
	Confidence           float64
	Source               string  // "lsp.<call>" | "comment.<kind>" | ...
	ValidationState      string  // "validated" | "unresolved"
	Weight               float64
}

// SymbolDiff is one symbol's worth of "what changed" in the post-commit
// FileFactDiff. The five booleans are the D-06 decision dimensions: any of
// the first four → graph-changing; BodyOnlyChanged WITHOUT any of them →
// not graph-changing. NodeID is the symbol's node id (kept stable across
// edits via Phase 59's stable_key derivation).
type SymbolDiff struct {
	NodeID            NodeID
	SignatureChanged  bool
	ExportedChanged   bool
	KindChanged       bool
	StableKeyChanged  bool
	BodyOnlyChanged   bool
}

// FileFactDiff is the post-commit diff the handler hands to
// ComputeGraphRepair. It owns three SymbolDiff slices (removed / changed /
// added) and the edge add / remove sets the cascade or fact emitter
// computed during the tx.
type FileFactDiff struct {
	RemovedSymbols []SymbolDiff
	ChangedSymbols []SymbolDiff
	AddedSymbols   []SymbolDiff
	AddedEdges     []GraphEdge
	RemovedEdges   []GraphEdge
}

// GraphRepair is the value Engine.ApplyRepair consumes. RemovedNodes,
// DirtyNodes, InvalidatedIncoming, and InvalidatedOutgoing are deduplicated
// node-id slices that the scheduler / repair worker projects against the
// effective graph.
//
// IsEmpty() is the load-bearing D-06 short-circuit: a body-only edit
// produces a GraphRepair where every slice is empty, and the handler skips
// ApplyRepair entirely (no mutex, no tx, no graph_version advance).
type GraphRepair struct {
	RemovedNodes        []NodeID
	DirtyNodes          []NodeID
	UpsertedEdges       []GraphEdge
	RemovedEdges        []GraphEdge
	InvalidatedIncoming []NodeID
	InvalidatedOutgoing []NodeID
}

// IsEmpty reports whether the repair carries any work. True iff every slice
// is len 0. The handler short-circuits ApplyRepair when this returns true
// (D-06 body-only invariant).
func (r GraphRepair) IsEmpty() bool {
	return len(r.RemovedNodes) == 0 &&
		len(r.DirtyNodes) == 0 &&
		len(r.UpsertedEdges) == 0 &&
		len(r.RemovedEdges) == 0 &&
		len(r.InvalidatedIncoming) == 0 &&
		len(r.InvalidatedOutgoing) == 0
}

// ComputeGraphRepair turns a FileFactDiff into a GraphRepair value (D-06
// decision policy):
//
//   - RemovedSymbols → RemovedNodes + InvalidatedIncoming + InvalidatedOutgoing.
//   - ChangedSymbols with any of {SignatureChanged, ExportedChanged,
//     KindChanged, StableKeyChanged} → DirtyNodes + InvalidatedIncoming +
//     InvalidatedOutgoing.
//   - ChangedSymbols with ONLY BodyOnlyChanged → no contribution
//     (graph_version stays put).
//   - AddedSymbols → DirtyNodes (the new node is "fresh" and its rank row
//     is missing until the next repair fills it).
//   - AddedEdges → UpsertedEdges + DirtyNodes for both endpoints.
//   - RemovedEdges → RemovedEdges + DirtyNodes for both endpoints.
//
// All node-id slices come back deduplicated.
func ComputeGraphRepair(diff FileFactDiff) GraphRepair {
	var (
		removed   nodeSet
		dirty     nodeSet
		invIn     nodeSet
		invOut    nodeSet
		upserted  []GraphEdge
		removedEs []GraphEdge
	)

	for _, s := range diff.RemovedSymbols {
		removed.add(s.NodeID)
		invIn.add(s.NodeID)
		invOut.add(s.NodeID)
	}
	for _, s := range diff.ChangedSymbols {
		if s.SignatureChanged || s.ExportedChanged || s.KindChanged || s.StableKeyChanged {
			dirty.add(s.NodeID)
			invIn.add(s.NodeID)
			invOut.add(s.NodeID)
			continue
		}
		// BodyOnlyChanged (or no flag at all) → no contribution.
	}
	for _, s := range diff.AddedSymbols {
		dirty.add(s.NodeID)
	}
	for _, e := range diff.AddedEdges {
		upserted = append(upserted, e)
		dirty.add(e.SrcNodeID)
		dirty.add(e.DstNodeID)
	}
	for _, e := range diff.RemovedEdges {
		removedEs = append(removedEs, e)
		dirty.add(e.SrcNodeID)
		dirty.add(e.DstNodeID)
	}

	return GraphRepair{
		RemovedNodes:        removed.slice(),
		DirtyNodes:          dirty.slice(),
		UpsertedEdges:       upserted,
		RemovedEdges:        removedEs,
		InvalidatedIncoming: invIn.slice(),
		InvalidatedOutgoing: invOut.slice(),
	}
}

// nodeSet is a tiny dedup helper backed by an insertion-ordered slice + a
// map for O(1) presence checks. Insertion order is preserved so callers
// see deterministic output (the sort happens in P03 when the scheduler
// builds the frontier).
type nodeSet struct {
	seen  map[NodeID]struct{}
	order []NodeID
}

func (s *nodeSet) add(n NodeID) {
	if s.seen == nil {
		s.seen = make(map[NodeID]struct{})
	}
	if _, ok := s.seen[n]; ok {
		return
	}
	s.seen[n] = struct{}{}
	s.order = append(s.order, n)
}

func (s *nodeSet) slice() []NodeID {
	if len(s.order) == 0 {
		return nil
	}
	out := make([]NodeID, len(s.order))
	copy(out, s.order)
	return out
}
