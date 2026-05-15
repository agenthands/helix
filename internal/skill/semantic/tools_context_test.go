package semantic

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)

// tools_context_test.go — RED-gate failing tests for tools_context.go.
//
// Tests must compile (exercising the typed-args + handler signature) and
// fail at runtime due to handler panic. Task 3 GREEN replaces the panic
// with the real implementation and turns these tests green.

// ---------- mocks ----------

// mockRetrievalAccessor implements RetrievalAccessor with per-method
// injection points + invocation counters.
type mockRetrievalAccessor struct {
	textRanksReturn  []TextRank
	textRanksErr     error
	graphRanksReturn []GraphRank
	graphRanksErr    error
	pendingReturn    bool
	topEdgesReturn   []string
	topEdgesErr      error

	queryCalls    atomic.Int64
	pageRankCalls atomic.Int64
	pendingCalls  atomic.Int64
	topEdgesCalls atomic.Int64

	mu                  sync.Mutex
	topEdgesByID        map[string][]string // optional per-symbol override
	queriesObserved     []string
	personalizeObserved [][]string
}

func (m *mockRetrievalAccessor) QueryBleve(task string, anchors []string) ([]TextRank, error) {
	m.queryCalls.Add(1)
	m.mu.Lock()
	m.queriesObserved = append(m.queriesObserved, task)
	m.mu.Unlock()
	return m.textRanksReturn, m.textRanksErr
}

func (m *mockRetrievalAccessor) PersonalizedPageRank(ctx context.Context, repoID string, anchors []string) ([]GraphRank, error) {
	m.pageRankCalls.Add(1)
	m.mu.Lock()
	m.personalizeObserved = append(m.personalizeObserved, append([]string(nil), anchors...))
	m.mu.Unlock()
	return m.graphRanksReturn, m.graphRanksErr
}

func (m *mockRetrievalAccessor) RetrievalPending(ws workspace.WorkspaceKey) bool {
	m.pendingCalls.Add(1)
	return m.pendingReturn
}

// RetrievalStatus is a zero-value stub. The get_semantic_context handler
// does not consume RetrievalStatus(ws); it only consumes RetrievalPending(ws).
// Plan 69-05 wires the production adapter that returns a meaningful value.
func (m *mockRetrievalAccessor) RetrievalStatus(ws workspace.WorkspaceKey) RetrievalStatus {
	return RetrievalStatus{}
}

func (m *mockRetrievalAccessor) TopEdgesFor(ctx context.Context, repoID, symbolID string) ([]string, error) {
	m.topEdgesCalls.Add(1)
	if m.topEdgesByID != nil {
		if v, ok := m.topEdgesByID[symbolID]; ok {
			return v, nil
		}
	}
	return m.topEdgesReturn, m.topEdgesErr
}

// mockStoreAccessorForContext implements StoreAccessor with the minimum
// surface needed by tools_context.go (CurrentGraphVersion +
// OverlayHasPendingRows). Other methods return zero/empty values.
type mockStoreAccessorForContext struct {
	graphVersion       uint64
	graphVersionErr    error
	overlayHasPending  bool
	latestSnapshot     uint64
	latestSnapshotErr  error
	queryAdjOut        map[graph.NodeID]map[graph.NodeID]float64
	queryAdjIn         map[graph.NodeID]map[graph.NodeID]float64
	queryAdjErr        error
	graphVersionCalls  atomic.Int64
	overlayActiveCalls atomic.Int64
}

func (m *mockStoreAccessorForContext) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	return m.latestSnapshot, m.latestSnapshotErr
}
func (m *mockStoreAccessorForContext) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	m.graphVersionCalls.Add(1)
	return m.graphVersion, m.graphVersionErr
}
func (m *mockStoreAccessorForContext) OverlayHasPendingRows(repoID string) bool {
	m.overlayActiveCalls.Add(1)
	return m.overlayHasPending
}
func (m *mockStoreAccessorForContext) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
) {
	return m.queryAdjOut, m.queryAdjIn, m.queryAdjErr
}
func (m *mockStoreAccessorForContext) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
	return 0, nil
}
func (m *mockStoreAccessorForContext) OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) ([]string, uint64, error) {
	return nil, 0, nil
}
func (m *mockStoreAccessorForContext) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (uint64, bool, error) {
	return 0, false, nil
}

