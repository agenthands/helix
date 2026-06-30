package extract

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/classifier"
	"github.com/agenthands/helix/internal/semantic/dataflow"
	"github.com/agenthands/helix/internal/semantic/minhash"
	"github.com/agenthands/helix/internal/semantic/relatedidx"
)

// FingerprintBody computes the MinHash near-clone signature and the
// ASTProfile structural-profile vector for a symbol body, and stamps them
// onto sf. It is the single shared entry point every per-language provider
// calls so the SIMILAR_TO (MinHash) and DATA_FLOWS (ASTProfile) edge inputs
// are computed identically across all 11 languages.
//
// MUST be called inside provider.Extract while the tree-sitter tree is still
// alive — the returned Signature / ASTProfile are value copies safe to carry
// on the fact after the tree closes, but the *tree_sitter.Node argument is
// not.
//
// Gating:
//   - body == nil → no-op (sf.MinHash / sf.Profile / sf.ContextVec stay nil).
//   - sf.MinHash is set only when the body has >= minhash.MinNodes leaf
//     tokens (ComputeSignature's own threshold); tiny bodies yield no
//     near-clone signal and stay nil so SIMILAR_TO never links trivia.
//   - sf.Profile is set whenever a body exists — the structural profile is
//     meaningful even for short bodies, and DATA_FLOWS bucketing tolerates
//     small functions.
//   - sf.ContextVec is set only when the body has >= relatedidx.MinTokens
//     vocabulary tokens; trivia stays nil so SEMANTICALLY_RELATED never links
//     on too little vocabulary. MinHash captures SHAPE, ContextVec captures
//     VOCABULARY — the two are computed from the same body but are orthogonal
//     signals (see relatedidx package doc).
//
// Callers gate on symbol kind (function/method) before calling; container
// kinds (struct/class/interface) have no behavioral body worth profiling.
func FingerprintBody(sf *SymbolFact, body *tree_sitter.Node, source []byte) {
	if sf == nil || body == nil {
		return
	}
	if sig, ok := minhash.ComputeSignature(body, source); ok {
		sf.MinHash = sig
	}
	prof := classifier.ComputeProfile(body, source)
	sf.Profile = &prof
	if vec, ok := relatedidx.ComputeVector(body, source); ok {
		sf.ContextVec = vec
	}
	// Case-1 param->target flow summary (DATA_FLOWS). Unlike MinHash/Profile/
	// ContextVec, this is a data-dependence signal, not a similarity one; nil
	// when the body has no params or no param reaches a target (anti-vacuity).
	sf.FlowSummary = dataflow.AnalyzeFlow(body, source)
}

// IsFingerprintableKind reports whether a symbol kind has a behavioral body
// worth fingerprinting for SIMILAR_TO / DATA_FLOWS. Only functions and
// methods carry a statement body; container and leaf kinds do not.
func IsFingerprintableKind(k SymbolKind) bool {
	return k == KindFunction || k == KindMethod
}
