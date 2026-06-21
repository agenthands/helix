package aggregator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ablations_test.go locks REPORT-03: renderAblations renders all 5 delta tables
// (full vs no_lsp, no_semantic, no_structured_edit, baseline_plain, baseline_rag)
// with CI-overlap markers via the reused ciOverlap predicate; the full vs
// no_semantic delta is computed at AGGREGATE time (deltas.go deliberately omits
// it). A missing operand renders em-dash. The renderer is RNG-free and byte-stable.

// TestAblationComparisonSet locks the fixed 5-comparison set + order.
func TestAblationComparisonSet(t *testing.T) {
	want := []string{
		"full_minus_no_lsp",
		"full_minus_no_semantic",
		"full_minus_no_structured_edit",
		"full_minus_baseline_plain",
		"full_minus_baseline_rag",
	}
	require.Len(t, ablationComparisons, len(want), "exactly 5 ablation comparisons")
	for i, c := range ablationComparisons {
		assert.Equal(t, want[i], c.name, "ablation comparison %d name + order", i)
	}
	// no_semantic MUST be one of them (the deltas.go non-operand, computed here).
	var hasNoSemantic bool
	for _, c := range ablationComparisons {
		if c.other == "your_agent_no_semantic" {
			hasNoSemantic = true
		}
	}
	assert.True(t, hasNoSemantic, "full vs no_semantic must be an aggregate-time comparison")
}

// TestAblationsRender locks the table/marker/em-dash contract with injected rows.
func TestAblationsRender(t *testing.T) {
	rows := []AblationRow{
		// Disjoint CIs -> no overlap -> a directional delta claim is allowed.
		{Comparison: "full_minus_no_lsp",
			FullCI:  ci(0.90, 0.85, 0.95),
			OtherCI: ci(0.40, 0.30, 0.50),
			Present: true},
		// Overlapping CIs -> overlap marker, suppress the X>Y claim.
		{Comparison: "full_minus_no_semantic",
			FullCI:  ci(0.70, 0.55, 0.85),
			OtherCI: ci(0.60, 0.45, 0.75),
			Present: true},
		// Absent operand -> em-dash.
		{Comparison: "full_minus_baseline_rag",
			FullCI:  ci(0.90, 0.85, 0.95),
			OtherCI: nullCI(),
			Present: false},
	}
	md := renderAblations(rows, testFooter())

	assert.Contains(t, md, "full_minus_no_lsp")
	assert.Contains(t, md, "full_minus_no_semantic")
	assert.Contains(t, md, "full_minus_baseline_rag")

	// Disjoint pair: delta point 0.90-0.40 = 0.5000 rendered.
	assert.Contains(t, md, "0.5000", "the full vs no_lsp delta point must render")
	// Overlapping pair: a CI-overlap marker appears.
	assert.Contains(t, md, "CI overlap", "an overlapping full-vs-other pair must mark the CI overlap")
	// Absent operand renders an em-dash.
	assert.Contains(t, md, "—", "an absent ablation operand must render an em-dash, never a fabricated 0")

	// Footer provenance.
	assert.Contains(t, md, "2027-01-28", "ablations footer cites the cost-table valid_until")
}

// TestAblationsByteStable: a double render of the same input is byte-identical.
func TestAblationsByteStable(t *testing.T) {
	rows := []AblationRow{
		{Comparison: "full_minus_no_lsp", FullCI: ci(0.9, 0.8, 1.0), OtherCI: ci(0.4, 0.3, 0.5), Present: true},
		{Comparison: "full_minus_no_semantic", FullCI: ci(0.7, 0.6, 0.8), OtherCI: ci(0.6, 0.5, 0.7), Present: true},
	}
	a := renderAblations(rows, testFooter())
	b := renderAblations(rows, testFooter())
	assert.Equal(t, a, b, "renderAblations must be byte-stable across renders")
}

// abFixture writes a canonical-mode-named multi-run runDir for the ablations
// golden: your_agent_full + the 5 ablation operands, each N=3, with a clear,
// asymmetric success split so the deltas are non-trivial and a swapped operand is
// caught. no_semantic IS present (the aggregate-time comparison the matrix delta
// pass omits). baseline_rag is included so all 5 comparisons render present.
func abFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	tasks := []string{"task-1", "task-2", "task-3"}
	// per-mode all-runs success boolean (uniform within a mode for a clean golden).
	modeSuccess := map[string]bool{
		"your_agent_full":               true,
		"your_agent_no_lsp":             false,
		"your_agent_no_semantic":        false,
		"your_agent_no_structured_edit": true,
		"baseline_plain":                false,
		"baseline_rag":                  false,
	}
	for _, task := range tasks {
		for mode, succ := range modeSuccess {
			for i := 0; i < 3; i++ {
				writeCostedRow(t, dir, task, mode, i, metric(succ, 1000, 100, 5, 3, 0.9))
			}
		}
	}
	return dir
}

// TestAblationsGolden runs the aggregate-time ablation reduce over abFixture and
// diffs the rendered ablations.md against the committed golden. Regenerate with
// `go test ./bench/aggregator/ -run TestAblationsGolden -update`.
func TestAblationsGolden(t *testing.T) {
	dir := abFixture(t)
	rep, err := Aggregate(dir, aggConfig(3))
	require.NoError(t, err)
	require.NotNil(t, rep)

	// All 5 comparisons present in the reduced ablation rows.
	require.Len(t, rep.Ablations, len(ablationComparisons),
		"the aggregate-time reduce must produce exactly 5 ablation rows")
	got := renderAblations(rep.Ablations, rep.Footer)
	assertGolden(t, "ablations.golden.md", got)

	// no_semantic is genuinely computed at aggregate-time (present, not em-dash).
	var noSem AblationRow
	for _, r := range rep.Ablations {
		if r.Comparison == "full_minus_no_semantic" {
			noSem = r
		}
	}
	assert.True(t, noSem.Present, "full vs no_semantic must be computed at aggregate-time (present)")
	assert.True(t, noSem.FullCI.OK && noSem.OtherCI.OK, "both no_semantic operands present in loaded")
	_ = strings.TrimSpace
}
