package graph

import "context"

// Ranker is the Phase 64 consumption seam. Both methods carry graph_version
// in their response so callers can detect when their cached result is
// stale relative to a subsequent graph advance.
type Ranker interface {
	// Rank returns the top-Limit ranked nodes for (repo, projection),
	// stamped with the workspace's current graph_version. Each RankedNode
	// carries its own ScoreStatus computed at READ time per D-07.
	Rank(ctx context.Context, req RankRequest) (RankResponse, error)

	// Status returns the per-projection summary (node count, exact / stale
	// / approximate / missing tally) for one workspace.
	Status(ctx context.Context, repoID string) (RankStatus, error)
}

// RankReader is the read-side seam the production Ranker consumes. Tests
// stub this; the Phase 62 P03 daemon wiring binds it to a thin adapter on
// *store.Store. Phase 62 P02 ships only the Status path concretely; the
// Rank path returns an empty Nodes slice with the correct GraphVersion
// header (P03 fills the actual node-list query).
type RankReader interface {
	CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error)
	ProjectionStatus(ctx context.Context, repoID string) (map[string]ProjectionStatus, error)
}

// RankRequest is the input to Ranker.Rank.
type RankRequest struct {
	RepoID        string
	Projection    string   // "call_graph"
	SeedFiles     []string // GRAPH-02 personalized PR seed
	SeedSymbolIDs []NodeID // GRAPH-02
	Limit         int
}

// RankResponse is the output of Ranker.Rank.
type RankResponse struct {
	GraphVersion    uint64
	EnrichmentLevel string // from Phase 61 cascade: "skeleton", "validated", ...
	Nodes           []RankedNode
}

// RankedNode is one entry in RankResponse.Nodes.
type RankedNode struct {
	NodeID      NodeID
	Score       float64
	ScoreStatus ScoreStatus
}

// RankStatus is the output of Ranker.Status.
type RankStatus struct {
	GraphVersion  uint64
	PerProjection map[string]ProjectionStatus
}

// ProjectionStatus is the per-projection score-status histogram.
type ProjectionStatus struct {
	NodeCount    int
	ExactCount   int
	StaleCount   int
	ApproxCount  int
	MissingCount int
}

// engineRanker is the production Ranker implementation. It holds a
// RankReader and synthesises RankResponse / RankStatus values.
type engineRanker struct {
	reader RankReader
}

// NewRanker wraps the supplied RankReader.
func NewRanker(r RankReader) Ranker {
	return &engineRanker{reader: r}
}

func (r *engineRanker) Rank(ctx context.Context, req RankRequest) (RankResponse, error) {
	if r == nil || r.reader == nil {
		return RankResponse{}, errNilRanker
	}
	gv, err := r.reader.CurrentGraphVersion(ctx, req.RepoID)
	if err != nil {
		return RankResponse{}, err
	}
	// Phase 62 P02 ships only the GraphVersion stamp; P03 fills the node
	// list when the scheduler + frontier are wired. Returning an empty
	// Nodes slice keeps the signature stable for Phase 64 consumers.
	return RankResponse{
		GraphVersion: gv,
		Nodes:        nil,
	}, nil
}

func (r *engineRanker) Status(ctx context.Context, repoID string) (RankStatus, error) {
	if r == nil || r.reader == nil {
		return RankStatus{}, errNilRanker
	}
	gv, err := r.reader.CurrentGraphVersion(ctx, repoID)
	if err != nil {
		return RankStatus{}, err
	}
	per, err := r.reader.ProjectionStatus(ctx, repoID)
	if err != nil {
		return RankStatus{}, err
	}
	if per == nil {
		per = map[string]ProjectionStatus{}
	}
	return RankStatus{
		GraphVersion:  gv,
		PerProjection: per,
	}, nil
}

// errNilRanker is a sentinel for misconfigured construction. Production
// wiring constructs the ranker once at daemon bootstrap with a non-nil
// reader; the sentinel exists so unit tests asserting nil-safety can match
// it without panic.
var errNilRanker = rankerError("graph: ranker has nil reader")

type rankerError string

func (e rankerError) Error() string { return string(e) }
