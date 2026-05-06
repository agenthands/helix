// Phase 62 P02 RED gate — failing tests for the Ranker interface.
//
// Pin two contract bullets for Phase 64 consumption:
//
//  1. Rank carries graph_version on every response.
//  2. Status returns a per-projection ProjectionStatus map.
package graph

import (
	"context"
	"testing"
)

// fakeRankReader is the read-side seam ranker production wires through.
type fakeRankReader struct {
	gv     uint64
	status map[string]ProjectionStatus
}

func (f *fakeRankReader) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return f.gv, nil
}

func (f *fakeRankReader) ProjectionStatus(ctx context.Context, repoID string) (map[string]ProjectionStatus, error) {
	return f.status, nil
}

func TestRanker_RankCarriesGraphVersion(t *testing.T) {
	r := NewRanker(&fakeRankReader{gv: 42})
	resp, err := r.Rank(context.Background(), RankRequest{
		RepoID:     "ws",
		Projection: "call_graph",
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("Rank: %v", err)
	}
	if resp.GraphVersion != 42 {
		t.Errorf("GraphVersion=%d, want 42", resp.GraphVersion)
	}
}

func TestRanker_StatusCarriesPerProjectionStatus(t *testing.T) {
	r := NewRanker(&fakeRankReader{
		gv: 9,
		status: map[string]ProjectionStatus{
			"call_graph": {NodeCount: 3, ExactCount: 1, StaleCount: 1, MissingCount: 1},
		},
	})
	st, err := r.Status(context.Background(), "ws")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.GraphVersion != 9 {
		t.Errorf("GraphVersion=%d, want 9", st.GraphVersion)
	}
	if st.PerProjection == nil {
		t.Fatal("PerProjection is nil")
	}
	cg, ok := st.PerProjection["call_graph"]
	if !ok {
		t.Fatalf("call_graph projection missing; got %v", st.PerProjection)
	}
	if cg.NodeCount != 3 {
		t.Errorf("NodeCount=%d, want 3", cg.NodeCount)
	}
}
