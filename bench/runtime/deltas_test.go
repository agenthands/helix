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
		TokensInput: iPtr(1000), TokensOutput: iPtr(200), ToolCalls: iPtr(10),
		FilesModified: iPtr(3), EditLocality: fPtr(0.9),
	})
	baselinePlain := writeModeRow(t, outDir, task, "baseline_plain", evaluators.Metrics{
		TokensInput: iPtr(1500), TokensOutput: iPtr(300), ToolCalls: iPtr(25),
		FilesModified: iPtr(5), EditLocality: fPtr(0.5),
	})
	noLSP := writeModeRow(t, outDir, task, "no_lsp", evaluators.Metrics{
		TokensInput: iPtr(1200), TokensOutput: iPtr(250), ToolCalls: iPtr(15),
		FilesModified: iPtr(4), EditLocality: fPtr(0.7),
	})
	noEdit := writeModeRow(t, outDir, task, "no_structured_edit", evaluators.Metrics{
		TokensInput: iPtr(1100), TokensOutput: iPtr(220), ToolCalls: iPtr(12),
		FilesModified: iPtr(3), EditLocality: fPtr(0.8),
	})

	outcomes := []CellOutcome{
		outcomeFor(task, "your_agent_full", full),
		outcomeFor(task, "baseline_plain", baselinePlain),
		outcomeFor(task, "no_lsp", noLSP),
		outcomeFor(task, "no_structured_edit", noEdit),
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

	m := evaluators.Metrics{TokensInput: iPtr(100), ToolCalls: iPtr(5)}
	full := writeModeRow(t, outDir, task, "your_agent_full", m)
	bp := writeModeRow(t, outDir, task, "baseline_plain", m)
	nl := writeModeRow(t, outDir, task, "no_lsp", m)
	ne := writeModeRow(t, outDir, task, "no_structured_edit", m)

	outcomes := []CellOutcome{
		outcomeFor(task, "your_agent_full", full),
		outcomeFor(task, "baseline_plain", bp),
		outcomeFor(task, "no_lsp", nl),
		outcomeFor(task, "no_structured_edit", ne),
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

	m := evaluators.Metrics{TokensInput: iPtr(100)}
	full := writeModeRow(t, outDir, task, "your_agent_full", m)
	bp := writeModeRow(t, outDir, task, "baseline_plain", m)
	nl := writeModeRow(t, outDir, task, "no_lsp", m)
	// no_structured_edit row deliberately ABSENT.

	outcomes := []CellOutcome{
		outcomeFor(task, "your_agent_full", full),
		outcomeFor(task, "baseline_plain", bp),
		outcomeFor(task, "no_lsp", nl),
	}

	report, err := ComputeAndWriteDeltas(outcomes)
	require.NoError(t, err, "an incomplete task must not error")
	assert.Empty(t, report.Computed, "no task should have deltas computed")
	require.Len(t, report.Skipped, 1, "the incomplete task must be reported as skipped")
	assert.Equal(t, task, report.Skipped[0].Task)

	// No row should have gained ablation_deltas.
	assert.Nil(t, readDeltas(t, full), "an incomplete task's rows must NOT be written back")
}

// TestDeltaIncludesBaselineRagOperand (Phase 83 Task 3, Open Q2 RESOLVED): once
// baseline_rag emits real rows it is a delta OPERAND — a delta set containing a
// baseline_rag row produces a `full_minus_baseline_rag` entry (the headline
// control-arm comparison), and that delta lands on every written-back row. The
// prior "baseline_rag excluded as a non-operand stub" behavior is inverted.
func TestDeltaIncludesBaselineRagOperand(t *testing.T) {
	outDir := t.TempDir()
	const task = "IT-go-patch-apply-1"

	full := writeModeRow(t, outDir, task, "your_agent_full", evaluators.Metrics{
		TokensInput: iPtr(1000), TokensOutput: iPtr(200), ToolCalls: iPtr(10),
		FilesModified: iPtr(3), EditLocality: fPtr(0.9),
	})
	bp := writeModeRow(t, outDir, task, "baseline_plain", evaluators.Metrics{
		TokensInput: iPtr(1500), ToolCalls: iPtr(25),
	})
	nl := writeModeRow(t, outDir, task, "no_lsp", evaluators.Metrics{
		TokensInput: iPtr(1200), ToolCalls: iPtr(15),
	})
	ne := writeModeRow(t, outDir, task, "no_structured_edit", evaluators.Metrics{
		TokensInput: iPtr(1100), ToolCalls: iPtr(12),
	})
	rag := writeModeRow(t, outDir, task, "baseline_rag", evaluators.Metrics{
		TokensInput: iPtr(800), ToolCalls: iPtr(5),
	})

	outcomes := []CellOutcome{
		outcomeFor(task, "your_agent_full", full),
		outcomeFor(task, "baseline_plain", bp),
		outcomeFor(task, "no_lsp", nl),
		outcomeFor(task, "no_structured_edit", ne),
		outcomeFor(task, "baseline_rag", rag),
	}

	report, err := ComputeAndWriteDeltas(outcomes)
	require.NoError(t, err)
	require.Equal(t, []string{task}, report.Computed)
	require.Empty(t, report.Skipped)

	deltas := readDeltas(t, full)
	require.NotNil(t, deltas)
	require.Len(t, deltas, 4, "now 4 deltas: full vs baseline_plain/no_lsp/no_structured_edit/baseline_rag")

	br := deltas["full_minus_baseline_rag"]
	require.NotNil(t, br, "baseline_rag must be a delta operand (full_minus_baseline_rag present)")
	assert.Equal(t, float64(1000-800), br["tokens_input"])
	assert.Equal(t, float64(10-5), br["tool_calls"])

	// The baseline_rag row itself also carries the same deltas object (every
	// operand's row reports the task's deltas).
	assert.NotNil(t, readDeltas(t, rag), "the baseline_rag row must carry ablation_deltas too")
}

// TestDeltaOmitsBaselineRagWhenAbsent (Phase 83 Task 3): baseline_rag is an
// OPTIONAL operand — when its row is absent (a run that did not include the arm),
// the task is NOT skipped (the 4 honest modes still gate completeness) and the
// full_minus_baseline_rag comparison is simply omitted (no nil-baseline arithmetic).
func TestDeltaOmitsBaselineRagWhenAbsent(t *testing.T) {
	outDir := t.TempDir()
	const task = "IT-go-norag-1"

	m := evaluators.Metrics{TokensInput: iPtr(100), ToolCalls: iPtr(5)}
	full := writeModeRow(t, outDir, task, "your_agent_full", m)
	bp := writeModeRow(t, outDir, task, "baseline_plain", m)
	nl := writeModeRow(t, outDir, task, "no_lsp", m)
	ne := writeModeRow(t, outDir, task, "no_structured_edit", m)
	// baseline_rag row deliberately ABSENT.

	outcomes := []CellOutcome{
		outcomeFor(task, "your_agent_full", full),
		outcomeFor(task, "baseline_plain", bp),
		outcomeFor(task, "no_lsp", nl),
		outcomeFor(task, "no_structured_edit", ne),
	}

	report, err := ComputeAndWriteDeltas(outcomes)
	require.NoError(t, err)
	require.Equal(t, []string{task}, report.Computed, "the 4 honest modes still gate completeness")
	require.Empty(t, report.Skipped, "a missing baseline_rag must NOT skip the task")

	deltas := readDeltas(t, full)
	require.NotNil(t, deltas)
	assert.Len(t, deltas, 3, "without a baseline_rag row, only the 3 honest deltas are produced")
	_, present := deltas["full_minus_baseline_rag"]
	assert.False(t, present, "full_minus_baseline_rag must be omitted when the baseline_rag row is absent")
}
