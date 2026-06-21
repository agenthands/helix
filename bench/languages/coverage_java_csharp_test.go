package languages_test

import (
	"testing"

	"github.com/agenthands/helix/bench/languages"
	csharp "github.com/agenthands/helix/bench/languages/csharp"
	javarunner "github.com/agenthands/helix/bench/languages/java"
)

// TestJavaCapabilitiesAtLeast8 is the TOOLBENCH-06 capability gate: the authored
// Java corpus must cover at least 8 of the runner's declared capabilities.
func TestJavaCapabilitiesAtLeast8(t *testing.T) {
	declared := javarunner.JavaRunner{}.Capabilities()

	rep, err := languages.Coverage(corpusRoot, "internal-toolbench", "java", declared)
	if err != nil {
		t.Fatalf("Coverage() error: %v", err)
	}
	if rep.Covered < 8 {
		t.Errorf("java covered = %d, want >= 8 (declared %d, missing %v)",
			rep.Covered, rep.Declared, rep.Missing)
	}
}

// TestCSharpCapabilitiesAtLeast6 is the TOOLBENCH-07 capability gate: the authored
// C# corpus must cover at least 6 of the runner's declared capabilities.
func TestCSharpCapabilitiesAtLeast6(t *testing.T) {
	declared := csharp.CSharpRunner{}.Capabilities()

	rep, err := languages.Coverage(corpusRoot, "internal-toolbench", "csharp", declared)
	if err != nil {
		t.Fatalf("Coverage() error: %v", err)
	}
	if rep.Covered < 6 {
		t.Errorf("csharp covered = %d, want >= 6 (declared %d, missing %v)",
			rep.Covered, rep.Declared, rep.Missing)
	}
}
