// Tests for the closed-enum constants and the ClassifyLookupErr classifier.
// Phase 65 D-04 / D-05 — exact string values are SPEC §24 contract; any rename
// breaks every consumer's envelope shape.
package integ

import (
	"errors"
	"fmt"
	"testing"
)

// TestSource_ClosedEnum locks the wire-string for each Source constant. SPEC
// §24 phrasing: "semantic" / "tree_sitter" / "fallback".
func TestSource_ClosedEnum(t *testing.T) {
	cases := []struct {
		got  Source
		want string
	}{
		{SourceSemantic, "semantic"},
		{SourceTreeSitter, "tree_sitter"},
		{SourceFallback, "fallback"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("Source constant: got %q, want %q", string(c.got), c.want)
		}
	}
}

// TestFallbackReason_ClosedEnum locks the wire-string for each FallbackReason
// constant. SPEC §24 phrasing.
func TestFallbackReason_ClosedEnum(t *testing.T) {
	cases := []struct {
		got  FallbackReason
		want string
	}{
		{FallbackReasonIndexDisabled, "index_disabled"},
		{FallbackReasonNoSnapshotYet, "no_snapshot_yet"},
		{FallbackReasonIndexBuilding, "index_building"},
		{FallbackReasonIndexError, "index_error"},
		{FallbackReasonBleveRebuilding, "bleve_rebuilding"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("FallbackReason constant: got %q, want %q", string(c.got), c.want)
		}
	}
}

// TestClassifyLookupErr asserts the errors.Is ladder maps each sentinel onto
// the matching FallbackReason. Per Pitfall §3 doctrine, the classifier MUST
// NOT emit FallbackReasonIndexDisabled — that decision belongs to the
// consumer's source-selection step (D-04 / Pattern 5).
func TestClassifyLookupErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want FallbackReason
	}{
		{"nil err is empty", nil, FallbackReason("")},
		{"ErrNoSnapshot", ErrNoSnapshot, FallbackReasonNoSnapshotYet},
		{"ErrIndexBuilding", ErrIndexBuilding, FallbackReasonIndexBuilding},
		{"ErrIndexErrored", ErrIndexErrored, FallbackReasonIndexError},
		{"ErrBleveRebuilding", ErrBleveRebuilding, FallbackReasonBleveRebuilding},
		{"unknown err defaults to IndexError", errors.New("oops"), FallbackReasonIndexError},
		// Wrapped sentinels still classify correctly (errors.Is ladder).
		{"wrapped ErrNoSnapshot", fmt.Errorf("upstream: %w", ErrNoSnapshot), FallbackReasonNoSnapshotYet},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifyLookupErr(c.err)
			if got != c.want {
				t.Errorf("ClassifyLookupErr(%v) = %q, want %q", c.err, got, c.want)
			}
			if got == FallbackReasonIndexDisabled {
				t.Fatalf("ClassifyLookupErr must not emit FallbackReasonIndexDisabled (Pitfall §3 doctrine)")
			}
		})
	}
}
