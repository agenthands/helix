package repomap

import (
	"math"
	"sort"
)

// PageRank computes PageRank scores for all files in the graph using
// power iteration. If personalization is non-nil and non-empty, it is
// used as the teleportation vector (Personalized PageRank); otherwise
// uniform teleportation is used.
//
// Parameters:
//   - damping: probability of following a link (typically 0.85)
//   - epsilon: convergence threshold (sum of abs rank differences)
//   - maxIter: maximum number of iterations
//   - personalization: optional map of file -> teleport weight
//
// Per T-28-02: maxIter caps computation to mitigate DoS on large graphs.
func (g *FileGraph) PageRank(damping float64, epsilon float64, maxIter int, personalization map[string]float64) map[string]float64 {
	g.mu.RLock()
	n := len(g.Files)
	if n == 0 {
		g.mu.RUnlock()
		return nil
	}

	files := make([]string, 0, n)
	for f := range g.Files {
		files = append(files, f)
	}

	// Copy edges under lock.
	edges := make(map[string]map[string]float64, len(g.Edges))
	for src, targets := range g.Edges {
		dst := make(map[string]float64, len(targets))
		for k, v := range targets {
			dst[k] = v
		}
		edges[src] = dst
	}
	g.mu.RUnlock()

	// Initialize teleport vector.
	teleport := make(map[string]float64, n)
	if len(personalization) > 0 {
		total := 0.0
		for _, w := range personalization {
			total += w
		}
		for f, w := range personalization {
			teleport[f] = w / total
		}
		// Fill remaining nodes with small baseline.
		for _, f := range files {
			if _, ok := teleport[f]; !ok {
				teleport[f] = (1.0 / float64(n)) * 0.01
			}
		}
		// Re-normalize so sum = 1.0.
		total = 0.0
		for _, w := range teleport {
			total += w
		}
		for f := range teleport {
			teleport[f] /= total
		}
	} else {
		for _, f := range files {
			teleport[f] = 1.0 / float64(n)
		}
	}

	// Initialize rank = teleport.
	rank := make(map[string]float64, n)
	for f, w := range teleport {
		rank[f] = w
	}

	// Precompute which files have outgoing edges and their total weights.
	outWeight := make(map[string]float64, len(edges))
	for src, targets := range edges {
		total := 0.0
		for _, w := range targets {
			total += w
		}
		outWeight[src] = total
	}

	fn := float64(n)

	for iter := 0; iter < maxIter; iter++ {
		// Compute dangling rank: sum of rank for nodes with no outgoing edges.
		danglingRank := 0.0
		for _, f := range files {
			if _, hasOut := outWeight[f]; !hasOut {
				danglingRank += rank[f]
			}
		}

		newRank := make(map[string]float64, n)

		// Teleport + dangling redistribution component.
		for _, f := range files {
			newRank[f] = (1-damping)*teleport[f] + damping*danglingRank/fn
		}

		// Link component.
		for src, targets := range edges {
			tw := outWeight[src]
			if tw == 0 {
				continue
			}
			contribution := damping * rank[src] / tw
			for dst, w := range targets {
				newRank[dst] += contribution * w
			}
		}

		// Check convergence.
		diff := 0.0
		for _, f := range files {
			diff += math.Abs(newRank[f] - rank[f])
		}
		rank = newRank
		if diff < epsilon {
			break
		}
	}

	return rank
}

// RankFiles runs PageRank and returns files sorted descending by score.
// Convenience wrapper with default epsilon=1e-6 and maxIter=100.
func (g *FileGraph) RankFiles(damping float64, personalization map[string]float64) []RankedFile {
	scores := g.PageRank(damping, 1e-6, 100, personalization)
	if scores == nil {
		return nil
	}

	ranked := make([]RankedFile, 0, len(scores))
	for path, score := range scores {
		ranked = append(ranked, RankedFile{Path: path, Score: score})
	}

	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	return ranked
}
