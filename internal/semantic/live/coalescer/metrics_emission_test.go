package coalescer_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/coalescer"
	"github.com/agenthands/helix/internal/workspace"
)

// CR-01 regression: helix_semantic_live_updates_total{kind, outcome}
// must be emitted at the coalescer drop, applied, and error sites.
// Pre-fix: the metric helper was wired in obs/metrics.go but no
// production code path called it. This test pins the wiring at every
// outcome boundary by injecting a recording MetricsSink into Config and
// driving each branch.

type recordingMetrics struct {
	mu      sync.Mutex
	outcome map[string]int // key: "kind/outcome"
}

func newRecordingMetrics() *recordingMetrics {
	return &recordingMetrics{outcome: make(map[string]int)}
}

func (r *recordingMetrics) SemanticLiveUpdatesInc(kind, outcome string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outcome[kind+"/"+outcome]++
}

func (r *recordingMetrics) Get(kind, outcome string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.outcome[kind+"/"+outcome]
}

type errHandler struct{}

func (errHandler) Dispatch(_ context.Context, _ live.SourceChangeEvent) error {
	return errors.New("synthetic dispatch error")
}

func TestCoalescer_EmitsAppliedMetricOnSuccess(t *testing.T) {
	rm := newRecordingMetrics()
	h := newRecordingHandler()
	cfg := coalescer.Config{
		Debounce:            5 * time.Millisecond,
		MaxBatchDelay:       50 * time.Millisecond,
		BulkChangeThreshold: 100,
		QueueSize:           16,
		Metrics:             rm,
	}
	c := coalescer.New(workspace.WorkspaceKey{RepoRoot: "/ws"}, cfg, h, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	c.Enqueue(live.SourceChangeEvent{
		RepoID: "/ws", Kind: live.ChangeFileModified, Path: "/ws/foo.go",
		Source: live.ChangeSourceFsnotify, ObservedAt: time.Now(),
	})

	// Wait for the dispatch to land.
	select {
	case <-h.wakeCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler.Dispatch never fired")
	}

	// Metric must have advanced by 1 on the file_modified/applied bucket.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if rm.Get("file_modified", "applied") == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("CR-01 regression: applied counter did not advance — got %d for file_modified/applied", rm.Get("file_modified", "applied"))
}

func TestCoalescer_EmitsErrorMetricOnDispatchFailure(t *testing.T) {
	rm := newRecordingMetrics()
	cfg := coalescer.Config{
		Debounce:            5 * time.Millisecond,
		MaxBatchDelay:       50 * time.Millisecond,
		BulkChangeThreshold: 100,
		QueueSize:           16,
		Metrics:             rm,
	}
	c := coalescer.New(workspace.WorkspaceKey{RepoRoot: "/ws"}, cfg, errHandler{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	c.Enqueue(live.SourceChangeEvent{
		RepoID: "/ws", Kind: live.ChangeFileDeleted, Path: "/ws/gone.go",
		Source: live.ChangeSourceFsnotify, ObservedAt: time.Now(),
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if rm.Get("file_deleted", "error") == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("CR-01 regression: error counter did not advance — got %d for file_deleted/error", rm.Get("file_deleted", "error"))
}

// blockingHandler never returns from Dispatch so that the queue
// genuinely fills (consumer-blocked, not consumer-fast). The test then
// pumps enough events to overflow and asserts the dropped counter.
type blockingHandler struct {
	gate chan struct{}
}

func newBlockingHandler() *blockingHandler {
	return &blockingHandler{gate: make(chan struct{})}
}

func (b *blockingHandler) Dispatch(ctx context.Context, _ live.SourceChangeEvent) error {
	select {
	case <-b.gate:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingHandler) Release() { close(b.gate) }

func TestCoalescer_EmitsDroppedMetricOnQueueFull(t *testing.T) {
	rm := newRecordingMetrics()
	cfg := coalescer.Config{
		Debounce:            time.Hour, // never auto-flush
		MaxBatchDelay:       time.Hour,
		BulkChangeThreshold: 1000000,
		QueueSize:           2, // tiny so we can saturate fast
		Metrics:             rm,
	}
	bh := newBlockingHandler()
	defer bh.Release()
	c := coalescer.New(workspace.WorkspaceKey{RepoRoot: "/ws"}, cfg, bh, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	// Pump well over QueueSize. The Run loop will drain the channel into
	// the pending map (which is unbounded), so the visible drop site is
	// the Enqueue select-default. To force the channel-full path we
	// pump faster than Run can drain — a tight loop of 1024 enqueues
	// against QueueSize=2 reliably saturates.
	const N = 1024
	for i := 0; i < N; i++ {
		c.Enqueue(live.SourceChangeEvent{
			RepoID: "/ws", Kind: live.ChangeHelixEdit, Path: "/ws/file.go",
			Source: live.ChangeSourceHelixEdit, ObservedAt: time.Now(),
		})
	}

	// Some events landed in the channel and the pending map; some hit the
	// drop branch. Exact ratio is timing-dependent. We assert at least
	// ONE drop so the metric wiring is proven (not the exact count).
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if rm.Get("helix_edit", "dropped") >= 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("CR-01 regression: dropped counter did not advance after %d enqueues — got %d for helix_edit/dropped (queue may have been drained too fast; tighten QueueSize or block harder)", N, rm.Get("helix_edit", "dropped"))
}
