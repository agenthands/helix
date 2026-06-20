//go:build !windows
// +build !windows

package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFiveOfSixSmoke is the phase-level gate (criterion #1). It drives the
// scripted agent across ALL SIX modes on one seed task through RunMatrix, runs
// the Plan-05 post-matrix delta pass over the outcomes, and asserts the full
// five-of-six contract:
//
//   - exactly 4 REAL result.v2.json rows on disk (your_agent_full, baseline_plain,
//     no_lsp, no_structured_edit), each schema-valid and (after the delta pass)
//     carrying ablation_deltas;
//   - exactly 1 additional REAL row (your_agent_no_semantic) that OMITS
//     ablation_status (the Phase 81 deferral-marker removal — ABLATE-06 landed)
//     and is NOT a delta operand (no ablation_deltas);
//   - ZERO rows for baseline_rag — its cell is Deferred==true, Success==false, and
//     no file exists at its ResultPath.
//
// It is hermetic (scripted agent, no model, no network) and SKIPs when no helix
// binary is resolvable (mirrors TestDaemonTap).
func TestFiveOfSixSmoke(t *testing.T) {
	helixBin := resolveHelixBin()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build ./cmd/helix'); skipping five-of-six smoke")
	}

	const task = "IT-go-patch-apply-1"
	seed := seedDirForTest(t)
	datasetsRoot := filepath.Dir(filepath.Dir(filepath.Dir(seed))) // .../bench/datasets

	allModes := []string{
		"your_agent_full",
		"baseline_plain",
		"no_lsp",
		"no_structured_edit",
		"your_agent_no_semantic",
		"baseline_rag",
	}

	cells, err := ExpandMatrix([]string{"internal-toolbench"}, []string{"go"}, allModes, []string{task})
	require.NoError(t, err)
	require.Len(t, cells, len(allModes))

	outDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	t.Cleanup(cancel)

	summary, err := RunMatrix(ctx, cells, 2, RunMatrixConfig{
		RunID:        time.Now().UTC().Format("20060102T150405Z"),
		HelixBin:     helixBin,
		OutDir:       outDir,
		DatasetsRoot: datasetsRoot,
		Agent:        "scripted",
		RunnersRoot:  benchRunnersRootForTest(t),
	})
	require.NoError(t, err)
	require.Equal(t, len(allModes), summary.Total)

	// Index outcomes by mode and surface any infra errors loudly.
	ocByMode := make(map[string]CellOutcome, len(summary.Outcomes))
	for _, oc := range summary.Outcomes {
		require.NoError(t, oc.Err, "cell %s/%s must not have an infra error", oc.Cell.Task, oc.Cell.Mode)
		ocByMode[oc.Cell.Mode] = oc
	}

	// baseline_rag: registered + fail-closed. Deferred, not a success, no row file.
	rag := ocByMode["baseline_rag"]
	assert.True(t, rag.Deferred, "baseline_rag cell must be Deferred")
	assert.False(t, rag.Success, "baseline_rag cell must NOT be a success")
	_, statErr := os.Stat(rag.Result.ResultPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "baseline_rag must write NO result.v2.json")

	// Run the Plan-05 post-matrix delta pass over the outcomes (this is the unit
	// the runBench wiring invokes — exercised here directly at the matrix tier).
	report, err := ComputeAndWriteDeltas(summary.Outcomes)
	require.NoError(t, err)
	require.Equal(t, []string{task}, report.Computed, "the one task with all 4 real modes must have deltas computed")
	require.Empty(t, report.Skipped, "no task should be skipped (all 4 real modes present)")

	// The 4 real modes: exactly 4 rows on disk, each schema-valid + carrying deltas.
	realModes := []string{"your_agent_full", "baseline_plain", "no_lsp", "no_structured_edit"}
	for _, m := range realModes {
		oc := ocByMode[m]
		assert.True(t, oc.Success, "real mode %s must be a success", m)
		require.FileExists(t, oc.Result.ResultPath, "real mode %s must write a result row", m)
		b, rerr := os.ReadFile(oc.Result.ResultPath)
		require.NoError(t, rerr)
		assert.NoError(t, Validate(b), "real mode %s row must be schema-valid after delta write-back", m)

		var doc struct {
			AblationDeltas map[string]map[string]float64 `json:"ablation_deltas"`
			AblationStatus string                        `json:"ablation_status"`
		}
		require.NoError(t, json.Unmarshal(b, &doc))
		require.NotNil(t, doc.AblationDeltas, "real mode %s row must carry ablation_deltas", m)
		assert.Len(t, doc.AblationDeltas, 3, "real mode %s must carry exactly 3 deltas", m)
		assert.Empty(t, doc.AblationStatus, "honest real mode %s must NOT carry ablation_status", m)
	}

	// The no_semantic row: a REAL row that (post-Phase-81) OMITS the deferral
	// marker — the kernel disable_semantic_subsystem guarantee (ABLATE-06) landed,
	// so the row is a clean measurement, not a partial. It is still NOT one of the
	// 3 delta operands (so it carries no ablation_deltas).
	ns := ocByMode["your_agent_no_semantic"]
	require.FileExists(t, ns.Result.ResultPath, "your_agent_no_semantic must write a row")
	nsBytes, err := os.ReadFile(ns.Result.ResultPath)
	require.NoError(t, err)
	var nsDoc struct {
		AblationStatus string                        `json:"ablation_status"`
		AblationDeltas map[string]map[string]float64 `json:"ablation_deltas"`
	}
	require.NoError(t, json.Unmarshal(nsBytes, &nsDoc))
	assert.Empty(t, nsDoc.AblationStatus,
		"the no_semantic row must OMIT ablation_status — the guarantee_pending_phase_81 marker is removed in Phase 81")
	assert.Nil(t, nsDoc.AblationDeltas,
		"the no_semantic arm is NOT a delta operand and must not gain ablation_deltas")

	// Exactly 4 real rows + 1 partial row = 5 rows on disk; baseline_rag = 0.
	rowCount := 0
	for _, m := range append(append([]string{}, realModes...), "your_agent_no_semantic", "baseline_rag") {
		if _, err := os.Stat(ocByMode[m].Result.ResultPath); err == nil {
			rowCount++
		}
	}
	assert.Equal(t, 5, rowCount, "five-of-six: exactly 4 real-with-deltas + 1 no_semantic row; baseline_rag writes none")
}
