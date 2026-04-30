package upgrade

import (
	"strings"

	"golang.org/x/mod/semver"
)

// Canonical normalizes a version string to the SemVer form expected by
// `golang.org/x/mod/semver` (which requires a leading `v`). Accepts both
// `v1.9.0` and `1.9.0` and returns the `v`-prefixed form. Empty input is
// returned unchanged so the caller's downstream `semver.IsValid` check
// fails loudly rather than this helper producing `v`.
func Canonical(v string) string {
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// IsDowngrade reports whether installing `target` would be a downgrade or
// no-op relative to `current`. Returns true when target ≤ current per
// `semver.Compare`.
//
// Equal versions count as a downgrade (no-op) per CONTEXT.md D-10:
// `helix upgrade` running against the same tag exits 0 with the
// "already up to date" message rather than re-installing the running
// binary. Build metadata (`+build.42`) is ignored by `semver.Compare`
// per the SemVer 2.0.0 spec.
func IsDowngrade(current, target string) bool {
	return semver.Compare(Canonical(target), Canonical(current)) <= 0
}

// IsPrerelease reports whether tag has a SemVer pre-release segment such
// as `-rc1`, `-beta.1`, or `-alpha`. Used by the upgrade flow to filter
// out pre-release tags unless the user explicitly passes `--prerelease`
// per CONTEXT.md D-11.
//
// Build metadata (`+build.42`) is NOT a pre-release segment and returns
// false (RESEARCH.md A5).
func IsPrerelease(tag string) bool {
	return semver.Prerelease(Canonical(tag)) != ""
}
