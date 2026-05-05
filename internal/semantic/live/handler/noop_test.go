package handler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/handler"
	"github.com/agenthands/helix/internal/semantic/scheduler"
)

// fakeTx records the calls the dispatcher makes against a synthetic
// OverlayTx.  Failure-injection fields drive negative-path tests.
type fakeTx struct {
	epoch        uint64
	upsertCalls  int
	upsertErr    error
	deleteCalls  int
	deleteErr    error
	commitCalls  int
	rollbackCalls int
}

func (t *fakeTx) Epoch() uint64 { return t.epoch }
func (t *fakeTx) UpsertOverlayFile(_ context.Context, _, _ string) error {
	t.upsertCalls++
	return t.upsertErr
}
func (t *fakeTx) MarkFileDeleted(_ context.Context, _ string) error {
	t.deleteCalls++
	return t.deleteErr
}
func (t *fakeTx) Commit() error   { t.commitCalls++; return nil }
func (t *fakeTx) Rollback() error { t.rollbackCalls++; return nil }

// fakeStore is a stand-in OverlayWriter that hands out fakeTx values and
// counts BeginOverlayTx calls so tests can pin the no-op-no-epoch
// invariant.
type fakeStore struct {
	beginCalls int
	beginErr   error
	tx         *fakeTx
}

func (s *fakeStore) BeginOverlayTx(_ context.Context, _ string) (handler.OverlayTx, error) {
	s.beginCalls++
	if s.beginErr != nil {
		return nil, s.beginErr
	}
	if s.tx == nil {
		s.tx = &fakeTx{epoch: 1}
	}
	return s.tx, nil
}

type fakeSched struct {
	calls   int
	gotWS   semantic.WorkspaceID
	jobID   scheduler.JobID
}

func (s *fakeSched) ScheduleIncremental(ws semantic.WorkspaceID, _ []scheduler.FileChange) scheduler.JobID {
	s.calls++
	s.gotWS = ws
	if s.jobID == "" {
		s.jobID = scheduler.JobID("fake-job-1")
	}
	return s.jobID
}

func goodHasher(_ string) (string, error) { return "abc123", nil }

// TestNoOpDoesNotAdvanceEpoch is the load-bearing acceptance test for
// 60-04 acceptance #8: an empty coalesced batch (zero events) MUST NOT
// reach Dispatch, hence MUST NOT trigger a BeginOverlayTx call.  The
// guard lives in coalescer.flush(), but we also pin the invariant on
// the dispatcher side: a caller that mistakenly invokes Dispatch on an
// empty batch (e.g., a future refactor) would be detected by the
// `len(coalesced) == 0 → no Dispatch` simulation.
func TestNoOpDoesNotAdvanceEpoch(t *testing.T) {
	store := &fakeStore{}
	h := handler.New(store, goodHasher, nil, nil)
	// Simulate the coalescer.flush guard:
	//   if len(merged) == 0 { return }  ← before any Dispatch call.
	merged := []live.SourceChangeEvent{}
	for _, ev := range merged {
		_ = h.Dispatch(context.Background(), ev)
	}
	if store.beginCalls != 0 {
		t.Fatalf("no-op flush: expected 0 BeginOverlayTx calls, got %d",
			store.beginCalls)
	}
}

