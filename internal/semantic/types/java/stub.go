// Package java implements the Phase 62 P05 Java type-resolver stub.
//
// Per D-12 v1: Java has no static type analyzer. The stub is LSP-conditional
// — it short-circuits to confidence=1.0 + validated when Phase 61's LSP
// cascade has produced a RESOLVES_TO edge with source=lsp.* + confidence
// >= 1.0; otherwise it emits 0.20 + unresolved.
//
// A future v1.10.x phase will replace this stub with a full ladder
// resolver. Until then, the stub guarantees that LSP-validated facts
// are surfaced verbatim while preserving the D-12 invariant that no row
// is silently dropped (every ref produces an emit decision).
package java

import (
	"context"
	"strings"

	"github.com/agenthands/helix/internal/semantic/types"
)

// Stub is the Java LSP-conditional type-resolver stub.
type Stub struct {
	store types.EffectiveReader
}

// NewStub constructs a Stub over the given EffectiveReader.
func NewStub(store types.EffectiveReader) *Stub {
	return &Stub{store: store}
}

// ResolveChain short-circuits on Phase 61 LSP-validated edges; otherwise
// emits 0.20 unresolved.
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
				Source:          e.Source, // preserve "lsp.<call>" lineage
			}, nil
		}
	}
	return types.ChainResponse{
		Resolved:        false,
		Confidence:      types.ConfidenceUnknown,
		EvidenceKind:    types.EvidenceUnknown,
		ValidationState: "unresolved",
		Source:          "stub.java",
		Reason:          "java: no LSP fact + no static analyzer in v1",
	}, nil
}

// ResolveSymbol mirrors ResolveChain — short-circuit on LSP, otherwise 0.20.
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
		Source:          "stub.java",
		Reason:          "java: no LSP fact + no static analyzer in v1",
	}, nil
}
