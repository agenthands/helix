package graph

import (
	"cmp"
	"sort"
)

// Ranked pairs a node ID with its PageRank score. Returned by RankNodes.
type Ranked[T cmp.Ordered] struct {
	Node  T
	Score float64
}

// buildTeleport constructs the teleport vector over the sorted node set.
//
// When personalize is empty/nil, every node gets 1/n. Otherwise:
//   - Each sorted node receives its personalize weight (type-asserted from
//     `any` to T) if present; nodes without an explicit weight are seeded
//     with a small baseline (1/n * 0.01) to keep the chain ergodic.
//   - The vector is renormalized so it sums to 1.0.
//
// All iteration is over `sorted` (D-03) so map iteration order cannot
// affect the returned vector.
func buildTeleport[T cmp.Ordered](sorted []T, personalize map[any]float64) map[T]float64 {
	n := len(sorted)
	teleport := make(map[T]float64, n)

	if len(personalize) == 0 {
		uniform := 1.0 / float64(n)
		for _, k := range sorted {
			teleport[k] = uniform
		}
		return teleport
	}

	// Seed sorted-iteration order from the personalize map. Type-assert
	// each key into T; non-T keys are silently skipped (caller contract).
	baseline := (1.0 / float64(n)) * 0.01
	total := 0.0
	for _, k := range sorted {
		if w, ok := personalize[any(k)]; ok && w > 0 {
			teleport[k] = w
		} else {
			teleport[k] = baseline
		}
		total += teleport[k]
	}

	// Renormalize over the sorted node set so the vector sums to 1.0.
	if total > 0 {
		for _, k := range sorted {
			teleport[k] /= total
		}
	}

	return teleport
}

// RankNodes runs PageRank and returns nodes sorted by score descending.
// Tie-break (equal scores) follows the stable-key NodeID order (GRAPH-03):
// the secondary key is the node ID ascending, so equal-score nodes appear
// in sorted-key order.
func RankNodes[T cmp.Ordered](nodes []T, edges map[T]map[T]float64, opts Options) []Ranked[T] {
	scores := PageRank(nodes, edges, opts)
	if len(scores) == 0 {
		return nil
	}

	out := make([]Ranked[T], 0, len(scores))
	// Iterate scores via a sorted key set so the input slice we pass to
	// sort.SliceStable is itself deterministic before sorting (defense in
	// depth: sort.SliceStable preserves equal-key order, but we want the
	// PRE-sort order to be deterministic too in case any future change
	// adds a non-stable sort).
	keys := make([]T, 0, len(scores))
	for k := range scores {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		out = append(out, Ranked[T]{Node: k, Score: scores[k]})
	}

	// Primary key: score descending. Secondary key: node ascending (stable).
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Node < out[j].Node
	})
	return out
}
