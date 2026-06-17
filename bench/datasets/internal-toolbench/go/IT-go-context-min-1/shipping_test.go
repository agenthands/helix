package shipping

import "testing"

// TestShippingCost pins the corrected target. The spot-checks on the noise
// helpers assert the fix stayed minimal — touching them is unnecessary and the
// task is solvable using only the symbol get_context surfaces (D-05).
func TestShippingCost(t *testing.T) {
	cases := []struct {
		weight int
		want   int
	}{
		{0, 0},
		{1, 3},
		{4, 12},
		{10, 30},
	}
	for _, c := range cases {
		if got := ShippingCost(c.weight); got != c.want {
			t.Errorf("ShippingCost(%d) = %d, want %d", c.weight, got, c.want)
		}
	}

	// Noise helpers must remain unchanged.
	if Surcharge(10) != 15 {
		t.Errorf("Surcharge(10) = %d, want 15", Surcharge(10))
	}
	if !Discountable(150) {
		t.Errorf("Discountable(150) = false, want true")
	}
	if RoundUp(23) != 30 {
		t.Errorf("RoundUp(23) = %d, want 30", RoundUp(23))
	}
}
