package live_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live"
)

// stubLookup is a deterministic FileHashLookup. The known map records
// (path → hash) for paths the store "remembers"; absent paths are unknown.
type stubLookup struct {
	known map[string]string
	err   error
}

func (s stubLookup) EffectiveContentHash(_ context.Context, _ semantic.RepoID, path string) (string, bool, error) {
	if s.err != nil {
		return "", false, s.err
	}
	h, ok := s.known[path]
	return h, ok, nil
}

func stubHasher(_ string) (string, error) { return "deadbeef", nil }

func TestClassifyPathChange_HelixEditShortCircuits(t *testing.T) {
	// Source=helix_edit MUST short-circuit without I/O — the kernel hook
	// fires AFTER the on-disk write, so the file may even be gone (renamed
	// in a follow-up tool call) and we still want the helix_edit signal.
	kind, ok, err := live.ClassifyPathChange(
		context.Background(),
		"ws1",
		"/nonexistent/path/that/should/not/be/stat-ed.go",
		stubLookup{known: nil},
		stubHasher,
		live.ChangeSourceHelixEdit,
	)
	if err != nil {
		t.Fatalf("helix_edit should not error: %v", err)
	}
	if !ok || kind != live.ChangeHelixEdit {
		t.Fatalf("helix_edit: got (%v, %v), want (ChangeHelixEdit, true)", kind, ok)
	}
}

func TestClassifyPathChange_PathExistsUnknown_ReturnsCreated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.go")
	if err := os.WriteFile(path, []byte("package x"), 0o644); err != nil {
		t.Fatal(err)
	}
	kind, ok, err := live.ClassifyPathChange(
		context.Background(),
		"ws1",
		path,
		stubLookup{known: nil},
		stubHasher,
		live.ChangeSourceFsnotify,
	)
	if err != nil || !ok || kind != live.ChangeFileCreated {
		t.Fatalf("got (%v, %v, %v), want (ChangeFileCreated, true, nil)", kind, ok, err)
	}
}

func TestClassifyPathChange_PathExistsKnown_ReturnsModified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.go")
	if err := os.WriteFile(path, []byte("package x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lookup := stubLookup{known: map[string]string{path: "previous-hash"}}
	kind, ok, err := live.ClassifyPathChange(
		context.Background(),
		"ws1",
		path,
		lookup,
		stubHasher,
		live.ChangeSourceFsnotify,
	)
	if err != nil || !ok || kind != live.ChangeFileModified {
		t.Fatalf("got (%v, %v, %v), want (ChangeFileModified, true, nil)", kind, ok, err)
	}
}

func TestClassifyPathChange_PathMissingKnown_ReturnsDeleted(t *testing.T) {
	path := "/tmp/definitely/does/not/exist/file_60_04.go"
	lookup := stubLookup{known: map[string]string{path: "hash"}}
	kind, ok, err := live.ClassifyPathChange(
		context.Background(),
		"ws1",
		path,
		lookup,
		stubHasher,
		live.ChangeSourceFsnotify,
	)
	if err != nil || !ok || kind != live.ChangeFileDeleted {
		t.Fatalf("got (%v, %v, %v), want (ChangeFileDeleted, true, nil)", kind, ok, err)
	}
}

func TestClassifyPathChange_PathMissingUnknown_NoOp(t *testing.T) {
	path := "/tmp/never/existed/file.go"
	kind, ok, err := live.ClassifyPathChange(
		context.Background(),
		"ws1",
		path,
		stubLookup{known: nil},
		stubHasher,
		live.ChangeSourceFsnotify,
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Fatalf("missing+unknown: expected ok=false (no-op), got kind=%v", kind)
	}
}

func TestClassifyPathChange_LookupError_Propagates(t *testing.T) {
	want := errors.New("store down")
	dir := t.TempDir()
	path := filepath.Join(dir, "x.go")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := live.ClassifyPathChange(
		context.Background(),
		"ws1",
		path,
		stubLookup{err: want},
		stubHasher,
		live.ChangeSourceFsnotify,
	)
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

// TestClassifyPathChange_RejectsSymlinkLikeBehavior — we use os.Lstat (not
// os.Stat) so the classifier classifies a dangling symlink as missing. This
// is the EXECUTOR DECISION called out in the threat model (T-60-04-03):
// reject symlink-escape by refusing to chase the link.
func TestClassifyPathChange_DanglingSymlinkClassifiedAsMissing(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "does_not_exist")
	link := filepath.Join(dir, "dangling_link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks not supported on this fs: %v", err)
	}
	// The symlink itself "exists" via Lstat but its target does not.
	// Classifier with lookup unaware → no-op (not Created).
	kind, ok, err := live.ClassifyPathChange(
		context.Background(),
		"ws1",
		link,
		stubLookup{known: nil},
		stubHasher,
		live.ChangeSourceFsnotify,
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// Either:
	//   - Lstat says symlink exists, classifier says ChangeFileCreated; OR
	//   - we explicitly reject symlinks and treat as missing → no-op
	// We pick (b) per T-60-04-03 to match the existing repomap walker.
	if ok && kind != live.ChangeFileDeleted {
		t.Fatalf("dangling symlink should be a no-op (or deleted if known); got kind=%v ok=%v",
			kind, ok)
	}
}
