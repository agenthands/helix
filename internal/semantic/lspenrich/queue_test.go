package lspenrich_test

import (
	"context"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
)

// L1: NewLaneQueue creates a queue; EnqueueLane returns true; Depth reports 1.
func TestLaneQueue_EnqueueLane_BasicHigh(t *testing.T) {
	q := lspenrich.NewLaneQueue(1024, 1024)
	job := lspqueue.RevalidateFileJob{RepoID: "r1", Path: "a.go"}
	if !q.EnqueueLane(lspenrich.LaneHigh, job) {
		t.Fatal("EnqueueLane(LaneHigh) returned false on empty queue")
	}
	if d := q.Depth(lspenrich.LaneHigh); d != 1 {
		t.Fatalf("Depth(LaneHigh): got %d, want 1", d)
	}
	if d := q.Depth(lspenrich.LaneBackground); d != 0 {
		t.Fatalf("Depth(LaneBackground): got %d, want 0", d)
	}
}

// L2: 100 background jobs + 1 high job; high drained first, background drained
// FIFO afterwards. Acceptance #3 at unit level.
func TestLaneQueue_StrictPriority_HighBeforeBackground(t *testing.T) {
	q := lspenrich.NewLaneQueue(1024, 1024)
	// Seed 100 background jobs.
	for i := 0; i < 100; i++ {
		ok := q.EnqueueLane(lspenrich.LaneBackground, lspqueue.RevalidateFileJob{
			RepoID: "r1", Path: bgPath(i),
		})
		if !ok {
			t.Fatalf("EnqueueLane(LaneBackground) #%d returned false", i)
		}
	}
	// Then 1 high.
	highJob := lspqueue.RevalidateFileJob{RepoID: "r1", Path: "high.go"}
	if !q.EnqueueLane(lspenrich.LaneHigh, highJob) {
		t.Fatal("EnqueueLane(LaneHigh) returned false")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// First Drain MUST return the high job.
	got, lane, err := q.Drain(ctx)
	if err != nil {
		t.Fatalf("first Drain: %v", err)
	}
	if lane != lspenrich.LaneHigh {
		t.Fatalf("first Drain lane: got %q, want %q", lane, lspenrich.LaneHigh)
	}
	if got.Path != highJob.Path {
		t.Fatalf("first Drain path: got %q, want %q", got.Path, highJob.Path)
	}

	// Subsequent 100 drains MUST return background jobs FIFO.
	for i := 0; i < 100; i++ {
		got, lane, err = q.Drain(ctx)
		if err != nil {
			t.Fatalf("Drain #%d: %v", i+1, err)
		}
		if lane != lspenrich.LaneBackground {
			t.Fatalf("Drain #%d lane: got %q, want %q", i+1, lane, lspenrich.LaneBackground)
		}
		if got.Path != bgPath(i) {
			t.Fatalf("Drain #%d path: got %q, want %q (FIFO violated)",
				i+1, got.Path, bgPath(i))
		}
	}
}

// L3: Drain blocks until ctx.Done() when both lanes empty.
func TestLaneQueue_Drain_BlocksUntilCtxDone(t *testing.T) {
	q := lspenrich.NewLaneQueue(16, 16)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := q.Drain(ctx)
	if err == nil {
		t.Fatal("Drain returned nil error on empty queues + cancelled ctx")
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("Drain returned too fast (%v); expected to block until ctx deadline", elapsed)
	}
}

// L4: Cap-1 high lane drops the second enqueue.
func TestLaneQueue_FullLaneDrops(t *testing.T) {
	q := lspenrich.NewLaneQueue(1, 1024)
	job := lspqueue.RevalidateFileJob{RepoID: "r1", Path: "a.go"}
	if !q.EnqueueLane(lspenrich.LaneHigh, job) {
		t.Fatal("first high enqueue returned false")
	}
	if q.EnqueueLane(lspenrich.LaneHigh, job) {
		t.Fatal("second high enqueue returned true on cap-1 queue (expected drop)")
	}
}

// L5: Drain reports lane label correctly.
func TestLaneQueue_Drain_LaneLabel(t *testing.T) {
	q := lspenrich.NewLaneQueue(16, 16)
	q.EnqueueLane(lspenrich.LaneBackground, lspqueue.RevalidateFileJob{Path: "bg.go"})
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	got, lane, err := q.Drain(ctx)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if got.Path != "bg.go" {
		t.Fatalf("Drain path: got %q, want bg.go", got.Path)
	}
	if lane != lspenrich.LaneBackground {
		t.Fatalf("Drain lane: got %q, want %q", lane, lspenrich.LaneBackground)
	}
}

// L6: Legacy Enqueue (no lane arg) routes to LaneHigh — backward-compat for
// any caller still holding the renamed-interface single-method shape.
func TestLaneQueue_Enqueue_LegacyAliasMapsToHigh(t *testing.T) {
	q := lspenrich.NewLaneQueue(16, 16)
	if !q.Enqueue(lspqueue.RevalidateFileJob{Path: "legacy.go"}) {
		t.Fatal("legacy Enqueue returned false")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	got, lane, err := q.Drain(ctx)
	if err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if got.Path != "legacy.go" {
		t.Fatalf("Drain path: got %q, want legacy.go", got.Path)
	}
	if lane != lspenrich.LaneHigh {
		t.Fatalf("legacy Enqueue lane: got %q, want %q (must map to LaneHigh)",
			lane, lspenrich.LaneHigh)
	}
}

// EnqueueLane with an unknown lane returns false (closed-enum guard).
func TestLaneQueue_EnqueueLane_UnknownLaneRejected(t *testing.T) {
	q := lspenrich.NewLaneQueue(16, 16)
	if q.EnqueueLane(lspenrich.Lane("garbage"), lspqueue.RevalidateFileJob{Path: "x.go"}) {
		t.Fatal("EnqueueLane returned true for unknown lane (expected closed-enum reject)")
	}
}

func bgPath(i int) string {
	// Stable, FIFO-checkable string.
	return "bg-" + itoa3(i) + ".go"
}

func itoa3(i int) string {
	// Zero-padded 3-digit string so lexical and numeric order agree (the FIFO
	// assertion compares the string Path field).
	const digits = "0123456789"
	return string([]byte{digits[(i/100)%10], digits[(i/10)%10], digits[i%10]})
}
