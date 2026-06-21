// Package aiderpolyglot is the dataset-loader-only Aider Polyglot adapter
// (ADAPTER-AIDER-01). It does NOT run the upstream Python harness: it shallow-
// clones Aider-AI/polyglot-benchmark at a PINNED sha (pin.go), maps each
// exercise's .meta/config.json (files.solution / files.test / files.example),
// and drives the upstream 2-attempt + stderr-reprompt protocol (tries=2,
// timeout=180s) using the dataset's NATIVE per-language test commands (pytest /
// cargo / gradlew / jest / ctest / go) — never the TOOLBENCH runner argv.
//
// It is a LEAF package: it imports only the Go standard library, mirroring the
// bench/ragindex + bench/container leaf discipline (no internal/kernel,
// internal/semantic, or bench/runtime imports). The hermetic fixture-set test
// (loader_test.go) is the SOLE authoritative proof of the config.json mapping +
// 2-attempt protocol; the live pinned-sha clone (pin.go / clone_test.go) is
// network-gated and skips cleanly offline, never the sole proof.
package aiderpolyglot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// tries is the upstream aider hardcoded attempt count (benchmark.py:
// `for i in range(tries=2)`). The driver makes at most this many agent edits.
const tries = 2

// attemptTimeout is the upstream aider per-attempt timeout (benchmark.py
// timeout=180s). Threaded into RunExercise's per-attempt context budget.
const attemptTimeout = 180 * time.Second

// Config decodes the subset of an exercise's .meta/config.json that the adapter
// needs: the files.solution (the stub the agent edits), files.test (restored each
// attempt), and files.example (reference under .meta/) lists, plus an optional
// non_hermetic marker (SC#3 — a task whose test phase needs network).
type Config struct {
	Files struct {
		// Solution lists the stub file(s) the agent edits (e.g. ["wordy.py"]).
		Solution []string `json:"solution"`
		// Test lists the test file(s) restored pristine each attempt.
		Test []string `json:"test"`
		// Example lists the reference solution path(s) under .meta/.
		Example []string `json:"example"`
	} `json:"files"`
	// NonHermetic, when true in the config, marks the exercise as needing network
	// at test time (SC#3). flagNonHermetic also applies per-language rules.
	NonHermetic bool `json:"non_hermetic"`
}

// Exercise is a loaded, validated dataset exercise. It carries the language
// (persisted onto the result.v2 `language` field from Plan 01 so the aggregator
// can slice per-language pass-rate for SC#1) and the source dir from which the
// pristine solution/test files are restored each attempt.
type Exercise struct {
	// Name is the validated exercise directory name (V5 path-segment safe).
	Name string
	// Language is the per-track language (python/rust/go/java/javascript/cpp);
	// it populates the result.v2 `language` provenance field (Plan 01).
	Language string
	// Config is the decoded .meta/config.json file mapping.
	Config Config
	// SrcDir is the on-disk exercise dir the pristine files are copied FROM.
	SrcDir string
	// NonHermetic records whether this exercise needs network at test time (SC#3),
	// either from the config marker or a per-language rule (flagNonHermetic).
	NonHermetic bool
	// basePrompt is the initial agent instruction (unexported; set by the loader
	// or the test). Attempt 2 appends the captured failure text to it.
	basePrompt string
}

// validatePathSegment rejects names that could escape a join root once they
// become a path segment — a clone of bench/runtime/validate.go's predicate
// (V5 / T-85-07-02), kept in this leaf package so it carries no bench/runtime
// import. It rejects "", anything filepath.Clean rewrites ("..", "a//b"), any
// embedded separator, and a leading dot. MUST run BEFORE any filepath.Join.
func validatePathSegment(name, kind string) error {
	if name == "" {
		return fmt.Errorf("%s is empty", kind)
	}
	if name != filepath.Clean(name) || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("%s %q contains path separators, parent refs, or a leading dot", kind, name)
	}
	return nil
}

// loadExercise reads dir/.meta/config.json and returns a validated Exercise. The
// exercise name (the last path element of dir) is validated via
// validatePathSegment BEFORE it is trusted, so a traversal segment is rejected
// up front (T-85-07-02). NO network is touched: this is the hermetic mapping the
// fixture-set test drives.
func loadExercise(dir, language string) (*Exercise, error) {
	name := filepath.Base(dir)
	if err := validatePathSegment(name, "exercise name"); err != nil {
		return nil, err
	}
	if err := validatePathSegment(language, "language"); err != nil {
		return nil, err
	}
	cfgPath := filepath.Join(dir, ".meta", "config.json")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("aiderpolyglot: read %s: %w", cfgPath, err)
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("aiderpolyglot: parse %s: %w", cfgPath, err)
	}
	if len(cfg.Files.Solution) == 0 || len(cfg.Files.Test) == 0 {
		return nil, fmt.Errorf("aiderpolyglot: %s missing files.solution or files.test", cfgPath)
	}
	ex := &Exercise{
		Name:     name,
		Language: language,
		Config:   cfg,
		SrcDir:   dir,
	}
	flagNonHermetic(ex)
	return ex, nil
}

