// Package golang is the Go LanguageRunner: it wraps `go test ./... -json` into a
// structured TestOutcome (D-10) and registers itself for (internal-toolbench, go)
// via init().
package golang

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agenthands/helix/bench/languages"
)

func init() {
	languages.Register("internal-toolbench", "go", &GoRunner{})
}

// GoRunner runs Go test suites. It is stateless and safe to share.
type GoRunner struct{}

// Compile-time conformance.
var _ languages.LanguageRunner = (*GoRunner)(nil)

// Detect reports whether repoDir contains a go.mod.
func (GoRunner) Detect(repoDir string) bool {
	_, err := os.Stat(filepath.Join(repoDir, "go.mod"))
	return err == nil
}

// Setup is a no-op for a hermetic go.mod project (RESEARCH §Pattern 2).
func (GoRunner) Setup(ctx context.Context, repoDir string) error {
	return nil
}

// Capabilities statically declares all 10 capability classes (D-11).
func (GoRunner) Capabilities() []languages.Capability {
	return []languages.Capability{
		languages.CapSemanticView,
		languages.CapLSPDiagnostics,
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

// RunTests runs `go test ./... -json` in repoDir, captures the raw test2json
// stream, parses per-test rows, and reports Passed=(exit code == 0) as the
// authoritative gate. A ctx cancellation/timeout is returned as an infra error
// (mirrors bench/runtime/cell.runVerify's ctx.Err()→infra vs *exec.ExitError→
// exit-code split); a non-zero test exit is a TestOutcome (Passed=false, nil err).
//
// Pitfall 4: a compile failure produces a non-zero exit with zero test rows. Such
// an outcome is Passed=false with empty Tests — pass is NEVER inferred from the
// absence of failing rows.
func (GoRunner) RunTests(ctx context.Context, repoDir string) (languages.TestOutcome, error) {
	cmd := exec.CommandContext(ctx, "go", "test", "./...", "-json")
	cmd.Dir = repoDir

	raw, err := cmd.Output()
	// cmd.Output captures stdout; on a non-zero exit it returns an *exec.ExitError
	// whose Stderr we ignore (test2json goes to stdout, already in raw). Combine
	// any stdout already captured even on ExitError.
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
			// raw, so the test2json stream is intact.
		} else {
			// Non-ExitError (e.g. `go` not on PATH): infra failure.
			return languages.TestOutcome{Raw: raw}, err
		}
	}

	tests := parseTest2JSON(raw)

	return languages.TestOutcome{
		Passed:   exitCode == 0,
		Tests:    tests,
		Raw:      raw,
		ExitCode: exitCode,
	}, nil
}

// test2jsonEvent is one decoded line of `go test -json` output. Only the fields
// RunTests consumes are declared.
type test2jsonEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
}

// parseTest2JSON streams the test2json event lines and collects per-test rows
// (events where Test != "" and Action ∈ {pass, fail, skip}). pass → Passed=true;
// fail/skip → Passed=false. Package-level events (Test == "") are ignored, as are
// start/run/output/bench/pause/cont events.
func parseTest2JSON(raw []byte) []languages.TestResult {
	var results []languages.TestResult
	dec := json.NewDecoder(bytes.NewReader(raw))
	for {
		var ev test2jsonEvent
		if err := dec.Decode(&ev); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			// A malformed line (e.g. a build-error banner that is not JSON) ends
			// structured parsing; the exit code remains the authoritative gate.
			break
		}
		if ev.Test == "" {
			continue
		}
		switch ev.Action {
		case "pass", "fail", "skip":
			results = append(results, languages.TestResult{
				Name:    ev.Test,
				Package: ev.Package,
				Passed:  ev.Action == "pass",
				Elapsed: ev.Elapsed,
			})
		}
	}
	return results
}
