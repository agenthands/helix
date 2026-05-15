// status_e2e_external_test.go — Phase 69 Plan 69-06 (STATUS-03):
// end-to-end integration test that exercises the production
// daemon.NewSchedulerAccessorForStore + daemon.NewRetrievalAccessorForStore
// factory constructors through the real get_semantic_graph_status
// handler. This is the SAME code path the daemon's semSchedulerAdapter
// / semRetrievalAdapter invoke at runtime — not a parallel test-local
// reimplementation (revision Warning 3 closure).
//
// Package-cycle resolution: this file lives in `package semantic_test`
// (black-box test package) so it can import internal/daemon for the
// factory constructors. The handler entry point is reached via the
// test-only `HandleGetSemanticGraphStatusForTest` export defined in
// export_status_test.go (`package semantic`).

package semantic_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/daemon"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/obs"
	semanticpkg "github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/retrieval"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/skill/semantic"
	"github.com/agenthands/helix/internal/workspace"
)

// ---------- minimal harness (mirrors integration_test.go newE2EHarness,
// inlined here because the in-package harness is unexported) ----------

// statusE2EHarness owns the per-test *Store + bleve engine + recoverer +
// SemanticSkill assembled with PLACEHOLDER adapters that the test will
// later swap for the production daemon factory constructors.
type statusE2EHarness struct {
	store     *semanticstore.Store
	engine    *retrieval.Engine
	recoverer *retrieval.Recoverer
	skill     *semantic.SemanticSkill
	ws        workspace.WorkspaceKey
}

func newStatusE2EHarness(t *testing.T, factsForFull semanticstore.Facts) *statusE2EHarness {
	t.Helper()

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
	store, err := semanticstore.Open(context.Background(), storeCfg, logger, provider.Metrics())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	bleveDir := filepath.Join(wsDir, ".helix", "semantic.bleve")
	engine, err := retrieval.New(bleveDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })

	rec := retrieval.NewRecoverer(engine, store, logger)

	ws := workspace.WorkspaceKey{RepoRoot: wsDir, Language: "go", Toolchain: "go1.22"}

	h := &statusE2EHarness{
		store:     store,
		engine:    engine,
		recoverer: rec,
		ws:        ws,
	}

	s := &semantic.SemanticSkill{}
	require.NoError(t, s.Init(skill.SkillDeps{}))

	// Placeholder adapters — the test will swap scheduler + retrieval for
	// the production daemon factory constructors before calling status.
	s.SetStore(&plainStoreAcc{store: store})
	s.SetScheduler(&placeholderSchedAcc{})
	s.SetQueue(&zeroQueueAcc{})
	s.SetLive(&zeroLiveAcc{})
	s.SetRetrieval(&placeholderRetrievalAcc{})
	s.SetCompactor(&noopCompactAcc{})
	s.SetSessionAccessor(&staticSessionAcc{
		ws:   ws,
		sess: &mcp.SessionInfo{SessionID: "test-session", Mode: "review"},
	})

	// Production-shape buildFn: BeginSnapshot → WriteSnapshotFacts(facts) →
	// CommitSnapshot. Mirrors integration_test.go::makeBuildFn.
	buildFn := func(ctx context.Context, ws workspace.WorkspaceKey, mode string, st semantic.BuildState) (semantic.IndexResult, error) {
		repoID := ws.Hash()
		startedAt := time.Now()
		var baseSnapshotID uint64
		if mode == "incremental" {
			latest, err := store.LatestCommittedSnapshot(ctx, repoID)
			if err != nil {
				return semantic.IndexResult{}, fmt.Errorf("LatestCommittedSnapshot: %w", err)
			}
			baseSnapshotID = latest
		}
		snap, err := store.BeginSnapshot(ctx, semanticstore.SnapshotMeta{
			RepoID:         repoID,
			BaseSnapshotID: baseSnapshotID,
		})
		if err != nil {
			return semantic.IndexResult{}, fmt.Errorf("BeginSnapshot: %w", err)
		}
		committed := false
		defer func() {
			if !committed {
				_ = store.AbortSnapshot(context.Background(), snap, "buildFn: not committed")
			}
		}()
		st.SetSnapshotID(snap.ID)

		factsToWrite := factsForFull
		if mode == "incremental" {
			factsToWrite = semanticstore.Facts{}
		}
		// Stamp Files[*].RepoID so FK on semantic_files holds.
		stamped := factsToWrite
		if len(factsToWrite.Files) > 0 {
			ff := make([]semanticstore.FileFact, len(factsToWrite.Files))
			copy(ff, factsToWrite.Files)
			for i := range ff {
				ff[i].RepoID = snap.RepoID
			}
			stamped.Files = ff
		}
		if err := store.WriteSnapshotFacts(ctx, snap, stamped); err != nil {
			return semantic.IndexResult{}, fmt.Errorf("WriteSnapshotFacts: %w", err)
		}
		summary := semanticstore.SnapshotSummary{
			SymbolCount: len(stamped.Symbols),
			DurationMs:  time.Since(startedAt).Milliseconds(),
		}
		if err := store.CommitSnapshot(ctx, snap, summary); err != nil {
			return semantic.IndexResult{}, fmt.Errorf("CommitSnapshot: %w", err)
		}
		committed = true
		return semantic.IndexResult{
			SnapshotID:   snap.ID,
			FilesIndexed: int64(len(stamped.Files)),
			DurationMs:   time.Since(startedAt).Milliseconds(),
		}, nil
	}
	runner := semantic.NewProductionIndexRunner(&plainStoreAcc{store: store}, buildFn, 30*time.Second)
	s.SetRunner(runner)

	h.skill = s
	return h
}

