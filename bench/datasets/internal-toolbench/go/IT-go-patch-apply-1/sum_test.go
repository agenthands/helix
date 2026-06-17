package sumdoubler

import "testing"

// TestDouble fails against the deliberately-wrong seed implementation
// (Double returns x). It passes once the scripted edit makes Double return x*2.
func TestDouble(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, 0},
		{3, 6},
		{-4, -8},
		{21, 42},
	}
	for _, c := range cases {
		if got := Double(c.in); got != c.want {
			t.Errorf("Double(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
