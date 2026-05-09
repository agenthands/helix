// Phase 63 P63-02 Task 1: IsQuiescent accessor tests.

package graph_test

import (
	"testing"

	graphpkg "github.com/agenthands/helix/internal/semantic/graph"
)

// TestRankScheduler_IsQuiescent_TrueOnEmptyState: a fresh scheduler with
// no pending work and no in-flight repair returns true.
func TestRankScheduler_IsQuiescent_TrueOnEmptyState(t *testing.T) {
	s := graphpkg.NewRankScheduler("repo", graphpkg.SchedulerConfig{
		Damping: 0.85, Epsilon: 1e-6, MaxIter: 50,
	}, nil, nil, nil)
	if !s.IsQuiescent() {
		t.Errorf("fresh scheduler IsQuiescent: got false, want true")
	}
}

// TestRankScheduler_IsQuiescent_NilSafe: a nil receiver is trivially
// quiescent (the gate uses nil-safe access patterns).
func TestRankScheduler_IsQuiescent_NilSafe(t *testing.T) {
	var s *graphpkg.RankScheduler
	if !s.IsQuiescent() {
		t.Errorf("nil RankScheduler IsQuiescent: got false, want true")
	}
}
