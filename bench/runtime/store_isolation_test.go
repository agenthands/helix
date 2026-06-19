//go:build !windows
// +build !windows

package runtime

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/agenthands/helix/bench/languages/go" // register the Go runner so RunnerFor resolves the store-ON cell (D-10)
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// benchDatasetsRootForTest returns the absolute path of bench/datasets derived
// from this test file's location, so the seed-dir join is cwd-independent
// (mirrors seedDirForTest / benchRunnersRootForTest).
func benchDatasetsRootForTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	benchDir := filepath.Dir(filepath.Dir(thisFile)) // .../bench
	root := filepath.Join(benchDir, "datasets")
	require.DirExists(t, root)
	return root
}

// TestStoreIsolationParallel is the D-03 end-to-end proof (security T-78-04): two
// store-ON cells (the incremental_update fixture, which RunMatrix opts into the
// per-cell semantic store via meta.storeOptIn) run at --parallel=2 and must:
//
//  1. BOTH succeed (Summary.Succeeded == 2). The Pitfall-1 deadlock — two cells
//     opening the SAME cwd-relative .helix/semantic.duckdb under a shared CWD —
//     would manifest as a daemon hanging at startup ("socket did not appear
//     within 10s"), yielding 0 successes. Both succeeding proves the D-03
//     WithWorkingDir(repoDir) per-cell working dir broke the shared lock.
//  2. Resolve to DISTINCT per-cell .helix/semantic.duckdb paths, each under its
//     own per-cell repo dir (RepoFor -> <tmp>/<task>/<mode>/repo) — the per-cell
//     isolation the fix installs.
//  3. Report RejectedForeignPid == 0 for each cell (no cross-cell PID leakage in
//     the PID-gated daemon tap).
//
// It SKIPs when no helix binary is resolvable (mirrors the daemon-tap
// integration test gating; this test spawns real daemons with a live store).
func TestStoreIsolationParallel(t *testing.T) {
	helixBin := resolveHelixBin()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build ./cmd/helix'); skipping store-isolation integration test")
	}

	// Two COPIES of the incremental_update store-ON cell (the plan's "two copies"
	// option). They share the (benchmark, language, mode, task) identity, but each
	// RunCell builds its OWN ephemeral sandbox under a distinct random scratch
	// root, so WithWorkingDir(repoDir) lands a DISTINCT .helix/semantic.duckdb per
	// cell. (The durable <task>/<mode> artifact path is shared and last-writer-
	// wins — benign here; the store isolation we assert lives in the per-cell
	// scratch sandbox, not the durable out dir.) Only your_agent_full has a
	// MODE.md in bench/runners, so both copies use it.
	const task = "IT-go-incremental-update-1"
	const mode = "your_agent_full"

	cells := []Cell{
		{Benchmark: "internal-toolbench", Language: "go", Mode: mode, Task: task},
		{Benchmark: "internal-toolbench", Language: "go", Mode: mode, Task: task},
	}

	// Sanity: the fixture must actually opt the store ON (capability=incremental_update).
	seedDir := filepath.Join(benchDatasetsRootForTest(t), "internal-toolbench", "go", task)
	require.DirExists(t, seedDir)
	require.True(t, readTaskMeta(seedDir).storeOptIn(),
		"the incremental_update fixture MUST derive StoreOptIn=true — this test is meaningless store-OFF")

	outDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	cfg := RunMatrixConfig{
		RunID:        time.Now().UTC().Format("20060102T150405Z"),
		HelixBin:     helixBin,
		OutDir:       outDir,
		DatasetsRoot: benchDatasetsRootForTest(t),
		Agent:        "scripted",
		RunnersRoot:  benchRunnersRootForTest(t),
	}

	sum, err := RunMatrix(ctx, cells, 2, cfg)
	require.NoError(t, err, "RunMatrix must not return a usage error")

	// (1) Both store-ON cells must succeed under --parallel=2. A shared-lock
	// deadlock would hang a daemon at startup and drop Succeeded below 2.
	require.Equal(t, 2, sum.Total, "expected exactly 2 cells in the matrix")
	assert.Equal(t, 2, sum.Succeeded,
		"both store-ON cells must succeed under --parallel=2 (no DuckDB shared-lock deadlock; D-03)")

	// (3) Zero foreign-PID rejections per cell (no cross-cell PID leakage).
	for _, oc := range sum.Outcomes {
		assert.NoError(t, oc.Err, "cell %s/%s must have no infra error", oc.Cell.Task, oc.Cell.Mode)
		assert.Equal(t, 0, oc.Result.RejectedForeignPid,
			"cell %s/%s expected zero foreign-PID rejections in the PID-gated tap (D-03)", oc.Cell.Task, oc.Cell.Mode)
	}

	// (2) Each cell's .helix/semantic.duckdb is a DISTINCT path under its own
	// per-cell repo dir. Each cell's RunCell creates its OWN ephemeral sandbox
	// (distinct CellResult.ScratchDir), and WithWorkingDir(repoDir) makes the
	// cwd-relative store default (".helix/semantic.duckdb") resolve under that
	// cell's repo: <ScratchDir>/<task>/<mode>/repo/.helix/semantic.duckdb. We
	// derive the per-cell store path from each outcome's ScratchDir and assert
	// the two differ. (On a successful cell the scratch is cleaned up, so we
	// assert the PATHS are distinct rather than that the files still exist —
	// distinct roots PLUS the no-deadlock success in (1) prove the stores were
	// private per cell, which is exactly what the shared-CWD deadlock would
	// have prevented.)
	require.Len(t, sum.Outcomes, 2)
	storePathA := perCellStorePath(sum.Outcomes[0].Result.ScratchDir, task, sum.Outcomes[0].Cell.Mode)
	storePathB := perCellStorePath(sum.Outcomes[1].Result.ScratchDir, task, sum.Outcomes[1].Cell.Mode)
	require.NotEmpty(t, sum.Outcomes[0].Result.ScratchDir, "cell A must record its sandbox scratch root")
	require.NotEmpty(t, sum.Outcomes[1].Result.ScratchDir, "cell B must record its sandbox scratch root")
	assert.NotEqual(t, storePathA, storePathB,
		"the two store-ON cells must resolve to distinct .helix/semantic.duckdb paths (per-cell isolation, D-03)")
}

// perCellStorePath returns the per-cell semantic store path
// (<scratchRoot>/<task>/<mode>/repo/.helix/semantic.duckdb). It mirrors the
// sandbox RepoFor layout and the cwd-relative store default
// (internal/config/defaults.go: "semantic_index.store.path" = ".helix/semantic.duckdb").
func perCellStorePath(scratchRoot, task, mode string) string {
	return filepath.Join(scratchRoot, task, mode, "repo", ".helix", "semantic.duckdb")
}
