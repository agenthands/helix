// Package daemon: v2.12 Phase 136 type-resolver production consumer.
//
// type_resolver_wiring.go owns the batch type-edge pipeline that turns a
// C-family var→type link (Phase 135's linkVarTypes) into a committed
// RESOLVES_TO edge with a real target:
//
//   - batchEffectiveReader — a types.EffectiveReader over the in-memory
//     out.Symbols of the CURRENT batch (NOT the committed store, which is
//     stale at factsFromExtracted time). Tier-1 LSP stays dead in-batch.
//
//   - newIndexedResolver — constructs a per-language resolver carrying the
//     batch typeIndex (via NewResolverWithIndex) so the annotation tier binds
//     a real NodeID target.
//
//   - resolveTypeEdges — the producer + deterministic driver + converter:
//     linkVarTypes → ChainRequests → per-(scope,lang) sorted FixpointResolve
//     → EdgeFacts appended to out.Edges BEFORE the dense EdgeID stamp.
//
// This REPLACES the Phase-62 bootstrap dispatcher + the dead
// SetSemanticGraph/TypeResolver seams (removed in Phase 136 — the per-batch
// dispatcher here is the dispatcher's real production consumer).
package daemon

import (
	"context"
	"sort"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/graph"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/semantic/types"
	typesc "github.com/agenthands/helix/internal/semantic/types/c"
	typescsharp "github.com/agenthands/helix/internal/semantic/types/c_sharp"
	typescpp "github.com/agenthands/helix/internal/semantic/types/cpp"
	typesgo "github.com/agenthands/helix/internal/semantic/types/golang"
	typesjava "github.com/agenthands/helix/internal/semantic/types/java"
	typespython "github.com/agenthands/helix/internal/semantic/types/python"
	typests "github.com/agenthands/helix/internal/semantic/types/typescript"
)

// typeResolveParams carries the type-resolution knobs threaded from
// semanticConfig into the batch driver. Values default to the
// TypeResolutionConfig defaults (8 / 0.45 / false) in loadSemanticConfig.
type typeResolveParams struct {
	maxFixpoint    int
	minConfidence  float64
	emitUnresolved bool
}

// batchEffectiveReader is a types.EffectiveReader backed by the in-memory
// symbols of the current batch (out.Symbols), keyed by NodeID. It is built
// once per batch and is deterministic. QueryEffectiveEdges returns nil — the
// Tier-1 LSP-cascade read stays dead in-batch (uncommitted; L5).
type batchEffectiveReader struct {
	syms map[graph.NodeID]types.SymbolFact
}

func (r *batchEffectiveReader) QueryEffectiveEdges(context.Context, types.EdgeQuery) ([]types.EdgeFact, error) {
	return nil, nil
}

func (r *batchEffectiveReader) QueryEffectiveSymbol(_ context.Context, _ string, n graph.NodeID) (types.SymbolFact, error) {
	if s, ok := r.syms[n]; ok {
		return s, nil
	}
	// Degrade to NodeID-only (Tier-7 D-12 floor) on a miss.
	return types.SymbolFact{NodeID: n}, nil
}

// newIndexedResolver returns the per-language resolver carrying the batch
// typeIndex, or nil for languages without a full resolver (php/ruby/rust/
// kotlin stubs — they carry no typeIndex path and produce no links).
func newIndexedResolver(lang string, reader types.EffectiveReader, idx map[string]graph.NodeID) types.Resolver {
	switch lang {
	case "go":
		return typesgo.NewResolverWithIndex(reader, idx)
	case "typescript", "javascript":
		return typests.NewResolverWithIndex(reader, idx)
	case "python":
		return typespython.NewResolverWithIndex(reader, idx)
	case "java":
		return typesjava.NewResolverWithIndex(reader, idx)
	case "c_sharp":
		return typescsharp.NewResolverWithIndex(reader, idx)
	case "c":
		return typesc.NewResolverWithIndex(reader, idx)
	case "cpp":
		return typescpp.NewResolverWithIndex(reader, idx)
	default:
		return nil
	}
}

// typeEdgeGroupKey groups ChainRequests per (language, scope-file) for the
// fixpoint driver. Iterating SORTED keys — and refs sorted within a group —
// keeps the emit order deterministic (the EdgeID stamp is positional; a
// nondeterministic edge order = non-byte-identical snapshot).
type typeEdgeGroupKey struct {
	lang string
	file string
}

