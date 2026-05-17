package semantic

// D-09 INVARIANT: tests in this file assert that handleGetChangeImpactGraph
// NEVER calls BeginSnapshot / CommitSnapshot / AbortSnapshot / WriteSnapshotFacts.
// The canary recorder below panics / fails the test immediately on any forbidden call.

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)

// ---------- D-09 / D-13 recorder canary for get_change_impact_graph ----------

type changeImpactStoreRec struct {
	t *testing.T

	graphVersion      uint64
	latestSnapshot    uint64
	overlayHasPending bool

	// Forbidden write canaries.
	beginSnapshotCalls      atomic.Int64
	commitSnapshotCalls     atomic.Int64
	abortSnapshotCalls      atomic.Int64
	writeSnapshotFactsCalls atomic.Int64
}

func (r *changeImpactStoreRec) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	return r.latestSnapshot, nil
}
func (r *changeImpactStoreRec) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return r.graphVersion, nil
}
func (r *changeImpactStoreRec) OverlayHasPendingRows(repoID string) bool {
	return r.overlayHasPending
}
func (r *changeImpactStoreRec) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
) {
	return nil, nil, nil
}
func (r *changeImpactStoreRec) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
	return 0, nil
}
func (r *changeImpactStoreRec) OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) ([]string, uint64, error) {
	return nil, 0, nil
}
func (r *changeImpactStoreRec) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (uint64, bool, error) {
	return 0, false, nil
}

// Forbidden write methods — NOT on StoreAccessor interface; only for canary enforcement.
func (r *changeImpactStoreRec) BeginSnapshot(ctx context.Context, repoID string) (uint64, error) {
	r.beginSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: get_change_impact_graph handler must NOT call BeginSnapshot")
	return 0, nil
}
func (r *changeImpactStoreRec) CommitSnapshot(ctx context.Context, snapID uint64) error {
	r.commitSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: get_change_impact_graph handler must NOT call CommitSnapshot")
	return nil
}
func (r *changeImpactStoreRec) AbortSnapshot(ctx context.Context, snapID uint64) error {
	r.abortSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: get_change_impact_graph handler must NOT call AbortSnapshot")
	return nil
}
func (r *changeImpactStoreRec) WriteSnapshotFacts(ctx context.Context, snapID uint64, facts any) error {
	r.writeSnapshotFactsCalls.Add(1)
	r.t.Fatalf("D-09 violation: get_change_impact_graph handler must NOT call WriteSnapshotFacts")
	return nil
}

// ---------- Fake ImpactLookupAccessor ----------

// fixImpactLookupAccessor implements ImpactLookupAccessor with a canned result.
type fixImpactLookupAccessor struct {
	impacts []integ.Impact
	err     error
}

func (f *fixImpactLookupAccessor) ExpandFrom(ctx context.Context, ws workspace.WorkspaceKey, sym integ.SymbolID, depth int) ([]integ.Impact, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.impacts, nil
}

func (f *fixImpactLookupAccessor) Status(ctx context.Context, ws workspace.WorkspaceKey) (integ.SemanticStatus, error) {
	return integ.SemanticStatus{State: integ.StatusReady, GraphVersion: 42}, nil
}

// ---------- Fake SymbolByNameAccessor for seed resolution ----------

type fixSymbolByNameForImpact struct {
	results []integ.SymbolID
	err     error
}

