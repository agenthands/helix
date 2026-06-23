package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	aiderpolyglot "github.com/agenthands/helix/bench/datasets/aider-polyglot"
)

// fakeNativeTester is a scripted TestFn whose verdict depends on the work-dir
// stub content, so the hermetic test actually GRADES the edit the AgentFn made
// (non-vacuous): it passes only when the applied stub body matches wantBody.
type fakeNativeTester struct {
	stub     string
	wantBody string
}

func (f fakeNativeTester) run(_ context.Context, _ *aiderpolyglot.Exercise, workDir string) aiderpolyglot.TestResult {
	got, err := os.ReadFile(filepath.Join(workDir, f.stub))
	if err != nil {
		return aiderpolyglot.TestResult{Passed: false, Output: "read stub: " + err.Error()}
	}
	if string(got) == f.wantBody {
		return aiderpolyglot.TestResult{Passed: true}
	}
	return aiderpolyglot.TestResult{Passed: false, Output: "stub body != reference"}
}

// seedExercise lays down a tiny aider-style fixture under a temp SrcDir (stub +
// reference example) and a work dir seeded with the pristine stub, returning the
// loaded-style Exercise and the work dir. No network, no real toolchain.
func seedExercise(t *testing.T) (*aiderpolyglot.Exercise, string, string) {
	t.Helper()
	srcDir := t.TempDir()
	workDir := t.TempDir()

	const stub = "wordy.go"
	const example = ".meta/example.go"
	const stubBody = "package wordy\n\nfunc Answer(q string) (int, bool) { return 0, false }\n"
	const refBody = "package wordy\n\nfunc Answer(q string) (int, bool) { return 42, true }\n"

	// reference body lives under SrcDir/.meta/example.go
	if err := os.MkdirAll(filepath.Join(srcDir, ".meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, example), []byte(refBody), 0o644); err != nil {
		t.Fatal(err)
	}
	// pristine stub lives in the work dir (the agent edits the work copy)
	if err := os.WriteFile(filepath.Join(workDir, stub), []byte(stubBody), 0o644); err != nil {
		t.Fatal(err)
	}

	ex := &aiderpolyglot.Exercise{
		Name:     "wordy",
		Language: "go",
		SrcDir:   srcDir,
	}
	ex.Config.Files.Solution = []string{stub}
	ex.Config.Files.Example = []string{example}
	// no Test files seeded → empty SrcDir-test list; the WR-01 restore over an
	// empty Test slice is a no-op, so the hermetic grade reads the applied stub.
	return ex, workDir, refBody
}

// TestAiderEditAgentHermetic (EDITBENCH-01, SOLE authoritative proof — Pitfall 1):
// drive RunExercise with the deterministic EDIT agent over an IN-PROCESS apply
// seam (NO HELIX_BIN, NO network) + a fake native tester that grades the applied
// stub. The reference body is applied → tests pass → edit_format_applied=true.
func TestAiderEditAgentHermetic(t *testing.T) {
	ex, workDir, refBody := seedExercise(t)

	var applied bool
	// In-process apply seam: read the reference body, write it to the work stub —
	// the deterministic-transform contract WITHOUT a live session.
	agent := newEditAgentWithApply(&applied, func(_ context.Context, workDir, stub, body string) error {
		return os.WriteFile(filepath.Join(workDir, stub), []byte(body), 0o644)
	})

	tester := fakeNativeTester{stub: "wordy.go", wantBody: refBody}
	res := aiderpolyglot.RunExercise(context.Background(), ex, workDir, tester.run, agent)

	if !res.Passed {
		t.Fatalf("RunExercise = %+v, want Passed (reference body applied)", res)
	}
	if !applied {
		t.Fatal("edit_format_applied = false, want true after a successful reference edit")
	}
}

// TestAiderEditAgentAntiTamper (anti-vacuity, revert-and-fail): when the agent
// applies a WRONG body, the native tester grades it false — proving the baseline
// pass is non-vacuous and the test actually grades the applied edit, not merely
// that the agent ran.
func TestAiderEditAgentAntiTamper(t *testing.T) {
	ex, workDir, refBody := seedExercise(t)

	var applied bool
	// Apply seam writes a WRONG body (ignores the reference) — the grade must fail.
	agent := newEditAgentWithApply(&applied, func(_ context.Context, workDir, stub, _ string) error {
		return os.WriteFile(filepath.Join(workDir, stub), []byte("package wordy\n// wrong\n"), 0o644)
	})

	tester := fakeNativeTester{stub: "wordy.go", wantBody: refBody}
	res := aiderpolyglot.RunExercise(context.Background(), ex, workDir, tester.run, agent)

	if res.Passed {
		t.Fatal("RunExercise Passed with a WRONG edit — the grade is vacuous (anti-vacuity violated)")
	}
	// applied is still set true (the apply seam succeeded transport-wise); the
	// FAILURE comes from the grade, not the apply. That is the correct separation:
	// edit_format_applied records "the edit format was applied", pass/fail records
	// "the applied edit graded green".
	if !applied {
		t.Fatal("edit_format_applied = false, want true (the apply seam itself succeeded)")
	}
}

// TestAiderEditAgentNoOpEditFailsClosed (WR-02 regression): the live apply seam ends
// with verifyStubApplied, which fails CLOSED when the post-edit stub content does NOT
// equal the reference body — the exact situation when replace_in_file returns a
// non-error "0 replacement(s) made" (literal pattern miss + fuzzy unavailable). Here
// the apply seam is a NO-OP (leaves the stub at its original body and runs the same
// verify the live seam runs); the agent MUST return an error and set
// edit_format_applied=false, NOT record a vacuous true.
func TestAiderEditAgentNoOpEditFailsClosed(t *testing.T) {
	ex, workDir, refBody := seedExercise(t)

	var applied bool
	// No-op seam: simulate replace_in_file's "0 replacement(s) made" — the tool call
	// "succeeds" but the stub is unchanged. Then run the SAME post-edit verification
	// the live seam runs (verifyStubApplied). It must reject the no-op.
	agent := newEditAgentWithApply(&applied, func(_ context.Context, wd, stub, body string) error {
		// deliberately do NOT write body — the stub keeps its original (≠ body) content
		return verifyStubApplied(wd, stub, body)
	})

	tester := fakeNativeTester{stub: "wordy.go", wantBody: refBody}
	res := aiderpolyglot.RunExercise(context.Background(), ex, workDir, tester.run, agent)

	if res.Passed {
		t.Fatal("RunExercise Passed on a no-op edit — the verify-applied guard did not fail closed (WR-02)")
	}
	if applied {
		t.Fatal("edit_format_applied = true on a no-op edit — WR-02 false-green (a non-matching replace_in_file recorded as applied)")
	}
}

// TestVerifyStubApplied is the focused unit proof of the WR-02 guard: it accepts only
// when the post-edit stub content equals the intended reference body, and rejects the
// no-op (stub unchanged) and missing-file cases.
func TestVerifyStubApplied(t *testing.T) {
	const stub = "wordy.go"
	const body = "package wordy\n\nfunc Answer(q string) (int, bool) { return 42, true }\n"

	t.Run("applied-equals-body", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, stub), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := verifyStubApplied(dir, stub, body); err != nil {
			t.Fatalf("verifyStubApplied rejected a correctly-applied stub: %v", err)
		}
	})

	t.Run("no-op-stub-unchanged-rejected", func(t *testing.T) {
		dir := t.TempDir()
		// stub still holds the ORIGINAL body (the "0 replacement(s) made" no-op case)
		if err := os.WriteFile(filepath.Join(dir, stub), []byte("package wordy\n\nfunc Answer(q string) (int, bool) { panic(\"stub\") }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := verifyStubApplied(dir, stub, body); err == nil {
			t.Fatal("verifyStubApplied accepted a no-op edit (stub unchanged); want fail-closed error")
		}
	})

	t.Run("missing-file-rejected", func(t *testing.T) {
		if err := verifyStubApplied(t.TempDir(), stub, body); err == nil {
			t.Fatal("verifyStubApplied accepted a missing stub; want fail-closed error")
		}
	})
}

