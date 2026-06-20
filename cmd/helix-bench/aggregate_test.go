package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/runners"
	"github.com/agenthands/helix/bench/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aggregate_test.go is the D-02 in-process subcommand test. It drives the real
// cobra tree (newRootCmd) with args ["aggregate", <tmpDir>] over PURELY
// synthetic result.v2.json fixtures — NO HELIX_BIN, no daemon, no network — and
// asserts the two operator-facing contracts:
//
//   - a deficient run (a cell with fewer than --runs valid rows) -> Execute()
//     returns a non-nil error AND no leaderboard.md is written (fail-closed,
//     D-05);
//   - a sufficient N=3 run -> Execute() returns nil AND both leaderboard.md +
//     cost_quality.md exist.
//
// The end-to-end confidence check over a REAL daemon-produced matrix is the
// separate manual HELIX_BIN path (Plan 82-07 Task 2 human-verify), driven via
// the built binary, not a Go test — these in-process tests are deliberately
// HELIX_BIN-free so `go test ./...` is never false-green on this surface.

// aggGoldenModelID is the cost-table model_id the aggregator package's golden
// row prices; projecting it into each fixture row keeps the fixtures realistic.
// The cost join itself is not asserted here (the cost table is resolved relative
// to the daemon CWD, not this test's), so cost cells degrade to em-dashes — the
// leaderboard.md still renders, which is all these contracts assert.
const aggGoldenModelID = "claude-sonnet-4-5-20250929"

// writeAggRow writes one schema-valid result.v2.json at
// <outDir>/<task>/<mode>/<runIndex>/result.v2.json (the durable-tree layout the
// aggregator globs). It mirrors the aggregator package's fixture writer.
func writeAggRow(t *testing.T, outDir, task, mode string, runIndex int, m evaluators.Metrics) {
	t.Helper()
	b, err := runtime.BuildResult(runtime.ResultInput{
		TaskID:    task,
		Mode:      mode,
		Benchmark: "internal-toolbench",
		RunIndex:  runIndex,
		Outcome:   "pass",
		Metrics:   m,
		Fairness:  runners.FairnessContract{ModelID: aggGoldenModelID},
	})
	require.NoError(t, err, "BuildResult(%s/%s/%d)", task, mode, runIndex)
	require.NoError(t, runtime.Validate(b), "row must be schema-valid before write")
	dir := filepath.Join(outDir, task, mode, strconv.Itoa(runIndex))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "result.v2.json"), b, 0o600))
}

func aggPtrBool(v bool) *bool        { return &v }
func aggPtrInt(v int) *int           { return &v }
func aggPtrFloat(v float64) *float64 { return &v }

// aggMetric builds a fully-populated single-run Metrics.
func aggMetric(success bool, ti, to, tc, fr int, loc float64) evaluators.Metrics {
	return evaluators.Metrics{
		TaskSuccess:  aggPtrBool(success),
		TokensInput:  aggPtrInt(ti),
		TokensOutput: aggPtrInt(to),
		ToolCalls:    aggPtrInt(tc),
		FilesRead:    aggPtrInt(fr),
		EditLocality: aggPtrFloat(loc),
	}
}

// runAggregateCmd executes `helix-bench aggregate <dir> --runs <runs>` through
// the real cobra tree in-process and returns the Execute() error.
func runAggregateCmd(t *testing.T, dir string, runs int) error {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"aggregate", dir, "--runs", strconv.Itoa(runs)})
	return root.Execute()
}

// TestAggregateCmdFailClosed: a deficient run dir (one cell with 2 of 3 rows)
// makes Execute() return a non-nil error AND writes NO leaderboard.md (D-05
// fail-closed, single-exit non-zero).
func TestAggregateCmdFailClosed(t *testing.T) {
	dir := t.TempDir()
	// task-1/full has 3 rows; task-1/no_lsp has only 2 (deficient for N=3).
	for i := 0; i < 3; i++ {
		writeAggRow(t, dir, "task-1", "full", i, aggMetric(true, 1000, 100, 5, 3, 0.9))
	}
	for i := 0; i < 2; i++ {
		writeAggRow(t, dir, "task-1", "no_lsp", i, aggMetric(false, 2000, 200, 9, 6, 0.5))
	}

	err := runAggregateCmd(t, dir, 3)
	require.Error(t, err, "a deficient cell must fail closed (non-zero exit)")
	assert.NoFileExists(t, filepath.Join(dir, "leaderboard.md"),
		"a deficient run must write no leaderboard.md")
	assert.NoFileExists(t, filepath.Join(dir, "cost_quality.md"),
		"a deficient run must write no cost_quality.md")
}

// TestAggregateCmdSufficient: a sufficient N=3 run dir makes Execute() return
// nil AND writes both leaderboard.md + cost_quality.md.
func TestAggregateCmdSufficient(t *testing.T) {
	dir := t.TempDir()
	for _, task := range []string{"task-1", "task-2", "task-3"} {
		for i := 0; i < 3; i++ {
			writeAggRow(t, dir, task, "full", i, aggMetric(true, 1000, 100, 5, 3, 0.9))
			writeAggRow(t, dir, task, "no_lsp", i, aggMetric(false, 2000, 200, 9, 6, 0.5))
		}
	}

	err := runAggregateCmd(t, dir, 3)
	require.NoError(t, err, "a sufficient run must exit 0")

	for _, name := range []string{"leaderboard.md", "cost_quality.md"} {
		p := filepath.Join(dir, name)
		assert.FileExists(t, p)
		info, statErr := os.Stat(p)
		require.NoError(t, statErr)
		assert.NotZero(t, info.Size(), "%s must be non-empty", name)
	}
}
