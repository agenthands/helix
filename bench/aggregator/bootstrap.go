// Package aggregator computes multi-run benchmark aggregates: BCa bootstrap
// confidence intervals, unbiased pass@k, and cost rollups over a run directory.
//
// This file implements the BCa (bias-corrected accelerated) bootstrap CI — the
// statistical heart of STATS-02. It is a PROPER BCa, not a percentile-only or
// normal-approximation bootstrap (D-06): the endpoints are adjusted by a
// bias-correction term z0 (from the proportion of bootstrap replicates below the
// observed statistic) AND an acceleration term a (the skewness of the jackknife
// leave-one-out distribution). All randomness flows through an injected
// *rand.Rand so the same seed yields byte-identical output (D-08), and every
// degenerate case (empty / all-identical / single-element) is handled without a
// crash or NaN/Inf (D-09).
//
// References: Efron & Tibshirani, "An Introduction to the Bootstrap" (1993),
// §14.3 (z0), eq. 14.15 (jackknife a), eq. 14.10 (BCa endpoint adjustment).
package aggregator

import (
	"math"
	"math/rand/v2"
	"sort"
)

// phiInv is the inverse standard-normal CDF, Φ⁻¹(p), via the stdlib error
// function: Φ⁻¹(p) = √2 · erfinv(2p − 1). Exact to machine precision.
func phiInv(p float64) float64 { return math.Sqrt2 * math.Erfinv(2*p-1) }

// phi is the standard-normal CDF, Φ(z) = 0.5 · erfc(−z/√2).
func phi(z float64) float64 { return 0.5 * math.Erfc(-z/math.Sqrt2) }

