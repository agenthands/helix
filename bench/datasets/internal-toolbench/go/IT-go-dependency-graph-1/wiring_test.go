package depgraph

import (
	"testing"

	"toolbench/depgraph/alpha"
	"toolbench/depgraph/beta"
	"toolbench/depgraph/gamma"
)

// TestDependentsWired asserts that EVERY package the dependency graph shows
// depending on core surfaces core.Tag() through Label. Any package left with an
// empty Label fails its subtest, so the fixture passes only if the agent acted
// on the full dependency-graph output (D-05).
func TestDependentsWired(t *testing.T) {
	const want = "core:v1"
	cases := map[string]func() string{
		"alpha": alpha.Label,
		"beta":  beta.Label,
		"gamma": gamma.Label,
	}
	for name, label := range cases {
		if got := label(); got != want {
			t.Errorf("%s.Label() = %q, want %q (core.Tag must be wired in)", name, got, want)
		}
	}
}
