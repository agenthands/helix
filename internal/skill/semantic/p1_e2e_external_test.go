// p1_e2e_external_test.go — Phase 73-04 Tasks 3 & 4: real-store E2E suite
// for the 6 Phase 71/72 P1 MCP tools.
//
// Design decisions:
//
//   - D-02: one shared buildP1E2EFixture(t) builder is called by every tool
//     test (no per-tool tempdir setup, no mutation of productionAdapterFixture).
//     The builder provisions a real duckdb *Store + bleve engine + multi-language
//     populated graph in a t.TempDir so each subtest gets isolation via separate
//     fixture calls.
//
//   - SC#4: all 6 P1 tool tests exercise a real *Store (not the in-memory
//     populated_graph_fixture_test.go mock). The bleve engine is opened but
//     retrieval queries return empty (no rebuild run) — the find_related_symbols
//     handler degrades gracefully to an empty result set, still emitting a valid
//     FreshnessV2 envelope.
//
//   - Store-backed accessor adapters: p1StoreSymbolByName, p1StoreExtractorRun,
//     p1StoreClusterMap, p1StoreClusterMember, p1StoreClusterPageRank wrap the
//     *Store methods directly. TypeChain/SymbolEdges/EdgeEvidence/ClusterMembership
//     are inline fixture implementations — Schema 5 has no per-symbol type-chain
//     or directed-edge SQL readers on *Store.
//
//   - Closed-enum assertions: each test asserts freshness.status ∈
//     {current, stale, unknown}, freshness.source is a declared FreshnessSource
//     value, fallback_reason is empty OR a declared closed-enum reason, and
//     confidence ∈ [0.0, 1.0] (when present).
//
// Package-cycle note: this file is `package semantic_test` (black-box) so it
// can import internal/daemon for the production ImpactLookup constructor. The
// handler entry points are reached via the Handle*ForTest exports defined in
// export_p1_test.go (package semantic).
//
// extractText helper is declared in status_e2e_external_test.go (same package).

package semantic_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/daemon"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
	semanticpkg "github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/semantic/retrieval"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/skill/semantic"
	"github.com/agenthands/helix/internal/workspace"
)

// ---------------------------------------------------------------------------
// Fixture symbol IDs — stable_key strings used as SymbolID throughout.
// ---------------------------------------------------------------------------

const (
	p1SeedGoServeHTTP = "repo/src/svc.go::ServeHTTP"
	p1SeedGoHandle    = "repo/src/svc.go::handle"
	p1SeedGoRequest   = "repo/src/types.go::Request"
	p1SeedTSFetch     = "repo/web/api.ts::fetchUser"
	p1SeedTSUser      = "repo/web/types.ts::User"
	p1SeedJavaFooBar  = "repo/api/Foo.java::Foo.bar"
	p1SeedJavaBar     = "repo/api/Bar.java::Bar"
)

// Internal numeric NodeID values used in semantic_symbols and adjacency maps.
const (
	p1NodeServeHTTP uint64 = 1001
	p1NodeHandle    uint64 = 1002
	p1NodeRequest   uint64 = 1003
	p1NodeTSFetch   uint64 = 1004
	p1NodeTSUser    uint64 = 1005
	p1NodeJavaFoo   uint64 = 1006
	p1NodeJavaBar   uint64 = 1007
)

// ---------------------------------------------------------------------------
// p1E2EFixture is the shared fixture returned by buildP1E2EFixture.
// ---------------------------------------------------------------------------

type p1E2EFixture struct {
	skill  *semantic.SemanticSkill
	store  *semanticstore.Store
	engine *retrieval.Engine
	ws     workspace.WorkspaceKey
	repoID string
	// graphVersion is the overlay graph_version after seeding clusters.
	graphVersion uint64
	// seedClusterIDEncoded is the opaque cluster token for cluster 1 (Go cluster).
	// Format: "{projection}:{graph_version}:{cluster_int_id}" as produced by
	// encodeClusterID in tools_cluster_map.go.
	seedClusterIDEncoded string
}

// ---------------------------------------------------------------------------
// buildP1E2EFixture provisions the shared real-store E2E fixture.
//
// Fixture data mirrors the populated_graph_fixture_test.go layout (Go +
// TypeScript + Java symbols and CALLS edges) but backed by a real *Store +
// bleve in a tempdir. All 6 P1 tools return non-degenerate results with valid
// closed-enum envelope fields when called against this fixture.
// ---------------------------------------------------------------------------

