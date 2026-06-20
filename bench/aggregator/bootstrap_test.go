package aggregator

import (
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedRNG builds the canonical deterministic PCG generator used throughout the
// BCa tests. The second stream constant mirrors the implementation contract
// (PATTERNS §Seeded RNG) so callers that pass a fresh seed get reproducible draws.
func seedRNG(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
}

// statMean is the per-task statistic used in the tests (the leaderboard mean).
func statMean(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	var s float64
	for _, v := range x {
		s += v
	}
	return s / float64(len(x))
}

// percentileInterval computes a PLAIN percentile bootstrap interval over the
// SAME bootstrap distribution that BCaInterval would build, using the SAME
// nearest-rank quantile rule. It deliberately omits z0 and acceleration so that
// a fake-BCa (percentile bootstrap masquerading as BCa) produces endpoints
// identical to this helper — which the skew test then rejects.
func percentileInterval(t *testing.T, vals []float64, stat func([]float64) float64, B int, alpha float64, rng *rand.Rand) (lo, hi float64) {
	t.Helper()
	m := len(vals)
	require.Greater(t, m, 0, "percentileInterval needs a non-empty sample")
	thetaStar := make([]float64, B)
	resample := make([]float64, m)
	for b := 0; b < B; b++ {
		for i := 0; i < m; i++ {
			resample[i] = vals[rng.IntN(m)]
		}
		thetaStar[b] = stat(resample)
	}
	sort.Float64s(thetaStar)
	q := func(p float64) float64 {
		idx := int(math.Round(p * float64(B-1)))
		if idx < 0 {
			idx = 0
		}
		if idx > B-1 {
			idx = B - 1
		}
		return thetaStar[idx]
	}
	return q(alpha / 2), q(1 - alpha/2)
}

const (
	testB     = 10000
	testAlpha = 0.05
)

// TestBCa is the STATS-02 acceptance suite for the BCa bootstrap primitive.
// It encodes the six behavior groups from 82-02-PLAN: skew-divergence (the
// fake-BCa discriminator), symmetric-convergence, parameter containment,
// width sanity (incl. shrink-with-m), determinism (D-08), and the D-09
// degenerate matrix (empty / all-identical / m==1).
func TestBCa(t *testing.T) {
	// --- Group 1: BCa != percentile on a deliberately right-skewed sample. ---
	// This is the discriminating signal a percentile-bootstrap-masquerading-as-BCa
	// CANNOT reproduce: z0 (from #{theta*<theta_hat}/B) and jackknife a must shift
	// the endpoints away from the plain percentile cut points.
	t.Run("BCa_differs_from_percentile_on_skew", func(t *testing.T) {
		skewed := []float64{1, 1, 1, 2, 2, 3, 5, 8, 13}

		bcaLo, bcaHi, ok := BCaInterval(skewed, statMean, testB, testAlpha, seedRNG(42))
		require.True(t, ok, "non-empty sample must yield a CI")

		// Build the percentile interval over the SAME bootstrap distribution by
		// re-seeding identically — same draws, same sorted distribution, same
		// quantile rule — so the ONLY difference is the z0/a adjustment.
		pctlLo, pctlHi := percentileInterval(t, skewed, statMean, testB, testAlpha, seedRNG(42))

		diverged := math.Abs(bcaLo-pctlLo) > 1e-6 || math.Abs(bcaHi-pctlHi) > 1e-6
		assert.Truef(t, diverged,
			"BCa endpoints must differ from percentile on a skewed sample (fake-BCa check): bca=[%g,%g] pctl=[%g,%g]",
			bcaLo, bcaHi, pctlLo, pctlHi)

		// And the BCa interval must still contain the observed mean.
		mean := statMean(skewed)
		assert.LessOrEqual(t, bcaLo, mean, "lo must be <= observed mean")
		assert.GreaterOrEqual(t, bcaHi, mean, "hi must be >= observed mean")
		assert.False(t, math.IsNaN(bcaLo) || math.IsNaN(bcaHi), "no NaN endpoints")
		assert.False(t, math.IsInf(bcaLo, 0) || math.IsInf(bcaHi, 0), "no Inf endpoints")
	})

	// --- Group 2: symmetric sample => z0~0, a~0 => BCa ~ percentile. ---
	t.Run("BCa_matches_percentile_on_symmetric", func(t *testing.T) {
		symmetric := []float64{-4, -3, -2, -1, 0, 1, 2, 3, 4}

		bcaLo, bcaHi, ok := BCaInterval(symmetric, statMean, testB, testAlpha, seedRNG(7))
		require.True(t, ok)
		pctlLo, pctlHi := percentileInterval(t, symmetric, statMean, testB, testAlpha, seedRNG(7))

		// For a symmetric sample z0 and a vanish, so endpoints should land very
		// close to the plain percentile cut points (small tolerance for the
		// residual z0/a wobble from a finite B).
		assert.InDeltaf(t, pctlLo, bcaLo, 0.35, "symmetric lo: bca %g vs pctl %g", bcaLo, pctlLo)
		assert.InDeltaf(t, pctlHi, bcaHi, 0.35, "symmetric hi: bca %g vs pctl %g", bcaHi, pctlHi)
	})

	// --- Group 3: containment of a known parameter (mean of a synthetic sample). ---
	t.Run("BCa_contains_true_mean", func(t *testing.T) {
		// Synthetic sample with an exactly-known mean of 10.0.
		vals := []float64{6, 7, 8, 9, 10, 11, 12, 13, 14}
		const trueMean = 10.0
		require.InDelta(t, trueMean, statMean(vals), 1e-9)

		lo, hi, ok := BCaInterval(vals, statMean, testB, testAlpha, seedRNG(101))
		require.True(t, ok)
		assert.LessOrEqual(t, lo, trueMean, "true mean must be >= lo")
		assert.GreaterOrEqual(t, hi, trueMean, "true mean must be <= hi")
	})

	// --- Group 4: width sanity — hi > lo, and width shrinks as m grows. ---
	t.Run("width_sanity_and_shrinks_with_m", func(t *testing.T) {
		small := []float64{2, 4, 6, 8}
		loS, hiS, ok := BCaInterval(small, statMean, testB, testAlpha, seedRNG(5))
		require.True(t, ok)
		assert.Greater(t, hiS, loS, "non-degenerate sample must have hi > lo")

		// Same distribution tiled to a larger m (mean preserved, spread preserved):
		// more "tasks" must tighten the CI of the mean.
		large := []float64{
			2, 4, 6, 8, 2, 4, 6, 8, 2, 4, 6, 8,
			2, 4, 6, 8, 2, 4, 6, 8, 2, 4, 6, 8,
		}
		loL, hiL, ok := BCaInterval(large, statMean, testB, testAlpha, seedRNG(5))
		require.True(t, ok)
		assert.Greater(t, hiL, loL)

		widthSmall := hiS - loS
		widthLarge := hiL - loL
		assert.Lessf(t, widthLarge, widthSmall,
			"CI width must shrink as m grows: small(m=4)=%g large(m=24)=%g", widthSmall, widthLarge)
	})

	// --- Group 5: determinism (D-08) — same seed + same input => identical [lo,hi]. ---
	t.Run("determinism_same_seed", func(t *testing.T) {
		vals := []float64{1, 1, 2, 3, 5, 8, 13, 21}
		lo1, hi1, ok1 := BCaInterval(vals, statMean, testB, testAlpha, seedRNG(999))
		lo2, hi2, ok2 := BCaInterval(vals, statMean, testB, testAlpha, seedRNG(999))
		require.True(t, ok1)
		require.True(t, ok2)
		assert.Equal(t, lo1, lo2, "same seed must yield bit-identical lo")
		assert.Equal(t, hi1, hi2, "same seed must yield bit-identical hi")
	})

	// --- Group 6: degenerate matrix (D-09) — empty / all-identical / m==1. ---
	t.Run("degenerate_empty_returns_null_CI", func(t *testing.T) {
		lo, hi, ok := BCaInterval(nil, statMean, testB, testAlpha, seedRNG(1))
		assert.False(t, ok, "empty vector must signal a null CI (ok==false)")
		assert.False(t, math.IsNaN(lo) || math.IsNaN(hi), "null CI must not be NaN")
		lo, hi, ok = BCaInterval([]float64{}, statMean, testB, testAlpha, seedRNG(1))
		assert.False(t, ok)
		assert.False(t, math.IsNaN(lo) || math.IsNaN(hi))
	})

	t.Run("degenerate_all_identical_point_CI", func(t *testing.T) {
		lo, hi, ok := BCaInterval([]float64{5, 5, 5, 5}, statMean, testB, testAlpha, seedRNG(3))
		require.True(t, ok, "all-identical sample still yields a (point) CI")
		assert.Equal(t, 5.0, lo, "all-identical lo must be the value")
		assert.Equal(t, 5.0, hi, "all-identical hi must be the value")
		assert.False(t, math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0))
	})

	t.Run("degenerate_single_element_point_CI", func(t *testing.T) {
		lo, hi, ok := BCaInterval([]float64{7}, statMean, testB, testAlpha, seedRNG(3))
		require.True(t, ok, "m==1 must not crash; a=0 path yields a point CI")
		assert.Equal(t, 7.0, lo)
		assert.Equal(t, 7.0, hi)
		assert.False(t, math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0))
	})
}

