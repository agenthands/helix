// Package regression_checker computes regression_rate (METRIC-05 / D-05) from a
// pre/post pair of languages.TestOutcome values. It is a PURE transform over two
// outcomes — the actual pre-patch RunTests double-run (on the unmodified clone,
// BEFORE the agent's patch is driven) is wired into RunCell by Plan 04; this
// package only consumes the two outcomes it is handed.
//
// The denominator is the CACHED pre-patch passing set: the per-test rows that
// passed BEFORE the patch, keyed on (Package, Name) (D-05). The numerator is the
// count of those cached-passing members that are NOT passing post-patch. A test
// that was ALREADY failing pre-patch is not in the cached set, so its post-patch
// failure must NOT count as a regression (Pitfall 6) — regression_rate measures
// only newly-broken pre-existing tests.
//
// regression_rate = failing_pre-existing_tests_post_patch / passing_pre-existing_tests_pre_patch
//
// An empty pre-patch passing set is an undefined denominator: the rate is nulled
// and annotated with a MetricError (D-07).
package regression_checker

import (
	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/languages"
)

// graderName is stamped into every MetricError this package emits (D-07).
const graderName = "regression_checker"

// metricName is the metric this grader produces.
const metricName = "regression_rate"

// testKey identifies a per-test row across the pre/post runs.
type testKey struct {
	pkg  string
	name string
}

// RegressionRate computes regression_rate from the pre-patch and post-patch
// outcomes. Returns (nil, *MetricError) when the pre-patch passing set is empty
// (undefined denominator); otherwise (rate, nil).
func RegressionRate(prePatch, postPatch languages.TestOutcome) (*float64, *evaluators.MetricError) {
	// Cache the pre-patch passing set (denominator).
	passingPre := make(map[testKey]struct{})
	for _, r := range prePatch.Tests {
		if r.Passed {
			passingPre[testKey{r.Package, r.Name}] = struct{}{}
		}
	}

	denom := len(passingPre)
	if denom == 0 {
		return nil, &evaluators.MetricError{
			Metric: metricName,
			Grader: graderName,
			Reason: "no pre-patch passing tests (undefined denominator)",
		}
	}

	// Index post-patch pass/fail by key. A cached-passing member counts toward the
	// numerator when it is now failing OR absent from the post-patch run (a test
	// that disappeared is no longer passing).
	postPassed := make(map[testKey]bool, len(postPatch.Tests))
	for _, r := range postPatch.Tests {
		postPassed[testKey{r.Package, r.Name}] = r.Passed
	}

	regressed := 0
	for k := range passingPre {
		if !postPassed[k] { // false when failing OR absent post-patch
			regressed++
		}
	}

	rate := float64(regressed) / float64(denom)
	return &rate, nil
}