// mockQueueAccessorForContext implements QueueAccessor with depth injection.
type mockQueueAccessorForContext struct {
	depth         int
	lastEnqueueAt int64
}

func (m *mockQueueAccessorForContext) DepthAll(ws workspace.WorkspaceKey) int {
	return m.depth
}
func (m *mockQueueAccessorForContext) LastEnqueueAt(ws workspace.WorkspaceKey) int64 {
	return m.lastEnqueueAt
}

// newSkillForContextTest constructs a SemanticSkill with the given session
// mode + retrieval/store/queue mocks wired up.
func newSkillForContextTest(
	t *testing.T,
	mode string,
	retrieval *mockRetrievalAccessor,
	store *mockStoreAccessorForContext,
	queue *mockQueueAccessorForContext,
) *SemanticSkill {
	t.Helper()
	s := &SemanticSkill{}
	if err := s.Init(skill.SkillDeps{}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	sess := &mcp.SessionInfo{SessionID: "test-context-session", Mode: mode}
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/repo-context-test", Language: "go", Toolchain: "go1.22"}
	s.SetSessionAccessor(&mockSessionAccessor{ws: ws, sess: sess})
	s.SetRetrieval(retrieval)
	s.SetStore(store)
	s.SetQueue(queue)
	return s
}

// ---------- happy-path: ranked + evidence-backed ----------

// TestContextHandler_HappyPath_RankedEvidence: text=[A,B,C], graph=[C,B,A].
// Top edges return [edge1, edge2]. Result.Candidates is non-empty; per-
// candidate Evidence is populated with TextRank, GraphRank, and TopEdges.
func TestContextHandler_HappyPath_RankedEvidence(t *testing.T) {
	retrieval := &mockRetrievalAccessor{
		textRanksReturn:  []TextRank{{SymbolID: "A"}, {SymbolID: "B"}, {SymbolID: "C"}},
		graphRanksReturn: []GraphRank{{SymbolID: "C"}, {SymbolID: "B"}, {SymbolID: "A"}},
		topEdgesReturn:   []string{"edge1", "edge2"},
	}
	store := &mockStoreAccessorForContext{graphVersion: 99}
	queue := &mockQueueAccessorForContext{depth: 0}
	s := newSkillForContextTest(t, "read", retrieval, store, queue)

	res := s.handleGetSemanticContext(context.Background(), GetSemanticContextArgs{
		Task: "implement user auth",
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	var out ContextResult
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, textOf(res))
	}
	if len(out.Candidates) == 0 {
		t.Fatalf("expected ≥1 candidate, got 0 (raw=%s)", textOf(res))
	}
	for i, c := range out.Candidates {
		if len(c.Evidence.TopEdges) == 0 {
			t.Fatalf("candidate[%d] %q: TopEdges empty; mock returned 2 edges", i, c.SymbolID)
		}
		if c.Evidence.TextRank == 0 && c.Evidence.GraphRank == 0 {
			t.Fatalf("candidate[%d] %q: both ranks zero — Evidence not populated", i, c.SymbolID)
		}
	}
	if out.GraphVersion != 99 {
		t.Fatalf("graph_version: got %d, want 99", out.GraphVersion)
	}
}

// ---------- retrieval pending ----------

// TestContextHandler_RetrievalPending_ReturnsStale: when bleve recovery is
// active, the handler returns immediately with RetrievalPending=true,
// Freshness=stale, and Candidates=nil. QueryBleve / PageRank MUST NOT be
// called.
func TestContextHandler_RetrievalPending_ReturnsStale(t *testing.T) {
	retrieval := &mockRetrievalAccessor{
		pendingReturn: true,
	}
	store := &mockStoreAccessorForContext{graphVersion: 7}
	queue := &mockQueueAccessorForContext{}
	s := newSkillForContextTest(t, "read", retrieval, store, queue)

	res := s.handleGetSemanticContext(context.Background(), GetSemanticContextArgs{
		Task: "anything",
	})
	if res.IsError {
		t.Fatalf("expected success (pending → stale envelope), got error: %s", textOf(res))
	}
	var out ContextResult
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, textOf(res))
	}
	if !out.RetrievalPending {
		t.Fatalf("retrieval_pending: got false, want true")
	}
	if out.Freshness != FreshnessStale {
		t.Fatalf("freshness: got %q, want %q", out.Freshness, FreshnessStale)
	}
	if len(out.Candidates) != 0 {
		t.Fatalf("candidates: got %d, want 0 when pending", len(out.Candidates))
	}
	if retrieval.queryCalls.Load() != 0 {
		t.Fatalf("QueryBleve must NOT be called when retrieval pending; got %d calls", retrieval.queryCalls.Load())
	}
	if retrieval.pageRankCalls.Load() != 0 {
		t.Fatalf("PersonalizedPageRank must NOT be called when retrieval pending; got %d calls", retrieval.pageRankCalls.Load())
	}
}

