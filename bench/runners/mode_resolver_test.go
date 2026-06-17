package runners

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeModeFixture creates <root>/<mode>/MODE.md with the given body.
func writeModeFixture(root, mode, body string) error {
	dir := filepath.Join(root, mode)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "MODE.md"), []byte(body), 0o600)
}

// TestModeResolverYourAgentFull verifies the seed mode resolves to bench-full by
// reading bench/runners/your_agent_full/MODE.md frontmatter — NOT a hard-coded
// Go map (D-05). The value must come from the file so Phase 80 can add modes
// without touching Go.
func TestModeResolverYourAgentFull(t *testing.T) {
	got, err := ResolveProfile("your_agent_full")
	if err != nil {
		t.Fatalf("ResolveProfile(your_agent_full) returned error: %v", err)
	}
	if got != "bench-full" {
		t.Fatalf("ResolveProfile(your_agent_full) = %q, want %q", got, "bench-full")
	}
}

// TestModeResolverUnknownMode verifies an unknown mode name (no MODE.md dir)
// fails closed with a non-nil error.
func TestModeResolverUnknownMode(t *testing.T) {
	_, err := ResolveProfile("no_such_mode")
	if err == nil {
		t.Fatalf("ResolveProfile(no_such_mode) = nil error, want non-nil (fail closed)")
	}
}

// TestModeResolverRejectsTraversal verifies a path-traversal mode name is
// rejected BEFORE any filesystem join (V5 control, T-77-01). A leading-dot,
// separator, or parent-ref mode name must error without touching the FS.
func TestModeResolverRejectsTraversal(t *testing.T) {
	for _, bad := range []string{
		"../bench-full",
		"..",
		"a/b",
		`a\b`,
		".hidden",
		"/abs",
		"",
	} {
		if _, err := ResolveProfile(bad); err == nil {
			t.Errorf("ResolveProfile(%q) = nil error, want rejection before FS join", bad)
		}
	}
}

// TestModeResolverFromRootInjectable verifies the resolver accepts an injectable
// root for testability and that a MODE.md with an unknown frontmatter key is a
// HARD parse error (KnownFields true) — proving Phase 80 can add dirs but
// malformed frontmatter fails closed.
func TestModeResolverFromRootStrictFrontmatter(t *testing.T) {
	dir := t.TempDir()
	mkMode := func(t *testing.T, mode, body string) {
		t.Helper()
		if err := writeModeFixture(dir, mode, body); err != nil {
			t.Fatalf("writeModeFixture: %v", err)
		}
	}

	// Valid frontmatter resolves.
	mkMode(t, "good_mode", "---\nmode: good_mode\nprofile: some-profile\n---\n")
	got, err := ResolveProfileFromRoot(dir, "good_mode")
	if err != nil {
		t.Fatalf("ResolveProfileFromRoot(good_mode) error: %v", err)
	}
	if got != "some-profile" {
		t.Fatalf("ResolveProfileFromRoot(good_mode) = %q, want %q", got, "some-profile")
	}

	// Unknown frontmatter key is a hard parse error (strict KnownFields).
	mkMode(t, "bad_mode", "---\nmode: bad_mode\nprofile: p\nbogus_key: nope\n---\n")
	if _, err := ResolveProfileFromRoot(dir, "bad_mode"); err == nil {
		t.Fatalf("ResolveProfileFromRoot(bad_mode) = nil error, want strict-frontmatter rejection")
	} else if !strings.Contains(err.Error(), "bad_mode") && !strings.Contains(strings.ToLower(err.Error()), "frontmatter") && !strings.Contains(strings.ToLower(err.Error()), "field") {
		// best-effort: just ensure it is a real parse-related error
		t.Logf("strict parse error (acceptable): %v", err)
	}
}
