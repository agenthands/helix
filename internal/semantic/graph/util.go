// Phase 62 P03: shared determinism helper.
//
// Pitfall 1 (62-RESEARCH.md) catalogued the easiest determinism slip in the
// rank scheduler: ranging a node-keyed map without sort-before-iterate.
// Every node-keyed iteration in this package MUST funnel through
// sortedNodeIDs (or a hand-sorted slice) so the GRAPH-01 byte-equal
// determinism contract is preserved at every layer above the engine.

package graph

import "sort"

// sortedNodeIDs returns the keys of a NodeID-keyed map in ascending order.
// Generic over the value type so callers don't have to construct an
// adapter for adjacency maps (map[NodeID]float64) vs presence sets
// (map[NodeID]struct{}). The output slice is a fresh allocation; mutating
// it does not affect the input map.
func sortedNodeIDs[V any](m map[NodeID]V) []NodeID {
	if len(m) == 0 {
		return nil
	}
	out := make([]NodeID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