// ---------- token-budget clamping ----------

// TestContextHandler_TokenBudgetClamped_Min: MaxTokens=0 should default to
// 2048 (DefaultContextBudget). The candidates are tiny so all of them pack;
// we just assert the handler succeeds and emits a parseable envelope.
func TestContextHandler_TokenBudgetClamped_Min(t *testing.T) {
	retrieval := &mockRetrievalAccessor{
		textRanksReturn:  []TextRank{{SymbolID: "A"}},
		graphRanksReturn: []GraphRank{{SymbolID: "A"}},
		topEdgesReturn:   nil,
	}
	store := &mockStoreAccessorForContext{}
	queue := &mockQueueAccessorForContext{}
	s := newSkillForContextTest(t, "read", retrieval, store, queue)

	res := s.handleGetSemanticContext(context.Background(), GetSemanticContextArgs{
		Task:      "x",
		MaxTokens: 0, // → default 2048
	})
	if res.IsError {
		t.Fatalf("MaxTokens=0 must default; got error: %s", textOf(res))
	}
}

// TestContextHandler_TokenBudgetClamped_Max: MaxTokens=99999 should clamp to
// 32768 (MaxTokenBudget). Handler succeeds.
func TestContextHandler_TokenBudgetClamped_Max(t *testing.T) {
	retrieval := &mockRetrievalAccessor{
		textRanksReturn:  []TextRank{{SymbolID: "A"}},
		graphRanksReturn: []GraphRank{{SymbolID: "A"}},
	}
	store := &mockStoreAccessorForContext{}
	queue := &mockQueueAccessorForContext{}
	s := newSkillForContextTest(t, "read", retrieval, store, queue)

	res := s.handleGetSemanticContext(context.Background(), GetSemanticContextArgs{
		Task:      "x",
		MaxTokens: 99999, // → clamped to 32768
	})
	if res.IsError {
		t.Fatalf("MaxTokens=99999 must clamp; got error: %s", textOf(res))
	}
}

// ---------- path traversal ----------

// TestContextHandler_PathTraversalRejected: args.Files=["../etc"] returns
// IsError; QueryBleve is NEVER called.
func TestContextHandler_PathTraversalRejected(t *testing.T) {
	retrieval := &mockRetrievalAccessor{}
	store := &mockStoreAccessorForContext{}
	queue := &mockQueueAccessorForContext{}
	s := newSkillForContextTest(t, "read", retrieval, store, queue)

	res := s.handleGetSemanticContext(context.Background(), GetSemanticContextArgs{
		Task:  "x",
		Files: []string{"../etc"},
	})
	if !res.IsError {
		t.Fatalf("expected path-traversal error, got success: %s", textOf(res))
	}
	if !strings.Contains(textOf(res), "path traversal") && !strings.Contains(textOf(res), "..") {
		t.Fatalf("error must mention path traversal, got: %s", textOf(res))
	}
	if retrieval.queryCalls.Load() != 0 {
		t.Fatalf("QueryBleve must NOT be called when path validation fails")
	}
}

// ---------- determinism ----------

