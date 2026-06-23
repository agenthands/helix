package fuzzyrobust

import (
	"testing"
)

// TestStrategy_Match: capturedStrategy == expectedStrategy reports a strategy
// match AND an editsim.ES similarity in [0,1] for matchedText vs expectedText.
func TestStrategy_Match(t *testing.T) {
	cs := ScoreCase(StrategyWhitespace, StrategyWhitespace, "return a + b", "return a + b")
	if !cs.StrategyMatch {
		t.Fatalf("ScoreCase same strategy: StrategyMatch=false, want true")
	}
	if cs.Similarity < 0 || cs.Similarity > 1 {
		t.Fatalf("Similarity=%v out of [0,1]", cs.Similarity)
	}
	if cs.Similarity != 1.0 {
		t.Fatalf("identical matched/expected: Similarity=%v, want 1.0", cs.Similarity)
	}
	if cs.Outcome != StrategyWhitespace {
		t.Fatalf("Outcome=%q, want %q", cs.Outcome, StrategyWhitespace)
	}
}

// TestStrategy_Mismatch: a captured strategy differing from the expected tier
// reports a strategy MISMATCH (the score grades selection, not just similarity).
// This is the real indent->whitespace divergence the cascade exhibits.
func TestStrategy_Mismatch(t *testing.T) {
	cs := ScoreCase(StrategyIndentationFlex, StrategyWhitespace, "x := 1", "x := 1")
	if cs.StrategyMatch {
		t.Fatalf("ScoreCase differing strategy: StrategyMatch=true, want false")
	}
	// Similarity can still be high (the text matched); selection is what failed.
	if cs.Similarity != 1.0 {
		t.Fatalf("identical text: Similarity=%v, want 1.0 (selection mismatch is orthogonal)", cs.Similarity)
	}
	if cs.Outcome != StrategyWhitespace {
		t.Fatalf("Outcome should reflect the CAPTURED outcome %q, got %q", StrategyWhitespace, cs.Outcome)
	}
}

// TestStrategy_Refusal: a case whose captured outcome is ambiguous_match is
// scored as a refusal (not a match, not a no_match) and never panics.
func TestStrategy_Refusal(t *testing.T) {
	cs := ScoreCase(OutcomeAmbiguous, OutcomeAmbiguous, "", "should be ignored")
	if !cs.Refused {
		t.Fatalf("captured ambiguous_match: Refused=false, want true")
	}
	if cs.NoMatch {
		t.Fatalf("a refusal must NOT be classified as no_match (Pitfall 6)")
	}
	if !cs.StrategyMatch {
		t.Fatalf("expected==captured==ambiguous_match should be a strategy match")
	}
	if cs.Outcome != OutcomeAmbiguous {
		t.Fatalf("Outcome=%q, want %q", cs.Outcome, OutcomeAmbiguous)
	}

	// A refusal is distinct from a no_match.
	nm := ScoreCase(OutcomeNoMatch, OutcomeNoMatch, "", "")
	if !nm.NoMatch {
		t.Fatalf("captured no_match: NoMatch=false, want true")
	}
	if nm.Refused {
		t.Fatalf("a no_match must NOT be classified as a refusal (Pitfall 6)")
	}
}

// TestStrategy_TotalNoPanic: ScoreCase is total on empty/nil/unicode inputs.
func TestStrategy_TotalNoPanic(t *testing.T) {
	_ = ScoreCase("", "", "", "")
	_ = ScoreCase("exact", "", "a", "")
	_ = ScoreCase("", "no_match", "", "b")
	_ = ScoreCase("日本語", "日本x", "日本語", "日本x")
	_ = ScoreCase(StrategyEllipsis, StrategyExact, "café", "cafe")
}
