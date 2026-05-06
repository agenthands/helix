package graph

import (
	"cmp"
	"math"
	"sort"
)

// PageRank computes weighted PageRank over the supplied directed graph
// and returns a deterministic score map keyed by node ID.
//
// Inputs:
//   - nodes: full node-id slice. The caller may pass it unsorted; the
//     engine sorts a copy ONCE at entry and uses the sorted slice for
//     every node-keyed iteration thereafter (D-03).
//   - edges: adjacency map edges[src][dst] = weight. Missing src keys
//     are treated as dangling.
//   - opts: convergence options; see Options.withDefaults.
//
// Determinism contract (D-03): every iteration step that visits node IDs
// uses the pre-sorted `sorted` slice. The only map ranges in the inner
// loop are over edges[src] for accumulating per-target contributions —
// floating-point addition is commutative so the sum is order-safe.
//
// Returns nil for empty/nil node sets. A single-node input collapses to
// score = 1.0 for that node (or its teleport weight).
func PageRank[T cmp.Ordered](nodes []T, edges map[T]map[T]float64, opts Options) map[T]float64 {
	opts = opts.withDefaults()
	n := len(nodes)
	if n == 0 {
		return nil
	}

	// (1) Sort node IDs ONCE. Every subsequent node-keyed iteration uses
	// `sorted` so map iteration order can never leak into the result.
	sorted := append([]T(nil), nodes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	if n == 1 {
		out := make(map[T]float64, 1)
		// Honor an explicit personalize entry on the one node if present;
		// otherwise default to 1.0 (the only mass available).
		score := 1.0
		if len(opts.Personalize) > 0 {
			if w, ok := opts.Personalize[any(sorted[0])]; ok && w > 0 {
				score = 1.0
				_ = w // re-normalization on a single-node graph is trivially 1.0
			}
		}
		out[sorted[0]] = score
		return out
	}

	// (2) Build teleport vector over the sorted slice (D-03). When
	// Personalize is set, type-assert keys to T and re-normalize over the
	// active node set so seeded + non-seeded weights sum to 1.0.
	teleport := buildTeleport(sorted, opts.Personalize)

	// (3) Initialize rank = teleport, iterating sorted (D-03).
	rank := make(map[T]float64, n)
	for _, k := range sorted {
		rank[k] = teleport[k]
	}

	// (4) Pre-compute total outbound weight per source. Inner sum is
	// commutative — order-safe — so we can range edges[src] directly.
	outWeight := make(map[T]float64, len(edges))
	for _, src := range sorted {
		targets, ok := edges[src]
		if !ok {
			continue
		}
		total := 0.0
		// commutative — order-safe (D-03 inner-loop annotation)
		for _, w := range targets {
			total += w
		}
		if total > 0 {
			outWeight[src] = total
		}
	}

	fn := float64(n)
	d := opts.Damping

	// (5) Power iteration.
	for iter := 0; iter < opts.MaxIter; iter++ {
		// Dangling mass = sum of rank for nodes with no outgoing weight.
		// Iterate sorted (D-03).
		dangling := 0.0
		for _, k := range sorted {
			if _, hasOut := outWeight[k]; !hasOut {
				dangling += rank[k]
			}
		}

		next := make(map[T]float64, n)

		// Teleport + dangling redistribution component, sorted iteration.
		for _, k := range sorted {
			next[k] = (1.0-d)*teleport[k] + d*dangling/fn
		}

		// Link component: outer loop over sorted (D-03). Inner loop
		// over edges[src] is commutative — order-safe.
		for _, src := range sorted {
			tw, hasOut := outWeight[src]
			if !hasOut || tw == 0 {
				continue
			}
			contribution := d * rank[src] / tw
			// commutative — order-safe (addition into next[dst] only)
			for dst, w := range edges[src] {
				next[dst] += contribution * w
			}
		}

		// Convergence check, sorted iteration.
		diff := 0.0
		for _, k := range sorted {
			diff += math.Abs(next[k] - rank[k])
		}
		rank = next
		if diff < opts.Epsilon {
			break
		}
	}

	return rank
}
