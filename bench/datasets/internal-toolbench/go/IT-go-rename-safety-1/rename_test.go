package rename

import "testing"

// TestRenamed references the NEW name (Bar) directly, and exercises Describe,
// which also called the old Foo. The package compiles only if the rename
// touched EVERY reference find_references surfaced (the definition in widget.go
// and the call site in consumer.go) — the D-05 force-consumption assertion.
func TestRenamed(t *testing.T) {
	if got := Bar(); got != "widget" {
		t.Errorf("Bar() = %q, want %q", got, "widget")
	}
	if got := Describe(); got != "this is a widget" {
		t.Errorf("Describe() = %q, want %q", got, "this is a widget")
	}
}