// ---------- placeholder accessors ----------

type plainStoreAcc struct{ store *semanticstore.Store }

func (a *plainStoreAcc) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	return a.store.LatestCommittedSnapshot(ctx, repoID)
}
func (a *plainStoreAcc) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return a.store.CurrentGraphVersion(ctx, repoID)
}
func (a *plainStoreAcc) OverlayHasPendingRows(repoID string) bool {
	return a.store.OverlayHasPendingRows(repoID)
}
func (a *plainStoreAcc) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	map[graph.NodeID]map[graph.NodeID]float64, map[graph.NodeID]map[graph.NodeID]float64, error,
) {
	return a.store.QueryEffectiveAdjacency(ctx, repoID, projection)
}
func (a *plainStoreAcc) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
	return a.store.CurrentOverlayEpoch(ctx, repoID)
}
func (a *plainStoreAcc) OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) ([]string, uint64, error) {
	return a.store.OverlayChangedPathsSince(ctx, repoID, baseEpoch)
}
func (a *plainStoreAcc) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (uint64, bool, error) {
	return a.store.LatestCommittedSnapshotBaseEpoch(ctx, repoID)
}

type placeholderSchedAcc struct{}

func (placeholderSchedAcc) IsQuiescent(_ string) bool { return true }
func (placeholderSchedAcc) ScoreStatus(_, _ string) graph.ScoreStatus {
	return graph.ScoreStatusMissing
}
func (placeholderSchedAcc) ClusterStatus(_ string) semantic.ClusterStatus {
	return semantic.ClusterStatus{State: "unknown", Reason: "placeholder"}
}

type zeroQueueAcc struct{}

func (zeroQueueAcc) DepthAll(_ workspace.WorkspaceKey) int        { return 0 }
func (zeroQueueAcc) LastEnqueueAt(_ workspace.WorkspaceKey) int64 { return 0 }

type zeroLiveAcc struct{}

func (zeroLiveAcc) OnWorkspaceChanged(_ workspace.WorkspaceKey, _ []string) error { return nil }
func (zeroLiveAcc) LastFlushAt(_ workspace.WorkspaceKey) int64                    { return 0 }
func (zeroLiveAcc) FlushNow(_ context.Context, _ workspace.WorkspaceKey) error    { return nil }

type placeholderRetrievalAcc struct{}

