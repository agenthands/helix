package retrieval

// rrf.go — STUB (RED gate). Real implementation lands in Task 1 GREEN.

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

// Fuse merges text and graph rankings via weighted Reciprocal Rank Fusion.
// gvLookup returns the graph_version associated with symbolID for tiebreak;
// callers may pass a closure returning a single per-snapshot gv.
//
// STUB body: panics until Task 1 GREEN supplies the real implementation.
func Fuse(text []TextRank, graph []GraphRank, cfg RRFConfig, gvLookup func(symbolID string) uint64) []FusedCandidate {
	panic("retrieval.Fuse: not implemented (RED gate — Task 1 GREEN fills this in)")
}
