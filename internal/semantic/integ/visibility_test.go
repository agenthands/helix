// visibility_test.go — Tests for Visibility closed enum and ParseVisibility.
// Phase 66 Plan 02.
package integ

import (
	"testing"
)

// TestParseVisibility exercises the full translation table for ParseVisibility.
// 8 rows minimum per plan spec; this covers all canonical extractor strings,
// the empty-string sentinel (T-66-10 mitigation), an unknown junk value,
// and mixed-case input (must map to VisUnknown — parsers are case-sensitive
// because extractor strings are code-generated constants, not user input).
func TestParseVisibility(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Visibility
	}{
		// Canonical extractor strings — one per enum value.
		{name: "exported", raw: "exported", want: VisExported},
		{name: "public", raw: "public", want: VisPublic},
		{name: "protected", raw: "protected", want: VisProtected},
		{name: "package", raw: "package", want: VisPackage},
		{name: "private", raw: "private", want: VisPrivate},
		// Degenerate inputs — all must map to VisUnknown (T-66-10 mitigation).
		{name: "empty_string", raw: "", want: VisUnknown},
		{name: "unknown_junk", raw: "internal-only", want: VisUnknown},
		// Mixed-case must NOT match (extractor strings are canonical lowercase).
		{name: "mixed_case_Exported", raw: "Exported", want: VisUnknown},
		// A few extra cases to cover future extractor additions.
		{name: "numeric_string", raw: "0", want: VisUnknown},
		{name: "whitespace_only", raw: "   ", want: VisUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseVisibility(tc.raw)
			if got != tc.want {
				t.Errorf("ParseVisibility(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// TestIsPublicLike verifies the IsPublicLike truth table: exactly VisPublic,
// VisExported, and VisProtected return true; all others return false.
// This is the gate predicate for G-002 / G-003 (plan spec).
func TestIsPublicLike(t *testing.T) {
	tests := []struct {
		vis  Visibility
		want bool
	}{
		{VisPublic, true},
		{VisExported, true},
		{VisProtected, true},
		{VisPackage, false},
		{VisPrivate, false},
		{VisUnknown, false}, // T-66-10: unknown must NEVER be public-like
	}

	for _, tc := range tests {
		t.Run(string(tc.vis), func(t *testing.T) {
			got := tc.vis.IsPublicLike()
			if got != tc.want {
				t.Errorf("(%q).IsPublicLike() = %v, want %v", tc.vis, got, tc.want)
			}
		})
	}
}

// TestVisibility_RoundTrip confirms that ParseVisibility(string(v)) == v for
// every canonical value (i.e., the string representation is stable and
// self-consistent). VisUnknown is excluded from the round-trip check because
// it maps from multiple inputs ("", junk, etc.) and string(VisUnknown) is
// "unknown" which does NOT appear in the extractors, so it parses back to
// VisUnknown via the default branch — correct, but tested explicitly.
func TestVisibility_RoundTrip(t *testing.T) {
	canonical := []Visibility{
		VisPublic, VisExported, VisProtected, VisPackage, VisPrivate,
	}
	for _, v := range canonical {
		t.Run(string(v), func(t *testing.T) {
			got := ParseVisibility(string(v))
			if got != v {
				t.Errorf("ParseVisibility(string(%q)) = %q, want %q", v, got, v)
			}
		})
	}
	// VisUnknown round-trip: string("unknown") is NOT in the switch, so maps to VisUnknown.
	if got := ParseVisibility(string(VisUnknown)); got != VisUnknown {
		t.Errorf("ParseVisibility(%q) = %q, want VisUnknown", string(VisUnknown), got)
	}
}
