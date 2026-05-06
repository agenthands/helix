package repomap

import (
	"github.com/agenthands/helix/internal/graph"
)

// snapshot copies the file/edge maps under g.mu.RLock so the engine can
// run lock-free. Per D-04, the public FileGraph API is preserved.
func (g *FileGraph) snapshot() (nodes []string, edges map[string]map[string]float64) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	n := len(g.Files)
	if n == 0 {
		return nil, nil
	}
	nodes = make([]string, 0, n)
	for f := range g.Files {
		nodes = append(nodes, f)
	}
	edges = make(map[string]map[string]float64, len(g.Edges))
	for src, targets := range g.Edges {
		dst := make(map[string]float64, len(targets))
		for k, v := range targets {
			dst[k] = v
		}
		edges[src] = dst
	}
	return nodes, edges
}

// toAnyMap converts a string-keyed personalization map to the engine's
// `map[any]float64` shape. Returns nil for empty/nil input.
func toAnyMap(p map[string]float64) map[any]float64 {
	if len(p) == 0 {
		return nil
	}
	out := make(map[any]float64, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
}

// PageRank computes PageRank scores for all files in the graph by
// delegating to the deterministic engine in internal/graph.
//
// Per Phase 62 D-04, the FileGraph public API is preserved verbatim.
// Per T-28-02: maxIter caps computation to mitigate DoS on large graphs.
func (g *FileGraph) PageRank(damping float64, epsilon float64, maxIter int, personalization map[string]float64) map[string]float64 {
	nodes, edges := g.snapshot()
	if nodes == nil {
		return nil
	}
	return graph.PageRank(nodes, edges, graph.Options{
		Damping:     damping,
		Epsilon:     epsilon,
		MaxIter:     maxIter,
		Personalize: toAnyMap(personalization),
	})
}

// RankFiles runs PageRank and returns files sorted descending by score.
// Convenience wrapper with default epsilon=1e-6 and maxIter=100.
//
// Equal-score nodes are ordered by path ascending — the GRAPH-03
// stable-key NodeID tiebreak that flows from graph.RankNodes.
func (g *FileGraph) RankFiles(damping float64, personalization map[string]float64) []RankedFile {
	nodes, edges := g.snapshot()
	if nodes == nil {
		return nil
	}
	ranked := graph.RankNodes(nodes, edges, graph.Options{
		Damping:     damping,
		Epsilon:     1e-6,
		MaxIter:     100,
		Personalize: toAnyMap(personalization),
	})
	if len(ranked) == 0 {
		return nil
	}
	out := make([]RankedFile, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, RankedFile{Path: r.Node, Score: r.Score})
	}
	return out
}
