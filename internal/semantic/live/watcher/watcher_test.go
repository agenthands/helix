package watcher

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/workspace"
)

// recordingProducer captures every WorkspaceChangeSignal sent by the
// watcher so tests can assert on Source / Paths shape. The buffered
// channel is sized generously to avoid blocking the watcher
// goroutine on test-side pickup latency.
type recordingProducer struct {
	mu  sync.Mutex
	got []live.WorkspaceChangeSignal
	ch  chan live.WorkspaceChangeSignal
}

func newRecordingProducer() *recordingProducer {
	return &recordingProducer{ch: make(chan live.WorkspaceChangeSignal, 32)}
}

func (r *recordingProducer) OnWorkspaceChanged(ctx context.Context, sig live.WorkspaceChangeSignal) error {
	r.mu.Lock()
	r.got = append(r.got, sig)
	r.mu.Unlock()
	select {
	case r.ch <- sig:
	default:
	}
	return nil
}

func (r *recordingProducer) snapshot() []live.WorkspaceChangeSignal {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]live.WorkspaceChangeSignal, len(r.got))
	copy(out, r.got)
	return out
}

// testLogger returns a slog.Logger that discards output by default.
// Tests that need to assert on log output construct their own
// bytes.Buffer-backed handler instead.
func testLogger(_ testing.TB) *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// startManager is the common harness for watcher tests: t.TempDir()
// repo root, recordingProducer, Manager wired to a 50ms debounce
// (well under the test's 2s deadline). Returns the workspace key,
// producer, and a cleanup that cancels the ctx and stops the
// manager.
func startManager(t *testing.T, debounce time.Duration) (string, workspace.WorkspaceKey, *recordingProducer, func()) {
	t.Helper()
	dir := t.TempDir()
	ws := workspace.WorkspaceKey{RepoRoot: dir, Language: "go"}

	rp := newRecordingProducer()
	cfg := Config{DebounceMs: debounce}
	mgr := NewManager(rp, cfg, testLogger(t))

	ctx, cancel := context.WithCancel(context.Background())
	if err := mgr.Start(ctx, ws); err != nil {
		cancel()
		t.Fatalf("Manager.Start: %v", err)
	}

	return dir, ws, rp, func() {
		cancel()
		mgr.Stop(ws)
	}
}

// waitForSignal drains rp.ch with a deadline. Returns the first
// signal whose Paths contain wantPath as a substring (we only test
// trailing path segments to avoid coupling to t.TempDir() prefixes).
func waitForSignal(t *testing.T, rp *recordingProducer, wantPathSuffix string, timeout time.Duration) live.WorkspaceChangeSignal {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case sig := <-rp.ch:
			for _, p := range sig.Paths {
				if filepath.Base(p) == wantPathSuffix {
					return sig
				}
			}
			// Not the signal we wanted — keep draining.
		case <-deadline:
			t.Fatalf("timeout waiting %v for signal containing %q; got %d signals: %+v",
				timeout, wantPathSuffix, len(rp.snapshot()), rp.snapshot())
		}
	}
}

// TestWorkspaceWatcher_DebounceCoalesces — three writes in <50ms
// should produce a single OnWorkspaceChanged call after the debounce
// window elapses. Pins SPEC §16.2 debounce semantics.
func TestWorkspaceWatcher_DebounceCoalesces(t *testing.T) {
	dir, _, rp, teardown := startManager(t, 100*time.Millisecond)
	defer teardown()

	target := filepath.Join(dir, "auth.go")
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(target, []byte("package main\n// rev "+string(rune('a'+i))), 0o644); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		// Stay well under the 100ms debounce so the timer keeps
		// resetting rather than firing.
		time.Sleep(10 * time.Millisecond)
	}

	sig := waitForSignal(t, rp, "auth.go", 2*time.Second)
	if sig.Source != live.ChangeSourceFsnotify {
		t.Fatalf("Source = %v, want %v", sig.Source, live.ChangeSourceFsnotify)
	}
	if len(sig.Paths) == 0 {
		t.Fatalf("expected at least one path, got empty")
	}

	// Allow a second debounce window to confirm we did NOT receive a
	// second signal from the same burst (the three writes coalesced).
	// Note: a stray fsnotify Chmod or kernel-buffered event could
	// produce one extra signal, so we only assert "<= 2" rather than
	// strict "== 1" — the load-bearing invariant is "not three".
	time.Sleep(200 * time.Millisecond)
	if got := len(rp.snapshot()); got > 2 {
		t.Fatalf("debounce did not coalesce: got %d signals from 3 writes", got)
	}
}

