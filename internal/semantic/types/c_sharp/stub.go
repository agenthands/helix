// Package c_sharp implements the Phase 62 P05 C# type-resolver stub.
//
// Per D-12 v1: C# has no static type analyzer. The stub is LSP-conditional
// — it short-circuits to confidence=1.0 + validated when Phase 61's LSP
// cascade has produced a RESOLVES_TO edge with source=lsp.* + confidence
// >= 1.0; otherwise it emits 0.20 + unresolved.
//
// A future phase will replace this stub with a full ladder resolver.
// Until then, the stub guarantees D-12: no row is silently dropped.
package c_sharp

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
		Source:          "stub.c_sharp",
		Reason:          "c_sharp: no LSP fact + no static analyzer in v1",
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
		Source:          "stub.c_sharp",
		Reason:          "c_sharp: no LSP fact + no static analyzer in v1",
	}, nil
}
