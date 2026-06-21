package cpp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/bench/languages"
)

// Compile-time conformance assertion (criterion C3 / TOOLBENCH-10).
var _ languages.LanguageRunner = (*CppRunner)(nil)

// TestParseCtestJUnitGolden is the SOLE authoritative proof of the parser
// (RESEARCH Pitfall 1 / MEMORY false-green): it unmarshals a COMMITTED,
// live-captured (CMake 3.31.6) ctest --output-junit XML with NO subprocess, so it
// runs everywhere — even where ctest is absent. The fixture has one passing
// testcase (status="run", no child <failure>) and one failing testcase (a child
// <failure>); a child <failure> is the fail signal (RESEARCH §Reporter Formats).
func TestParseCtestJUnitGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/ctest-junit.xml")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	got := parseCtestJUnit(raw)
	if len(got) != 2 {
		t.Fatalf("parseCtestJUnit returned %d rows, want 2: %+v", len(got), got)
	}

	byName := map[string]languages.TestResult{}
	for _, r := range got {
		byName[r.Name] = r
	}

	if r, ok := byName["pass_test"]; !ok || !r.Passed || r.Skipped {
		t.Errorf("pass_test = %+v, want Passed=true Skipped=false", r)
	}
	if r, ok := byName["fail_test"]; !ok || r.Passed || r.Skipped {
		t.Errorf("fail_test = %+v, want Passed=false Skipped=false", r)
	}
}

// TestParseCtestJUnitMalformed proves the parser is total: a corrupt XML does not
// panic and yields no rows (the exit code stays authoritative in RunTests).
func TestParseCtestJUnitMalformed(t *testing.T) {
	if rows := parseCtestJUnit([]byte("<not-xml")); len(rows) != 0 {
		t.Errorf("malformed parse = %+v, want empty", rows)
	}
	if rows := parseCtestJUnit(nil); len(rows) != 0 {
		t.Errorf("nil parse = %+v, want empty", rows)
	}
}

func TestDetect(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CMakeLists.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !(CppRunner{}).Detect(dir) {
		t.Errorf("Detect(dir with CMakeLists.txt) = false, want true")
	}
	if (CppRunner{}).Detect(t.TempDir()) {
		t.Errorf("Detect(empty dir) = true, want false")
	}
}

func TestCapabilitiesAtLeastSix(t *testing.T) {
	caps := (CppRunner{}).Capabilities()
	if len(caps) < 6 {
		t.Errorf("Capabilities() = %d, want >= 6 (TOOLBENCH-08)", len(caps))
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
	if languages.RunnerFor("internal-toolbench", "cpp") == nil {
		t.Errorf("RunnerFor(internal-toolbench, cpp) = nil, want the C++ runner")
	}
}

// TestRunTestsLive is ADDITIVE — ctest IS present in this env, so this layer runs
// end-to-end against a tiny generated CMake project; it skips cleanly if ctest is
// removed. Never the sole proof (the golden test is).
func TestRunTestsLive(t *testing.T) {
	if _, err := exec.LookPath("ctest"); err != nil {
		t.Skip("ctest not on PATH — live layer skipped (golden ctest-JUnit test is the authoritative proof)")
	}
	cmake, err := exec.LookPath("cmake")
	if err != nil {
		t.Skip("cmake not on PATH — live layer skipped")
	}

	dir := t.TempDir()
	mustWrite(t, dir, "CMakeLists.txt", `cmake_minimum_required(VERSION 3.21)
project(golden CXX)
enable_testing()
add_executable(pass_test pass_test.cpp)
add_test(NAME pass_test COMMAND pass_test)
`)
	mustWrite(t, dir, "pass_test.cpp", "int main(){return 0;}\n")

	// Configure + build into <dir>/build, then point the runner at that build dir
	// (where ctest finds CTestTestfile.cmake).
	build := filepath.Join(dir, "build")
	if out, err := exec.Command(cmake, "-S", dir, "-B", build).CombinedOutput(); err != nil {
		t.Skipf("cmake configure failed (no compiler?): %v\n%s", err, out)
	}
	if out, err := exec.Command(cmake, "--build", build).CombinedOutput(); err != nil {
		t.Skipf("cmake build failed: %v\n%s", err, out)
	}

	out, runErr := (CppRunner{}).RunTests(context.Background(), build)
	if runErr != nil {
		t.Fatalf("RunTests infra error: %v", runErr)
	}
	if !out.Passed {
		t.Errorf("Passed = false on a green suite (exit %d, raw: %s)", out.ExitCode, out.Raw)
	}
}

func mustWrite(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
