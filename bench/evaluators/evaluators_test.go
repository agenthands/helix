package evaluators

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agenthands/helix/bench/languages"
	"github.com/agenthands/helix/internal/eval/trace"
)

// gitRepoWithModification builds a hermetic git repo under t.TempDir(): it
// commits a tracked file, then modifies it (without committing) so
// patch_validator sees a non-empty git-tracked denominator AND a modified file
// in the working tree. Returns the repo dir. Skips the test if git is absent.
func gitRepoWithModification(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.go", "b.go")
	run("commit", "-m", "seed")
	// Modify one tracked file (working-tree change, not committed) so
	// `git diff --name-only` reports it as modified.
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nvar X = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// passingOutcome is a TestOutcome whose Passed gate is true with two passing
// per-test rows (a usable pre-patch passing set for regression_checker).
func passingOutcome() languages.TestOutcome {
	return languages.TestOutcome{
		Passed:   true,
		ExitCode: 0,
		Tests: []languages.TestResult{
			{Package: "p", Name: "TestOne", Passed: true},
			{Package: "p", Name: "TestTwo", Passed: true},
		},
	}
}

// usagePresentTrace is a MergedTrace carrying a provider usage block and a
// non-empty tool-call summary so the trace graders populate.
func usagePresentTrace() trace.MergedTrace {
	return trace.MergedTrace{
		DurationMs: 2500,
		Usage: trace.Usage{
			InputTokens:         1000,
			OutputTokens:        200,
			CacheReadTokens:     50,
			CacheCreationTokens: 10,
		},
		ToolCallSummary: trace.ToolCallSummary{
			Total:  3,
			ByTool: map[string]int{"go_to_definition": 2, "read_file": 1},
		},
	}
}

// allMetricNames enumerates the 17 metric field names on Metrics for the
// all-present assertion. (Two FAIR-03 cached-token columns are part of the 19
// fields; "all 17 metrics present" means every metric field is non-nil.)
func nonNilFieldCount(t *testing.T, m Metrics) int {
	t.Helper()
	v := reflect.ValueOf(m)
	count := 0
	for i := 0; i < v.NumField(); i++ {
		if !v.Field(i).IsNil() {
			count++
		}
	}
	return count
}

// TestAllSeventeenMetrics: a usage-present input over a real git repo with a
// pre-patch passing set produces a Metrics with every field non-nil and no
// metric_errors.
func TestAllSeventeenMetrics(t *testing.T) {
	repo := gitRepoWithModification(t)
	in := GradeInput{
		TestOutcome:         passingOutcome(),
		PrePatchOutcome:     passingOutcome(),
		RepoDir:             repo,
		Merged:              usagePresentTrace(),
		UsagePresent:        true,
		Agent:               "claude",
		CompileErrorsBefore: intPtr(0),
	}

	m, errs := Grade(context.Background(), in)

	total := reflect.TypeOf(m).NumField()
	got := nonNilFieldCount(t, m)
	if got != total {
		t.Fatalf("expected all %d metric fields populated, got %d non-nil; metrics=%+v errs=%+v", total, got, m, errs)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no metric_errors on all-success input, got %d: %+v", len(errs), errs)
	}
}

// TestPerMetricIsolation (D-07): a single grader failure nulls only that
// grader's metric(s) + records exactly that grader's annotation; the rest still
// populate and Grade never returns an error (it has no error return).
func TestPerMetricIsolation(t *testing.T) {
	repo := gitRepoWithModification(t)

	// Single failure: UsagePresent=false forces token_meter to null the 4 token
	// metrics and append exactly one annotation. Everything else populates.
	t.Run("single", func(t *testing.T) {
		in := GradeInput{
			TestOutcome:         passingOutcome(),
			PrePatchOutcome:     passingOutcome(),
			RepoDir:             repo,
			Merged:              usagePresentTrace(),
			UsagePresent:        false, // token_meter fails
			Agent:               "scripted",
			CompileErrorsBefore: intPtr(0),
		}
		m, errs := Grade(context.Background(), in)

		if m.TokensInput != nil || m.TokensOutput != nil ||
			m.TokensInputCachedRead != nil || m.TokensInputCacheWrite != nil {
			t.Fatalf("token metrics must be nil when usage absent: %+v", m)
		}
		// All non-token metrics still populate.
		if m.TaskSuccess == nil || m.ToolCalls == nil || m.EditLocality == nil ||
			m.RegressionRate == nil || m.SemanticToolCalls == nil {
			t.Fatalf("non-token metrics must still populate under D-07 isolation: %+v", m)
		}
		if len(errs) != 1 {
			t.Fatalf("expected exactly one metric_errors entry (token_meter), got %d: %+v", len(errs), errs)
		}
		if errs[0].Grader != "token_meter" {
			t.Fatalf("expected the failed grader to be token_meter, got %q", errs[0].Grader)
		}
	})

	// Multiple failures: usage absent (token_meter) AND a non-git RepoDir
	// (patch_validator's EditLocality + EditDistancePatch) → at least two
	// distinct metric_errors; the rest populate.
	t.Run("multi", func(t *testing.T) {
		in := GradeInput{
			TestOutcome:         passingOutcome(),
			PrePatchOutcome:     passingOutcome(),
			RepoDir:             filepath.Join(t.TempDir(), "not-a-git-repo"),
			Merged:              usagePresentTrace(),
			UsagePresent:        false, // token_meter fails
			Agent:               "scripted",
			CompileErrorsBefore: intPtr(0),
		}
		m, errs := Grade(context.Background(), in)

		if m.EditLocality != nil {
			t.Fatalf("edit_locality must be nil on a non-git repo: %+v", m)
		}
		if m.TokensInput != nil {
			t.Fatalf("tokens_input must be nil when usage absent: %+v", m)
		}
		// Surviving metrics still populate.
		if m.TaskSuccess == nil || m.ToolCalls == nil || m.RegressionRate == nil {
			t.Fatalf("surviving metrics must still populate: %+v", m)
		}
		if len(errs) < 2 {
			t.Fatalf("expected >=2 metric_errors (token_meter + patch_validator), got %d: %+v", len(errs), errs)
		}
	})
}

func intPtr(i int) *int { return &i }