// TestNativeTestFn proves newNativeTestFn builds the per-language argv via
// NativeTestCommand and fail-closes for an unknown language WITHOUT shelling a
// real toolchain (a trivially-passing /-failing command exercises the exit path).
func TestNativeTestFn(t *testing.T) {
	fn := newNativeTestFn()

	// Unknown language → fail-closed (NativeTestCommand errors), Passed=false.
	t.Run("unknown-language-fail-closed", func(t *testing.T) {
		ex := &aiderpolyglot.Exercise{Language: "cobol"}
		res := fn(context.Background(), ex, t.TempDir())
		if res.Passed {
			t.Fatal("unknown language graded Passed, want fail-closed false")
		}
		if res.Output == "" {
			t.Fatal("unknown language must carry the fail-closed error in Output")
		}
	})

	// A trivially-passing native command (go test over an empty module dir with no
	// test files exits 0) → Passed=true. Exercises the live exec.CommandContext path
	// without a heavy toolchain build.
	t.Run("trivially-passing", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module trivial\n\ngo 1.21\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "trivial.go"), []byte("package trivial\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		ex := &aiderpolyglot.Exercise{Language: "go"}
		res := fn(context.Background(), ex, dir)
		if !res.Passed {
			t.Fatalf("trivially-passing `go test ./...` graded false: %s", res.Output)
		}
	})
}
