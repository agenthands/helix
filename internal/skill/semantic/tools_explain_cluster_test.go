package semantic

import (
	"context"
	"encoding/json"
	"math"
	"sync/atomic"
	"testing"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)

// ---------- D-09 / D-13 recorder canary for explain_cluster ----------
//
// explainClusterStoreRec implements StoreAccessor and adds forbidden
// snapshot-write canaries. Any regression that reaches BeginSnapshot /
// CommitSnapshot / AbortSnapshot / WriteSnapshotFacts via the handler will
// fail loudly rather than silently.
type explainClusterStoreRec struct {
	t *testing.T

	graphVersion      uint64
	latestSnapshot    uint64
	overlayHasPending bool

	// Per-plan adjacency: supports QueryEffectiveAdjacency for cohesion /
	// conductance computation (OQ-3 compute-on-demand).
	adjacencyOut map[graph.NodeID]map[graph.NodeID]float64

	// Forbidden write canaries.
	beginSnapshotCalls      atomic.Int64
	commitSnapshotCalls     atomic.Int64
	abortSnapshotCalls      atomic.Int64
	writeSnapshotFactsCalls atomic.Int64
}

func (r *explainClusterStoreRec) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	return r.latestSnapshot, nil
}
func (r *explainClusterStoreRec) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return r.graphVersion, nil
}
func (r *explainClusterStoreRec) OverlayHasPendingRows(repoID string) bool {
	return r.overlayHasPending
}
func (r *explainClusterStoreRec) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
) {
	if r.adjacencyOut != nil {
		return r.adjacencyOut, nil, nil
	}
	return nil, nil, nil
}
func (r *explainClusterStoreRec) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
	return 0, nil
}
func (r *explainClusterStoreRec) OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) ([]string, uint64, error) {
	return nil, 0, nil
}
func (r *explainClusterStoreRec) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (uint64, bool, error) {
	return 0, false, nil
}

// Forbidden write methods (NOT on StoreAccessor interface — purely for canary).
func (r *explainClusterStoreRec) BeginSnapshot(ctx context.Context, repoID string) (uint64, error) {
	r.beginSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: explain_cluster handler must NOT call BeginSnapshot")
	return 0, nil
}
func (r *explainClusterStoreRec) CommitSnapshot(ctx context.Context, snapID uint64) error {
	r.commitSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: explain_cluster handler must NOT call CommitSnapshot")
	return nil
}
func (r *explainClusterStoreRec) AbortSnapshot(ctx context.Context, snapID uint64) error {
	r.abortSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: explain_cluster handler must NOT call AbortSnapshot")
	return nil
}
func (r *explainClusterStoreRec) WriteSnapshotFacts(ctx context.Context, snapID uint64, facts any) error {
	r.writeSnapshotFactsCalls.Add(1)
	r.t.Fatalf("D-09 violation: explain_cluster handler must NOT call WriteSnapshotFacts")
	return nil
}

// ---------- Fake accessor implementations ----------

// fixClusterMemberAccessor implements ClusterMemberAccessor with a canned
// member row slice. The limit parameter is respected.
type fixClusterMemberAccessor struct {
	rows []ClusterMemberRow
	err  error
}

