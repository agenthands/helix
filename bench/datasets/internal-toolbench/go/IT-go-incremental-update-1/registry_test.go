package incrementalupdate

import "testing"

// TestUpdateIncrements fails against the deliberately-wrong pre-edit state
// (registeredCapability() returns "legacy", so Update resolves the zero handler
// and returns 0 for every input). It passes only after the scripted edit flips
// the token to "incremental", wiring Update to the incrementalHandler (n+1).
//
// This assertion depends on the REFRESHED state: the scripted agent edits the
// token, then calls refresh_semantic_graph{wait_for_lsp:true} (the capability's
// named tool) to drive the Phase 70 overlay-drain so the live semantic graph
// reflects the new registeredCapability()->registry wiring, then queries it.
// The go test below pins the post-refresh behavior.
func TestUpdateIncrements(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, 1},
		{1, 2},
		{41, 42},
		{-1, 0},
	}
	for _, c := range cases {
		if got := Update(c.in); got != c.want {
			t.Errorf("Update(%d) = %d, want %d (capability token must be wired to the incremental handler)", c.in, got, c.want)
		}
	}
}

// TestRegisteredCapabilityIsIncremental pins the token directly: post-edit it
// must be "incremental" (a registered key), so Lookup resolves the real handler.
func TestRegisteredCapabilityIsIncremental(t *testing.T) {
	if got := registeredCapability(); got != "incremental" {
		t.Fatalf("registeredCapability() = %q, want \"incremental\" (the registered capability token)", got)
	}
	if _, ok := registry[registeredCapability()]; !ok {
		t.Fatalf("registeredCapability() %q is not a registered capability key", registeredCapability())
	}
}
