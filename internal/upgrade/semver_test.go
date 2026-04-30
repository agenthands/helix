package upgrade

import "testing"

func TestSemverIsDowngrade(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		current string
		target  string
		want    bool
	}{
		// D-10: equal counts as downgrade-refused (already up to date).
		{"equal_no_v", "1.9.0", "1.9.0", true},
		{"equal_with_v", "v1.9.0", "v1.9.0", true},
		// Strict downgrade.
		{"older_target", "v1.9.0", "v1.8.0", true},
		{"older_minor", "v1.9.1", "v1.9.0", true},
		// Strict upgrade.
		{"newer_target", "v1.9.0", "v1.10.0", false},
		{"newer_patch", "v1.9.0", "v1.9.1", false},
		// Mixed leading-v normalization (Canonical handles both).
		{"mixed_v_prefix", "1.9.0", "v1.10.0", false},
		// Build metadata is not a pre-release; equal-version-with-build counts as no-op (downgrade).
		{"build_meta_equal", "v1.9.0", "v1.9.0+build.42", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsDowngrade(tc.current, tc.target); got != tc.want {
				t.Fatalf("IsDowngrade(%q, %q) = %v, want %v", tc.current, tc.target, got, tc.want)
			}
		})
	}
}

func TestSemverIsPrerelease(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tag  string
		want bool
	}{
		{"v1.9.0", false},
		{"v1.9.0-rc1", true},
		{"v1.9.0-beta.1", true},
		{"v1.9.0-alpha", true},
		// Build metadata is NOT a pre-release segment per SemVer 2.0.0 — RESEARCH.md A5.
		{"v1.9.0+build.42", false},
		{"1.9.0", false},
		{"1.9.0-rc1", true},
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			if got := IsPrerelease(tc.tag); got != tc.want {
				t.Fatalf("IsPrerelease(%q) = %v, want %v", tc.tag, got, tc.want)
			}
		})
	}
}

func TestSemverCanonical(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"1.9.0", "v1.9.0"},
		{"v1.9.0", "v1.9.0"},
		{"", ""},
		{"v0.0.1", "v0.0.1"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := Canonical(tc.in); got != tc.want {
				t.Fatalf("Canonical(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