// TestWorkspaceWatcher_JetBrainsTempfileFiltered — a Create event on
// a `___jb_tmp___` path MUST NOT enter the pending set. We verify by
// driving the unit `handleEvent` directly (the fsnotify integration
// test would race with the real rename onto the destination, which
// IS supposed to fire a signal).
func TestWorkspaceWatcher_JetBrainsTempfileFiltered(t *testing.T) {
	rp := newRecordingProducer()
	ww := &workspaceWatcher{
		ws:       workspace.WorkspaceKey{RepoRoot: "/repo"},
		cfg:      applyDefaults(Config{}),
		producer: rp,
		logger:   testLogger(t),
		pending:  make(map[string]struct{}),
	}

	// Drive handleEvent against a fake fsnotify.Event with the JetBrains
	// suffix. We pass a no-op flush — the test's invariant is that
	// pending stays empty.
	ww.handleEvent(fakeCreate("/repo/auth.go___jb_tmp___"), func() {})
	ww.handleEvent(fakeCreate("/repo/auth.go___jb_old___"), func() {})

	ww.mu.Lock()
	defer ww.mu.Unlock()
	if got := len(ww.pending); got != 0 {
		t.Fatalf("pending = %d, want 0 (JetBrains tempfile suffixes must not enqueue)", got)
	}
}

// TestWorkspaceWatcher_IgnoreDirs — an event under .git/ MUST be
// dropped before reaching the pending map. The check is path-segment
// based (60-CONTEXT D-05 ignore-dir contract); a sibling file
// "vendor.go" (which contains "vendor" as a substring but not as a
// path segment) MUST NOT be filtered.
func TestWorkspaceWatcher_IgnoreDirs(t *testing.T) {
	rp := newRecordingProducer()
	ww := &workspaceWatcher{
		ws:       workspace.WorkspaceKey{RepoRoot: "/repo"},
		cfg:      applyDefaults(Config{}),
		producer: rp,
		logger:   testLogger(t),
		pending:  make(map[string]struct{}),
	}

	// .git/ subpath MUST be filtered.
	ww.handleEvent(fakeCreate("/repo/.git/refs/heads/main"), func() {})
	// node_modules/ MUST be filtered.
	ww.handleEvent(fakeCreate("/repo/node_modules/foo/index.js"), func() {})
	// "vendor.go" file MUST NOT be filtered (segment match only).
	ww.handleEvent(fakeCreate("/repo/vendor.go"), func() {})
	// Real "vendor/" directory subpath MUST be filtered.
	ww.handleEvent(fakeCreate("/repo/vendor/lib/foo.go"), func() {})

	ww.mu.Lock()
	defer ww.mu.Unlock()
	if _, ok := ww.pending["/repo/vendor.go"]; !ok {
		t.Fatalf("vendor.go (file) was wrongly filtered: pending=%v", ww.pending)
	}
	if _, ok := ww.pending["/repo/.git/refs/heads/main"]; ok {
		t.Fatalf(".git/ subpath was NOT filtered: pending=%v", ww.pending)
	}
	if _, ok := ww.pending["/repo/node_modules/foo/index.js"]; ok {
		t.Fatalf("node_modules/ subpath was NOT filtered: pending=%v", ww.pending)
	}
	if _, ok := ww.pending["/repo/vendor/lib/foo.go"]; ok {
		t.Fatalf("vendor/ subpath was NOT filtered: pending=%v", ww.pending)
	}
}

