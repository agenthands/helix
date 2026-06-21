package broken

import "testing"

// TestGreeting can only run if broken.go compiles, which requires every
// get_diagnostics error to be fixed (unused import, wrong return type, undefined
// variable). It also pins the corrected behavior: Greeting returns a string.
func TestGreeting(t *testing.T) {
	if got := Greeting("Ada"); got != "Hello, Ada!" {
		t.Errorf("Greeting(%q) = %q, want %q", "Ada", got, "Hello, Ada!")
	}
}