// copyFile copies srcRoot/rel to workRoot/rel verbatim, creating parent dirs. rel
// is taken from the validated config; it is cleaned and rejected if it escapes
// either root (defense in depth on top of the exercise-name validation).
func copyFile(srcRoot, workRoot, rel string) error {
	if rel != filepath.Clean(rel) || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("aiderpolyglot: unsafe relative path %q", rel)
	}
	src := filepath.Join(srcRoot, rel)
	dst := filepath.Join(workRoot, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// restorePristine restores the exercise's pristine solution stub(s) AND test
// file(s) from SrcDir into workRoot, so the work tree starts from a clean
// solution stub and the original (un-tampered) test file — exactly the upstream
// benchmark.py "restore solution from pristine before running tests" step. The
// agent edits the work copy; the pristine source is never mutated. It is used to
// seed the work tree before the first attempt.
func restorePristine(ex *Exercise, srcRoot, workRoot string) error {
	for _, rel := range ex.Config.Files.Solution {
		if err := copyFile(srcRoot, workRoot, rel); err != nil {
			return fmt.Errorf("aiderpolyglot: restore solution %q: %w", rel, err)
		}
	}
	return restorePristineTests(ex, srcRoot, workRoot)
}

// restorePristineTests restores ONLY the pristine test file(s) from SrcDir into
// workRoot. This is the load-bearing anti-tamper step (WR-01): the upstream
// invariant is that the agent may iteratively edit the SOLUTION stub across
// attempts, but the graded TEST file must be reset to its committed form before
// EACH test run so a test-tampering agent cannot force a spurious green. Unlike
// restorePristine it deliberately does NOT reset the solution stub, preserving
// the agent's iterative fix between attempts. The pristine source is never
// mutated.
func restorePristineTests(ex *Exercise, srcRoot, workRoot string) error {
	for _, rel := range ex.Config.Files.Test {
		if err := copyFile(srcRoot, workRoot, rel); err != nil {
			return fmt.Errorf("aiderpolyglot: restore test %q: %w", rel, err)
		}
	}
	return nil
}

// TestResult is the outcome of one test-run attempt. Output carries the captured
// stderr/stdout that the 2-attempt driver re-prompts the agent with on failure.
type TestResult struct {
	// Passed reports whether the native test command succeeded (exit 0).
	Passed bool
	// Output is the captured combined test stderr/stdout used to re-prompt the
	// agent on attempt 2 (the upstream "errors + test_failures" instruction).
	Output string
}

// TestFn runs the exercise's native test command against the work dir and
// returns the structured result. It is injected so the hermetic test supplies a
// fake (no toolchain invoked); the live driver supplies a real implementation
// that shells out to nativeTestCommand under attemptTimeout.
type TestFn func(ctx context.Context, ex *Exercise, workDir string) TestResult

// AgentFn lets the agent edit the solution stub in workDir given the current
// prompt (the base prompt on attempt 1; base + captured failure on attempt 2).
type AgentFn func(ctx context.Context, ex *Exercise, workDir string, prompt string) error

// AttemptResult summarizes a RunExercise outcome.
type AttemptResult struct {
	// Passed is true if any attempt's tests passed.
	Passed bool
	// Attempts is the number of agent edit + test cycles actually run (1 or 2).
	Attempts int
	// LastOutput is the final attempt's captured test output (empty on pass).
	LastOutput string
}

// RunExercise drives the upstream aider 2-attempt + stderr-reprompt protocol
// (benchmark.py): for attempt in 0..tries(2): the agent edits the stub, the
// pristine TEST file(s) are restored, the native tests run, on pass the loop
// breaks, on fail the next prompt becomes base + the captured failure output and
// the loop continues. runTests and agent are injected so the hermetic test
// drives the EXACT protocol with NO toolchain. Each attempt's test run is given
// an attemptTimeout (180s) child context.
//
// Anti-tamper invariant (WR-01): the graded test file is restored to its
// committed (pristine) form AFTER the agent edits but BEFORE each test run, so an
// agent that edits a files.test path cannot force a spurious green — the edited
// solution stub is preserved across attempts (iterative fixing) while the test is
// reset every attempt. Restoration runs only when ex.SrcDir is set (a real loaded
// exercise); the pure-scripted fake-tester unit tests leave it empty and skip it.
func RunExercise(ctx context.Context, ex *Exercise, workDir string, runTests TestFn, agent AgentFn) AttemptResult {
	prompt := ex.basePrompt
	var res AttemptResult
	for i := 0; i < tries; i++ {
		res.Attempts++
		if err := agent.editOrCall(ctx, ex, workDir, prompt); err != nil {
			// An agent error ends the attempt as a failure; the loop may retry.
			res.LastOutput = err.Error()
			prompt = ex.basePrompt + "\n\n# Previous attempt failed:\n" + err.Error()
			continue
		}
		// Restore the pristine test file(s) before grading so a test-tampering
		// agent cannot produce a false pass (upstream benchmark.py invariant).
		if ex.SrcDir != "" {
			if err := restorePristineTests(ex, ex.SrcDir, workDir); err != nil {
				res.LastOutput = err.Error()
				prompt = ex.basePrompt + "\n\n# Previous attempt failed:\n" + err.Error()
				continue
			}
		}
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		tr := runTests(attemptCtx, ex, workDir)
		cancel()
		if tr.Passed {
			res.Passed = true
			res.LastOutput = ""
			return res
		}
		res.LastOutput = tr.Output
		// Re-prompt attempt 2 with the captured failure (upstream "errors +
		// test_failures" instruction), exactly as benchmark.py sets instructions.
		prompt = ex.basePrompt + "\n\n# The tests failed with this output. Fix the solution:\n" + tr.Output
	}
	return res
}

// editOrCall adapts a bare AgentFn so RunExercise can call it uniformly. (The
// indirection keeps AgentFn a plain func type while letting the driver invoke it
// without a nil check scattered through the loop.)
func (a AgentFn) editOrCall(ctx context.Context, ex *Exercise, workDir, prompt string) error {
	if a == nil {
		return fmt.Errorf("aiderpolyglot: nil agent")
	}
	return a(ctx, ex, workDir, prompt)
}

// nativeTestCommand returns the dataset's OWN per-language test argv (aider's
// authoritative run_unit_tests table — RESEARCH §Aider Polyglot Dataset), NOT
// the HELIX TOOLBENCH runner commands. The first element is the binary/script;
// the rest are fixed args. An unknown language fail-closes.
func nativeTestCommand(language string) ([]string, error) {
	switch language {
	case "python":
		return []string{"pytest"}, nil
	case "rust":
		// --include-ignored (after "--" so cargo forwards it to libtest) is
		// LOAD-BEARING (WR-02): Exercism's Rust track marks all but the first
		// acceptance test #[ignore], so plain `cargo test` runs only one test and
		// a do-nothing stub that merely compiles exits 0 → a vacuous false pass.
		// Running the ignored tests is what makes the Rust pass/fail trustworthy.
		// The TOOLBENCH rust runner (bench/languages/rust/runner.go) deliberately
		// keeps plain `cargo test` for ITS own dataset; this adapter must NOT.
		return []string{"cargo", "test", "--", "--include-ignored"}, nil
	case "go":
		return []string{"go", "test", "./..."}, nil
	case "java":
		return []string{"./gradlew", "test"}, nil
	case "javascript":
		return []string{"./npm-test.sh"}, nil
	case "cpp":
		return []string{"./cpp-test.sh"}, nil
	default:
		return nil, fmt.Errorf("aiderpolyglot: no native test command for language %q", language)
	}
}

// flagNonHermetic records whether the exercise needs network at test time (SC#3),
// from either the config's non_hermetic marker or a per-language rule. The loaded
// dataset carries this flag so the runner can route only hermetic tasks through
// the Plan 02 container.Run(--network=none) seam and explicitly defer (or
// network-allow) the flagged ones. Per RESEARCH §Pitfall (offline dep
// resolution), languages whose toolchain typically fetches deps at test time
// without a pre-resolved cache are conservatively flagged.
func flagNonHermetic(ex *Exercise) {
	if ex.Config.NonHermetic {
		ex.NonHermetic = true
		return
	}
	// Conservative per-language rule: rust/java/javascript test phases fetch
	// crates/artifacts/node_modules unless a pre-resolved offline cache is baked
	// into the toolchain image (RESEARCH §Pitfall, SC#3). Python/Go/C++ fixtures
	// in this dataset are hermetic with the committed stubs.
	switch ex.Language {
	case "rust", "java", "javascript":
		ex.NonHermetic = true
	}
}
