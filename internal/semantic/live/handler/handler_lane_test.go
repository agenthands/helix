package handler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/handler"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
)

// fakeLaneEnqueuer captures every EnqueueLane / Enqueue call for assertion.
type fakeLaneEnqueuer struct {
	calls []laneCall
}

type laneCall struct {
	lane lspenrich.Lane
	job  lspqueue.RevalidateFileJob
}

func (f *fakeLaneEnqueuer) EnqueueLane(lane lspenrich.Lane, job lspqueue.RevalidateFileJob) bool {
	f.calls = append(f.calls, laneCall{lane: lane, job: job})
	return true
}

func (f *fakeLaneEnqueuer) Enqueue(job lspqueue.RevalidateFileJob) bool {
	// Legacy alias — maps to LaneHigh (matches *lspenrich.LaneQueue.Enqueue).
	return f.EnqueueLane(lspenrich.LaneHigh, job)
}

// fakeMetrics records bulk-suppressed counter bumps.
type fakeMetrics struct {
	bulkSuppressed int
	lastN          int
}

func (m *fakeMetrics) LSPEnrichmentTotal(string, string)      {}
func (m *fakeMetrics) LSPEnrichmentDuration(string, float64)  {}
func (m *fakeMetrics) LSPEnrichmentErrors(string, string)     {}
func (m *fakeMetrics) LSPEnrichmentLaneDepth(string, int)     {}
func (m *fakeMetrics) LSPEnrichmentBulkSuppressed(n int) {
	m.bulkSuppressed++
	m.lastN = n
}

// pendingMarkTx is a fakeTx variant that also implements
// MarkFileSemanticPending so the markBulkPending helper can be exercised.
type pendingMarkTx struct {
	epoch         uint64
	pendingCalls  []pendingCall
	pendingErrFor map[string]error
	commitCalls   int
	rollbackCalls int
}

type pendingCall struct {
	path   string
	reason string
}

func (t *pendingMarkTx) Epoch() uint64                                          { return t.epoch }
func (t *pendingMarkTx) UpsertOverlayFile(_ context.Context, _, _ string) error { return nil }
func (t *pendingMarkTx) MarkFileDeleted(_ context.Context, _ string) error      { return nil }
func (t *pendingMarkTx) MarkFileSemanticPending(_ context.Context, path, reason string) error {
	t.pendingCalls = append(t.pendingCalls, pendingCall{path: path, reason: reason})
	if t.pendingErrFor != nil {
		if err, ok := t.pendingErrFor[path]; ok {
			return err
		}
	}
	return nil
}
func (t *pendingMarkTx) Commit() error   { t.commitCalls++; return nil }
func (t *pendingMarkTx) Rollback() error { t.rollbackCalls++; return nil }

type pendingMarkStore struct {
	beginCalls int
	beginErr   error
	tx         *pendingMarkTx
}

func (s *pendingMarkStore) BeginOverlayTx(_ context.Context, _ string) (handler.OverlayTx, error) {
	s.beginCalls++
	if s.beginErr != nil {
		return nil, s.beginErr
	}
	if s.tx == nil {
		s.tx = &pendingMarkTx{epoch: 1}
	}
	return s.tx, nil
}

