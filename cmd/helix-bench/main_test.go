package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelixBenchHelpListsFiveSubcommands verifies --help exits 0 (Execute
// returns nil) and lists exactly the five BENCH-02 subcommands. Uses the
// in-process cobra capture idiom (SetOut/SetErr/SetArgs) — no subprocess, no
// PATH dependence.
func TestHelixBenchHelpListsFiveSubcommands(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--help"})

	require.NoError(t, root.Execute())

	out := buf.String()
	for _, name := range []string{"run", "fetch-datasets", "doctor", "report", "validate-cost-table"} {
		assert.Contains(t, out, name, "help output must mention %q subcommand", name)
	}

	// Exactly five top-level subcommands (BENCH-02). verify-tos is a Makefile
	// gate, not a sixth subcommand.
	assert.Len(t, newRootCmd().Commands(), 5, "helix-bench must expose exactly 5 subcommands")
}

// TestHelixBenchDoctorExitsZero verifies the doctor subcommand returns nil
// (exit 0) on a clean host.
func TestHelixBenchDoctorExitsZero(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"doctor"})

	require.NoError(t, root.Execute(), "doctor must exit 0 on a clean host")
}
