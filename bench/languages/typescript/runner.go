// Package typescript is the TypeScript LanguageRunner: it wraps
// `vitest run --reporter=json` into a structured TestOutcome and registers itself
// for (internal-toolbench, typescript) via init(). It is a clone of
// bench/languages/go/runner.go — only Detect, the Setup argv, the
// exec.CommandContext invocation, and the parser differ; the ctx.Err()→infra vs
// *exec.ExitError→exit-code split and the Passed=(exit code == 0) authoritative
// gate are IDENTICAL (D-10 / Pitfall 4).
package typescript

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
	languages.Register("internal-toolbench", "typescript", &TSRunner{})
}

// TSRunner runs TypeScript test suites via vitest (`vitest run --reporter=json`).
// It is stateless and safe to share.
type TSRunner struct{}

// Compile-time conformance.
var _ languages.LanguageRunner = (*TSRunner)(nil)

// Detect reports whether repoDir is a TypeScript project: it requires BOTH a
// package.json AND a tsconfig.json. The tsconfig.json requirement is the
// precedence rule that prevents the TS and JS runners from both claiming the same
// directory — a bare package.json (no tsconfig.json) belongs to the JS runner.
func (TSRunner) Detect(repoDir string) bool {
	for _, marker := range []string{"package.json", "tsconfig.json"} {
		if _, err := os.Stat(filepath.Join(repoDir, marker)); err != nil {
			return false
		}
	}
	return true
}

// Setup honors cancellation only (WR-05): returning ctx.Err() yields nil for a
// live context and the cancellation error when already cancelled, so a future
// non-trivial TS setup (npm/pnpm install) copied from this shape does not silently
// ignore an already-cancelled context.
func (TSRunner) Setup(ctx context.Context, repoDir string) error {
	return ctx.Err()
}

// Capabilities declares 9 of the 10 capability classes (≥8 required by
// TOOLBENCH-04), including CapLSPDiagnostics for tsserver diagnostics (a named
// TOOLBENCH-04 requirement). CapCallGraph is omitted as an explicit, declared gap
// (the cross-file TS call-graph corpus task is not yet authored).
func (TSRunner) Capabilities() []languages.Capability {
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

// RunTests runs `vitest run --reporter=json` in repoDir, captures the JSON report
// from stdout, parses per-test rows, and reports Passed=(exit code == 0) as the
// authoritative gate. A ctx cancellation/timeout is returned as an infra error
// (mirrors the Go runner's ctx.Err()→infra vs *exec.ExitError→exit-code split); a
// non-zero test exit is a TestOutcome (Passed=false, nil err).
//
// Pitfall 4: a type-check/compile failure produces a non-zero exit with no (or
// partial) test rows. Such an outcome is Passed=false — pass is NEVER inferred
// from the absence of failing rows.
func (TSRunner) RunTests(ctx context.Context, repoDir string) (languages.TestOutcome, error) {
	cmd := exec.CommandContext(ctx, "vitest", "run", "--reporter=json")
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
			// Non-ExitError (e.g. `vitest` not on PATH): infra failure.
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
// vitest `--reporter=json` and jest `--json` outputs share (RESEARCH §Reporter
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

// parseJestStyleJSON decodes the committed vitest/jest JSON report into per-test
// rows. status ∈ {passed, failed, skipped, pending, todo} drives the
// Passed/Skipped tri-state: passed → Passed=true; skipped/pending/todo →
// Skipped=true; anything else → fail. vitest emits "skipped"; jest emits
// "pending" for the same concept, so both are mapped to Skipped. The row Name is
// the fullName (falling back to title when fullName is empty). A malformed/empty
// report yields no rows and never panics; this is detail-only, because Passed is
// gated on the subprocess exit code in RunTests and is never inferred from these
// rows (Pitfall 4 / IN-04).
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
