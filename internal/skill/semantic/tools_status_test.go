package semantic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)

// ---------- mocks ----------
//
// The status handler reads from StoreAccessor / SchedulerAccessor /
// QueueAccessor / LiveAccessor / RetrievalAccessor. Each mock below is a
// minimal injection stub (no recorder canaries needed — the status handler
// is a pure read path; there's no forbidden surface to assert non-call on).
//
// recorderStoreAccessor + mockLiveAccessor + mockQueueAccessor + mockSessionAccessor
// are reused FROM tools_refresh_test.go / tools_index_test.go — Go test
// package files share the package, so we just consume them here.

// statusMockScheduler implements SchedulerAccessor for status tests.
//
// Production behavior reminder: until Phase 65/67 wires a real cluster-status
// source, the daemon adapter returns
// ClusterStatus{State:"unknown", Reason:"phase-62-clustering-no-status-accessor"}.
// The default zero-value clusterStatus on this mock mirrors that contract so
// tests reflect what agents will actually see in production (W1 closure).
type statusMockScheduler struct {
	isQuiescent   bool
	scoreStatuses map[string]graph.ScoreStatus // projection -> status
	clusterStatus ClusterStatus
}

func (m *statusMockScheduler) IsQuiescent(repoID string) bool { return m.isQuiescent }

func (m *statusMockScheduler) ScoreStatus(repoID, projection string) graph.ScoreStatus {
	if m.scoreStatuses == nil {
		return graph.ScoreStatusMissing
	}
	if s, ok := m.scoreStatuses[projection]; ok {
		return s
	}
	return graph.ScoreStatusMissing
}

func (m *statusMockScheduler) ClusterStatus(repoID string) ClusterStatus {
	return m.clusterStatus
}

// statusMockRetrieval implements RetrievalAccessor for the retrieval_pending
// path. Other RetrievalAccessor methods return zero values; the status
// handler only consumes RetrievalPending(ws).
type statusMockRetrieval struct {
	pending bool
}

func (m *statusMockRetrieval) QueryBleve(task string, anchors []string) ([]TextRank, error) {
	return nil, nil
}
func (m *statusMockRetrieval) PersonalizedPageRank(ctx context.Context, repoID string, anchors []string) ([]GraphRank, error) {
	return nil, nil
}
func (m *statusMockRetrieval) RetrievalPending(ws workspace.WorkspaceKey) bool { return m.pending }
func (m *statusMockRetrieval) TopEdgesFor(ctx context.Context, repoID, symbolID string) ([]string, error) {
	return nil, nil
}

// RetrievalStatus is a zero-value stub for Phase 69-04 RED/GREEN. Plan 69-06
// will replace this with status-bearing test fixtures once the retrieval
// status surface lands in tools_status.go.
func (m *statusMockRetrieval) RetrievalStatus(ws workspace.WorkspaceKey) RetrievalStatus {
	return RetrievalStatus{}
}

// productionDefaultClusterStatus mirrors what the real daemon adapter
// returns until Phase 65/67 wires a live cluster source (W1 closure).
func productionDefaultClusterStatus() ClusterStatus {
	return ClusterStatus{
		State:  "unknown",
		Reason: "phase-62-clustering-no-status-accessor",
	}
}

