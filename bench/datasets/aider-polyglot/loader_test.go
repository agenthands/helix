package aiderpolyglot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureWordy is the committed Python fixture exercise dir.
func fixtureWordy(t *testing.T) string {
	t.Helper()
	return filepath.Join("fixtures", "python", "exercises", "practice", "wordy")
}

// TestLoadExercise parses the committed .meta/config.json and returns the
// solution/test/example file lists with NO network (the hermetic authoritative
// proof of the config.json mapping).
func TestLoadExercise(t *testing.T) {
	ex, err := loadExercise(fixtureWordy(t), "python")
	if err != nil {
		t.Fatalf("loadExercise: %v", err)
	}
	if ex.Language != "python" {
		t.Fatalf("Language = %q, want python", ex.Language)
	}
	if got := ex.Config.Files.Solution; len(got) != 1 || got[0] != "wordy.py" {
		t.Fatalf("Files.Solution = %v, want [wordy.py]", got)
	}
	if got := ex.Config.Files.Test; len(got) != 1 || got[0] != "wordy_test.py" {
		t.Fatalf("Files.Test = %v, want [wordy_test.py]", got)
	}
	if got := ex.Config.Files.Example; len(got) != 1 || got[0] != ".meta/example.py" {
		t.Fatalf("Files.Example = %v, want [.meta/example.py]", got)
	}
}

// TestLoadExercisePathSegmentRejected proves an exercise name carrying a
// traversal segment is rejected BEFORE any path Join (T-85-07-02).
func TestLoadExercisePathSegmentRejected(t *testing.T) {
	for _, bad := range []string{"..", "../etc", "a/b", "a\\b", ".hidden"} {
		dir := filepath.Join("fixtures", "python", "exercises", "practice", bad)
		if _, err := loadExercise(dir, "python"); err == nil {
			t.Fatalf("loadExercise(name=%q) = nil err, want rejection", bad)
		}
	}
}

