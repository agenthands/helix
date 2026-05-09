package retrieval

import "sort"

// FusedCandidate is the per-symbol output of RRF score fusion. The Score
// field is the sum of `WText/(K+rank_text+1) + WGraph/(K+rank_graph+1)`.
// GraphVersion is recorded for the (score desc, graph_version desc,
// symbol_id asc) tiebreak that satisfies the Phase 62 sort-before-iterate
// determinism doctrine.
type FusedCandidate struct {
	SymbolID     string
	Score        float64
	GraphVersion uint64
	TextRank     int // 1-based rank in the text ranking; 0 if absent.
	GraphRank    int // 1-based rank in the graph ranking; 0 if absent.
	MatchedTerms []string
}

// Fuse merges text and graph rankings via weighted Reciprocal Rank Fusion
// (CONTEXT.md D-07).
//
//	score(s) = WText  / (K + rank_text(s)  + 1)
//	         + WGraph / (K + rank_graph(s) + 1)
//
// Both ranks are 0-based at iteration time and become 1-based in the output
// (`TextRank`/`GraphRank` are 0 for symbols absent from that input list).
//
// gvLookup returns the graph_version associated with symbolID; callers may
// pass a closure returning a single per-snapshot gv. The lookup is invoked
// once per output symbol, not per input slot.
//
// Output is sorted (score desc, graph_version desc, symbol_id asc) — the
// Phase 62 sort-before-iterate determinism doctrine. Same inputs always
// produce byte-identical output across runs (asserted by
// TestRRF_Determinism_TieScore over 10 runs).
func Fuse(text []TextRank, graph []GraphRank, cfg RRFConfig, gvLookup func(symbolID string) uint64) []FusedCandidate {
	scores := make(map[string]float64)
	textRankByID := make(map[string]int)
	graphRankByID := make(map[string]int)

	for rank, t := range text {
		// 1-based rank: first entry is rank=1.
		oneBased := rank + 1
		// Capture only the first occurrence of any symbolID — duplicates in
		// the input would otherwise distort the score.
		if _, seen := textRankByID[t.SymbolID]; !seen {
			textRankByID[t.SymbolID] = oneBased
			scores[t.SymbolID] += cfg.WText / float64(cfg.K+oneBased)
		}
	}
	for rank, g := range graph {
		oneBased := rank + 1
		if _, seen := graphRankByID[g.SymbolID]; !seen {
			graphRankByID[g.SymbolID] = oneBased
			scores[g.SymbolID] += cfg.WGraph / float64(cfg.K+oneBased)
		}
	}

	// Materialize the output in symbol_id-ASC order first to give the
	// downstream stable sort a deterministic seed (sort.SliceStable
	// preserves relative order of equal-rank entries).
	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]FusedCandidate, 0, len(ids))
	for _, id := range ids {
		var gv uint64
		if gvLookup != nil {
			gv = gvLookup(id)
		}
		out = append(out, FusedCandidate{
			SymbolID:     id,
			Score:        scores[id],
			GraphVersion: gv,
			TextRank:     textRankByID[id],
			GraphRank:    graphRankByID[id],
		})
	}

	// Three-key tiebreak: score desc → graph_version desc → symbol_id asc.
	// Stable sort preserves the symbol_id-ASC seed for fully-tied entries.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].GraphVersion != out[j].GraphVersion {
			return out[i].GraphVersion > out[j].GraphVersion
		}
		return out[i].SymbolID < out[j].SymbolID
	})
	return out
}