func (f *fixSymbolByNameForImpact) QuerySymbolByName(ctx context.Context, repoID, path, name string) ([]integ.SymbolID, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

// ---------- Fake ExtractorRunAccessor ----------

type fixExtractorRunForImpact struct{}

func (f *fixExtractorRunForImpact) LatestExtractorRunID(ctx context.Context, repoID string) (string, error) {
	return "run-42", nil
}

// ---------- Test harness helper ----------

// newChangeImpactHarness sets up a SemanticSkill wired for get_change_impact_graph tests.
func newChangeImpactHarness(t *testing.T, mode string, impacts []integ.Impact) (*SemanticSkill, *changeImpactStoreRec) {
	t.Helper()
	s := &SemanticSkill{}
	_ = s.Init(skill.SkillDeps{})

	sess := &mcp.SessionInfo{SessionID: "test-change-impact", Mode: mode}
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/repo-impact", Language: "go", Toolchain: "go1.22"}
	s.SetSessionAccessor(&mockSessionAccessor{ws: ws, sess: sess})

	store := &changeImpactStoreRec{
		t:              t,
		graphVersion:   42,
		latestSnapshot: 7,
	}
	s.SetStore(store)
	s.SetExtractorRun(&fixExtractorRunForImpact{})
	s.SetSymbolByName(&fixSymbolByNameForImpact{
		results: []integ.SymbolID{"sym:pkg:A"},
	})
	s.SetImpactLookup(&fixImpactLookupAccessor{impacts: impacts})

	return s, store
}

// makeImpacts builds a slice of integ.Impact with the given number of
// entries, each with `edgesPerNode` evidence edges of the given internal kind.
func makeImpacts(count, edgesPerNode int, internalKind string, confidence float64) []integ.Impact {
	impacts := make([]integ.Impact, 0, count)
	for i := range count {
		edges := make([]integ.Edge, 0, edgesPerNode)
		for j := range edgesPerNode {
			edges = append(edges, integ.Edge{
				From:       integ.SymbolID("sym:A"),
				To:         integ.SymbolID("sym:B" + string(rune('0'+j))),
				Kind:       internalKind,
				Confidence: confidence,
			})
		}
		impacts = append(impacts, integ.Impact{
			SymbolID:   integ.SymbolID("sym:Node" + string(rune('A'+i%26))),
			Confidence: confidence,
			Evidence:   integ.Evidence{Edges: edges},
		})
	}
	return impacts
}

// ---------- Tests ----------

// TestGetChangeImpactGraph_HappyPath verifies the nominal subgraph response:
// 3 impacts with 2 edges each → Nodes (3), Edges (6), Truncated=false.
func TestGetChangeImpactGraph_HappyPath(t *testing.T) {
	impacts := makeImpacts(3, 2, "CALLS", 0.9)
	s, _ := newChangeImpactHarness(t, "review", impacts)

	ctx := context.Background()
	result := s.handleGetChangeImpactGraph(ctx, GetChangeImpactGraphArgs{
		Seed:     SeedInput{SymbolID: "sym:A"},
		MaxDepth: 2,
	})

	if result.IsError {
		t.Fatalf("expected success; got IsError=true: %v", extractText(t, result))
	}

	var resp GetChangeImpactGraphResult
	if err := json.Unmarshal([]byte(extractText(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(resp.Nodes))
	}
	if len(resp.Edges) != 6 {
		t.Errorf("expected 6 edges, got %d", len(resp.Edges))
	}
	if resp.Truncated {
		t.Errorf("expected Truncated=false for 3 nodes")
	}
	if resp.NodesCount != 3 {
		t.Errorf("expected NodesCount=3, got %d", resp.NodesCount)
	}
	if resp.EdgesCount != 6 {
		t.Errorf("expected EdgesCount=6, got %d", resp.EdgesCount)
	}
	if resp.ReachedDepth != 2 {
		t.Errorf("expected ReachedDepth=2, got %d", resp.ReachedDepth)
	}
	if resp.Freshness.GraphVersion == 0 {
		t.Errorf("expected non-zero GraphVersion in freshness")
	}
	// All edges should have a valid non-empty EdgeKind.
	for i, e := range resp.Edges {
		if e.EdgeKind == "" {
			t.Errorf("edges[%d].EdgeKind is empty", i)
		}
	}
}

// TestGetChangeImpactGraph_ReviewTierEnforced asserts that sessions with
// mode="read" receive PermissionDenied, while mode="review" succeeds.
func TestGetChangeImpactGraph_ReviewTierEnforced(t *testing.T) {
	impacts := makeImpacts(1, 1, "CALLS", 0.9)

	// Read mode must be rejected.
	s, _ := newChangeImpactHarness(t, "read", impacts)
	ctx := context.Background()
	result := s.handleGetChangeImpactGraph(ctx, GetChangeImpactGraphArgs{
		Seed: SeedInput{SymbolID: "sym:A"},
	})
	if !result.IsError {
		t.Errorf("expected IsError=true for mode=read; got success")
	}
	txt := extractText(t, result)
	if txt == "" {
		t.Errorf("expected non-empty error text for PermissionDenied")
	}

	// Review mode must succeed.
	s2, _ := newChangeImpactHarness(t, "review", impacts)
	result2 := s2.handleGetChangeImpactGraph(ctx, GetChangeImpactGraphArgs{
		Seed: SeedInput{SymbolID: "sym:A"},
	})
	if result2.IsError {
		t.Errorf("expected success for mode=review; got IsError=true: %v", extractText(t, result2))
	}
}

// TestGetChangeImpactGraph_ReadOnlyCanary verifies the D-09 invariant:
// the handler MUST NOT call BeginSnapshot / CommitSnapshot / AbortSnapshot /
// WriteSnapshotFacts. The recorder store fails the test on any such call.
func TestGetChangeImpactGraph_ReadOnlyCanary(t *testing.T) {
	impacts := makeImpacts(2, 1, "CALLS", 0.9)
	s, store := newChangeImpactHarness(t, "review", impacts)
	ctx := context.Background()

	s.handleGetChangeImpactGraph(ctx, GetChangeImpactGraphArgs{
		Seed: SeedInput{SymbolID: "sym:A"},
	})

	// Verify none of the write-canary counters were incremented.
	if n := store.beginSnapshotCalls.Load(); n != 0 {
		t.Errorf("D-09: BeginSnapshot called %d times", n)
	}
	if n := store.commitSnapshotCalls.Load(); n != 0 {
		t.Errorf("D-09: CommitSnapshot called %d times", n)
	}
	if n := store.abortSnapshotCalls.Load(); n != 0 {
		t.Errorf("D-09: AbortSnapshot called %d times", n)
	}
	if n := store.writeSnapshotFactsCalls.Load(); n != 0 {
		t.Errorf("D-09: WriteSnapshotFacts called %d times", n)
	}
}

// TestGetChangeImpactGraph_DepthClamping checks that MaxDepth=0 uses default 2
// and MaxDepth=10 is clamped to 5.
func TestGetChangeImpactGraph_DepthClamping(t *testing.T) {
	impacts := makeImpacts(1, 1, "CALLS", 0.9)

	cases := []struct {
		inputDepth    int
		expectedDepth int
	}{
		{0, 2},
		{-1, 2},
		{10, 5},
		{5, 5},
		{3, 3},
	}
	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			s, _ := newChangeImpactHarness(t, "review", impacts)
			ctx := context.Background()
			result := s.handleGetChangeImpactGraph(ctx, GetChangeImpactGraphArgs{
				Seed:     SeedInput{SymbolID: "sym:A"},
				MaxDepth: tc.inputDepth,
			})
			if result.IsError {
				t.Fatalf("unexpected error: %v", extractText(t, result))
			}
			var resp GetChangeImpactGraphResult
			if err := json.Unmarshal([]byte(extractText(t, result)), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if resp.ReachedDepth != tc.expectedDepth {
				t.Errorf("MaxDepth=%d → want ReachedDepth=%d, got %d",
					tc.inputDepth, tc.expectedDepth, resp.ReachedDepth)
			}
		})
	}
}

