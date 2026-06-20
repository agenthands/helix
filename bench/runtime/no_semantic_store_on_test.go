//go:build !windows
// +build !windows

package runtime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/agenthands/helix/bench/languages/go" // register the Go runner so RunnerFor resolves the store-ON cell (D-10)
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoSemanticStoreOnZeroReads is the Phase 81 Plan 07 (GAP 2 / CR-01) DYNAMIC
// proof that the no_semantic ablation arm makes ZERO back-channel reads against
// the DuckDB semantic store EVEN WHEN THE STORE IS OPEN.
//
// Why this test exists (and why the shipped five_of_six smoke could not provide
// it): five_of_six_test.go drives the no_semantic arm against the store-OFF seed
// IT-go-patch-apply-1 (capability=patch_apply → StoreOptIn=false), so the daemon
// never opens a DuckDB store and the COUNTED read chokepoint
// (s.queryContext / s.queryRowContext) is structurally unreachable. CR-01 was
// therefore LATENT: the daemon-internal background read pipelines
// (SetActivateCallback drivers + SetFileFactStore) reached the chokepoint
// UNGATED, but only a store-ON no_semantic cell would actually read through them.
//
// This test deliberately runs a STORE-ON no_semantic cell:
//   - Seed: IT-go-incremental-update-1 (capability=incremental_update — the
//     store-on class), with StoreOptIn=true so the daemon OPENS its per-cell
//     DuckDB store (WithWorkingDir per-cell store). The store + semantic bundle
//     are BUILT (D-04 build-but-block).
//   - Mode: your_agent_no_semantic → profile bench-no-semantic, which carries
//     disable_semantic_subsystem: true → effSemanticDisabled. Plan 07's Task 1
//     gates the background read pipelines on effSemanticDisabled, so they drive
//     ZERO reads against the open store.
//
// Teeth (depends_on 81-06): Plan 06 made the runtime assertion fail-CLOSED — the
// daemon is now torn down GRACEFULLY (DaemonHandle.Stop, SIGTERM) so d.shutdown()
// flushes the helix_semantic_store_reads_total line, and an ABSENT line is a HARD
// failure (no longer a silent count=0). So if CR-01 ever regressed (a background
// pipeline read the open store on the no_semantic arm), the count would be
// non-zero and RunCell would HARD-FAIL the cell with SemanticReadViolation=true.
// Both teeth (81-06 fail-closed assertion + 81-07 gated pipelines) are present
// together — the masking interaction at 81-VERIFICATION.md:113 is broken.
//
// Hermetic (scripted agent, no model/network); SKIPs when no helix binary is
// resolvable (mirrors store_isolation_test.go / five_of_six_test.go). Because the
// bench smoke SKIPs without HELIX_BIN (the known false-green per MEMORY), this
// test only has teeth when run with HELIX_BIN set (or `helix` on PATH).
func TestNoSemanticStoreOnZeroReads(t *testing.T) {
	helixBin := resolveHelixBin()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build ./cmd/helix'); skipping store-ON no_semantic regression test")
	}

	const (
		benchmark = "internal-toolbench"
		language  = "go"
		task      = "IT-go-incremental-update-1"
		mode      = "your_agent_no_semantic"
	)

	datasetsRoot := benchDatasetsRootForTest(t)
	seedDir := filepath.Join(datasetsRoot, benchmark, language, task)
	require.DirExists(t, seedDir)

	// Sanity: the seed is the store-ON class (capability=incremental_update). If
	// this ever flips to a store-OFF seed the test goes vacuous (store never
	// opens) — the same trap that made CR-01 latent under five_of_six.
	require.True(t, readTaskMeta(seedDir).storeOptIn(),
		"the incremental_update fixture MUST derive StoreOptIn=true — this store-ON proof is meaningless store-OFF")

	outDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	// Drive ONE store-ON no_semantic cell end-to-end. StoreOptIn=true is set
	// explicitly (matching what runOneCell derives from the incremental_update
	// capability) so the daemon opens its DuckDB store on the no_semantic arm.
	res, err := RunCell(ctx, CellConfig{
		RunID:       time.Now().UTC().Format("20060102T150405Z"),
		Benchmark:   benchmark,
		Language:    language,
		Task:        task,
		Mode:        mode,
		HelixBin:    helixBin,
		SeedDir:     seedDir,
		OutDir:      outDir,
		RunnersRoot: benchRunnersRootForTest(t),
		Agent:       "scripted",
		StoreOptIn:  true, // store-ON: the daemon OPENS the DuckDB store (build-but-block, D-04)
	})

	// No infra error. A CR-01 regression (a background pipeline reading the open
	// store) would surface here as a non-nil error via RunCell's fail-closed
	// assertNoSemanticReads (81-06 teeth), so a clean run is itself part of the
	// proof.
	require.NoError(t, err,
		"store-ON no_semantic cell must run end-to-end with no infra error (a non-zero read would hard-fail here, CR-01 + 81-06)")

	// The load-bearing assertion: ZERO semantic-store reads on the open store.
	assert.Equal(t, 0, res.SemanticStoreReads,
		"store-ON no_semantic arm must make ZERO back-channel reads against the open DuckDB store (CR-01 closed structurally by Plan 07 Task 1)")
	assert.False(t, res.SemanticReadViolation,
		"no SemanticReadViolation: the kernel gate + gated background pipelines held on a store-ON arm")
	assert.Equal(t, 0, res.VerifyExitCode,
		"the cell's verify.sh must pass (the no_semantic arm still completes the task)")
}
