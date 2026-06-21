package languages_test

import (
	"testing"

	"github.com/agenthands/helix/bench/languages"
	python "github.com/agenthands/helix/bench/languages/python"
)

// TestPyCapabilitiesAtLeast8 is the TOOLBENCH-03 capability gate: the authored
// Python corpus must cover at least 8 of the runner's declared capabilities. It
// mirrors how TestCoverageGoIsTenOfTen exercises the aggregator, but asserts a
// ≥8 floor (not an exact 10/10) per the per-language thresholds.
func TestPyCapabilitiesAtLeast8(t *testing.T) {
	declared := python.PyRunner{}.Capabilities()

	rep, err := languages.Coverage(corpusRoot, "internal-toolbench", "python", declared)
	if err != nil {
		t.Fatalf("Coverage() error: %v", err)
	}
	if rep.Covered < 8 {
		t.Errorf("python covered = %d, want >= 8 (declared %d, missing %v)",
			rep.Covered, rep.Declared, rep.Missing)
	}
}
