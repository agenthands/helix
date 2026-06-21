package price

import "testing"

// TestDiscount fails against the deliberately-wrong body (returns price
// unchanged) and passes once the fuzzy_edit replaces the return with the 10%
// discount. The expected values pin the corrected behavior.
func TestDiscount(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{100, 90},
		{50, 45},
		{200, 180},
		{0, 0},
	}
	for _, c := range cases {
		if got := Discount(c.in); got != c.want {
			t.Errorf("Discount(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