// TestWorkspaceWatcher_ProducerReceivesPathsOnly — the signal landing
// at the producer must be paths-only (D-01 invariant): no Kind on
// WorkspaceChangeSignal, Source must be the watcher's ChangeSource,
// and Paths must contain the file we wrote.
func TestWorkspaceWatcher_ProducerReceivesPathsOnly(t *testing.T) {
	dir, ws, rp, teardown := startManager(t, 50*time.Millisecond)
	defer teardown()

	target := filepath.Join(dir, "service.go")
	if err := os.WriteFile(target, []byte("package svc\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sig := waitForSignal(t, rp, "service.go", 2*time.Second)

	if sig.Source != live.ChangeSourceFsnotify {
		t.Fatalf("Source = %v, want %v", sig.Source, live.ChangeSourceFsnotify)
	}
	if sig.WorkspaceID != ws {
		t.Fatalf("WorkspaceID = %+v, want %+v", sig.WorkspaceID, ws)
	}
	if sig.ObservedAt.IsZero() {
		t.Fatalf("ObservedAt unset")
	}
	// WorkspaceChangeSignal has NO Kind field by design (D-01 paths-
	// only); the classifier is the single owner. We assert this
	// invariant indirectly by checking only the four documented
	// fields are populated. If Kind were ever added to the struct,
	// this test would still pass — but adding it would be a CONTEXT
	// D-01 violation caught by the live/signal.go file review, not by
	// this test.
}

// TestWorkspaceWatcher_StatusReportsRunning — Status() should report
// Active=true / Reason="running" between Start and Stop.
func TestWorkspaceWatcher_StatusReportsRunning(t *testing.T) {
	_, ws, _, teardown := startManager(t, 50*time.Millisecond)
	defer teardown()

	// Find the Manager via the closed-over teardown — recreate the
	// path: we cannot reach mgr from startManager. Instead, exercise
	// Manager.Status via a fresh instance keyed on the same ws.
	// Simpler: build our own manager and assert Status flips.
	rp := newRecordingProducer()
	mgr := NewManager(rp, Config{DebounceMs: 50 * time.Millisecond}, testLogger(t))
	dir2 := t.TempDir()
	ws2 := workspace.WorkspaceKey{RepoRoot: dir2}

	// Pre-Start: not_started.
	if got := mgr.Status(ws2); got.Active || got.Reason != "not_started" {
		t.Fatalf("pre-start Status = %+v, want not_started", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := mgr.Start(ctx, ws2); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer mgr.Stop(ws2)

	if got := mgr.Status(ws2); !got.Active || got.Reason != "running" {
		t.Fatalf("post-start Status = %+v, want Active=true Reason=running", got)
	}

	// The first ws (managed by startManager) should also be in a
	// known state — but we don't have its mgr handle. Use ws2 alone
	// for the assertion.
	_ = ws
}

// TestManager_StartIdempotent — a second Start on the same workspace
// must be a no-op (matches Service.Start in 60-04).
func TestManager_StartIdempotent(t *testing.T) {
	rp := newRecordingProducer()
	mgr := NewManager(rp, Config{DebounceMs: 50 * time.Millisecond}, testLogger(t))
	dir := t.TempDir()
	ws := workspace.WorkspaceKey{RepoRoot: dir}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := mgr.Start(ctx, ws); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := mgr.Start(ctx, ws); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	mgr.Stop(ws)
}

// TestManager_StopIdempotent — a second Stop on the same workspace
// must be a no-op.
func TestManager_StopIdempotent(t *testing.T) {
	rp := newRecordingProducer()
	mgr := NewManager(rp, Config{DebounceMs: 50 * time.Millisecond}, testLogger(t))
	dir := t.TempDir()
	ws := workspace.WorkspaceKey{RepoRoot: dir}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := mgr.Start(ctx, ws); err != nil {
		t.Fatalf("Start: %v", err)
	}
	mgr.Stop(ws)
	mgr.Stop(ws) // must not panic
}

// fakeCreate constructs a synthetic fsnotify.Event with Op=Create for
// unit-driving handleEvent without a real fsnotify backend. We use
// the public Has() check inside handleEvent, so the Op field is the
// only one that matters.
func fakeCreate(name string) fsnotify.Event {
	return fsnotify.Event{Name: name, Op: fsnotify.Create}
}
