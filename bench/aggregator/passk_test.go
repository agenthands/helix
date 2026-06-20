package aggregator

import (
	"math"
	"testing"
)

// passAtKLog is an INDEPENDENT derivation of the HumanEval unbiased pass@k,
// computed in log-space via math.Lgamma (log-binomial). It is defined HERE in
// the test — not imported from the implementation — so the form-agreement test
// cross-checks two genuinely independent code paths (D-11). The implementation's
// product form is the PRIMARY computation; this log-binomial form must agree
// with it to ~1e-12.
func passAtKLog(n, c, k int) float64 {
	if k > n {
		return 1.0
	}
	if n-c < k {
		return 1.0
	}
	// logBinom returns ln C(a,b) via lgamma; -Inf for b<0 || b>a.
	logBinomLocal := func(a, b int) float64 {
		if b < 0 || b > a {
			return math.Inf(-1)
		}
		la, _ := math.Lgamma(float64(a) + 1)
		lb, _ := math.Lgamma(float64(b) + 1)
		lab, _ := math.Lgamma(float64(a-b) + 1)
		return la - lb - lab
	}
	// 1 - C(n-c,k)/C(n,k) = 1 - exp(logBinom(n-c,k) - logBinom(n,k)).
	return 1.0 - math.Exp(logBinomLocal(n-c, k)-logBinomLocal(n, k))
}

// TestPassAtK asserts PassAtK against published + hand-computed reference values
// (STATS-03). The (10,3,5)->0.91667 case is the load-bearing k>=2 anchor: it
// catches BOTH the naive 1-(1-p)^k estimator (which gives 0.83193) AND the
// c-vs-k loop-count bug (a k-term loop gives 0.97348). k=1-only cases cannot
// catch the loop bug because the product runs once either way at k=1.
func TestPassAtK(t *testing.T) {
	const eps = 1e-9
	cases := []struct {
		name    string
		n, c, k int
		want    float64
	}{
		// Load-bearing k>=2 anti-naive AND anti-k-term-loop anchor: 11/12.
		// (1-5/8)(1-5/9)(1-5/10) = 0.375*0.444444*0.5 = 1/12 -> 1-1/12 = 0.91667.
		{"anchor_10_3_5", 10, 3, 5, 11.0 / 12.0},
		// pass@1 == c/n success-rate identity.
		{"pass1_5_1_1", 5, 1, 1, 0.2},
		{"pass1_7_3_1", 7, 3, 1, 3.0 / 7.0},
		{"pass1_10_7_1", 10, 7, 1, 0.7},
		{"pass1_4_4_1", 4, 4, 1, 1.0},
		// 1 - C(3,2)/C(5,2) = 1 - 3/10 = 0.7.
		{"k2_5_2_2", 5, 2, 2, 0.7},
		// No correct samples -> 0.0 (C(n,k)/C(n,k) = 1).
		{"zero_c_8_0_3", 8, 0, 3, 0.0},
		{"zero_c_5_0_1", 5, 0, 1, 0.0},
		// n-c < k early-return branch -> 1.0.
		{"early_5_5_3", 5, 5, 3, 1.0},
		{"early_5_4_2", 5, 4, 2, 1.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PassAtK(tc.n, tc.c, tc.k)
			if math.Abs(got-tc.want) > eps {
				t.Fatalf("PassAtK(%d,%d,%d) = %.10f, want %.10f", tc.n, tc.c, tc.k, got, tc.want)
			}
		})
	}
}

// TestPassAtKDomainGuard locks WR-02: the k>n branch must NOT fabricate 1.0 for
// c<n (the estimator is undefined out of its k<=n domain). The only k>n input
// with a guaranteed answer is c==n (every draw correct -> 1.0); any other k>n is
// an out-of-contract caller bug surfaced as NaN, never a silent wrong 1.0.
func TestPassAtKDomainGuard(t *testing.T) {
	// c < n with k > n MUST be NaN, not the old wrong 1.0.
	for _, tc := range []struct{ n, c, k int }{
		{2, 0, 3}, // zero correct samples -> must not claim success
		{3, 1, 4},
		{5, 4, 6},
	} {
		got := PassAtK(tc.n, tc.c, tc.k)
		if !math.IsNaN(got) {
			t.Fatalf("PassAtK(%d,%d,%d) = %.10f, want NaN (estimator undefined for k>n, c<n)",
				tc.n, tc.c, tc.k, got)
		}
	}
	// c == n with k > n is the one guaranteed case: every draw is correct -> 1.0.
	for _, tc := range []struct{ n, c, k int }{
		{2, 2, 3},
		{4, 4, 10},
	} {
		got := PassAtK(tc.n, tc.c, tc.k)
		if math.Abs(got-1.0) > 1e-12 {
			t.Fatalf("PassAtK(%d,%d,%d) = %.10f, want 1.0 (c==n, all draws correct)",
				tc.n, tc.c, tc.k, got)
		}
	}
}

// TestPassAtKAntiNaive locks Pitfall 1: the result for the k>=2 anchor MUST NOT
// equal the biased naive estimator 1-(1-c/n)^k. This is an explicit guard that
// the implementation did not silently regress to the forbidden form.
func TestPassAtKAntiNaive(t *testing.T) {
	const eps = 1e-9
	naive := 1.0 - math.Pow(1.0-3.0/10.0, 5) // = 1 - 0.7^5 = 0.83193
	got := PassAtK(10, 3, 5)
	if math.Abs(got-naive) <= eps {
		t.Fatalf("PassAtK(10,3,5) = %.10f equals the FORBIDDEN naive estimator %.10f; "+
			"the unbiased form must give 0.91667", got, naive)
	}
	if math.Abs(got-0.91667) > 1e-4 {
		t.Fatalf("PassAtK(10,3,5) = %.10f, want ~0.91667 (the unbiased anchor)", got)
	}
}

// TestPassAtKFormAgreement (D-11) cross-checks the implementation's PRIMARY
// product form against an INDEPENDENT log-binomial (lgamma) derivation computed
// in the test, across a grid. Two independent derivations agreeing to ~1e-12 is
// strong correctness evidence; a k-vs-c loop bug would break the agreement.
func TestPassAtKFormAgreement(t *testing.T) {
	const tol = 1e-12
	for n := 1; n <= 12; n++ {
		for c := 0; c <= n; c++ {
			for k := 1; k <= n; k++ {
				prod := PassAtK(n, c, k)
				lg := passAtKLog(n, c, k)
				if math.Abs(prod-lg) > tol {
					t.Fatalf("form disagreement at (n=%d,c=%d,k=%d): product=%.15f lgamma=%.15f (diff %.3e)",
						n, c, k, prod, lg, math.Abs(prod-lg))
				}
			}
		}
	}
}
