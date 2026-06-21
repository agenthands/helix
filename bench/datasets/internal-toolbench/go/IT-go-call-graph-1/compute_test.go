package callgraph

import "testing"

// TestCallersGuarded calls every incoming caller of Compute with a zero divisor.
// Each unguarded site panics (integer divide by zero), so the fixture passes
// only if the agent guarded ALL callers the incoming call hierarchy surfaced
// (RunA, RunB, RunC) — the D-05 force-consumption assertion.
func TestCallersGuarded(t *testing.T) {
	callers := map[string]func(int) int{
		"RunA": RunA,
		"RunB": RunB,
		"RunC": RunC,
	}
	for name, fn := range callers {
		t.Run(name, func(t *testing.T) {
			if got := fn(0); got != -1 {
				t.Errorf("%s(0) = %d, want -1 (zero divisor must be guarded)", name, got)
			}
			// Non-zero still computes normally.
			if got := fn(4); got != 25 {
				t.Errorf("%s(4) = %d, want 25", name, got)
			}
		})
	}
}
