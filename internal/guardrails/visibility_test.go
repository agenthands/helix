// visibility_test.go — Tests for guardrails.Visibility alias parity.
// Phase 66 Plan 02.
package guardrails

import (
	"testing"

	"github.com/agenthands/helix/internal/semantic/integ"
)

// TestVisibility_AliasParity asserts that the guardrails package re-exports
// integ.Visibility values without modification — both at compile time (type
// compatibility) and at runtime (constant equality). This test ensures that
// rule predicates importing guardrails receive exactly the same enum values
// as code importing integ directly.
func TestVisibility_AliasParity(t *testing.T) {
	// Compile-time check: guardrails.Visibility is assignable from integ.Visibility.
	var _ Visibility = integ.VisExported
	var _ integ.Visibility = VisExported

	// Runtime equality checks for all six constants.
	pairs := []struct {
		guardrailsVal Visibility
		integVal      integ.Visibility
		name          string
	}{
		{VisPublic, integ.VisPublic, "VisPublic"},
		{VisExported, integ.VisExported, "VisExported"},
		{VisProtected, integ.VisProtected, "VisProtected"},
		{VisPackage, integ.VisPackage, "VisPackage"},
		{VisPrivate, integ.VisPrivate, "VisPrivate"},
		{VisUnknown, integ.VisUnknown, "VisUnknown"},
	}

	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			if p.guardrailsVal != p.integVal {
				t.Errorf("guardrails.%s (%q) != integ.%s (%q)", p.name, p.guardrailsVal, p.name, p.integVal)
			}
		})
	}
}

// TestVisibility_ParseVisibilityParity asserts that guardrails.ParseVisibility
// and integ.ParseVisibility produce identical results for a representative
// sample of inputs.
func TestVisibility_ParseVisibilityParity(t *testing.T) {
	inputs := []string{"exported", "public", "protected", "package", "private", "", "junk"}
	for _, raw := range inputs {
		t.Run(raw, func(t *testing.T) {
			got := ParseVisibility(raw)
			want := integ.ParseVisibility(raw)
			if got != want {
				t.Errorf("ParseVisibility(%q): guardrails=%q integ=%q", raw, got, want)
			}
		})
	}
}

// TestVisibility_IsPublicLike_ThroughAlias asserts that IsPublicLike() on
// guardrails-package Visibility values behaves identically to the method on
// integ.Visibility values (since it is a type alias, the method set is shared).
func TestVisibility_IsPublicLike_ThroughAlias(t *testing.T) {
	tests := []struct {
		vis  Visibility
		want bool
	}{
		{VisPublic, true},
		{VisExported, true},
		{VisProtected, true},
		{VisPackage, false},
		{VisPrivate, false},
		{VisUnknown, false},
	}
	for _, tc := range tests {
		t.Run(string(tc.vis), func(t *testing.T) {
			if tc.vis.IsPublicLike() != tc.want {
				t.Errorf("(%q).IsPublicLike() = %v, want %v", tc.vis, tc.vis.IsPublicLike(), tc.want)
			}
		})
	}
}