// TestGetChangeImpactGraph_NodeCap asserts that 250 impacts result in
// Truncated=true, NodesCount=250, and len(Nodes)=200.
func TestGetChangeImpactGraph_NodeCap(t *testing.T) {
	impacts := makeImpacts(250, 1, "CALLS", 0.9)
	s, _ := newChangeImpactHarness(t, "review", impacts)

	ctx := context.Background()
	result := s.handleGetChangeImpactGraph(ctx, GetChangeImpactGraphArgs{
		Seed:     SeedInput{SymbolID: "sym:A"},
		MaxDepth: 2,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", extractText(t, result))
	}
	var resp GetChangeImpactGraphResult
	if err := json.Unmarshal([]byte(extractText(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !resp.Truncated {
		t.Errorf("expected Truncated=true for 250 impacts")
	}
	if resp.NodesCount != 250 {
		t.Errorf("expected NodesCount=250, got %d", resp.NodesCount)
	}
	if len(resp.Nodes) != 200 {
		t.Errorf("expected len(Nodes)=200 (hard cap), got %d", len(resp.Nodes))
	}
}

// TestGetChangeImpactGraph_EdgeKindsFilter checks that passing edge_kinds=["calls"]
// filters out edges of other kinds (e.g., "IMPORTS" which maps to "references").
func TestGetChangeImpactGraph_EdgeKindsFilter(t *testing.T) {
	// Build impacts with mixed edge kinds: CALLS and IMPORTS.
	callsEdge := integ.Edge{From: "sym:A", To: "sym:B", Kind: "CALLS", Confidence: 0.9}
	importsEdge := integ.Edge{From: "sym:A", To: "sym:C", Kind: "IMPORTS", Confidence: 0.9}
	impacts := []integ.Impact{
		{SymbolID: "sym:NodeA", Confidence: 0.9, Evidence: integ.Evidence{Edges: []integ.Edge{callsEdge, importsEdge}}},
	}
	s, _ := newChangeImpactHarness(t, "review", impacts)
	ctx := context.Background()

	// Filter to only "calls" surface kind.
	result := s.handleGetChangeImpactGraph(ctx, GetChangeImpactGraphArgs{
		Seed:      SeedInput{SymbolID: "sym:A"},
		MaxDepth:  2,
		EdgeKinds: []string{"calls"},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", extractText(t, result))
	}
	var resp GetChangeImpactGraphResult
	if err := json.Unmarshal([]byte(extractText(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Only "calls" edges should remain; IMPORTS (→ "references") should be filtered out.
	for i, e := range resp.Edges {
		if e.EdgeKind != EdgeKindCalls {
			t.Errorf("edges[%d]: expected EdgeKind=%q, got %q", i, EdgeKindCalls, e.EdgeKind)
		}
	}
	if len(resp.Edges) == 0 {
		t.Errorf("expected at least one 'calls' edge to survive the filter")
	}
}

// TestGetChangeImpactGraph_ConfidenceCap checks OQ-1: when any impact has
// Confidence < 0.8 → result.ConfidenceCap != nil with Value=0.6 and
// Reason="type_resolver_tier_3"; all edge Confidence values must be ≤ 0.6.
func TestGetChangeImpactGraph_ConfidenceCap(t *testing.T) {
	// One impact at confidence 0.5 — below the 0.8 trigger.
	impacts := []integ.Impact{
		{
			SymbolID:   "sym:NodeA",
			Confidence: 0.5,
			Evidence: integ.Evidence{
				Edges: []integ.Edge{
					{From: "sym:A", To: "sym:B", Kind: "CALLS", Confidence: 0.9}, // will be capped to 0.6
					{From: "sym:A", To: "sym:C", Kind: "CALLS", Confidence: 0.4}, // already below cap
				},
			},
		},
		{
			SymbolID:   "sym:NodeB",
			Confidence: 0.95,
			Evidence: integ.Evidence{
				Edges: []integ.Edge{
					{From: "sym:B", To: "sym:D", Kind: "CALLS", Confidence: 0.95}, // will be capped
				},
			},
		},
	}
	s, _ := newChangeImpactHarness(t, "review", impacts)
	ctx := context.Background()

	result := s.handleGetChangeImpactGraph(ctx, GetChangeImpactGraphArgs{
		Seed:     SeedInput{SymbolID: "sym:A"},
		MaxDepth: 2,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %v", extractText(t, result))
	}
	var resp GetChangeImpactGraphResult
	if err := json.Unmarshal([]byte(extractText(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.ConfidenceCap == nil {
		t.Fatalf("expected ConfidenceCap to be set (OQ-1 trigger: any impact.Confidence < 0.8)")
	}
	if resp.ConfidenceCap.Value != 0.6 {
		t.Errorf("expected ConfidenceCap.Value=0.6, got %f", resp.ConfidenceCap.Value)
	}
	if resp.ConfidenceCap.Reason != "type_resolver_tier_3" {
		t.Errorf("expected ConfidenceCap.Reason=%q, got %q", "type_resolver_tier_3", resp.ConfidenceCap.Reason)
	}

	for i, e := range resp.Edges {
		if e.Confidence > 0.6 {
			t.Errorf("edges[%d].Confidence=%f exceeds cap 0.6 (ConfidenceCap not applied)",
				i, e.Confidence)
		}
	}
}

// TestGetChangeImpactGraph_SeedNotFound checks that when resolveSeed returns
// ResolutionNotFound, the response has FallbackReason="symbol_not_found" and
// IsError=false (graceful degradation).
func TestGetChangeImpactGraph_SeedNotFound(t *testing.T) {
	s, _ := newChangeImpactHarness(t, "review", nil)
	// Override SymbolByName to return no results.
	s.SetSymbolByName(&fixSymbolByNameForImpact{results: nil})

	ctx := context.Background()
	result := s.handleGetChangeImpactGraph(ctx, GetChangeImpactGraphArgs{
		Seed: SeedInput{FilePath: "pkg/foo.go", SymbolName: "DoesNotExist"},
	})
	if result.IsError {
		t.Fatalf("expected IsError=false for not_found; got error: %v", extractText(t, result))
	}
	var resp GetChangeImpactGraphResult
	if err := json.Unmarshal([]byte(extractText(t, result)), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.FallbackReason != "symbol_not_found" {
		t.Errorf("expected FallbackReason=%q, got %q", "symbol_not_found", resp.FallbackReason)
	}
	if resp.Freshness.GraphVersion == 0 {
		t.Errorf("expected Freshness to be populated even on not_found path")
	}
}
