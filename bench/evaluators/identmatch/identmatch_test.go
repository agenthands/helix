package identmatch

import (
	"math"
	"testing"
)

const eps = 1e-9

// CCE IM-EM / IM-F1 (Identifier-Match) reference behavior.
//
// Provenance: CrossCodeEval (Ding et al., NeurIPS 2023; arXiv:2310.11248).
// Identifier-Match tokenizes each completion to its IDENTIFIER set —
// [A-Za-z_][A-Za-z0-9_]* tokens minus a language-agnostic keyword set — and
// reports IM-EM (the identifier SETS are equal) and IM-F1 (harmonic mean of
// precision/recall over the two identifier sets). The expected (em, f1) values
// below are computed by hand from the set definitions (e.g. the partial-overlap
// row has |intersection|=2 over sets of size 3, so precision=recall=2/3 and
// f1=2/3). This hermetic test is the SOLE authoritative proof (no network).
func TestMatch_CCEExamples(t *testing.T) {
	cases := []struct {
		name   string
		pred   string
		gold   string
		wantEM bool
		wantF1 float64
	}{
		// Identical identifiers (keyword `int` filtered out, neither helps nor
		// hurts): ids = {x, compute, y} on both sides ⇒ EM true, F1 1.0.
		{"identical", "int x = compute(y)", "int x = compute(y)", true, 1.0},
		// Partial overlap: pred ids {x,compute,y}, gold ids {x,compute,z};
		// intersection {x,compute} (2), each set size 3 ⇒ P=R=2/3, F1=2/3.
		{"partial_overlap", "x = compute(y)", "x = compute(z)", false, 2.0 / 3.0},
		// Keyword-only on both sides ⇒ both identifier sets empty ⇒ EM true,
		// F1 1.0 (empty-vs-empty convention). Proves keywords are excluded.
		{"keyword_only", "if else for", "while return break", true, 1.0},
		// Both empty strings ⇒ empty identifier sets ⇒ EM true, F1 1.0.
		{"both_empty", "", "", true, 1.0},
		// One side has an identifier, the other is keyword-only (empty set):
		// disjoint ⇒ EM false, F1 0.0.
		{"one_empty_set", "value", "return", false, 0.0},
		// Pred drops one identifier present in gold: pred {a,b}, gold {a,b,c};
		// intersection 2; P=2/2=1, R=2/3 ⇒ F1 = 2*1*(2/3)/(1+2/3) = 0.8.
		{"dropped_identifier", "a + b", "a + b + c", false, 0.8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			em, f1 := Match(tc.pred, tc.gold)
			if em != tc.wantEM {
				t.Fatalf("Match(%q,%q) em = %v, want %v", tc.pred, tc.gold, em, tc.wantEM)
			}
			if math.Abs(f1-tc.wantF1) > eps {
				t.Fatalf("Match(%q,%q) f1 = %v, want %v", tc.pred, tc.gold, f1, tc.wantF1)
			}
		})
	}
}

// Partial-overlap rows assert 0 < f1 < 1 AND em == false (a genuine partial
// match, not a rubber stamp).
func TestMatch_PartialBounds(t *testing.T) {
	em, f1 := Match("x = compute(y)", "x = compute(z)")
	if em {
		t.Fatalf("partial overlap reported em=true, want false")
	}
	if !(f1 > 0.0 && f1 < 1.0) {
		t.Fatalf("partial overlap f1 = %v, want strictly in (0,1)", f1)
	}
}

// Match is total on every edge input (no panic), including empty and unicode.
func TestMatch_TotalNoPanic(t *testing.T) {
	_, _ = Match("", "")
	_, _ = Match("a", "")
	_, _ = Match("", "b")
	_, _ = Match("café := naïve", "café := naïve")
}
