// Phase 63 P63-02 Task 1: LastEnqueueAt + DepthAll accessor tests.

package lspenrich_test

import (
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
)

// TestLaneQueue_LastEnqueueAt_StampsOnSuccess: pre-enqueue zero, post
// enqueue stamped within reasonable window.
func TestLaneQueue_LastEnqueueAt_StampsOnSuccess(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	if !q.LastEnqueueAt().IsZero() {
		t.Fatalf("LastEnqueueAt pre-enqueue: got %v, want zero", q.LastEnqueueAt())
	}

	if !q.EnqueueLane(lspenrich.LaneHigh, lspqueue.RevalidateFileJob{}) {
		t.Fatal("EnqueueLane returned false on empty queue")
	}
	got := q.LastEnqueueAt()
	if got.IsZero() {
		t.Fatal("LastEnqueueAt did not stamp on successful enqueue")
	}
	if d := time.Since(got); d > 5*time.Second {
		t.Errorf("LastEnqueueAt too far in the past: %v", d)
	}
}

// TestLaneQueue_DepthAll: sums both lanes.
func TestLaneQueue_DepthAll(t *testing.T) {
	q := lspenrich.NewLaneQueue(8, 8)
	if got := q.DepthAll(); got != 0 {
		t.Errorf("empty DepthAll: got %d, want 0", got)
	}
	q.EnqueueLane(lspenrich.LaneHigh, lspqueue.RevalidateFileJob{})
	q.EnqueueLane(lspenrich.LaneHigh, lspqueue.RevalidateFileJob{})
	q.EnqueueLane(lspenrich.LaneBackground, lspqueue.RevalidateFileJob{})
	if got := q.DepthAll(); got != 3 {
		t.Errorf("DepthAll after enqueue: got %d, want 3", got)
	}
}

// TestLaneQueue_LastEnqueueAt_NilSafe: nil receiver returns zero.
func TestLaneQueue_LastEnqueueAt_NilSafe(t *testing.T) {
	var q *lspenrich.LaneQueue
	if !q.LastEnqueueAt().IsZero() {
		t.Errorf("nil receiver LastEnqueueAt: got %v, want zero", q.LastEnqueueAt())
	}
	if got := q.DepthAll(); got != 0 {
		t.Errorf("nil receiver DepthAll: got %d, want 0", got)
	}
}
