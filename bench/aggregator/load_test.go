package aggregator

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeRunRow builds a schema-valid result.v2.json for (task, mode, runIndex)
// with the given metrics and writes it to <outDir>/<task>/<mode>/<runIndex>/
// result.v2.json. It is adapted from bench/runtime/deltas_test.go:writeModeRow,
// generalizing the always-"0" run_index segment to 0..N-1 so a (task,mode) cell
// can carry N durable rows (the unit the N-gate counts).
func writeRunRow(t *testing.T, outDir, task, mode string, runIndex int, m evaluators.Metrics) string {
	t.Helper()
	b, err := runtime.BuildResult(runtime.ResultInput{
		TaskID:    task,
		Mode:      mode,
		Benchmark: "internal-toolbench",
		RunIndex:  runIndex,
		Outcome:   "pass",
		Metrics:   m,
	})
	require.NoError(t, err, "BuildResult(%s/%s/%d)", task, mode, runIndex)
	require.NoError(t, runtime.Validate(b), "row must be schema-valid before write-back")
	dir := filepath.Join(outDir, task, mode, strconv.Itoa(runIndex))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	p := filepath.Join(dir, "result.v2.json")
	require.NoError(t, os.WriteFile(p, b, 0o600))
	return p
}

// writeGarbageRow writes a deliberately schema-invalid / non-decoding
// result.v2.json at <outDir>/<task>/<mode>/<runIndex>/. It exists on disk
// (len(glob) sees it) but fails runtime.Validate, so the loader must count it as
// INVALID -> the cell is deficient (Pitfall 3: valid != file-present).
func writeGarbageRow(t *testing.T, outDir, task, mode string, runIndex int) string {
	t.Helper()
	dir := filepath.Join(outDir, task, mode, strconv.Itoa(runIndex))
	require.NoError(t, os.MkdirAll(dir, 0o700))
	p := filepath.Join(dir, "result.v2.json")
	require.NoError(t, os.WriteFile(p, []byte("{not valid}"), 0o600))
	return p
}

func ptrInt(v int) *int    { return &v }
func ptrBool(v bool) *bool { return &v }

// TestNGate locks the STATS-01 fail-closed N-gate (D-05) and its valid-row
// definition (D-01): a row is valid iff it exists, decodes, and passes
// runtime.Validate; expectedN comes from the caller arg, never len(glob).
func TestNGate(t *testing.T) {
	t.Run("sufficient_N_groups_rows_no_error", func(t *testing.T) {
		dir := t.TempDir()
		// 2 tasks x 1 mode x N=3 valid rows each.
		for _, task := range []string{"task-a", "task-b"} {
			for i := 0; i < 3; i++ {
				writeRunRow(t, dir, task, "honest", i, evaluators.Metrics{
					TaskSuccess: ptrBool(true),
					TokensInput: ptrInt(100 + i),
				})
			}
		}

		loaded, err := Load(dir, 3)
		require.NoError(t, err)
		require.NotNil(t, loaded)

		// Both cells present with exactly 3 grouped rows each.
		rowsA := loaded.Rows("task-a", "honest")
		require.Len(t, rowsA, 3)
		rowsB := loaded.Rows("task-b", "honest")
		require.Len(t, rowsB, 3)

		// Each row exposes the full doc + nullable metrics.
		require.NotNil(t, rowsA[0].Doc, "full doc preserved")
		_, hasMetrics := rowsA[0].Doc["metrics"]
		assert.True(t, hasMetrics, "full doc carries the metrics object verbatim")
		require.NotNil(t, rowsA[0].Metrics.TokensInput)
	})

	t.Run("deficient_cell_hard_error_names_cell_writes_nothing", func(t *testing.T) {
		dir := t.TempDir()
		// task-a has 3 valid rows; task-b has only 2.
		for i := 0; i < 3; i++ {
			writeRunRow(t, dir, "task-a", "honest", i, evaluators.Metrics{TaskSuccess: ptrBool(true)})
		}
		for i := 0; i < 2; i++ {
			writeRunRow(t, dir, "task-b", "honest", i, evaluators.Metrics{TaskSuccess: ptrBool(true)})
		}

		loaded, err := Load(dir, 3)
		require.Error(t, err, "deficient cell must fail closed")
		assert.Nil(t, loaded, "no result returned on deficiency -> caller writes nothing")
		assert.Contains(t, err.Error(), "task-b/honest: got 2 want 3",
			"error names the deficient cell with got/want counts")
	})

	t.Run("invalid_row_counts_as_deficient_not_skipped_to_pass", func(t *testing.T) {
		dir := t.TempDir()
		// Exactly 3 FILES on disk for the cell, but one fails runtime.Validate ->
		// 2 VALID rows -> deficient. Proves valid != file-present and
		// expectedN != len(glob).
		writeRunRow(t, dir, "task-a", "honest", 0, evaluators.Metrics{TaskSuccess: ptrBool(true)})
		writeRunRow(t, dir, "task-a", "honest", 1, evaluators.Metrics{TaskSuccess: ptrBool(true)})
		writeGarbageRow(t, dir, "task-a", "honest", 2)

		loaded, err := Load(dir, 3)
		require.Error(t, err, "an invalid on-disk file must count as deficient, not silently pass")
		assert.Nil(t, loaded)
		assert.Contains(t, err.Error(), "task-a/honest: got 2 want 3",
			"the garbage row is excluded -> got 2 even though 3 files exist")
	})

	t.Run("multiple_deficient_cells_all_named", func(t *testing.T) {
		dir := t.TempDir()
		writeRunRow(t, dir, "task-a", "honest", 0, evaluators.Metrics{TaskSuccess: ptrBool(true)})
		writeRunRow(t, dir, "task-b", "honest", 0, evaluators.Metrics{TaskSuccess: ptrBool(true)})

		loaded, err := Load(dir, 3)
		require.Error(t, err)
		assert.Nil(t, loaded)
		msg := err.Error()
		assert.True(t, strings.Contains(msg, "task-a/honest: got 1 want 3"), "names task-a: %s", msg)
		assert.True(t, strings.Contains(msg, "task-b/honest: got 1 want 3"), "names task-b: %s", msg)
	})
}

