package calc

import "testing"

// TestCalculator exercises EVERY method get_symbol_overview enumerates on
// calc.go (Add, Sub, Mul, Div). Any stub left as panic("not implemented") makes
// its subtest panic, so the fixture passes only if the agent implemented the
// FULL overview output (D-05).
func TestCalculator(t *testing.T) {
	c := Calculator{}
	if got := c.Add(7, 5); got != 12 {
		t.Errorf("Add(7,5) = %d, want 12", got)
	}
	if got := c.Sub(7, 5); got != 2 {
		t.Errorf("Sub(7,5) = %d, want 2", got)
	}
	if got := c.Mul(7, 5); got != 35 {
		t.Errorf("Mul(7,5) = %d, want 35", got)
	}
	if got := c.Div(20, 5); got != 4 {
		t.Errorf("Div(20,5) = %d, want 4", got)
	}
}
