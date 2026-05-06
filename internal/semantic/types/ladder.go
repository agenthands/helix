package types

// SPEC §38.2 7-tier confidence ladder. Every type-resolver edge carries one
// of these values verbatim. Comment-derived edges are CAPPED at
// ConfidenceComment unless independently confirmed by LSP (TYPES-03 +
// CapCommentConfidence).
const (
	ConfidenceLSP         = 1.00 // LSP-confirmed (Phase 61 cascade).
	ConfidenceAnnotation  = 0.90 // Typed declaration / explicit annotation.
	ConfidenceConstructor = 0.80 // Constructor return-type derivable.
	ConfidenceAssignment  = 0.70 // Assignment-flow within fixpoint scope.
	ConfidenceComment     = 0.60 // Doc-comment fallback (capped).
	ConfidenceHeuristic   = 0.45 // Name-shape match (e.g., userRepo → User*).
	ConfidenceUnknown     = 0.20 // No signal — emitted, never silently skipped.
)

// CapCommentConfidence enforces TYPES-03: comment-derived edges NEVER exceed
// 0.60 unless LSP-confirmed. The storage boundary (tx.UpsertEdgesWithMerge,
// P02) provides the second guarantee — if a validated lsp.* row arrives
// later, it deletes any comment row sharing (src, edge_kind) regardless of
// confidence value.
func CapCommentConfidence(c float64, kind EvidenceKind) float64 {
	if kind == EvidenceComment && c > ConfidenceComment {
		return ConfidenceComment
	}
	return c
}

// ConfidenceForEvidence returns the canonical SPEC §38.2 confidence value for
// each EvidenceKind. Per-language resolvers SHOULD populate ChainResponse
// using this helper to keep the ladder values centralised.
func ConfidenceForEvidence(kind EvidenceKind) float64 {
	switch kind {
	case EvidenceLSP:
		return ConfidenceLSP
	case EvidenceAnnotation:
		return ConfidenceAnnotation
	case EvidenceConstructor:
		return ConfidenceConstructor
	case EvidenceAssignment:
		return ConfidenceAssignment
	case EvidenceComment:
		return ConfidenceComment
	case EvidenceHeuristic:
		return ConfidenceHeuristic
	default:
		return ConfidenceUnknown
	}
}
