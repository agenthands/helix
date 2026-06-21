package repobench

import (
	"github.com/agenthands/helix/bench/evaluators/editsim"
	"github.com/agenthands/helix/bench/evaluators/exactmatch"
)

// AccAtK is the RepoBench-R retrieval metric (acc@k; RepoBench paper Liu et al.,
// ICLR 2024, arXiv:2306.03091, confirmed RESEARCH A3). Given the model's ranked
// retrieval ordering over the candidate snippets (ranked holds candidate indices
// best-first) and the gold snippet index (Task.GoldSnippetIndex, an index into
// Context), AccAtK reports whether the gold snippet appears within the top-k of
// the ordering — i.e. within ranked[:min(k, len(ranked))].
//
// Boundaries: k<=0 is a vacuous empty top-k (never a hit); k>=len(ranked) clamps
// to len(ranked) so any present gold is a hit; a gold absent from ranked is never
// a hit. It is total and never panics.
func AccAtK(ranked []int, gold, k int) bool {
	if k <= 0 {
		return false
	}
	if k > len(ranked) {
		k = len(ranked)
	}
	for i := 0; i < k; i++ {
		if ranked[i] == gold {
			return true
		}
	}
	return false
}

// CompletionScore is the RepoBench-C / -P next_line metric. It scores a predicted
// completion against the ground-truth next_line by DELEGATING to the Plan 01
// scorers — exactmatch.EM for Exact Match and editsim.ES for the normalized
// edit similarity (the same CM-ES family RepoBench-C reports). It deliberately
// does NOT re-implement edit distance here: the edit-distance logic lives once,
// in package editsim, and is reused by both CrossCodeEval and RepoBench so the
// metrics are identical across the two completion benchmarks.
//
// It returns (em, es): em is true iff pred exactly equals gold; es is in [0,1]
// (1.0 == identical). RepoBench-P (pipeline = retrieve-then-complete) scores its
// final completion with the SAME (em, es), so this one function serves both -C
// and -P.
func CompletionScore(pred, gold string) (em bool, es float64) {
	return exactmatch.EM(pred, gold), editsim.ES(pred, gold)
}
