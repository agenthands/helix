package aggregator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// render_all_test.go locks the Phase 89 (REPORT-05) shared-render contract: a
// single Aggregate over the golden fixture now emits ALL 4 reports — leaderboard.md,
// cost_quality.md, per_language.md, ablations.md — through the shared renderAll
// path, each byte-identical to its committed golden. Before 89-03 only the first
// two were written to disk; renderAll closes that gap so `report` and `aggregate`
// have one render path (the byte-reproducibility substrate).
//
// The leaderboard/cost goldens are the existing testdata/*.golden.md (the
// goldenFixture tree). per_language.md / ablations.md over goldenFixture need their
// OWN goldens (the package's standalone per_language/ablations golden tests use
// DIFFERENT fixtures), committed as goldenfix_*.golden.md and regenerated with
// `-update`.

// renderAllGoldens maps each report Aggregate writes to its committed golden for
// the goldenFixture tree.
var renderAllGoldens = []struct{ report, golden string }{
	{"leaderboard.md", "leaderboard.golden.md"},
	{"cost_quality.md", "cost_quality.golden.md"},
	{"per_language.md", "goldenfix_per_language.golden.md"},
	{"ablations.md", "goldenfix_ablations.golden.md"},
}

// TestRenderAllEmitsFourReports: Aggregate writes all 4 reports into runDir, each
// matching its committed golden (the shared renderAll path is what both Aggregate
// and the report subcommand call).
func TestRenderAllEmitsFourReports(t *testing.T) {
	dir := goldenFixture(t)
	_, err := Aggregate(dir, aggConfig(3))
	require.NoError(t, err)

	for _, g := range renderAllGoldens {
		got, err := os.ReadFile(filepath.Join(dir, g.report))
		require.NoError(t, err, "renderAll must write %s to runDir", g.report)
		if *updateGolden {
			require.NoError(t, os.WriteFile(filepath.Join("testdata", g.golden), got, 0o600))
			continue
		}
		want, err := os.ReadFile(filepath.Join("testdata", g.golden))
		require.NoError(t, err, "missing golden %s — regenerate with -update", g.golden)
		assert.Equal(t, string(want), string(got),
			"%s drifted from its committed golden (shared renderAll byte-stability)", g.report)
	}
}
