// Package rust implements the Phase 62 P05 Rust type-resolver stub.
//
// Per D-12 v1: Rust has no static type analyzer. The stub is LSP-conditional
// — it short-circuits to confidence=1.0 + validated when Phase 61's LSP
// cascade has produced a RESOLVES_TO edge; otherwise emits 0.20 + unresolved.
package rust

import (
	"context"
	"strings"

	"github.com/agenthands/helix/internal/semantic/types"
)

type Stub struct {
	store types.EffectiveReader
}

func NewStub(store types.EffectiveReader) *Stub {
	return &Stub{store: store}
}

func (s *Stub) ResolveChain(ctx context.Context, req types.ChainRequest) (types.ChainResponse, error) {
	edges, err := s.store.QueryEffectiveEdges(ctx, types.EdgeQuery{
		RepoID:    req.RepoID,
		SrcNodeID: req.RefNodeID,
		EdgeKind:  "RESOLVES_TO",
	})
	if err != nil {
		return types.ChainResponse{}, err
	}
	for _, e := range edges {
		if e.Confidence >= 1.0 && strings.HasPrefix(e.Source, "lsp.") {
			return types.ChainResponse{
				Resolved:        true,
				Target:          e.DstNodeID,
				Confidence:      types.ConfidenceLSP,
				EvidenceKind:    types.EvidenceLSP,
				ValidationState: "validated",
				Source:          e.Source,
			}, nil
		}
	}
	return types.ChainResponse{
		Resolved:        false,
		Confidence:      types.ConfidenceUnknown,
		EvidenceKind:    types.EvidenceUnknown,
		ValidationState: "unresolved",
		Source:          "stub.rust",
		Reason:          "rust: no LSP fact + no static analyzer in v1",
	}, nil
}

func (s *Stub) ResolveSymbol(ctx context.Context, req types.SymbolRequest) (types.SymbolResponse, error) {
	edges, err := s.store.QueryEffectiveEdges(ctx, types.EdgeQuery{
		RepoID:    req.RepoID,
		SrcNodeID: req.SymbolNodeID,
		EdgeKind:  "RESOLVES_TO",
	})
	if err != nil {
		return types.SymbolResponse{}, err
	}
	for _, e := range edges {
		if e.Confidence >= 1.0 && strings.HasPrefix(e.Source, "lsp.") {
			return types.SymbolResponse{
				Resolved:        true,
				TypeNodeID:      e.DstNodeID,
				Confidence:      types.ConfidenceLSP,
				EvidenceKind:    types.EvidenceLSP,
				ValidationState: "validated",
				Source:          e.Source,
			}, nil
		}
	}
	return types.SymbolResponse{
		Resolved:        false,
		Confidence:      types.ConfidenceUnknown,
		EvidenceKind:    types.EvidenceUnknown,
		ValidationState: "unresolved",
		Source:          "stub.rust",
		Reason:          "rust: no LSP fact + no static analyzer in v1",
	}, nil
}
