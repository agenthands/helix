// Phase 62 P03: 1-hop incremental-repair frontier (D-09).
//
// ComputeFrontier turns a (changed-nodes ∪ 1-hop-neighbors) seed into the
// deterministic frontier the scheduler will run PageRank over. The function
// is pure: same inputs → same output. Determinism is enforced via
// sort-before-iterate on every node-keyed iteration (Pitfall 1).
//
// If the union exceeds maxNodes the function returns (nil, true) so the
// caller can take the "all stale + full recompute" branch (D-09 hard
// invariant — predictable budget over rank accuracy).

package graph

import "sort"

// ComputeFrontier returns the deterministic 1-hop frontier given the
// supplied changed-node seeds plus the current effective adjacency. The
// result is sorted ascending by NodeID. When the frontier exceeds
// maxNodes the function returns (nil, true) — callers MUST treat that as
// the overflow signal (D-09).
//
// adjacencyOut is keyed by source node and maps to a (dst → weight) inner
// map; adjacencyIn is keyed by destination node and maps to a (src → weight)
// inner map. Either may be nil; nil maps yield no neighbors for the
// corresponding direction.
func ComputeFrontier(
	changed []NodeID,
	adjacencyOut, adjacencyIn map[NodeID]map[NodeID]float64,
	maxNodes int,
) (frontier []NodeID, overflow bool) {

	seen := make(map[NodeID]struct{}, len(changed)*4)

	// Sort the changed seed once so the iteration order does not depend on
	// the caller's slice ordering (every Phase 62 iterator that drives
	// rank-impacting state lives at this layer of discipline).
	changedSorted := append([]NodeID(nil), changed...)
	sort.Slice(changedSorted, func(i, j int) bool { return changedSorted[i] < changedSorted[j] })

	add := func(n NodeID) bool {
		if _, ok := seen[n]; ok {
			return true
		}
		seen[n] = struct{}{}
		return len(seen) <= maxNodes
	}

	for _, n := range changedSorted {
		if !add(n) {
			return nil, true
		}
		// Outgoing 1-hop — sortedNodeIDs guards Pitfall 1.
		for _, dst := range sortedNodeIDs(adjacencyOut[n]) {
			if !add(dst) {
				return nil, true
			}
		}
		// Incoming 1-hop.
		for _, src := range sortedNodeIDs(adjacencyIn[n]) {
			if !add(src) {
				return nil, true
			}
		}
	}

	return sortedNodeIDs(seen), false
}
