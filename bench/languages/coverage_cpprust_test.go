package languages_test

import (
	"testing"

	"github.com/agenthands/helix/bench/languages"
	cpp "github.com/agenthands/helix/bench/languages/cpp"
)

// TestCppCapabilitiesAtLeast6 is the TOOLBENCH-08 capability gate: the authored
// C++ corpus must cover at least 6 of the runner's declared capabilities.
func TestCppCapabilitiesAtLeast6(t *testing.T) {
	declared := cpp.CppRunner{}.Capabilities()

	rep, err := languages.Coverage(corpusRoot, "internal-toolbench", "cpp", declared)
	if err != nil {
		t.Fatalf("Coverage() error: %v", err)
	}
	if rep.Covered < 6 {
		t.Errorf("cpp covered = %d, want >= 6 (declared %d, missing %v)",
			rep.Covered, rep.Declared, rep.Missing)
	}
}
