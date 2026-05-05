package scanner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/workspace"
)

// recordingProducer captures every WorkspaceChangeSignal the scanner emits
// for assertion. Safe for concurrent OnWorkspaceChanged from a single
// scanner goroutine.
type recordingProducer struct {
	mu   sync.Mutex
	sigs []live.WorkspaceChangeSignal
}

func (r *recordingProducer) OnWorkspaceChanged(_ context.Context, sig live.WorkspaceChangeSignal) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Copy paths slice so callers can mutate the source without poisoning
	// our recording.
	cp := make([]string, len(sig.Paths))
	copy(cp, sig.Paths)
	sig.Paths = cp
	r.sigs = append(r.sigs, sig)
	return nil
}

func (r *recordingProducer) snapshot() []live.WorkspaceChangeSignal {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]live.WorkspaceChangeSignal, len(r.sigs))
	copy(out, r.sigs)
	return out
}

// stubLookup returns a static known-files map for the scanner.
type stubLookup struct {
	files map[string]string
}

func (s *stubLookup) KnownFiles(_ context.Context, _ semantic.RepoID) (map[string]string, error) {
	out := make(map[string]string, len(s.files))
	for k, v := range s.files {
		out[k] = v
	}
	return out, nil
}

func testLogger(_ *testing.T) *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func pathInList(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

// TestScanner_DetectsNewFile verifies the immediate-first-scan behavior:
// a fresh tmpdir with one file and an empty store yields a created-event
// signal within the test budget.
func TestScanner_DetectsNewFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rp := &recordingProducer{}
	sc := New(
		workspace.WorkspaceKey{RepoRoot: dir},
		semantic.RepoID("ws1"),
		rp,
		&stubLookup{files: map[string]string{}},
		Config{Interval: 50 * time.Millisecond},
		testLogger(t),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = sc.Run(ctx)

	sigs := rp.snapshot()
	if len(sigs) == 0 {
		t.Fatal("expected at least one signal")
	}
	if sigs[0].Source != live.ChangeSourceManifestScan {
		t.Fatalf("source=%v, want manifest_scan", sigs[0].Source)
	}
	if !pathInList(sigs[0].Paths, filepath.Join(dir, "a.go")) {
		t.Fatalf("paths=%v missing a.go", sigs[0].Paths)
	}
}

// TestScannerCatchesWatcherMisses is the load-bearing acceptance #10
// invariant: the scanner detects an off-watcher edit within 2× interval.
func TestScannerCatchesWatcherMisses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatalf("WriteFile v1: %v", err)
	}
	oldHash, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	// Off-watcher edit: change content WITHOUT going through the kernel
	// hook OR an fsnotify event. The scanner is the only producer that
	// can detect this.
	if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
		t.Fatalf("WriteFile v2: %v", err)
	}
	rp := &recordingProducer{}
	sc := New(
		workspace.WorkspaceKey{RepoRoot: dir},
		semantic.RepoID("ws1"),
		rp,
		&stubLookup{files: map[string]string{path: oldHash}},
		Config{Interval: 200 * time.Millisecond},
		testLogger(t),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = sc.Run(ctx)

	sigs := rp.snapshot()
	if len(sigs) == 0 {
		t.Fatal("scanner did not detect the off-watcher edit within 500ms (acceptance #10)")
	}
	if !pathInList(sigs[0].Paths, path) {
		t.Fatalf("paths=%v missing %s", sigs[0].Paths, path)
	}
	if sigs[0].Source != live.ChangeSourceManifestScan {
		t.Fatalf("source=%v, want manifest_scan", sigs[0].Source)
	}
}

// TestScanner_DetectsDeletion verifies that a path present in the store
// but absent from disk surfaces in the changed-paths slice — the
// deletion-detection branch (HandleFileDeleted upstream consumer).
func TestScanner_DetectsDeletion(t *testing.T) {
	dir := t.TempDir()
	gone := filepath.Join(dir, "deleted.go")

	rp := &recordingProducer{}
	sc := New(
		workspace.WorkspaceKey{RepoRoot: dir},
		semantic.RepoID("ws1"),
		rp,
		&stubLookup{files: map[string]string{gone: "deadbeef00000000"}},
		Config{Interval: 50 * time.Millisecond},
		testLogger(t),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_ = sc.Run(ctx)

	sigs := rp.snapshot()
	if len(sigs) == 0 {
		t.Fatal("expected deletion signal")
	}
	if !pathInList(sigs[0].Paths, gone) {
		t.Fatalf("paths=%v missing deleted path %s", sigs[0].Paths, gone)
	}
}

// TestScanner_NoDiffNoSignal asserts the no-op-flush invariant: when the
// scan finds zero changed paths, the producer is NOT called. This is the
// load-bearing 60-04 acceptance #8 mirror at the producer side.
func TestScanner_NoDiffNoSignal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	hash, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}

	rp := &recordingProducer{}
	sc := New(
		workspace.WorkspaceKey{RepoRoot: dir},
		semantic.RepoID("ws1"),
		rp,
		&stubLookup{files: map[string]string{path: hash}},
		Config{Interval: 50 * time.Millisecond},
		testLogger(t),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Millisecond)
	defer cancel()
	_ = sc.Run(ctx)

	if got := len(rp.snapshot()); got != 0 {
		t.Fatalf("expected zero signals on clean diff, got %d", got)
	}
}

// TestWalk_SkipsDotGit asserts skipDirs is honored — files under .git/
// must not surface to the producer.
func TestWalk_SkipsDotGit(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: foo"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	var seen []string
	if err := Walk(dir, func(p string) error {
		seen = append(seen, p)
		return nil
	}); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	for _, p := range seen {
		if filepath.Base(filepath.Dir(p)) == ".git" {
			t.Errorf("Walk surfaced .git child: %s", p)
		}
	}
	// And the regular file IS surfaced.
	if !pathInList(seen, filepath.Join(dir, "a.go")) {
		t.Fatalf("Walk did not surface a.go: %v", seen)
	}
}

// TestManager_StartStopIdempotent exercises the per-workspace lifecycle.
func TestManager_StartStopIdempotent(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(
		&recordingProducer{},
		&stubLookup{files: map[string]string{}},
		func(ws workspace.WorkspaceKey) semantic.RepoID { return semantic.RepoID(ws.RepoRoot) },
		Config{Interval: time.Second},
		testLogger(t),
	)
	ws := workspace.WorkspaceKey{RepoRoot: dir}
	if err := mgr.Start(context.Background(), ws); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Second Start is a no-op.
	if err := mgr.Start(context.Background(), ws); err != nil {
		t.Fatalf("Start (second): %v", err)
	}
	mgr.Stop(ws)
	mgr.Stop(ws) // no-op
}

// TestHashFile_StableForSameContent locks the xxhash64 invariant so
// downstream content-hash comparisons (classifier + scanner) cannot drift.
func TestHashFile_StableForSameContent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	content := []byte("hello, helix\n")
	if err := os.WriteFile(a, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, content, 0o644); err != nil {
		t.Fatal(err)
	}
	ha, err := HashFile(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := HashFile(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("HashFile not stable across paths: %s != %s", ha, hb)
	}
	if len(ha) != 16 {
		t.Fatalf("HashFile digest length = %d, want 16", len(ha))
	}
}
