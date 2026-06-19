package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/bench/evaluators"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// intPtr / floatPtr build the nullable metric pointers the helper operates on.
func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

// writeModeRow builds a schema-valid result.v2.json for (task, mode) with the
// given metrics and writes it to <outDir>/<task>/<mode>/0/result.v2.json,
// returning the path. It mirrors the durable layout cellDurablePaths produces.
func writeModeRow(t *testing.T, outDir, task, mode string, m evaluators.Metrics) string {
	t.Helper()
	b, err := BuildResult(ResultInput{
		TaskID:    task,
		Mode:      mode,
		Benchmark: "internal-toolbench",
		RunIndex:  0,
		Outcome:   "pass",
		Metrics:   m,
	})
	require.NoError(t, err, "BuildResult(%s/%s)", task, mode)
	require.NoError(t, Validate(b), "row must be schema-valid before write-back")
	dir := filepath.Join(outDir, task, mode, "0")
	require.NoError(t, os.MkdirAll(dir, 0700))
	p := filepath.Join(dir, "result.v2.json")
	require.NoError(t, os.WriteFile(p, b, 0600))
	return p
}

// outcomeFor builds a CellOutcome whose Result.ResultPath points at the on-disk
// row the helper will read back.
func outcomeFor(task, mode, resultPath string) CellOutcome {
	return CellOutcome{
		Cell:    Cell{Benchmark: "internal-toolbench", Language: "go", Mode: mode, Task: task},
		Result:  CellResult{Task: task, Mode: mode, ResultPath: resultPath},
		Success: true,
	}
}

// readDeltas reads the ablation_deltas open property back from an on-disk row.
func readDeltas(t *testing.T, path string) map[string]map[string]float64 {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc struct {
		AblationDeltas map[string]map[string]float64 `json:"ablation_deltas"`
	}
	require.NoError(t, json.Unmarshal(b, &doc))
	return doc.AblationDeltas
}

// TestAblationDeltasArithmetic asserts the helper computes EXACTLY the 3 fixed
// deltas (full−baseline_plain, full−no_lsp, full−no_structured_edit) with correct
// arithmetic on the comparable metrics (Test 1, RED).
func TestAblationDeltasArithmetic(t *testing.T) {
	outDir := t.TempDir()
	const task = "IT-go-patch-apply-1"

	full := writeModeRow(t, outDir, task, "your_agent_full", evaluators.Metrics{
		TokensInput: intPtr(1000), TokensOutput: intPtr(200), ToolCalls: intPtr(10),
		FilesModified: intPtr(3), EditLocality: floatPtr(0.9),
	})
	baselinePlain := writeModeRow(t, outDir, task, "baseline_plain", evaluators.Metrics{
		TokensInput: intPtr(1500), TokensOutput: intPtr(300), ToolCalls: intPtr(25),
		FilesModified: intPtr(5), EditLocality: floatPtr(0.5),
	})
	noLSP := writeModeRow(t, outDir, task, "your_agent_no_lsp", evaluators.Metrics{
		TokensInput: intPtr(1200), TokensOutput: intPtr(250), ToolCalls: intPtr(15),
		FilesModified: intPtr(4), EditLocality: floatPtr(0.7),
	})
	noEdit := writeModeRow(t, outDir, task, "your_agent_no_structured_edit", evaluators.Metrics{
		TokensInput: intPtr(1100), TokensOutput: intPtr(220), ToolCalls: intPtr(12),
		FilesModified: intPtr(3), EditLocality: floatPtr(0.8),
	})

	outcomes := []CellOutcome{
		outcomeFor(task, "your_agent_full", full),
		outcomeFor(task, "baseline_plain", baselinePlain),
		outcomeFor(task, "your_agent_no_lsp", noLSP),
		outcomeFor(task, "your_agent_no_structured_edit", noEdit),
	}

	report, err := ComputeAndWriteDeltas(outcomes)
	require.NoError(t, err)
	require.Equal(t, 1, len(report.Computed), "exactly one task with all 4 real modes")
	require.Empty(t, report.Skipped, "no task should be skipped")

	deltas := readDeltas(t, full)
	require.NotNil(t, deltas, "ablation_deltas must be present on the full row")
	require.Len(t, deltas, 3, "exactly 3 deltas (full vs baseline_plain/no_lsp/no_structured_edit)")

	// full − baseline_plain
	bp := deltas["full_minus_baseline_plain"]
	require.NotNil(t, bp)
	assert.Equal(t, float64(1000-1500), bp["tokens_input"])
	assert.Equal(t, float64(200-300), bp["tokens_output"])
	assert.Equal(t, float64(10-25), bp["tool_calls"])
	assert.Equal(t, float64(3-5), bp["files_modified"])
	assert.InDelta(t, 0.9-0.5, bp["edit_locality"], 1e-9)

	// full − no_lsp
	nl := deltas["full_minus_no_lsp"]
	require.NotNil(t, nl)
	assert.Equal(t, float64(1000-1200), nl["tokens_input"])
	assert.Equal(t, float64(10-15), nl["tool_calls"])

	// full − no_structured_edit
	ne := deltas["full_minus_no_structured_edit"]
	require.NotNil(t, ne)
	assert.Equal(t, float64(1000-1100), ne["tokens_input"])
	assert.Equal(t, float64(3-3), ne["files_modified"])
}

