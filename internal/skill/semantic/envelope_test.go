package semantic

import (
	"encoding/json"
	"testing"
)

// TestFreshnessEnum_ClosedSet asserts the four Freshness constants match the
// SPEC §26.2 closed enum strings.
func TestFreshnessEnum_ClosedSet(t *testing.T) {
	cases := []struct {
		name string
		val  Freshness
		want string
	}{
		{"fresh", FreshnessFresh, "fresh"},
		{"stale", FreshnessStale, "stale"},
		{"structurally_fresh_semantically_pending", FreshnessStructurallyFreshSemanticallyPending, "structurally_fresh_semantically_pending"},
		{"overlay_active", FreshnessOverlayActive, "overlay_active"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.val) != tc.want {
				t.Fatalf("Freshness mismatch: got %q, want %q", tc.val, tc.want)
			}
		})
	}
}

// TestIndexStatusEnum_ClosedSet asserts the three IndexStatus constants match
// the SPEC §23.1 closed enum strings.
func TestIndexStatusEnum_ClosedSet(t *testing.T) {
	cases := []struct {
		name string
		val  IndexStatus
		want string
	}{
		{"committed", IndexStatusCommitted, "committed"},
		{"building", IndexStatusBuilding, "building"},
		{"failed", IndexStatusFailed, "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.val) != tc.want {
				t.Fatalf("IndexStatus mismatch: got %q, want %q", tc.val, tc.want)
			}
		})
	}
}

// TestClusterStatus_DefaultUnknownShape asserts that the documented W1 shape
// (State="unknown", Reason="phase-62-clustering-no-status-accessor")
// JSON-marshals to the documented field tags.
func TestClusterStatus_DefaultUnknownShape(t *testing.T) {
	cs := ClusterStatus{
		State:  "unknown",
		Reason: "phase-62-clustering-no-status-accessor",
	}
	got, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	want := `{"state":"unknown","reason":"phase-62-clustering-no-status-accessor"}`
	if string(got) != want {
		t.Fatalf("ClusterStatus JSON mismatch:\n got  = %s\n want = %s", got, want)
	}

	// Reason is omitempty when empty: cs2.Reason == "" should not emit the
	// reason key (regression guard against a future tag change).
	cs2 := ClusterStatus{State: "current"}
	got2, err := json.Marshal(cs2)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	want2 := `{"state":"current"}`
	if string(got2) != want2 {
		t.Fatalf("ClusterStatus omitempty mismatch:\n got  = %s\n want = %s", got2, want2)
	}
}