// clamp01 confines p to [0,1]; extreme z0/a can push the BCa-adjusted
// percentiles slightly outside the unit interval, which we guard against.
func clamp01(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// StatMean is the most common per-task statistic for a leaderboard cell: the
// arithmetic mean of the per-task value vector. Exported so callers (Plan 06)
// can pass it directly to BCaInterval.
func StatMean(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	var s float64
	for _, v := range x {
		s += v
	}
	return s / float64(len(x))
}

// BCaInterval computes the bias-corrected accelerated (BCa) bootstrap confidence
// interval for stat(vals) at confidence level 1−alpha (e.g. alpha=0.05 → 95% CI).
//
//	vals  the resampling units (the per-task statistic vector); bootstrapping
//	      resamples ACROSS these units (D-07).
//	stat  the statistic to estimate (e.g. StatMean).
//	B     number of bootstrap replicates (>= 10000 per D-07).
//	alpha two-tail level; the interval spans [alpha/2, 1-alpha/2].
//	rng   a seeded generator (rand.New(rand.NewPCG(seed, seed2))) — REQUIRED for
//	      deterministic output (D-08); BCaInterval never touches global rand state.
//
// Returns (lo, hi, ok). ok==false signals a NULL CI — only for an empty vals
// (the metric was nil across all tasks); the caller renders such a cell as "—",
// never as a fabricated [0,0] (D-09). Degenerate-but-non-empty inputs return a
// point CI [θ̂, θ̂] with ok==true:
//   - all-identical vals (zero bootstrap spread): point CI.
//   - m == 1: jackknife acceleration is undefined → a=0, and a single value
//     resamples to itself → point CI.
//
// Quantile rule (locked for byte-stable reproduction, A5): nearest-rank over the
// sorted bootstrap distribution, idx = clamp(round(p·(B−1)), 0, B−1). This is a
// PROPER BCa — it computes z0 and the jackknife a; it is NOT a percentile
// bootstrap (Pitfall 2).
func BCaInterval(vals []float64, stat func([]float64) float64, B int, alpha float64, rng *rand.Rand) (lo, hi float64, ok bool) {
	m := len(vals)
	if m == 0 {
		// Null CI: the metric was nil across all tasks. Never fabricate [0,0].
		return 0, 0, false
	}

	thetaHat := stat(vals)

	// Step 1 — B bootstrap replicates by resampling m indices with replacement
	// via the injected RNG; collect and sort theta-star.
	thetaStar := bootstrapReplicates(vals, stat, B, rng)
	sort.Float64s(thetaStar)

	// Step 2 — bias correction z0 from the fraction of replicates below thetaHat.
	// An all-identical bootstrap distribution (p0 in {0,1}) means zero spread →
	// return the point CI early (avoids Φ⁻¹(±∞)).
	below := 0
	for _, ts := range thetaStar {
		if ts < thetaHat {
			below++
		}
	}
	p0 := float64(below) / float64(len(thetaStar))
	if p0 <= 0 || p0 >= 1 {
		return thetaHat, thetaHat, true
	}
	z0 := phiInv(p0)

	// Step 3 — acceleration a via the jackknife leave-one-out skewness. m==1 or a
	// zero-spread jackknife distribution yields a=0 (bias-corrected percentile).
	a := jackknifeAccel(vals, stat)

	// Step 4 — BCa-adjusted percentile levels.
	a1, a2 := bcaPercentiles(z0, a, alpha)

	// Step 5 — read endpoints off the sorted bootstrap distribution.
	lo = quantile(thetaStar, a1)
	hi = quantile(thetaStar, a2)
	return lo, hi, true
}

// bootstrapReplicates draws B resamples of size m (with replacement) from vals
// using rng, returning the statistic of each resample.
func bootstrapReplicates(vals []float64, stat func([]float64) float64, B int, rng *rand.Rand) []float64 {
	m := len(vals)
	out := make([]float64, B)
	resample := make([]float64, m)
	for b := 0; b < B; b++ {
		for i := 0; i < m; i++ {
			resample[i] = vals[rng.IntN(m)]
		}
		out[b] = stat(resample)
	}
	return out
}

// jackknifeAccel computes the acceleration a as the skewness of the leave-one-out
// jackknife distribution (Efron & Tibshirani 1993, eq. 14.15):
//
//	a = Σ(θ̄ − θ̂_(i))³ / (6 · (Σ(θ̄ − θ̂_(i))²)^(3/2))
//
// Degenerate guard: if the denominator is 0 (m==1, or all jackknife replicates
// identical) return a=0 — never divide by zero.
func jackknifeAccel(vals []float64, stat func([]float64) float64) float64 {
	m := len(vals)
	if m < 2 {
		return 0
	}
	jack := make([]float64, m)
	loo := make([]float64, 0, m-1)
	for i := 0; i < m; i++ {
		loo = loo[:0]
		for j := 0; j < m; j++ {
			if j != i {
				loo = append(loo, vals[j])
			}
		}
		jack[i] = stat(loo)
	}
	var mean float64
	for _, v := range jack {
		mean += v
	}
	mean /= float64(m)

	var num, sumSq float64
	for _, v := range jack {
		d := mean - v
		num += d * d * d
		sumSq += d * d
	}
	den := 6 * math.Pow(sumSq, 1.5)
	if den == 0 {
		return 0
	}
	return num / den
}

// bcaPercentiles maps the two-tail level alpha to BCa-adjusted percentile levels
// (a1, a2) using bias-correction z0 and acceleration a (Efron & Tibshirani 1993,
// eq. 14.10). When the adjustment denominator collapses toward 0 it falls back to
// the bias-corrected (no-acceleration) form for that endpoint, and the result is
// clamped into [0,1].
func bcaPercentiles(z0, a, alpha float64) (a1, a2 float64) {
	zlo, zhi := phiInv(alpha/2), phiInv(1-alpha/2)
	adj := func(z float64) float64 {
		den := 1 - a*(z0+z)
		if math.Abs(den) < 1e-12 {
			return phi(z0 + (z0 + z)) // Pitfall 6 guard: no division by ~0.
		}
		return phi(z0 + (z0+z)/den)
	}
	return clamp01(adj(zlo)), clamp01(adj(zhi))
}

// quantile reads the value at percentile p from an already-sorted slice using the
// nearest-rank rule idx = clamp(round(p·(n−1)), 0, n−1). Locked for byte-stable
// reproduction (A5).
func quantile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	idx := int(math.Round(p * float64(n-1)))
	if idx < 0 {
		idx = 0
	}
	if idx > n-1 {
		idx = n - 1
	}
	return sorted[idx]
}
