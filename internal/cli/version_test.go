package cli

import (
	"os"
	"strings"
	"testing"
)

// TestFormatVersionDefault verifies the default sentinel output.
func TestFormatVersionDefault(t *testing.T) {
	// Snapshot and restore globals so other tests are unaffected.
	origV, origC, origD := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = origV, origC, origD })

	Version = "2.0.0-dev"
	Commit = "none"
	Date = "unknown"

	got := FormatVersion()
	want := "serena version 2.0.0-dev (commit none, built unknown)"
	if got != want {
		t.Fatalf("FormatVersion() = %q, want %q", got, want)
	}
}

// TestFormatVersionInjected verifies ldflag-injected metadata is rendered with
// the commit truncated to 7 hex chars.
func TestFormatVersionInjected(t *testing.T) {
	origV, origC, origD := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = origV, origC, origD })

	Version = "1.9.0"
	Commit = "abc1234567890def1234567890abcdef12345678"
	Date = "2026-04-26"

	got := FormatVersion()
	want := "serena version 1.9.0 (commit abc1234, built 2026-04-26)"
	if got != want {
		t.Fatalf("FormatVersion() = %q, want %q", got, want)
	}
}

// TestFormatVersionShortCommit ensures short (<7 char) commit values pass
// through untruncated and do not panic.
func TestFormatVersionShortCommit(t *testing.T) {
	origV, origC, origD := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = origV, origC, origD })

	Version = "1.9.0"
	Commit = "abcd"
	Date = "2026-04-26"

	got := FormatVersion()
	want := "serena version 1.9.0 (commit abcd, built 2026-04-26)"
	if got != want {
		t.Fatalf("FormatVersion() = %q, want %q", got, want)
	}
}

// TestRootGoLiteralRemoved is a regression gate: the bare "2.0.0-dev" literal
// must NOT live in root.go anymore — it belongs only in version.go defaults.
func TestRootGoLiteralRemoved(t *testing.T) {
	data, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatalf("read root.go: %v", err)
	}
	if strings.Contains(string(data), `"serena version 2.0.0-dev"`) {
		t.Fatal(`root.go still contains the literal "serena version 2.0.0-dev"; ` +
			`use FormatVersion() from version.go instead`)
	}
}
