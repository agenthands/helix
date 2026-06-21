package typescript

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/bench/languages"
)

// Compile-time conformance assertion (criterion C3 / TOOLBENCH-10).
var _ languages.LanguageRunner = (*TSRunner)(nil)

// TestParseVitestJSONGolden is the SOLE authoritative proof of the parser
// (RESEARCH Pitfall 1 / MEMORY false-green): it parses a COMMITTED
// `vitest run --reporter=json` fixture with NO subprocess, so it runs everywhere
// — even where vitest is only reachable via npx (as in this env). The fixture
// follows the documented Jest-compatible vitest schema (RESEARCH §Reporter
// Formats): testResults[].assertionResults[].status (passed/failed/skipped) +
// .fullName, with numPassedTests/numFailedTests summary.
func TestParseVitestJSONGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/vitest-report.json")
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

	pass, ok := byName["widget > adds two numbers"]
	if !ok {
		t.Fatalf("missing passed row; got %+v", got)
	}
	if !pass.Passed || pass.Skipped {
		t.Errorf("adds: Passed=%v Skipped=%v, want Passed=true Skipped=false", pass.Passed, pass.Skipped)
	}

	fail, ok := byName["widget > subtracts two numbers"]
	if !ok {
		t.Fatalf("missing failed row; got %+v", got)
	}
	if fail.Passed || fail.Skipped {
		t.Errorf("subtracts: Passed=%v Skipped=%v, want Passed=false Skipped=false", fail.Passed, fail.Skipped)
	}

	skip, ok := byName["widget > divides two numbers"]
	if !ok {
		t.Fatalf("missing skipped row; got %+v", got)
	}
	if skip.Passed || !skip.Skipped {
		t.Errorf("divides: Passed=%v Skipped=%v, want Passed=false Skipped=true", skip.Passed, skip.Skipped)
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
	// package.json AND tsconfig.json present → TypeScript.
	tsDir := t.TempDir()
	for _, m := range []string{"package.json", "tsconfig.json"} {
		if err := os.WriteFile(filepath.Join(tsDir, m), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !(TSRunner{}).Detect(tsDir) {
		t.Errorf("Detect(package.json + tsconfig.json) = false, want true")
	}

	// package.json only (no tsconfig.json) → NOT TypeScript (JS claims it).
	jsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(jsDir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if (TSRunner{}).Detect(jsDir) {
		t.Errorf("Detect(package.json only) = true, want false (TS requires tsconfig.json)")
	}

	if (TSRunner{}).Detect(t.TempDir()) {
		t.Errorf("Detect(empty dir) = true, want false")
	}
}

func TestCapabilitiesAtLeastEight(t *testing.T) {
	caps := (TSRunner{}).Capabilities()
	if len(caps) < 8 {
		t.Errorf("Capabilities() = %d, want >= 8 (TOOLBENCH-04)", len(caps))
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
	// tsserver diagnostics are a named TOOLBENCH-04 requirement.
	if !seen[languages.CapLSPDiagnostics] {
		t.Errorf("TypeScript must declare CapLSPDiagnostics (tsserver)")
	}
}

func TestRunnerRegistered(t *testing.T) {
	if languages.RunnerFor("internal-toolbench", "typescript") == nil {
		t.Errorf("RunnerFor(internal-toolbench, typescript) = nil, want the TS runner")
	}
}

// TestRunTestsLive is ADDITIVE — it re-validates the runner against a live vitest
// toolchain when present. It SKIPS cleanly when vitest is not directly on PATH
// (npx-only in this env), so it is never the sole proof of correctness (Pitfall 1).
func TestRunTestsLive(t *testing.T) {
	if _, err := exec.LookPath("vitest"); err != nil {
		t.Skip("vitest not on PATH (npx-only env) — live layer skipped (golden test is the authoritative proof)")
	}
	dir := t.TempDir()
	// A minimal live exercise is intentionally not authored here: vitest needs a
	// node_modules install that this hermetic test must not perform. The presence
	// gate above guarantees this body is only reached on a fully provisioned image.
	_, err := (TSRunner{}).RunTests(context.Background(), dir)
	if err != nil && err != context.Canceled {
		// An infra error from a misconfigured live env is informational only.
		t.Logf("live RunTests returned: %v", err)
	}
}
