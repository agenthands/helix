package patch_validator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// gitInitRepo creates a git repo in t.TempDir() with n committed tracked files
// (f0.txt … f{n-1}.txt), each containing a single line. Returns the repo dir.
func gitInitRepo(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	for i := 0; i < n; i++ {
		name := "f" + strconv.Itoa(i) + ".txt"
		if err := os.WriteFile(filepath.Join(dir, name), []byte("line0\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		runGit(t, dir, "add", name)
	}
	if n > 0 {
		runGit(t, dir, "commit", "-m", "init")
	}
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestEditLocality(t *testing.T) {
	ctx := context.Background()

	t.Run("root-only edit → locality > 0.9", func(t *testing.T) {
		dir := gitInitRepo(t, 20)
		writeFile(t, dir, "f0.txt", "changed\n")
		loc, mod, mErr := EditLocality(ctx, dir)
		if mErr != nil {
			t.Fatalf("unexpected MetricError: %+v", mErr)
		}
		if loc == nil {
			t.Fatalf("locality nil, want value")
		}
		if *loc <= 0.9 {
			t.Fatalf("edit_locality = %v, want > 0.9", *loc)
		}
		if mod == nil || *mod != 1 {
			t.Fatalf("files_modified = %v, want 1", mod)
		}
	})

	t.Run("all-files edit → locality == 0.0", func(t *testing.T) {
		n := 5
		dir := gitInitRepo(t, n)
		for i := 0; i < n; i++ {
			writeFile(t, dir, "f"+strconv.Itoa(i)+".txt", "changed\n")
		}
		loc, mod, mErr := EditLocality(ctx, dir)
		if mErr != nil {
			t.Fatalf("unexpected MetricError: %+v", mErr)
		}
		if loc == nil || *loc != 0.0 {
			t.Fatalf("edit_locality = %v, want 0.0", loc)
		}
		if mod == nil || *mod != n {
			t.Fatalf("files_modified = %v, want %d", mod, n)
		}
	})

	t.Run("zero tracked files → nil locality + MetricError", func(t *testing.T) {
		dir := t.TempDir()
		runGit(t, dir, "init")
		runGit(t, dir, "config", "user.email", "test@example.com")
		runGit(t, dir, "config", "user.name", "test")
		loc, _, mErr := EditLocality(ctx, dir)
		if loc != nil {
			t.Fatalf("edit_locality = %v, want nil", *loc)
		}
		if mErr == nil {
			t.Fatalf("want MetricError for undefined denominator, got nil")
		}
		if mErr.Metric != "edit_locality" || mErr.Grader != "patch_validator" {
			t.Fatalf("MetricError = %+v, want metric=edit_locality grader=patch_validator", mErr)
		}
	})

	t.Run("untracked file excluded from denominator", func(t *testing.T) {
		dir := gitInitRepo(t, 4)
		writeFile(t, dir, "f0.txt", "changed\n")
		// an untracked scratch file: must NOT change the tracked denominator (4).
		writeFile(t, dir, "scratch.tmp", "junk\n")
		loc, mod, mErr := EditLocality(ctx, dir)
		if mErr != nil {
			t.Fatalf("unexpected MetricError: %+v", mErr)
		}
		// denominator stays 4 tracked, numerator 1 → 1 - 1/4 = 0.75
		if loc == nil || *loc != 0.75 {
			t.Fatalf("edit_locality = %v, want 0.75 (untracked excluded)", loc)
		}
		if mod == nil || *mod != 1 {
			t.Fatalf("files_modified = %v, want 1 (untracked not counted)", mod)
		}
	})
}

func TestEditDistancePatch(t *testing.T) {
	ctx := context.Background()
	dir := gitInitRepo(t, 1)
	// f0.txt is "line0\n" (1 line). Replace with 3 lines and add nothing else:
	// git diff --numstat reports added/deleted line counts. Changing 1 line to 3
	// lines = 1 deleted + 3 added = 4. To make a clean 3-added + 2-deleted case,
	// start from a 2-line file.
	writeFile(t, dir, "f0.txt", "a\nb\n")
	runGit(t, dir, "add", "f0.txt")
	runGit(t, dir, "commit", "-m", "two lines")
	// now mutate: delete both lines, add 3 new ones → numstat 3 added, 2 deleted = 5
	writeFile(t, dir, "f0.txt", "x\ny\nz\n")
	dist, mErr := EditDistancePatch(ctx, dir)
	if mErr != nil {
		t.Fatalf("unexpected MetricError: %+v", mErr)
	}
	if dist == nil || *dist != 5 {
		t.Fatalf("edit_distance_patch = %v, want 5 (3 added + 2 deleted)", dist)
	}
}

// TestSumNumstat covers LO-03: the binary sentinel ("-\t-") contributes 0
// silently, but any OTHER non-integer numstat field is a parse anomaly that
// surfaces a MetricError rather than being silently dropped.
func TestSumNumstat(t *testing.T) {
	t.Run("text lines sum added+deleted", func(t *testing.T) {
		dist, mErr := sumNumstat([]string{"3\t2\tf.go", "1\t0\tg.go"})
		if mErr != nil {
			t.Fatalf("unexpected MetricError: %+v", mErr)
		}
		if dist == nil || *dist != 6 {
			t.Fatalf("sum = %v, want 6", dist)
		}
	})

	t.Run("binary sentinel contributes 0, no error", func(t *testing.T) {
		dist, mErr := sumNumstat([]string{"-\t-\tbin.dat", "4\t1\tf.go"})
		if mErr != nil {
			t.Fatalf("binary sentinel must not error: %+v", mErr)
		}
		if dist == nil || *dist != 5 {
			t.Fatalf("sum = %v, want 5 (binary contributes 0)", dist)
		}
	})

	t.Run("malformed line surfaces a MetricError", func(t *testing.T) {
		dist, mErr := sumNumstat([]string{"3\t2\tf.go", "garbage\t1\th.go"})
		if dist != nil {
			t.Fatalf("malformed line must null the metric, got %v", *dist)
		}
		if mErr == nil {
			t.Fatal("expected a MetricError for a malformed numstat line")
		}
		if mErr.Metric != "edit_distance_patch" || mErr.Grader != graderName {
			t.Fatalf("MetricError = %+v, want metric=edit_distance_patch grader=%s", mErr, graderName)
		}
	})
}