// newSkillForStatusTest constructs a SemanticSkill with the given session mode
// + status mocks wired up. All mocks are optional — pass nil to omit a seam.
func newSkillForStatusTest(
	t *testing.T,
	mode string,
	store *recorderStoreAccessor,
	scheduler *statusMockScheduler,
	queue *mockQueueAccessor,
	live *mockLiveAccessor,
	retrieval *statusMockRetrieval,
) *SemanticSkill {
	t.Helper()
	s := &SemanticSkill{}
	if err := s.Init(skill.SkillDeps{}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	sess := &mcp.SessionInfo{
		SessionID: "test-status-session",
		Mode:      mode,
	}
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/repo-status-test", Language: "go", Toolchain: "go1.22"}
	s.SetSessionAccessor(&mockSessionAccessor{ws: ws, sess: sess})
	if store != nil {
		s.SetStore(store)
	}
	if scheduler != nil {
		s.SetScheduler(scheduler)
	}
	if queue != nil {
		s.SetQueue(queue)
	}
	if live != nil {
		s.SetLive(live)
	}
	if retrieval != nil {
		s.SetRetrieval(retrieval)
	}
	return s
}

// ---------- Test 1: empty store ----------

// TestStatusHandler_EmptyStore: all accessors return zero values; the response
// envelope reflects a fully-fresh empty state. cluster_status defaults to the
// production-adapter shape (state="unknown", reason="phase-62-..."), proving
// W1 is closed at the response-shape level.
func TestStatusHandler_EmptyStore(t *testing.T) {
	store := &recorderStoreAccessor{t: t} // all zero-valued
	scheduler := &statusMockScheduler{
		// Production default for cluster_status (W1 closure):
		clusterStatus: productionDefaultClusterStatus(),
		// scoreStatuses nil → ScoreStatus returns "missing" for every projection.
	}
	queue := &mockQueueAccessor{depthFn: constDepth(0)}
	live := &mockLiveAccessor{lastFlushAt: 0}
	retrieval := &statusMockRetrieval{pending: false}

	s := newSkillForStatusTest(t, "read", store, scheduler, queue, live, retrieval)

	res := s.handleGetSemanticGraphStatus(context.Background(), GetSemanticGraphStatusArgs{})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	var out StatusResult
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, textOf(res))
	}

	if out.LatestSnapshotID != 0 {
		t.Errorf("latest_snapshot_id: got %d, want 0", out.LatestSnapshotID)
	}
	if out.GraphVersion != 0 {
		t.Errorf("graph_version: got %d, want 0", out.GraphVersion)
	}
	if out.OverlayActive {
		t.Errorf("overlay_active: got true, want false")
	}
	if out.PendingLSPFiles != 0 {
		t.Errorf("pending_lsp_files: got %d, want 0", out.PendingLSPFiles)
	}
	if out.LastLiveUpdateMs != 0 {
		t.Errorf("last_live_update_ms: got %d, want 0", out.LastLiveUpdateMs)
	}
	if out.RetrievalPending {
		t.Errorf("retrieval_pending: got true, want false")
	}
	if out.Freshness != FreshnessFresh {
		t.Errorf("freshness: got %q, want %q", out.Freshness, FreshnessFresh)
	}

	// score_status: 3 projections, all "missing".
	if len(out.ScoreStatus) != 3 {
		t.Fatalf("score_status: got %d entries, want 3 (CALL_GRAPH/REFERENCE/FILE_DEPENDENCY)", len(out.ScoreStatus))
	}
	for _, projection := range []string{"CALL_GRAPH_PAGERANK", "REFERENCE_PAGERANK", "FILE_DEPENDENCY_PAGERANK"} {
		if got := out.ScoreStatus[projection]; got != string(graph.ScoreStatusMissing) {
			t.Errorf("score_status[%q]: got %q, want %q", projection, got, graph.ScoreStatusMissing)
		}
	}

	// cluster_status: production-adapter default (W1).
	if out.ClusterStatus.State != "unknown" {
		t.Errorf("cluster_status.state: got %q, want %q", out.ClusterStatus.State, "unknown")
	}
	if out.ClusterStatus.Reason != "phase-62-clustering-no-status-accessor" {
		t.Errorf("cluster_status.reason: got %q, want %q",
			out.ClusterStatus.Reason, "phase-62-clustering-no-status-accessor")
	}
}

// ---------- Test 2: populated store ----------

