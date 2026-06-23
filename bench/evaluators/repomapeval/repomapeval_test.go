package repomapeval

import (
	"math"
	"testing"
)

const eps = 1e-9

func goldSet(ids ...string) map[string]bool {
	s := make(map[string]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}

// TestMetrics_NDCG: hand-computed binary-relevance nDCG@k values.
//
// Convention: gain rel_i ∈ {0,1}, discount 1/log2(i+2) for the 0-indexed loop
// position i; IDCG over min(len(gold),k) ideal items; nDCG=0 when IDCG=0.
func TestMetrics_NDCG(t *testing.T) {
	cases := []struct {
		name   string
		ranked []string
		gold   map[string]bool
		k      int
		want   float64
	}{
		// All gold at the top ⇒ DCG == IDCG ⇒ 1.0.
		{"ideal_all_top", []string{"a", "b", "c"}, goldSet("a", "b"), 10, 1.0},
		// No gold in top-k ⇒ 0.0.
		{"none_in_topk", []string{"x", "y", "z"}, goldSet("a", "b"), 3, 0.0},
		// Mixed: gold at pos0 and pos2. DCG = 1/log2(2) + 1/log2(4) = 1.0 + 0.5 = 1.5.
		// IDCG (2 gold) = 1/log2(2) + 1/log2(3) = 1.0 + 0.6309297535714... = 1.6309297535714.
		// nDCG = 1.5 / 1.6309297535714 = 0.91972149...
		{"mixed_pos0_pos2", []string{"a", "b", "c"}, goldSet("a", "c"), 3,
			(1.0/math.Log2(2) + 1.0/math.Log2(4)) / (1.0/math.Log2(2) + 1.0/math.Log2(3))},
		// Empty gold ⇒ IDCG 0 ⇒ 0.0 (no panic).
		{"empty_gold", []string{"a", "b"}, goldSet(), 10, 0.0},
		// Empty ranking ⇒ 0.0.
		{"empty_ranking", nil, goldSet("a"), 10, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NDCGAtK(tc.ranked, tc.gold, tc.k)
			if math.Abs(got-tc.want) > eps {
				t.Fatalf("NDCGAtK(%v, gold, %d) = %v, want %v", tc.ranked, tc.k, got, tc.want)
			}
			if got < 0.0-eps || got > 1.0+eps {
				t.Fatalf("NDCGAtK = %v out of [0,1]", got)
			}
		})
	}
}

// TestMetrics_Recall: recall@k = hits/len(gold); len(gold)==0 ⇒ 0.0.
func TestMetrics_Recall(t *testing.T) {
	cases := []struct {
		name   string
		ranked []string
		gold   map[string]bool
		k      int
		want   float64
	}{
		{"all_hit", []string{"a", "b", "c"}, goldSet("a", "b"), 10, 1.0},
		{"half_hit", []string{"a", "x", "y", "z"}, goldSet("a", "b"), 10, 0.5},
		{"cutoff_excludes", []string{"x", "y", "a"}, goldSet("a"), 2, 0.0},
		{"empty_gold", []string{"a"}, goldSet(), 10, 0.0},
		{"empty_ranking", nil, goldSet("a"), 10, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RecallAtK(tc.ranked, tc.gold, tc.k)
			if math.Abs(got-tc.want) > eps {
				t.Fatalf("RecallAtK(%v, gold, %d) = %v, want %v", tc.ranked, tc.k, got, tc.want)
			}
		})
	}
}

// TestMetrics_MRR: 1/(rank+1) for the first gold hit (0-indexed rank); 0.0 if
// no gold appears.
func TestMetrics_MRR(t *testing.T) {
	cases := []struct {
		name   string
		ranked []string
		gold   map[string]bool
		want   float64
	}{
		{"first", []string{"a", "b"}, goldSet("a"), 1.0},
		{"second", []string{"x", "a"}, goldSet("a"), 0.5},
		{"third", []string{"x", "y", "a"}, goldSet("a"), 1.0 / 3.0},
		{"none", []string{"x", "y"}, goldSet("a"), 0.0},
		{"empty_ranking", nil, goldSet("a"), 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MRR(tc.ranked, tc.gold)
			if math.Abs(got-tc.want) > eps {
				t.Fatalf("MRR(%v, gold) = %v, want %v", tc.ranked, got, tc.want)
			}
		})
	}
}

// TestMetrics_BudgetFit: fraction of gold surviving into the rendered ID list;
// len(gold)==0 ⇒ 0.0.
func TestMetrics_BudgetFit(t *testing.T) {
	cases := []struct {
		name     string
		rendered []string
		gold     map[string]bool
		want     float64
	}{
		{"all_retained", []string{"a", "b", "c"}, goldSet("a", "b"), 1.0},
		{"half_retained", []string{"a", "x"}, goldSet("a", "b"), 0.5},
		{"none_retained", []string{"x", "y"}, goldSet("a", "b"), 0.0},
		{"dup_rendered_counts_once", []string{"a", "a", "a"}, goldSet("a", "b"), 0.5},
		{"empty_gold", []string{"a"}, goldSet(), 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BudgetFitRatio(tc.rendered, tc.gold)
			if math.Abs(got-tc.want) > eps {
				t.Fatalf("BudgetFitRatio(%v, gold) = %v, want %v", tc.rendered, got, tc.want)
			}
		})
	}
}

// TestMetrics_TotalNoPanic: every metric is total on empty/nil/unicode inputs.
func TestMetrics_TotalNoPanic(t *testing.T) {
	_ = NDCGAtK(nil, nil, 0)
	_ = NDCGAtK(nil, nil, 10)
	_ = NDCGAtK([]string{"日本語:関数"}, goldSet("日本語:関数"), 10)
	_ = RecallAtK(nil, nil, 0)
	_ = RecallAtK([]string{""}, goldSet(""), 10)
	_ = MRR(nil, nil)
	_ = MRR([]string{"日本語:関数"}, goldSet("日本語:関数"))
	_ = BudgetFitRatio(nil, nil)
	_ = BudgetFitRatio([]string{"x"}, nil)
}
