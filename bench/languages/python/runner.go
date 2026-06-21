// Package python is the Python LanguageRunner: it wraps `pytest --json-report`
// into a structured TestOutcome and registers itself for (internal-toolbench,
// python) via init(). It is a clone of bench/languages/go/runner.go — only
// Detect, the Setup argv, the exec.CommandContext invocation, and the parser
// differ; the ctx.Err()→infra vs *exec.ExitError→exit-code split and the
// Passed=(exit code == 0) authoritative gate are IDENTICAL (D-10 / Pitfall 4).
package python

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agenthands/helix/bench/languages"
)

func init() {
	languages.Register("internal-toolbench", "python", &PyRunner{})
}

// PyRunner runs Python test suites via pytest + pytest-json-report. It is
// stateless and safe to share.
type PyRunner struct{}

// Compile-time conformance.
var _ languages.LanguageRunner = (*PyRunner)(nil)

// Detect reports whether repoDir looks like a Python project (any of the common
// project markers). Analog of the Go runner's go.mod check.
func (PyRunner) Detect(repoDir string) bool {
	for _, marker := range []string{"pyproject.toml", "setup.py", "requirements.txt"} {
		if _, err := os.Stat(filepath.Join(repoDir, marker)); err == nil {
			return true
		}
	}
	return false
}

// Setup honors cancellation only (WR-05): returning ctx.Err() yields nil for a
// live context and the cancellation error when the operator has already
// cancelled, so a future non-trivial Python setup (pip install) copied from this
// shape does not silently ignore an already-cancelled context.
func (PyRunner) Setup(ctx context.Context, repoDir string) error {
	return ctx.Err()
}

// Capabilities declares 9 of the 10 capability classes (≥8 required by
// TOOLBENCH-03). CapLSPDiagnostics is omitted: the Python LSP (pyright/pylsp)
// diagnostic surface is not yet wired into the toolbench corpus, so it is an
// explicit, declared gap rather than a silently-claimed capability.
func (PyRunner) Capabilities() []languages.Capability {
	return []languages.Capability{
		languages.CapSemanticView,
		languages.CapRenameSafety,
		languages.CapFuzzySearch,
		languages.CapCallGraph,
		languages.CapDependencyGraph,
		languages.CapPatchApply,
		languages.CapContextMinimization,
		languages.CapIncrementalUpdate,
		languages.CapFailureHandling,
	}
}

// RunTests runs `pytest --json-report --json-report-file=<repoDir>/.report.json -q`
// in repoDir, reads the emitted JSON report, parses per-test rows, and reports
// Passed=(exit code == 0) as the authoritative gate. A ctx cancellation/timeout
// is returned as an infra error (mirrors the Go runner's ctx.Err()→infra vs
// *exec.ExitError→exit-code split); a non-zero test exit is a TestOutcome
// (Passed=false, nil err).
//
// Pitfall 4: a collection error / import failure produces a non-zero exit with no
// (or partial) test rows. Such an outcome is Passed=false — pass is NEVER
// inferred from the absence of failing rows.
func (PyRunner) RunTests(ctx context.Context, repoDir string) (languages.TestOutcome, error) {
	reportPath := filepath.Join(repoDir, ".report.json")
	cmd := exec.CommandContext(ctx, "pytest", "--json-report",
		"--json-report-file="+reportPath, "-q")
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
			// Non-ExitError (e.g. `pytest` not on PATH): infra failure.
			return languages.TestOutcome{}, err
		}
	}

	// The report is written even when tests fail; a missing/unreadable report is
	// advisory-only (the exit code remains the authoritative gate).
	raw, _ := os.ReadFile(reportPath)

	return languages.TestOutcome{
		Passed:   exitCode == 0,
		Tests:    parsePytestJSON(raw),
		Raw:      raw,
		ExitCode: exitCode,
	}, nil
}

// pytestReport is the subset of the pytest-json-report 1.5.0 schema that
// RunTests consumes (github.com/numirias/pytest-json-report). Only the
// per-test nodeid + outcome are decoded.
type pytestReport struct {
	Tests []struct {
		NodeID  string `json:"nodeid"`
		Outcome string `json:"outcome"`
	} `json:"tests"`
}

// parsePytestJSON decodes the committed pytest-json report into per-test rows.
// outcome ∈ {passed, failed, skipped} drives the Passed/Skipped tri-state
// (passed → Passed=true; skipped → Skipped=true; anything else → fail). A
// malformed/empty report yields no rows and never panics; this is detail-only,
// because Passed is gated on the subprocess exit code in RunTests and is never
// inferred from these rows (Pitfall 4 / IN-04).
func parsePytestJSON(raw []byte) []languages.TestResult {
	var report pytestReport
	if err := json.Unmarshal(raw, &report); err != nil {
		// Malformed/empty report — advisory rows omitted; exit code stays the gate.
		return nil
	}

	var results []languages.TestResult
	for _, tc := range report.Tests {
		results = append(results, languages.TestResult{
			Name:    tc.NodeID,
			Passed:  tc.Outcome == "passed",
			Skipped: tc.Outcome == "skipped",
		})
	}
	return results
}
