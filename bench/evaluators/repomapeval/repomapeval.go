// Package repomapeval is the pure stdlib-only RepoMap ranking-quality evaluator.
// It scores a COMMITTED captured ranking (the pre-parsed get-repo-map /
// get-context order, refreshed HELIX_BIN-gated by the regenerator OUTSIDE this
// leaf) against a COMMITTED symbol-level gold corpus authored from task ground
// truth. It is a LEAF: it imports ONLY the Go standard library — no
// internal/repomap, no internal/fuzzy, no bench/runtime, no bench/datasets. That
// boundary is enforced by TestLeafImports (the vet-ablation-leakage analyzer
// scopes its import check to bench/runners/*, NOT bench/evaluators/*), not by the
// build — see CORPUS.md and RESEARCH Pitfall 1.
//
// The four metric functions are binary-relevance IR metrics over a ranked list
// of "file:symbol" IDs and a gold membership set:
//
//   - RecallAtK     — fraction of gold present in the top-k of the ranking.
//   - MRR           — reciprocal rank of the first gold hit (0 if none appear).
//   - NDCGAtK       — headline metric (D-07): position-weighted, so it is most
//     sensitive to ORDERING — the property the reversed/random discriminator
//     bites on. Binary gain rel_i ∈ {0,1}, discount 1/log2(i+2) at 0-indexed
//     position i, IDCG over min(len(gold),k) ideal items, nDCG=0 when IDCG=0.
//   - BudgetFitRatio — gold-retention-within-budget (D-08): fraction of gold that
//     survives into the rendered ID list.
//
// Every function is PURE and TOTAL on every input (empty / nil / unicode) and
// NEVER panics, mirroring the editsim.ES discipline. Any ordering a metric
// depends on uses a total-order comparator so equal-relevance ties never drift
// the committed bytes (Pitfall 4).
package repomapeval

import "math"

// RecallAtK returns the fraction of gold IDs that appear in the top-k of ranked:
// hits / len(gold). It is total: len(gold)==0 returns 0.0 (no division by zero,
// no panic). A non-positive k yields 0.0 (an empty top-k window).
func RecallAtK(ranked []string, gold map[string]bool, k int) float64 {
	if len(gold) == 0 {
		return 0.0
	}
	hits := 0
	for i := 0; i < k && i < len(ranked); i++ {
		if gold[ranked[i]] {
			hits++
		}
	}
	return float64(hits) / float64(len(gold))
}

// MRR returns the reciprocal rank 1/(rank+1) of the FIRST gold hit in ranked
// (0-indexed rank), or 0.0 if no gold ID appears. It is total on every input.
func MRR(ranked []string, gold map[string]bool) float64 {
	for i, id := range ranked {
		if gold[id] {
			return 1.0 / float64(i+1)
		}
	}
	return 0.0
}

// NDCGAtK returns the binary-relevance normalized discounted cumulative gain at
// cutoff k over ranked vs the gold set (the headline metric, D-07).
//
//	DCG  = Σ_{i=0..min(k,len)-1} rel_i / log2(i+2)        rel_i ∈ {0,1}
//	IDCG = Σ_{i=0..min(k,|gold|)-1} 1 / log2(i+2)         (all gold ideal-first)
//	nDCG = DCG / IDCG                                     (0 when IDCG == 0)
//
// It is total: empty ranking, empty gold, or k<=0 all return 0.0 (never panics,
// always within [0,1]).
func NDCGAtK(ranked []string, gold map[string]bool, k int) float64 {
	dcg := 0.0
	for i := 0; i < k && i < len(ranked); i++ {
		if gold[ranked[i]] {
			dcg += 1.0 / math.Log2(float64(i+2))
		}
	}
	ideal := len(gold)
	if ideal > k {
		ideal = k
	}
	idcg := 0.0
	for i := 0; i < ideal; i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0.0
	}
	return dcg / idcg
}

// BudgetFitRatio returns the fraction of gold IDs that survive into rendered (the
// gold-retention-within-budget ratio, D-08). A gold ID counts once regardless of
// how many times it appears in rendered. It is total: len(gold)==0 returns 0.0.
func BudgetFitRatio(rendered []string, gold map[string]bool) float64 {
	if len(gold) == 0 {
		return 0.0
	}
	retained := make(map[string]bool, len(gold))
	for _, id := range rendered {
		if gold[id] {
			retained[id] = true
		}
	}
	return float64(len(retained)) / float64(len(gold))
}