func buildP1E2EFixture(t *testing.T) *p1E2EFixture {
	t.Helper()
	ctx := context.Background()

	wsDir := t.TempDir()
	t.Chdir(wsDir)

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	provider := obs.Noop(logger.Handler())
	storeCfg := semanticpkg.Config{
		Enabled: true,
		Store: semanticpkg.StoreConfig{
			Kind:        "duckdb",
			Path:        filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB",
			Threads:     2,
		},
	}
	store, err := semanticstore.Open(ctx, storeCfg, logger, provider.Metrics())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	bleveDir := filepath.Join(wsDir, ".helix", "semantic.bleve")
	engine, err := retrieval.New(bleveDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })

	ws := workspace.WorkspaceKey{RepoRoot: wsDir, Language: "go", Toolchain: "go1.22"}
	repoID := ws.Hash()

	// ---- Step 1: commit a multi-language snapshot with 7 symbols + CALLS edges. ----

	files := []semanticstore.FileFact{
		{FileID: 1, RepoID: repoID, Path: "repo/src/svc.go", Language: "go", ContentHash: "h1"},
		{FileID: 2, RepoID: repoID, Path: "repo/src/types.go", Language: "go", ContentHash: "h2"},
		{FileID: 3, RepoID: repoID, Path: "repo/web/api.ts", Language: "typescript", ContentHash: "h3"},
		{FileID: 4, RepoID: repoID, Path: "repo/web/types.ts", Language: "typescript", ContentHash: "h4"},
		{FileID: 5, RepoID: repoID, Path: "repo/api/Foo.java", Language: "java", ContentHash: "h5"},
		{FileID: 6, RepoID: repoID, Path: "repo/api/Bar.java", Language: "java", ContentHash: "h6"},
	}

	symbols := []semanticstore.SymbolFact{
		{SymbolID: p1NodeServeHTTP, NodeID: p1NodeServeHTTP, FileID: 1,
			Language: "go", Kind: "function", Name: "ServeHTTP",
			QualifiedName: "svc.ServeHTTP", StableKey: p1SeedGoServeHTTP,
			StartLine: 10, StartCol: 1, EndLine: 20, EndCol: 1,
			Visibility: "public", Confidence: 1.0},
		{SymbolID: p1NodeHandle, NodeID: p1NodeHandle, FileID: 1,
			Language: "go", Kind: "function", Name: "handle",
			QualifiedName: "svc.handle", StableKey: p1SeedGoHandle,
			StartLine: 25, StartCol: 1, EndLine: 35, EndCol: 1,
			Visibility: "private", Confidence: 1.0},
		{SymbolID: p1NodeRequest, NodeID: p1NodeRequest, FileID: 2,
			Language: "go", Kind: "type", Name: "Request",
			QualifiedName: "svc.Request", StableKey: p1SeedGoRequest,
			StartLine: 5, StartCol: 1, EndLine: 10, EndCol: 1,
			Visibility: "public", Confidence: 1.0},
		{SymbolID: p1NodeTSFetch, NodeID: p1NodeTSFetch, FileID: 3,
			Language: "typescript", Kind: "function", Name: "fetchUser",
			QualifiedName: "api.fetchUser", StableKey: p1SeedTSFetch,
			StartLine: 3, StartCol: 1, EndLine: 8, EndCol: 1,
			Visibility: "public", Confidence: 1.0},
		{SymbolID: p1NodeTSUser, NodeID: p1NodeTSUser, FileID: 4,
			Language: "typescript", Kind: "type", Name: "User",
			QualifiedName: "types.User", StableKey: p1SeedTSUser,
			StartLine: 1, StartCol: 1, EndLine: 5, EndCol: 1,
			Visibility: "public", Confidence: 1.0},
		{SymbolID: p1NodeJavaFoo, NodeID: p1NodeJavaFoo, FileID: 5,
			Language: "java", Kind: "method", Name: "Foo.bar",
			QualifiedName: "Foo.bar", StableKey: p1SeedJavaFooBar,
			StartLine: 10, StartCol: 1, EndLine: 15, EndCol: 1,
			Visibility: "public", Confidence: 1.0},
		{SymbolID: p1NodeJavaBar, NodeID: p1NodeJavaBar, FileID: 6,
			Language: "java", Kind: "class", Name: "Bar",
			QualifiedName: "Bar", StableKey: p1SeedJavaBar,
			StartLine: 1, StartCol: 1, EndLine: 20, EndCol: 1,
			Visibility: "public", Confidence: 1.0},
	}

	// Seed edges with EdgeKind="call_graph" so ExpandFrom's QueryEffectiveAdjacency
	// finds adjacency: ServeHTTP → handle, fetchUser → User, Foo.bar → Bar.
	edges := []semanticstore.EdgeFact{
		{EdgeID: 1, SrcNodeID: p1NodeServeHTTP, DstNodeID: p1NodeHandle,
			EdgeKind: "call_graph", Weight: 1.0, Confidence: 1.0,
			Source: "lsp.go.text_document_references"},
		{EdgeID: 2, SrcNodeID: p1NodeTSFetch, DstNodeID: p1NodeTSUser,
			EdgeKind: "call_graph", Weight: 0.9, Confidence: 0.9, Source: "ast"},
		{EdgeID: 3, SrcNodeID: p1NodeJavaFoo, DstNodeID: p1NodeJavaBar,
			EdgeKind: "call_graph", Weight: 0.8, Confidence: 0.8, Source: "ast"},
	}

	snap, err := store.BeginSnapshot(ctx, semanticstore.SnapshotMeta{RepoID: repoID})
	require.NoError(t, err)
	committed := false
	defer func() {
		if !committed {
			_ = store.AbortSnapshot(context.Background(), snap, "p1E2E fixture: not committed")
		}
	}()
	require.NoError(t, store.WriteSnapshotFacts(ctx, snap, semanticstore.Facts{
		Files:   files,
		Symbols: symbols,
		Edges:   edges,
	}))
	require.NoError(t, store.CommitSnapshot(ctx, snap, semanticstore.SnapshotSummary{
		FileCount:   len(files),
		SymbolCount: len(symbols),
		EdgeCount:   len(edges),
	}))
	committed = true

	// ---- Step 2: open overlay tx, bump graph_version, seed scores + clusters. ----

	tx, err := store.BeginOverlayTx(ctx, repoID)
	require.NoError(t, err)
	gv, err := tx.BumpGraphVersion(ctx)
	require.NoError(t, err)
	require.Greater(t, gv, uint64(0), "BumpGraphVersion must yield graph_version > 0")

	// Score rows so find_related_symbols PersonalizedPageRank has ranked nodes.
	scoreRows := []semanticstore.ScoreRow{
		{NodeID: p1NodeServeHTTP, Score: 0.95, GraphVersion: gv, Status: "exact"},
		{NodeID: p1NodeHandle, Score: 0.80, GraphVersion: gv, Status: "exact"},
		{NodeID: p1NodeRequest, Score: 0.60, GraphVersion: gv, Status: "exact"},
		{NodeID: p1NodeTSFetch, Score: 0.70, GraphVersion: gv, Status: "exact"},
		{NodeID: p1NodeTSUser, Score: 0.50, GraphVersion: gv, Status: "exact"},
		{NodeID: p1NodeJavaFoo, Score: 0.65, GraphVersion: gv, Status: "exact"},
		{NodeID: p1NodeJavaBar, Score: 0.45, GraphVersion: gv, Status: "exact"},
	}
	require.NoError(t, tx.UpsertGraphScores(ctx, "call_graph", scoreRows))

	// Cluster rows so get_cluster_map + explain_cluster return real data.
	require.NoError(t, tx.UpsertClusters(ctx, "weak_components", gv, []semanticstore.ClusterSummary{
		{ID: 1, MemberCount: 3},
		{ID: 2, MemberCount: 2},
		{ID: 3, MemberCount: 2},
	}))
	require.NoError(t, tx.UpsertClusterMembers(ctx, "weak_components", gv, []semanticstore.ClusterMemberRow{
		{ClusterID: 1, NodeID: p1NodeServeHTTP},
		{ClusterID: 1, NodeID: p1NodeHandle},
		{ClusterID: 1, NodeID: p1NodeRequest},
		{ClusterID: 2, NodeID: p1NodeTSFetch},
		{ClusterID: 2, NodeID: p1NodeTSUser},
		{ClusterID: 3, NodeID: p1NodeJavaFoo},
		{ClusterID: 3, NodeID: p1NodeJavaBar},
	}))
	require.NoError(t, tx.Commit())

	// ---- Step 3: wire the SemanticSkill. ----

	s := &semantic.SemanticSkill{}
	require.NoError(t, s.Init(skill.SkillDeps{}))

	// Session accessor: review+ mode so get_change_impact_graph passes mode check.
	s.SetSessionAccessor(&p1StaticSessionAcc{
		ws:   ws,
		sess: &mcp.SessionInfo{SessionID: "p1-e2e-test", Mode: "review"},
	})

	// Core store/retrieval/queue/live/compact accessors.
	s.SetStore(&p1PlainStoreAcc{store: store})
	s.SetScheduler(&p1PlaceholderSchedAcc{})
	s.SetQueue(&p1ZeroQueueAcc{})
	s.SetLive(&p1ZeroLiveAcc{})
	s.SetRetrieval(&p1PlaceholderRetrievalAcc{})
	s.SetCompactor(&p1NoopCompactAcc{})

	// Phase 71 seams — store-backed where *Store provides the method.
	s.SetSymbolByName(&p1StoreSymbolByName{store: store})
	s.SetExtractorRun(&p1StoreExtractorRun{store: store})
	s.SetClusterMembership(&p1InlineClusterMembership{})
	s.SetTypeChain(&p1InlineTypeChain{})
	s.SetSymbolEdges(&p1InlineSymbolEdges{})
	s.SetEdgeEvidence(&p1InlineEdgeEvidence{})

	// Phase 72 seams — real store-backed cluster accessors.
	s.SetClusterMap(&p1StoreClusterMap{store: store})
	s.SetClusterMember(&p1StoreClusterMember{store: store})
	s.SetClusterPageRank(&p1StoreClusterPageRank{store: store})

	// ImpactLookup: production integSemanticLookup backed by the real *Store.
	impactLookup := daemon.NewIntegSemanticLookupForTest(store, ws)
	s.SetImpactLookup(&p1IntegLookupAcc{lookup: impactLookup})

	// Pre-encode a cluster token for the explain_cluster test.
	// Format mirrors encodeClusterID in tools_cluster_map.go.
	clusterToken := fmt.Sprintf("weak_components:%d:1", gv)

	return &p1E2EFixture{
		skill:                s,
		store:                store,
		engine:               engine,
		ws:                   ws,
		repoID:               repoID,
		graphVersion:         gv,
		seedClusterIDEncoded: clusterToken,
	}
}

