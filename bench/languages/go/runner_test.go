package golang

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/bench/languages"
)

// Compile-time conformance assertion (criterion C3 / TOOLBENCH-10).
var _ languages.LanguageRunner = (*GoRunner)(nil)

// writeModule creates a tiny Go module on disk under a fresh temp dir and returns
// the dir. files maps relative path → contents.
func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	return dir
}

func TestCapabilitiesAreTheTenClasses(t *testing.T) {
	caps := (&GoRunner{}).Capabilities()
	if len(caps) != 10 {
		t.Fatalf("Capabilities() = %d classes, want 10: %v", len(caps), caps)
	}
	want := map[languages.Capability]bool{
		languages.CapSemanticView:        true,
		languages.CapLSPDiagnostics:      true,
		languages.CapRenameSafety:        true,
		languages.CapFuzzySearch:         true,
		languages.CapCallGraph:           true,
		languages.CapDependencyGraph:     true,
		languages.CapPatchApply:          true,
		languages.CapContextMinimization: true,
		languages.CapIncrementalUpdate:   true,
		languages.CapFailureHandling:     true,
	}
	got := map[languages.Capability]bool{}
	for _, c := range caps {
		got[c] = true
	}
	for c := range want {
		if !got[c] {
			t.Errorf("Capabilities() missing %q", c)
		}
	}
}

func TestDetect(t *testing.T) {
	withMod := writeModule(t, map[string]string{"go.mod": "module x\n\ngo 1.21\n"})
	if !(&GoRunner{}).Detect(withMod) {
		t.Errorf("Detect(dir with go.mod) = false, want true")
	}
	withoutMod := t.TempDir()
	if (&GoRunner{}).Detect(withoutMod) {
		t.Errorf("Detect(dir without go.mod) = true, want false")
	}
}

func TestRunTestsPassing(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module x\n\ngo 1.21\n",
		"x_test.go": `package x

import "testing"

func TestAlwaysPasses(t *testing.T) {}
`,
	})
	out, err := (&GoRunner{}).RunTests(context.Background(), dir)
	if err != nil {
		t.Fatalf("RunTests infra error: %v", err)
	}
	if !out.Passed {
		t.Errorf("Passed = false, want true (raw: %s)", out.Raw)
	}
	if out.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", out.ExitCode)
	}
	if len(out.Tests) < 1 {
		t.Fatalf("len(Tests) = %d, want >= 1", len(out.Tests))
	}
	if len(out.Raw) == 0 {
		t.Errorf("Raw is empty, want non-empty test2json stream")
	}
	var found bool
	for _, tr := range out.Tests {
		if tr.Name == "TestAlwaysPasses" {
			found = true
			if !tr.Passed {
				t.Errorf("TestAlwaysPasses parsed Passed=false, want true")
			}
		}
	}
	if !found {
		t.Errorf("did not find TestAlwaysPasses in parsed Tests: %+v", out.Tests)
	}
}

func TestRunTestsFailing(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module x\n\ngo 1.21\n",
		"x_test.go": `package x

import "testing"

func TestAlwaysFails(t *testing.T) { t.Fatal("boom") }
`,
	})
	out, err := (&GoRunner{}).RunTests(context.Background(), dir)
	if err != nil {
		t.Fatalf("RunTests infra error: %v", err)
	}
	if out.Passed {
		t.Errorf("Passed = true, want false")
	}
	if out.ExitCode == 0 {
		t.Errorf("ExitCode = 0, want non-zero")
	}
	var found bool
	for _, tr := range out.Tests {
		if tr.Name == "TestAlwaysFails" {
			found = true
			if tr.Passed {
				t.Errorf("TestAlwaysFails parsed Passed=true, want false")
			}
		}
	}
	if !found {
		t.Errorf("did not find TestAlwaysFails in parsed Tests: %+v", out.Tests)
	}
}

// Pitfall 4: a module that does NOT compile must be Passed=false with empty Tests
// and a non-zero exit — do NOT infer pass from zero failing test rows.
func TestRunTestsCompileFailure(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module x\n\ngo 1.21\n",
		"x_test.go": `package x

import "testing"

func TestBroken(t *testing.T) { this is not valid go }
`,
	})
	out, err := (&GoRunner{}).RunTests(context.Background(), dir)
	if err != nil {
		t.Fatalf("RunTests infra error: %v", err)
	}
	if out.Passed {
		t.Errorf("Passed = true on compile failure, want false")
	}
	if out.ExitCode == 0 {
		t.Errorf("ExitCode = 0 on compile failure, want non-zero")
	}
	if len(out.Tests) != 0 {
		t.Errorf("Tests = %+v on compile failure, want empty", out.Tests)
	}
}

func TestRunnerForFallbackSignal(t *testing.T) {
	if r := languages.RunnerFor("internal-toolbench", "go"); r == nil {
		t.Errorf("RunnerFor(internal-toolbench, go) = nil, want the Go runner")
	}
	if r := languages.RunnerFor("internal-toolbench", "rust"); r != nil {
		t.Errorf("RunnerFor(internal-toolbench, rust) = %v, want nil fallback signal", r)
	}
	if r := languages.RunnerFor("other", "go"); r != nil {
		t.Errorf("RunnerFor(other, go) = %v, want nil fallback signal", r)
	}
}
