// Package rust is the Rust LanguageRunner: it wraps `cargo test` and parses the
// libtest TEXT stdout (NOT --message-format=json) into a structured TestOutcome,
// and registers itself for (internal-toolbench, rust) via init(). It is a clone
// of bench/languages/go/runner.go — only Detect, the Setup argv, the
// exec.CommandContext invocation, and the parser differ; the ctx.Err()→infra vs
// *exec.ExitError→exit-code split and the Passed=(exit code == 0) authoritative
// gate are IDENTICAL (D-10 / Pitfall 4).
//
// Pitfall 2 (RESEARCH): `cargo test --message-format=json` emits only
// compiler-artifact / build-finished JSON on STABLE Rust — NO per-test rows. So
// per-test detail MUST come from the plain libtest TEXT lines
// (`test NAME ... ok|FAILED|ignored`), parsed here with the Go template's
// malformed-line-resync bufio.Scanner discipline. The exit code remains the
// authoritative Passed gate; rows are advisory. An explicit argv-assertion test
// (TestRustDoesNotUseMessageFormatJSON) forbids re-introducing the json flag.
package rust

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agenthands/helix/bench/languages"
)

func init() {
	languages.Register("internal-toolbench", "rust", &RustRunner{})
}

// RustRunner runs Rust (cargo test) suites. It is stateless and safe to share.
type RustRunner struct{}

// Compile-time conformance.
var _ languages.LanguageRunner = (*RustRunner)(nil)

// cargoTestArgv is the fixed argv the runner shells out with. It is a package
// var (not interpolated from task code, mitigating T-85-04-01) and deliberately
// excludes `--message-format` / `json` (Pitfall 2 / T-85-04-04). testArgv exposes
// it to the anti-Pitfall-2 test.
var cargoTestArgv = []string{"cargo", "test"}

// testArgv returns a copy of the fixed cargo argv for the anti-Pitfall-2 guard
// (TestRustDoesNotUseMessageFormatJSON).
func testArgv() []string {
	out := make([]string, len(cargoTestArgv))
	copy(out, cargoTestArgv)
	return out
}

// Detect reports whether repoDir contains a Cargo.toml. Analog of the Go runner's
// go.mod check.
func (RustRunner) Detect(repoDir string) bool {
	_, err := os.Stat(filepath.Join(repoDir, "Cargo.toml"))
	return err == nil
}

// Setup honors cancellation only (WR-05): returning ctx.Err() yields nil for a
// live context and the cancellation error when already cancelled, so a future
// non-trivial Rust setup (cargo fetch) copied from this shape does not silently
// ignore an already-cancelled context.
func (RustRunner) Setup(ctx context.Context, repoDir string) error {
	return ctx.Err()
}

// Capabilities declares 9 of the 10 capability classes (≥8 required by
// TOOLBENCH-09). CapLSPDiagnostics is omitted as an explicit, declared gap: the
// rust-analyzer diagnostic surface is not yet wired into the toolbench corpus, so
// it is a declared gap rather than a silently-claimed capability.
func (RustRunner) Capabilities() []languages.Capability {
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

// RunTests runs `cargo test` (PLAIN — NO --message-format=json, Pitfall 2) in
// repoDir, captures the libtest TEXT stdout, parses per-test rows, and reports
// Passed=(exit code == 0) as the authoritative gate. A ctx cancellation/timeout
// is returned as an infra error (mirrors the Go runner's ctx.Err()→infra vs
// *exec.ExitError→exit-code split); a non-zero test exit is a TestOutcome
// (Passed=false, nil err).
//
// Pitfall 4: a compile failure produces a non-zero exit with zero parsed test
// rows. Such an outcome is Passed=false with empty Tests — pass is NEVER inferred
// from the absence of failing rows.
func (RustRunner) RunTests(ctx context.Context, repoDir string) (languages.TestOutcome, error) {
	cmd := exec.CommandContext(ctx, cargoTestArgv[0], cargoTestArgv[1:]...)
	cmd.Dir = repoDir

	raw, err := cmd.Output()
	// cmd.Output captures stdout (where libtest writes its per-test lines); on a
	// non-zero exit it returns an *exec.ExitError but the stdout it captured in
	// raw is still intact.
	if ctx.Err() != nil {
		// Cancellation/timeout: infra error, not a task outcome.
		return languages.TestOutcome{Raw: raw}, ctx.Err()
	}

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			// Non-ExitError (e.g. `cargo` not on PATH): infra failure.
			return languages.TestOutcome{Raw: raw}, err
		}
	}

	return languages.TestOutcome{
		Passed:   exitCode == 0,
		Tests:    parseLibtestText(raw),
		Raw:      raw,
		ExitCode: exitCode,
	}, nil
}

// parseLibtestText streams cargo's libtest TEXT output and collects per-test rows
// from lines of the form `test <name> ... ok|FAILED|ignored` (ok → Passed=true;
// FAILED → Passed=false; ignored → Skipped=true). Every other line — the
// `running N tests` banner, the `failures:` section, the `---- NAME stdout ----`
// blocks, the indented failure list, and the `test result:` summary — is a
// non-matching line that is SKIPPED (re-sync), never aborting the scan (IN-04).
//
// This mirrors the Go template's malformed-line-resync bufio.Scanner discipline
// (go/runner.go:127-159) and grows the scanner buffer past the 64KiB default
// (T-85-04-03). It is detail-only: Passed is gated on the subprocess exit code in
// RunTests, never inferred from these advisory rows (Pitfall 2 / Pitfall 4 /
// T-85-04-02), so a crafted text line can never flip pass/fail.
func parseLibtestText(raw []byte) []languages.TestResult {
	var results []languages.TestResult
	scan := bufio.NewScanner(bytes.NewReader(raw))
	// libtest lines are short, but a stray very-long line (e.g. captured stdout
	// from a panic) must not abort the scan — grow well past the 64KiB default.
	scan.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scan.Scan() {
		line := bytes.TrimSpace(scan.Bytes())
		// A libtest per-test line is exactly: test <name> ... <status>
		// Anchor on the "test " prefix and the " ... " separator; anything else
		// (banner, failures section, summary, blank) is skipped and we re-sync.
		if !bytes.HasPrefix(line, []byte("test ")) {
			continue
		}
		idx := bytes.Index(line, []byte(" ... "))
		if idx < 0 {
			// e.g. the "test result: ..." summary line has no " ... " separator.
			continue
		}
		name := string(bytes.TrimSpace(line[len("test "):idx]))
		status := string(bytes.TrimSpace(line[idx+len(" ... "):]))
		if name == "" {
			continue
		}
		switch status {
		case "ok":
			results = append(results, languages.TestResult{Name: name, Passed: true})
		case "FAILED":
			results = append(results, languages.TestResult{Name: name, Passed: false})
		case "ignored":
			results = append(results, languages.TestResult{Name: name, Skipped: true})
		default:
			// Unknown status token (e.g. "ok (N tests)" benchmark forms or a
			// truncated line) — skip, re-sync. Exit code stays authoritative.
			continue
		}
	}
	return results
}