// resolveTypeEdges is the batch type-edge producer + driver + converter. It
// runs at the tail of factsFromExtracted, AFTER dedup + the nameToNode build,
// so out.Symbols is final and nameToNode keys point at definition nodes. It
// appends RESOLVES_TO EdgeFacts to out.Edges (BEFORE the dense EdgeID stamp
// the buildFn applies later).
//
// Determinism: every map is materialized into a sorted slice before it drives
// the emit path — no Go-map iteration order leaks into out.Edges.
func resolveTypeEdges(
	out *semanticstore.Facts,
	repoID string,
	extracted []*extract.ExtractedFile,
	nameToNode map[string]uint64,
	nameCount map[string]int,
	fileIDToPath map[uint64]string,
	params typeResolveParams,
) {
	// 1. typeIndex from nameToNode. Skip ambiguous names (nameCount>1): they
	//    have no unique target, so the annotation tier must NOT fabricate one
	//    (anti-mis-bind — the edge is gated, never bound to an arbitrary node).
	typeIndex := make(map[string]graph.NodeID, len(nameToNode))
	for name, node := range nameToNode {
		if nameCount[name] > 1 {
			continue
		}
		typeIndex[name] = graph.NodeID(node)
	}

	// 2. Batch reader over out.Symbols + a RefStableKey→NodeID map. The
	//    reader carries FilePath (via fileIDToPath) so cross-package langs'
	//    D-13 scope guard behaves; for flat-scope C it is not load-bearing.
	symsByNode := make(map[graph.NodeID]types.SymbolFact, len(out.Symbols))
	stableKeyToNode := make(map[string]uint64, len(out.Symbols))
	for i := range out.Symbols {
		s := out.Symbols[i]
		if s.NodeID == 0 {
			continue
		}
		symsByNode[graph.NodeID(s.NodeID)] = types.SymbolFact{
			NodeID:    graph.NodeID(s.NodeID),
			Language:  s.Language,
			Kind:      s.Kind,
			StableKey: s.StableKey,
			FilePath:  fileIDToPath[s.FileID],
			Signature: s.Signature,
		}
		if s.StableKey != "" {
			if _, exists := stableKeyToNode[s.StableKey]; !exists {
				stableKeyToNode[s.StableKey] = s.NodeID
			}
		}
	}
	reader := &batchEffectiveReader{syms: symsByNode}

	// 3. Producer: linkVarTypes → ChainRequests. Slice-order iteration keeps
	//    the pre-group order deterministic. Skip links whose TypeName is not a
	//    UNIQUE nameToNode key, or whose RefStableKey did not survive dedup.
	links := linkVarTypes(extracted)
	refs := make([]types.ChainRequest, 0, len(links))
	for _, l := range links {
		if _, ok := typeIndex[l.TypeName]; !ok {
			continue
		}
		refNode, ok := stableKeyToNode[extract.CanonicalizeStableSymbolKey(l.RefStableKey)]
		if !ok || refNode == 0 {
			continue
		}
		sym := symsByNode[graph.NodeID(refNode)]
		refs = append(refs, types.ChainRequest{
			RepoID:      repoID,
			Language:    sym.Language,
			FilePath:    sym.FilePath,
			RefNodeID:   graph.NodeID(refNode),
			RefKind:     "RESOLVES_TO",
			ChainTokens: []string{l.TypeName},
		})
	}
	if len(refs) == 0 {
		return
	}

	// 4a. One per-batch dispatcher with an indexed resolver per language present.
	reg := map[string]types.Resolver{}
	for _, r := range refs {
		if _, seen := reg[r.Language]; seen {
			continue
		}
		if res := newIndexedResolver(r.Language, reader, typeIndex); res != nil {
			reg[r.Language] = res
		}
	}
	dispatcher := types.NewDispatcher(reg)

	// 4b. Group by (lang, scope-file); iterate SORTED keys, refs sorted within.
	groups := map[typeEdgeGroupKey][]types.ChainRequest{}
	for _, r := range refs {
		k := typeEdgeGroupKey{lang: r.Language, file: r.FilePath}
		groups[k] = append(groups[k], r)
	}
	keys := make([]typeEdgeGroupKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].lang != keys[j].lang {
			return keys[i].lang < keys[j].lang
		}
		return keys[i].file < keys[j].file
	})

	maxIter := params.maxFixpoint
	if maxIter <= 0 {
		maxIter = 8
	}
	ctx := context.Background()
	for _, k := range keys {
		gr := groups[k]
		sort.Slice(gr, func(i, j int) bool {
			if gr[i].RefNodeID != gr[j].RefNodeID {
				return gr[i].RefNodeID < gr[j].RefNodeID
			}
			return firstChainToken(gr[i]) < firstChainToken(gr[j])
		})
		resps, err := types.FixpointResolve(ctx, dispatcher, gr, maxIter)
		if err != nil {
			// Best-effort: a resolver error degrades this group to no edges
			// rather than failing the whole snapshot build.
			continue
		}
		for i := range resps {
			edge, ok := typeEdgeFromResponse(gr[i], resps[i], symsByNode, params)
			if !ok {
				continue
			}
			out.Edges = append(out.Edges, edge)
		}
	}
}

// firstChainToken returns the request's first chain token ("" when empty) —
// the secondary sort key for within-group determinism.
func firstChainToken(r types.ChainRequest) string {
	if len(r.ChainTokens) > 0 {
		return r.ChainTokens[0]
	}
	return ""
}

// typeEdgeFromResponse converts one resolved ChainResponse into a RESOLVES_TO
// EdgeFact, applying the gates:
//
//   - dst=0 (unreadable target) → dropped (never persist a fabricated/empty
//     endpoint; L4 row-bloat guard).
//   - Confidence < MinConfidenceForEdge → dropped.
//   - Unresolved response → dropped unless EmitUnresolvedEdges is set.
func typeEdgeFromResponse(
	req types.ChainRequest,
	resp types.ChainResponse,
	symsByNode map[graph.NodeID]types.SymbolFact,
	params typeResolveParams,
) (semanticstore.EdgeFact, bool) {
	if resp.Target == 0 {
		return semanticstore.EdgeFact{}, false
	}
	if resp.Confidence < params.minConfidence {
		return semanticstore.EdgeFact{}, false
	}
	if !resp.Resolved && !params.emitUnresolved {
		return semanticstore.EdgeFact{}, false
	}
	srcKind := "symbol"
	if s, ok := symsByNode[req.RefNodeID]; ok && s.Kind != "" {
		srcKind = s.Kind
	}
	source := resp.Source
	if source == "" {
		source = "unknown"
	}
	state := resp.ValidationState
	if !resp.Resolved {
		state = "unresolved"
	}
	return semanticstore.EdgeFact{
		SrcNodeID:       uint64(req.RefNodeID),
		DstNodeID:       uint64(resp.Target),
		EdgeKind:        req.RefKind, // "RESOLVES_TO"
		SrcKind:         srcKind,
		DstKind:         "type",
		Source:          source,
		Weight:          1.0,
		Confidence:      resp.Confidence,
		ValidationState: state,
	}, true
}