// ---------------------------------------------------------------------------
// Store-backed accessor adapters.
// ---------------------------------------------------------------------------

// p1StoreSymbolByName wraps *Store.QuerySymbolByName, converting []string → []integ.SymbolID.
type p1StoreSymbolByName struct{ store *semanticstore.Store }

func (a *p1StoreSymbolByName) QuerySymbolByName(ctx context.Context, repoID, path, name string) ([]integ.SymbolID, error) {
	raw, err := a.store.QuerySymbolByName(ctx, repoID, path, name)
	if err != nil {
		return nil, err
	}
	out := make([]integ.SymbolID, len(raw))
	for i, s := range raw {
		out[i] = integ.SymbolID(s)
	}
	return out, nil
}

// p1StoreExtractorRun wraps *Store.LatestExtractorRunID.
type p1StoreExtractorRun struct{ store *semanticstore.Store }

func (a *p1StoreExtractorRun) LatestExtractorRunID(ctx context.Context, repoID string) (string, error) {
	return a.store.LatestExtractorRunID(ctx, repoID)
}

// p1StoreClusterMap wraps *Store.QueryClusterSummaries, converting the
// store-internal ClusterSummaryResult → skill-layer ClusterSummaryRow.
type p1StoreClusterMap struct{ store *semanticstore.Store }

