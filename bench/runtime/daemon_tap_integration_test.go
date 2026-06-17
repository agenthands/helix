//go:build !windows
// +build !windows

package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolveHelixBin returns the helix binary to drive the integration tests, or
// "" if none is available (the caller SKIPs). It prefers the HELIX_BIN env
// override (so CI / `make bench-quick` can point at a freshly-built binary
// without polluting PATH), then falls back to `helix` on PATH — mirroring
// helix-eval's --helix-bin resolution and the eval integration test's LookPath.
func resolveHelixBin() string {
	if env := os.Getenv("HELIX_BIN"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	if p, err := exec.LookPath("helix"); err == nil {
		return p
	}
	return ""
}

// seedDirForTest returns the absolute path of the
// internal-toolbench/go/IT-go-patch-apply-1 seed task, derived from this test
// file's location so it is cwd-independent.
func seedDirForTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	benchDir := filepath.Dir(filepath.Dir(thisFile)) // .../bench
	seed := filepath.Join(benchDir, "datasets", "internal-toolbench", "go", "IT-go-patch-apply-1")
	require.DirExists(t, seed)
	return seed
}

// TestDaemonTap is the BENCH-04 end-to-end spine. It runs one cell on the seed
// IT-go-patch-apply-1 task in your_agent_full mode and asserts the THREE Nyquist signals
// the smoke must carry (an exit-code-only smoke would alias all three):
//
//  1. 2-leg merged trace: ToolCallSummary.Total >= 1 AND the CC leg is present.
//  2. zero PID cross-talk: DaemonTapResult.RejectedForeignPid == 0.
//  3. result.v2 schema validity.
//
// It also asserts verifyExit == 0 (the scripted edit made `go test` pass) and
// that the durable artifacts exist on disk. SKIPs when no helix binary is
// resolvable (mirrors internal/eval/runner.daemon_tap_integration_test.go).
func TestDaemonTap(t *testing.T) {
	helixBin := resolveHelixBin()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build ./cmd/helix'); skipping integration test")
	}

	outDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	cfg := CellConfig{
		RunID:       time.Now().UTC().Format("20060102T150405Z"),
		Benchmark:   "internal-toolbench",
		Language:    "go",
		Task:        "IT-go-patch-apply-1",
		Mode:        "your_agent_full",
		HelixBin:    helixBin,
		SeedDir:     seedDirForTest(t),
		OutDir:      outDir,
		RunnersRoot: benchRunnersRootForTest(t),
	}

	res, err := RunCell(ctx, cfg)
	require.NoError(t, err, "RunCell must complete without an infrastructure error")

	// Nyquist signal 1: 2-leg trace — tool calls counted AND CC leg present.
	assert.GreaterOrEqual(t, res.ToolCallTotal, 1,
		"expected >=1 daemon tool call in the merged trace (2-leg / signal 1)")
	assert.True(t, res.CCLegPresent,
		"expected the synthesized CC (agent-tap) leg to be present in the merged trace")

	// Nyquist signal 2: zero PID cross-talk.
	assert.Equal(t, 0, res.RejectedForeignPid,
		"expected zero foreign-PID rejections in the PID-gated daemon tap (signal 2)")

	// Nyquist signal 3: result.v2 schema validity.
	assert.True(t, res.ResultValid, "emitted result.v2.json must validate against the schema (signal 3)")

	// The scripted edit must have made `go test` pass.
	assert.Equal(t, 0, res.VerifyExitCode, "verify.sh (go test) must pass after the scripted edit")
	assert.Equal(t, "success", res.Merged.Outcome, "merged outcome must be success when verify passes")

	// Durable artifacts must exist on disk.
	assert.FileExists(t, res.ResultPath, "result.v2.json must be written to the durable out dir")
	assert.FileExists(t, res.MergedTracePath, "trace.json must be written to the durable out dir")
}
