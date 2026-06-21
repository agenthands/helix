// Package java is the Java LanguageRunner: it wraps `mvn test
// -Dsurefire.useFile=false` into a structured TestOutcome by parsing the
// surefire JUnit reports under target/surefire-reports, and registers itself for
// (internal-toolbench, java) via init(). It is a clone of
// bench/languages/go/runner.go — only Detect, the Setup argv, the
// exec.CommandContext invocation, and the parser differ; the ctx.Err()→infra vs
// *exec.ExitError→exit-code split and the Passed=(exit code == 0) authoritative
// gate are IDENTICAL (D-10 / Pitfall 4).
//
// mvn/javac are ABSENT in this env, so RunTests' live path is exercised only in
// CI with a JDK image; the hermetic parseSurefireXML golden test is the
// authoritative proof here.
package java

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
	languages.Register("internal-toolbench", "java", &JavaRunner{})
}

// JavaRunner runs Java (Maven Surefire) test suites. It is stateless and safe to
// share.
type JavaRunner struct{}

// Compile-time conformance.
var _ languages.LanguageRunner = (*JavaRunner)(nil)

// Detect reports whether repoDir contains a pom.xml (a Maven project). Analog of
// the Go runner's go.mod check.
func (JavaRunner) Detect(repoDir string) bool {
	_, err := os.Stat(filepath.Join(repoDir, "pom.xml"))
	return err == nil
}

// Setup honors cancellation only (WR-05): returning ctx.Err() yields nil for a
// live context and the cancellation error when already cancelled, so a future
// non-trivial Java setup (mvn -o dependency:go-offline) copied from this shape
// does not silently ignore an already-cancelled context.
func (JavaRunner) Setup(ctx context.Context, repoDir string) error {
	return ctx.Err()
}

// Capabilities declares 9 of the 10 capability classes (≥8 required by
// TOOLBENCH-06). CapFuzzySearch is omitted as an explicit, declared gap (the
// fuzzy-edit corpus tasks are authored against the file-oriented runners first).
func (JavaRunner) Capabilities() []languages.Capability {
	return []languages.Capability{
		languages.CapSemanticView,
		languages.CapLSPDiagnostics,
		languages.CapRenameSafety,
		languages.CapCallGraph,
		languages.CapDependencyGraph,
		languages.CapPatchApply,
		languages.CapContextMinimization,
		languages.CapIncrementalUpdate,
		languages.CapFailureHandling,
	}
}

// RunTests runs `mvn test -Dsurefire.useFile=false` in repoDir, reads every
// target/surefire-reports/TEST-*.xml, parses per-test rows, and reports
// Passed=(exit code == 0) as the authoritative gate. A ctx cancellation/timeout
// is returned as an infra error (mirrors the Go runner's ctx.Err()→infra vs
// *exec.ExitError→exit-code split); a non-zero test exit is a TestOutcome
// (Passed=false, nil err).
//
// Pitfall 4: a compile failure produces a non-zero exit with no surefire reports.
// Such an outcome is Passed=false with empty Tests — pass is NEVER inferred from
// the absence of failing rows.
func (JavaRunner) RunTests(ctx context.Context, repoDir string) (languages.TestOutcome, error) {
	cmd := exec.CommandContext(ctx, "mvn", "test", "-Dsurefire.useFile=false")
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
			// Non-ExitError (e.g. `mvn` not on PATH): infra failure.
			return languages.TestOutcome{}, err
		}
	}

	// Collect and parse every surefire report. A missing report dir is
	// advisory-only — the exit code remains the authoritative gate.
	reports, _ := filepath.Glob(filepath.Join(repoDir, "target", "surefire-reports", "TEST-*.xml"))
	var raw []byte
	var tests []languages.TestResult
	for _, rpt := range reports {
		b, readErr := os.ReadFile(rpt)
		if readErr != nil {
			continue
		}
		raw = append(raw, b...)
		tests = append(tests, parseSurefireXML(b)...)
	}

	return languages.TestOutcome{
		Passed:   exitCode == 0,
		Tests:    tests,
		Raw:      raw,
		ExitCode: exitCode,
	}, nil
}

// surefireSuite is the subset of the surefire JUnit schema RunTests consumes. A
// <testcase> with no child <failure>/<error>/<skipped> is a pass; a child
// <failure> or <error> is a fail; a child <skipped> is a skip.
type surefireSuite struct {
	Cases []struct {
		Name      string    `xml:"name,attr"`
		ClassName string    `xml:"classname,attr"`
		Time      float64   `xml:"time,attr"`
		Failure   *struct{} `xml:"failure"`
		Error     *struct{} `xml:"error"`
		Skipped   *struct{} `xml:"skipped"`
	} `xml:"testcase"`
}

// parseSurefireXML unmarshals one surefire TEST-*.xml into per-test rows using
// encoding/xml (NOT regex — RESEARCH §Don't Hand-Roll). Presence of a child
// <failure>/<error> marks the case failed; <skipped> marks it skipped; absence of
// all three marks it passed. A malformed report yields no rows and never panics;
// this is detail-only, because Passed is gated on the subprocess exit code in
// RunTests and is never inferred from these rows (Pitfall 4).
func parseSurefireXML(raw []byte) []languages.TestResult {
	var suite surefireSuite
	if err := xml.Unmarshal(raw, &suite); err != nil {
		return nil
	}

	var results []languages.TestResult
	for _, tc := range suite.Cases {
		skipped := tc.Skipped != nil
		failed := tc.Failure != nil || tc.Error != nil
		results = append(results, languages.TestResult{
			Name:    tc.Name,
			Package: tc.ClassName,
			Passed:  !skipped && !failed,
			Skipped: skipped,
			Elapsed: tc.Time,
		})
	}
	return results
}
