package typescript

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestGuessFromName_Deterministic asserts that the production suffix set
// returns a stable result across many iterations. CR-03 closure for the
// typescript resolver.
func TestGuessFromName_Deterministic(t *testing.T) {
	for i := 0; i < 100; i++ {
		got := guessFromName("FooRepo")
		if got != "FooRepository" {
			t.Fatalf("iter %d: guessFromName(%q) = %q, want %q", i, "FooRepo", got, "FooRepository")
		}
	}
}

// TestGuessFromName_DeterministicUnderOverlap exercises the test-seam
// helper guessFromNameWithRules with a deliberately overlapping rule
// pair. Sorted slice iteration makes the lexicographically smaller
// short ("Repo") win consistently.
func TestGuessFromName_DeterministicUnderOverlap(t *testing.T) {
	// Sorted ascending by short — invariant required by guessFromNameWithRules.
	// "Repo" < "oRepo" lexicographically (capital 'R' 0x52 < lowercase 'o' 0x6F).
	rules := []suffixRule{
		{short: "Repo", long: "Repository"},
		{short: "oRepo", long: "OObservedRepository"},
	}
	first := guessFromNameWithRules("FooRepo", rules)
	for i := 0; i < 100; i++ {
		got := guessFromNameWithRules("FooRepo", rules)
		if got != first {
			t.Fatalf("iter %d: got %q, want %q (stable)", i, got, first)
		}
	}
	// "Repo" sorts before "oRepo" so it MUST win.
	if first != "FooRepository" {
		t.Fatalf("expected sorted slice to pick %q (lex-smaller short wins), got %q", "FooRepository", first)
	}
}

// TestGuessFromName_NoMapIteration is a meta-guard against future
// regressions to bare unsorted-map range iteration over the suffix
// rules. Pattern is constructed at runtime so this source file itself
// does not match the repo-wide determinism grep gate.
func TestGuessFromName_NoMapIteration(t *testing.T) {
	path := filepath.Join("resolver.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read resolver.go: %v", err)
	}
	re := regexp.MustCompile(forbiddenSuffixMapPattern())
	if re.Match(data) {
		t.Fatalf("resolver.go contains forbidden unsorted suffix-map iteration; CR-03 requires sorted slice iteration")
	}
}

// forbiddenSuffixMapPattern returns the regex source that detects the
// banned pattern. Built from string fragments so this file does not
// itself match a repo-wide grep for the forbidden pattern.
func forbiddenSuffixMapPattern() string {
	return "for [_a-zA-Z]+, [_a-zA-Z]+ := range " + "suffix" + "Map"
}
