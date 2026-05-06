package types

import (
	"context"

	"github.com/agenthands/helix/internal/semantic/graph"
)

// EvidenceKind tracks what produced a confidence value (SPEC §38.2 ladder).
// The string forms also flow through telemetry as the closed-enum
// confidence_tier label in SemanticTypesResolutionInc (P02 helper).
type EvidenceKind string

const (
	EvidenceLSP         EvidenceKind = "lsp"         // 1.00
	EvidenceAnnotation  EvidenceKind = "annotation"  // 0.90
	EvidenceConstructor EvidenceKind = "constructor" // 0.80
	EvidenceAssignment  EvidenceKind = "assignment"  // 0.70
	EvidenceComment     EvidenceKind = "comment"     // 0.60 (capped)
	EvidenceHeuristic   EvidenceKind = "heuristic"   // 0.45
	EvidenceUnknown     EvidenceKind = "unknown"     // 0.20
)

// Resolver is the per-language type-resolver contract. The Dispatcher routes
// requests by ChainRequest.Language; per-language implementations live in
// subpackages (golang/, typescript/, python/, java/, php/, ruby/).
type Resolver interface {
	ResolveChain(ctx context.Context, req ChainRequest) (ChainResponse, error)
	ResolveSymbol(ctx context.Context, req SymbolRequest) (SymbolResponse, error)
}

// ChainRequest is one access-chain to resolve, e.g., a.b.c.d.
//
// RefKind is the kind of edge to emit when this chain resolves
// (RESOLVES_TO | CALLS | USES_TYPE). EmitEdges threads it verbatim into the
// EdgeRow.EdgeKind column; per-language resolvers do not own the routing
// decision.
//
// FilePath is the path of the file containing the reference. The fixpoint
// uses it for D-13 cross-package guards.
type ChainRequest struct {
	RepoID      string
	Language    string
	FilePath    string
	RefNodeID   graph.NodeID
	RefKind     string   // "RESOLVES_TO" | "CALLS" | "USES_TYPE"
	ChainTokens []string // ["a", "b", "c", "d"] for a.b.c.d
}

// ChainResponse is the resolver's verdict for one access-chain.
//
//   - Resolved=true + Confidence=1.00 + ValidationState="validated": LSP-
//     confirmed (or LSP-cascade short-circuit for Java stub).
//   - Resolved=true + Confidence ≤ 0.60: validated but lower-tier
//     (annotation/constructor/assignment/comment/heuristic).
//   - Resolved=false: non-converged. ValidationState MUST be "unresolved"
//     (TYPES-04 invariant; FixpointResolve + EmitEdges enforce this).
type ChainResponse struct {
	Resolved        bool
	Target          graph.NodeID
	Confidence      float64
	EvidenceKind    EvidenceKind
	ValidationState string // "validated" | "unresolved"
	Source          string // "lsp.<call>" | "comment.<kind>" | "annotation" | ...
	Reason          string // free-form; populated when Resolved=false
}

// SymbolRequest asks the resolver to determine a symbol's type (e.g., "what
// is the static type of variable X").
type SymbolRequest struct {
	RepoID       string
	Language     string
	FilePath     string
	SymbolNodeID graph.NodeID
}

// SymbolResponse is the symbol-type verdict. Same TYPES-04 invariant: when
// Resolved=false the ValidationState MUST be "unresolved".
type SymbolResponse struct {
	Resolved        bool
	TypeNodeID      graph.NodeID
	Confidence      float64
	EvidenceKind    EvidenceKind
	ValidationState string
	Source          string
	Reason          string
}

