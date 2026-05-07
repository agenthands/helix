package python

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestGuessFromName_Deterministic asserts that the production suffix set
// returns a stable result across many iterations. CR-03 closure for the
// python resolver: snake_case mirror of the Go test.
func TestGuessFromName_Deterministic(t *testing.T) {
	for i := 0; i < 100; i++ {
		got := guessFromName("user_repo")
		if got != "UserRepository" {
			t.Fatalf("iter %d: guessFromName(%q) = %q, want %q", i, "user_repo", got, "UserRepository")
		}
	}
}

// TestGuessFromName_DeterministicUnderOverlap exercises the test-seam
// helper guessFromNameWithRules with a deliberately overlapping rule
// pair where one suffix's "_short" tail subsumes another. Sorted slice
// iteration makes the lexicographically smaller short ("bar_repo") win
// — under map iteration this would flip-flop run to run.
func TestGuessFromName_DeterministicUnderOverlap(t *testing.T) {
	// Sorted ascending by short — invariant required by guessFromNameWithRules.
	// "bar_repo" < "repo" lexicographically, so bar_repo MUST win when both
	// match `foo_bar_repo` (suffix _repo and suffix _bar_repo both match).
	rules := []suffixRule{
		{short: "bar_repo", long: "BarRepoFull"},
		{short: "repo", long: "Repository"},
	}
	first := guessFromNameWithRules("foo_bar_repo", rules)
	for i := 0; i < 100; i++ {
		got := guessFromNameWithRules("foo_bar_repo", rules)
		if got != first {
			t.Fatalf("iter %d: got %q, want %q (stable)", i, got, first)
		}
	}
	// "bar_repo" sorts before "repo" lexicographically so it MUST win.
	if first != "FooBarRepoFull" {
		t.Fatalf("expected sorted slice to pick %q (lex-smaller short wins), got %q", "FooBarRepoFull", first)
	}
}

// TestGuessFromName_NoMapIteration is a meta-guard against future
// regressions to the bare `for k, v := range suffixMap` pattern.
func TestGuessFromName_NoMapIteration(t *testing.T) {
	path := filepath.Join("resolver.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read resolver.go: %v", err)
	}
	re := regexp.MustCompile(`for [_a-zA-Z]+, [_a-zA-Z]+ := range suffixMap`)
	if re.Match(data) {
		t.Fatalf("resolver.go contains forbidden pattern `for k, v := range suffixMap`; CR-03 requires sorted slice iteration")
	}
}
