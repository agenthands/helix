// Package csharp is the C# LanguageRunner: it wraps `dotnet test --logger
// "trx;LogFileName=..."` into a structured TestOutcome by parsing the emitted TRX
// XML, and registers itself for (internal-toolbench, csharp) via init(). It is a
// clone of bench/languages/go/runner.go — only Detect, the Setup argv, the
// exec.CommandContext invocation, and the parser differ; the ctx.Err()→infra vs
// *exec.ExitError→exit-code split and the Passed=(exit code == 0) authoritative
// gate are IDENTICAL (D-10 / Pitfall 4).
package csharp

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
	languages.Register("internal-toolbench", "csharp", &CSharpRunner{})
}

// CSharpRunner runs C# (dotnet test) suites. It is stateless and safe to share.
type CSharpRunner struct{}

// Compile-time conformance.
var _ languages.LanguageRunner = (*CSharpRunner)(nil)

// Detect reports whether repoDir contains a .csproj or .sln. Analog of the Go
// runner's go.mod check.
func (CSharpRunner) Detect(repoDir string) bool {
	for _, pat := range []string{"*.csproj", "*.sln"} {
		matches, err := filepath.Glob(filepath.Join(repoDir, pat))
		if err == nil && len(matches) > 0 {
			return true
		}
	}
	return false
}

// Setup honors cancellation only (WR-05): returning ctx.Err() yields nil for a
// live context and the cancellation error when already cancelled, so a future
// non-trivial C# setup (dotnet restore) copied from this shape does not silently
// ignore an already-cancelled context.
func (CSharpRunner) Setup(ctx context.Context, repoDir string) error {
	return ctx.Err()
}

// Capabilities declares 7 of the 10 capability classes (≥6 required by
// TOOLBENCH-07). CapLSPDiagnostics, CapCallGraph, and CapDependencyGraph are
// omitted as explicit, declared gaps: the C# semantic surface (csharp-ls /
// OmniSharp) is not yet wired into the toolbench corpus for those classes.
func (CSharpRunner) Capabilities() []languages.Capability {
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

// RunTests runs `dotnet test --logger "trx;LogFileName=R.trx"` in repoDir, reads
// the emitted TRX, parses per-test rows, and reports Passed=(exit code == 0) as
// the authoritative gate. A ctx cancellation/timeout is returned as an infra
// error (mirrors the Go runner's ctx.Err()→infra vs *exec.ExitError→exit-code
// split); a non-zero test exit is a TestOutcome (Passed=false, nil err).
//
// Pitfall 4: a build failure produces a non-zero exit with no TRX. Such an
// outcome is Passed=false with empty Tests — pass is NEVER inferred from the
// absence of failing rows.
func (CSharpRunner) RunTests(ctx context.Context, repoDir string) (languages.TestOutcome, error) {
	cmd := exec.CommandContext(ctx, "dotnet", "test", "--logger", "trx;LogFileName=R.trx")
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
			// Non-ExitError (e.g. `dotnet` not on PATH): infra failure.
			return languages.TestOutcome{}, err
		}
	}

	// dotnet writes the TRX under TestResults/. Glob for it; a missing TRX is
	// advisory-only — the exit code remains the authoritative gate.
	matches, _ := filepath.Glob(filepath.Join(repoDir, "TestResults", "R.trx"))
	var raw []byte
	if len(matches) > 0 {
		raw, _ = os.ReadFile(matches[0])
	}

	return languages.TestOutcome{
		Passed:   exitCode == 0,
		Tests:    parseTRX(raw),
		Raw:      raw,
		ExitCode: exitCode,
	}, nil
}

// trxRun is the subset of the TRX schema RunTests consumes. Only the
// <Results><UnitTestResult> rows are decoded (the <RunInfos> diagnostic blocks
// are deliberately excluded by anchoring on the Results path). outcome ∈
// {Passed, Failed, NotExecuted, ...}.
type trxRun struct {
	Results []struct {
		TestName string `xml:"testName,attr"`
		Outcome  string `xml:"outcome,attr"`
	} `xml:"Results>UnitTestResult"`
}

// parseTRX unmarshals a TRX file into per-test rows using encoding/xml (NOT regex
// — RESEARCH §Don't Hand-Roll). outcome="Passed" → Passed=true;
// outcome="NotExecuted" → Skipped=true; anything else (Failed/Error/Timeout/…) →
// fail. encoding/xml matches local element names regardless of the TRX default
// namespace, so the Results>UnitTestResult path resolves. A malformed TRX yields
// no rows and never panics; this is detail-only, because Passed is gated on the
// subprocess exit code in RunTests and is never inferred from these rows
// (Pitfall 4).
func parseTRX(raw []byte) []languages.TestResult {
	var run trxRun
	if err := xml.Unmarshal(raw, &run); err != nil {
		return nil
	}

	var results []languages.TestResult
	for _, r := range run.Results {
		results = append(results, languages.TestResult{
			Name:    r.TestName,
			Passed:  r.Outcome == "Passed",
			Skipped: r.Outcome == "NotExecuted",
		})
	}
	return results
}
