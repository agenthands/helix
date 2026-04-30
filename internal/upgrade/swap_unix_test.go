//go:build !windows

package upgrade

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSwapUnixSameFsRename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	current := filepath.Join(dir, "helix")
	newBin := filepath.Join(dir, "helix.new")

	if err := os.WriteFile(current, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("seed current: %v", err)
	}
	if err := os.WriteFile(newBin, []byte("NEW"), 0o755); err != nil {
		t.Fatalf("seed new: %v", err)
	}

	// Simulate a running process keeping the old inode alive via an open FD.
	f, err := os.Open(current)
	if err != nil {
		t.Fatalf("open old inode: %v", err)
	}
	defer func() { _ = f.Close() }()

	if err := swap(current, newBin); err != nil {
		t.Fatalf("swap: %v", err)
	}

	// New content lands at current path.
	got, err := os.ReadFile(current)
	if err != nil {
		t.Fatalf("read after swap: %v", err)
	}
	if string(got) != "NEW" {
		t.Errorf("after swap, current = %q, want NEW", string(got))
	}

	// Old FD still serves the old bytes (POSIX inode persistence).
	buf := make([]byte, 3)
	if _, err := f.ReadAt(buf, 0); err != nil {
		t.Fatalf("read old FD: %v", err)
	}
	if string(buf) != "OLD" {
		t.Errorf("old FD reads %q, want OLD (inode persistence broken)", string(buf))
	}

	// New file path should be gone (rename, not copy).
	if _, err := os.Stat(newBin); !os.IsNotExist(err) {
		t.Errorf("newBin survived swap: err=%v", err)
	}
}