// TestStatusHandler_PopulatedStore: store has a committed snapshot, overlay
// has pending rows, LSP queue is non-empty, score_status varies per projection.
// Asserts every field in the envelope reflects the supplied state, and
// freshness resolves to "structurally_fresh_semantically_pending" because
// overlay+pending-LSP simultaneously triggers that case.
func TestStatusHandler_PopulatedStore(t *testing.T) {
	store := &recorderStoreAccessor{
		t:                 t,
		latestSnapshot:    42,
		graphVersion:      99,
		overlayHasPending: true,
	}
	scheduler := &statusMockScheduler{
		scoreStatuses: map[string]graph.ScoreStatus{
			"CALL_GRAPH_PAGERANK":      graph.ScoreStatusExact,
			"REFERENCE_PAGERANK":       graph.ScoreStatusStale,
			"FILE_DEPENDENCY_PAGERANK": graph.ScoreStatusApproximate,
		},
		clusterStatus: productionDefaultClusterStatus(),
	}
	queue := &mockQueueAccessor{depthFn: constDepth(5)}
	live := &mockLiveAccessor{lastFlushAt: 1700000000000}
	retrieval := &statusMockRetrieval{pending: false}

	s := newSkillForStatusTest(t, "read", store, scheduler, queue, live, retrieval)

	res := s.handleGetSemanticGraphStatus(context.Background(), GetSemanticGraphStatusArgs{})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	var out StatusResult
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, textOf(res))
	}

	if out.LatestSnapshotID != 42 {
		t.Errorf("latest_snapshot_id: got %d, want 42", out.LatestSnapshotID)
	}
	if out.GraphVersion != 99 {
		t.Errorf("graph_version: got %d, want 99", out.GraphVersion)
	}
	if !out.OverlayActive {
		t.Errorf("overlay_active: got false, want true")
	}
	if out.PendingLSPFiles != 5 {
		t.Errorf("pending_lsp_files: got %d, want 5", out.PendingLSPFiles)
	}
	if out.LastLiveUpdateMs != 1700000000000 {
		t.Errorf("last_live_update_ms: got %d, want 1700000000000", out.LastLiveUpdateMs)
	}
	if out.RetrievalPending {
		t.Errorf("retrieval_pending: got true, want false")
	}
	// overlay + pendingLSP > 0 → structurally_fresh_semantically_pending.
	if out.Freshness != FreshnessStructurallyFreshSemanticallyPending {
		t.Errorf("freshness: got %q, want %q",
			out.Freshness, FreshnessStructurallyFreshSemanticallyPending)
	}

	// score_status varies per projection.
	wants := map[string]string{
		"CALL_GRAPH_PAGERANK":      string(graph.ScoreStatusExact),
		"REFERENCE_PAGERANK":       string(graph.ScoreStatusStale),
		"FILE_DEPENDENCY_PAGERANK": string(graph.ScoreStatusApproximate),
	}
	for projection, want := range wants {
		if got := out.ScoreStatus[projection]; got != want {
			t.Errorf("score_status[%q]: got %q, want %q", projection, got, want)
		}
	}

	// cluster_status preserved.
	if out.ClusterStatus.State != "unknown" {
		t.Errorf("cluster_status.state: got %q, want %q", out.ClusterStatus.State, "unknown")
	}
}

// ---------- Test 3: cluster_status default-unknown JSON shape (W1 closure) ----------

// TestStatusHandler_ClusterStatus_DefaultUnknown_ReasonPopulated proves W1 is
// closed at the wire-format level: cluster_status marshals to
//
//	{"state":"unknown","reason":"phase-62-clustering-no-status-accessor"}
//
// Until Phase 65/67 wires a live cluster source, this is the response shape
// every agent will see. The structured Reason field gives observability the
// gap-tracking signal it needs.
func TestStatusHandler_ClusterStatus_DefaultUnknown_ReasonPopulated(t *testing.T) {
	store := &recorderStoreAccessor{t: t}
	scheduler := &statusMockScheduler{
		clusterStatus: productionDefaultClusterStatus(),
	}
	queue := &mockQueueAccessor{depthFn: constDepth(0)}
	live := &mockLiveAccessor{}
	retrieval := &statusMockRetrieval{}

	s := newSkillForStatusTest(t, "read", store, scheduler, queue, live, retrieval)

	res := s.handleGetSemanticGraphStatus(context.Background(), GetSemanticGraphStatusArgs{})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}

	// Assert in-memory struct shape.
	var out StatusResult
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, textOf(res))
	}
	if out.ClusterStatus.State != "unknown" {
		t.Errorf("cluster_status.state: got %q, want %q", out.ClusterStatus.State, "unknown")
	}
	if out.ClusterStatus.Reason != "phase-62-clustering-no-status-accessor" {
		t.Errorf("cluster_status.reason: got %q, want %q",
			out.ClusterStatus.Reason, "phase-62-clustering-no-status-accessor")
	}

	// Assert wire-format shape: marshal the ClusterStatus alone and verify the
	// canonical JSON form. Doing this on the field (not the whole envelope)
	// keeps the assertion focused on the W1-closure-relevant bytes.
	b, err := json.Marshal(out.ClusterStatus)
	if err != nil {
		t.Fatalf("marshal cluster_status: %v", err)
	}
	wantJSON := `{"state":"unknown","reason":"phase-62-clustering-no-status-accessor"}`
	if string(b) != wantJSON {
		t.Errorf("cluster_status JSON: got %s, want %s", string(b), wantJSON)
	}
}

// ---------- Test 4: freshness closed-enum table ----------

