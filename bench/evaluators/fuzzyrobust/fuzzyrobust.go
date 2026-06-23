package fuzzyrobust

import "github.com/agenthands/helix/bench/evaluators/editsim"

// CaseScore is the graded result of comparing one drift case's structurally
// EXPECTED strategy (the perturbation tier — D-05) against the CAPTURED outcome
// recorded by the non-leaf harness. It grades strategy SELECTION (match /
// mismatch) AND reports the editsim.ES text similarity — selection and
// similarity are orthogonal (a captured strategy can differ from the expected
// tier while the matched text is still identical to the expected text).
type CaseScore struct {
	// Expected is the structurally-derived expected outcome label (D-05).
	Expected string
	// Outcome is the captured outcome label (one of the five-value vocabulary:
	// exact / whitespace_normalized / indentation_flexible / ambiguous_match /
	// no_match).
	Outcome string
	// StrategyMatch is true iff the captured outcome equals the expected one —
	// the cascade selected the tier (or refusal) the perturbation implies.
	StrategyMatch bool
	// Similarity is editsim.ES(matchedText, expectedText) in [0,1]. For a refusal
	// (ambiguous_match) or no_match there is no matched text, so it is 0.
	Similarity float64
	// Refused is true iff the captured outcome is the ambiguity refusal
	// (ambiguous_match, ErrAmbiguous) — distinct from NoMatch (Pitfall 6).
	Refused bool
	// NoMatch is true iff the captured outcome is no_match (ErrNoMatch) —
	// distinct from Refused (Pitfall 6).
	NoMatch bool
}

// ScoreCase grades one drift case. expectedStrategy is the structurally-derived
// expected outcome (the perturbation tier, or ambiguous_match for the
// duplicate-block case — D-05); capturedStrategy is the observed outcome the
// non-leaf harness recorded. matchedText is the region fuzzy.Match landed on (for
// a successful match); expectedText is the un-drifted base block. The function
// reuses editsim.ES for matched-vs-expected similarity (D-05 — never
// re-implemented) and is TOTAL on every input (mirrors editsim.ES: no panic on
// empty/nil/unicode).
//
// The outcome classification distinguishes a refusal (ambiguous_match) from a
// no_match from a successful match — conflating them is the FUZZBENCH
// anti-vacuity failure (Pitfall 6). For a refusal or a no_match there is no
// matched text, so Similarity is 0 and is not derived from editsim.ES.
func ScoreCase(expectedStrategy, capturedStrategy, matchedText, expectedText string) CaseScore {
	cs := CaseScore{
		Expected:      expectedStrategy,
		Outcome:       capturedStrategy,
		StrategyMatch: expectedStrategy == capturedStrategy,
		Refused:       capturedStrategy == OutcomeAmbiguous,
		NoMatch:       capturedStrategy == OutcomeNoMatch,
	}
	// Similarity only applies to a successful match (one of the four cascade
	// tiers). A refusal / no_match has no matched region.
	if !cs.Refused && !cs.NoMatch {
		cs.Similarity = editsim.ES(matchedText, expectedText)
	}
	return cs
}
