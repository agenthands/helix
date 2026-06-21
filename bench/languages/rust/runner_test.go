package rust

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/bench/languages"
)

// Compile-time conformance assertion (criterion C3 / TOOLBENCH-10).
var _ languages.LanguageRunner = (*RustRunner)(nil)

// TestParseLibtestTextGolden is the SOLE authoritative proof of the parser
// (RESEARCH Pitfall 1 / Pitfall 2 / MEMORY false-green): it parses a COMMITTED,
// live-captured (cargo 1.96.0) libtest TEXT stream with NO subprocess, so it runs
// everywhere. The fixture has one `... ok`, one `... FAILED`, and one
// `... ignored`, plus the surrounding `running N tests`, `failures:`, the
// `---- NAME stdout ----` block, the indented failure list, and the
// `test result:` summary — all of which the parser must ignore (re-sync) except
// the three real per-test lines. CRITICAL: this is libtest TEXT, NOT
// --message-format=json (which carries no per-test rows on stable, Pitfall 2).
func TestParseLibtestTextGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/cargo-libtest.txt")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	got := parseLibtestText(raw)
	if len(got) != 3 {
		t.Fatalf("parseLibtestText returned %d rows, want 3: %+v", len(got), got)
	}

	byName := map[string]languages.TestResult{}
	for _, r := range got {
		byName[r.Name] = r
	}

	if r, ok := byName["tests::it_works"]; !ok || !r.Passed || r.Skipped {
		t.Errorf("it_works = %+v, want Passed=true Skipped=false", r)
	}
	if r, ok := byName["tests::it_fails"]; !ok || r.Passed || r.Skipped {
		t.Errorf("it_fails = %+v, want Passed=false Skipped=false", r)
	}
	if r, ok := byName["tests::it_is_ignored"]; !ok || r.Passed || !r.Skipped {
		t.Errorf("it_is_ignored = %+v, want Passed=false Skipped=true", r)
	}
}

// TestParseLibtestTextMalformed proves the parser is total: garbage input does
// not panic and yields no rows (the exit code stays authoritative in RunTests).
// A long line well past the 64KiB default scanner buffer must not abort parsing.
func TestParseLibtestTextMalformed(t *testing.T) {
	if rows := parseLibtestText([]byte("not a libtest stream at all\nrandom\n")); len(rows) != 0 {
		t.Errorf("malformed parse = %+v, want empty", rows)
	}
	if rows := parseLibtestText(nil); len(rows) != 0 {
		t.Errorf("nil parse = %+v, want empty", rows)
	}
	// A 1MiB line followed by a real row must not exhaust the scanner.
	huge := make([]byte, 1<<20)
	for i := range huge {
		huge[i] = 'x'
	}
	stream := append(huge, []byte("\ntest m::a ... ok\n")...)
	if rows := parseLibtestText(stream); len(rows) != 1 || !rows[0].Passed {
		t.Errorf("long-line parse = %+v, want one passing row", rows)
	}
}

// TestRustDoesNotUseMessageFormatJSON is the anti-Pitfall-2 guard: it asserts the
// runner's exec argv parses libtest TEXT and NEVER asks for --message-format json
// (which would silently yield zero per-test rows on stable Rust). It inspects the
// argv the runner constructs via the exported testArgv hook.
func TestRustDoesNotUseMessageFormatJSON(t *testing.T) {
	argv := testArgv()
	if len(argv) == 0 || argv[0] != "cargo" {
		t.Fatalf("argv = %v, want it to start with cargo", argv)
	}
	for _, a := range argv {
		if a == "--message-format" || a == "--message-format=json" || a == "json" {
			t.Fatalf("argv %v contains a --message-format json flag (Pitfall 2 re-introduced)", argv)
		}
	}
}

func TestDetect(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !(RustRunner{}).Detect(dir) {
		t.Errorf("Detect(dir with Cargo.toml) = false, want true")
	}
	if (RustRunner{}).Detect(t.TempDir()) {
		t.Errorf("Detect(empty dir) = true, want false")
	}
}

func TestCapabilitiesAtLeastEight(t *testing.T) {
	caps := (RustRunner{}).Capabilities()
	if len(caps) < 8 {
		t.Errorf("Capabilities() = %d, want >= 8 (TOOLBENCH-09)", len(caps))
	}
	valid := map[languages.Capability]bool{
		languages.CapSemanticView: true, languages.CapLSPDiagnostics: true,
		languages.CapRenameSafety: true, languages.CapFuzzySearch: true,
		languages.CapCallGraph: true, languages.CapDependencyGraph: true,
		languages.CapPatchApply: true, languages.CapContextMinimization: true,
		languages.CapIncrementalUpdate: true, languages.CapFailureHandling: true,
	}
	seen := map[languages.Capability]bool{}
	for _, c := range caps {
		if !valid[c] {
			t.Errorf("declared unknown capability %q", c)
		}
		if seen[c] {
			t.Errorf("declared duplicate capability %q", c)
		}
		seen[c] = true
	}
}

func TestRunnerRegistered(t *testing.T) {
	if languages.RunnerFor("internal-toolbench", "rust") == nil {
		t.Errorf("RunnerFor(internal-toolbench, rust) = nil, want the Rust runner")
	}
}

// TestRunTestsLive is ADDITIVE — cargo IS present in this env, so this layer runs
// end-to-end against a tiny generated crate; it skips cleanly if cargo is
// removed. Never the sole proof (the golden test is).
func TestRunTestsLive(t *testing.T) {
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not on PATH — live layer skipped (golden libtest-text test is the authoritative proof)")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(
		"[package]\nname = \"golden\"\nversion = \"0.1.0\"\nedition = \"2021\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "lib.rs"), []byte(
		"#[cfg(test)]\nmod tests {\n    #[test]\n    fn it_works() { assert_eq!(2 + 2, 4); }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := (RustRunner{}).RunTests(context.Background(), dir)
	if err != nil {
		t.Skipf("RunTests infra error (no network / offline registry?): %v", err)
	}
	if !out.Passed {
		t.Errorf("Passed = false on a green crate (exit %d, raw: %s)", out.ExitCode, out.Raw)
	}
}