func (a *p1StoreClusterMap) QueryClusterSummaries(ctx context.Context, repoID, projection string, graphVersion uint64, topN int) ([]semantic.ClusterSummaryRow, error) {
	raw, err := a.store.QueryClusterSummaries(ctx, repoID, projection, graphVersion, topN)
	if err != nil {
		return nil, err
	}
	out := make([]semantic.ClusterSummaryRow, len(raw))
	for i, r := range raw {
		out[i] = semantic.ClusterSummaryRow{ClusterIntID: r.ClusterIntID, MemberCount: r.MemberCount}
	}
	return out, nil
}

// p1StoreClusterMember wraps *Store.QueryClusterMembers, converting
// ClusterMemberResult → ClusterMemberRow.
type p1StoreClusterMember struct{ store *semanticstore.Store }

func (a *p1StoreClusterMember) QueryClusterMembers(ctx context.Context, repoID, projection string, graphVersion, clusterIntID uint64, limit int) ([]semantic.ClusterMemberRow, error) {
	raw, err := a.store.QueryClusterMembers(ctx, repoID, projection, graphVersion, clusterIntID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]semantic.ClusterMemberRow, len(raw))
	for i, r := range raw {
		out[i] = semantic.ClusterMemberRow{NodeID: r.NodeID, SymbolID: r.SymbolID}
	}
	return out, nil
}

// p1StoreClusterPageRank wraps *Store.QueryNodePageRanks (same return type).
type p1StoreClusterPageRank struct{ store *semanticstore.Store }

func (a *p1StoreClusterPageRank) QueryNodePageRanks(ctx context.Context, repoID, projection string, graphVersion uint64, nodeIDs []uint64) (map[uint64]float64, error) {
	return a.store.QueryNodePageRanks(ctx, repoID, projection, graphVersion, nodeIDs)
}

// ---------------------------------------------------------------------------
// Inline fixture adapters for seams without *Store SQL readers.
// ---------------------------------------------------------------------------

// p1InlineClusterMembership returns cluster IDs matching the seeded layout.
type p1InlineClusterMembership struct{}

func (a *p1InlineClusterMembership) ClusterIDOf(_ context.Context, _ string, sym integ.SymbolID) (uint64, int, error) {
	switch string(sym) {
	case p1SeedGoServeHTTP, p1SeedGoHandle, p1SeedGoRequest:
		return 1, 3, nil
	case p1SeedTSFetch, p1SeedTSUser:
		return 2, 2, nil
	case p1SeedJavaFooBar, p1SeedJavaBar:
		return 3, 2, nil
	}
	return 0, 0, nil
}

// p1InlineTypeChain returns a tier1_lsp type-chain entry for seeded seeds.
type p1InlineTypeChain struct{}

