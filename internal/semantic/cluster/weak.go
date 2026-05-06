// Phase 62 P04 — weak-component algorithm (GRAPH-06).
//
// See doc.go for the determinism contract. The implementation is a sorted-
// key union-find with smaller-ID-becomes-root tiebreak. Input directed
// edges are treated as undirected (a weak component is connected after
// edge orientation is dropped).
//
// Complexity: O((N + E) · α(N)) where α is the inverse Ackermann; for the
// per-workspace cardinality (< 100k nodes) this is sub-second on typical
// hardware. T-62-04-D1 accepts the unbounded-graph case for v1.

package cluster

import (
	"sort"

	"github.com/agenthands/helix/internal/semantic/graph"
)

// Cluster groups NodeIDs into a connected component over the undirected
// projection of the effective graph. ID is the smallest member NodeID
// (deterministic naming). Members is sorted ascending.
type Cluster struct {
	ID      graph.NodeID
	Members []graph.NodeID
}

// WeakComponents partitions `nodes` into weak (undirected) components over
// the directed `edges` map. The output Cluster slice is sorted by ID
// ascending; each Members slice is sorted ascending. Same input → byte-
// identical output across runs (TestWeakComponents_HexDigest with
// -count=10).
//
// Empty `nodes` returns nil. Edges referencing NodeIDs not present in
// `nodes` are silently ignored at the union step (the parent map only
// contains the supplied nodes).
func WeakComponents(nodes []graph.NodeID, edges map[graph.NodeID]map[graph.NodeID]float64) []Cluster {
	if len(nodes) == 0 {
		return nil
	}

	// Sort the node slice once (T-62-04-T1: every node-keyed iteration in
	// this package must funnel through a sorted slice).
	sortedNodes := append([]graph.NodeID(nil), nodes...)
	sort.Slice(sortedNodes, func(i, j int) bool { return sortedNodes[i] < sortedNodes[j] })

	parent := make(map[graph.NodeID]graph.NodeID, len(sortedNodes))
	for _, n := range sortedNodes {
		parent[n] = n
	}

	// Path-compressing find. Recursion depth is bounded by O(log N) after
	// path compression; safe for our cardinality.
	var find func(x graph.NodeID) graph.NodeID
	find = func(x graph.NodeID) graph.NodeID {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}

	// Sorted union-find: smaller-ID always becomes the new root, so the
	// final root for a component is its smallest NodeID — and thus the
	// cluster ID is deterministic across runs.
	union := func(a, b graph.NodeID) {
		// Skip nodes not in the parent map (edges may reference IDs the
		// caller didn't include; silently ignore so a stray edge can't
		// corrupt the partition).
		if _, ok := parent[a]; !ok {
			return
		}
		if _, ok := parent[b]; !ok {
			return
		}
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		if ra < rb {
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}

	// Iterate edges via sorted source keys, then sorted destination keys
	// per source. Although union is associative (any iteration order
	// reaches the same final partition), the determinism contract is
	// belt-and-suspenders: sort everything.
	sortedSrcs := make([]graph.NodeID, 0, len(edges))
	for src := range edges {
		sortedSrcs = append(sortedSrcs, src)
	}
	sort.Slice(sortedSrcs, func(i, j int) bool { return sortedSrcs[i] < sortedSrcs[j] })

	for _, src := range sortedSrcs {
		dsts := edges[src]
		sortedDsts := make([]graph.NodeID, 0, len(dsts))
		for dst := range dsts {
			sortedDsts = append(sortedDsts, dst)
		}
		sort.Slice(sortedDsts, func(i, j int) bool { return sortedDsts[i] < sortedDsts[j] })
		for _, dst := range sortedDsts {
			union(src, dst)
		}
	}

	// Group members by root by iterating sortedNodes — the resulting
	// members[root] slice is therefore already in ascending order.
	members := make(map[graph.NodeID][]graph.NodeID)
	for _, n := range sortedNodes {
		root := find(n)
		members[root] = append(members[root], n)
	}

	// Build output sorted by cluster ID (= root).
	rootList := make([]graph.NodeID, 0, len(members))
	for r := range members {
		rootList = append(rootList, r)
	}
	sort.Slice(rootList, func(i, j int) bool { return rootList[i] < rootList[j] })

	clusters := make([]Cluster, 0, len(rootList))
	for _, r := range rootList {
		clusters = append(clusters, Cluster{ID: r, Members: members[r]})
	}
	return clusters
}