// EffectiveReader is the narrow seam over store reads. Tests stub it; the
// daemon wraps *store.Store via a small adapter (see internal/daemon).
//
// QueryEffectiveEdges returns the LIVE edge facts for (repo, src, kind);
// resolvers consume Phase 61 LSP-cascade edges via this read.
//
// QueryEffectiveSymbol returns the LIVE symbol fact for (repo, node); used
// by per-language resolvers for annotation / constructor / comment tier
// inputs.
type EffectiveReader interface {
	QueryEffectiveEdges(ctx context.Context, q EdgeQuery) ([]EdgeFact, error)
	QueryEffectiveSymbol(ctx context.Context, repoID string, nodeID graph.NodeID) (SymbolFact, error)
}

// EdgeQuery filters QueryEffectiveEdges results by source and kind.
type EdgeQuery struct {
	RepoID    string
	SrcNodeID graph.NodeID
	EdgeKind  string // empty = any
}

// EdgeFact is one row returned by QueryEffectiveEdges. It mirrors
// graph.GraphEdge but lives here so the types package does not depend on
// the graph package's internal shape (the seam is narrow on purpose).
type EdgeFact struct {
	SrcNodeID       graph.NodeID
	DstNodeID       graph.NodeID
	EdgeKind        string
	Source          string
	Confidence      float64
	ValidationState string
}

// SymbolFact is the narrow projection of a symbol row used by per-language
// resolvers. Only fields the ladder consumes are surfaced.
type SymbolFact struct {
	NodeID     graph.NodeID
	Language   string
	Kind       string
	StableKey  string
	FilePath   string
	Signature  string // for annotation tier
	DocComment string // for comment-fallback tier
}

// Dispatcher implements Resolver by routing on ChainRequest.Language.
//
//   - Known language → delegate to the registered Resolver.
//   - "javascript" with no explicit entry → fall through to the
//     "typescript"-registered resolver (D-11; TS and JS share the same
//     resolver).
//   - Unknown language → emit an unresolved 0.20 response with a
//     "no resolver for language" reason. NEVER returns an error — D-12
//     invariant: the dispatcher always emits a row, never silently drops.
type Dispatcher struct {
	byLang map[string]Resolver
}

// NewDispatcher constructs a Dispatcher over the given language→resolver
// registry. The registry is consumed by reference; callers SHOULD NOT
// mutate it after construction.
func NewDispatcher(reg map[string]Resolver) *Dispatcher {
	if reg == nil {
		reg = map[string]Resolver{}
	}
	return &Dispatcher{byLang: reg}
}

// resolverFor returns the registered Resolver for a language, applying the
// javascript→typescript alias rule (D-11).
func (d *Dispatcher) resolverFor(lang string) (Resolver, bool) {
	if r, ok := d.byLang[lang]; ok {
		return r, true
	}
	if lang == "javascript" {
		if r, ok := d.byLang["typescript"]; ok {
			return r, true
		}
	}
	return nil, false
}

// ResolveChain dispatches to the per-language Resolver. Unknown languages
// produce an unresolved 0.20 response without erroring.
func (d *Dispatcher) ResolveChain(ctx context.Context, req ChainRequest) (ChainResponse, error) {
	r, ok := d.resolverFor(req.Language)
	if !ok {
		return ChainResponse{
			Resolved:        false,
			Confidence:      ConfidenceUnknown,
			EvidenceKind:    EvidenceUnknown,
			ValidationState: "unresolved",
			Reason:          "no resolver for language: " + req.Language,
		}, nil
	}
	return r.ResolveChain(ctx, req)
}

// ResolveSymbol dispatches the symbol-type request. Same unknown-language
// behaviour as ResolveChain.
func (d *Dispatcher) ResolveSymbol(ctx context.Context, req SymbolRequest) (SymbolResponse, error) {
	r, ok := d.resolverFor(req.Language)
	if !ok {
		return SymbolResponse{
			Resolved:        false,
			Confidence:      ConfidenceUnknown,
			EvidenceKind:    EvidenceUnknown,
			ValidationState: "unresolved",
			Reason:          "no resolver for language: " + req.Language,
		}, nil
	}
	return r.ResolveSymbol(ctx, req)
}