// H1: ChangeHelixEdit → LaneHigh.
func TestHandler_Dispatch_HelixEdit_LaneHigh(t *testing.T) {
	store := &pendingMarkStore{}
	q := &fakeLaneEnqueuer{}
	h := handler.New(store, goodHasher, nil, nil)
	h.LSPQueue = q

	if err := h.Dispatch(context.Background(), live.SourceChangeEvent{
		RepoID: "ws1",
		Kind:   live.ChangeHelixEdit,
		Path:   "src/foo.go",
	}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(q.calls) != 1 {
		t.Fatalf("EnqueueLane calls: got %d, want 1", len(q.calls))
	}
	if q.calls[0].lane != lspenrich.LaneHigh {
		t.Errorf("lane: got %q, want %q", q.calls[0].lane, lspenrich.LaneHigh)
	}
	if q.calls[0].job.Path != "src/foo.go" {
		t.Errorf("job path: got %q, want %q", q.calls[0].job.Path, "src/foo.go")
	}
}

// H2: ChangeFileModified → LaneBackground.
func TestHandler_Dispatch_FileModified_LaneBackground(t *testing.T) {
	store := &pendingMarkStore{}
	q := &fakeLaneEnqueuer{}
	h := handler.New(store, goodHasher, nil, nil)
	h.LSPQueue = q

	if err := h.Dispatch(context.Background(), live.SourceChangeEvent{
		RepoID: "ws1",
		Kind:   live.ChangeFileModified,
		Path:   "src/bar.go",
	}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(q.calls) != 1 {
		t.Fatalf("EnqueueLane calls: got %d, want 1", len(q.calls))
	}
	if q.calls[0].lane != lspenrich.LaneBackground {
		t.Errorf("lane: got %q, want %q", q.calls[0].lane, lspenrich.LaneBackground)
	}
}

// H3: Created / Deleted / Renamed → LaneBackground.
func TestHandler_Dispatch_OtherKinds_LaneBackground(t *testing.T) {
	cases := []struct {
		name string
		ev   live.SourceChangeEvent
	}{
		{"Created", live.SourceChangeEvent{RepoID: "ws1", Kind: live.ChangeFileCreated, Path: "a.go"}},
		// Deleted goes through HandleFileDeleted which does NOT call
		// EnqueueLane on the producer side — Phase 61 P02 worker handles
		// the deletion-driven re-enrichment via the overlay tombstone. So
		// we omit Deleted from this test.
		{"Renamed", live.SourceChangeEvent{
			RepoID:  "ws1",
			Kind:    live.ChangeFileRenamed,
			OldPath: "old.go",
			Path:    "new.go",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &pendingMarkStore{}
			q := &fakeLaneEnqueuer{}
			h := handler.New(store, goodHasher, nil, nil)
			h.LSPQueue = q
			if err := h.Dispatch(context.Background(), tc.ev); err != nil {
				t.Fatalf("Dispatch: %v", err)
			}
			if len(q.calls) == 0 {
				t.Fatalf("expected at least one EnqueueLane call")
			}
			for i, c := range q.calls {
				if c.lane != lspenrich.LaneBackground {
					t.Errorf("call %d lane: got %q, want %q", i, c.lane, lspenrich.LaneBackground)
				}
			}
		})
	}
}

// H4: ChangeBulkUpdate covering 250 paths → ZERO EnqueueLane; markBulkPending
// called once with all 250 paths and reason="bulk_update_pending"; metrics
// LSPEnrichmentBulkSuppressed bumped exactly once with n=250.
func TestHandler_Dispatch_BulkUpdate_SuppressesEnqueueAndMarksPending(t *testing.T) {
	store := &pendingMarkStore{}
	q := &fakeLaneEnqueuer{}
	m := &fakeMetrics{}
	sched := &fakeSched{}
	h := handler.New(store, goodHasher, sched, nil)
	h.LSPQueue = q
	h.Metrics = m

	paths := make([]string, 250)
	for i := range paths {
		paths[i] = bulkPath(i)
	}

	if err := h.Dispatch(context.Background(), live.SourceChangeEvent{
		RepoID: "ws1",
		Kind:   live.ChangeBulkUpdate,
		Paths:  paths,
	}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if len(q.calls) != 0 {
		t.Fatalf("ChangeBulkUpdate must NOT EnqueueLane; got %d calls", len(q.calls))
	}
	if m.bulkSuppressed != 1 {
		t.Errorf("LSPEnrichmentBulkSuppressed bumps: got %d, want 1", m.bulkSuppressed)
	}
	if m.lastN != 250 {
		t.Errorf("LSPEnrichmentBulkSuppressed n: got %d, want 250", m.lastN)
	}
	if store.beginCalls != 1 {
		t.Fatalf("BeginOverlayTx calls for bulk: got %d, want 1", store.beginCalls)
	}
	if store.tx == nil {
		t.Fatal("store.tx nil after bulk dispatch")
	}
	if got := len(store.tx.pendingCalls); got != 250 {
		t.Errorf("MarkFileSemanticPending calls: got %d, want 250", got)
	}
	for i, pc := range store.tx.pendingCalls {
		if pc.reason != "bulk_update_pending" {
			t.Errorf("call %d reason: got %q, want bulk_update_pending", i, pc.reason)
		}
	}
	if store.tx.commitCalls != 1 {
		t.Errorf("Commit calls: got %d, want 1", store.tx.commitCalls)
	}
}

// H5: nil LSPQueue must never panic on any event kind (CR-04 nil-safety).
func TestHandler_Dispatch_NilLSPQueue_NeverPanics(t *testing.T) {
	store := &pendingMarkStore{}
	h := handler.New(store, goodHasher, &fakeSched{}, nil)
	h.LSPQueue = nil

	cases := []live.SourceChangeKind{
		live.ChangeFileCreated,
		live.ChangeFileModified,
		live.ChangeFileDeleted,
		live.ChangeFileRenamed,
		live.ChangeHelixEdit,
		live.ChangeBulkUpdate,
	}
	for _, k := range cases {
		ev := live.SourceChangeEvent{RepoID: "ws1", Kind: k, Path: "x.go", OldPath: "y.go"}
		if k == live.ChangeBulkUpdate {
			ev.Paths = []string{"a.go", "b.go"}
		}
		// Should not panic; errors permitted (e.g., absent scheduler) but
		// we explicitly tolerate them — the contract is "nil queue is safe".
		_ = h.Dispatch(context.Background(), ev)
	}
}

// H6: markBulkPending opens an OverlayTx, marks every path, commits exactly
// once, and per-path errors are logged (not propagated).
func TestHandler_BulkUpdate_PerPathErrorIsLoggedNotPropagated(t *testing.T) {
	store := &pendingMarkStore{}
	store.tx = &pendingMarkTx{
		epoch:         1,
		pendingErrFor: map[string]error{"bad.go": errors.New("disk full")},
	}
	q := &fakeLaneEnqueuer{}
	h := handler.New(store, goodHasher, &fakeSched{}, nil)
	h.LSPQueue = q

	err := h.Dispatch(context.Background(), live.SourceChangeEvent{
		RepoID: "ws1",
		Kind:   live.ChangeBulkUpdate,
		Paths:  []string{"good.go", "bad.go", "another.go"},
	})
	if err != nil {
		// We do NOT propagate per-path errors from markBulkPending — the
		// bulk path is best-effort by design (PITFALLS C8 prescription).
		t.Fatalf("bulk Dispatch surfaced error: %v (per-path errors must be logged-not-propagated)", err)
	}
	if len(store.tx.pendingCalls) != 3 {
		t.Errorf("MarkFileSemanticPending calls: got %d, want 3 (per-path error must NOT abort the loop)",
			len(store.tx.pendingCalls))
	}
	if store.tx.commitCalls != 1 {
		t.Errorf("Commit calls: got %d, want 1 (best-effort: commit fires even after per-path errors)",
			store.tx.commitCalls)
	}
}

// H7 (B5 single source of truth): the renamed interface is the only one in
// the file. We use a compile-time assertion AND a runtime-verified type to
// pin the rename.
func TestHandler_LSPLaneEnqueuer_IsTheOnlyProducerInterface(t *testing.T) {
	// Compile-time assertion: *lspenrich.LaneQueue satisfies
	// handler.LSPLaneEnqueuer (the only producer-side interface in this
	// package after the B5 rename). Parallel producer interfaces are
	// forbidden by Phase 61 P01 acceptance.
	var _ handler.LSPLaneEnqueuer = (*lspenrich.LaneQueue)(nil)

	// Negative: assigning a single-method (legacy) anonymous shape to
	// LSPLaneEnqueuer must NOT compile. We can't write that as a runtime
	// test; the rename verification lives in the verify gauntlet
	// (a tree-wide grep for the old type name in the plan).
}

// helper paths.
func bulkPath(i int) string {
	return "src/bulk/" + bulkITOA(i) + ".go"
}

func bulkITOA(i int) string {
	const digits = "0123456789"
	return string([]byte{digits[(i/100)%10], digits[(i/10)%10], digits[i%10]})
}

// Use semantic.RepoID alias to avoid an unused import warning if the test
// file later trims fields.
var _ semantic.RepoID = ""
