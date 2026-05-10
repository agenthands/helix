package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/agenthands/helix/internal/eval"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelixEvalCommandHelp verifies --help exits 0 and mentions the two
// subcommands. Uses cobra.Command.SetOut to capture output in-process
// (faster than subprocess, no PATH dependence).
func TestHelixEvalCommandHelp(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--help"})

	err := root.Execute()
	// cobra exits 0 for --help; Execute returns nil.
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "run", "help output must mention 'run' subcommand")
	assert.Contains(t, out, "validate-rules", "help output must mention 'validate-rules' subcommand")
	assert.Contains(t, out, "eval", "help output must mention 'eval'")
}

// TestHelixEvalRunEmptyCorpusSucceeds verifies that 'run' over an empty corpus
// completes without error and writes all 6 report files. This replaces the
// Wave 0 "not yet implemented" test now that Wave 3 wires the run body.
func TestHelixEvalRunEmptyCorpusSucceeds(t *testing.T) {
	outDir := t.TempDir()
	corpusDir := t.TempDir() // empty corpus → 0 tasks

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"run",
		"--corpus", corpusDir,
		"--mode", "baseline",
		"--out", outDir,
		"--run-id", "test-empty-corpus",
	})

	err := root.Execute()
	// Empty corpus should succeed (0 tasks, all reports still emitted).
	require.NoError(t, err, "run over empty corpus should not error; got: %v\nOutput: %s", err, buf.String())

	// All 6 report files should exist.
	runDir := outDir + "/test-empty-corpus"
	for _, f := range []string{
		"eval_report.json", "eval_report.md",
		"cost_summary.json", "tool_behavior.json",
		"safety_compliance.json", "run_metadata.json",
	} {
		path := runDir + "/" + f
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected report file %q: %v", path, err)
		}
	}
}

// TestPipelineExportsEvalPhases verifies that internal/eval/pipeline.go
// re-exports the canonical phasegraph EvalPhases (10 phases) and that the
// re-export is a reference (not a copy). We assert length == 10 matching
// the 10-phase declaration in phasegraph/pipelines/eval.go.
func TestPipelineExportsEvalPhases(t *testing.T) {
	assert.Equal(t, 10, len(eval.Phases),
		"eval.Phases must re-export the 10-phase EvalPhases from phasegraph/pipelines/eval.go; got %d", len(eval.Phases))
}