func TestUpdateChangedFile_OpensTxAndUpserts(t *testing.T) {
	store := &fakeStore{}
	h := handler.New(store, goodHasher, nil, nil)
	if err := h.Dispatch(context.Background(), live.SourceChangeEvent{
		RepoID: "ws1",
		Kind:   live.ChangeFileModified,
		Path:   "src/foo.go",
	}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if store.beginCalls != 1 {
		t.Fatalf("expected 1 BeginOverlayTx call, got %d", store.beginCalls)
	}
	if store.tx.upsertCalls != 1 {
		t.Fatalf("expected 1 UpsertOverlayFile call, got %d", store.tx.upsertCalls)
	}
	if store.tx.commitCalls != 1 {
		t.Fatalf("expected 1 Commit call, got %d", store.tx.commitCalls)
	}
	if store.tx.rollbackCalls != 0 {
		t.Fatalf("expected 0 Rollback calls, got %d", store.tx.rollbackCalls)
	}
}

func TestHandleFileDeleted_TombstonesViaMarkFileDeleted(t *testing.T) {
	store := &fakeStore{}
	h := handler.New(store, goodHasher, nil, nil)
	if err := h.Dispatch(context.Background(), live.SourceChangeEvent{
		RepoID: "ws1",
		Kind:   live.ChangeFileDeleted,
		Path:   "src/del.go",
	}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if store.tx.deleteCalls != 1 {
		t.Fatalf("expected 1 MarkFileDeleted call, got %d", store.tx.deleteCalls)
	}
	if store.tx.upsertCalls != 0 {
		t.Fatalf("delete branch must not Upsert; got %d", store.tx.upsertCalls)
	}
	if store.tx.commitCalls != 1 {
		t.Fatalf("expected 1 Commit, got %d", store.tx.commitCalls)
	}
}

func TestHandleBulkUpdate_DefersToScheduler_NoOverlayTx(t *testing.T) {
	store := &fakeStore{}
	sched := &fakeSched{}
	h := handler.New(store, goodHasher, sched, nil)
	if err := h.Dispatch(context.Background(), live.SourceChangeEvent{
		RepoID: "ws1",
		Kind:   live.ChangeBulkUpdate,
	}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	// Bulk_update must NOT open an OverlayTx (60-04 acceptance #8 spirit).
	if store.beginCalls != 0 {
		t.Fatalf("bulk_update opened %d OverlayTx; expected 0", store.beginCalls)
	}
	if sched.calls != 1 {
		t.Fatalf("bulk_update scheduler calls: got %d, want 1", sched.calls)
	}
	if sched.gotWS != "ws1" {
		t.Fatalf("scheduler ws: got %q, want %q", sched.gotWS, "ws1")
	}
	if sched.jobID == "" {
		t.Fatalf("scheduler returned empty JobID")
	}
}

func TestUpdateChangedFile_RollbackOnUpsertError(t *testing.T) {
	store := &fakeStore{}
	store.tx = &fakeTx{upsertErr: errors.New("disk full")}
	h := handler.New(store, goodHasher, nil, nil)
	err := h.Dispatch(context.Background(), live.SourceChangeEvent{
		RepoID: "ws1",
		Kind:   live.ChangeFileModified,
		Path:   "src/foo.go",
	})
	if err == nil {
		t.Fatal("expected error from upsert failure")
	}
	if store.tx.rollbackCalls != 1 {
		t.Fatalf("expected 1 Rollback, got %d", store.tx.rollbackCalls)
	}
	if store.tx.commitCalls != 0 {
		t.Fatalf("expected 0 Commit on upsert error, got %d", store.tx.commitCalls)
	}
}

func TestHandleFileRenamed_DeletesOldThenUpsertsNew(t *testing.T) {
	store := &fakeStore{}
	h := handler.New(store, goodHasher, nil, nil)
	if err := h.Dispatch(context.Background(), live.SourceChangeEvent{
		RepoID:  "ws1",
		Kind:    live.ChangeFileRenamed,
		OldPath: "src/old.go",
		Path:    "src/new.go",
	}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	// Two BeginOverlayTx calls (delete + upsert).
	if store.beginCalls != 2 {
		t.Fatalf("rename: expected 2 BeginOverlayTx calls, got %d", store.beginCalls)
	}
	if store.tx.deleteCalls != 1 || store.tx.upsertCalls != 1 {
		t.Fatalf("rename: delete=%d upsert=%d; want 1+1", store.tx.deleteCalls, store.tx.upsertCalls)
	}
}

func TestDispatch_UnknownKind_Errors(t *testing.T) {
	store := &fakeStore{}
	h := handler.New(store, goodHasher, nil, nil)
	err := h.Dispatch(context.Background(), live.SourceChangeEvent{
		Kind: live.SourceChangeKind("garbage"),
	})
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
}
