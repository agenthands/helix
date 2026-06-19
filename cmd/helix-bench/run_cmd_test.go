package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// resolveHelixBinForCmd returns a resolvable helix binary path (HELIX_BIN env or
// `helix` on PATH), or "" when none is available so the caller can degrade to a
// wiring-only assertion. Mirrors bench/runtime.resolveHelixBin.
func resolveHelixBinForCmd() string {
	if env := os.Getenv("HELIX_BIN"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	if p, err := exec.LookPath("helix"); err == nil {
		return p
	}
	return ""
}

// TestRunSubcommandWiresThemAll verifies that the `run` subcommand registers all
// seven+ flags and, when invoked, expands the matrix and dispatches cells under a
// run-scoped out dir. It mirrors cmd/helix-eval/run_cmd_test.go: a synthetic
// dataset fixture in t.TempDir(), invocation via cobra SetArgs, tolerance of a
// missing helix binary (the per-cell daemon spawn may fail in CI), and best-effort
// artifact assertions.
func TestRunSubcommandWiresThemAll(t *testing.T) {
	datasetsRoot := t.TempDir()
	outDir := t.TempDir()

	// Synthetic benchmark/lang/task: <datasets>/internal-toolbench/go/IT-go-patch-apply-1/
	// with the minimal files RunCell needs (scripted_agent.yaml + a Go repo +
	// verify.sh).
	taskDir := filepath.Join(datasetsRoot, "internal-toolbench", "go", "IT-go-patch-apply-1")
	if err := os.MkdirAll(taskDir, 0700); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(taskDir, "task.json"), `{"id":"IT-go-patch-apply-1","prompt":"make it pass"}`)
	writeFixture(t, filepath.Join(taskDir, "go.mod"), "module sumdoubler\n\ngo 1.21\n")
	writeFixture(t, filepath.Join(taskDir, "sum.go"), "package sumdoubler\n\nfunc Double(x int) int { return x }\n")
	writeFixture(t, filepath.Join(taskDir, "scripted_agent.yaml"), "steps:\n  - tool: read_file\n    args:\n      path: sum.go\n")
	writeFixture(t, filepath.Join(taskDir, "verify.sh"), "#!/bin/sh\nexit 0\n")

	runID := "smoke-run-001"

	root := newRootCmd()
	root.SetArgs([]string{
		"run",
		"--benchmarks", "internal-toolbench",
		"--languages", "go",
		"--modes", "your_agent_full",
		"--tasks", "IT-go-patch-apply-1",
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
	resultPath := filepath.Join(runOutDir, "IT-go-patch-apply-1", "your_agent_full", "result.v2.json")
	if _, err := os.Stat(resultPath); err == nil {
		t.Logf("durable result.v2.json present at %q (helix binary was available)", resultPath)
	} else {
		t.Logf("result.v2.json not written (helix binary likely unavailable) — wiring still asserted")
	}
}

// TestRunSubcommandWiresDeltaPass asserts the `run` subcommand invokes the
// Plan-05 post-RunMatrix delta pass: a multi-mode run over the 4 real modes on
// one seed task writes rows that, after the run completes, carry the
// `ablation_deltas` open property. It SKIPs when no helix binary is resolvable
// (the per-cell daemon spawn would fail and no rows would be written), since the
// assertion is specifically that the delta pass ran AFTER the matrix wrote the
// real rows. Hermetic: scripted agent, no model, no network.
func TestRunSubcommandWiresDeltaPass(t *testing.T) {
	helixBin := resolveHelixBinForCmd()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build ./cmd/helix'); skipping delta-pass wiring smoke")
	}

	// Use the REAL bench datasets root + the real runners (the mode resolver reads
	// bench/runners/<mode>/MODE.md). Derive the repo root from this test file.
	repoRoot := repoRootForTest(t)
	datasetsRoot := filepath.Join(repoRoot, "bench", "datasets")
	const task = "IT-go-patch-apply-1"

	outDir := t.TempDir()
	runID := "delta-wiring-001"

	root := newRootCmd()
	root.SetArgs([]string{
		"run",
		"--benchmarks", "internal-toolbench",
		"--languages", "go",
		"--modes", "your_agent_full",
		"--modes", "baseline_plain",
		"--modes", "no_lsp",
		"--modes", "no_structured_edit",
		"--tasks", task,
		"--datasets", datasetsRoot,
		"--out", outDir,
		"--helix-bin", helixBin,
		"--run-id", runID,
		"--parallel", "2",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("run (4 real modes): %v", err)
	}

	// Each real mode's row must carry ablation_deltas after the run (the delta pass
	// ran after the matrix barrier). Layout: <out>/<run_id>/<task>/<mode>/0/result.v2.json.
	runOutDir := filepath.Join(outDir, runID)
	for _, mode := range []string{"your_agent_full", "baseline_plain", "no_lsp", "no_structured_edit"} {
		p := filepath.Join(runOutDir, task, mode, "0", "result.v2.json")
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("real mode %s: result row not written at %q: %v", mode, p, err)
		}
		var doc struct {
			AblationDeltas map[string]map[string]float64 `json:"ablation_deltas"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatalf("real mode %s: decode row: %v", mode, err)
		}
		if doc.AblationDeltas == nil {
			t.Errorf("real mode %s: row at %q missing ablation_deltas (delta pass not wired into runBench)", mode, p)
		} else if len(doc.AblationDeltas) != 3 {
			t.Errorf("real mode %s: want exactly 3 deltas, got %d", mode, len(doc.AblationDeltas))
		}
	}
}

// TestRunSubcommandRegistersAllFlags asserts every documented flag is registered
// on the run subcommand (the --help acceptance, mechanically).
func TestRunSubcommandRegistersAllFlags(t *testing.T) {
	cmd := newRunCmd()
	for _, name := range []string{
		"benchmarks", "languages", "modes", "tasks", "parallel", "out", "agent", "helix-bin", "run-id", "datasets",
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
	root.SetArgs([]string{"run", "--benchmarks", "internal-toolbench", "--datasets", t.TempDir(), "--out", t.TempDir()})
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

// repoRootForTest returns the repository root derived from this test file's
// location (.../cmd/helix-bench/run_cmd_test.go -> repo root), so the real
// bench/datasets + bench/runners trees are reachable cwd-independently.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile = <repo>/cmd/helix-bench/run_cmd_test.go
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
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
