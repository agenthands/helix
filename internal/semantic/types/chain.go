package types

import (
	"context"
	"fmt"
)

// ResolveAccessChain walks an access chain (a.b.c.d) hop by hop, calling the
// per-language Resolver for each token. The walker is bounded at maxDepth
// (TYPES-02 invariant; the production setting is 8 — see
// SemanticIndex.TypeResolution.MaxChainDepth).
//
// Behaviour:
//
//   - Empty tokens → unresolved response, no resolver call.
//   - len(tokens) > maxDepth → unresolved response, no resolver call. Reason
//     cites the depth bound.
//   - Per-hop: invoke lang.ResolveChain with a single-token request whose
//     RefNodeID points at the previous hop's resolved Target. Stop at the
//     first hop where Resolved=false and return that response verbatim.
//   - All hops resolve → return the last response (carries the final
//     Target, Confidence, EvidenceKind, etc.).
func ResolveAccessChain(ctx context.Context, lang Resolver, req ChainRequest, maxDepth int) (ChainResponse, error) {
	if len(req.ChainTokens) == 0 {
		return ChainResponse{
			Resolved:        false,
			Confidence:      ConfidenceUnknown,
			EvidenceKind:    EvidenceUnknown,
			ValidationState: "unresolved",
			Reason:          "empty chain",
		}, nil
	}
	if len(req.ChainTokens) > maxDepth {
		return ChainResponse{
			Resolved:        false,
			Confidence:      ConfidenceUnknown,
			EvidenceKind:    EvidenceUnknown,
			ValidationState: "unresolved",
			Reason: fmt.Sprintf("chain too deep (length=%d > max_chain_depth=%d)",
				len(req.ChainTokens), maxDepth),
		}, nil
	}

	cur := req
	var last ChainResponse
	for i := range req.ChainTokens {
		hopReq := cur
		hopReq.ChainTokens = []string{req.ChainTokens[i]}
		resp, err := lang.ResolveChain(ctx, hopReq)
		if err != nil {
			return ChainResponse{}, err
		}
		last = resp
		if !resp.Resolved {
			break
		}
		// Advance: next hop starts from this hop's resolved target.
		cur.RefNodeID = resp.Target
	}
	return last, nil
}
