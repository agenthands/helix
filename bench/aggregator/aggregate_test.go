package aggregator

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/runners"
	"github.com/agenthands/helix/bench/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aggregate_test.go exercises the pure orchestrator end-to-end: fail-closed on a
// deficient cell (D-05), the two-level reduction (D-07) over a synthetic
// multi-run matrix, and byte-deterministic reports under a fixed seed (D-08).
// All fixtures are synthetic result.v2.json rows over a t.TempDir() runDir — NO
// HELIX_BIN, NO network.

// goldenModelID is the cost-table golden row this package already ships
// (testdata/cost-table.golden.yaml). The fixture writer projects it into each
// row's model_id so PriceFor can join cost during Aggregate.
const goldenModelID = "claude-sonnet-4-5-20250929"

// aggToday is the injected "today" the cost golden's dates are kept fresh
// against (mirrors cost_test.go's 2026-06-20).
var aggToday = time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)

// writeCostedRow writes a schema-valid result.v2.json carrying the given metrics
// AND the golden model_id (so the cost join succeeds) at
// <outDir>/<task>/<mode>/<runIndex>/result.v2.json.
func writeCostedRow(t *testing.T, outDir, task, mode string, runIndex int, m evaluators.Metrics) {
	t.Helper()
	b, err := runtime.BuildResult(runtime.ResultInput{
		TaskID:    task,
		Mode:      mode,
		Benchmark: "internal-toolbench",
		RunIndex:  runIndex,
		Outcome:   "pass",
		Metrics:   m,
		Fairness:  runners.FairnessContract{ModelID: goldenModelID},
	})
	require.NoError(t, err, "BuildResult(%s/%s/%d)", task, mode, runIndex)
	require.NoError(t, runtime.Validate(b), "row must be schema-valid before write")
	dir := filepath.Join(outDir, task, mode, strconv.Itoa(runIndex))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "result.v2.json"), b, 0o600))
}

// metric builds a fully-populated Metrics for a single run.
func metric(success bool, ti, to, tc, fr int, locality float64) evaluators.Metrics {
	return evaluators.Metrics{
		TaskSuccess:  ptrBool(success),
		TokensInput:  ptrInt(ti),
		TokensOutput: ptrInt(to),
		ToolCalls:    ptrInt(tc),
		FilesRead:    ptrInt(fr),
		EditLocality: ptrFloat(locality),
	}
}

func ptrFloat(v float64) *float64 { return &v }

// aggConfig is the standard test config: fixed seed, golden cost table, injected today.
func aggConfig(expectedN int) Config {
	return Config{
		ExpectedN:     expectedN,
		Seed:          42,
		Iterations:    10000,
		CILevel:       0.95,
		KValues:       []int{1, expectedN},
		CostTablePath: "testdata/cost-table.golden.yaml",
		Today:         aggToday,
	}
}

// TestAggregateFailClosed: a runDir with a deficient (task,mode) yields a non-nil
// error AND writes NO leaderboard.md / cost_quality.md (D-05 fail-closed).
func TestAggregateFailClosed(t *testing.T) {
	dir := t.TempDir()
	// task-1/full has 3 runs; task-1/no_lsp has only 1 (deficient for N=3).
	for i := 0; i < 3; i++ {
		writeCostedRow(t, dir, "task-1", "full", i, metric(true, 1000, 100, 5, 3, 0.9))
	}
	writeCostedRow(t, dir, "task-1", "no_lsp", 0, metric(false, 1000, 100, 5, 3, 0.9))

	rep, err := Aggregate(dir, aggConfig(3))
	require.Error(t, err, "a deficient cell must fail closed")
	assert.Nil(t, rep)
	assert.NoFileExists(t, filepath.Join(dir, "leaderboard.md"))
	assert.NoFileExists(t, filepath.Join(dir, "cost_quality.md"))
}