// TestLoadNilMetricPreserved locks Pitfall 4: a present metric stays set and an
// absent/null metric decodes to a nil pointer, never a fabricated 0.
func TestLoadNilMetricPreserved(t *testing.T) {
	dir := t.TempDir()
	// task_success present, tokens_input null (left nil in the Metrics record).
	for i := 0; i < 3; i++ {
		writeRunRow(t, dir, "task-a", "honest", i, evaluators.Metrics{
			TaskSuccess: ptrBool(true),
			TokensInput: nil,
		})
	}

	loaded, err := Load(dir, 3)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	rows := loaded.Rows("task-a", "honest")
	require.Len(t, rows, 3)
	require.NotNil(t, rows[0].Metrics.TaskSuccess, "present metric preserved")
	assert.True(t, *rows[0].Metrics.TaskSuccess)
	assert.Nil(t, rows[0].Metrics.TokensInput, "null metric stays nil, never fabricated 0")
}

// TestLoadZeroDiscoveryFailsClosed locks CR-01: an empty runDir, a non-existent
// runDir, and a tree with only non-numeric run-index segments all discover ZERO
// result.v2 rows. Load MUST return an error (fail closed), not a valid empty
// &Loaded{} — otherwise Aggregate would render an authoritative-looking empty
// report on a typo'd / missing path.
func TestLoadZeroDiscoveryFailsClosed(t *testing.T) {
	t.Run("empty_dir", func(t *testing.T) {
		dir := t.TempDir() // exists, no rows
		loaded, err := Load(dir, 3)
		require.Error(t, err, "empty runDir must fail closed")
		assert.Nil(t, loaded)
	})

	t.Run("non_existent_dir", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nope")
		loaded, err := Load(dir, 3)
		require.Error(t, err, "non-existent runDir must fail closed")
		assert.Nil(t, loaded)
	})

	t.Run("only_non_numeric_run_index", func(t *testing.T) {
		dir := t.TempDir()
		// A result.v2.json under a NON-numeric run-index segment is skipped by
		// globRows, so zero valid candidates are discovered.
		d := filepath.Join(dir, "task-a", "honest", "not-a-number")
		require.NoError(t, os.MkdirAll(d, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(d, "result.v2.json"), []byte("{}"), 0o600))
		loaded, err := Load(dir, 3)
		require.Error(t, err, "a tree with only non-numeric run-index segments must fail closed")
		assert.Nil(t, loaded)
	})
}
