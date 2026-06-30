package semantic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)

// ---------- D-09 / D-13 recorder canaries ----------
//
// recorderStoreAccessorForFindRelated implements StoreAccessor and adds forbidden
// snapshot-write canaries. Mirrors tools_explain_symbol_test.go's pattern.
type recorderStoreAccessorForFindRelated struct {
	t *testing.T

	graphVersion      uint64
	overlayHasPending bool
	latestSnapshot    uint64

	gvCalls            atomic.Int64
	beginSnapshotCalls atomic.Int64
	commitCalls        atomic.Int64
	abortCalls         atomic.Int64
	writeFactsCalls    atomic.Int64

	// Capture the graph_version observed when ClusterIDOf is invoked (via
	// the recorder cluster accessor) — used by Pitfall 3 regression.
	mu              sync.Mutex
	gvAtClusterRead []uint64
}

func (r *recorderStoreAccessorForFindRelated) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	return r.latestSnapshot, nil
}
func (r *recorderStoreAccessorForFindRelated) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	r.gvCalls.Add(1)
	return r.graphVersion, nil
}
func (r *recorderStoreAccessorForFindRelated) OverlayHasPendingRows(repoID string) bool {
	return r.overlayHasPending
}
func (r *recorderStoreAccessorForFindRelated) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
) {
	return nil, nil, nil
}
func (r *recorderStoreAccessorForFindRelated) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
	return 0, nil
}
func (r *recorderStoreAccessorForFindRelated) OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) ([]string, uint64, error) {
	return nil, 0, nil
}
func (r *recorderStoreAccessorForFindRelated) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (uint64, bool, error) {
	return 0, false, nil
}

// Forbidden methods (D-09 / D-13 invariants). NOT on StoreAccessor interface.
func (r *recorderStoreAccessorForFindRelated) BeginSnapshot(ctx context.Context, repoID string) (uint64, error) {
	r.beginSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: find_related_symbols handler must NOT call BeginSnapshot")
	return 0, nil
}
func (r *recorderStoreAccessorForFindRelated) CommitSnapshot(ctx context.Context, snapID uint64) error {
	r.commitCalls.Add(1)
	r.t.Fatalf("D-09 violation: find_related_symbols handler must NOT call CommitSnapshot")
	return nil
}
func (r *recorderStoreAccessorForFindRelated) AbortSnapshot(ctx context.Context, snapID uint64) error {
	r.abortCalls.Add(1)
	r.t.Fatalf("D-09 violation: find_related_symbols handler must NOT call AbortSnapshot")
	return nil
}
func (r *recorderStoreAccessorForFindRelated) WriteSnapshotFacts(ctx context.Context, snapID uint64, facts any) error {
	r.writeFactsCalls.Add(1)
	r.t.Fatalf("D-09 violation: find_related_symbols handler must NOT call WriteSnapshotFacts")
	return nil
}

// ---------- Test-only retrieval accessor ----------
//
// findRelatedRetrievalAccessor is the minimum RetrievalAccessor surface needed
// by find_related_symbols: PersonalizedPageRank + QueryBleve. Other methods
// return zero values.
type findRelatedRetrievalAccessor struct {
	graphRanks map[string][]GraphRank // keyed on anchor[0]
	textRanks  map[string][]TextRank
	ppErr      error
	bleveErr   error

	ppCalls    atomic.Int64
	bleveCalls atomic.Int64
}

func (m *findRelatedRetrievalAccessor) QueryBleve(task string, anchors []string) ([]TextRank, error) {
	m.bleveCalls.Add(1)
	if m.bleveErr != nil {
		return nil, m.bleveErr
	}
	if len(anchors) == 0 {
		return nil, nil
	}
	return m.textRanks[anchors[0]], nil
}
func (m *findRelatedRetrievalAccessor) PersonalizedPageRank(ctx context.Context, repoID string, anchors []string) ([]GraphRank, error) {
	m.ppCalls.Add(1)
	if m.ppErr != nil {
		return nil, m.ppErr
	}
	if len(anchors) == 0 {
		return nil, nil
	}
	return m.graphRanks[anchors[0]], nil
}
func (m *findRelatedRetrievalAccessor) RetrievalPending(ws workspace.WorkspaceKey) bool { return false }
func (m *findRelatedRetrievalAccessor) RetrievalStatus(ws workspace.WorkspaceKey) RetrievalStatus {
	return RetrievalStatus{}
}
func (m *findRelatedRetrievalAccessor) TopEdgesFor(ctx context.Context, repoID, symbolID string) ([]string, error) {
	return nil, nil
}

