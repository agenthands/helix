package golang

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestGuessFromName_Deterministic asserts that the production suffix set
// returns a stable result across many iterations. CR-03 closure: with a
// map literal + `for k, v := range` Go's randomised iteration order does
// not bite today only because the production suffix set is overlap-free
// at the tail level — but the contract must be locked structurally.
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
// pair where one suffix IS a tail of another. Under map iteration this
// would flip-flop; under sorted slice iteration the lexicographically
// smaller short ("Repo") wins consistently.
func TestGuessFromName_DeterministicUnderOverlap(t *testing.T) {
	rules := []suffixRule{
		{short: "Repo", long: "Repository"},
		{short: "oRepo", long: "ObservedRepository"},
	}
	first := guessFromNameWithRules("FooRepo", rules)
	for i := 0; i < 100; i++ {
		got := guessFromNameWithRules("FooRepo", rules)
		if got != first {
			t.Fatalf("iter %d: got %q, want %q (stable)", i, got, first)
		}
	}
	// "Repo" sorts before "oRepo" lexicographically so it MUST be the winner.
	if first != "FooRepository" {
		t.Fatalf("expected sorted slice to pick %q (lex-smaller short wins), got %q", "FooRepository", first)
	}
}

// TestGuessFromName_NoMapIteration is a meta-guard against future
// regressions to the bare `for k, v := range suffixMap` pattern. Phase
// 62's whole-tree determinism doctrine forbids it.
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