// TestStatusHandler_FreshnessEnum_AllValuesValid drives a small table of
// (overlayActive, pendingLSP, retrievalPending) tuples and asserts the
// freshness field matches the closed enum (SPEC §26.2). The test pins the
// priority ordering specified in the handler's switch so a future regression
// that swaps two cases is caught.
func TestStatusHandler_FreshnessEnum_AllValuesValid(t *testing.T) {
	cases := []struct {
		name             string
		overlayActive    bool
		pendingLSPFiles  int
		retrievalPending bool
		wantFreshness    Freshness
	}{
		// Highest priority: retrievalPending overrides everything else.
		{"retrieval-rebuilding-overrides", true, 5, true, FreshnessStale},
		{"retrieval-rebuilding-on-clean", false, 0, true, FreshnessStale},

		// overlay + pending LSP -> structurally_fresh_semantically_pending.
		{"overlay-and-pending-lsp", true, 3, false, FreshnessStructurallyFreshSemanticallyPending},

		// overlay only (no pending LSP) -> overlay_active.
		{"overlay-no-lsp", true, 0, false, FreshnessOverlayActive},

		// no overlay, no LSP, no retrieval rebuild -> fresh.
		{"clean-state", false, 0, false, FreshnessFresh},

		// pendingLSP without overlay falls through to fresh — pending LSP
		// alone (without an overlay-active signal) is not a freshness gate
		// per SPEC §26.2.
		{"pending-lsp-no-overlay", false, 5, false, FreshnessFresh},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &recorderStoreAccessor{
				t:                 t,
				overlayHasPending: tc.overlayActive,
			}
			scheduler := &statusMockScheduler{
				clusterStatus: productionDefaultClusterStatus(),
			}
			queue := &mockQueueAccessor{depthFn: constDepth(tc.pendingLSPFiles)}
			live := &mockLiveAccessor{}
			retrieval := &statusMockRetrieval{pending: tc.retrievalPending}

			s := newSkillForStatusTest(t, "read", store, scheduler, queue, live, retrieval)

			res := s.handleGetSemanticGraphStatus(context.Background(), GetSemanticGraphStatusArgs{})
			if res.IsError {
				t.Fatalf("expected success, got error: %s", textOf(res))
			}
			var out StatusResult
			if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
				t.Fatalf("unmarshal: %v (raw=%s)", err, textOf(res))
			}
			if out.Freshness != tc.wantFreshness {
				t.Errorf("freshness: got %q, want %q (overlay=%v pendingLSP=%d retrievalPending=%v)",
					out.Freshness, tc.wantFreshness,
					tc.overlayActive, tc.pendingLSPFiles, tc.retrievalPending)
			}

			// Closed-enum guard: regardless of inputs, Freshness MUST be one
			// of the four values declared in envelope.go.
			switch out.Freshness {
			case FreshnessFresh, FreshnessStale,
				FreshnessStructurallyFreshSemanticallyPending,
				FreshnessOverlayActive:
				// OK.
			default:
				t.Errorf("freshness %q is not a member of the closed enum", out.Freshness)
			}
		})
	}
}

// ---------- Test 5: retrieval_pending => freshness=stale ----------

// TestStatusHandler_RetrievalPending_FreshnessStale: retrieval engine is
// rebuilding (e.g., bleve recovery after daemon restart). Response carries
// retrieval_pending=true and freshness=stale. This case has the highest
// priority in the freshness switch — it overrides overlay/pending-LSP signals
// because the retrieval index physically cannot serve fresh results until the
// rebuild completes.
func TestStatusHandler_RetrievalPending_FreshnessStale(t *testing.T) {
	store := &recorderStoreAccessor{
		t:                 t,
		latestSnapshot:    100,
		graphVersion:      200,
		overlayHasPending: true, // Even with overlay, retrievalPending wins.
	}
	scheduler := &statusMockScheduler{
		scoreStatuses: map[string]graph.ScoreStatus{
			"CALL_GRAPH_PAGERANK": graph.ScoreStatusExact,
		},
		clusterStatus: productionDefaultClusterStatus(),
	}
	queue := &mockQueueAccessor{depthFn: constDepth(2)} // Even with pending LSP.
	live := &mockLiveAccessor{}
	retrieval := &statusMockRetrieval{pending: true}

	s := newSkillForStatusTest(t, "read", store, scheduler, queue, live, retrieval)

	res := s.handleGetSemanticGraphStatus(context.Background(), GetSemanticGraphStatusArgs{})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	var out StatusResult
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, textOf(res))
	}

	if !out.RetrievalPending {
		t.Errorf("retrieval_pending: got false, want true")
	}
	if out.Freshness != FreshnessStale {
		t.Errorf("freshness: got %q, want %q (retrievalPending overrides overlay/pendingLSP)",
			out.Freshness, FreshnessStale)
	}
}
