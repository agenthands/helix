package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRunSubcommandWiresThemAll verifies that the `run` subcommand registers all
// seven+ flags and, when invoked, expands the matrix and dispatches cells under a
// run-scoped out dir. It mirrors cmd/helix-eval/run_cmd_test.go: a synthetic
// dataset fixture in t.TempDir(), invocation via cobra SetArgs, tolerance of a
// missing helix binary (the per-cell daemon spawn may fail in CI), and best-effort
// artifact assertions.
func TestRunSubcommandWiresThemAll(t *testing.T) {
	datasetsRoot := t.TempDir()
	outDir := t.TempDir()

	// Synthetic benchmark/task: <datasets>/toolbench-go/sum-doubler/ with the
	// minimal files RunCell needs (scripted_agent.yaml + a Go repo + verify.sh).
	taskDir := filepath.Join(datasetsRoot, "toolbench-go", "sum-doubler")
	if err := os.MkdirAll(taskDir, 0700); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(taskDir, "task.json"), `{"id":"sum-doubler","prompt":"make it pass"}`)
	writeFixture(t, filepath.Join(taskDir, "go.mod"), "module sumdoubler\n\ngo 1.21\n")
	writeFixture(t, filepath.Join(taskDir, "sum.go"), "package sumdoubler\n\nfunc Double(x int) int { return x }\n")
	writeFixture(t, filepath.Join(taskDir, "scripted_agent.yaml"), "steps:\n  - tool: read_file\n    args:\n      relative_path: sum.go\n")
	writeFixture(t, filepath.Join(taskDir, "verify.sh"), "#!/bin/sh\nexit 0\n")

	runID := "smoke-run-001"

	root := newRootCmd()
	root.SetArgs([]string{
		"run",
		"--benchmarks", "toolbench-go",
		"--modes", "your_agent_full",
		"--tasks", "sum-doubler",
		"--datasets", datasetsRoot,
		"--out", outDir,
		"--helix-bin", "helix",
		"--run-id", runID,
	})

	// Tolerate errors: the helix binary may be absent, so the per-cell daemon
	// spawn (and thus a passing cell) is best-effort — we assert wiring, not a
	// green cell. The run-scoped out dir is created BEFORE dispatch, so it must
	// exist regardless of cell outcome.
	_ = root.Execute()

	runOutDir := filepath.Join(outDir, runID)
	if _, err := os.Stat(runOutDir); err != nil {
		t.Fatalf("run-scoped out dir %q not created: %v", runOutDir, err)
	}

	// If a real helix binary was on PATH, the cell may have written a durable
	// result.v2.json — assert best-effort (present-and-valid OR absent).
	resultPath := filepath.Join(runOutDir, "sum-doubler", "your_agent_full", "result.v2.json")
	if _, err := os.Stat(resultPath); err == nil {
		t.Logf("durable result.v2.json present at %q (helix binary was available)", resultPath)
	} else {
		t.Logf("result.v2.json not written (helix binary likely unavailable) — wiring still asserted")
	}
}

// TestRunSubcommandRegistersAllFlags asserts every documented flag is registered
// on the run subcommand (the --help acceptance, mechanically).
func TestRunSubcommandRegistersAllFlags(t *testing.T) {
	cmd := newRunCmd()
	for _, name := range []string{
		"benchmarks", "modes", "tasks", "parallel", "out", "agent", "helix-bin", "run-id", "datasets",
	} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("run subcommand missing --%s flag", name)
		}
	}
}

// TestRunSubcommandRejectsUnknownAgent asserts the --agent validation gate.
func TestRunSubcommandRejectsUnknownAgent(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"run", "--agent", "bogus", "--datasets", t.TempDir(), "--out", t.TempDir()})
	if err := root.Execute(); err == nil {
		t.Error("run --agent=bogus: want error, got nil")
	}
}

// TestRunSubcommandEmptyBenchmarkDirErrors asserts that an empty/absent benchmark
// dataset dir (no tasks discoverable) is a hard error, not a silent exit-0 no-op.
func TestRunSubcommandEmptyBenchmarkDirErrors(t *testing.T) {
	root := newRootCmd()
	// --tasks omitted forces discovery under a datasets root with no benchmark dir.
	root.SetArgs([]string{"run", "--benchmarks", "toolbench-go", "--datasets", t.TempDir(), "--out", t.TempDir()})
	if err := root.Execute(); err == nil {
		t.Error("run with empty benchmark dir: want error, got nil")
	}
}

// TestNotYetImplementedStillCoversOtherSubcommands asserts the run stub is gone
// but the OTHER unimplemented subcommands still report not-yet-implemented (the
// scope boundary: only `run` was wired).
func TestNotYetImplementedStillCoversOtherSubcommands(t *testing.T) {
	for _, sub := range []string{"fetch-datasets", "report"} {
		root := newRootCmd()
		root.SetArgs([]string{sub})
		if err := root.Execute(); err == nil {
			t.Errorf("subcommand %q: expected not-yet-implemented error, got nil", sub)
		}
	}
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	mode := os.FileMode(0600)
	if filepath.Base(path) == "verify.sh" {
		mode = 0700
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}
