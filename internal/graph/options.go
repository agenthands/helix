// Package graph hosts the deterministic, generic PageRank engine used by
// the repomap and (in later Phase 62 plans) the semantic-graph subsystem.
//
// Determinism contract (D-03): every node-keyed iteration in the engine
// runs over a pre-sorted slice of node IDs. Map ranges only appear in
// commutative aggregation contexts (sum of edge weights / contributions)
// where order is irrelevant.
//
// Import invariant (D-01, hard): this package depends on the standard
// library only — `cmp`, `math`, `sort`. It MUST NOT acquire any project
// imports. The downstream packages (repomap, semantic) are the consumers.
package graph

// Options carry tunables for PageRank. Zero / non-positive values fall
// back to documented defaults via withDefaults().
//
//   - Damping default 0.85 (must lie in (0, 1) — values <=0 or >=1 reset).
//   - Epsilon default 1e-6 (sum of |delta| convergence threshold).
//   - MaxIter default 100.
//   - Personalize: optional teleport vector. Keys are stored as `any` so
//     this struct is type-parameter free; at PageRank call time the
//     engine asserts each key to T and re-normalizes over the active
//     node set. Nil / empty → uniform teleport.
type Options struct {
	Damping     float64
	Epsilon     float64
	MaxIter     int
	Personalize map[any]float64
}

// withDefaults returns a copy with sentinel values replaced by defaults.
func (o Options) withDefaults() Options {
	if o.Damping <= 0 || o.Damping >= 1 {
		o.Damping = 0.85
	}
	if o.Epsilon <= 0 {
		o.Epsilon = 1e-6
	}
	if o.MaxIter <= 0 {
		o.MaxIter = 100
	}
	return o
}
