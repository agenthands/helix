// Package retrieval implements the bleve-backed full-text retrieval engine
// and weighted RRF score-fusion primitive consumed by the
// `get_semantic_context` MCP tool (Phase 64 P64-07).
//
// File ownership: this package is ADDED by P64-07. It NEVER edits
// internal/semantic/store/effective_graph.go (that file is owned by P64-02 —
// IterateCommittedSymbols + LatestCommittedSnapshot are consumed via the
// local StoreReader interface seam declared in recovery.go).
//
// The package keeps its own TextRank/GraphRank types so the retrieval engine
// can be import-cycle-free with internal/skill/semantic; the daemon-side
// adapter (P64-08) translates `[]retrieval.TextRank` into the skill-package
// types when implementing the skill-side `RetrievalAccessor` interface.
package retrieval

// RRFConfig parameterizes weighted Reciprocal Rank Fusion.
//
// Per CONTEXT.md D-07:
//   - K is the rank-decay constant (60 by default; standard RRF value).
//   - WText / WGraph are scalar weights on the text and graph rankings.
//   - Defaults are equal (1.0 / 1.0) — classic unweighted RRF.
//
// Weights are Go-internal constants; no `semantic_index.*` config keys are
// exposed this phase. Phase 67's evaluation harness is the natural place to
// gather signal on whether weights should become tunable.
type RRFConfig struct {
	K      int     // Rank-decay constant. 60 default.
	WText  float64 // Scalar weight on text rankings. 1.0 default.
	WGraph float64 // Scalar weight on graph rankings. 1.0 default.
}

// DefaultRRFConfig returns the production default RRF configuration:
// K=60, WText=1.0, WGraph=1.0 (classic equal-weight RRF). Keeps the
// retrieval boundary stable while leaving room for Phase 67 tuning.
func DefaultRRFConfig() RRFConfig {
	return RRFConfig{K: 60, WText: 1.0, WGraph: 1.0}
}

// Token-budget + corpus shape constants. Surfaced from this package so the
// skill-side handler (tools_context.go) and the corpus mapper (corpus.go)
// share the same numeric guarantees.
const (
	// DefaultContextBudget is the default max_tokens budget when the caller
	// omits the field on get_semantic_context.
	DefaultContextBudget = 2048
	// MinTokenBudget is the lower clamp on max_tokens. A budget below this
	// is unusable in practice (cannot fit a single candidate's evidence).
	MinTokenBudget = 64
	// MaxTokenBudget is the upper clamp on max_tokens. Caps DoS via
	// unbounded request budgets (T-64-07-02).
	MaxTokenBudget = 32768
	// CommentWindowLines is the half-width of the source-comment window
	// captured for each symbol's bleve doc (D-06: ~5 lines above + below
	// the declaration anchor line).
	CommentWindowLines = 5
	// TopEdgesPerCandidate is the cap on per-candidate ContextEvidence.TopEdges
	// (CONTEXT.md "evidence shape"). Keeps the response envelope sane under
	// token-budget pressure.
	TopEdgesPerCandidate = 5
)
