package upgrade

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProbeWritableSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// installPath is a hypothetical binary in dir (need not exist; ProbeWritable
	// only inspects the parent directory).
	installPath := filepath.Join(dir, "helix")
	if err := ProbeWritable(installPath); err != nil {
		t.Fatalf("ProbeWritable(%q) = %v, want nil", installPath, err)
	}
}

func TestProbeWritableFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows ACLs make 0o555 a no-op for the file owner; the chmod-based
		// unwritable simulation is unix-only. Per the plan, skip on Windows.
		t.Skip("chmod-based unwritable test is unix-only")
	}
	dir := t.TempDir()
	installPath := filepath.Join(dir, "helix")

	// Make the directory non-writable. t.TempDir's cleanup runs after the
	// test as long as the dir is writable — but since t.TempDir created the
	// dir owned by us, we restore mode in t.Cleanup so cleanup succeeds.
	t.Cleanup(func() {
		// Restore writability so t.TempDir cleanup can rmdir.
		_ = chmod(dir, 0o755)
	})
	if err := chmod(dir, 0o555); err != nil {
		t.Fatalf("setup chmod(0o555) failed: %v", err)
	}

	err := ProbeWritable(installPath)
	if err == nil {
		t.Fatalf("ProbeWritable(%q) = nil, want error", installPath)
	}
}

func TestSudoHint(t *testing.T) {
	t.Parallel()
	hint := SudoHint("/usr/local/bin/helix", []string{"--prerelease"})
	if !strings.Contains(hint, "/usr/local/bin/helix") {
		t.Errorf("hint missing install path: %q", hint)
	}
	if !strings.Contains(hint, "sudo helix upgrade --prerelease") {
		t.Errorf("hint missing sudo re-invocation: %q", hint)
	}
	if !strings.Contains(hint, "cannot write") {
		t.Errorf("hint missing user-actionable phrase: %q", hint)
	}
}

func TestSudoHintEmptyArgs(t *testing.T) {
	t.Parallel()
	hint := SudoHint("/usr/local/bin/helix", nil)
	if !strings.Contains(hint, "sudo helix upgrade") {
		t.Errorf("hint missing sudo command on empty args: %q", hint)
	}
}
