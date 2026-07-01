package daemon

import (
	"github.com/agenthands/helix/internal/semantic/extract"
)

// varTypeLink pairs a variable / field / parameter symbol with the NAME of
// its declared type. It is the INPUT CONTRACT for Phase 136's reference
// producer: 136 turns each link into a ChainRequest whose RefNodeID is the
// referencing symbol and whose declared type reaches the resolver via
// ChainTokens (Option A co-driver). This phase (135) only PRODUCES the
// association from the raw extracted facts; it does NOT resolve, emit edges,
// or touch the producer.
type varTypeLink struct {
	// RefName is the referencing symbol's bare name (e.g. "p").
	RefName string
	// RefStableKey is the referencing symbol's canonical StableKey, the
	// stable handle 136 resolves to a NodeID.
	RefStableKey extract.StableSymbolKey
	// TypeName is the bare declared-type name (e.g. "Foo"), matching the
	// resolver's nameToNode keys.
	TypeName string
}

// linkVarTypes walks the raw extracted files and, for each variable / field /
// parameter symbol carrying a non-empty in-memory DeclaredType, emits a
// (referencing symbol → declared type name) association.
//
// Determinism (D1 precursor): iteration is strictly slice-order — files in
// input order, symbols in ef.Symbols order — with NO Go-map iteration in the
// emit path, so repeated calls over the same input produce an identical slice.
//
// Anti-vacuity: a symbol with an empty DeclaredType (a C primitive like
// `int`, or any decl with no named type reference) yields NO link. This is
// what distinguishes a real linkage from a stub that fabricates a target.
func linkVarTypes(extracted []*extract.ExtractedFile) []varTypeLink {
	var links []varTypeLink
	for _, ef := range extracted {
		if ef == nil {
			continue
		}
		for i := range ef.Symbols {
			s := ef.Symbols[i]
			if !isVarTypeCarrier(s.Kind) {
				continue
			}
			if s.DeclaredType == "" {
				continue
			}
			links = append(links, varTypeLink{
				RefName:      s.Name,
				RefStableKey: s.StableKey,
				TypeName:     s.DeclaredType,
			})
		}
	}
	return links
}

// isVarTypeCarrier reports whether a symbol kind can carry a declared type
// that participates in var→type linkage: variables, struct/class fields, and
// function parameters.
func isVarTypeCarrier(k extract.SymbolKind) bool {
	switch k {
	case extract.KindVariable, extract.KindField, extract.KindParameter:
		return true
	default:
		return false
	}
}
