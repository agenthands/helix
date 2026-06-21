// Package cpp is the C++ LanguageRunner: it wraps `ctest --output-junit R.xml`
// into a structured TestOutcome by parsing the emitted JUnit XML, and registers
// itself for (internal-toolbench, cpp) via init(). It is a clone of
// bench/languages/go/runner.go — only Detect, the Setup argv, the
// exec.CommandContext invocation, and the parser differ; the ctx.Err()→infra vs
// *exec.ExitError→exit-code split and the Passed=(exit code == 0) authoritative
// gate are IDENTICAL (D-10 / Pitfall 4).
package cpp

import (
	"context"
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agenthands/helix/bench/languages"
)

func init() {
	languages.Register("internal-toolbench", "cpp", &CppRunner{})
}

// CppRunner runs C++ (cmake/ctest) test suites. It is stateless and safe to
// share.
type CppRunner struct{}

// Compile-time conformance.
var _ languages.LanguageRunner = (*CppRunner)(nil)

// Detect reports whether repoDir contains a CMakeLists.txt. Analog of the Go
// runner's go.mod check.
func (CppRunner) Detect(repoDir string) bool {
	_, err := os.Stat(filepath.Join(repoDir, "CMakeLists.txt"))
	return err == nil
}

// Setup honors cancellation only (WR-05): returning ctx.Err() yields nil for a
// live context and the cancellation error when already cancelled, so a future
// non-trivial C++ setup (cmake configure + build) copied from this shape does not
// silently ignore an already-cancelled context. The real configure/build step is
// a follow-on; this runner invokes ctest against an already-built tree.
func (CppRunner) Setup(ctx context.Context, repoDir string) error {
	return ctx.Err()
}

// Capabilities declares 7 of the 10 capability classes (≥6 required by
// TOOLBENCH-08). CapLSPDiagnostics, CapCallGraph, and CapDependencyGraph are
// omitted as explicit, declared gaps: the C++ semantic surface (clangd) is not
// yet wired into the toolbench corpus for those classes. CapSemanticView IS
// declared (clangd backs the outline/definition tools).
func (CppRunner) Capabilities() []languages.Capability {
	return []languages.Capability{
		languages.CapSemanticView,
		languages.CapRenameSafety,
		languages.CapFuzzySearch,
		languages.CapPatchApply,
		languages.CapContextMinimization,
		languages.CapIncrementalUpdate,
		languages.CapFailureHandling,
	}
}

// RunTests runs `ctest --output-junit R.xml` in repoDir (a configured+built CMake
// tree), reads the emitted JUnit XML, parses per-test rows, and reports
// Passed=(exit code == 0) as the authoritative gate. A ctx cancellation/timeout
// is returned as an infra error (mirrors the Go runner's ctx.Err()→infra vs
// *exec.ExitError→exit-code split); a non-zero test exit is a TestOutcome
// (Passed=false, nil err).
//
// Pitfall 4: a build/config failure produces a non-zero exit with no JUnit XML.
// Such an outcome is Passed=false with empty Tests — pass is NEVER inferred from
// the absence of failing rows.
func (CppRunner) RunTests(ctx context.Context, repoDir string) (languages.TestOutcome, error) {
	reportPath := filepath.Join(repoDir, "R.xml")
	cmd := exec.CommandContext(ctx, "ctest", "--output-junit", "R.xml")
	cmd.Dir = repoDir

	err := cmd.Run()
	if ctx.Err() != nil {
		// Cancellation/timeout: infra error, not a task outcome.
		return languages.TestOutcome{}, ctx.Err()
	}

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			// Non-ExitError (e.g. `ctest` not on PATH): infra failure.
			return languages.TestOutcome{}, err
		}
	}

	// ctest writes R.xml in repoDir; a missing/unreadable report is advisory-only
	// (the exit code remains the authoritative gate).
	raw, _ := os.ReadFile(reportPath)

	return languages.TestOutcome{
		Passed:   exitCode == 0,
		Tests:    parseCtestJUnit(raw),
		Raw:      raw,
		ExitCode: exitCode,
	}, nil
}

// ctestJUnit is the subset of the ctest JUnit schema RunTests consumes. Each
// <testcase> carries a name; the presence of a child <failure> element is the
// fail signal (RESEARCH §Reporter Formats: "child <failure> ⇒ fail"). ctest does
// not emit <skipped> children for the default run, so the C++ runner reports a
// pass/fail binary (Skipped stays false).
type ctestJUnit struct {
	TestCases []struct {
		Name    string `xml:"name,attr"`
		Failure *struct {
			Message string `xml:"message,attr"`
		} `xml:"failure"`
	} `xml:"testcase"`
}

// parseCtestJUnit unmarshals a ctest --output-junit file into per-test rows using
// encoding/xml (NOT regex — RESEARCH §Don't Hand-Roll). A testcase with NO child
// <failure> → Passed=true; a testcase WITH a child <failure> → Passed=false. A
// malformed XML yields no rows and never panics; this is detail-only, because
// Passed is gated on the subprocess exit code in RunTests and is never inferred
// from these rows (Pitfall 4 / IN-04).
func parseCtestJUnit(raw []byte) []languages.TestResult {
	var suite ctestJUnit
	if err := xml.Unmarshal(raw, &suite); err != nil {
		// Malformed/empty XML — advisory rows omitted; exit code stays the gate.
		return nil
	}

	var results []languages.TestResult
	for _, tc := range suite.TestCases {
		results = append(results, languages.TestResult{
			Name:   tc.Name,
			Passed: tc.Failure == nil,
		})
	}
	return results
}
