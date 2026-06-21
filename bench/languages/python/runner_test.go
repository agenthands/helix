package python

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/bench/languages"
)

// Compile-time conformance assertion (criterion C3 / TOOLBENCH-10).
var _ languages.LanguageRunner = (*PyRunner)(nil)

// TestParsePytestJSONGolden is the SOLE authoritative proof of the parser
// (RESEARCH Pitfall 1 / MEMORY false-green): it parses a COMMITTED
// pytest-json-report fixture with NO subprocess, so it runs everywhere — even
// where pytest-json-report is not installed (as in this env). The fixture
// follows the documented pytest-json-report 1.5.0 schema
// (github.com/numirias/pytest-json-report README): tests[].nodeid +
// tests[].outcome (passed/failed/skipped).
func TestParsePytestJSONGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/pytest_report.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	got := parsePytestJSON(raw)
	if len(got) != 3 {
		t.Fatalf("parsePytestJSON returned %d rows, want 3: %+v", len(got), got)
	}

	byName := map[string]languages.TestResult{}
	for _, r := range got {
		byName[r.Name] = r
	}

	pass, ok := byName["test_widget.py::test_addition"]
	if !ok {
		t.Fatalf("missing passed row; got %+v", got)
	}
	if !pass.Passed || pass.Skipped {
		t.Errorf("addition: Passed=%v Skipped=%v, want Passed=true Skipped=false", pass.Passed, pass.Skipped)
	}

	fail, ok := byName["test_widget.py::test_subtraction"]
	if !ok {
		t.Fatalf("missing failed row; got %+v", got)
	}
	if fail.Passed || fail.Skipped {
		t.Errorf("subtraction: Passed=%v Skipped=%v, want Passed=false Skipped=false", fail.Passed, fail.Skipped)
	}

	skip, ok := byName["test_widget.py::test_division"]
	if !ok {
		t.Fatalf("missing skipped row; got %+v", got)
	}
	if skip.Passed || !skip.Skipped {
		t.Errorf("division: Passed=%v Skipped=%v, want Passed=false Skipped=true", skip.Passed, skip.Skipped)
	}
}

// TestParsePytestJSONMalformed proves the parser is total: a corrupt report does
// not panic and yields no rows (the exit code stays authoritative in RunTests).
func TestParsePytestJSONMalformed(t *testing.T) {
	if rows := parsePytestJSON([]byte("not json at all")); len(rows) != 0 {
		t.Errorf("malformed parse = %+v, want empty", rows)
	}
	if rows := parsePytestJSON(nil); len(rows) != 0 {
		t.Errorf("nil parse = %+v, want empty", rows)
	}
}

func TestDetect(t *testing.T) {
	for _, marker := range []string{"pyproject.toml", "setup.py", "requirements.txt"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, marker), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !(PyRunner{}).Detect(dir) {
			t.Errorf("Detect(dir with %s) = false, want true", marker)
		}
	}
	if (PyRunner{}).Detect(t.TempDir()) {
		t.Errorf("Detect(empty dir) = true, want false")
	}
}

func TestCapabilitiesAtLeastEight(t *testing.T) {
	caps := (PyRunner{}).Capabilities()
	if len(caps) < 8 {
		t.Errorf("Capabilities() = %d, want >= 8 (TOOLBENCH-03)", len(caps))
	}
	// All declared capabilities must be members of the closed enum.
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
	if languages.RunnerFor("internal-toolbench", "python") == nil {
		t.Errorf("RunnerFor(internal-toolbench, python) = nil, want the Python runner")
	}
}

// TestRunTestsLive is ADDITIVE — it re-validates the runner against the live
// pytest toolchain when present. It SKIPS cleanly when pytest or the
// pytest-json-report plugin is absent (as in this env), so it is never the sole
// proof of correctness (Pitfall 1).
func TestRunTestsLive(t *testing.T) {
	if _, err := exec.LookPath("pytest"); err != nil {
		t.Skip("pytest not on PATH — live layer skipped (golden test is the authoritative proof)")
	}
	// pytest-json-report plugin presence check: a probe run that asks for the
	// flag fails fast when the plugin is missing.
	probe := exec.Command("pytest", "--json-report", "--help")
	if out, err := probe.CombinedOutput(); err != nil || !bytes.Contains(out, []byte("--json-report")) {
		t.Skip("pytest-json-report plugin not installed — live layer skipped")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test_x.py"), []byte("def test_ok():\n    assert True\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := (PyRunner{}).RunTests(context.Background(), dir)
	if err != nil {
		t.Fatalf("RunTests infra error: %v", err)
	}
	if !out.Passed {
		t.Errorf("Passed = false on a green suite (raw: %s)", out.Raw)
	}
}