// TestRestorePristine proves the pristine solution stub is restored into the
// work dir so attempt 2 starts clean (read the file back to prove content).
func TestRestorePristine(t *testing.T) {
	srcRoot := fixtureWordy(t)
	workRoot := t.TempDir()
	ex, err := loadExercise(srcRoot, "python")
	if err != nil {
		t.Fatalf("loadExercise: %v", err)
	}
	// Dirty the work copy first (as a prior attempt would).
	solRel := ex.Config.Files.Solution[0]
	if err := os.MkdirAll(filepath.Dir(filepath.Join(workRoot, solRel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workRoot, solRel), []byte("DIRTY"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := restorePristine(ex, srcRoot, workRoot); err != nil {
		t.Fatalf("restorePristine: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(workRoot, solRel))
	if err != nil {
		t.Fatal(err)
	}
	pristine, err := os.ReadFile(filepath.Join(srcRoot, solRel))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(pristine) {
		t.Fatalf("restored content = %q, want pristine %q", got, pristine)
	}
	if strings.Contains(string(got), "DIRTY") {
		t.Fatal("restorePristine did not overwrite the dirtied stub")
	}
}

// fakeAgent records each prompt it sees and returns scripted outcomes.
type fakeAgent struct {
	prompts []string
	calls   int
}

// fakeTester returns a scripted pass/fail and a fixed stderr per attempt.
type fakeTester struct {
	results []TestResult
	calls   int
}

func (f *fakeTester) run(_ context.Context, _ *Exercise, _ string) TestResult {
	r := f.results[f.calls]
	f.calls++
	return r
}

func (f *fakeAgent) edit(_ context.Context, _ *Exercise, _ string, prompt string) error {
	f.prompts = append(f.prompts, prompt)
	f.calls++
	return nil
}

// TestTwoAttemptReprompt proves: fail attempt 1 → re-prompt attempt 2 with the
// captured stderr → pass → exactly 2 attempts (the benchmark.py loop).
func TestTwoAttemptReprompt(t *testing.T) {
	agent := &fakeAgent{}
	tester := &fakeTester{results: []TestResult{
		{Passed: false, Output: "AssertionError: 0 != 5"},
		{Passed: true},
	}}
	ex := &Exercise{Language: "python", Name: "wordy", basePrompt: "implement wordy"}
	res := RunExercise(context.Background(), ex, t.TempDir(), tester.run, agent.edit)
	if res.Attempts != 2 {
		t.Fatalf("Attempts = %d, want 2", res.Attempts)
	}
	if !res.Passed {
		t.Fatal("Passed = false, want true (passed on attempt 2)")
	}
	if agent.calls != 2 {
		t.Fatalf("agent.calls = %d, want 2", agent.calls)
	}
	// Attempt 2's prompt must carry the captured attempt-1 failure text.
	if !strings.Contains(agent.prompts[1], "AssertionError: 0 != 5") {
		t.Fatalf("attempt-2 prompt = %q, want it to carry attempt-1 stderr", agent.prompts[1])
	}
	// Attempt 1's prompt must be the base prompt (no failure text yet).
	if strings.Contains(agent.prompts[0], "AssertionError") {
		t.Fatalf("attempt-1 prompt leaked failure text: %q", agent.prompts[0])
	}
}

// TestTwoAttemptPassFirst proves a pass on attempt 1 breaks the loop (1 attempt).
func TestTwoAttemptPassFirst(t *testing.T) {
	agent := &fakeAgent{}
	tester := &fakeTester{results: []TestResult{{Passed: true}}}
	ex := &Exercise{Language: "python", Name: "wordy", basePrompt: "implement wordy"}
	res := RunExercise(context.Background(), ex, t.TempDir(), tester.run, agent.edit)
	if res.Attempts != 1 || !res.Passed {
		t.Fatalf("got Attempts=%d Passed=%v, want 1/true", res.Attempts, res.Passed)
	}
}

// TestTwoAttemptFailBoth proves failing both attempts runs exactly 2 and reports fail.
func TestTwoAttemptFailBoth(t *testing.T) {
	agent := &fakeAgent{}
	tester := &fakeTester{results: []TestResult{
		{Passed: false, Output: "fail1"},
		{Passed: false, Output: "fail2"},
	}}
	ex := &Exercise{Language: "python", Name: "wordy", basePrompt: "implement wordy"}
	res := RunExercise(context.Background(), ex, t.TempDir(), tester.run, agent.edit)
	if res.Attempts != 2 || res.Passed {
		t.Fatalf("got Attempts=%d Passed=%v, want 2/false", res.Attempts, res.Passed)
	}
}

// TestRunExerciseRestoresTamperedTest proves the WR-01 anti-tamper invariant: an
// agent that edits the graded TEST file has that edit reverted to the pristine
// committed form BEFORE the tests are graded, so test-tampering cannot force a
// spurious pass. It also proves the agent's SOLUTION edit is preserved across the
// run (only the test file is reset).
func TestRunExerciseRestoresTamperedTest(t *testing.T) {
	srcRoot := fixtureWordy(t)
	workRoot := t.TempDir()
	ex, err := loadExercise(srcRoot, "python")
	if err != nil {
		t.Fatalf("loadExercise: %v", err)
	}
	// Seed the work tree with the pristine solution + test (as a real run would
	// before the first attempt).
	if err := restorePristine(ex, srcRoot, workRoot); err != nil {
		t.Fatalf("seed restorePristine: %v", err)
	}
	testRel := ex.Config.Files.Test[0]
	solRel := ex.Config.Files.Solution[0]
	pristineTest, err := os.ReadFile(filepath.Join(srcRoot, testRel))
	if err != nil {
		t.Fatal(err)
	}

	const tamper = "# TAMPERED: assert True\n"
	const solEdit = "# agent solution edit\n"
	// The agent tampers the graded test file AND edits the solution stub.
	tamperingAgent := AgentFn(func(_ context.Context, _ *Exercise, workDir, _ string) error {
		if err := os.WriteFile(filepath.Join(workDir, testRel), []byte(tamper), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(workDir, solRel), []byte(solEdit), 0o644)
	})

	// At grading time the tester inspects the on-disk test file. If the loop
	// restored it, the content is pristine (never the tamper marker).
	var sawTamperAtGrade bool
	grader := func(_ context.Context, _ *Exercise, workDir string) TestResult {
		got, rerr := os.ReadFile(filepath.Join(workDir, testRel))
		if rerr != nil {
			return TestResult{Passed: false, Output: rerr.Error()}
		}
		if strings.Contains(string(got), "TAMPERED") {
			sawTamperAtGrade = true
		}
		if string(got) != string(pristineTest) {
			return TestResult{Passed: false, Output: "test file not pristine at grading"}
		}
		return TestResult{Passed: true}
	}

	res := RunExercise(context.Background(), ex, workRoot, grader, tamperingAgent)
	if sawTamperAtGrade {
		t.Fatal("graded against a tampered test file — restorePristineTests was not wired into the loop")
	}
	if !res.Passed {
		t.Fatalf("RunExercise = %+v, want Passed (test restored to pristine each attempt)", res)
	}
	// The agent's SOLUTION edit must survive (only the test is reset).
	gotSol, err := os.ReadFile(filepath.Join(workRoot, solRel))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotSol) != solEdit {
		t.Fatalf("solution stub = %q, want the agent edit preserved %q", gotSol, solEdit)
	}
}

// TestNativeTestCommand proves the loader uses the dataset's NATIVE per-language
// command table (pytest / cargo / gradlew / jest / ctest / go), NOT the
// TOOLBENCH runner argv.
func TestNativeTestCommand(t *testing.T) {
	cases := map[string][]string{
		"python":     {"pytest"},
		"rust":       {"cargo", "test", "--", "--include-ignored"},
		"go":         {"go", "test", "./..."},
		"java":       {"./gradlew", "test"},
		"javascript": {"./npm-test.sh"},
		"cpp":        {"./cpp-test.sh"},
	}
	for lang, want := range cases {
		got, err := nativeTestCommand(lang)
		if err != nil {
			t.Fatalf("nativeTestCommand(%q): %v", lang, err)
		}
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Fatalf("nativeTestCommand(%q) = %v, want %v", lang, got, want)
		}
	}
	if _, err := nativeTestCommand("cobol"); err == nil {
		t.Fatal("nativeTestCommand(unknown) = nil err, want rejection")
	}
}

// TestTwoAttemptTimeoutBudget proves the 180s upstream timeout constant is wired.
func TestTwoAttemptTimeoutBudget(t *testing.T) {
	if attemptTimeout.Seconds() != 180 {
		t.Fatalf("attemptTimeout = %v, want 180s", attemptTimeout)
	}
	if tries != 2 {
		t.Fatalf("tries = %d, want 2", tries)
	}
}
