package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// report_test.go is the D-02 in-process test for the Phase 89 (REPORT-05) `report
// --run-id` subcommand: a thin RunE over the SAME aggregator render path as
// `aggregate`, resolving <out>/<run-id>/ and regenerating all 4 reports. It asserts
//
//   - report --run-id over a committed fixture tree produces the SAME 4 report bytes
//     as aggregate over that tree (report==aggregate byte-equality, shared renderAll);
//   - an invalid --run-id (".." traversal, leading '-', empty) is REFUSED with a
//     non-zero exit BEFORE any filepath.Join (T-89-03-01 path-traversal mitigation).
//
// Like aggregate_test.go these are PURELY synthetic result.v2.json fixtures — NO
// HELIX_BIN, no daemon, no network.

// fourReports is the byte-reproducible report set report/aggregate both emit.
var fourReports = []string{"leaderboard.md", "cost_quality.md", "per_language.md", "ablations.md"}

// writeReportFixtureTree builds a sufficient N=3 multi-run tree under
// outRoot/<runID>/ that both aggregate and report can resolve.
func writeReportFixtureTree(t *testing.T, runDir string) {
	t.Helper()
	tasks := []string{"task-1", "task-2", "task-3"}
	for _, task := range tasks {
		for i := 0; i < 3; i++ {
			writeAggRow(t, runDir, task, "full", i, aggMetric(true, 1000, 100, 5, 3, 0.9))
			writeAggRow(t, runDir, task, "no_lsp", i, aggMetric(i < 1, 2000, 200, 9, 6, 0.5))
		}
	}
}

// runReportCmd executes `helix-bench report --run-id <id> --out <out> --runs <runs>
// --cost-table <ct>` through the real cobra tree in-process and returns the error.
func runReportCmd(t *testing.T, out, runID string, runs int, costTable string) error {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"report",
		"--run-id", runID,
		"--out", out,
		"--runs", strconv.Itoa(runs),
		"--cost-table", costTable,
	})
	return root.Execute()
}

// TestReportSubcommandRegistered: the report subcommand is wired and exposes
// --run-id (registration untouched, count still 6 — asserted in main_test.go).
func TestReportSubcommandRegistered(t *testing.T) {
	root := newRootCmd()
	var found *bool
	for _, c := range root.Commands() {
		if c.Name() == "report" {
			b := c.Flags().Lookup("run-id") != nil
			found = &b
		}
	}
	require.NotNil(t, found, "report subcommand must be registered")
	assert.True(t, *found, "report must expose a --run-id flag")
}

// TestReportEqualsAggregate (REPORT-05 shared renderAll): report --run-id over a
// fixture tree produces byte-identical reports to aggregate over the SAME tree.
func TestReportEqualsAggregate(t *testing.T) {
	ct := repoCostTablePath(t)

	// aggregate over an independent copy of the tree.
	aggRoot := t.TempDir()
	aggDir := filepath.Join(aggRoot, "RUN-A")
	writeReportFixtureTree(t, aggDir)
	require.NoError(t, runAggregateCmd(t, aggDir, 3, ct))

	// report --run-id over a second identical tree under <out>/<run-id>/.
	repOut := t.TempDir()
	repDir := filepath.Join(repOut, "RUN-A")
	writeReportFixtureTree(t, repDir)
	require.NoError(t, runReportCmd(t, repOut, "RUN-A", 3, ct))

	for _, name := range fourReports {
		aggBytes, err := os.ReadFile(filepath.Join(aggDir, name))
		require.NoError(t, err, "aggregate must write %s", name)
		repBytes, err := os.ReadFile(filepath.Join(repDir, name))
		require.NoError(t, err, "report must write %s", name)
		assert.Equal(t, string(aggBytes), string(repBytes),
			"%s must be byte-identical between report and aggregate (shared renderAll)", name)
	}
}

// TestReportRunIDValidation (T-89-03-01): an invalid --run-id is REFUSED with a
// non-zero exit and writes NOTHING — the segment is rejected before any
// filepath.Join can escape the report tree.
func TestReportRunIDValidation(t *testing.T) {
	ct := repoCostTablePath(t)
	for _, bad := range []string{"../etc", "..", "-rf", "a/b", "with space", ""} {
		out := t.TempDir()
		err := runReportCmd(t, out, bad, 3, ct)
		require.Error(t, err, "invalid run-id %q must be refused (non-zero exit)", bad)
		// No report tree should have been created from a rejected segment.
		entries, rdErr := os.ReadDir(out)
		require.NoError(t, rdErr)
		assert.Empty(t, entries,
			"a rejected run-id %q must write nothing under --out", bad)
	}
}

// TestReportOutValidation (WR-03): a --out root carrying a '..' traversal segment is
// REFUSED with a non-zero exit and writes NOTHING — closing the gap where a valid
// --run-id could still relocate the whole report tree via `--out ../../x`. An
// operator-trusted absolute --out is allowed (covered elsewhere); only upward escape
// is rejected.
func TestReportOutValidation(t *testing.T) {
	ct := repoCostTablePath(t)
	for _, badOut := range []string{"..", "../etc", "../../x", "a/../../b"} {
		// A VALID run-id, so the rejection can ONLY come from the --out validation.
		// Assert the SPECIFIC --out error so the test discriminates the WR-03 guard
		// from an incidental fail-closed Aggregate error on a missing directory.
		err := runReportCmd(t, badOut, "good-run", 3, ct)
		require.Error(t, err, "an --out with a '..' traversal segment %q must be refused", badOut)
		assert.Contains(t, err.Error(), "invalid --out",
			"%q must be refused by the --out validation (not an incidental Aggregate error)", badOut)
	}
}

// TestReportRunIDRequired: an empty/missing --run-id is an error (no default tree).
func TestReportRunIDMissing(t *testing.T) {
	ct := repoCostTablePath(t)
	out := t.TempDir()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"report", "--out", out, "--cost-table", ct})
	require.Error(t, root.Execute(), "report with no --run-id must fail")
}
