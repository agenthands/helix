package runner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/eval/runner"
)

// TestZDRGate_SyntheticOK verifies that a corpus with all synthetic tasks
// (no source.yaml or source.yaml with source: synthetic) is allowed
// without any env-var.
func TestZDRGate_SyntheticOK(t *testing.T) {
	t.Setenv("HELIX_EVAL_ZDR_VERIFIED", "")
	dir := t.TempDir()

	// Task with no source.yaml → defaults to synthetic.
	taskDir := filepath.Join(dir, "task-synthetic")
	if err := os.MkdirAll(taskDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Write a task.md so it's recognised as a valid task directory.
	if err := os.WriteFile(filepath.Join(taskDir, "task.md"), []byte("# task"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := runner.AssertCorpusAllowed(dir); err != nil {
		t.Fatalf("expected nil error for synthetic corpus, got: %v", err)
	}
}

// TestZDRGate_HelixOSSAllowed verifies that a corpus path that resolves
// under the helix repo root is allowed without env-var (OSS Helix secondary
// source per EVAL-06).
func TestZDRGate_HelixOSSAllowed(t *testing.T) {
	t.Setenv("HELIX_EVAL_ZDR_VERIFIED", "")

	// Use eval/corpus (inside the repo) as the corpus dir.
	// The marker file is go.mod in the module root.
	// We just need to verify a path that contains a .helix-repo-marker or go.mod.
	repoRoot := filepath.Join(os.Getenv("GOPATH"), "src", "github.com", "agenthands", "helix")
	if _, err := os.Stat(repoRoot); err != nil {
		// Try relative to test file.
		repoRoot = findRepoRoot(t)
	}

	corpusPath := filepath.Join(repoRoot, "eval", "corpus")
	if err := os.MkdirAll(corpusPath, 0700); err != nil {
		t.Fatal(err)
	}

	if err := runner.AssertCorpusAllowed(corpusPath); err != nil {
		t.Fatalf("expected nil error for helix-OSS corpus path %q, got: %v", corpusPath, err)
	}
}

// TestZDRGate_NonSyntheticBlocked verifies that a corpus with an external
// task is REJECTED when HELIX_EVAL_ZDR_VERIFIED is not set.
func TestZDRGate_NonSyntheticBlocked(t *testing.T) {
	t.Setenv("HELIX_EVAL_ZDR_VERIFIED", "")
	dir := t.TempDir()

	// Task with source: external.
	taskDir := filepath.Join(dir, "task-external")
	if err := os.MkdirAll(taskDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "task.md"), []byte("# task"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "source.yaml"), []byte("source: external\n"), 0600); err != nil {
		t.Fatal(err)
	}

	err := runner.AssertCorpusAllowed(dir)
	if err == nil {
		t.Fatal("expected error for external corpus without ZDR env-var, got nil")
	}
}

// TestZDRGate_NonSyntheticAllowedWithEnvVar verifies that the same external
// task IS allowed when HELIX_EVAL_ZDR_VERIFIED=1 (with a WARN log emitted).
func TestZDRGate_NonSyntheticAllowedWithEnvVar(t *testing.T) {
	t.Setenv("HELIX_EVAL_ZDR_VERIFIED", "1")
	dir := t.TempDir()

	taskDir := filepath.Join(dir, "task-external")
	if err := os.MkdirAll(taskDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "task.md"), []byte("# task"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "source.yaml"), []byte("source: external\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := runner.AssertCorpusAllowed(dir); err != nil {
		t.Fatalf("expected nil error with HELIX_EVAL_ZDR_VERIFIED=1, got: %v", err)
	}
}

// findRepoRoot walks upward from the test binary's location to find go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod not found)")
		}
		dir = parent
	}
}
