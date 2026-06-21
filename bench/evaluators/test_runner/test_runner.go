// Package test_runner is the Phase 79 exec grader that transforms a
// languages.TestOutcome into the success/correctness/compile-error metrics
// (METRIC-01/02). It owns no subprocess: the TestOutcome is produced by a
// languages.LanguageRunner (e.g. bench/languages/go.GoRunner) and this grader is
// a pure transform over it.
//
// The success gate is exit-code-authoritative (Pitfall 4): task_success and
// verified_correctness are derived from TestOutcome.Passed (defined as exit==0),
// NEVER from the count of per-test rows. A compile failure yields a non-zero exit
// with zero rows; reading "no failing rows ⇒ pass" off that empty slice is the
// exact bug the gate forbids.
package test_runner

import (
	"github.com/agenthands/helix/bench/evaluators"
	"github.com/agenthands/helix/bench/languages"
)

// graderName is the value stamped into every MetricError.Grader this package
// emits (D-07).
const graderName = "test_runner"

// Result is the typed bundle of metrics this grader produces. Every metric is a
// nullable pointer mirroring evaluators.Metrics so a per-metric failure nulls one
// field without dropping the row (D-07); Errs carries the matching annotations.
type Result struct {
	TaskSuccess         *bool
	VerifiedCorrectness *bool
	CompileErrorsBefore *int
	CompileErrorsAfter  *int
	Errs                []evaluators.MetricError
}

// Grade transforms a post-patch TestOutcome (and an optional pre-patch outcome)
// into the success/compile-error metrics.
//
//   - task_success / verified_correctness ← post.Passed (exit==0). Both reflect
//     the same exit-authoritative gate: a run that exits 0 verifiably passed its
//     suite. They are returned as distinct pointers so Plan 04 may later diverge
//     verified_correctness (e.g. a UTBoost rescore) without touching task_success.
//   - compile_errors_after ← the compile-failure signal: a non-zero exit with
//     ZERO parsed test rows is a compile failure (count 1); any other state
//     (exit 0, or non-zero exit with rows ⇒ test failures) is 0.
//   - compile_errors_before ← the same signal applied to the pre-patch outcome
//     when supplied; nil when no pre-patch outcome is threaded in.
func Grade(post languages.TestOutcome, pre *languages.TestOutcome) Result {
	success := post.Passed
	verified := post.Passed
	after := compileErrorCount(post)

	res := Result{
		TaskSuccess:         &success,
		VerifiedCorrectness: &verified,
		CompileErrorsAfter:  &after,
	}

	if pre != nil {
		before := compileErrorCount(*pre)
		res.CompileErrorsBefore = &before
	}

	return res
}

// compileErrorCount returns 1 when the outcome is a compile failure and 0
// otherwise. The compile-failure signal (Pitfall 4) is a non-zero exit code with
// an empty Tests slice: the suite never produced a single per-test event because
// the package failed to build. A non-zero exit WITH rows means the code compiled
// and a test failed (0 compile errors); a zero exit is a clean pass (0).
//
// len(Tests) is consulted ONLY here, to distinguish a compile failure from a test
// failure — it is never used to gate success.
func compileErrorCount(o languages.TestOutcome) int {
	if o.ExitCode != 0 && len(o.Tests) == 0 {
		return 1
	}
	return 0
}