// TestBCaPercentilesCanInvert documents the WR-01 root cause: under an extreme
// bias-correction z0 combined with a sizable acceleration a, the BCa-adjusted
// percentile levels (a1, a2) CROSS (a1 > a2). The enumerated cases mirror the
// review's findings (alpha=0.05). This is the upstream condition the
// BCaInterval ordering guard must absorb.
func TestBCaPercentilesCanInvert(t *testing.T) {
	const alpha = 0.05
	cases := []struct{ z0, a float64 }{
		{-1.00, -0.50},
		{2.00, 1.00},
		{-3.00, -0.50},
	}
	sawInversion := false
	for _, tc := range cases {
		a1, a2 := bcaPercentiles(tc.z0, tc.a, alpha)
		if a1 > a2 {
			sawInversion = true
		}
		// Whatever the order, both levels must stay in [0,1] (clamp discipline).
		assert.GreaterOrEqual(t, a1, 0.0)
		assert.LessOrEqual(t, a1, 1.0)
		assert.GreaterOrEqual(t, a2, 0.0)
		assert.LessOrEqual(t, a2, 1.0)
	}
	require.True(t, sawInversion,
		"the enumerated extreme (z0,a) cases must reproduce an a1>a2 inversion")
}

// TestBCaIntervalOrderingGuard locks WR-01: a sample whose bootstrap-of-means
// distribution drives an extreme z0 (and a non-trivial jackknife a) must NOT
// return an inverted interval (lo > hi). The ordering guard swaps inverted
// endpoints so the published CI always satisfies lo <= hi. We scan many seeds
// and several skewed/discrete samples to surface any inversion the guard must
// catch; with the guard in place, every returned interval is well-ordered.
func TestBCaIntervalOrderingGuard(t *testing.T) {
	samples := [][]float64{
		// Highly skewed discrete sample: drives z0 far from 0 and a non-zero a.
		{0, 0, 0, 0, 0, 0, 0, 0, 1, 100},
		{0, 0, 0, 1, 1, 1, 50},
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1000},
		{0, 0, 0, 0, 0, 1, 1, 2, 3, 80},
	}
	for si, vals := range samples {
		for seed := uint64(1); seed <= 64; seed++ {
			lo, hi, ok := BCaInterval(vals, statMean, testB, testAlpha, seedRNG(seed))
			if !ok {
				continue
			}
			require.LessOrEqualf(t, lo, hi,
				"sample[%d] seed=%d returned inverted CI lo=%.6f hi=%.6f (ordering guard failed)",
				si, seed, lo, hi)
			require.Falsef(t, math.IsNaN(lo) || math.IsNaN(hi),
				"sample[%d] seed=%d produced NaN endpoint", si, seed)
		}
	}
}
