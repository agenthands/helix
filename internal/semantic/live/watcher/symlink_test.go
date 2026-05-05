package watcher

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/agenthands/helix/internal/workspace"
)

// CR-02 regression: invariant #6 says "both watcher and scanner must
// skip symlinks". Pre-fix, addRecursive only checked d.IsDir() and
// added symlinked subdirectories to fsnotify, watching wherever the
// link pointed (potentially out of workspace). Post-fix, the WalkDir
// callback rejects entries whose Type carries fs.ModeSymlink before
// calling fw.Add.
//
// The test creates a real workspace with a symlinked subdirectory
// pointing outside the workspace and asserts the watch list does NOT
// include the linked target.
func TestAddRecursive_SkipsSymlinkedDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows")
	}

	wsRoot := t.TempDir()
	outside := t.TempDir() // a separate dir to be the symlink target

	// Create a real subdirectory under the workspace root so we can
	// confirm the watcher DOES add a normal subdir.
	realSub := filepath.Join(wsRoot, "real_subdir")
	if err := os.Mkdir(realSub, 0o755); err != nil {
		t.Fatalf("mkdir real_subdir: %v", err)
	}

	// Create a symlinked subdirectory pointing to the outside dir. The
	// fix MUST NOT call fw.Add on this path.
	linkPath := filepath.Join(wsRoot, "link_to_outside")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	ws := workspace.WorkspaceKey{RepoRoot: wsRoot}
	cfg := applyDefaults(Config{})
	rp := newRecordingProducer()
	ww, err := newWorkspaceWatcher(ws, cfg, rp, testLogger(t))
	if err != nil {
		t.Fatalf("newWorkspaceWatcher: %v", err)
	}
	t.Cleanup(func() { _ = ww.fw.Close() })

	// Inspect the active fsnotify watch list. fsnotify's WatchList()
	// returns the absolute paths currently registered.
	watchList := ww.fw.WatchList()
	hasReal, hasLink, hasOutside := false, false, false
	for _, w := range watchList {
		switch w {
		case realSub:
			hasReal = true
		case linkPath:
			hasLink = true
		case outside:
			hasOutside = true
		}
	}

	if !hasReal {
		t.Errorf("expected fw.WatchList to include real subdir %q, got %v", realSub, watchList)
	}
	if hasLink {
		t.Errorf("CR-02 regression: fw.WatchList includes symlink path %q (invariant #6 violation): %v", linkPath, watchList)
	}
	if hasOutside {
		t.Errorf("CR-02 regression: fw.WatchList includes symlink TARGET %q outside workspace (invariant #6 violation): %v", outside, watchList)
	}
}
