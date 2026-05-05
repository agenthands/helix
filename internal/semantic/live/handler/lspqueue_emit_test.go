package handler_test

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live/handler"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
)

// CR-04 regression: invariant — every successful UpdateChangedFile
// commit MUST enqueue a Phase 61 LSP revalidation job. Pre-fix, the
// handler had no LSPQueue field; the queue was constructed in the
// daemon and never read. Post-fix, handler.LSPQueue is wired by the
// daemon and Enqueue fires after Commit().
//
// We exercise the real lspqueue.Queue (not a mock) to also pin the
// non-blocking-send semantics from queue.go:42-49.
func TestHandler_UpdateChangedFile_EnqueuesLSPRevalidation(t *testing.T) {
	ctx := context.Background()

	store := &fakeStore{tx: &fakeTx{epoch: 7}}
	q := lspqueue.New(8)
	h := handler.New(store, goodHasher, nil, nil)
	h.LSPQueue = q

	if err := h.UpdateChangedFile(ctx, semantic.RepoID("/ws"), "/ws/foo.go"); err != nil {
		t.Fatalf("UpdateChangedFile: %v", err)
	}

	if got, want := q.Len(), 1; got != want {
		t.Fatalf("CR-04 regression: lspqueue.Len() = %d, want %d after one successful commit", got, want)
	}

	job := <-q.Channel()
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
