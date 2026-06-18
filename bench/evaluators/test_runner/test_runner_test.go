package test_runner

import (
	"testing"

	"github.com/agenthands/helix/bench/languages"
)

// TestRunnerSuccessGate locks the exit-code-authoritative success gate
// (Pitfall 4): success is derived from TestOutcome.Passed (exit==0), never from
// the presence/absence of per-test rows.
func TestRunnerSuccessGate(t *testing.T) {
	t.Run("all-pass → success & verified true, no compile errors", func(t *testing.T) {
		outcome := languages.TestOutcome{
			Passed:   true,
			ExitCode: 0,
			Tests: []languages.TestResult{
				{Package: "p", Name: "TestA", Passed: true},
				{Package: "p", Name: "TestB", Passed: true},
			},
		}
		g := Grade(outcome, nil)
		if g.Errs != nil {
			t.Fatalf("unexpected errors: %v", g.Errs)
		}
		if g.TaskSuccess == nil || *g.TaskSuccess != true {
			t.Fatalf("task_success = %v, want true", g.TaskSuccess)
		}
		if g.VerifiedCorrectness == nil || *g.VerifiedCorrectness != true {
			t.Fatalf("verified_correctness = %v, want true", g.VerifiedCorrectness)
		}
		if g.CompileErrorsAfter == nil || *g.CompileErrorsAfter != 0 {
			t.Fatalf("compile_errors_after = %v, want 0", g.CompileErrorsAfter)
		}
	})

	t.Run("compile failure (non-zero exit, empty rows) → success false, compile error counted", func(t *testing.T) {
		outcome := languages.TestOutcome{
			Passed:   false,
			ExitCode: 1,
			Tests:    []languages.TestResult{}, // zero rows
		}
		g := Grade(outcome, nil)
		// Pitfall 4: an empty Tests slice must NOT be read as "no failures ⇒ pass".
		if g.TaskSuccess == nil || *g.TaskSuccess != false {
			t.Fatalf("task_success = %v, want false (empty Tests must not infer pass)", g.TaskSuccess)
		}
		if g.CompileErrorsAfter == nil || *g.CompileErrorsAfter == 0 {
			t.Fatalf("compile_errors_after = %v, want non-zero (compile failed)", g.CompileErrorsAfter)
		}
	})

	t.Run("compiles but a test fails → success false, compile_errors_after 0", func(t *testing.T) {
		outcome := languages.TestOutcome{
			Passed:   false,
			ExitCode: 1,
			Tests: []languages.TestResult{
				{Package: "p", Name: "TestA", Passed: false},
				{Package: "p", Name: "TestB", Passed: true},
			},
		}
		g := Grade(outcome, nil)
		if g.TaskSuccess == nil || *g.TaskSuccess != false {
			t.Fatalf("task_success = %v, want false", g.TaskSuccess)
		}
		if g.CompileErrorsAfter == nil || *g.CompileErrorsAfter != 0 {
			t.Fatalf("compile_errors_after = %v, want 0 (it compiled)", g.CompileErrorsAfter)
		}
	})

	t.Run("pre-patch compile failure → compile_errors_before counted", func(t *testing.T) {
		pre := &languages.TestOutcome{Passed: false, ExitCode: 2, Tests: []languages.TestResult{}}
		post := languages.TestOutcome{
			Passed:   true,
			ExitCode: 0,
			Tests:    []languages.TestResult{{Package: "p", Name: "TestA", Passed: true}},
		}
		g := Grade(post, pre)
		if g.CompileErrorsBefore == nil || *g.CompileErrorsBefore == 0 {
			t.Fatalf("compile_errors_before = %v, want non-zero", g.CompileErrorsBefore)
		}
		if g.CompileErrorsAfter == nil || *g.CompileErrorsAfter != 0 {
			t.Fatalf("compile_errors_after = %v, want 0", g.CompileErrorsAfter)
		}
	})
}
