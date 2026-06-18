package regression_checker

import (
	"testing"

	"github.com/agenthands/helix/bench/languages"
)

func tr(pkg, name string, passed bool) languages.TestResult {
	return languages.TestResult{Package: pkg, Name: name, Passed: passed}
}

func TestRegressionRate(t *testing.T) {
	t.Run("clean patch (all pass pre & post) → 0.0", func(t *testing.T) {
		pre := languages.TestOutcome{Passed: true, Tests: []languages.TestResult{
			tr("p", "A", true), tr("p", "B", true), tr("p", "C", true),
		}}
		post := languages.TestOutcome{Passed: true, Tests: []languages.TestResult{
			tr("p", "A", true), tr("p", "B", true), tr("p", "C", true),
		}}
		rate, mErr := RegressionRate(pre, post)
		if mErr != nil {
			t.Fatalf("unexpected MetricError: %+v", mErr)
		}
		if rate == nil || *rate != 0.0 {
			t.Fatalf("regression_rate = %v, want 0.0", rate)
		}
	})

	t.Run("one regression (B was passing, now fails) → 1/3", func(t *testing.T) {
		pre := languages.TestOutcome{Passed: true, Tests: []languages.TestResult{
			tr("p", "A", true), tr("p", "B", true), tr("p", "C", true),
		}}
		post := languages.TestOutcome{Passed: false, Tests: []languages.TestResult{
			tr("p", "A", true), tr("p", "B", false), tr("p", "C", true),
		}}
		rate, mErr := RegressionRate(pre, post)
		if mErr != nil {
			t.Fatalf("unexpected MetricError: %+v", mErr)
		}
		want := 1.0 / 3.0
		if rate == nil || *rate != want {
			t.Fatalf("regression_rate = %v, want %v", rate, want)
		}
	})

	t.Run("pre-existing failure NOT counted → 0.0", func(t *testing.T) {
		// C was already failing pre-patch; it is not in the cached passing set, so
		// its post-patch failure must not count (Pitfall 6).
		pre := languages.TestOutcome{Passed: false, Tests: []languages.TestResult{
			tr("p", "A", true), tr("p", "B", true), tr("p", "C", false),
		}}
		post := languages.TestOutcome{Passed: false, Tests: []languages.TestResult{
			tr("p", "A", true), tr("p", "B", true), tr("p", "C", false),
		}}
		rate, mErr := RegressionRate(pre, post)
		if mErr != nil {
			t.Fatalf("unexpected MetricError: %+v", mErr)
		}
		if rate == nil || *rate != 0.0 {
			t.Fatalf("regression_rate = %v, want 0.0 (pre-existing failure excluded)", rate)
		}
	})

	t.Run("empty pre-patch passing set → nil + MetricError", func(t *testing.T) {
		pre := languages.TestOutcome{Passed: false, Tests: []languages.TestResult{
			tr("p", "A", false),
		}}
		post := languages.TestOutcome{Passed: false, Tests: []languages.TestResult{
			tr("p", "A", false),
		}}
		rate, mErr := RegressionRate(pre, post)
		if rate != nil {
			t.Fatalf("regression_rate = %v, want nil", *rate)
		}
		if mErr == nil {
			t.Fatalf("want MetricError for undefined denominator, got nil")
		}
		if mErr.Metric != "regression_rate" || mErr.Grader != "regression_checker" {
			t.Fatalf("MetricError = %+v, want metric=regression_rate grader=regression_checker", mErr)
		}
	})
}
