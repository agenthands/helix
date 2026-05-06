package types

import (
	"context"

	semanticstore "github.com/agenthands/helix/internal/semantic/store"
)

// EmitTx is the narrow seam over the storage merge boundary. The daemon
// wraps a *semanticstore.OverlayTx; tests stub it. EmitEdges routes EVERY
// row through UpsertEdgesWithMerge so the D-14 two-phase predicate (P02)
// applies (LSP wins; comment edges silently upgraded).
type EmitTx interface {
	UpsertEdgesWithMerge(ctx context.Context, edges []semanticstore.EdgeRow) error
}

// EmitEdges turns ChainResponses into RESOLVES_TO/CALLS/USES_TYPE EdgeRows
// and writes them via tx.UpsertEdgesWithMerge.
//
// Invariants:
//
//   - TYPES-03: every EdgeRow's Confidence is run through CapCommentConfidence
//     so comment-derived rows never exceed 0.60 in flight.
//
//   - TYPES-04: every Resolved=false response yields an EdgeRow with
//     ValidationState="unresolved" — never "validated", regardless of what
//     the per-language resolver wrote.
//
//   - Empty input is a no-op (no UpsertEdgesWithMerge call).
func EmitEdges(ctx context.Context, tx EmitTx, repoID string, refs []ChainRequest, responses []ChainResponse) error {
	if len(responses) == 0 || tx == nil {
		return nil
	}
	rows := make([]semanticstore.EdgeRow, 0, len(responses))
	for i, resp := range responses {
		ref := ChainRequest{}
		if i < len(refs) {
			ref = refs[i]
		}
		// TYPES-03 cap.
		conf := CapCommentConfidence(resp.Confidence, resp.EvidenceKind)
		// TYPES-04 enforcement: any unresolved response must carry the
		// "unresolved" state, no exceptions.
		state := resp.ValidationState
		if !resp.Resolved {
			state = "unresolved"
		}
		// Source fallback: per the dispatcher's unknown-language path, an
		// unresolved response may have an empty Source; populate a stable
		// closed-enum value so downstream label cardinality stays bounded.
		source := resp.Source
		if source == "" {
			source = sourceFallback(resp.EvidenceKind)
		}
		rows = append(rows, semanticstore.EdgeRow{
			SrcNodeID:       uint64(ref.RefNodeID),
			DstNodeID:       uint64(resp.Target),
			EdgeKind:        ref.RefKind,
			Source:          source,
			Confidence:      conf,
			Weight:          1.0,
			ValidationState: state,
		})
	}
	return tx.UpsertEdgesWithMerge(ctx, rows)
}

// sourceFallback returns a stable closed-enum source string for responses
// that did not populate Source. The values match the SemanticTypesResolution
// label allowlist (P02 metric helper).
func sourceFallback(kind EvidenceKind) string {
	switch kind {
	case EvidenceLSP:
		return "lsp.unknown"
	case EvidenceAnnotation:
		return "annotation"
	case EvidenceConstructor:
		return "constructor"
	case EvidenceAssignment:
		return "assignment"
	case EvidenceComment:
		return "comment.unknown"
	case EvidenceHeuristic:
		return "heuristic"
	default:
		return "unknown"
	}
}