func (a *p1InlineTypeChain) TypeChainForSymbol(_ context.Context, _ string, sym integ.SymbolID) ([]semantic.TypeChainRow, error) {
	switch string(sym) {
	case p1SeedGoServeHTTP:
		return []semantic.TypeChainRow{{Tier: "tier1_lsp", EvidenceKind: "lsp", TargetSymbolID: p1SeedGoHandle}}, nil
	case p1SeedTSFetch:
		return []semantic.TypeChainRow{{Tier: "tier1_lsp", EvidenceKind: "lsp", TargetSymbolID: p1SeedTSUser}}, nil
	case p1SeedJavaFooBar:
		return []semantic.TypeChainRow{{Tier: "tier1_lsp", EvidenceKind: "lsp", TargetSymbolID: p1SeedJavaBar}}, nil
	}
	return []semantic.TypeChainRow{}, nil
}

// p1InlineSymbolEdges returns CALLS edges for the seeded symbol adjacency.
type p1InlineSymbolEdges struct{}

func (a *p1InlineSymbolEdges) CallersOf(_ context.Context, _ string, sym integ.SymbolID) ([]semantic.SymbolEdgeRow, error) {
	if string(sym) == p1SeedGoHandle {
		return []semantic.SymbolEdgeRow{
			{From: integ.SymbolID(p1SeedGoServeHTTP), To: integ.SymbolID(p1SeedGoHandle), InternalKind: "CALLS"},
		}, nil
	}
	return []semantic.SymbolEdgeRow{}, nil
}

func (a *p1InlineSymbolEdges) IncomingEdgesOf(_ context.Context, _ string, sym integ.SymbolID) ([]semantic.SymbolEdgeRow, error) {
	if string(sym) == p1SeedGoHandle {
		return []semantic.SymbolEdgeRow{
			{From: integ.SymbolID(p1SeedGoServeHTTP), To: integ.SymbolID(p1SeedGoHandle), InternalKind: "CALLS"},
		}, nil
	}
	return []semantic.SymbolEdgeRow{}, nil
}

func (a *p1InlineSymbolEdges) OutgoingEdgesOf(_ context.Context, _ string, sym integ.SymbolID) ([]semantic.SymbolEdgeRow, error) {
	if string(sym) == p1SeedGoServeHTTP {
		return []semantic.SymbolEdgeRow{
			{From: integ.SymbolID(p1SeedGoServeHTTP), To: integ.SymbolID(p1SeedGoHandle), InternalKind: "CALLS"},
		}, nil
	}
	return []semantic.SymbolEdgeRow{}, nil
}

// p1InlineEdgeEvidence returns LSP evidence for the ServeHTTP → handle CALLS edge.
type p1InlineEdgeEvidence struct{}

func (a *p1InlineEdgeEvidence) EvidenceForEdge(_ context.Context, _ string, from, to integ.SymbolID, internalKinds []string) ([]semantic.EdgeEvidenceRow, error) {
	if string(from) == p1SeedGoServeHTTP && string(to) == p1SeedGoHandle {
		for _, k := range internalKinds {
			if k == "CALLS" {
				return []semantic.EdgeEvidenceRow{
					{
						InternalKind:   "CALLS",
						Source:         "lsp.go.text_document_references",
						TreeSitterKind: "call_expression",
						File:           "repo/src/svc.go",
						Range:          &semantic.EvidenceRange{StartLine: 12, EndLine: 12, EndCol: 20},
						Tier:           "tier1_lsp",
						EvidenceKind:   "lsp",
					},
				}, nil
			}
		}
	}
	return []semantic.EdgeEvidenceRow{}, nil
}

// p1IntegLookupAcc wraps integ.SemanticLookup as an ImpactLookupAccessor.
type p1IntegLookupAcc struct{ lookup integ.SemanticLookup }

func (a *p1IntegLookupAcc) ExpandFrom(ctx context.Context, ws workspace.WorkspaceKey, sym integ.SymbolID, depth int) ([]integ.Impact, error) {
	return a.lookup.ExpandFrom(ctx, ws, sym, depth)
}
func (a *p1IntegLookupAcc) Status(ctx context.Context, ws workspace.WorkspaceKey) (integ.SemanticStatus, error) {
	return a.lookup.Status(ctx, ws)
}

// ---------------------------------------------------------------------------
// Core skill accessors (mirrors status_e2e_external_test.go stubs but with
// p1-prefixed names to avoid redeclaration in the same package).
// ---------------------------------------------------------------------------

// p1PlainStoreAcc is the StoreAccessor backed by the real *Store.
type p1PlainStoreAcc struct{ store *semanticstore.Store }

func (a *p1PlainStoreAcc) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	return a.store.LatestCommittedSnapshot(ctx, repoID)
}
func (a *p1PlainStoreAcc) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return a.store.CurrentGraphVersion(ctx, repoID)
}
func (a *p1PlainStoreAcc) OverlayHasPendingRows(repoID string) bool {
	return a.store.OverlayHasPendingRows(repoID)
}
func (a *p1PlainStoreAcc) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	map[graph.NodeID]map[graph.NodeID]float64,
	map[graph.NodeID]map[graph.NodeID]float64, error,
) {
	return a.store.QueryEffectiveAdjacency(ctx, repoID, projection)
}
func (a *p1PlainStoreAcc) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
	return a.store.CurrentOverlayEpoch(ctx, repoID)
}
func (a *p1PlainStoreAcc) OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) ([]string, uint64, error) {
	return a.store.OverlayChangedPathsSince(ctx, repoID, baseEpoch)
}
func (a *p1PlainStoreAcc) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (uint64, bool, error) {
	return a.store.LatestCommittedSnapshotBaseEpoch(ctx, repoID)
}

