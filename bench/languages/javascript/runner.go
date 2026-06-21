// Package javascript is the JavaScript LanguageRunner: it wraps `jest --json`
// into a structured TestOutcome and registers itself for
// (internal-toolbench, javascript) via init(). It is a clone of
// bench/languages/go/runner.go — only Detect, the Setup argv, the
// exec.CommandContext invocation, and the parser differ; the ctx.Err()→infra vs
// *exec.ExitError→exit-code split and the Passed=(exit code == 0) authoritative
// gate are IDENTICAL (D-10 / Pitfall 4).
package javascript

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
	languages.Register("internal-toolbench", "javascript", &JSRunner{})
}

// JSRunner runs JavaScript test suites via jest (`jest --json`). It is stateless
// and safe to share.
type JSRunner struct{}

// Compile-time conformance.
var _ languages.LanguageRunner = (*JSRunner)(nil)

// Detect reports whether repoDir is a JavaScript project: a package.json present
// AND no tsconfig.json. The tsconfig.json ABSENCE check is the precedence rule
// that prevents the TS and JS runners from both claiming the same directory — a
// package.json paired with a tsconfig.json belongs to the TS runner (which
// requires both); a bare package.json belongs here.
func (JSRunner) Detect(repoDir string) bool {
	if _, err := os.Stat(filepath.Join(repoDir, "package.json")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(repoDir, "tsconfig.json")); err == nil {
		// tsconfig.json present → TypeScript precedence; JS yields.
		return false
	}
	return true
}

// Setup honors cancellation only (WR-05): returning ctx.Err() yields nil for a
// live context and the cancellation error when already cancelled, so a future
// non-trivial JS setup (npm/pnpm install) copied from this shape does not silently
// ignore an already-cancelled context.
func (JSRunner) Setup(ctx context.Context, repoDir string) error {
	return ctx.Err()
}

// Capabilities declares 9 of the 10 capability classes (≥8 required by
// TOOLBENCH-05), including CapLSPDiagnostics for eslint diagnostics (a named
// TOOLBENCH-05 requirement). CapCallGraph is omitted as an explicit, declared gap
// (the cross-file JS call-graph corpus task is not yet authored).
func (JSRunner) Capabilities() []languages.Capability {
	return []languages.Capability{
		languages.CapSemanticView,
		languages.CapLSPDiagnostics,
		languages.CapRenameSafety,
		languages.CapFuzzySearch,
		languages.CapDependencyGraph,
		languages.CapPatchApply,
		languages.CapContextMinimization,
		languages.CapIncrementalUpdate,
		languages.CapFailureHandling,
	}
}

// RunTests runs `jest --json` in repoDir, captures the JSON report from stdout,
// parses per-test rows, and reports Passed=(exit code == 0) as the authoritative
// gate. A ctx cancellation/timeout is returned as an infra error (mirrors the Go
// runner's ctx.Err()→infra vs *exec.ExitError→exit-code split); a non-zero test
// exit is a TestOutcome (Passed=false, nil err).
//
// Pitfall 4: a syntax error / failed import produces a non-zero exit with no (or
// partial) test rows. Such an outcome is Passed=false — pass is NEVER inferred
// from the absence of failing rows.
func (JSRunner) RunTests(ctx context.Context, repoDir string) (languages.TestOutcome, error) {
	cmd := exec.CommandContext(ctx, "jest", "--json")
	cmd.Dir = repoDir

	raw, err := cmd.Output()
	if ctx.Err() != nil {
		// Cancellation/timeout: infra error, not a task outcome.
		return languages.TestOutcome{Raw: raw}, ctx.Err()
	}

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
			// On ExitError, cmd.Output() still returns the stdout it captured in
			// raw, so the JSON report is intact.
		} else {
			// Non-ExitError (e.g. `jest` not on PATH): infra failure.
			return languages.TestOutcome{Raw: raw}, err
		}
	}

	return languages.TestOutcome{
		Passed:   exitCode == 0,
		Tests:    parseJestStyleJSON(raw),
		Raw:      raw,
		ExitCode: exitCode,
	}, nil
}

// jestStyleReport is the subset of the Jest-compatible reporter schema that the
// jest `--json` and vitest `--reporter=json` outputs share (RESEARCH §Reporter
// Formats). Only the per-assertion fullName/title + status are decoded.
type jestStyleReport struct {
	TestResults []struct {
		AssertionResults []struct {
			Title    string `json:"title"`
			FullName string `json:"fullName"`
			Status   string `json:"status"`
		} `json:"assertionResults"`
	} `json:"testResults"`
}

// parseJestStyleJSON decodes the committed jest/vitest JSON report into per-test
// rows. status ∈ {passed, failed, skipped, pending, todo} drives the
// Passed/Skipped tri-state: passed → Passed=true; skipped/pending/todo →
// Skipped=true; anything else → fail. jest emits "pending" for a skipped test;
// vitest emits "skipped" for the same concept, so both are mapped to Skipped. The
// row Name is the fullName (falling back to title when fullName is empty). A
// malformed/empty report yields no rows and never panics; this is detail-only,
// because Passed is gated on the subprocess exit code in RunTests and is never
// inferred from these rows (Pitfall 4 / IN-04).
func parseJestStyleJSON(raw []byte) []languages.TestResult {
	var report jestStyleReport
	if err := json.Unmarshal(raw, &report); err != nil {
		// Malformed/empty report — advisory rows omitted; exit code stays the gate.
		return nil
	}

	var results []languages.TestResult
	for _, file := range report.TestResults {
		for _, ar := range file.AssertionResults {
			name := ar.FullName
			if name == "" {
				name = ar.Title
			}
			results = append(results, languages.TestResult{
				Name:    name,
				Passed:  ar.Status == "passed",
				Skipped: ar.Status == "skipped" || ar.Status == "pending" || ar.Status == "todo",
			})
		}
	}
	return results
}
