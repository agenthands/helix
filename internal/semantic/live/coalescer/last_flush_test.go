// Phase 63 P63-02 Task 1: LastFlushAt + SetOnFlush accessor tests.

package coalescer_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/coalescer"
	"github.com/agenthands/helix/internal/workspace"
)

// TestCoalescer_LastFlushAt_StampsOnFlush verifies that the post-flush
// timestamp accessor (consumed by the Phase 63 compaction gate's idle
// check) is stamped within an expected window of "now" after a flush
// completes. Pre-flush LastFlushAt() returns the zero value.
func TestCoalescer_LastFlushAt_StampsOnFlush(t *testing.T) {
	ws := workspace.WorkspaceKey{RepoRoot: "/r"}
	h := newRecordingHandler()
	c := coalescer.New(ws, coalescer.Config{
		Debounce:            5 * time.Millisecond,
		MaxBatchDelay:       100 * time.Millisecond,
		BulkChangeThreshold: 200,
		QueueSize:           16,
	}, h, nil)

	// Pre-flush: zero value.
	if !c.LastFlushAt().IsZero() {
		t.Fatalf("LastFlushAt() pre-flush: got %v, want zero value", c.LastFlushAt())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	c.Enqueue(live.SourceChangeEvent{Kind: live.ChangeFileModified, Path: "a.go"})

	// Wait for flush.
	select {
	case <-h.wakeCh:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch never fired")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if !c.LastFlushAt().IsZero() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	got := c.LastFlushAt()
	if got.IsZero() {
		t.Fatal("LastFlushAt() never stamped after flush")
	}
	if d := time.Since(got); d > 5*time.Second {
		t.Errorf("LastFlushAt() too far in the past: %v", d)
	}
}

// TestCoalescer_SetOnFlush_Invoked verifies that the post-flush hook is
// invoked at least once after a flush. The hook fires for both the
// dispatch path and the empty-flush short-circuit; here we drive the
// dispatch path.
func TestCoalescer_SetOnFlush_Invoked(t *testing.T) {
	ws := workspace.WorkspaceKey{RepoRoot: "/r"}
	h := newRecordingHandler()
	c := coalescer.New(ws, coalescer.Config{
		Debounce:            5 * time.Millisecond,
		MaxBatchDelay:       100 * time.Millisecond,
		BulkChangeThreshold: 200,
		QueueSize:           16,
	}, h, nil)

	var hookCount atomic.Int32
	c.SetOnFlush(func() { hookCount.Add(1) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	c.Enqueue(live.SourceChangeEvent{Kind: live.ChangeFileModified, Path: "a.go"})

	select {
	case <-h.wakeCh:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch never fired")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if hookCount.Load() > 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if got := hookCount.Load(); got < 1 {
		t.Errorf("OnFlush hook count: got %d, want >= 1", got)
	}
}
