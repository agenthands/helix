// Package languages defines the LanguageRunner seam that the bench harness uses
// to detect, set up, and run a benchmark task's tests in a language-agnostic way.
//
// This is the Phase 85-facing contract: every language adapter (Go today; Rust,
// TypeScript, Python, … in Phase 85) implements LanguageRunner and registers
// itself via the registry keyed by (benchmark, language). The harness asks
// RunnerFor(benchmark, lang); a nil result is the D-10 "fall back to verify.sh"
// signal.
//
// RunTests returns a structured TestOutcome (parsed per-test results) rather than
// a bare exit code, because Phase 79 grades structured output. Passed is the
// authoritative gate and is defined as (exit code == 0); the per-test rows are
// advisory detail layered on top of that gate.
package languages

import "context"

// Capability is a closed enum of the tool-capability classes a benchmark task may
// exercise. The string VALUES below are the source of truth for the task.json
// `capability` field (D-06): downstream plans MUST use these identical strings.
type Capability string

// The 10 capability classes (D-06 / D-11). The string values are fixed; the Go
// identifiers are discretionary.
const (
	CapSemanticView        Capability = "semantic_view"
	CapLSPDiagnostics      Capability = "lsp_diagnostics"
	CapRenameSafety        Capability = "rename_safety"
	CapFuzzySearch         Capability = "fuzzy_search"
	CapCallGraph           Capability = "call_graph"
	CapDependencyGraph     Capability = "dependency_graph"
	CapPatchApply          Capability = "patch_apply"
	CapContextMinimization Capability = "context_minimization"
	CapIncrementalUpdate   Capability = "incremental_update"
	CapFailureHandling     Capability = "failure_handling"
)

// TestResult is one parsed per-test row from the underlying test runner's
// structured output (Go's test2json `pass`/`fail`/`skip` events).
type TestResult struct {
	Name    string  // the test function name (test2json `Test` field)
	Package string  // the import path of the package the test belongs to
	Passed  bool    // true for a `pass` event; false for `fail`/`skip`
	Skipped bool    // true for a `skip` event (tri-state: pass vs fail vs skip)
	Elapsed float64 // seconds, from the test2json `Elapsed` field
}

// TestOutcome is the structured result of RunTests. Passed is the authoritative
// gate (exit code == 0). Tests holds the parsed per-test detail (empty on a
// compile failure, Pitfall 4). Raw is the unparsed runner output. ExitCode is the
// subprocess exit code (0 on success).
type TestOutcome struct {
	Passed   bool
	Tests    []TestResult
	Raw      []byte
	ExitCode int
}

// LanguageRunner is the per-language adapter the bench harness drives. A runner is
// stateless and safe to share; all per-task state lives in repoDir.
type LanguageRunner interface {
	// Detect reports whether repoDir is a project of this runner's language
	// (e.g. the Go runner checks for go.mod).
	Detect(repoDir string) bool

	// Setup performs any pre-test preparation (dependency fetch, build cache
	// warm-up). For a hermetic go.mod project it is a no-op.
	Setup(ctx context.Context, repoDir string) error

	// RunTests runs the language's test suite in repoDir and returns the
	// structured outcome. A ctx cancellation/timeout MUST be returned as an
	// error (infra failure), distinct from a non-zero test exit (which is a
	// TestOutcome with Passed=false, nil error).
	RunTests(ctx context.Context, repoDir string) (TestOutcome, error)

	// Capabilities statically declares the capability classes this runner's
	// tasks may exercise.
	Capabilities() []Capability
}
