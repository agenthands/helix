package semantic

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)

// ---------- mocks ----------
//
// These mocks live in tools_refresh_test.go (package-local). The recorder
// flavors below are the load-bearing piece of the D-09/D-13 enforcement:
//   - recorderStoreAccessor implements the production StoreAccessor interface
//     (read-only surface from accessors.go) AND defines extra methods named
//     BeginSnapshot/CommitSnapshot/AbortSnapshot/WriteSnapshotFacts whose
//     bodies call t.Fatal. The production handler can ONLY reach the read
//     surface through the StoreAccessor interface (Go compile-time guarantee),
//     so any future regression that adds a snapshot-write call would have to
//     either (a) extend the StoreAccessor interface in accessors.go (visible
//     in the diff) or (b) type-assert past the interface to reach the real
//     *Store (also visible in the diff). The recorder methods serve as
//     belt-and-braces canary: if a future change ever uses a richer interface
//     and reaches these methods, the test FAILS at the call site.
//   - recorderCompactorAccessor implements CompactorAccessor; OnFlush calls
//     t.Fatal. The production handler must NEVER call OnFlush (D-13).
//
// Both recorders also count invocations so tests can assert non-call.

// recorderStoreAccessor is a recorder-style mock StoreAccessor. The methods
// declared in the interface return injected values; the extra forbidden
// methods (BeginSnapshot et al.) are canary methods whose bodies call t.Fatal
// if ever invoked.
type recorderStoreAccessor struct {
	t *testing.T

	// Injection points for the read surface.
	graphVersion      uint64
	graphVersionErr   error
	overlayHasPending bool
	latestSnapshot    uint64
	latestSnapshotErr error
	queryAdjOut       map[graph.NodeID]map[graph.NodeID]float64
	queryAdjIn        map[graph.NodeID]map[graph.NodeID]float64
	queryAdjErr       error

	// Forbidden-call counters (assertion canaries).
	beginSnapshotCalls      atomic.Int64
	commitSnapshotCalls     atomic.Int64
	abortSnapshotCalls      atomic.Int64
	writeSnapshotFactsCalls atomic.Int64
}

func (r *recorderStoreAccessor) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	return r.latestSnapshot, r.latestSnapshotErr
}
func (r *recorderStoreAccessor) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return r.graphVersion, r.graphVersionErr
}
func (r *recorderStoreAccessor) OverlayHasPendingRows(repoID string) bool {
	return r.overlayHasPending
}
func (r *recorderStoreAccessor) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
) {
	return r.queryAdjOut, r.queryAdjIn, r.queryAdjErr
}

// Phase 70-04 seam stubs — tools_refresh_test does not yet exercise the
// incremental drain path; return cold-start zero values. Plan 70-05 will
// extend these with injection points + counters when it wires the refresh
// tool to FlushNow + OverlayChangedPathsSince.
func (r *recorderStoreAccessor) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
	return 0, nil
}
func (r *recorderStoreAccessor) OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) ([]string, uint64, error) {
	return nil, 0, nil
}
func (r *recorderStoreAccessor) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (uint64, bool, error) {
	return 0, false, nil
}

// Forbidden methods — D-09 / D-13 invariants. These methods are NOT part of
// the StoreAccessor interface; they only exist on this recorder type so we
// can fail loudly if a future regression ever reaches them via type assertion.
func (r *recorderStoreAccessor) BeginSnapshot(ctx context.Context, repoID string) (uint64, error) {
	r.beginSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: refresh handler must NOT call BeginSnapshot")
	return 0, nil
}
func (r *recorderStoreAccessor) CommitSnapshot(ctx context.Context, snapID uint64) error {
	r.commitSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: refresh handler must NOT call CommitSnapshot")
	return nil
}
func (r *recorderStoreAccessor) AbortSnapshot(ctx context.Context, snapID uint64) error {
	r.abortSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: refresh handler must NOT call AbortSnapshot")
	return nil
}
func (r *recorderStoreAccessor) WriteSnapshotFacts(ctx context.Context, snapID uint64, facts any) error {
	r.writeSnapshotFactsCalls.Add(1)
	r.t.Fatalf("D-09 violation: refresh handler must NOT call WriteSnapshotFacts")
	return nil
}

// recorderCompactorAccessor implements CompactorAccessor. OnFlush calls
// t.Fatal — the refresh handler must NEVER trigger compaction (D-13).
type recorderCompactorAccessor struct {
	t     *testing.T
	calls atomic.Int64
}

