package cochange

import (
	"context"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

// gitAvailable reports whether git is runnable in this environment.
func gitAvailable(t *testing.T) bool {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; skipping co-change miner integration test")
		return false
	}
	return true
}

// runGit runs git in dir with the given args, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// commitFiles writes the named files (with their content) into dir, stages
// everything, and commits. Files absent from the map are left untouched.
func commitFiles(t *testing.T, dir, msg string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if content == "" {
			// deletion signal handled by caller via runGit rm; here just skip.
			continue
		}
		if err := writeFile(path, content); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "--allow-empty", "-m", msg)
}

func writeFile(path, content string) error {
	return exec.Command("sh", "-c", "mkdir -p \"$(dirname \"$1\")\" && printf '%s' \"$2\" > \"$1\"", "_", path, content).Run()
}

// TestMine_CoChangePairs: three commits each touching a.go + b.go (coupled)
// plus one commit touching only c.go (isolated). The a/b pair must surface
// with count 3; c.go must not appear in any pair.
func TestMine_CoChangePairs(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "t@t")
	runGit(t, dir, "config", "user.name", "t")

	v := 0
	bump := func() string { v++; return string(rune('a' + v)) }
	commitFiles(t, dir, "c1", map[string]string{"a.go": bump(), "b.go": bump()})
	commitFiles(t, dir, "c2", map[string]string{"a.go": bump(), "b.go": bump()})
	commitFiles(t, dir, "c3", map[string]string{"a.go": bump(), "b.go": bump()})
	commitFiles(t, dir, "c4", map[string]string{"c.go": bump()}) // isolated

	got, err := MineWith(context.Background(), dir, 100, 2)
	if err != nil {
		t.Fatalf("Mine: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 co-change pair, got %d: %+v", len(got), got)
	}
	cc := got[0]
	pair := []string{cc.PathA, cc.PathB}
	sort.Strings(pair)
	if pair[0] != "a.go" || pair[1] != "b.go" {
		t.Errorf("expected {a.go,b.go}, got {%s,%s}", cc.PathA, cc.PathB)
	}
	if cc.Count != 3 {
		t.Errorf("count = %d, want 3", cc.Count)
	}
}

// TestMine_NotARepo: a non-git directory yields an error (best-effort contract).
func TestMine_NotARepo(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()
	if _, err := Mine(context.Background(), dir); err == nil {
		t.Fatal("expected error mining a non-git directory, got nil")
	}
}

// TestFileNodeID_Stable: the same (repoID, path) must produce the same node
// ID across calls, and distinct paths must produce distinct IDs.
func TestFileNodeID_Stable(t *testing.T) {
	a := FileNodeID("repo", "a/b.go")
	b := FileNodeID("repo", "a/b.go")
	if a != b {
		t.Errorf("FileNodeID not stable: %d != %d", a, b)
	}
	if a&0x8000000000000000 != 0 {
		t.Errorf("FileNodeID high bit set: %d", a)
	}
	c := FileNodeID("repo", "a/c.go")
	if a == c {
		t.Errorf("expected distinct IDs for distinct paths")
	}
}
