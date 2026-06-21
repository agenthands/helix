//go:build !windows
// +build !windows

package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allowedToolsForSeed is the set of tool names an IT-go-patch-apply-1 cell legitimately
// issues: activate_project (harness setup — points the daemon workspace at the
// cloned repo) plus the scripted task's single replace_in_file edit. Any daemon
// tool_call in a cell's merged trace OUTSIDE this set is foreign-cell leakage
// (F-07 / criterion #4) — a PID cross-talk escape.
var allowedToolsForSeed = map[string]struct{}{
	"activate_project": {},
	"replace_in_file":  {},
}

// TestCrossCell is the METRIC-06 / criterion-#4 parallel PID-cross-talk
// regression (PITFALLS line 337). It boots N cells (same seed task, distinct
// cell out dirs) CONCURRENTLY and asserts, per cell:
//
//  1. DaemonTapResult.RejectedForeignPid == 0 — the per-cell PID gate rejected
//     no foreign-daemon log lines; and
//  2. no merged trace contains a daemon tool_call the scripted task never made —
//     no neighbor's tool calls leaked across the per-cell daemon.log boundary.
//
// Parallelism is kept modest (>=2; the criterion-#1 target is --parallel=4).
// SKIPs when no helix binary is resolvable.
func TestCrossCell(t *testing.T) {
	helixBin := resolveHelixBin()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build ./cmd/helix'); skipping integration test")
	}

	const nCells = 3 // >=2; bounded so CI stays inside the budget
	seedDir := seedDirForTest(t)
	runnersRoot := benchRunnersRootForTest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	type cellOutcome struct {
		idx int
		res CellResult
		err error
	}
	outcomes := make([]cellOutcome, nCells)

	var wg sync.WaitGroup
	for i := 0; i < nCells; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cfg := CellConfig{
				RunID:       time.Now().UTC().Format("20060102T150405Z"),
				Benchmark:   "internal-toolbench",
				Language:    "go",
				Task:        "IT-go-patch-apply-1",
				Mode:        "your_agent_full",
				RunIndex:    idx,
				HelixBin:    helixBin,
				SeedDir:     seedDir,
				OutDir:      t.TempDir(), // distinct durable out dir per cell
				RunnersRoot: runnersRoot,
			}
			res, err := RunCell(ctx, cfg)
			outcomes[idx] = cellOutcome{idx: idx, res: res, err: err}
		}(i)
	}
	wg.Wait()

	for _, oc := range outcomes {
		require.NoErrorf(t, oc.err, "cell %d must complete without an infrastructure error", oc.idx)

		// (1) zero foreign-PID rejections per cell.
		assert.Equalf(t, 0, oc.res.RejectedForeignPid,
			"cell %d: PID-gated tap rejected a foreign-daemon log line (cross-talk)", oc.idx)

		// (2) no foreign tool names in this cell's merged trace.
		for tool := range oc.res.Merged.ToolCallSummary.ByTool {
			_, ok := allowedToolsForSeed[tool]
			assert.Truef(t, ok,
				"cell %d: merged trace contains foreign tool %q the scripted task never called", oc.idx, tool)
		}

		// Sanity: each cell still produced a real 2-leg trace and a valid result.
		assert.GreaterOrEqualf(t, oc.res.ToolCallTotal, 1, "cell %d: expected >=1 tool call", oc.idx)
		assert.Truef(t, oc.res.CCLegPresent, "cell %d: expected CC leg present", oc.idx)
		assert.Truef(t, oc.res.ResultValid, "cell %d: expected schema-valid result.v2", oc.idx)
	}
}