type p1PlaceholderSchedAcc struct{}

func (p1PlaceholderSchedAcc) IsQuiescent(_ string) bool { return true }
func (p1PlaceholderSchedAcc) ScoreStatus(_, _ string) graph.ScoreStatus {
	return graph.ScoreStatusMissing
}
func (p1PlaceholderSchedAcc) ClusterStatus(_ string) semantic.ClusterStatus {
	return semantic.ClusterStatus{State: "unknown", Reason: "placeholder"}
}

type p1ZeroQueueAcc struct{}

func (p1ZeroQueueAcc) DepthAll(_ workspace.WorkspaceKey) int        { return 0 }
func (p1ZeroQueueAcc) LastEnqueueAt(_ workspace.WorkspaceKey) int64 { return 0 }

type p1ZeroLiveAcc struct{}

func (p1ZeroLiveAcc) OnWorkspaceChanged(_ workspace.WorkspaceKey, _ []string) error { return nil }
func (p1ZeroLiveAcc) LastFlushAt(_ workspace.WorkspaceKey) int64                    { return 0 }
func (p1ZeroLiveAcc) FlushNow(_ context.Context, _ workspace.WorkspaceKey) error    { return nil }

type p1PlaceholderRetrievalAcc struct{}

func (p1PlaceholderRetrievalAcc) QueryBleve(_ string, _ []string) ([]semantic.TextRank, error) {
	return nil, nil
}
func (p1PlaceholderRetrievalAcc) PersonalizedPageRank(_ context.Context, _ string, _ []string) ([]semantic.GraphRank, error) {
	return nil, nil
}
func (p1PlaceholderRetrievalAcc) RetrievalPending(_ workspace.WorkspaceKey) bool { return false }
func (p1PlaceholderRetrievalAcc) TopEdgesFor(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (p1PlaceholderRetrievalAcc) RetrievalStatus(_ workspace.WorkspaceKey) semantic.RetrievalStatus {
	return semantic.RetrievalStatus{}
}

type p1NoopCompactAcc struct{}

func (p1NoopCompactAcc) OnFlush(_ workspace.WorkspaceKey) error { return nil }

type p1StaticSessionAcc struct {
	ws   workspace.WorkspaceKey
	sess *mcp.SessionInfo
}

func (a *p1StaticSessionAcc) Session(_ context.Context) *mcp.SessionInfo         { return a.sess }
func (a *p1StaticSessionAcc) Workspace(_ context.Context) workspace.WorkspaceKey { return a.ws }

// ---------------------------------------------------------------------------
// Closed-enum validation helpers.
// ---------------------------------------------------------------------------

// p1ValidFreshnessStatuses is the three-state FreshnessStatus closed enum.
var p1ValidFreshnessStatuses = map[string]bool{
	"current": true, "stale": true, "unknown": true,
}

// p1ValidFreshnessSources is the FreshnessSource closed enum plus empty (allowed
// when freshness.source is omitted on degraded paths).
var p1ValidFreshnessSources = map[string]bool{
	"graph": true, "type_resolver_ladder": true, "ast_fallback": true, "": true,
}

// p1ValidFallbackReasons is the union of all declared fallback_reason values
// across the 6 P1 tools. Empty string = success path (no fallback).
var p1ValidFallbackReasons = map[string]bool{
	"":                           true,
	"symbol_not_found":           true,
	"type_resolver_degraded":     true,
	"cluster_boost_unavailable":  true,
	"cluster_map_unavailable":    true,
	"cluster_member_unavailable": true,
	"edge_not_found":             true,
	"impact_lookup_unavailable":  true,
}

// assertP1Envelope checks closed-enum membership for FreshnessV2 + confidence
// + fallback_reason fields on a decoded JSON result map.
func assertP1Envelope(t *testing.T, tool, text string) {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &body),
		"%s: result is not JSON: %s", tool, text)

	// freshness must be present and an object.
	rawFresh, ok := body["freshness"]
	require.True(t, ok, "%s: result must have 'freshness' field", tool)
	freshMap, ok := rawFresh.(map[string]interface{})
	require.True(t, ok, "%s: freshness must be an object", tool)

	status, _ := freshMap["status"].(string)
	require.True(t, p1ValidFreshnessStatuses[status],
		"%s: freshness.status %q not in {current, stale, unknown}", tool, status)

	source, _ := freshMap["source"].(string)
	require.True(t, p1ValidFreshnessSources[source],
		"%s: freshness.source %q is not a declared FreshnessSource", tool, source)

	// fallback_reason must be empty or a declared closed-enum value.
	fallbackReason, _ := body["fallback_reason"].(string)
	require.True(t, p1ValidFallbackReasons[fallbackReason],
		"%s: fallback_reason %q is not in the declared closed-enum set", tool, fallbackReason)

	// confidence ∈ [0.0, 1.0] — present on some tools, absent on others.
	if rawConf, exists := body["confidence"]; exists {
		conf, ok := rawConf.(float64)
		require.True(t, ok, "%s: confidence must be float64, got %T", tool, rawConf)
		require.GreaterOrEqual(t, conf, 0.0, "%s: confidence must be >= 0.0", tool)
		require.LessOrEqual(t, conf, 1.0, "%s: confidence must be <= 1.0", tool)
	}
}

