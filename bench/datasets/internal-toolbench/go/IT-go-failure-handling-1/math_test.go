package mathfix

import "testing"

// TestNegate fails against the deliberately-wrong body (returns n) and passes
// only after the recovery edit makes Negate return -n. The failed edit against
// the wrong path must NOT corrupt math.go, so the recovery is what flips the
// test green.
func TestNegate(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, 0},
		{5, -5},
		{-7, 7},
		{42, -42},
	}
	for _, c := range cases {
		if got := Negate(c.in); got != c.want {
			t.Errorf("Negate(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
