package aggregator

import (
	"math"
	"testing"
	"time"

	"github.com/agenthands/helix/bench/cost"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goldenToday is the INJECTED clock for every cost test. The golden fixture
// (testdata/cost-table.golden.yaml) is dated fresh relative to this date so the
// D-13 freshness gate passes for the golden math; the stale fixtures are dated
// to TRIP the gate against the same today (determinism, D-11/D-13).
func goldenToday(t *testing.T) time.Time {
	t.Helper()
	d, err := time.Parse(cost.DateLayout, "2026-06-20")
	require.NoError(t, err)
	return d
}

// loadGoldenRow loads the frozen golden cost table and prices the COST-02 model
// through the real bench/cost freshness gate (one source of truth — Open Q1).
func loadGoldenRow(t *testing.T) cost.CostRow {
	t.Helper()
	ct, err := cost.LoadCostTable("testdata/cost-table.golden.yaml")
	require.NoError(t, err)
	row, err := cost.PriceFor(ct, "claude-sonnet-4-5-20250929", goldenToday(t))
	require.NoError(t, err, "golden fixture must pass the freshness gate")
	return row
}

// TestCostPerSolved is the COST-02 anchor: the hand-computed 3.555 USD golden,
// the solved-only filter (no solved -> null), and the nil-token exclusion
// (Pitfall 4 — a fully-nil result has no computable cost, never $0).
func TestCostPerSolved(t *testing.T) {
	row := loadGoldenRow(t)

	t.Run("golden 3.555 USD per solved task", func(t *testing.T) {
		// T1: ti=1_000_000, cr=0, cw=0, to=100_000
		//   = (1_000_000*3.0 + 0*0.30 + 0*3.0 + 100_000*15.0)/1e6 = 4.50
		t1 := rowMetrics{
			TaskSuccess:           ptrBool(true),
			TokensInput:           ptrInt(1_000_000),
			TokensInputCachedRead: ptrInt(0),
			TokensInputCacheWrite: ptrInt(0),
			TokensOutput:          ptrInt(100_000),
		}
		usd1, ok := perResultUSD(t1, row)
		require.True(t, ok)
		assert.InDelta(t, 4.50, usd1, 1e-9)

		// T2: ti=500_000, cr=200_000, cw=100_000, to=50_000
		//   = (500_000*3.0 + 200_000*0.30 + 100_000*3.0 + 50_000*15.0)/1e6
		//   = (1_500_000 + 60_000 + 300_000 + 750_000)/1e6 = 2.61
		//   (cache-write priced at the INPUT rate per D-14: 100_000*3.0/1e6 = 0.30)
		t2 := rowMetrics{
			TaskSuccess:           ptrBool(true),
			TokensInput:           ptrInt(500_000),
			TokensInputCachedRead: ptrInt(200_000),
			TokensInputCacheWrite: ptrInt(100_000),
			TokensOutput:          ptrInt(50_000),
		}
		usd2, ok := perResultUSD(t2, row)
		require.True(t, ok)
		assert.InDelta(t, 2.61, usd2, 1e-9)

		// cost_per_solved_task = (4.50 + 2.61)/2 = 3.555
		got, ok := costPerSolvedTask(map[string]float64{"t1": usd1, "t2": usd2})
		require.True(t, ok)
		assert.InDelta(t, 3.555, got, 1e-9)
	})

	t.Run("no solved task -> null, not NaN", func(t *testing.T) {
		got, ok := costPerSolvedTask(map[string]float64{})
		assert.False(t, ok, "empty solved-task map yields ok==false")
		assert.False(t, math.IsNaN(got), "must be null/omitted, never NaN")
	})

	t.Run("fully-nil tokens -> ok==false (never fabricated $0)", func(t *testing.T) {
		m := rowMetrics{TaskSuccess: ptrBool(true)} // all token fields nil
		usd, ok := perResultUSD(m, row)
		require.False(t, ok, "no computable cost -> ok==false")
		assert.Equal(t, 0.0, usd, "returns the zero value but ok==false marks it absent")
	})

	t.Run("partial-nil tokens price only the present terms", func(t *testing.T) {
		// only tokens_output present; the other three terms are skipped.
		m := rowMetrics{TaskSuccess: ptrBool(true), TokensOutput: ptrInt(100_000)}
		usd, ok := perResultUSD(m, row)
		require.True(t, ok, "a present field still yields a computable cost")
		assert.InDelta(t, 1.50, usd, 1e-9) // 100_000*15.0/1e6
	})
}

// TestCostFreshnessGate proves the join goes through the fail-closed D-13 gate:
// a past valid_until, a >90d-stale last_verified, and an unknown model_id are
// all hard errors against the INJECTED today.
func TestCostFreshnessGate(t *testing.T) {
	today := goldenToday(t)

	t.Run("past valid_until is a hard error", func(t *testing.T) {
		ct, err := cost.LoadCostTable("testdata/cost-table.past-valid-until.yaml")
		require.NoError(t, err)
		_, err = cost.PriceFor(ct, "claude-sonnet-4-5-20250929", today)
		require.Error(t, err, "past valid_until must fail closed")
		assert.Contains(t, err.Error(), "valid_until")
	})

	t.Run("stale last_verified is a hard error", func(t *testing.T) {
		ct, err := cost.LoadCostTable("testdata/cost-table.stale-verified.yaml")
		require.NoError(t, err)
		_, err = cost.PriceFor(ct, "claude-sonnet-4-5-20250929", today)
		require.Error(t, err, ">90d stale last_verified must fail closed")
		assert.Contains(t, err.Error(), "last_verified")
	})

	t.Run("unknown model_id is a hard error", func(t *testing.T) {
		ct, err := cost.LoadCostTable("testdata/cost-table.golden.yaml")
		require.NoError(t, err)
		_, err = cost.PriceFor(ct, "gpt-nonexistent-9999", today)
		require.Error(t, err, "unknown model_id cannot be priced -> hard error")
		assert.Contains(t, err.Error(), "no row for model_id")
	})
}
