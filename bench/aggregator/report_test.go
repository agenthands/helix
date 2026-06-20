package aggregator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// report_test.go unit-tests the render layer with INJECTED row data so the
// markdown assertions do NOT ride on bootstrap randomness (Task 1 RECOMMENDED
// path). The overlap predicate, CV detector, leaderboard render (sort + null
// em-dash + STATS-04 overlap warning) and cost_quality render (cost CI +
// FAIR-03 CV warning + valid_until footer) are each pinned here.

// ci builds a present (ok==true) ciValue.
func ci(point, lo, hi float64) ciValue { return ciValue{Point: point, Lo: lo, Hi: hi, OK: true} }

// nullCI builds an absent (ok==false) ciValue that must render as an em-dash.
func nullCI() ciValue { return ciValue{OK: false} }

// testFooter is a fixed footer for deterministic render assertions.
func testFooter() Footer {
	return Footer{
		Seed:                42,
		Iterations:          10000,
		CILevel:             0.95,
		Runs:                3,
		CostTableValidUntil: "2027-01-28",
	}
}

// TestOverlapGate locks the STATS-04 interval-intersection predicate AND the
// leaderboard annotation: adjacent rows whose sort-metric BCa CIs overlap are
// flagged ("CI overlap"); non-overlapping neighbors are not.
func TestOverlapGate(t *testing.T) {
	t.Run("predicate matches the interval-intersection rule", func(t *testing.T) {
		// A 0.70 [0.55,0.85] vs B 0.60 [0.45,0.75]: overlap because 0.55 <= 0.75.
		assert.True(t, ciOverlap(ci(0.70, 0.55, 0.85), ci(0.60, 0.45, 0.75)))
		// A 0.90 [0.85,0.95] vs B 0.50 [0.40,0.60]: disjoint, no overlap.
		assert.False(t, ciOverlap(ci(0.90, 0.85, 0.95), ci(0.50, 0.40, 0.60)))
		// Touching endpoints (lo_a == hi_b) count as overlap (<= is inclusive).
		assert.True(t, ciOverlap(ci(0.80, 0.60, 0.90), ci(0.50, 0.40, 0.60)))
		// A null CI never participates in an overlap claim (can't be compared).
		assert.False(t, ciOverlap(nullCI(), ci(0.50, 0.40, 0.60)))
		assert.False(t, ciOverlap(ci(0.50, 0.40, 0.60), nullCI()))
	})

	t.Run("overlapping neighbors render the warning", func(t *testing.T) {
		rows := []LeaderRow{
			{Mode: "full", Benchmark: "internal-toolbench", TaskSuccess: ci(0.70, 0.55, 0.85)},
			{Mode: "no_lsp", Benchmark: "internal-toolbench", TaskSuccess: ci(0.60, 0.45, 0.75)},
		}
		md := renderLeaderboard(rows, testFooter())
		assert.Contains(t, md, "CI overlap",
			"adjacent overlapping task_success CIs must render the STATS-04 warning")
		assert.Contains(t, strings.ToLower(md), "no x>y",
			"overlap annotation must suppress the directional superiority claim")
	})

	t.Run("non-overlapping neighbors render NO warning", func(t *testing.T) {
		rows := []LeaderRow{
			{Mode: "full", Benchmark: "internal-toolbench", TaskSuccess: ci(0.90, 0.85, 0.95)},
			{Mode: "no_lsp", Benchmark: "internal-toolbench", TaskSuccess: ci(0.50, 0.40, 0.60)},
		}
		md := renderLeaderboard(rows, testFooter())
		assert.NotContains(t, md, "CI overlap",
			"disjoint task_success CIs must NOT render the overlap warning")
	})
}