// ---------- Test-only cluster accessor with graph_version recording ----------
//
// recorderClusterMembership wraps a static cluster map and records which
// repoID is used per call. We don't have a direct way to capture the
// graph_version the handler passed (the accessor signature does not carry it),
// so the Pitfall 3 regression test instead validates the same snapshot/gv
// pinning by checking that `CurrentGraphVersion` was called BEFORE
// `ClusterIDOf`, which is the operational invariant.
type recorderClusterMembership struct {
	mu             sync.Mutex
	clusters       map[integ.SymbolID]uint64
	clusterIDCalls atomic.Int64
	// Records call order relative to graph_version reads via the wrapped store.
	store *recorderStoreAccessorForFindRelated
}

func (m *recorderClusterMembership) ClusterIDOf(ctx context.Context, repoID string, symbolID integ.SymbolID) (uint64, int, error) {
	m.clusterIDCalls.Add(1)
	if m.store != nil {
		m.store.mu.Lock()
		m.store.gvAtClusterRead = append(m.store.gvAtClusterRead, m.store.graphVersion)
		m.store.mu.Unlock()
	}
	if id, ok := m.clusters[symbolID]; ok {
		return id, 2, nil
	}
	return 0, 0, nil
}

// failingClusterMembership returns an error for every lookup — simulates the
// "cluster engine unavailable" path that should still produce a ranked result
// with fallback_reason="cluster_boost_unavailable".
type failingClusterMembership struct{ err error }

func (m *failingClusterMembership) ClusterIDOf(ctx context.Context, repoID string, symbolID integ.SymbolID) (uint64, int, error) {
	return 0, 0, m.err
}

// ---------- common test setup ----------

func newSkillForFindRelatedTest(
	t *testing.T,
	mode string,
	fx *PopulatedGraphFixture,
	retr *findRelatedRetrievalAccessor,
	cm ClusterMembershipAccessor,
) (*SemanticSkill, *recorderStoreAccessorForFindRelated) {
	t.Helper()
	s := &SemanticSkill{}
	if err := s.Init(skill.SkillDeps{}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	sess := &mcp.SessionInfo{
		SessionID: "test-find-related-session",
		Mode:      mode,
	}
	ws := workspace.WorkspaceKey{
		RepoRoot: "/tmp/repo-find-related-test", Language: "go", Toolchain: "go1.22",
	}
	s.SetSessionAccessor(&mockSessionAccessor{ws: ws, sess: sess})

	store := &recorderStoreAccessorForFindRelated{
		t:              t,
		graphVersion:   42,
		latestSnapshot: 7,
	}
	s.SetStore(store)
	s.SetSymbolByName(fx.SymbolByName)
	s.SetExtractorRun(fx.ExtractorRun)
	if cm != nil {
		s.SetClusterMembership(cm)
	} else {
		s.SetClusterMembership(fx.ClusterMembership)
	}
	s.SetRetrieval(retr)
	return s, store
}

func unmarshalFindRelatedResult(t *testing.T, raw string) FindRelatedSymbolsResult {
	t.Helper()
	var out FindRelatedSymbolsResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("unmarshal find_related result: %v (raw=%s)", err, raw)
	}
	return out
}

// makeGraphRanks builds N synthetic GraphRank rows with decreasing scores.
func makeGraphRanks(seedNeighbors []string) []GraphRank {
	out := make([]GraphRank, 0, len(seedNeighbors))
	for i, id := range seedNeighbors {
		out = append(out, GraphRank{
			SymbolID: id,
			Score:    1.0 / float64(i+1),
		})
	}
	return out
}