func (r *recorderCompactorAccessor) OnFlush(ws workspace.WorkspaceKey) error {
	r.calls.Add(1)
	r.t.Fatalf("D-13 violation: refresh handler must NOT call CompactorAccessor.OnFlush")
	return nil
}

// recordedLiveCall captures one OnWorkspaceChanged invocation.
type recordedLiveCall struct {
	ws    workspace.WorkspaceKey
	paths []string
}

// mockLiveAccessor implements LiveAccessor with recording.
type mockLiveAccessor struct {
	onWorkspaceChangedErr error
	lastFlushAt           int64

	mu    sync.Mutex
	calls []recordedLiveCall
}

func (m *mockLiveAccessor) OnWorkspaceChanged(ws workspace.WorkspaceKey, paths []string) error {
	m.mu.Lock()
	// Copy paths to avoid aliasing; assertion checks should observe a stable
	// snapshot regardless of subsequent caller mutations.
	cp := append([]string(nil), paths...)
	m.calls = append(m.calls, recordedLiveCall{ws: ws, paths: cp})
	m.mu.Unlock()
	return m.onWorkspaceChangedErr
}
func (m *mockLiveAccessor) LastFlushAt(ws workspace.WorkspaceKey) int64 {
	return m.lastFlushAt
}
func (m *mockLiveAccessor) FlushNow(_ context.Context, _ workspace.WorkspaceKey) error { return nil }
func (m *mockLiveAccessor) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}
func (m *mockLiveAccessor) firstCall() recordedLiveCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return recordedLiveCall{}
	}
	return m.calls[0]
}

// mockQueueAccessor implements QueueAccessor with a depth function so tests
// can model "drains over time" behavior.
type mockQueueAccessor struct {
	depthFn       func() int
	lastEnqueueAt int64
	depthCalls    atomic.Int64
}

func (m *mockQueueAccessor) DepthAll(ws workspace.WorkspaceKey) int {
	m.depthCalls.Add(1)
	if m.depthFn == nil {
		return 0
	}
	return m.depthFn()
}
func (m *mockQueueAccessor) LastEnqueueAt(ws workspace.WorkspaceKey) int64 {
	return m.lastEnqueueAt
}

// constDepth returns a depthFn that always returns the given depth.
func constDepth(n int) func() int { return func() int { return n } }

// drainAfter returns a depthFn that returns initialDepth until duration has
// elapsed since startedAt, then 0.
func drainAfter(initialDepth int, after time.Duration) func() int {
	startedAt := time.Now()
	return func() int {
		if time.Since(startedAt) >= after {
			return 0
		}
		return initialDepth
	}
}

