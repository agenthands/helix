package scheduler_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/scheduler"
)

// mockHandler counts UpdateChangedFile and HandleFileDeleted calls so
// the test can pin which paths went down which branch.
type mockHandler struct {
	mu          sync.Mutex
	updateCalls atomic.Int32
	deleteCalls atomic.Int32
	updatePaths []string
	deletePaths []string
}

func (h *mockHandler) UpdateChangedFile(_ context.Context, _ semantic.RepoID, path string) error {
	h.updateCalls.Add(1)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.updatePaths = append(h.updatePaths, path)
	return nil
}

func (h *mockHandler) HandleFileDeleted(_ context.Context, _ semantic.RepoID, path string) error {
	h.deleteCalls.Add(1)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.deletePaths = append(h.deletePaths, path)
	return nil
}

func TestScheduleIncremental_DispatchesPerKind(t *testing.T) {
	sched := scheduler.NewScheduler(nil)
	h := &mockHandler{}
	sched.SetIncrementalHandler(h)

	job := sched.ScheduleIncremental("ws1", []scheduler.FileChange{
		{Path: "a.go", Kind: "modified"},
		{Path: "b.go", Kind: "created"},
		{Path: "c.go", Kind: "deleted"},
	})
	if job == "" || string(job) == "phase60-incremental-stub" {
		t.Fatalf("expected non-empty, non-stub JobID; got %q", job)
	}

	// Wait for the dispatch goroutine to drain.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if h.updateCalls.Load() == 2 && h.deleteCalls.Load() == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := h.updateCalls.Load(); got != 2 {
		t.Fatalf("expected 2 UpdateChangedFile calls (modified + created), got %d", got)
	}
	if got := h.deleteCalls.Load(); got != 1 {
		t.Fatalf("expected 1 HandleFileDeleted call, got %d", got)
	}
}

func TestScheduleIncremental_ReturnsRealJobID(t *testing.T) {
	sched := scheduler.NewScheduler(nil)
	job := sched.ScheduleIncremental("ws1", nil)
	if job == "" {
		t.Fatal("expected non-empty JobID")
	}
	if string(job) == "phase60-incremental-stub" {
		t.Fatalf("expected sentinel JobID to be replaced; got %q", job)
	}
}

func TestScheduleIncremental_TransitionsToIndexing(t *testing.T) {
	sched := scheduler.NewScheduler(nil)
	_ = sched.ScheduleIncremental("ws1", nil)
	st := sched.Status("ws1")
	if st.State != scheduler.SemanticIndexing {
		t.Fatalf("expected state %q after ScheduleIncremental; got %q",
			scheduler.SemanticIndexing, st.State)
	}
}

// TestScheduleIncremental_NilHandlerSafe: scheduler may be constructed
// before live wiring lands; ScheduleIncremental MUST not panic when
// no handler is registered.
func TestScheduleIncremental_NilHandlerSafe(t *testing.T) {
	sched := scheduler.NewScheduler(nil) // no SetIncrementalHandler
	job := sched.ScheduleIncremental("ws1", []scheduler.FileChange{
		{Path: "a.go", Kind: "modified"},
	})
	if job == "" {
		t.Fatal("expected non-empty JobID even without handler")
	}
}
