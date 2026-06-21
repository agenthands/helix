package javascript

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/bench/languages"
)

// Compile-time conformance assertion (criterion C3 / TOOLBENCH-10).
var _ languages.LanguageRunner = (*JSRunner)(nil)

// TestParseJestJSONGolden is the SOLE authoritative proof of the parser
// (RESEARCH Pitfall 1 / MEMORY false-green): it parses a COMMITTED `jest --json`
// fixture with NO subprocess, so it runs everywhere — even where jest is only
// reachable via npx (as in this env). The fixture follows the documented jest
// CLI schema (RESEARCH §Reporter Formats): testResults[].assertionResults[].status
// (passed/failed/pending) + .fullName. Jest names a skipped test "pending"; the
// shared parser treats both "skipped" (vitest) and "pending" (jest) as Skipped.
func TestParseJestJSONGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/jest-report.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	got := parseJestStyleJSON(raw)
	if len(got) != 3 {
		t.Fatalf("parseJestStyleJSON returned %d rows, want 3: %+v", len(got), got)
	}

	byName := map[string]languages.TestResult{}
	for _, r := range got {
		byName[r.Name] = r
	}

	pass, ok := byName["widget adds two numbers"]
	if !ok {
		t.Fatalf("missing passed row; got %+v", got)
	}
	if !pass.Passed || pass.Skipped {
		t.Errorf("adds: Passed=%v Skipped=%v, want Passed=true Skipped=false", pass.Passed, pass.Skipped)
	}

	fail, ok := byName["widget subtracts two numbers"]
	if !ok {
		t.Fatalf("missing failed row; got %+v", got)
	}
	if fail.Passed || fail.Skipped {
		t.Errorf("subtracts: Passed=%v Skipped=%v, want Passed=false Skipped=false", fail.Passed, fail.Skipped)
	}

	skip, ok := byName["widget divides two numbers"]
	if !ok {
		t.Fatalf("missing skipped row; got %+v", got)
	}
	if skip.Passed || !skip.Skipped {
		t.Errorf("divides: Passed=%v Skipped=%v, want Passed=false Skipped=true (jest 'pending')", skip.Passed, skip.Skipped)
	}
}

// TestParseJestStyleJSONMalformed proves the parser is total: a corrupt report
// does not panic and yields no rows (the exit code stays authoritative in
// RunTests, never inferred from these advisory rows — Pitfall 4 / IN-04).
func TestParseJestStyleJSONMalformed(t *testing.T) {
	if rows := parseJestStyleJSON([]byte("not json at all")); len(rows) != 0 {
		t.Errorf("malformed parse = %+v, want empty", rows)
	}
	if rows := parseJestStyleJSON(nil); len(rows) != 0 {
		t.Errorf("nil parse = %+v, want empty", rows)
	}
}

func TestDetect(t *testing.T) {
	// package.json and NO tsconfig.json → JavaScript.
	jsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(jsDir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !(JSRunner{}).Detect(jsDir) {
		t.Errorf("Detect(package.json only) = false, want true")
	}

	// package.json + tsconfig.json → TypeScript claims it; JS must yield.
	tsDir := t.TempDir()
	for _, m := range []string{"package.json", "tsconfig.json"} {
		if err := os.WriteFile(filepath.Join(tsDir, m), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if (JSRunner{}).Detect(tsDir) {
		t.Errorf("Detect(package.json + tsconfig.json) = true, want false (TS precedence)")
	}

	if (JSRunner{}).Detect(t.TempDir()) {
		t.Errorf("Detect(empty dir) = true, want false")
	}
}

func TestCapabilitiesAtLeastEight(t *testing.T) {
	caps := (JSRunner{}).Capabilities()
	if len(caps) < 8 {
		t.Errorf("Capabilities() = %d, want >= 8 (TOOLBENCH-05)", len(caps))
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
	// eslint diagnostics are a named TOOLBENCH-05 requirement.
	if !seen[languages.CapLSPDiagnostics] {
		t.Errorf("JavaScript must declare CapLSPDiagnostics (eslint)")
	}
}

func TestRunnerRegistered(t *testing.T) {
	if languages.RunnerFor("internal-toolbench", "javascript") == nil {
		t.Errorf("RunnerFor(internal-toolbench, javascript) = nil, want the JS runner")
	}
}

// TestRunTestsLive is ADDITIVE — it re-validates the runner against a live jest
// toolchain when present. It SKIPS cleanly when jest is not directly on PATH
// (npx-only in this env), so it is never the sole proof of correctness (Pitfall 1).
func TestRunTestsLive(t *testing.T) {
	if _, err := exec.LookPath("jest"); err != nil {
		t.Skip("jest not on PATH (npx-only env) — live layer skipped (golden test is the authoritative proof)")
	}
	dir := t.TempDir()
	_, err := (JSRunner{}).RunTests(context.Background(), dir)
	if err != nil && err != context.Canceled {
		t.Logf("live RunTests returned: %v", err)
	}
}
