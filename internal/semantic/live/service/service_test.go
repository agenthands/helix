package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/coalescer"
	"github.com/agenthands/helix/internal/semantic/live/service"
	"github.com/agenthands/helix/internal/workspace"
)

// recordingHandler counts Dispatch calls.
type recordingHandler struct {
	mu     sync.Mutex
	events []live.SourceChangeEvent
}

func (h *recordingHandler) Dispatch(_ context.Context, ev live.SourceChangeEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, ev)
	return nil
}

func (h *recordingHandler) Snapshot() []live.SourceChangeEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]live.SourceChangeEvent, len(h.events))
	copy(out, h.events)
	return out
}

// stubClassifier returns ChangeFileModified for every non-empty path,
// no-op for empty paths.  Helix-edit short-circuits.
func stubClassifier(_ context.Context, _ semantic.RepoID, path string, src live.ChangeSource) (live.SourceChangeKind, bool, error) {
	if src == live.ChangeSourceHelixEdit {
		return live.ChangeHelixEdit, true, nil
	}
	if path == "" {
		return "", false, nil
	}
	return live.ChangeFileModified, true, nil
}

func repoIDFromKey(k workspace.WorkspaceKey) semantic.RepoID {
	return semantic.RepoID(k.RepoRoot)
}

// TestLiveService_OnEdit_NonBlocking pins T-60-04-05: OnEdit MUST return
// in O(microseconds) even when the per-workspace coalescer's input
// channel is saturated and the consumer goroutine is no longer draining.
func TestLiveService_OnEdit_NonBlocking(t *testing.T) {
	h := &recordingHandler{}
	cfg := coalescer.Config{
		Debounce:            5 * time.Second,
		MaxBatchDelay:       10 * time.Second,
		BulkChangeThreshold: 200,
		QueueSize:           4,
	}
	s := service.New(cfg, h, stubClassifier, repoIDFromKey, nopLogger{})
	ws := workspace.WorkspaceKey{RepoRoot: "ws1"}
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx, ws)
	cancel() // kill the consumer goroutine
	time.Sleep(50 * time.Millisecond)

	const calls = 1000
	start := time.Now()
	for i := 0; i < calls; i++ {
		_ = s.OnEdit(context.Background(), ws, []string{"a.go"})
	}
	elapsed := time.Since(start)
	avg := elapsed / calls
	if avg > 100*time.Microsecond {
		t.Fatalf("OnEdit avg latency = %v; want <100us per call", avg)
	}
}

// TestLiveService_OnEdit_RoutesToCorrectWorkspace: events on ws1 land
// in ws1's coalescer; events on ws2 land in ws2's coalescer. No leak.
func TestLiveService_OnEdit_RoutesToCorrectWorkspace(t *testing.T) {
	h := &recordingHandler{}
	cfg := coalescer.Config{
		Debounce:            50 * time.Millisecond,
		MaxBatchDelay:       500 * time.Millisecond,
		BulkChangeThreshold: 200,
		QueueSize:           16,
	}
	s := service.New(cfg, h, stubClassifier, repoIDFromKey, nopLogger{})
	ws1 := workspace.WorkspaceKey{RepoRoot: "ws1"}
	ws2 := workspace.WorkspaceKey{RepoRoot: "ws2"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx, ws1)
	s.Start(ctx, ws2)

	if err := s.OnEdit(context.Background(), ws1, []string{"a.go"}); err != nil {
		t.Fatal(err)
	}
	if err := s.OnEdit(context.Background(), ws2, []string{"b.go"}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	got := h.Snapshot()
	if len(got) != 2 {
		t.Fatalf("expected 2 dispatched events, got %d: %+v", len(got), got)
	}
	seen := map[semantic.RepoID]string{}
	for _, ev := range got {
		seen[ev.RepoID] = ev.Path
	}
	if seen["ws1"] != "a.go" {
		t.Fatalf("ws1 routed to %q, want a.go", seen["ws1"])
	}
	if seen["ws2"] != "b.go" {
		t.Fatalf("ws2 routed to %q, want b.go", seen["ws2"])
	}
}

// TestLiveService_OnWorkspaceChanged: external producer (mocked
// watcher) calls OnWorkspaceChanged(ctx, sig) and the resulting events
// reach the handler with the producer-supplied source label preserved.
func TestLiveService_OnWorkspaceChanged(t *testing.T) {
	h := &recordingHandler{}
	cfg := coalescer.Config{
		Debounce:            50 * time.Millisecond,
		MaxBatchDelay:       500 * time.Millisecond,
		BulkChangeThreshold: 200,
		QueueSize:           16,
	}
	s := service.New(cfg, h, stubClassifier, repoIDFromKey, nopLogger{})
	ws := workspace.WorkspaceKey{RepoRoot: "ws1"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx, ws)

	if err := s.OnWorkspaceChanged(context.Background(), live.WorkspaceChangeSignal{
		WorkspaceID: ws,
		Paths:       []string{"watched.go"},
		Source:      live.ChangeSourceFsnotify,
		ObservedAt:  time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	got := h.Snapshot()
	if len(got) != 1 {
		t.Fatalf("expected 1 dispatched event, got %d", len(got))
	}
	if got[0].Source != live.ChangeSourceFsnotify {
		t.Fatalf("source label dropped: got %q, want %q", got[0].Source, live.ChangeSourceFsnotify)
	}
	if got[0].Path != "watched.go" {
		t.Fatalf("path mismatch: got %q", got[0].Path)
	}
}

// TestLiveService_SatisfiesEditNotifier pins the load-bearing var-_
// assertion via runtime.
func TestLiveService_SatisfiesEditNotifier(t *testing.T) {
	h := &recordingHandler{}
	s := service.New(coalescer.Config{}, h, stubClassifier, repoIDFromKey, nopLogger{})
	var n kernel.EditNotifier = s
	if n == nil {
		t.Fatal("Service does not satisfy kernel.EditNotifier")
	}
}

// TestLiveService_StartIdempotent: calling Start twice MUST be a no-op.
func TestLiveService_StartIdempotent(t *testing.T) {
	h := &recordingHandler{}
	s := service.New(coalescer.Config{
		Debounce:  50 * time.Millisecond,
		QueueSize: 16,
	}, h, stubClassifier, repoIDFromKey, nopLogger{})
	ws := workspace.WorkspaceKey{RepoRoot: "ws1"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx, ws)
	s.Start(ctx, ws) // second call MUST be a no-op
	// race detector flags double-Run on the same channel if violated
}

// TestLiveService_StopCancelsCoalescer: after Stop, OnEdit lands on
// the unstarted-workspace path (logs + drops).
func TestLiveService_StopCancelsCoalescer(t *testing.T) {
	h := &recordingHandler{}
	s := service.New(coalescer.Config{
		Debounce:  50 * time.Millisecond,
		QueueSize: 16,
	}, h, stubClassifier, repoIDFromKey, nopLogger{})
	ws := workspace.WorkspaceKey{RepoRoot: "ws1"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx, ws)
	s.Stop(ws)
	if err := s.OnEdit(context.Background(), ws, []string{"a.go"}); err != nil {
		t.Fatalf("OnEdit after Stop: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	if got := h.Snapshot(); len(got) != 0 {
		t.Fatalf("OnEdit after Stop should drop; got %d events", len(got))
	}
}

type nopLogger struct{}

func (nopLogger) Warn(string, ...any) {}