// TestAggregateTwoLevelReduce: a 2-mode x 3-task x N=3 matrix yields one row per
// (mode x benchmark); task_success == mean of per-task success-rates; pass@1 ==
// task_success (c/n identity); every metric carries a CI; the reports render.
func TestAggregateTwoLevelReduce(t *testing.T) {
	dir := t.TempDir()
	tasks := []string{"task-1", "task-2", "task-3"}
	for _, task := range tasks {
		for i := 0; i < 3; i++ {
			// "full" all-success; "no_lsp" all-fail -> clearly separated success rates.
			writeCostedRow(t, dir, task, "full", i, metric(true, 1000, 100, 5, 3, 0.9))
			writeCostedRow(t, dir, task, "no_lsp", i, metric(false, 2000, 200, 9, 6, 0.5))
		}
	}

	rep, err := Aggregate(dir, aggConfig(3))
	require.NoError(t, err)
	require.NotNil(t, rep)

	// One leaderboard row per (mode x benchmark): full + no_lsp.
	require.Len(t, rep.Leaderboard, 2)
	byMode := map[string]LeaderRow{}
	for _, r := range rep.Leaderboard {
		byMode[r.Mode] = r
	}
	full, ok := byMode["full"]
	require.True(t, ok)
	noLsp, ok := byMode["no_lsp"]
	require.True(t, ok)

	// full: every task success-rate 3/3=1.0 -> mean 1.0; pass@1 == task_success.
	assert.True(t, full.TaskSuccess.OK)
	assert.InDelta(t, 1.0, full.TaskSuccess.Point, 1e-9)
	assert.InDelta(t, full.TaskSuccess.Point, full.PassAt1.Point, 1e-9,
		"pass@1 must equal task_success (c/n identity)")

	// no_lsp: every task success-rate 0/3=0.0 -> mean 0.0.
	assert.True(t, noLsp.TaskSuccess.OK)
	assert.InDelta(t, 0.0, noLsp.TaskSuccess.Point, 1e-9)

	// Reports were written to disk.
	assert.FileExists(t, filepath.Join(dir, "leaderboard.md"))
	assert.FileExists(t, filepath.Join(dir, "cost_quality.md"))
}

// TestAggregateNilMetricEmDash: a metric nil across ALL runs of ALL tasks yields
// a null CI rendered as an em-dash, not a fabricated 0.
func TestAggregateNilMetricEmDash(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		// edit_locality left nil on every row.
		m := evaluators.Metrics{
			TaskSuccess:  ptrBool(true),
			TokensInput:  ptrInt(1000),
			TokensOutput: ptrInt(100),
			ToolCalls:    ptrInt(5),
			FilesRead:    ptrInt(3),
			// EditLocality intentionally nil.
		}
		writeCostedRow(t, dir, "task-1", "full", i, m)
	}
	rep, err := Aggregate(dir, aggConfig(3))
	require.NoError(t, err)
	require.Len(t, rep.Leaderboard, 1)
	assert.False(t, rep.Leaderboard[0].EditLocality.OK,
		"a metric nil across all runs must produce a null CI (em-dash), never 0")
}

// TestDeterministic: two Aggregate calls with the SAME seed over the SAME runDir
// produce byte-identical leaderboard.md and cost_quality.md (D-08).
func TestDeterministic(t *testing.T) {
	build := func() (string, string) {
		dir := t.TempDir()
		tasks := []string{"task-1", "task-2", "task-3"}
		for _, task := range tasks {
			for i := 0; i < 3; i++ {
				writeCostedRow(t, dir, task, "full", i, metric(true, 1000, 100, 5, 3, 0.9))
				writeCostedRow(t, dir, task, "no_lsp", i, metric(i == 0, 2000, 200, 9, 6, 0.5))
			}
		}
		_, err := Aggregate(dir, aggConfig(3))
		require.NoError(t, err)
		lb, err := os.ReadFile(filepath.Join(dir, "leaderboard.md"))
		require.NoError(t, err)
		cq, err := os.ReadFile(filepath.Join(dir, "cost_quality.md"))
		require.NoError(t, err)
		return string(lb), string(cq)
	}

	lb1, cq1 := build()
	lb2, cq2 := build()
	assert.Equal(t, lb1, lb2, "same seed + same input must yield byte-identical leaderboard.md")
	assert.Equal(t, cq1, cq2, "same seed + same input must yield byte-identical cost_quality.md")
}
