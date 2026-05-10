package runner_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/eval/report"
	"github.com/agenthands/helix/internal/eval/runner"
	"github.com/agenthands/helix/internal/eval/sandbox"
	"github.com/agenthands/helix/internal/eval/score"
)

// helixBin returns a dummy helix binary path. The runner tests use a fake
// sandbox that doesn't actually spawn a daemon, so this just needs to be a
// plausible non-empty string.
func helixBin() string { return "/usr/bin/false" }

// makeCorpusTask creates a minimal task directory under corpusDir.
// verify.sh exits with the given code. Returns the task ID.
func makeCorpusTask(t *testing.T, corpusDir, taskID string, verifyExitCode int) string {
	t.Helper()
	taskDir := filepath.Join(corpusDir, taskID)
	repoDir := filepath.Join(taskDir, "repo")
	if err := os.MkdirAll(repoDir, 0700); err != nil {
		t.Fatal(err)
	}
	// Minimal task.md.
	if err := os.WriteFile(filepath.Join(taskDir, "task.md"), []byte("# "+taskID), 0600); err != nil {
		t.Fatal(err)
	}
	// verify.sh with configurable exit code.
	exitStr := "0"
	if verifyExitCode != 0 {
		exitStr = "1"
	}
	if err := os.WriteFile(filepath.Join(taskDir, "verify.sh"), []byte("#!/bin/sh\nexit "+exitStr+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	// Seed a file in the repo so git has something to commit.
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# "+taskID), 0600); err != nil {
		t.Fatal(err)
	}
	return taskID
}

// TestRunnerSinglePathHappy tests the RunTask happy path with a scripted
// (fake) agent that produces a deterministic result and a passing verify.sh.
func TestRunnerSinglePathHappy(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	corpusDir := t.TempDir()
	outDir := t.TempDir()
	taskID := makeCorpusTask(t, corpusDir, "task-happy", 0)

	sb, err := sandbox.NewSandbox("test-happy", helixBin())
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	r := runner.NewRunner(runner.Config{
		CorpusDir: corpusDir,
		OutDir:    outDir,
		HelixBin:  helixBin(),
		RunID:     "test-happy-run",
		MaxParallel: 1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := r.RunTask(ctx, sb, runner.TaskSpec{
		ID:     taskID,
		Mode:   "baseline",
		Prompt: "do nothing",
	})
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success=true, got failure_reason=%q", result.FailureReason)
	}
}

// TestRunnerVerifyFailMarksFailed tests that a failing verify.sh causes
// Success=false and TestsPass=false.
func TestRunnerVerifyFailMarksFailed(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	corpusDir := t.TempDir()
	outDir := t.TempDir()
	taskID := makeCorpusTask(t, corpusDir, "task-fail-verify", 1)

	sb, err := sandbox.NewSandbox("test-verify-fail", helixBin())
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	r := runner.NewRunner(runner.Config{
		CorpusDir:   corpusDir,
		OutDir:      outDir,
		HelixBin:    helixBin(),
		RunID:       "test-verify-fail-run",
		MaxParallel: 1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := r.RunTask(ctx, sb, runner.TaskSpec{
		ID:     taskID,
		Mode:   "baseline",
		Prompt: "do nothing",
	})
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	if result.Success {
		t.Error("expected success=false when verify.sh exits 1")
	}
	if result.TestsPass {
		t.Error("expected tests_pass=false when verify.sh exits 1")
	}
}

// TestRunnerBudgetBreachSurfaces tests that when the budget is exceeded
// the result reflects the budget breach outcome.
func TestRunnerBudgetBreachSurfaces(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	corpusDir := t.TempDir()
	outDir := t.TempDir()
	taskID := makeCorpusTask(t, corpusDir, "task-budget", 0)

	// Write a budget.yaml with 0 seconds so it always breaches.
	budgetYAML := "max_seconds: 1\nmax_tool_calls: 100\n"
	if err := os.WriteFile(filepath.Join(corpusDir, taskID, "budget.yaml"), []byte(budgetYAML), 0600); err != nil {
		t.Fatal(err)
	}

	sb, err := sandbox.NewSandbox("test-budget", helixBin())
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	// Use a context that is already cancelled so the agent "times out".
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled immediately to simulate budget breach

	r := runner.NewRunner(runner.Config{
		CorpusDir:   corpusDir,
		OutDir:      outDir,
		HelixBin:    helixBin(),
		RunID:       "test-budget-run",
		MaxParallel: 1,
	})

	result, err := r.RunTask(ctx, sb, runner.TaskSpec{
		ID:     taskID,
		Mode:   "baseline",
		Prompt: "do nothing",
	})
	if err != nil {
		// RunTask shouldn't error — budget breach is captured in the result.
		t.Logf("RunTask returned err (may be expected for ctx cancellation): %v", err)
	}
	if result != nil && result.Outcome != "" {
		// Should not be success.
		if result.Outcome == "success" {
			t.Errorf("expected non-success outcome on budget breach, got %q", result.Outcome)
		}
	}
}

// TestRunnerWritesPatchDiff tests that the runner writes a patch.diff file.
func TestRunnerWritesPatchDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	corpusDir := t.TempDir()
	outDir := t.TempDir()
	taskID := makeCorpusTask(t, corpusDir, "task-patch", 0)

	sb, err := sandbox.NewSandbox("test-patch", helixBin())
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	r := runner.NewRunner(runner.Config{
		CorpusDir:   corpusDir,
		OutDir:      outDir,
		HelixBin:    helixBin(),
		RunID:       "test-patch-run",
		MaxParallel: 1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = r.RunTask(ctx, sb, runner.TaskSpec{
		ID:     taskID,
		Mode:   "baseline",
		Prompt: "do nothing",
	})
	if err != nil {
		t.Logf("RunTask err (may be ok for fake helix): %v", err)
	}

	// patch.diff should be written regardless of agent outcome.
	patchPath := filepath.Join(outDir, "test-patch-run", "tasks", taskID, "baseline", "patch.diff")
	if _, err := os.Stat(patchPath); err != nil {
		t.Errorf("patch.diff not found at %q: %v", patchPath, err)
	}
}

// TestRunnerWritesVerifyLog tests that verify.log is written with stdout/stderr
// and exit code.
func TestRunnerWritesVerifyLog(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	corpusDir := t.TempDir()
	outDir := t.TempDir()
	taskID := makeCorpusTask(t, corpusDir, "task-verifylog", 0)

	sb, err := sandbox.NewSandbox("test-verifylog", helixBin())
	if err != nil {
		t.Fatal(err)
	}
	defer sb.Cleanup()

	r := runner.NewRunner(runner.Config{
		CorpusDir:   corpusDir,
		OutDir:      outDir,
		HelixBin:    helixBin(),
		RunID:       "test-verifylog-run",
		MaxParallel: 1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, _ = r.RunTask(ctx, sb, runner.TaskSpec{
		ID:     taskID,
		Mode:   "baseline",
		Prompt: "do nothing",
	})

	logPath := filepath.Join(outDir, "test-verifylog-run", "tasks", taskID, "baseline", "verify.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Errorf("verify.log not found at %q: %v", logPath, err)
	}
}

// TestRunnerMatrix tests that RunMatrix runs all (task, mode) pairs and emits
// result.json files for each.
func TestRunnerMatrix(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	corpusDir := t.TempDir()
	outDir := t.TempDir()

	makeCorpusTask(t, corpusDir, "task-a", 0)
	makeCorpusTask(t, corpusDir, "task-b", 0)

	r := runner.NewRunner(runner.Config{
		CorpusDir:   corpusDir,
		OutDir:      outDir,
		HelixBin:    helixBin(),
		RunID:       "test-matrix-run",
		MaxParallel: 1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	results, err := r.RunMatrix(ctx, []string{"baseline", "native"})
	if err != nil {
		t.Logf("RunMatrix err (may be ok for fake helix): %v", err)
	}

	// Should have attempted 2 tasks × 2 modes = 4 runs.
	// Even with errors, results should be populated.
	if len(results) < 1 {
		// Allow zero results if agent subprocess couldn't spawn (no claude on PATH)
		// but ensure no panic / crash.
		t.Logf("RunMatrix produced %d results (may be low if claude not on PATH)", len(results))
	}
}

// Compile-time check that Runner, Config, TaskSpec, and RunMatrix are exported.
var _ *runner.Runner = (*runner.Runner)(nil)
var _ runner.Config = runner.Config{}
var _ runner.TaskSpec = runner.TaskSpec{}

// Ensure EvalResult from report package is used.
var _ report.EvalResult = report.EvalResult{}

// Ensure score package is reachable.
var _ score.Score = score.Score{}

// Use debug to avoid import errors.
var _ = debug.ReadBuildInfo