// TestLeaderboardRender locks the column/sort/null-em-dash/footer contract (D-16).
func TestLeaderboardRender(t *testing.T) {
	rows := []LeaderRow{
		// Deliberately given OUT of sort order to prove sort-before-emit.
		{Mode: "no_lsp", Benchmark: "internal-toolbench",
			TaskSuccess: ci(0.50, 0.40, 0.60), PassAt1: ci(0.50, 0.40, 0.60),
			PassAtN: ci(0.66, 0.50, 0.80), TokensInput: nullCI()},
		{Mode: "full", Benchmark: "internal-toolbench",
			TaskSuccess: ci(0.90, 0.85, 0.95), PassAt1: ci(0.90, 0.85, 0.95),
			PassAtN: ci(0.95, 0.90, 0.99), TokensInput: ci(1000, 900, 1100)},
	}
	md := renderLeaderboard(rows, testFooter())

	// Sorted task_success DESC: "full" (0.90) precedes "no_lsp" (0.50).
	fullIdx := strings.Index(md, "full")
	noLspIdx := strings.Index(md, "no_lsp")
	require.GreaterOrEqual(t, fullIdx, 0)
	require.GreaterOrEqual(t, noLspIdx, 0)
	assert.Less(t, fullIdx, noLspIdx, "rows must be sorted by task_success descending")

	// Null CI renders an em-dash, never a fabricated 0.
	assert.Contains(t, md, "—", "a null CI metric must render an em-dash")

	// Footer provenance fields.
	assert.Contains(t, md, "10000", "footer records bootstrap iterations")
	assert.Contains(t, md, "0.95", "footer records ci_level")
	assert.Contains(t, md, "2027-01-28", "footer cites the cost-table valid_until")
}

// TestCV locks the FAIR-03 coefficient-of-variation detector: CV = stddev/mean
// of per-run USD across a (task,mode)'s N runs (A2 choice).
func TestCV(t *testing.T) {
	// Low-variance: 1.00, 1.01, 0.99 -> CV well under 0.05.
	low := coefVariation([]float64{1.00, 1.01, 0.99})
	assert.Less(t, low, 0.05, "near-identical per-run USD is low CV")

	// High-variance: 1.00, 2.00, 3.00 -> CV well over 0.05.
	high := coefVariation([]float64{1.00, 2.00, 3.00})
	assert.Greater(t, high, 0.05, "widely varying per-run USD is high CV")

	// Degenerate guards: empty and zero-mean never panic / NaN-leak.
	assert.Equal(t, 0.0, coefVariation(nil))
	assert.Equal(t, 0.0, coefVariation([]float64{0, 0, 0}))
}

// TestCostQualityRender locks COST-03 (cost_per_solved_task value [lo,hi]),
// the FAIR-03 high-variance warning naming the cell, and the valid_until footer.
func TestCostQualityRender(t *testing.T) {
	t.Run("high-variance cell renders the FAIR-03 warning", func(t *testing.T) {
		rows := []CostRow{
			{Mode: "full", Benchmark: "internal-toolbench",
				CostPerSolved: ci(3.55, 3.10, 4.00),
				VarianceFlags: []VarianceFlag{{Task: "task-7", Mode: "full", CV: 0.42}}},
		}
		md := renderCostQuality(rows, testFooter())
		assert.Contains(t, md, "cost_per_solved_task")
		assert.Contains(t, md, "task-7", "the variance warning must name the offending cell")
		assert.Contains(t, strings.ToLower(md), "variance",
			"a CV>0.05 cell must render a FAIR-03 variance warning")
		assert.Contains(t, md, "2027-01-28", "footer cites the cost-table valid_until")
	})

	t.Run("low-variance cell renders no FAIR-03 warning", func(t *testing.T) {
		rows := []CostRow{
			{Mode: "full", Benchmark: "internal-toolbench",
				CostPerSolved: ci(3.55, 3.10, 4.00), VarianceFlags: nil},
		}
		md := renderCostQuality(rows, testFooter())
		assert.NotContains(t, strings.ToLower(md), "variance",
			"a low-variance run must NOT render a FAIR-03 warning")
	})

	t.Run("null cost CI renders an em-dash", func(t *testing.T) {
		rows := []CostRow{
			{Mode: "no_lsp", Benchmark: "internal-toolbench", CostPerSolved: nullCI()},
		}
		md := renderCostQuality(rows, testFooter())
		assert.Contains(t, md, "—", "a null cost CI must render an em-dash, never $0")
	})
}
