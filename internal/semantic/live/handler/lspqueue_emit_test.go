package handler_test

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live/handler"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
)

// CR-04 regression: invariant — every successful UpdateChangedFile
// commit MUST enqueue a Phase 61 LSP revalidation job. Pre-fix, the
// handler had no LSPQueue field; the queue was constructed in the
// daemon and never read. Post-fix, handler.LSPQueue is wired by the
// daemon and EnqueueLane fires after Commit().
//
// Phase 61 P01 Task 3: the queue interface is now lane-aware
// (LSPLaneEnqueuer). The CR-04 entrypoint UpdateChangedFile (3-arg
// scheduler-facing variant) defaults to LaneBackground — this test
// asserts the legacy enqueue path produces a background-lane job.
//
// We exercise the real *lspenrich.LaneQueue (not a mock) to also pin
// the non-blocking-send semantics carried over from lspqueue.Queue.
func TestHandler_UpdateChangedFile_EnqueuesLSPRevalidation(t *testing.T) {
	ctx := context.Background()

	store := &fakeStore{tx: &fakeTx{epoch: 7}}
	q := lspenrich.NewLaneQueue(8, 8)
	h := handler.New(store, goodHasher, nil, nil)
	h.LSPQueue = q

	if err := h.UpdateChangedFile(ctx, semantic.RepoID("/ws"), "/ws/foo.go"); err != nil {
		t.Fatalf("UpdateChangedFile: %v", err)
	}

	// Total depth across both lanes should be 1.
	if got := q.Depth(lspenrich.LaneBackground) + q.Depth(lspenrich.LaneHigh); got != 1 {
		t.Fatalf("CR-04 regression: total depth = %d, want 1 after one successful commit", got)
	}
	// 3-arg UpdateChangedFile defaults to LaneBackground (Phase 61 D-01:
	// scheduler-driven incremental work is not foreground edit traffic).
	if got, want := q.Depth(lspenrich.LaneBackground), 1; got != want {
		t.Fatalf("background-lane depth = %d, want %d (3-arg UpdateChangedFile must default to LaneBackground)", got, want)
	}

	drainCtx, cancel := context.WithTimeout(ctx, 1e9)
	defer cancel()
	job, lane, err := q.Drain(drainCtx)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if lane != lspenrich.LaneBackground {
		t.Errorf("lane: got %q, want %q", lane, lspenrich.LaneBackground)
	}
	if job.RepoID != semantic.RepoID("/ws") {
		t.Errorf("RepoID = %q, want %q", job.RepoID, "/ws")
	}
	if job.Path != "/ws/foo.go" {
		t.Errorf("Path = %q, want %q", job.Path, "/ws/foo.go")
	}
}

// CR-04 corollary: when LSPQueue is nil (test path / unwired daemon),
// UpdateChangedFile MUST still succeed — the queue is best-effort.
func TestHandler_UpdateChangedFile_NilQueue_NoEnqueue(t *testing.T) {
	ctx := context.Background()

	store := &fakeStore{tx: &fakeTx{epoch: 1}}
	h := handler.New(store, goodHasher, nil, nil)
	// h.LSPQueue intentionally nil

	if err := h.UpdateChangedFile(ctx, semantic.RepoID("/ws"), "/ws/foo.go"); err != nil {
		t.Fatalf("UpdateChangedFile with nil LSPQueue: %v", err)
	}
}
