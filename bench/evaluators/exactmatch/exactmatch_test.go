package exactmatch

import "testing"

// CCE CM-EM (Code-Match Exact-Match) reference behavior.
//
// Provenance: CrossCodeEval (Ding et al., NeurIPS 2023; arXiv:2310.11248).
// CM-EM is defined as exact string equality between the predicted completion
// and the ground-truth completion line. The fixture rows below are
// paper-shaped single-line code completions exercising the exact-equality
// contract; they are committed values, not fabricated metric outputs (EM is a
// boolean equality so the expected value is mechanically the equality itself).
func TestEM_CCEExamples(t *testing.T) {
	cases := []struct {
		name string
		pred string
		gold string
		want bool
	}{
		// Identical completion ⇒ exact match.
		{"identical_call", "foo.bar()", "foo.bar()", true},
		// One-character difference ⇒ not an exact match.
		{"one_char_diff", "foo.bar()", "foo.baz()", false},
		// Whitespace difference is a difference (no trimming/normalization;
		// EM operates on the raw completion strings — see doc comment).
		{"trailing_space_diff", "x = 1", "x = 1 ", false},
		// Both empty ⇒ exact match (the empty completion equals the empty
		// ground truth). EM-on-empty semantics: documented as true.
		{"both_empty", "", "", true},
		// One empty ⇒ not a match.
		{"pred_empty", "", "y = compute(z)", false},
		// Unicode-bearing completion equality (totality on unicode input).
		{"unicode_equal", "café := \"naïve\"", "café := \"naïve\"", true},
		{"unicode_diff", "café := \"naïve\"", "café := \"naive\"", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EM(tc.pred, tc.gold)
			if got != tc.want {
				t.Fatalf("EM(%q, %q) = %v, want %v", tc.pred, tc.gold, got, tc.want)
			}
		})
	}
}

// EM is total on every edge input (no panic), including empty and unicode.
func TestEM_TotalNoPanic(t *testing.T) {
	_ = EM("", "")
	_ = EM("a", "")
	_ = EM("", "b")
	_ = EM("日本語", "日本語")
}