func (f *fixClusterMemberAccessor) QueryClusterMembers(ctx context.Context, repoID, projection string, graphVersion, clusterIntID uint64, limit int) ([]ClusterMemberRow, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := append([]ClusterMemberRow(nil), f.rows...)
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

// fixClusterPageRankAccessorForExplain implements ClusterPageRankAccessor with
// a static score map keyed on node ID.
type fixClusterPageRankAccessorForExplain struct {
	scores map[uint64]float64
	err    error
}

func (f *fixClusterPageRankAccessorForExplain) QueryNodePageRanks(ctx context.Context, repoID, projection string, graphVersion uint64, nodeIDs []uint64) (map[uint64]float64, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make(map[uint64]float64, len(nodeIDs))
	for _, id := range nodeIDs {
		if s, ok := f.scores[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

// ---------- Common test helpers ----------

// newSkillForExplainClusterTest builds a SemanticSkill wired with the
// explainClusterStoreRec canary store + supplied accessor fakes.
func newSkillForExplainClusterTest(
	t *testing.T,
	mode string,
	store *explainClusterStoreRec,
	cma ClusterMemberAccessor,
	cpra ClusterPageRankAccessor,
) *SemanticSkill {
	t.Helper()
	s := &SemanticSkill{}
	if err := s.Init(skill.SkillDeps{}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	sess := &mcp.SessionInfo{
		SessionID: "test-explain-cluster-session",
		Mode:      mode,
	}
	ws := workspace.WorkspaceKey{
		RepoRoot: "/tmp/repo-explain-cluster-test", Language: "go", Toolchain: "go1.22",
	}
	s.SetSessionAccessor(&mockSessionAccessor{ws: ws, sess: sess})
	s.SetStore(store)
	if cma != nil {
		s.SetClusterMember(cma)
	}
	if cpra != nil {
		s.SetClusterPageRank(cpra)
	}
	// Wire ExtractorRun so FreshnessV2.status == "current" in happy-path tests.
	s.SetExtractorRun(&fixtureExtractorRun{runID: "snap-72-cluster"})
	return s
}

// unmarshalExplainClusterResult decodes the JSON body of an explain_cluster result.
func unmarshalExplainClusterResult(t *testing.T, raw string) ExplainClusterResult {
	t.Helper()
	var out ExplainClusterResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("unmarshal ExplainClusterResult: %v (raw=%s)", err, raw)
	}
	return out
}

// ---------- Behavior tests ----------

// TestExplainCluster_ValidToken: given a valid cluster_id token from a fixture
// with graphVersion=42, clusterIntID=7, 4 members; result has MemberCount=4,
// MembersReturned=4, Members sorted by PageRank desc, FreshnessV2.GraphVersion==42,
// Cohesion > 0, IsError=false.
func TestExplainCluster_ValidToken(t *testing.T) {
	const graphVersion = uint64(42)
	const clusterIntID = uint64(7)
	const projection = "weak_components"

	// Four members with node IDs 10, 20, 30, 40.
	memberRows := []ClusterMemberRow{
		{NodeID: 10, SymbolID: "pkg:TypeA:MethodX"},
		{NodeID: 20, SymbolID: "pkg:TypeB:methodInternal"},
		{NodeID: 30, SymbolID: "pkg:TypeC:Init"},
		{NodeID: 40, SymbolID: "pkg:TypeD:helper"},
	}
	cma := &fixClusterMemberAccessor{rows: memberRows}

	// PageRank scores: 10→0.9, 20→0.5, 30→0.4, 40→0.1
	cpra := &fixClusterPageRankAccessorForExplain{
		scores: map[uint64]float64{
			10: 0.9,
			20: 0.5,
			30: 0.4,
			40: 0.1,
		},
	}

	// Adjacency: 3 intra-cluster edges (10→20, 20→30, 30→10)
	// n=4, n*(n-1)=12; cohesion = 3/12 = 0.25
	// 0 leaving edges → conductance = 0/(2*3+0) = 0.0
	store := &explainClusterStoreRec{
		t:              t,
		graphVersion:   graphVersion,
		latestSnapshot: 7,
		adjacencyOut: map[graph.NodeID]map[graph.NodeID]float64{
			10: {20: 1.0},
			20: {30: 1.0},
			30: {10: 1.0},
		},
	}

	s := newSkillForExplainClusterTest(t, "read", store, cma, cpra)
	clusterToken := encodeClusterID(projection, graphVersion, clusterIntID)

	res := s.handleExplainCluster(context.Background(), ExplainClusterArgs{
		ClusterID:  clusterToken,
		MaxMembers: 0, // default (200)
	})

	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalExplainClusterResult(t, textOf(res))

	if out.MemberCount != 4 {
		t.Errorf("member_count = %d; want 4", out.MemberCount)
	}
	if out.MembersReturned != 4 {
		t.Errorf("members_returned = %d; want 4", out.MembersReturned)
	}
	if len(out.Members) != 4 {
		t.Fatalf("members len = %d; want 4", len(out.Members))
	}
	// Members must be sorted by PageRank desc.
	if out.Members[0].PageRank < out.Members[1].PageRank {
		t.Errorf("members not sorted by PageRank desc: [0]=%f < [1]=%f", out.Members[0].PageRank, out.Members[1].PageRank)
	}
	// FreshnessV2 must carry the current graph version.
	if out.Freshness.GraphVersion != graphVersion {
		t.Errorf("freshness.graph_version = %d; want %d", out.Freshness.GraphVersion, graphVersion)
	}
	// Cohesion must be > 0 (3 intra edges out of 12 possible).
	if out.Cohesion <= 0 {
		t.Errorf("cohesion = %f; want > 0", out.Cohesion)
	}
	// FallbackReason must be empty on success.
	if out.FallbackReason != "" {
		t.Errorf("fallback_reason = %q; want empty", out.FallbackReason)
	}
}

// TestExplainCluster_StaleClustersID: given cluster_id with graphVersion=41 but
// store returns currentGV=42; result has FallbackReason=="stale_cluster_id",
// IsError==false (stale is a soft signal), Freshness.GraphVersion==42.
func TestExplainCluster_StaleClustersID(t *testing.T) {
	const storeGV = uint64(42)
	const tokenGV = uint64(41) // deliberately stale
	const clusterIntID = uint64(7)
	const projection = "weak_components"

	store := &explainClusterStoreRec{
		t:              t,
		graphVersion:   storeGV,
		latestSnapshot: 5,
	}
	s := newSkillForExplainClusterTest(t, "read", store, nil, nil)
	clusterToken := encodeClusterID(projection, tokenGV, clusterIntID)

	res := s.handleExplainCluster(context.Background(), ExplainClusterArgs{
		ClusterID: clusterToken,
	})

	// Stale-id response is NOT an error envelope.
	if res.IsError {
		t.Fatalf("stale_cluster_id must not be IsError=true; got error: %s", textOf(res))
	}
	out := unmarshalExplainClusterResult(t, textOf(res))

	if out.FallbackReason != "stale_cluster_id" {
		t.Errorf("fallback_reason = %q; want %q", out.FallbackReason, "stale_cluster_id")
	}
	// Freshness must reflect the current graph version (42), not the stale token's (41).
	if out.Freshness.GraphVersion != storeGV {
		t.Errorf("freshness.graph_version = %d; want %d (current)", out.Freshness.GraphVersion, storeGV)
	}
}

// TestExplainCluster_MaxMembersCap: fixture with 5 members, MaxMembers=3 →
// MembersReturned==3, MemberCount==5.
func TestExplainCluster_MaxMembersCap(t *testing.T) {
	const graphVersion = uint64(10)
	const clusterIntID = uint64(1)
	const projection = "weak_components"

	memberRows := []ClusterMemberRow{
		{NodeID: 1, SymbolID: "pkg:A"},
		{NodeID: 2, SymbolID: "pkg:B"},
		{NodeID: 3, SymbolID: "pkg:C"},
		{NodeID: 4, SymbolID: "pkg:D"},
		{NodeID: 5, SymbolID: "pkg:E"},
	}
	// Return all 5 rows regardless of limit for MemberCount, but the accessor
	// enforces the limit for returned members. We use a custom accessor that
	// returns total count separately.
	cma := &fixClusterMemberAccessor{rows: memberRows}
	store := &explainClusterStoreRec{
		t:              t,
		graphVersion:   graphVersion,
		latestSnapshot: 1,
	}
	s := newSkillForExplainClusterTest(t, "read", store, cma, nil)
	clusterToken := encodeClusterID(projection, graphVersion, clusterIntID)

	res := s.handleExplainCluster(context.Background(), ExplainClusterArgs{
		ClusterID:  clusterToken,
		MaxMembers: 3,
	})

	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalExplainClusterResult(t, textOf(res))

	if out.MembersReturned != 3 {
		t.Errorf("members_returned = %d; want 3 (MaxMembers cap)", out.MembersReturned)
	}
	if out.MemberCount != 5 {
		t.Errorf("member_count = %d; want 5 (total before cap)", out.MemberCount)
	}
	if len(out.Members) != 3 {
		t.Errorf("members len = %d; want 3", len(out.Members))
	}
}

// TestExplainCluster_CohesionConductance: fixture with 4 members, 3 intra
// edges (n*(n-1)=12 for directed) → Cohesion==3/12==0.25;
// 2 leaving edges → Separation==2/(2*3+2)==0.25; assert within 1e-9 tolerance.
func TestExplainCluster_CohesionConductance(t *testing.T) {
	const graphVersion = uint64(5)
	const clusterIntID = uint64(3)
	const projection = "weak_components"

	// 4 member nodes: 101, 102, 103, 104
	memberRows := []ClusterMemberRow{
		{NodeID: 101, SymbolID: "pkg:A:Foo"},
		{NodeID: 102, SymbolID: "pkg:A:Bar"},
		{NodeID: 103, SymbolID: "pkg:B:Baz"},
		{NodeID: 104, SymbolID: "pkg:B:Qux"},
	}
	cma := &fixClusterMemberAccessor{rows: memberRows}

	// Adjacency:
	//   intra edges: 101→102, 102→103, 103→101 (3 intra)
	//   leaving edges: 101→999, 102→998 (2 leaving, where 999 and 998 are outside cluster)
	store := &explainClusterStoreRec{
		t:              t,
		graphVersion:   graphVersion,
		latestSnapshot: 3,
		adjacencyOut: map[graph.NodeID]map[graph.NodeID]float64{
			101: {102: 1.0, 999: 1.0}, // 1 intra + 1 leaving
			102: {103: 1.0, 998: 1.0}, // 1 intra + 1 leaving
			103: {101: 1.0},           // 1 intra
			// node 104 has no outgoing edges in this fixture
		},
	}
	s := newSkillForExplainClusterTest(t, "read", store, cma, nil)
	clusterToken := encodeClusterID(projection, graphVersion, clusterIntID)

	res := s.handleExplainCluster(context.Background(), ExplainClusterArgs{
		ClusterID: clusterToken,
	})

	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalExplainClusterResult(t, textOf(res))

	// n=4, intra=3, n*(n-1)=12 → Cohesion = 3/12 = 0.25
	wantCohesion := 3.0 / 12.0
	if math.Abs(out.Cohesion-wantCohesion) > 1e-9 {
		t.Errorf("cohesion = %f; want %f (±1e-9)", out.Cohesion, wantCohesion)
	}

	// leaving=2, intra=3 → Separation = 2/(2*3+2) = 2/8 = 0.25
	wantSeparation := 2.0 / (2.0*3.0 + 2.0)
	if math.Abs(out.Separation-wantSeparation) > 1e-9 {
		t.Errorf("separation = %f; want %f (±1e-9)", out.Separation, wantSeparation)
	}
}

// TestExplainCluster_FreshnessV2Present: Freshness is non-zero
// (GraphVersion > 0, Status non-empty).
func TestExplainCluster_FreshnessV2Present(t *testing.T) {
	const graphVersion = uint64(77)
	const clusterIntID = uint64(2)
	const projection = "weak_components"

	memberRows := []ClusterMemberRow{
		{NodeID: 1, SymbolID: "pkg:X"},
	}
	cma := &fixClusterMemberAccessor{rows: memberRows}
	store := &explainClusterStoreRec{
		t:              t,
		graphVersion:   graphVersion,
		latestSnapshot: 9,
	}
	s := newSkillForExplainClusterTest(t, "read", store, cma, nil)
	clusterToken := encodeClusterID(projection, graphVersion, clusterIntID)

	res := s.handleExplainCluster(context.Background(), ExplainClusterArgs{
		ClusterID: clusterToken,
	})

	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalExplainClusterResult(t, textOf(res))

	if out.Freshness.GraphVersion == 0 {
		t.Errorf("freshness.graph_version == 0; want non-zero")
	}
	if out.Freshness.Status == "" {
		t.Errorf("freshness.status empty; want non-empty")
	}
}

// TestExplainCluster_ReadOnlyCanary: store recorder panics/calls t.Fatalf if
// Begin/Commit/Abort/Write is called during handleExplainCluster.
func TestExplainCluster_ReadOnlyCanary(t *testing.T) {
	const graphVersion = uint64(3)
	const clusterIntID = uint64(9)
	const projection = "weak_components"

	memberRows := []ClusterMemberRow{
		{NodeID: 1, SymbolID: "pkg:Y"},
	}
	cma := &fixClusterMemberAccessor{rows: memberRows}
	store := &explainClusterStoreRec{
		t:              t,
		graphVersion:   graphVersion,
		latestSnapshot: 2,
	}
	s := newSkillForExplainClusterTest(t, "read", store, cma, nil)
	clusterToken := encodeClusterID(projection, graphVersion, clusterIntID)

	res := s.handleExplainCluster(context.Background(), ExplainClusterArgs{
		ClusterID: clusterToken,
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}

	// Canary checks: these methods are NOT on the StoreAccessor interface;
	// the handler never calls them. If it does, t.Fatalf fires above.
	if store.beginSnapshotCalls.Load() != 0 {
		t.Errorf("BeginSnapshot called %d time(s) — D-09 violation", store.beginSnapshotCalls.Load())
	}
	if store.commitSnapshotCalls.Load() != 0 {
		t.Errorf("CommitSnapshot called %d time(s) — D-09 violation", store.commitSnapshotCalls.Load())
	}
	if store.abortSnapshotCalls.Load() != 0 {
		t.Errorf("AbortSnapshot called %d time(s) — D-09 violation", store.abortSnapshotCalls.Load())
	}
	if store.writeSnapshotFactsCalls.Load() != 0 {
		t.Errorf("WriteSnapshotFacts called %d time(s) — D-09 violation", store.writeSnapshotFactsCalls.Load())
	}
}

// TestExplainCluster_EntryPoints: fixture with 4 members where 2 have exported
// symbol IDs (capital letter after last ':'); assert entry_points contains
// exactly those 2, sorted by PageRank desc.
func TestExplainCluster_EntryPoints(t *testing.T) {
	const graphVersion = uint64(20)
	const clusterIntID = uint64(5)
	const projection = "weak_components"

	// node 10: exported (MethodX starts with capital M after last ':')
	// node 20: unexported (methodInternal starts with 'm')
	// node 30: exported (Init starts with capital I after last ':')
	// node 40: unexported (helper starts with 'h')
	memberRows := []ClusterMemberRow{
		{NodeID: 10, SymbolID: "pkg:TypeA:MethodX"},        // exported
		{NodeID: 20, SymbolID: "pkg:TypeB:methodInternal"}, // unexported
		{NodeID: 30, SymbolID: "pkg:TypeC:Init"},           // exported
		{NodeID: 40, SymbolID: "pkg:TypeD:helper"},         // unexported
	}
	cma := &fixClusterMemberAccessor{rows: memberRows}

	// PageRank: exported node 10 > exported node 30
	cpra := &fixClusterPageRankAccessorForExplain{
		scores: map[uint64]float64{
			10: 0.8,
			20: 0.6,
			30: 0.3,
			40: 0.1,
		},
	}
	store := &explainClusterStoreRec{
		t:              t,
		graphVersion:   graphVersion,
		latestSnapshot: 4,
	}
	s := newSkillForExplainClusterTest(t, "read", store, cma, cpra)
	clusterToken := encodeClusterID(projection, graphVersion, clusterIntID)

	res := s.handleExplainCluster(context.Background(), ExplainClusterArgs{
		ClusterID: clusterToken,
	})

	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalExplainClusterResult(t, textOf(res))

	// Exactly 2 exported entry points.
	if len(out.EntryPoints) != 2 {
		t.Fatalf("entry_points len = %d; want 2 (only exported symbols)", len(out.EntryPoints))
	}
	// All must be IsEntryPoint=true.
	for i, ep := range out.EntryPoints {
		if !ep.IsEntryPoint {
			t.Errorf("entry_points[%d].is_entry_point = false; want true", i)
		}
	}
	// First entry point must have higher PageRank (node 10 > node 30).
	if out.EntryPoints[0].PageRank < out.EntryPoints[1].PageRank {
		t.Errorf("entry_points not sorted by PageRank desc: [0]=%f < [1]=%f",
			out.EntryPoints[0].PageRank, out.EntryPoints[1].PageRank)
	}
	// SymbolIDs of entry points must be the two exported symbols.
	epSyms := map[string]bool{
		out.EntryPoints[0].SymbolID: true,
		out.EntryPoints[1].SymbolID: true,
	}
	if !epSyms["pkg:TypeA:MethodX"] {
		t.Errorf("entry_points missing exported symbol %q", "pkg:TypeA:MethodX")
	}
	if !epSyms["pkg:TypeC:Init"] {
		t.Errorf("entry_points missing exported symbol %q", "pkg:TypeC:Init")
	}
}

// TestExplainCluster_InvalidToken: args.ClusterID = "notvalid" → errorResult
// returned (IsError==true).
func TestExplainCluster_InvalidToken(t *testing.T) {
	store := &explainClusterStoreRec{t: t, graphVersion: 1, latestSnapshot: 1}
	s := newSkillForExplainClusterTest(t, "read", store, nil, nil)

	res := s.handleExplainCluster(context.Background(), ExplainClusterArgs{
		ClusterID: "notvalid",
	})

	if !res.IsError {
		t.Fatalf("expected IsError=true for invalid token, got success: %s", textOf(res))
	}
}