// newSkillForRefreshTest constructs a SemanticSkill with the given session
// mode + mocks wired up. Returns the skill, the recorder mocks (so tests can
// assert non-invocation), the live mock, and the queue mock.
func newSkillForRefreshTest(
	t *testing.T,
	mode string,
	live *mockLiveAccessor,
	queue *mockQueueAccessor,
	store *recorderStoreAccessor,
	compactor *recorderCompactorAccessor,
) *SemanticSkill {
	t.Helper()
	s := &SemanticSkill{}
	if err := s.Init(skill.SkillDeps{}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	sess := &mcp.SessionInfo{
		SessionID: "test-refresh-session",
		Mode:      mode,
	}
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/repo-refresh-test", Language: "go", Toolchain: "go1.22"}
	s.SetSessionAccessor(&mockSessionAccessor{ws: ws, sess: sess})
	s.SetLive(live)
	s.SetQueue(queue)
	s.SetStore(store)
	s.SetCompactor(compactor)
	return s
}

// ---------- happy path ----------

func TestRefreshHandler_HappyPath_DrainsLive(t *testing.T) {
	store := &recorderStoreAccessor{t: t, graphVersion: 42}
	live := &mockLiveAccessor{}
	queue := &mockQueueAccessor{depthFn: constDepth(0)}
	compactor := &recorderCompactorAccessor{t: t}
	s := newSkillForRefreshTest(t, "read", live, queue, store, compactor)

	res := s.handleRefreshSemanticGraph(context.Background(), RefreshSemanticGraphArgs{
		Paths: []string{"a.go", "b.go", "c.go"},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	var out RefreshResult
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, textOf(res))
	}
	if out.GraphVersion != 42 {
		t.Fatalf("graph_version: got %d, want 42", out.GraphVersion)
	}
	if out.FilesUpdated != 3 {
		t.Fatalf("files_updated: got %d, want 3", out.FilesUpdated)
	}
	if out.PendingLSP {
		t.Fatalf("pending_lsp: got true, want false (queue drained)")
	}
	if out.Freshness != FreshnessFresh {
		t.Fatalf("freshness: got %q, want %q", out.Freshness, FreshnessFresh)
	}
	if live.callCount() != 1 {
		t.Fatalf("live.OnWorkspaceChanged: got %d calls, want 1", live.callCount())
	}
}

// ---------- D-09 invariant: refresh never commits a snapshot ----------

func TestRefreshHandler_NoSnapshotCommit_Invariant(t *testing.T) {
	store := &recorderStoreAccessor{t: t, graphVersion: 100}
	live := &mockLiveAccessor{}
	queue := &mockQueueAccessor{depthFn: constDepth(0)}
	compactor := &recorderCompactorAccessor{t: t}
	s := newSkillForRefreshTest(t, "read", live, queue, store, compactor)

	res := s.handleRefreshSemanticGraph(context.Background(), RefreshSemanticGraphArgs{
		Paths: []string{"x.go"},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	// Belt-and-braces: even though the StoreAccessor interface itself doesn't
	// expose snapshot-write methods, assert the recorder counters all sit at
	// zero. If any future change reaches these via type assertion, t.Fatal
	// inside the recorder body would already have fired; this is a final
	// defensive check.
	if store.beginSnapshotCalls.Load() != 0 ||
		store.commitSnapshotCalls.Load() != 0 ||
		store.abortSnapshotCalls.Load() != 0 ||
		store.writeSnapshotFactsCalls.Load() != 0 {
		t.Fatalf("D-09 violation: recorder observed snapshot-write calls: begin=%d commit=%d abort=%d writeFacts=%d",
			store.beginSnapshotCalls.Load(),
			store.commitSnapshotCalls.Load(),
			store.abortSnapshotCalls.Load(),
			store.writeSnapshotFactsCalls.Load())
	}
}

// ---------- D-13 invariant: refresh never triggers compaction ----------

func TestRefreshHandler_NoCompactorCall_Invariant(t *testing.T) {
	store := &recorderStoreAccessor{t: t, graphVersion: 7}
	live := &mockLiveAccessor{}
	queue := &mockQueueAccessor{depthFn: constDepth(0)}
	compactor := &recorderCompactorAccessor{t: t}
	s := newSkillForRefreshTest(t, "read", live, queue, store, compactor)

	res := s.handleRefreshSemanticGraph(context.Background(), RefreshSemanticGraphArgs{
		Paths: []string{"y.go"},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	if compactor.calls.Load() != 0 {
		t.Fatalf("D-13 violation: CompactorAccessor.OnFlush invoked %d times (must be 0)", compactor.calls.Load())
	}
}

// ---------- D-11: paths filter is strict subset ----------

func TestRefreshHandler_PathsStrictSubset(t *testing.T) {
	store := &recorderStoreAccessor{t: t, graphVersion: 1}
	live := &mockLiveAccessor{}
	queue := &mockQueueAccessor{depthFn: constDepth(0)}
	compactor := &recorderCompactorAccessor{t: t}
	s := newSkillForRefreshTest(t, "read", live, queue, store, compactor)

	res := s.handleRefreshSemanticGraph(context.Background(), RefreshSemanticGraphArgs{
		Paths: []string{"a.go"},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	if live.callCount() != 1 {
		t.Fatalf("live.OnWorkspaceChanged: got %d calls, want exactly 1", live.callCount())
	}
	gotPaths := live.firstCall().paths
	if len(gotPaths) != 1 || gotPaths[0] != "a.go" {
		t.Fatalf("strict-subset violation: live received %v, expected [a.go]", gotPaths)
	}
}

// ---------- D-12: wait_for_lsp blocks with cap ----------

func TestRefreshHandler_WaitForLSP_TimeoutReportsPending(t *testing.T) {
	store := &recorderStoreAccessor{t: t, graphVersion: 5}
	live := &mockLiveAccessor{}
	// Queue NEVER drains — depth stuck at 5.
	queue := &mockQueueAccessor{depthFn: constDepth(5)}
	compactor := &recorderCompactorAccessor{t: t}
	s := newSkillForRefreshTest(t, "read", live, queue, store, compactor)

	start := time.Now()
	res := s.handleRefreshSemanticGraph(context.Background(), RefreshSemanticGraphArgs{
		Paths:      []string{"file.go"},
		WaitForLSP: true,
		MaxWaitMs:  200,
	})
	elapsed := time.Since(start)

	if res.IsError {
		t.Fatalf("expected success (timeout reports pending, not error), got error: %s", textOf(res))
	}
	var out RefreshResult
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, textOf(res))
	}
	if !out.PendingLSP {
		t.Fatalf("pending_lsp: got false, want true on timeout")
	}
	if out.PendingLSPFiles != 5 {
		t.Fatalf("pending_lsp_files: got %d, want 5", out.PendingLSPFiles)
	}
	if out.Freshness != FreshnessStructurallyFreshSemanticallyPending {
		t.Fatalf("freshness: got %q, want %q", out.Freshness, FreshnessStructurallyFreshSemanticallyPending)
	}
	// Allow generous slack for CI scheduling jitter; the bound is
	// 200ms +/- 100ms (i.e., 100..300ms). A handler that ignores
	// MaxWaitMs would either return immediately (~0ms) or block on
	// the default 3000ms cap.
	if elapsed < 100*time.Millisecond || elapsed > 350*time.Millisecond {
		t.Fatalf("elapsed: got %v, want ~200ms (100-350ms)", elapsed)
	}
}

func TestRefreshHandler_WaitForLSP_DrainsBeforeTimeout(t *testing.T) {
	store := &recorderStoreAccessor{t: t, graphVersion: 9}
	live := &mockLiveAccessor{}
	// Drain after 50ms.
	queue := &mockQueueAccessor{depthFn: drainAfter(3, 50*time.Millisecond)}
	compactor := &recorderCompactorAccessor{t: t}
	s := newSkillForRefreshTest(t, "read", live, queue, store, compactor)

	start := time.Now()
	res := s.handleRefreshSemanticGraph(context.Background(), RefreshSemanticGraphArgs{
		Paths:      []string{"f.go"},
		WaitForLSP: true,
		MaxWaitMs:  2000,
	})
	elapsed := time.Since(start)

	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	var out RefreshResult
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, textOf(res))
	}
	if out.PendingLSP {
		t.Fatalf("pending_lsp: got true, want false (queue drained before timeout)")
	}
	if out.Freshness != FreshnessFresh {
		t.Fatalf("freshness: got %q, want %q", out.Freshness, FreshnessFresh)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("elapsed: got %v; handler should have detected drain ~50ms after start", elapsed)
	}
}

// ---------- T-64-05-01: path traversal rejected ----------

func TestRefreshHandler_PathTraversalRejected(t *testing.T) {
	store := &recorderStoreAccessor{t: t}
	live := &mockLiveAccessor{}
	queue := &mockQueueAccessor{depthFn: constDepth(0)}
	compactor := &recorderCompactorAccessor{t: t}
	s := newSkillForRefreshTest(t, "read", live, queue, store, compactor)

	res := s.handleRefreshSemanticGraph(context.Background(), RefreshSemanticGraphArgs{
		Paths: []string{"../../etc"},
	})
	if !res.IsError {
		t.Fatalf("expected path-traversal error, got success: %s", textOf(res))
	}
	if !strings.Contains(textOf(res), "path traversal") {
		t.Fatalf("error must mention 'path traversal', got: %s", textOf(res))
	}
	if live.callCount() != 0 {
		t.Fatalf("live.OnWorkspaceChanged must NOT be called when path validation fails")
	}
}

// ---------- T-64-05-02: read tier accepts ----------

func TestRefreshHandler_ModeReadAccepted(t *testing.T) {
	store := &recorderStoreAccessor{t: t, graphVersion: 1}
	live := &mockLiveAccessor{}
	queue := &mockQueueAccessor{depthFn: constDepth(0)}
	compactor := &recorderCompactorAccessor{t: t}
	s := newSkillForRefreshTest(t, "read", live, queue, store, compactor)

	res := s.handleRefreshSemanticGraph(context.Background(), RefreshSemanticGraphArgs{})
	if res.IsError {
		t.Fatalf("read mode is the floor for refresh; got error: %s", textOf(res))
	}
}
