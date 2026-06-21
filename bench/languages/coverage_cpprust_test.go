package languages_test

import (
	"testing"

	"github.com/agenthands/helix/bench/languages"
	cpp "github.com/agenthands/helix/bench/languages/cpp"
	rust "github.com/agenthands/helix/bench/languages/rust"
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

// TestRustCapabilitiesAtLeast8 is the TOOLBENCH-09 capability gate: the authored
// Rust corpus must cover at least 8 of the runner's declared capabilities.
func TestRustCapabilitiesAtLeast8(t *testing.T) {
	declared := rust.RustRunner{}.Capabilities()

	rep, err := languages.Coverage(corpusRoot, "internal-toolbench", "rust", declared)
	if err != nil {
		t.Fatalf("Coverage() error: %v", err)
	}
	if rep.Covered < 8 {
		t.Errorf("rust covered = %d, want >= 8 (declared %d, missing %v)",
			rep.Covered, rep.Declared, rep.Missing)
	}
}
