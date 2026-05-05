package coalescer_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/coalescer"
	"github.com/agenthands/helix/internal/workspace"
)

// recordingHandler counts Dispatch calls and records the events it sees.
type recordingHandler struct {
	mu     sync.Mutex
	events []live.SourceChangeEvent
	wakeCh chan struct{} // pulse on every Dispatch
}

func newRecordingHandler() *recordingHandler {
	return &recordingHandler{wakeCh: make(chan struct{}, 256)}
}

func (h *recordingHandler) Dispatch(_ context.Context, ev live.SourceChangeEvent) error {
	h.mu.Lock()
	h.events = append(h.events, ev)
	h.mu.Unlock()
	select {
	case h.wakeCh <- struct{}{}:
	default:
	}
	return nil
}

func (h *recordingHandler) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.events)
}

func (h *recordingHandler) Snapshot() []live.SourceChangeEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]live.SourceChangeEvent, len(h.events))
	copy(out, h.events)
	return out
}

// TestCoalescer_Debounce: 3 events arrive within 50ms apart; debounce=200ms,
// max=1500ms.  After ~250ms wall time, exactly one merged event should
// fire.
func TestCoalescer_Debounce(t *testing.T) {
	h := newRecordingHandler()
	c := coalescer.New(workspace.WorkspaceKey{RepoRoot: "ws1"},
		coalescer.Config{
			Debounce:            200 * time.Millisecond,
			MaxBatchDelay:       1500 * time.Millisecond,
			BulkChangeThreshold: 200,
			QueueSize:           16,
		}, h, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	for i := 0; i < 3; i++ {
		c.Enqueue(live.SourceChangeEvent{
			Path: "a.go",
			Kind: live.ChangeFileModified,
		})
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for debounce + slack.
	select {
	case <-h.wakeCh:
		// got at least one Dispatch
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("debounce never fired; got %d events", h.Count())
	}
	// Allow the rest of any in-flight dispatches to settle.
	time.Sleep(100 * time.Millisecond)
	if got := h.Count(); got != 1 {
		t.Fatalf("debounce: expected 1 merged Dispatch, got %d", got)
	}
}

// TestCoalescer_MaxBatchDelay: continuous events resetting the debounce
// timer eventually flush via the max-batch ceiling.
func TestCoalescer_MaxBatchDelay(t *testing.T) {
	h := newRecordingHandler()
	c := coalescer.New(workspace.WorkspaceKey{RepoRoot: "ws1"},
		coalescer.Config{
			Debounce:            500 * time.Millisecond, // long
			MaxBatchDelay:       250 * time.Millisecond, // short ceiling
			BulkChangeThreshold: 200,
			QueueSize:           1024,
		}, h, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	// Stream events at 50ms intervals for 400ms total — debounce never
	// elapses (each new event resets it), but the max ceiling at 250ms
	// must force a flush.
	stop := time.NewTimer(400 * time.Millisecond)
	defer stop.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()

	go func() {
		for {
			select {
			case <-stop.C:
				return
			case <-tick.C:
				c.Enqueue(live.SourceChangeEvent{
					Path: "a.go",
					Kind: live.ChangeFileModified,
				})
			}
		}
	}()

	// Wait up to 600ms for the ceiling-induced flush.
	select {
	case <-h.wakeCh:
		// good — flush fired before continuous events ended
	case <-time.After(600 * time.Millisecond):
		t.Fatalf("max-batch ceiling never forced a flush; got %d events", h.Count())
	}
}

// TestCoalescer_NonBlockingEnqueue: fill the input channel buffer
// without running the consumer; subsequent Enqueue calls MUST drop
// (drops counter increments) and MUST return immediately.
func TestCoalescer_NonBlockingEnqueue(t *testing.T) {
	h := newRecordingHandler()
	c := coalescer.New(workspace.WorkspaceKey{RepoRoot: "ws1"},
		coalescer.Config{
			Debounce:            5 * time.Second, // never fire
			MaxBatchDelay:       10 * time.Second,
			BulkChangeThreshold: 200,
			QueueSize:           4, // tiny buffer, easy to fill
		}, h, nil)
	// Do NOT call Run — we want Enqueue to fill the channel without
	// being drained.

	// First 4 fill the buffer.
	for i := 0; i < 4; i++ {
		c.Enqueue(live.SourceChangeEvent{Path: "a.go", Kind: live.ChangeFileModified})
	}
	if c.Drops() != 0 {
		t.Fatalf("expected 0 drops after filling buffer, got %d", c.Drops())
	}

	// Subsequent enqueues drop and return immediately.
	const overflow = 100
	start := time.Now()
	for i := 0; i < overflow; i++ {
		c.Enqueue(live.SourceChangeEvent{Path: "a.go", Kind: live.ChangeFileModified})
	}
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("Enqueue blocked for %v; expected non-blocking", elapsed)
	}
	if got := c.Drops(); got != overflow {
		t.Fatalf("expected %d drops, got %d", overflow, got)
	}
}

// TestCoalescer_NoOpFlushDoesNotDispatch: enqueue created+deleted on
// the same path; after debounce, no Dispatch call should fire (the
// merged set is empty per SPEC §16.2 created+deleted = drop).
func TestCoalescer_NoOpFlushDoesNotDispatch(t *testing.T) {
	h := newRecordingHandler()
	c := coalescer.New(workspace.WorkspaceKey{RepoRoot: "ws1"},
		coalescer.Config{
			Debounce:            100 * time.Millisecond,
			MaxBatchDelay:       1000 * time.Millisecond,
			BulkChangeThreshold: 200,
			QueueSize:           16,
		}, h, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	c.Enqueue(live.SourceChangeEvent{Path: "x.go", Kind: live.ChangeFileCreated})
	c.Enqueue(live.SourceChangeEvent{Path: "x.go", Kind: live.ChangeFileDeleted})

	// Wait well past the debounce window.
	time.Sleep(300 * time.Millisecond)

	if got := h.Count(); got != 0 {
		t.Fatalf("no-op flush: expected 0 Dispatch calls, got %d (%+v)",
			got, h.Snapshot())
	}
}

// TestCoalescer_PerWorkspaceIsolation: two coalescers handle two
// distinct workspaces; events on ws1 do not leak into ws2's handler.
func TestCoalescer_PerWorkspaceIsolation(t *testing.T) {
	h1 := newRecordingHandler()
	h2 := newRecordingHandler()
	c1 := coalescer.New(workspace.WorkspaceKey{RepoRoot: "ws1"},
		coalescer.Config{Debounce: 100 * time.Millisecond, QueueSize: 16}, h1, nil)
	c2 := coalescer.New(workspace.WorkspaceKey{RepoRoot: "ws2"},
		coalescer.Config{Debounce: 100 * time.Millisecond, QueueSize: 16}, h2, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c1.Run(ctx)
	go c2.Run(ctx)

	for i := 0; i < 5; i++ {
		c1.Enqueue(live.SourceChangeEvent{Path: "a.go", Kind: live.ChangeFileModified})
	}
	for i := 0; i < 3; i++ {
		c2.Enqueue(live.SourceChangeEvent{Path: "b.go", Kind: live.ChangeFileModified})
	}

	// Wait for both flushes.
	time.Sleep(300 * time.Millisecond)

	// Each merges to a single event.
	if got := h1.Count(); got != 1 {
		t.Fatalf("ws1: expected 1 merged event, got %d", got)
	}
	if got := h2.Count(); got != 1 {
		t.Fatalf("ws2: expected 1 merged event, got %d", got)
	}
	// Sanity: the events carry the right path.
	if h1.Snapshot()[0].Path != "a.go" {
		t.Fatalf("ws1 saw wrong path: %v", h1.Snapshot())
	}
	if h2.Snapshot()[0].Path != "b.go" {
		t.Fatalf("ws2 saw wrong path: %v", h2.Snapshot())
	}
}

// errorHandler returns Dispatch errors for the first N calls then
// succeeds; used to verify per-event errors do NOT abort the batch.
type errorHandler struct {
	failFirst int32
	count     atomic.Int32
}

func (e *errorHandler) Dispatch(_ context.Context, _ live.SourceChangeEvent) error {
	n := e.count.Add(1)
	if n <= e.failFirst {
		return errBatchPart
	}
	return nil
}

var errBatchPart = sentinelErr("batch-part-failure")

type sentinelErr string

func (s sentinelErr) Error() string { return string(s) }

// TestCoalescer_PerEventErrorDoesNotAbortBatch: 3 events flush together;
// first dispatch returns an error.  All 3 must still be attempted.
func TestCoalescer_PerEventErrorDoesNotAbortBatch(t *testing.T) {
	h := &errorHandler{failFirst: 1}
	c := coalescer.New(workspace.WorkspaceKey{RepoRoot: "ws1"},
		coalescer.Config{
			Debounce:            100 * time.Millisecond,
			MaxBatchDelay:       1000 * time.Millisecond,
			BulkChangeThreshold: 200,
			QueueSize:           16,
		}, h, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	c.Enqueue(live.SourceChangeEvent{Path: "a.go", Kind: live.ChangeFileModified})
	c.Enqueue(live.SourceChangeEvent{Path: "b.go", Kind: live.ChangeFileModified})
	c.Enqueue(live.SourceChangeEvent{Path: "c.go", Kind: live.ChangeFileModified})

	time.Sleep(300 * time.Millisecond)

	if got := h.count.Load(); got != 3 {
		t.Fatalf("D-02 batch invariant: expected 3 Dispatch calls, got %d", got)
	}
}