// TestAblationDeltasWriteBackStillValid asserts the deltas land in EACH of the 4
// per-mode rows and every written row still passes runtime.Validate (Test 2, RED).
func TestAblationDeltasWriteBackStillValid(t *testing.T) {
	outDir := t.TempDir()
	const task = "IT-go-patch-apply-1"

	m := evaluators.Metrics{TokensInput: intPtr(100), ToolCalls: intPtr(5)}
	full := writeModeRow(t, outDir, task, "your_agent_full", m)
	bp := writeModeRow(t, outDir, task, "baseline_plain", m)
	nl := writeModeRow(t, outDir, task, "your_agent_no_lsp", m)
	ne := writeModeRow(t, outDir, task, "your_agent_no_structured_edit", m)

	outcomes := []CellOutcome{
		outcomeFor(task, "your_agent_full", full),
		outcomeFor(task, "baseline_plain", bp),
		outcomeFor(task, "your_agent_no_lsp", nl),
		outcomeFor(task, "your_agent_no_structured_edit", ne),
	}

	_, err := ComputeAndWriteDeltas(outcomes)
	require.NoError(t, err)

	for _, p := range []string{full, bp, nl, ne} {
		b, rerr := os.ReadFile(p)
		require.NoError(t, rerr)
		assert.NoError(t, Validate(b), "row %q must still validate after write-back", p)
		assert.NotNil(t, readDeltas(t, p), "every real-mode row must carry ablation_deltas")
	}
}

// TestAblationDeltasSkipsIncompleteTask asserts a task missing one of the 4 real
// modes is SKIPPED (no panic, no nil-baseline arithmetic) and reported (Test 3, RED).
func TestAblationDeltasSkipsIncompleteTask(t *testing.T) {
	outDir := t.TempDir()
	const task = "IT-go-incomplete-1"

	m := evaluators.Metrics{TokensInput: intPtr(100)}
	full := writeModeRow(t, outDir, task, "your_agent_full", m)
	bp := writeModeRow(t, outDir, task, "baseline_plain", m)
	nl := writeModeRow(t, outDir, task, "your_agent_no_lsp", m)
	// no_structured_edit row deliberately ABSENT.

	outcomes := []CellOutcome{
		outcomeFor(task, "your_agent_full", full),
		outcomeFor(task, "baseline_plain", bp),
		outcomeFor(task, "your_agent_no_lsp", nl),
	}

	report, err := ComputeAndWriteDeltas(outcomes)
	require.NoError(t, err, "an incomplete task must not error")
	assert.Empty(t, report.Computed, "no task should have deltas computed")
	require.Len(t, report.Skipped, 1, "the incomplete task must be reported as skipped")
	assert.Equal(t, task, report.Skipped[0].Task)

	// No row should have gained ablation_deltas.
	assert.Nil(t, readDeltas(t, full), "an incomplete task's rows must NOT be written back")
}