// ---------------------------------------------------------------------------
// Task 3: Fixture smoke test.
// ---------------------------------------------------------------------------

// TestP1E2EFixture_SeedsNonEmpty asserts that buildP1E2EFixture provisions a
// real store with symbol_count > 0, adjacency_count > 0, and cluster_count > 0.
// This isolates fixture-construction failures from per-tool assertion failures.
func TestP1E2EFixture_SeedsNonEmpty(t *testing.T) {
	ctx := context.Background()
	fix := buildP1E2EFixture(t)

	// Symbol count: the committed snapshot must carry 7 symbols.
	snapID, err := fix.store.LatestCommittedSnapshot(ctx, fix.repoID)
	require.NoError(t, err)
	require.NotZero(t, snapID, "fixture must commit a non-zero snapshot_id")

	symCount := 0
	require.NoError(t, fix.store.IterateCommittedSymbols(ctx, snapID, func(_ semanticstore.SymbolRow) bool {
		symCount++
		return true
	}))
	require.Greater(t, symCount, 0, "fixture must seed > 0 symbols")

	// Edge count: the "call_graph" effective adjacency must have > 0 edges.
	out, _, err := fix.store.QueryEffectiveAdjacency(ctx, fix.repoID, "call_graph")
	require.NoError(t, err)
	require.Greater(t, len(out), 0, "fixture must seed > 0 edges in call_graph adjacency")

	// Cluster count: the current graph_version must have > 0 clusters.
	clusters, err := fix.store.QueryClusterSummaries(ctx, fix.repoID, "weak_components", fix.graphVersion, 100)
	require.NoError(t, err)
	require.Greater(t, len(clusters), 0, "fixture must seed > 0 clusters")
}

// ---------------------------------------------------------------------------
// Task 4: 6 per-tool real-store E2E closed-enum assertion tests.
// ---------------------------------------------------------------------------

// TestP1E2E_ExplainSymbolDeep invokes explain_symbol_deep against the shared
// real-store fixture and asserts closed-enum envelope fields.
func TestP1E2E_ExplainSymbolDeep(t *testing.T) {
	ctx := context.Background()
	fix := buildP1E2EFixture(t)

	res := semantic.HandleExplainSymbolDeepForTest(fix.skill, ctx, semantic.ExplainSymbolDeepArgs{
		Seed: semantic.SeedInput{SymbolID: p1SeedGoServeHTTP},
	})
	require.NotNil(t, res)
	require.False(t, res.IsError, "explain_symbol_deep returned error: %s", extractText(t, res))

	text := extractText(t, res)
	assertP1Envelope(t, "explain_symbol_deep", text)

	// On the found-symbol path, fallback_reason must be empty.
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &body))
	fallback, _ := body["fallback_reason"].(string)
	require.Empty(t, fallback, "explain_symbol_deep on known symbol must not set fallback_reason")
}

// TestP1E2E_FindRelatedSymbols invokes find_related_symbols against the shared
// real-store fixture and asserts closed-enum envelope fields.
func TestP1E2E_FindRelatedSymbols(t *testing.T) {
	ctx := context.Background()
	fix := buildP1E2EFixture(t)

	res := semantic.HandleFindRelatedSymbolsForTest(fix.skill, ctx, semantic.FindRelatedSymbolsArgs{
		Seed: semantic.SeedInput{SymbolID: p1SeedGoServeHTTP},
		K:    10,
	})
	require.NotNil(t, res)
	require.False(t, res.IsError, "find_related_symbols returned error: %s", extractText(t, res))

	text := extractText(t, res)
	assertP1Envelope(t, "find_related_symbols", text)

	// seed.resolution must be a valid closed-enum value.
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &body))
	seed, _ := body["seed"].(map[string]interface{})
	require.NotNil(t, seed, "find_related_symbols: result must have seed object")
	resolution, _ := seed["resolution"].(string)
	require.Contains(t, []string{"exact", "ambiguous", "not_found"}, resolution,
		"find_related_symbols: seed.resolution must be a declared Resolution value")
}

