package languages_test

import (
	"testing"

	"github.com/agenthands/helix/bench/languages"
	javascript "github.com/agenthands/helix/bench/languages/javascript"
	typescript "github.com/agenthands/helix/bench/languages/typescript"
)

// TestTSCapabilitiesAtLeast8 is the TOOLBENCH-04 capability gate: the authored
// TypeScript corpus must cover at least 8 of the runner's declared capabilities.
func TestTSCapabilitiesAtLeast8(t *testing.T) {
	declared := typescript.TSRunner{}.Capabilities()

	rep, err := languages.Coverage(corpusRoot, "internal-toolbench", "typescript", declared)
	if err != nil {
		t.Fatalf("Coverage() error: %v", err)
	}
	if rep.Covered < 8 {
		t.Errorf("typescript covered = %d, want >= 8 (declared %d, missing %v)",
			rep.Covered, rep.Declared, rep.Missing)
	}
}

// TestJSCapabilitiesAtLeast8 is the TOOLBENCH-05 capability gate: the authored
// JavaScript corpus must cover at least 8 of the runner's declared capabilities.
func TestJSCapabilitiesAtLeast8(t *testing.T) {
	declared := javascript.JSRunner{}.Capabilities()

	rep, err := languages.Coverage(corpusRoot, "internal-toolbench", "javascript", declared)
	if err != nil {
		t.Fatalf("Coverage() error: %v", err)
	}
	if rep.Covered < 8 {
		t.Errorf("javascript covered = %d, want >= 8 (declared %d, missing %v)",
			rep.Covered, rep.Declared, rep.Missing)
	}
}
