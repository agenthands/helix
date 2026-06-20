package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelixBenchHelpListsSubcommands verifies --help exits 0 (Execute returns
// nil) and lists every top-level subcommand. Uses the in-process cobra capture
// idiom (SetOut/SetErr/SetArgs) — no subprocess, no PATH dependence.
//
// Phase 82 (D-02) added `aggregate` as the sixth subcommand, extending the
// Phase 75 BENCH-02 "exactly five" --help acceptance: `aggregate` is the
// always-intended aggregator surface the milestone roadmap reserved here, not a
// drive-by addition. verify-tos remains a Makefile-only gate, NOT a subcommand.
func TestHelixBenchHelpListsSubcommands(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--help"})

	require.NoError(t, root.Execute())

	out := buf.String()
	for _, name := range []string{"run", "fetch-datasets", "doctor", "report", "validate-cost-table", "aggregate"} {
		assert.Contains(t, out, name, "help output must mention %q subcommand", name)
	}

	// Exactly six top-level subcommands (the five BENCH-02 originals + the Phase
	// 82 aggregate). verify-tos is a Makefile gate, not a subcommand.
	assert.Len(t, newRootCmd().Commands(), 6, "helix-bench must expose exactly 6 subcommands")
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
