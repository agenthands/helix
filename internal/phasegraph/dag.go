package phasegraph

import "sort"

// adjList is the internal graph representation used by validation and Kahn
// topo-sort. It preserves the caller's PhaseSpec slice (so we can return real
// PhaseSpec values from kahnSort) and indexes by PhaseID for O(1) lookups.
//
// adjList is intentionally unexported: callers interact via the
// [ValidatePhaseGraph] entrypoint, never with the graph directly.
type adjList struct {
	phases []PhaseSpec
	byID   map[PhaseID]int // PhaseID → index into phases (last-wins on duplicate)
}

// buildPhaseGraph indexes phases by ID. Last-wins on duplicate IDs is
// deliberate: duplicate detection runs immediately afterward via
// findDuplicateIDs, which scans the original slice (not byID) and reports
// the first colliding ID.
func buildPhaseGraph(phases []PhaseSpec) *adjList {
	byID := make(map[PhaseID]int, len(phases))
	for i, p := range phases {
		byID[p.ID] = i
	}
	return &adjList{phases: phases, byID: byID}
}

// findDuplicateIDs returns a pointer to the first PhaseID that appears more
// than once in the original phase slice, or nil if every ID is unique.
//
// Returning *PhaseID rather than (PhaseID, bool) matches the SPEC §39.3
// pseudocode signature.
func (g *adjList) findDuplicateIDs() *PhaseID {
	seen := make(map[PhaseID]struct{}, len(g.phases))
	for i := range g.phases {
		id := g.phases[i].ID
		if _, ok := seen[id]; ok {
			return &id
		}
		seen[id] = struct{}{}
	}
	return nil
}

// findMissingDependencies returns a sorted, de-duplicated list of every
// PhaseID that some phase declared in Requires but no phase actually defines.
// Returns nil (not empty slice) when nothing is missing.
func (g *adjList) findMissingDependencies() []PhaseID {
	missing := map[PhaseID]struct{}{}
	for _, p := range g.phases {
		for _, dep := range p.Requires {
			if _, ok := g.byID[dep]; !ok {
				missing[dep] = struct{}{}
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	out := make([]PhaseID, 0, len(missing))
	for id := range missing {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// findCycle returns a slice of PhaseIDs that form a cycle, or nil if the graph
// is acyclic. Self-loops return [id]; multi-node cycles return the offending
// node sequence (entry node first).
//
// Implementation: iterative DFS with three-color marking
//
//	white = 0 (untouched)
//	gray  = 1 (on the current DFS stack)
//	black = 2 (fully explored)
//
// Encountering a gray node mid-traversal proves a back-edge → cycle.
// Iteration order is sorted by PhaseID so the reported cycle is deterministic
// across runs (important for golden-file tests downstream).
//
// findCycle assumes findMissingDependencies has already passed — every
// Requires entry is guaranteed to resolve via g.byID.
func (g *adjList) findCycle() []PhaseID {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[PhaseID]int, len(g.phases))

	// Sorted iteration order for determinism.
	roots := make([]PhaseID, 0, len(g.phases))
	for _, p := range g.phases {
		roots = append(roots, p.ID)
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i] < roots[j] })

	// Stack-frame for iterative DFS: which node, and which Requires index next.
	type frame struct {
		id      PhaseID
		nextDep int
	}

	for _, root := range roots {
		if color[root] != white {
			continue
		}
		stack := []frame{{id: root, nextDep: 0}}
		color[root] = gray
		// Special case: self-loop on the root before we even look at children.
		// Detected naturally by the inner loop below, but we also need to
		// preserve "stack" content as the cycle path. The inner loop handles it.

		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			node := g.phases[g.byID[top.id]]
			// Sort Requires for determinism within a node's children.
			deps := append([]PhaseID(nil), node.Requires...)
			sort.Slice(deps, func(i, j int) bool { return deps[i] < deps[j] })

			if top.nextDep >= len(deps) {
				// Done with this node.
				color[top.id] = black
				stack = stack[:len(stack)-1]
				continue
			}
			dep := deps[top.nextDep]
			top.nextDep++

			switch color[dep] {
			case white:
				color[dep] = gray
				stack = append(stack, frame{id: dep, nextDep: 0})
			case gray:
				// Back-edge → cycle. Build the cycle path: from the first
				// occurrence of `dep` on the stack to the current top, then
				// append `dep` again to close the loop visually.
				start := -1
				for i, f := range stack {
					if f.id == dep {
						start = i
						break
					}
				}
				cycle := make([]PhaseID, 0, len(stack)-start+1)
				if start == -1 {
					// Shouldn't happen: gray means "on stack", but defend.
					cycle = append(cycle, dep)
				} else {
					for _, f := range stack[start:] {
						cycle = append(cycle, f.id)
					}
				}
				return cycle
			case black:
				// Fully explored elsewhere — safe.
			}
		}
	}
	return nil
}

// kahnSort returns the phases in topological order using Kahn's algorithm.
// Caller must guarantee the graph is duplicate-free, dependency-complete, and
// acyclic (i.e. findDuplicateIDs / findMissingDependencies / findCycle all
// returned cleanly first). Order is deterministic: ties are broken by sorted
// PhaseID.
func (g *adjList) kahnSort() []PhaseSpec {
	indeg := make(map[PhaseID]int, len(g.phases))
	for _, p := range g.phases {
		// Initialize every node, including those with zero deps.
		if _, ok := indeg[p.ID]; !ok {
			indeg[p.ID] = 0
		}
		for range p.Requires {
			indeg[p.ID]++
		}
	}

	// Reverse adjacency: which phases depend on each ID?
	dependents := make(map[PhaseID][]PhaseID, len(g.phases))
	for _, p := range g.phases {
		for _, dep := range p.Requires {
			dependents[dep] = append(dependents[dep], p.ID)
		}
	}

	// Sorted ready queue → deterministic output.
	ready := make([]PhaseID, 0, len(g.phases))
	for id, n := range indeg {
		if n == 0 {
			ready = append(ready, id)
		}
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i] < ready[j] })

	out := make([]PhaseSpec, 0, len(g.phases))
	for len(ready) > 0 {
		// Pop smallest.
		id := ready[0]
		ready = ready[1:]
		out = append(out, g.phases[g.byID[id]])

		// Sort dependents for determinism.
		ds := append([]PhaseID(nil), dependents[id]...)
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		for _, d := range ds {
			indeg[d]--
			if indeg[d] == 0 {
				// Insert maintaining sorted order.
				ready = insertSorted(ready, d)
			}
		}
	}
	return out
}

// insertSorted inserts id into a sorted-by-PhaseID slice and returns the
// extended slice. O(n) per insert is fine: phase counts are tiny (≤ 12).
func insertSorted(s []PhaseID, id PhaseID) []PhaseID {
	pos := sort.Search(len(s), func(i int) bool { return s[i] >= id })
	s = append(s, "")
	copy(s[pos+1:], s[pos:])
	s[pos] = id
	return s
}

// reverse returns a new slice containing the input in reverse order. Used to
// derive ShutdownOrder from Order.
func reverse(in []PhaseSpec) []PhaseSpec {
	out := make([]PhaseSpec, len(in))
	for i, p := range in {
		out[len(in)-1-i] = p
	}
	return out
}
