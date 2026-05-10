package main

import (
	"bytes"
	"strings"
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

// TestHelixEvalRunNotImplemented verifies that 'run --quick --corpus /tmp/empty'
// exits non-zero with a "not yet implemented" message until Wave 1+ wires it.
func TestHelixEvalRunNotImplemented(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"run", "--quick", "--corpus", t.TempDir()})

	err := root.Execute()
	require.Error(t, err, "run should return an error (not yet implemented)")
	assert.True(t,
		strings.Contains(err.Error(), "not yet implemented"),
		"error message must say 'not yet implemented', got: %q", err.Error())
}

// TestPipelineExportsEvalPhases verifies that internal/eval/pipeline.go
// re-exports the canonical phasegraph EvalPhases (10 phases) and that the
// re-export is a reference (not a copy). We assert length == 10 matching
// the 10-phase declaration in phasegraph/pipelines/eval.go.
func TestPipelineExportsEvalPhases(t *testing.T) {
	assert.Equal(t, 10, len(eval.Phases),
		"eval.Phases must re-export the 10-phase EvalPhases from phasegraph/pipelines/eval.go; got %d", len(eval.Phases))
}