// TestContextHandler_Determinism_10Runs: same inputs → byte-identical JSON
// marshal of Candidates across 10 runs. Catches non-deterministic map
// iteration / score-tie ordering bugs (Phase 62 sort-before-iterate +
// CONTEXT.md acceptance test #4 / #8).
func TestContextHandler_Determinism_10Runs(t *testing.T) {
	retrieval := &mockRetrievalAccessor{
		textRanksReturn:  []TextRank{{SymbolID: "alpha"}, {SymbolID: "beta"}, {SymbolID: "gamma"}},
		graphRanksReturn: []GraphRank{{SymbolID: "gamma"}, {SymbolID: "beta"}, {SymbolID: "alpha"}},
		topEdgesReturn:   []string{"e1", "e2", "e3"},
	}
	store := &mockStoreAccessorForContext{graphVersion: 42}
	queue := &mockQueueAccessorForContext{}
	s := newSkillForContextTest(t, "read", retrieval, store, queue)

	args := GetSemanticContextArgs{
		Task:    "deterministic ranking",
		Files:   []string{"a.go", "b.go"},
		Symbols: []string{"sym-1"},
	}

	var first []byte
	for i := 0; i < 10; i++ {
		res := s.handleGetSemanticContext(context.Background(), args)
		if res.IsError {
			t.Fatalf("run %d: error: %s", i, textOf(res))
		}
		var out ContextResult
		if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
			t.Fatalf("run %d: unmarshal: %v", i, err)
		}
		blob, err := json.Marshal(out.Candidates)
		if err != nil {
			t.Fatalf("run %d: marshal candidates: %v", i, err)
		}
		if i == 0 {
			first = blob
			continue
		}
		if string(blob) != string(first) {
			t.Fatalf("non-deterministic candidates: run 0 = %s ; run %d = %s",
				first, i, blob)
		}
	}
}

// ---------- freshness mode default ----------

// TestContextHandler_FreshnessMode_AllowStale_Default: empty FreshnessMode
// defaults to allow_stale.
func TestContextHandler_FreshnessMode_AllowStale_Default(t *testing.T) {
	retrieval := &mockRetrievalAccessor{
		textRanksReturn:  []TextRank{{SymbolID: "A"}},
		graphRanksReturn: []GraphRank{{SymbolID: "A"}},
	}
	store := &mockStoreAccessorForContext{}
	queue := &mockQueueAccessorForContext{}
	s := newSkillForContextTest(t, "read", retrieval, store, queue)

	res := s.handleGetSemanticContext(context.Background(), GetSemanticContextArgs{
		Task: "x",
		// FreshnessMode left empty → default allow_stale.
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	var out ContextResult
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.FreshnessMode != FreshnessModeAllowStale {
		t.Fatalf("freshness_mode default: got %q, want %q", out.FreshnessMode, FreshnessModeAllowStale)
	}
}

// ---------- mode tier read ----------

// TestContextHandler_ModeReadAccepted: session.Mode="read" passes.
func TestContextHandler_ModeReadAccepted(t *testing.T) {
	retrieval := &mockRetrievalAccessor{
		textRanksReturn:  []TextRank{{SymbolID: "A"}},
		graphRanksReturn: []GraphRank{{SymbolID: "A"}},
	}
	store := &mockStoreAccessorForContext{}
	queue := &mockQueueAccessorForContext{}
	s := newSkillForContextTest(t, "read", retrieval, store, queue)

	res := s.handleGetSemanticContext(context.Background(), GetSemanticContextArgs{
		Task: "x",
	})
	if res.IsError {
		t.Fatalf("read mode is the floor for context; got error: %s", textOf(res))
	}
}

// ---------- TopEdges cap ----------

// TestContextHandler_TopEdgesFor_RespectsCap: mock TopEdgesFor returns 10
// edges; handler caps at 5 in Evidence.TopEdges.
func TestContextHandler_TopEdgesFor_RespectsCap(t *testing.T) {
	tooMany := []string{"e1", "e2", "e3", "e4", "e5", "e6", "e7", "e8", "e9", "e10"}
	retrieval := &mockRetrievalAccessor{
		textRanksReturn:  []TextRank{{SymbolID: "A"}},
		graphRanksReturn: []GraphRank{{SymbolID: "A"}},
		topEdgesReturn:   tooMany,
	}
	store := &mockStoreAccessorForContext{}
	queue := &mockQueueAccessorForContext{}
	s := newSkillForContextTest(t, "read", retrieval, store, queue)

	res := s.handleGetSemanticContext(context.Background(), GetSemanticContextArgs{
		Task: "x",
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	var out ContextResult
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Candidates) == 0 {
		t.Fatalf("expected ≥1 candidate")
	}
	for i, c := range out.Candidates {
		if len(c.Evidence.TopEdges) > 5 {
			t.Fatalf("candidate[%d] %q: TopEdges count %d > 5 cap; got=%v",
				i, c.SymbolID, len(c.Evidence.TopEdges), c.Evidence.TopEdges)
		}
	}
}