func (placeholderRetrievalAcc) QueryBleve(_ string, _ []string) ([]semantic.TextRank, error) {
	return nil, nil
}
func (placeholderRetrievalAcc) PersonalizedPageRank(_ context.Context, _ string, _ []string) ([]semantic.GraphRank, error) {
	return nil, nil
}
func (placeholderRetrievalAcc) RetrievalPending(_ workspace.WorkspaceKey) bool { return false }
func (placeholderRetrievalAcc) TopEdgesFor(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (placeholderRetrievalAcc) RetrievalStatus(_ workspace.WorkspaceKey) semantic.RetrievalStatus {
	return semantic.RetrievalStatus{}
}

type noopCompactAcc struct{}

func (noopCompactAcc) OnFlush(_ workspace.WorkspaceKey) error { return nil }

type staticSessionAcc struct {
	ws   workspace.WorkspaceKey
	sess *mcp.SessionInfo
}

func (a *staticSessionAcc) Session(_ context.Context) *mcp.SessionInfo         { return a.sess }
func (a *staticSessionAcc) Workspace(_ context.Context) workspace.WorkspaceKey { return a.ws }

// ---------- fixture builder ----------

func makeStatusFixtureFacts(symbolCount int) semanticstore.Facts {
	files := []semanticstore.FileFact{
		{
			FileID:      1,
			Path:        "src/fixture.go",
			Language:    "go",
			ContentHash: "fixture-hash",
		},
	}
	symbols := make([]semanticstore.SymbolFact, symbolCount)
	for i := 0; i < symbolCount; i++ {
		symbols[i] = semanticstore.SymbolFact{
			SymbolID:      uint64(100 + i),
			NodeID:        uint64(200 + i),
			FileID:        1,
			Language:      "go",
			Kind:          "function",
			Name:          fmt.Sprintf("Symbol_%d", i),
			QualifiedName: fmt.Sprintf("fixture.Symbol_%d", i),
			StableKey:     fmt.Sprintf("fixture-symbol-%d", i),
			Visibility:    "public",
			Confidence:    1.0,
		}
	}
	return semanticstore.Facts{Files: files, Symbols: symbols}
}

// extractText returns the first TextContent's text from a CallToolResult.
func extractText(t *testing.T, res *mcpsdk.CallToolResult) string {
	t.Helper()
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatalf("no TextContent in result %+v", res)
	return ""
}

// ---------- test ----------

// TestE2E_IndexThenStatus_NonPlaceholderClusterAndRetrieval closes STATUS-03
// (Phase 69 success criterion #4). It builds a 15-symbol indexed workspace,
// primes the bleve corpus via the real Recoverer.Probe path (so
// MetaKeyCorpusVersion + MetaKeyIndexedFiles are stamped exactly the way
// production stamps them), seeds 2 cluster_summary + 5 cluster_members
// rows for the current graph_version, swaps the placeholder
// scheduler/retrieval adapters for instances produced by the REAL daemon
// factory constructors (daemon.NewSchedulerAccessorForStore +
// daemon.NewRetrievalAccessorForStore), then invokes the
// get_semantic_graph_status handler and asserts non-placeholder values
// across both cluster_status and retrieval_status.
//
// Revision Warning 3 closure: the test exercises EXACTLY the same
// (*schedulerAccessorImpl).ClusterStatus / (*retrievalAccessorImpl).RetrievalStatus
// code paths the daemon's semSchedulerAdapter / semRetrievalAdapter use at
// runtime, not a parallel test-local re-implementation.
func TestE2E_IndexThenStatus_NonPlaceholderClusterAndRetrieval(t *testing.T) {
	ctx := context.Background()
	h := newStatusE2EHarness(t, makeStatusFixtureFacts(15))

	// 1. Index full — commits a 15-symbol snapshot through the production
	//    BeginSnapshot/WriteSnapshotFacts/CommitSnapshot pipeline.
	idxRes := semantic.HandleIndexSemanticGraphForTest(h.skill, ctx, "full")
	require.False(t, idxRes.IsError, "index returned error: %s", extractText(t, idxRes))
	var ir semantic.IndexResult
	require.NoError(t, json.Unmarshal([]byte(extractText(t, idxRes)), &ir))
	require.NotZero(t, ir.SnapshotID, "index must commit a non-zero snapshot")

	// 2. Resolve repoID + current graph_version. graph_version is bumped by
	//    BeginOverlayTx when the cluster seeding runs (step 4), so at this
	//    point CurrentGraphVersion is still 0 — we read it AFTER the overlay
	//    tx commits.
	repoID := h.ws.Hash()

	// 3. Open an overlay tx (this bumps graph_version to 1) and seed
	//    2 clusters totaling 5 members at that graph_version. Use the
	//    "weak_components" projection name — matches the Phase 62 cluster
	//    detector identifier and the factory's ClusterStatusForGraphVersion
	//    semantics (any projection works; the factory aggregates by
	//    graph_version, not by projection).
	tx, err := h.store.BeginOverlayTx(ctx, repoID)
	require.NoError(t, err)
	// BumpGraphVersion is the canonical advancer (D-06: ONLY ApplyRepair calls
	// this in production; the test stands in for that path so the cluster
	// rows below land at a non-zero graph_version, which is what the factory
	// reads back via ClusterStatusForGraphVersion).
	gv, err := tx.BumpGraphVersion(ctx)
	require.NoError(t, err)
	require.Greater(t, gv, uint64(0), "BumpGraphVersion must yield graph_version > 0")

	require.NoError(t, tx.UpsertClusters(ctx, "weak_components", gv, []semanticstore.ClusterSummary{
		{ID: 1, MemberCount: 3},
		{ID: 2, MemberCount: 2},
	}))
	require.NoError(t, tx.UpsertClusterMembers(ctx, "weak_components", gv, []semanticstore.ClusterMemberRow{
		{ClusterID: 1, NodeID: 100},
		{ClusterID: 1, NodeID: 101},
		{ClusterID: 1, NodeID: 102},
		{ClusterID: 2, NodeID: 200},
		{ClusterID: 2, NodeID: 201},
	}))
	require.NoError(t, tx.Commit())

	// 4. Prime the bleve corpus via the production Recoverer.Probe path.
	//    Probe spawns an async rebuild; rebuildBlocking iterates the
	//    committed snapshot, batches SymbolDocs into bleve, then stamps
	//    MetaKeyCorpusVersion (= current graph_version) and
	//    MetaKeyIndexedFiles (= distinct file count). We poll
	//    RetrievalPending until the rebuild completes so the meta is
	//    durably visible to the factory accessor.
	require.NoError(t, h.recoverer.Probe(ctx, h.ws))
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && h.recoverer.RetrievalPending(h.ws) {
		time.Sleep(20 * time.Millisecond)
	}
	require.False(t, h.recoverer.RetrievalPending(h.ws),
		"bleve rebuild did not complete within 15s")

	// 5. Swap placeholder accessors for the REAL daemon factory constructors.
	//    This is the heart of the revision Warning 3 fix: the assertions
	//    below now exercise the same (*schedulerAccessorImpl).ClusterStatus
	//    and (*retrievalAccessorImpl).RetrievalStatus code paths the daemon's
	//    semSchedulerAdapter / semRetrievalAdapter invoke at runtime.
	schedAcc := daemon.NewSchedulerAccessorForStore(h.store)
	retrAcc := daemon.NewRetrievalAccessorForStore(h.store, h.engine)
	h.skill.SetScheduler(schedAcc)
	h.skill.SetRetrieval(retrAcc)

	// 6. Invoke the status handler and decode the envelope.
	statRes := semantic.HandleGetSemanticGraphStatusForTest(h.skill, ctx)
	require.False(t, statRes.IsError, "status returned error: %s", extractText(t, statRes))
	var sr semantic.StatusResult
	require.NoError(t, json.Unmarshal([]byte(extractText(t, statRes)), &sr))

	// 7. ClusterStatus assertions — production factory derives state=current
	//    when row.ClusterCount > 0 AND row.IsCurrent. Seeded 2 clusters × 5
	//    members at the current graph_version, so:
	require.Equal(t, "current", sr.ClusterStatus.State,
		"ClusterStatus.State: expected 'current' (factory reads ClusterStatusForGraphVersion(repoID, gv) and sees IsCurrent==true)")
	require.Equal(t, 5, sr.ClusterStatus.MemberCount,
		"ClusterStatus.MemberCount: expected 5 (sum of seeded member counts: 3 + 2)")
	require.Greater(t, sr.ClusterStatus.ComputedAt, int64(0),
		"ClusterStatus.ComputedAt: must be > 0 (UpsertClusters stamps now() into computed_at)")

	// 8. RetrievalStatus assertions — production factory reads
	//    MetaKeyCorpusVersion / MetaKeyIndexedFiles / MetaKeyLastCompactAt
	//    from the bleve engine; Recoverer.rebuildBlocking just stamped the
	//    first two (compactor never ran in this fixture, so the third is
	//    absent and Reason should be "compactor-never-ran"). IndexedSymbols
	//    comes from engine.DocCount() — 15 symbols indexed via the rebuild.
	require.Equal(t, gv, sr.RetrievalStatus.CorpusVersion,
		"RetrievalStatus.CorpusVersion: expected to equal current graph_version (factory parses MetaKeyCorpusVersion bytes stamped by Recoverer.rebuildBlocking)")
	require.Greater(t, sr.RetrievalStatus.IndexedSymbols, int64(0),
		"RetrievalStatus.IndexedSymbols: expected > 0 (factory reads engine.DocCount() after rebuild upserted 15 SymbolDocs)")
	require.GreaterOrEqual(t, sr.RetrievalStatus.IndexedFiles, int64(1),
		"RetrievalStatus.IndexedFiles: expected >= 1 (factory parses MetaKeyIndexedFiles stamped by rebuildBlocking)")
	require.Contains(t, []string{"", "compactor-never-ran"}, sr.RetrievalStatus.Reason,
		"RetrievalStatus.Reason: expected empty or 'compactor-never-ran' (no compaction in fixture; lower-priority degradation reasons should NOT trigger)")
}
