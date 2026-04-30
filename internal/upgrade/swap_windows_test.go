//go:build windows

package upgrade

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSwapWindowsRenameToOld(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "helix.exe")
	newBin := filepath.Join(dir, "helix.new.exe")

	if err := os.WriteFile(current, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("seed current: %v", err)
	}
	if err := os.WriteFile(newBin, []byte("NEW"), 0o755); err != nil {
		t.Fatalf("seed new: %v", err)
	}

	if err := swap(current, newBin); err != nil {
		t.Fatalf("swap: %v", err)
	}

	got, err := os.ReadFile(current)
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	if string(got) != "NEW" {
		t.Errorf("after swap, current = %q, want NEW", string(got))
	}

	// .old should exist and contain OLD bytes.
	old, err := os.ReadFile(current + ".old")
	if err != nil {
		t.Fatalf("read .old: %v", err)
	}
	if string(old) != "OLD" {
		t.Errorf(".old content = %q, want OLD", string(old))
	}
}
