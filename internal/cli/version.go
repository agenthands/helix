package cli

import "fmt"

// Version, Commit, and Date are populated at link time via -ldflags by
// goreleaser (.goreleaser.yml) and the Makefile `build` target. Defaults
// are the "dev sentinel" values printed when ldflags are absent (e.g.
// `go install` from source). See .planning/phases/51-packaging-goreleaser/
// 51-CONTEXT.md D-15 for the policy.
var (
	Version = "2.0.0-dev"
	Commit  = "none"
	Date    = "unknown"
)

// FormatVersion returns the canonical --version output string.
// Commit is truncated to 7 chars when at least 7 chars long.
func FormatVersion() string {
	c := Commit
	if len(c) >= 7 {
		c = c[:7]
	}
	return fmt.Sprintf("serena version %s (commit %s, built %s)", Version, c, Date)
}
