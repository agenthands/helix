package repomap

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestGraph builds a FileGraph from an edge map for testing.
func newTestGraph(edges map[string]map[string]float64) *FileGraph {
	g := NewFileGraph()
	for src, targets := range edges {
		for dst, w := range targets {
			g.addEdge(src, dst, w)
		}
	}
	return g
}

func TestFileGraph_PageRank_UniformChain(t *testing.T) {
	// 3-node chain: A->B->C
	// B should rank higher than A (B gets rank from A).
	g := newTestGraph(map[string]map[string]float64{
		"A": {"B": 1.0},
		"B": {"C": 1.0},
	})
	// C must also be in graph (added by addEdge as dst).

	scores := g.PageRank(0.85, 1e-6, 100, nil)
	require.NotNil(t, scores)
	assert.Len(t, scores, 3)

	// Scores should sum to ~1.0.
	total := 0.0
	for _, s := range scores {
		total += s
	}
	assert.InDelta(t, 1.0, total, 0.01, "scores should sum to ~1.0")

	// B gets rank from A, so B > A.
	assert.Greater(t, scores["B"], scores["A"], "B should rank higher than A in chain A->B->C")
}

func TestFileGraph_PageRank_Personalized(t *testing.T) {
	// Star graph: A->B, A->C, A->D
	// Personalize on A: A's neighbors (B, C, D) should score higher than in uniform.
	g := newTestGraph(map[string]map[string]float64{
		"A": {"B": 1.0, "C": 1.0, "D": 1.0},
	})

	personalization := map[string]float64{"A": 1.0}
	scores := g.PageRank(0.85, 1e-6, 100, personalization)
	require.NotNil(t, scores)

	// With personalization on A, A's direct neighbors should get more rank.
	// B, C, D should be higher than they would be in a uniform teleport scenario.
	// A itself should have high score due to teleport.
	assert.Greater(t, scores["A"], scores["B"], "seed node A should have highest score")

	// All neighbors of A should have equal scores.
	assert.InDelta(t, scores["B"], scores["C"], 0.001)
	assert.InDelta(t, scores["B"], scores["D"], 0.001)

	// Scores should sum to ~1.0.
	total := 0.0
	for _, s := range scores {
		total += s
	}
	assert.InDelta(t, 1.0, total, 0.01)
}

func TestFileGraph_PageRank_DanglingNodes(t *testing.T) {
	// A->B, C is dangling (no outgoing edges).
	// C's rank should be redistributed; convergence should not fail.
	g := newTestGraph(map[string]map[string]float64{
		"A": {"B": 1.0},
	})
	// Add C as a dangling node (no outgoing edges).
	g.Files["C"] = true

	scores := g.PageRank(0.85, 1e-6, 100, nil)
	require.NotNil(t, scores)
	assert.Len(t, scores, 3)

	// All scores should be positive (rank redistributed from dangling node).
	for f, s := range scores {
		assert.Greater(t, s, 0.0, "file %s should have positive score", f)
	}

	// Scores should sum to ~1.0.
	total := 0.0
	for _, s := range scores {
		total += s
	}
	assert.InDelta(t, 1.0, total, 0.01)
}

func TestFileGraph_PageRank_EmptyGraph(t *testing.T) {
	g := NewFileGraph()
	scores := g.PageRank(0.85, 1e-6, 100, nil)
	assert.Nil(t, scores, "empty graph should return nil")
}

func TestFileGraph_PageRank_SingleNodeSelfLoop(t *testing.T) {
	g := newTestGraph(map[string]map[string]float64{
		"only": {"only": 1.0},
	})

	scores := g.PageRank(0.85, 1e-6, 100, nil)
	require.NotNil(t, scores)
	assert.Len(t, scores, 1)
	assert.InDelta(t, 1.0, scores["only"], 0.01, "single node should have score ~1.0")
}

func TestFileGraph_RankFiles_SortedDescending(t *testing.T) {
	// Chain A->B->C: C should rank highest (sink gets most rank).
	g := newTestGraph(map[string]map[string]float64{
		"A": {"B": 1.0},
		"B": {"C": 1.0},
	})

	ranked := g.RankFiles(0.85, nil)
	require.Len(t, ranked, 3)

	// Verify descending order.
	for i := 1; i < len(ranked); i++ {
		assert.GreaterOrEqual(t, ranked[i-1].Score, ranked[i].Score,
			"ranked[%d].Score (%f) should be >= ranked[%d].Score (%f)",
			i-1, ranked[i-1].Score, i, ranked[i].Score)
	}

	// Verify scores are positive and reasonable.
	for _, rf := range ranked {
		assert.Greater(t, rf.Score, 0.0)
		assert.NotEmpty(t, rf.Path)
	}

	// Verify sum ~1.0.
	total := 0.0
	for _, rf := range ranked {
		total += rf.Score
	}
	assert.InDelta(t, 1.0, total, 0.01)
}

// Suppress unused import warning for math.
var _ = math.Abs
