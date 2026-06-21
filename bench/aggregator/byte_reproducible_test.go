package aggregator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// byte_reproducible_test.go is the LOAD-BEARING Phase 89 (REPORT-05) proof: it
// renders all 4 reports over the committed multi-run fixture tree, then renders
// AGAIN over the SAME tree, and asserts the regenerated reports are BYTE-IDENTICAL
// to the first render (double-render diff-empty) AND to their committed goldens.
// This is the hermetic substitute for a live `report --run-id` run: byte-
// reproducibility is proven by the golden test alone — no daemon, no HELIX_BIN, no
// network. It exercises the SAME shared renderAll path the report subcommand calls,
// so a render that drifts on re-render (RNG leak, map-order, unsorted emit) fails
// HERE before it can ship.

// TestReportByteReproducible: a second Aggregate over the same run tree regenerates
// all 4 reports byte-identically (diff-empty), and each render matches its committed
// golden. This is the REPORT-05 capstone — the proof the phase exists for.
func TestReportByteReproducible(t *testing.T) {
	cfg := aggConfig(3)

	// First render.
	dir1 := goldenFixture(t)
	_, err := Aggregate(dir1, cfg)
	require.NoError(t, err)

	// Second render over an identically-built tree (the fixture is deterministic).
	dir2 := goldenFixture(t)
	_, err = Aggregate(dir2, cfg)
	require.NoError(t, err)

	for _, g := range renderAllGoldens {
		first, err := os.ReadFile(filepath.Join(dir1, g.report))
		require.NoError(t, err, "first render must write %s", g.report)
		second, err := os.ReadFile(filepath.Join(dir2, g.report))
		require.NoError(t, err, "second render must write %s", g.report)

		// Double-render diff-empty: the two renders are byte-identical.
		assert.Equal(t, string(first), string(second),
			"%s is NOT byte-reproducible across re-render (REPORT-05 diff-empty failed)", g.report)

		// And the render matches the committed golden (no drift from the frozen bytes).
		want, err := os.ReadFile(filepath.Join("testdata", g.golden))
		require.NoError(t, err, "missing golden %s", g.golden)
		assert.Equal(t, string(want), string(first),
			"%s drifted from its committed golden", g.report)
	}
}

// TestReportByteReproducibleSameDir: re-rendering INTO THE SAME runDir (overwriting
// the prior report files) also yields byte-identical bytes — the atomic temp+rename
// writeReport never produces a torn or differing file on a second pass. This mirrors
// what `report --run-id` does over a tree that already holds reports from a prior run.
func TestReportByteReproducibleSameDir(t *testing.T) {
	cfg := aggConfig(3)
	dir := goldenFixture(t)

	_, err := Aggregate(dir, cfg)
	require.NoError(t, err)
	first := map[string]string{}
	for _, g := range renderAllGoldens {
		b, err := os.ReadFile(filepath.Join(dir, g.report))
		require.NoError(t, err)
		first[g.report] = string(b)
	}

	// Re-render into the SAME dir.
	_, err = Aggregate(dir, cfg)
	require.NoError(t, err)
	for _, g := range renderAllGoldens {
		b, err := os.ReadFile(filepath.Join(dir, g.report))
		require.NoError(t, err)
		assert.Equal(t, first[g.report], string(b),
			"%s changed on re-render into the same dir (REPORT-05 byte-reproducibility failed)", g.report)
	}
}