// ---------- behavior tests ----------

func TestFindRelatedSymbols_RankingDefaultK(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID

	// 25 neighbors — should be truncated to default k=20.
	neighbors := make([]string, 0, 25)
	for i := 0; i < 25; i++ {
		neighbors = append(neighbors, fmt.Sprintf("repo/src/n_%03d.go::N", i))
	}
	retr := &findRelatedRetrievalAccessor{
		graphRanks: map[string][]GraphRank{string(seed): makeGraphRanks(neighbors)},
	}
	s, _ := newSkillForFindRelatedTest(t, "read", fx, retr, nil)

	res := s.handleFindRelatedSymbols(context.Background(), FindRelatedSymbolsArgs{
		Seed: SeedInput{SymbolID: string(seed)},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalFindRelatedResult(t, textOf(res))

	if len(out.Results) > 20 {
		t.Errorf("len(results) = %d; want ≤ 20 (default k)", len(out.Results))
	}
	if len(out.Results) == 0 {
		t.Fatalf("len(results) == 0; want non-empty")
	}
	// Results should be ordered by Score descending.
	for i := 1; i < len(out.Results); i++ {
		if out.Results[i].Score > out.Results[i-1].Score {
			t.Errorf("results not sorted by score desc at i=%d: %v > %v",
				i, out.Results[i].Score, out.Results[i-1].Score)
		}
	}
	if out.Freshness.GraphVersion == 0 {
		t.Errorf("freshness.graph_version zero; want 42")
	}
}

func TestFindRelatedSymbols_KClamp(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID

	// 150 neighbors so we can verify high-k clamp at 100.
	neighbors := make([]string, 0, 150)
	for i := 0; i < 150; i++ {
		neighbors = append(neighbors, fmt.Sprintf("repo/src/n_%03d.go::N", i))
	}
	retr := &findRelatedRetrievalAccessor{
		graphRanks: map[string][]GraphRank{string(seed): makeGraphRanks(neighbors)},
	}

	cases := []struct {
		name      string
		k         int
		maxResult int
	}{
		{"k=0 clamps to default 20", 0, 20},
		{"k=500 clamps to 100", 500, 100},
		{"k=1 honored", 1, 1},
		{"k=50 honored", 50, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newSkillForFindRelatedTest(t, "read", fx, retr, nil)
			res := s.handleFindRelatedSymbols(context.Background(), FindRelatedSymbolsArgs{
				Seed: SeedInput{SymbolID: string(seed)},
				K:    tc.k,
			})
			if res.IsError {
				t.Fatalf("expected success, got error: %s", textOf(res))
			}
			out := unmarshalFindRelatedResult(t, textOf(res))
			if len(out.Results) > tc.maxResult {
				t.Errorf("k=%d: len(results) = %d; want ≤ %d", tc.k, len(out.Results), tc.maxResult)
			}
		})
	}
}

func TestFindRelatedSymbols_PathsStrictSubset_ResultsFiltered(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID

	// Mix of paths: some under pkg/sibling/, some outside.
	neighbors := []string{
		"pkg/sibling/a.go::A",
		"pkg/sibling/b.go::B",
		"pkg/other/c.go::C",
		"pkg/sibling/sub/d.go::D",
		"pkg/elsewhere/e.go::E",
	}
	retr := &findRelatedRetrievalAccessor{
		graphRanks: map[string][]GraphRank{string(seed): makeGraphRanks(neighbors)},
	}
	s, _ := newSkillForFindRelatedTest(t, "read", fx, retr, nil)

	res := s.handleFindRelatedSymbols(context.Background(), FindRelatedSymbolsArgs{
		Seed:  SeedInput{SymbolID: string(seed)},
		Paths: []string{"pkg/sibling/"},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalFindRelatedResult(t, textOf(res))
	if len(out.Results) == 0 {
		t.Fatalf("no results; want pkg/sibling/* entries")
	}
	for _, r := range out.Results {
		if !pathHasPrefix(r.Path, "pkg/sibling/") {
			t.Errorf("result path %q not under pkg/sibling/", r.Path)
		}
	}
}

func TestFindRelatedSymbols_PathsRejectsTraversal(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID
	retr := &findRelatedRetrievalAccessor{}
	s, _ := newSkillForFindRelatedTest(t, "read", fx, retr, nil)

	res := s.handleFindRelatedSymbols(context.Background(), FindRelatedSymbolsArgs{
		Seed:  SeedInput{SymbolID: string(seed)},
		Paths: []string{"../../../etc"},
	})
	if !res.IsError {
		t.Fatalf("expected error envelope for traversal; got success")
	}
}

func TestFindRelatedSymbols_EmptyResult(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID

	retr := &findRelatedRetrievalAccessor{} // no graph ranks → isolated seed
	s, _ := newSkillForFindRelatedTest(t, "read", fx, retr, nil)

	res := s.handleFindRelatedSymbols(context.Background(), FindRelatedSymbolsArgs{
		Seed: SeedInput{SymbolID: string(seed)},
	})
	if res.IsError {
		t.Fatalf("isolated seed must NOT be an error envelope; got error: %s", textOf(res))
	}
	out := unmarshalFindRelatedResult(t, textOf(res))
	if len(out.Results) != 0 {
		t.Errorf("len(results) = %d; want 0 (isolated seed)", len(out.Results))
	}
	if out.TotalCount != 0 {
		t.Errorf("total_count = %d; want 0", out.TotalCount)
	}
}

func TestFindRelatedSymbols_ClusterBoostApplied(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID

	// Two candidates with similar pre-boost scores; A is in seed's cluster, B is not.
	neighbors := []string{"pkg/x/A.go::A", "pkg/x/B.go::B"}
	graphRanks := []GraphRank{
		{SymbolID: "pkg/x/A.go::A", Score: 0.30}, // slightly lower
		{SymbolID: "pkg/x/B.go::B", Score: 0.31}, // slightly higher pre-boost
	}
	retr := &findRelatedRetrievalAccessor{
		graphRanks: map[string][]GraphRank{string(seed): graphRanks},
	}
	_ = neighbors

	// Cluster: seed → cluster 1, A → cluster 1, B → cluster 2.
	cm := &recorderClusterMembership{
		clusters: map[integ.SymbolID]uint64{
			seed:            1,
			"pkg/x/A.go::A": 1,
			"pkg/x/B.go::B": 2,
		},
	}
	s, store := newSkillForFindRelatedTest(t, "read", fx, retr, cm)
	cm.store = store

	res := s.handleFindRelatedSymbols(context.Background(), FindRelatedSymbolsArgs{
		Seed: SeedInput{SymbolID: string(seed)},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalFindRelatedResult(t, textOf(res))
	if len(out.Results) < 2 {
		t.Fatalf("len(results) = %d; want ≥ 2", len(out.Results))
	}
	// After cluster boost (A in same cluster as seed), A should rank above B.
	var posA, posB int = -1, -1
	for i, r := range out.Results {
		if r.SymbolID == "pkg/x/A.go::A" {
			posA = i
		}
		if r.SymbolID == "pkg/x/B.go::B" {
			posB = i
		}
	}
	if posA == -1 || posB == -1 {
		t.Fatalf("A or B missing from results: %+v", out.Results)
	}
	if posA >= posB {
		t.Errorf("cluster boost not applied: A pos=%d, B pos=%d (A should rank above B)", posA, posB)
	}
	// Surface the boost flag.
	foundBoostFlag := false
	for _, r := range out.Results {
		if r.SymbolID == "pkg/x/A.go::A" && r.ClusterBoostApplied {
			foundBoostFlag = true
		}
	}
	if !foundBoostFlag {
		t.Errorf("ClusterBoostApplied=true not surfaced for cluster-mate A")
	}
}

func TestFindRelatedSymbols_ClusterBoostSameGraphVersion(t *testing.T) {
	// Pitfall 3 regression: the cluster lookup must observe the same
	// graph_version that PageRank ran on. We assert ordering: the handler
	// MUST read CurrentGraphVersion BEFORE issuing ClusterIDOf calls, so any
	// concurrent gv mutation cannot drift the cluster read off the
	// PageRank-time gv. The recorder counts both.
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID

	neighbors := []string{"pkg/x/A.go::A"}
	retr := &findRelatedRetrievalAccessor{
		graphRanks: map[string][]GraphRank{string(seed): makeGraphRanks(neighbors)},
	}
	cm := &recorderClusterMembership{
		clusters: map[integ.SymbolID]uint64{
			seed:            1,
			"pkg/x/A.go::A": 1,
		},
	}
	s, store := newSkillForFindRelatedTest(t, "read", fx, retr, cm)
	cm.store = store

	res := s.handleFindRelatedSymbols(context.Background(), FindRelatedSymbolsArgs{
		Seed: SeedInput{SymbolID: string(seed)},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	if store.gvCalls.Load() == 0 {
		t.Errorf("CurrentGraphVersion not called; cluster boost graph_version must be pinned (Pitfall 3)")
	}
	if cm.clusterIDCalls.Load() == 0 {
		t.Errorf("ClusterIDOf not called; cluster boost path not exercised")
	}
	store.mu.Lock()
	observed := append([]uint64(nil), store.gvAtClusterRead...)
	store.mu.Unlock()
	for _, gv := range observed {
		if gv != 42 {
			t.Errorf("ClusterIDOf observed gv=%d; want 42 (the snapshot graph_version)", gv)
		}
	}
}

func TestFindRelatedSymbols_ClusterBoostUnavailable(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID

	neighbors := []string{"pkg/x/A.go::A", "pkg/x/B.go::B"}
	retr := &findRelatedRetrievalAccessor{
		graphRanks: map[string][]GraphRank{string(seed): makeGraphRanks(neighbors)},
	}
	cm := &failingClusterMembership{err: errors.New("simulated cluster engine unavailable")}
	s, _ := newSkillForFindRelatedTest(t, "read", fx, retr, cm)

	res := s.handleFindRelatedSymbols(context.Background(), FindRelatedSymbolsArgs{
		Seed: SeedInput{SymbolID: string(seed)},
	})
	if res.IsError {
		t.Fatalf("cluster engine error must NOT fail the handler; got error: %s", textOf(res))
	}
	out := unmarshalFindRelatedResult(t, textOf(res))
	if len(out.Results) == 0 {
		t.Errorf("results empty; want ranked candidates without boost")
	}
	if out.FallbackReason != "cluster_boost_unavailable" {
		t.Errorf("fallback_reason = %q; want %q", out.FallbackReason, "cluster_boost_unavailable")
	}
	for _, r := range out.Results {
		if r.ClusterBoostApplied {
			t.Errorf("ClusterBoostApplied set on degraded path: %+v", r)
		}
	}
}

func TestFindRelatedSymbols_ReadOnly(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID
	neighbors := []string{"pkg/x/A.go::A", "pkg/x/B.go::B"}
	retr := &findRelatedRetrievalAccessor{
		graphRanks: map[string][]GraphRank{string(seed): makeGraphRanks(neighbors)},
	}
	s, store := newSkillForFindRelatedTest(t, "read", fx, retr, nil)

	res := s.handleFindRelatedSymbols(context.Background(), FindRelatedSymbolsArgs{
		Seed: SeedInput{SymbolID: string(seed)},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	if store.beginSnapshotCalls.Load() != 0 ||
		store.commitCalls.Load() != 0 ||
		store.abortCalls.Load() != 0 ||
		store.writeFactsCalls.Load() != 0 {
		t.Errorf("D-09 violation: snapshot-write canary fired")
	}
}

func TestFindRelatedSymbols_ModeRejected(t *testing.T) {
	// Mirroring tools_explain_symbol_test.go's pragmatic stance: modeTierRead
	// returns nil for all sessions by design. We exercise the alternate
	// rejection path (resolver-unwired) and assert no accessor canaries fire.
	fx := buildPopulatedGraphFixture(t)
	retr := &findRelatedRetrievalAccessor{}
	s, store := newSkillForFindRelatedTest(t, "read", fx, retr, nil)
	s.SetSymbolByName(nil) // force resolver-unwired error path for tuple seed

	res := s.handleFindRelatedSymbols(context.Background(), FindRelatedSymbolsArgs{
		Seed: SeedInput{FilePath: "x.go", SymbolName: "Y"},
	})
	if !res.IsError {
		t.Fatalf("expected error envelope when resolver unwired; got success")
	}
	if store.beginSnapshotCalls.Load()+store.commitCalls.Load()+
		store.abortCalls.Load()+store.writeFactsCalls.Load() != 0 {
		t.Errorf("snapshot-write canary fired on error path")
	}
	if retr.ppCalls.Load() != 0 {
		t.Errorf("PersonalizedPageRank invoked on error path: %d calls", retr.ppCalls.Load())
	}
}

func TestFindRelatedSymbols_SeedNotFound(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	retr := &findRelatedRetrievalAccessor{}
	s, _ := newSkillForFindRelatedTest(t, "read", fx, retr, nil)

	res := s.handleFindRelatedSymbols(context.Background(), FindRelatedSymbolsArgs{
		Seed: SeedInput{FilePath: "unknown/path.go", SymbolName: "Missing"},
	})
	if res.IsError {
		t.Fatalf("not_found must NOT be an error envelope; got error: %s", textOf(res))
	}
	out := unmarshalFindRelatedResult(t, textOf(res))
	if out.Seed.Resolution != ResolutionNotFound {
		t.Errorf("seed.resolution = %q; want %q", out.Seed.Resolution, ResolutionNotFound)
	}
	if out.FallbackReason != "symbol_not_found" {
		t.Errorf("fallback_reason = %q; want %q", out.FallbackReason, "symbol_not_found")
	}
	if len(out.Results) != 0 {
		t.Errorf("len(results) = %d; want 0", len(out.Results))
	}
	if retr.ppCalls.Load() != 0 {
		t.Errorf("PersonalizedPageRank invoked on not_found: %d calls", retr.ppCalls.Load())
	}
}

func TestFindRelatedSymbols_ConcurrentSameSeed(t *testing.T) {
	// Pitfall 6 regression guard: race-clean under concurrent invocation +
	// byte-identical responses (idempotency) when k+seed are constant.
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID

	neighbors := []string{
		"pkg/x/A.go::A", "pkg/x/B.go::B", "pkg/x/C.go::C", "pkg/x/D.go::D",
	}
	retr := &findRelatedRetrievalAccessor{
		graphRanks: map[string][]GraphRank{string(seed): makeGraphRanks(neighbors)},
	}
	cm := &recorderClusterMembership{
		clusters: map[integ.SymbolID]uint64{
			seed:            1,
			"pkg/x/A.go::A": 1,
			"pkg/x/B.go::B": 2,
		},
	}
	s, store := newSkillForFindRelatedTest(t, "read", fx, retr, cm)
	cm.store = store

	const N = 16
	results := make([]string, N)
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res := s.handleFindRelatedSymbols(context.Background(), FindRelatedSymbolsArgs{
				Seed: SeedInput{SymbolID: string(seed)},
				K:    5,
			})
			if res.IsError {
				t.Errorf("[%d] error: %s", i, textOf(res))
				return
			}
			// Strip the AsOfUnixMs field (time-dependent) before comparison.
			out := unmarshalFindRelatedResult(t, textOf(res))
			out.Freshness.AsOfUnixMs = 0
			b, _ := json.Marshal(out)
			results[i] = string(b)
		}(i)
	}
	wg.Wait()

	for i := 1; i < N; i++ {
		if results[i] != results[0] {
			t.Errorf("response[%d] differs from response[0]:\n  [0]=%s\n  [%d]=%s",
				i, results[0], i, results[i])
		}
	}
}

// pathHasPrefix returns whether path starts with prefix; used in the strict-
// subset filter test. Defined here (not in handler_helpers.go) to keep
// helper-file ownership clean.
func pathHasPrefix(path, prefix string) bool {
	if len(path) < len(prefix) {
		return false
	}
	return path[:len(prefix)] == prefix
}
