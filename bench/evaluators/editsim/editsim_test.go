package editsim

import (
	"math"
	"testing"
)

const eps = 1e-9

// CCE CM-ES (Code-Match Edit Similarity) reference behavior.
//
// Provenance: CrossCodeEval (Ding et al., NeurIPS 2023; arXiv:2310.11248).
// CM-ES is the normalized character-level edit similarity between the
// predicted and ground-truth completion lines: 1 - lev(pred,gold)/max(len).
// The expected ratios below are computed by hand from that closed form over
// the Levenshtein edit distance (rune count), e.g. ES("kitten","sitten") has
// lev=1, max=6 ⇒ 1 - 1/6 = 0.8333…. These are the committed CCE-shaped
// reference values; the test is the SOLE authoritative proof (no network).
func TestES_CCEExamples(t *testing.T) {
	cases := []struct {
		name string
		pred string
		gold string
		want float64
	}{
		// Identical ⇒ 1.0 for any non-empty string.
		{"identical", "foo.bar()", "foo.bar()", 1.0},
		// Classic single-substitution: lev=1, max=6 ⇒ 1 - 1/6.
		{"kitten_sitten", "kitten", "sitten", 1.0 - 1.0/6.0},
		// Totally distinct equal-length: lev=3, max=3 ⇒ 0.0 (and ≥ 0).
		{"distinct_equal_len", "abc", "xyz", 0.0},
		// Single substitution near end: lev=1, max=4 ⇒ 0.75.
		{"abcd_abce", "abcd", "abce", 0.75},
		// Both empty ⇒ 1.0 (both-empty short-circuit).
		{"both_empty", "", "", 1.0},
		// One empty ⇒ 0.0.
		{"pred_empty", "", "abcd", 0.0},
		{"gold_empty", "abcd", "", 0.0},
		// Insertion: "abc" vs "abcd" lev=1, max=4 ⇒ 0.75.
		{"insertion", "abc", "abcd", 0.75},
		// Unicode runes (length is in runes, not bytes): "café" vs "cafe"
		// lev=1 (é→e), max=4 ⇒ 0.75. Byte-length would wrongly give max=5.
		{"unicode_runes", "café", "cafe", 0.75},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ES(tc.pred, tc.gold)
			if math.Abs(got-tc.want) > eps {
				t.Fatalf("ES(%q, %q) = %v, want %v", tc.pred, tc.gold, got, tc.want)
			}
			// CCE CM-ES is ALWAYS within [0,1].
			if got < 0.0-eps || got > 1.0+eps {
				t.Fatalf("ES(%q, %q) = %v out of [0,1]", tc.pred, tc.gold, got)
			}
		})
	}
}

// TestES_NumstatDiscriminator (RESEARCH Pitfall 2): a git-numstat line-count
// distance over this single-line change would report an INTEGER (e.g. 2 =
// 1 added + 1 deleted line), or a count >= 1, NOT a character-level ratio in
// (0,1). This fixture asserts ES returns the character-level normalized ratio
// (1 - 1/12 for one substituted char over a 12-rune line), proving editsim is
// NOT reusing patch_validator.EditDistancePatch (git-numstat line distance).
func TestES_NumstatDiscriminator(t *testing.T) {
	pred := "return a + b" // 12 runes
	gold := "return a - b" // differs only at the operator: lev=1
	want := 1.0 - 1.0/12.0 // ≈ 0.91666… — a character-level ratio, not a line count
	got := ES(pred, gold)
	if math.Abs(got-want) > eps {
		t.Fatalf("ES discriminator = %v, want %v (char-level ratio, not a numstat line count)", got, want)
	}
	// A numstat distance would be an integer >= 1 (e.g. 2). Assert ES is a
	// proper fraction strictly inside (0,1): it cannot be a line count.
	if got <= 0.0 || got >= 1.0 {
		t.Fatalf("ES discriminator = %v; must be strictly in (0,1) (a char-level ratio, not an integer line count)", got)
	}
}

// ES is total on every edge input (no panic), including empty and unicode.
func TestES_TotalNoPanic(t *testing.T) {
	_ = ES("", "")
	_ = ES("a", "")
	_ = ES("", "b")
	_ = ES("日本語", "日本x")
}