// TestP1E2E_ValidateGraphEdge invokes validate_graph_edge against the shared
// real-store fixture and asserts closed-enum envelope fields.
func TestP1E2E_ValidateGraphEdge(t *testing.T) {
	ctx := context.Background()
	fix := buildP1E2EFixture(t)

	res := semantic.HandleValidateGraphEdgeForTest(fix.skill, ctx, semantic.ValidateGraphEdgeArgs{
		From:     semantic.SeedInput{SymbolID: p1SeedGoServeHTTP},
		To:       semantic.SeedInput{SymbolID: p1SeedGoHandle},
		EdgeKind: "calls",
	})
	require.NotNil(t, res)
	require.False(t, res.IsError, "validate_graph_edge returned error: %s", extractText(t, res))

	text := extractText(t, res)
	assertP1Envelope(t, "validate_graph_edge", text)

	// confidence must be present and ∈ [0.0, 1.0] (already checked in assertP1Envelope).
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &body))
	rawConf, ok := body["confidence"]
	require.True(t, ok, "validate_graph_edge: result must have 'confidence' field")
	conf, _ := rawConf.(float64)
	require.GreaterOrEqual(t, conf, 0.0)
	require.LessOrEqual(t, conf, 1.0)
}

// TestP1E2E_GetClusterMap invokes get_cluster_map against the shared
// real-store fixture and asserts closed-enum envelope fields.
func TestP1E2E_GetClusterMap(t *testing.T) {
	ctx := context.Background()
	fix := buildP1E2EFixture(t)

	res := semantic.HandleGetClusterMapForTest(fix.skill, ctx, semantic.GetClusterMapArgs{TopN: 10})
	require.NotNil(t, res)
	require.False(t, res.IsError, "get_cluster_map returned error: %s", extractText(t, res))

	text := extractText(t, res)
	assertP1Envelope(t, "get_cluster_map", text)

	// With a wired ClusterMapAccessor, fallback must not be "cluster_map_unavailable".
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &body))
	fallback, _ := body["fallback_reason"].(string)
	require.Empty(t, fallback, "get_cluster_map with wired accessor must not fall back")
	totalClusters, _ := body["total_clusters"].(float64)
	require.Greater(t, int(totalClusters), 0, "get_cluster_map must return > 0 clusters from seeded data")
}

// TestP1E2E_ExplainCluster invokes explain_cluster against the shared
// real-store fixture and asserts closed-enum envelope fields.
func TestP1E2E_ExplainCluster(t *testing.T) {
	ctx := context.Background()
	fix := buildP1E2EFixture(t)

	res := semantic.HandleExplainClusterForTest(fix.skill, ctx, semantic.ExplainClusterArgs{
		ClusterID:  fix.seedClusterIDEncoded,
		MaxMembers: 100,
	})
	require.NotNil(t, res)
	require.False(t, res.IsError, "explain_cluster returned error: %s", extractText(t, res))

	text := extractText(t, res)
	assertP1Envelope(t, "explain_cluster", text)

	// cluster_id must echo back the input token.
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &body))
	clusterID, _ := body["cluster_id"].(string)
	require.Equal(t, fix.seedClusterIDEncoded, clusterID,
		"explain_cluster must echo back the input cluster_id")
}

// TestP1E2E_GetChangeImpactGraph invokes get_change_impact_graph against the
// shared real-store fixture and asserts closed-enum envelope fields.
func TestP1E2E_GetChangeImpactGraph(t *testing.T) {
	ctx := context.Background()
	fix := buildP1E2EFixture(t)

	res := semantic.HandleGetChangeImpactGraphForTest(fix.skill, ctx, semantic.GetChangeImpactGraphArgs{
		Seed:     semantic.SeedInput{SymbolID: p1SeedGoServeHTTP},
		MaxDepth: 2,
	})
	require.NotNil(t, res)
	require.False(t, res.IsError, "get_change_impact_graph returned error: %s", extractText(t, res))

	text := extractText(t, res)
	assertP1Envelope(t, "get_change_impact_graph", text)

	// ImpactLookup is wired — fallback must not be "impact_lookup_unavailable".
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(text), &body))
	fallback, _ := body["fallback_reason"].(string)
	require.NotEqual(t, "impact_lookup_unavailable", fallback,
		"get_change_impact_graph: ImpactLookup is wired; must not report impact_lookup_unavailable")

	// If ConfidenceCap is non-nil, its reason must be the declared enum value.
	if rawCap, ok := body["confidence_cap"]; ok && rawCap != nil {
		capMap, isMap := rawCap.(map[string]interface{})
		require.True(t, isMap, "confidence_cap must be an object")
		capVal, _ := capMap["value"].(float64)
		require.GreaterOrEqual(t, capVal, 0.0)
		require.LessOrEqual(t, capVal, 1.0)
		capReason, _ := capMap["reason"].(string)
		require.Equal(t, "type_resolver_tier_3", capReason,
			"confidence_cap.reason must be 'type_resolver_tier_3'")
	}
}
