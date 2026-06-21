// Package editsim is the pure CrossCodeEval CM-ES (Code-Match Edit Similarity)
// scorer. It is a stdlib-only leaf with NO I/O and NO cross-package reach.
//
// CM-ES (CrossCodeEval; Ding et al., NeurIPS 2023, arXiv:2310.11248) is the
// NORMALIZED character-level edit similarity between the predicted and
// ground-truth completion lines:
//
//	ES(pred, gold) = 1 - levenshtein(pred, gold) / max(len(pred), len(gold))
//
// where len is the RUNE count (not byte length, so multi-byte unicode counts
// once) and levenshtein is the standard insert/delete/substitute edit distance.
// The result is always within [0,1]: 1.0 means identical, 0.0 means maximally
// dissimilar (e.g. two distinct equal-length strings sharing no aligned runes).
//
// IMPORTANT — this is NOT git-numstat line distance. It deliberately does NOT
// reuse bench/evaluators/patch_validator.EditDistancePatch (which is the summed
// added+deleted LINE count from `git diff --numstat`, an integer over a working
// tree — a completely different metric) and does NOT import the unexported
// internal/mcp Levenshtein (tuned for short tool-name typos). The edit distance
// is re-implemented locally below over runes. The TestES_NumstatDiscriminator
// fixture guards against the wrong-metric reuse (RESEARCH Pitfall 2), and the
// package imports stdlib only.
//
// NO normalization is applied to the inputs: ES operates on the raw completion
// strings (consistent with package exactmatch). Empty semantics: ES("","")==1.0
// (both empty short-circuits to a perfect match); when exactly one input is
// empty the ratio is 0.0. ES is total on every input and never panics.
package editsim

// ES returns the CrossCodeEval CM-ES normalized edit similarity in [0,1]:
// 1 - levenshtein(pred,gold)/max(len(pred),len(gold)) over runes. Both-empty is
// 1.0; one-empty is 0.0.
func ES(pred, gold string) float64 {
	pr := []rune(pred)
	gr := []rune(gold)
	if len(pr) == 0 && len(gr) == 0 {
		return 1.0
	}
	maxLen := len(pr)
	if len(gr) > maxLen {
		maxLen = len(gr)
	}
	// maxLen > 0 here (both-empty handled above), so the division is safe.
	d := levenshtein(pr, gr)
	return 1.0 - float64(d)/float64(maxLen)
}

// levenshtein computes the standard insert/delete/substitute edit distance over
// two rune slices using a two-row dynamic program (O(min(m,n)) space). This is a
// local re-implementation — the package intentionally takes no edit-distance
// dependency (see the package doc comment on the wrong-metric pitfall).
func levenshtein(a, b []rune) int {
	m, n := len(a), len(b)
	if m == 0 {
		return n
	}
	if n == 0 {
		return m
	}
	// prev[j] = edit distance between a[:i-1] and b[:j].
	prev := make([]int, n+1)
	curr := make([]int, n+1)
	for j := 0; j <= n; j++ {
		prev[j] = j
	}
	for i := 1; i <= m; i++ {
		curr[0] = i
		for j := 1; j <= n; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = min3(del, ins, sub)
		}
		prev, curr = curr, prev
	}
	return prev[n]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
