//go:build editor
// +build editor

package watcher_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/watcher"
	"github.com/agenthands/helix/internal/workspace"
)

// repoRoot resolves the repository root by climbing from the current
// test source file's directory. Editor-fixture tests need to invoke
// `go run` against testdata Go programs whose import paths only
// resolve from the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	// Climb from cwd (which is the package dir) until we find go.mod.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate go.mod from %s", dir)
	return ""
}

// recordingProducer is the editor-fixture-side fake; mirrors the one
// in watcher_test.go (kept duplicated to avoid an extra _internal
// helper export — these tests live in watcher_test package).
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

func testLogger(_ testing.TB) *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// setupWatcher spins up a Manager around a t.TempDir() workspace,
// seeds an initial empty file so editor saves have something to
// rename onto, and returns the target path + producer + teardown.
func setupWatcher(t *testing.T) (string, *recordingProducer, func()) {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "auth.go")
	if err := os.WriteFile(target, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	rp := newRecordingProducer()
	mgr := watcher.NewManager(rp, watcher.Config{DebounceMs: 100 * time.Millisecond}, testLogger(t))
	ws := workspace.WorkspaceKey{RepoRoot: dir}

	ctx, cancel := context.WithCancel(context.Background())
	if err := mgr.Start(ctx, ws); err != nil {
		cancel()
		t.Fatalf("Manager.Start: %v", err)
	}

	// Give the fsnotify backend a beat to install the inotify/FSEvents
	// watch on the directory before the editor program starts mutating
	// it. Without this the first event can race the Add() syscall and
	// the test will time out.
	time.Sleep(50 * time.Millisecond)

	return target, rp, func() {
		cancel()
		mgr.Stop(ws)
	}
}

// buildFixture compiles a single-file Go program from testdata into
// a tmpdir-rooted binary and returns the absolute path. We build
// once per test (cheap; the programs are <60 LOC) rather than `go
// run` because go-run mis-parses `target.go program-args` patterns
// when the program's argv looks file-like.
func buildFixture(t *testing.T, root, srcRel string) string {
	t.Helper()
	src := filepath.Join(root, srcRel)
	bin := filepath.Join(t.TempDir(), "fixture")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", srcRel, err, string(out))
	}
	return bin
}

// expectSignalForBase blocks until a signal lands containing a path
// whose Base matches wantBase, OR fails the test on timeout.
func expectSignalForBase(t *testing.T, rp *recordingProducer, wantBase string, timeout time.Duration) live.WorkspaceChangeSignal {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case sig := <-rp.ch:
			for _, p := range sig.Paths {
				if filepath.Base(p) == wantBase {
					return sig
				}
			}
		case <-deadline:
			t.Fatalf("timeout %v waiting for signal containing base %q; got %d signals: %+v",
				timeout, wantBase, len(rp.snapshot()), rp.snapshot())
		}
	}
}

// TestEditorFixtures_VimSwapRename — drives the Vim shell fixture
// against a live watcher; expects at least one signal whose Paths
// contain the target file by basename.
func TestEditorFixtures_VimSwapRename(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("vim fixture is bash; skipping on Windows")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}

	root := repoRoot(t)
	target, rp, teardown := setupWatcher(t)
	defer teardown()

	script := filepath.Join(root, "internal", "semantic", "live", "testdata", "editors", "vim", "save.sh")
	cmd := exec.Command("bash", script, target, "package main\n// modified by vim\n")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("vim save: %v\n%s", err, string(out))
	}

	sig := expectSignalForBase(t, rp, "auth.go", 3*time.Second)
	if sig.Source != live.ChangeSourceFsnotify {
		t.Fatalf("Source=%v, want fsnotify", sig.Source)
	}
}

// TestEditorFixtures_JetBrainsSafeWrite — drives the JetBrains Go
// fixture; the watcher MUST filter the ___jb_tmp___ and ___jb_old___
// suffixes so the resulting Paths contain `auth.go` (and ONLY
// `auth.go`, not the suffix variants).
func TestEditorFixtures_JetBrainsSafeWrite(t *testing.T) {
	root := repoRoot(t)
	target, rp, teardown := setupWatcher(t)
	defer teardown()

	bin := buildFixture(t, root, filepath.Join("internal", "semantic", "live", "testdata", "editors", "jetbrains", "save.go"))
	cmd := exec.Command(bin, target, "package main\n// modified by jetbrains\n")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("jetbrains save: %v\n%s", err, string(out))
	}

	sig := expectSignalForBase(t, rp, "auth.go", 5*time.Second)
	if sig.Source != live.ChangeSourceFsnotify {
		t.Fatalf("Source=%v, want fsnotify", sig.Source)
	}

	// Critical: NONE of the signal paths should carry the JetBrains
	// suffixes — handleEvent must have filtered them at the watcher
	// loop, BEFORE they reached the producer.
	for _, sig := range rp.snapshot() {
		for _, p := range sig.Paths {
			base := filepath.Base(p)
			if base == "auth.go___jb_tmp___" || base == "auth.go___jb_old___" {
				t.Fatalf("JetBrains tempfile suffix leaked through to producer: %s", p)
			}
		}
	}
}

// TestEditorFixtures_VSCodeAtomic — drives the VS Code atomic-rename
// fixture (sibling temp file + rename). The directory-level watch
// MUST survive the rename and emit a signal with Paths containing
// the target.
func TestEditorFixtures_VSCodeAtomic(t *testing.T) {
	root := repoRoot(t)
	target, rp, teardown := setupWatcher(t)
	defer teardown()

	bin := buildFixture(t, root, filepath.Join("internal", "semantic", "live", "testdata", "editors", "vscode", "save_atomic.go"))
	cmd := exec.Command(bin, target, "package main\n// modified by vscode atomic\n")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("vscode atomic save: %v\n%s", err, string(out))
	}

	sig := expectSignalForBase(t, rp, "auth.go", 5*time.Second)
	if sig.Source != live.ChangeSourceFsnotify {
		t.Fatalf("Source=%v, want fsnotify", sig.Source)
	}
}

// TestEditorFixtures_VSCodeTruncate — drives the VS Code default
// truncate-write path. Simplest case — direct WRITE on the target.
func TestEditorFixtures_VSCodeTruncate(t *testing.T) {
	root := repoRoot(t)
	target, rp, teardown := setupWatcher(t)
	defer teardown()

	bin := buildFixture(t, root, filepath.Join("internal", "semantic", "live", "testdata", "editors", "vscode", "save_truncate.go"))
	cmd := exec.Command(bin, target, "package main\n// modified by vscode truncate\n")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("vscode truncate save: %v\n%s", err, string(out))
	}

	sig := expectSignalForBase(t, rp, "auth.go", 5*time.Second)
	if sig.Source != live.ChangeSourceFsnotify {
		t.Fatalf("Source=%v, want fsnotify", sig.Source)
	}
}
