package fuzzyrobust

import (
	"testing"
)

// refusalGate is the predicate under test: it returns true iff the duplicate-
// block ambiguous case's captured outcome is the ambiguity REFUSAL
// (ambiguous_match, ErrAmbiguous), NOT no_match and NOT a successful match
// (Pitfall 6 / D-06). It is the single point both the green-path assertion
// (TestAmbiguousRefused) and the bite assertion (TestAmbiguousBites) exercise,
// so a flipped outcome demonstrably fails the SAME gate.
func refusalGate(captured CapturedOutcomes, ambiguousID string) bool {
	o, ok := captured.Outcomes[ambiguousID]
	if !ok {
		return false // an absent captured outcome is a failed gate (anti-vacuity)
	}
	return o.Strategy == OutcomeAmbiguous && o.Refused
}

// findAmbiguous returns the single duplicate-block ambiguous case ID for a
// language's drift corpus (D-06: >= 1 ships per language).
func findAmbiguous(t *testing.T, lang string) (string, DriftCorpus) {
	t.Helper()
	corpus, err := LoadDrift(testdataDir, lang)
	if err != nil {
		t.Fatalf("LoadDrift(%q): %v", lang, err)
	}
	for _, c := range corpus.Cases {
		if c.Ambiguous {
			if c.ExpectedStrategy != OutcomeAmbiguous {
				t.Fatalf("%s: ambiguous case %q has ExpectedStrategy=%q, want %q",
					lang, c.ID, c.ExpectedStrategy, OutcomeAmbiguous)
			}
			return c.ID, corpus
		}
	}
	t.Fatalf("%s: no duplicate-block ambiguous case found in drift corpus", lang)
	return "", corpus
}

// TestAmbiguousRefused asserts that for every language the committed captured
// outcome of the duplicate-block case is ambiguous_match (the ErrAmbiguous
// refusal) — the assertion FAILS if it were no_match, a successful match, or
// absent. This is the FUZZBENCH anti-vacuity proof (D-06, Pitfall 6).
func TestAmbiguousRefused(t *testing.T) {
	for _, lang := range corpusLanguages {
		ambiguousID, _ := findAmbiguous(t, lang)
		captured, err := LoadCaptured(testdataDir, lang)
		if err != nil {
			t.Fatalf("LoadCaptured(%q): %v", lang, err)
		}
		o, ok := captured.Outcomes[ambiguousID]
		if !ok {
			t.Fatalf("%s: ambiguous case %q has NO captured outcome (anti-vacuity)", lang, ambiguousID)
		}
		if o.Strategy == OutcomeNoMatch {
			t.Fatalf("%s: ambiguous case %q captured as %q — a refusal must NOT be a no_match (Pitfall 6)",
				lang, ambiguousID, OutcomeNoMatch)
		}
		if o.Strategy != OutcomeAmbiguous || !o.Refused {
			t.Fatalf("%s: ambiguous case %q captured strategy=%q refused=%v, want %q + refused=true",
				lang, ambiguousID, o.Strategy, o.Refused, OutcomeAmbiguous)
		}
		if !refusalGate(captured, ambiguousID) {
			t.Fatalf("%s: refusalGate FAILED on the committed captured outcome (gate should pass)", lang)
		}
	}
}

// TestAmbiguousBites proves the refusal gate is NON-VACUOUS: a deliberately
// mislabeled captured outcome (no_match substituted for the ambiguous case) MUST
// fail the SAME refusalGate the green path passes. A green-path-only gate is
// presumed broken (v1.12 CRITICAL), so this flipped-outcome assertion is the
// must-turn-RED discriminator (mirrors editsim_test TestES_NumstatDiscriminator).
func TestAmbiguousBites(t *testing.T) {
	for _, lang := range corpusLanguages {
		ambiguousID, _ := findAmbiguous(t, lang)
		captured, err := LoadCaptured(testdataDir, lang)
		if err != nil {
			t.Fatalf("LoadCaptured(%q): %v", lang, err)
		}
		// Sanity: the unflipped gate passes.
		if !refusalGate(captured, ambiguousID) {
			t.Fatalf("%s: refusalGate should pass on the committed outcome before flipping", lang)
		}
		// Flip the ambiguous case's outcome to no_match in a COPY.
		flipped := CapturedOutcomes{Language: captured.Language, Outcomes: map[string]CapturedOutcome{}}
		for id, o := range captured.Outcomes {
			flipped.Outcomes[id] = o
		}
		flipped.Outcomes[ambiguousID] = CapturedOutcome{
			ID: ambiguousID, Strategy: OutcomeNoMatch, Refused: false, ErrorKind: "no_match",
		}
		if refusalGate(flipped, ambiguousID) {
			t.Fatalf("%s: refusalGate PASSED on a flipped no_match outcome — the gate is vacuous (does not bite)", lang)
		}
		// A flipped "successful match" outcome must ALSO fail the gate.
		flipped.Outcomes[ambiguousID] = CapturedOutcome{
			ID: ambiguousID, Strategy: StrategyExact, Refused: false,
		}
		if refusalGate(flipped, ambiguousID) {
			t.Fatalf("%s: refusalGate PASSED on a flipped successful-match outcome — the gate is vacuous", lang)
		}
	}
}
