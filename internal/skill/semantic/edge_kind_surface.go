package semantic

// edge_kind_surface.go declares the closed lowercase MCP-surface enum used by
// Phase 71 read tools (explain_symbol_deep, find_related_symbols,
// validate_graph_edge) to classify edges, plus a one-way mapper from the
// extractor/resolver internal kind strings (CALLS / RESOLVES_TO / USES_TYPE /
// CONTAINS / IMPORTS / DEFINED_IN / IMPLEMENTS / EXTENDS / REFERENCES) into
// the surface enum.
//
// Decision references:
//   - 71-CONTEXT.md D3 — closed MCP-surface edge enum + internal_kind audit
//     field; decouples MCP contract from internal graph refactors.
//   - 71-RESEARCH.md Pitfall 2 — RESOLVES_TO maps to has_type (NOT uses_type);
//     every type-resolver pass writes RESOLVES_TO meaning "this symbol has
//     this type". Confirmed by reading the four resolver files.
//   - 71-RESEARCH.md Pitfall 4 — emit a bounded-label metric
//     `mcp_edge_kind_surface_other_total{internal_kind=…}` when an unmapped
//     internal kind collapses to EdgeKindOther, so the mapping table can
//     grow as new internal kinds are added. NOTE: the obs.Metrics seam is not
//     yet wired into this package; metric instrumentation is non-blocking
//     for Plan 71-02 and is carved out for the wave that wires
//     `internal/skill/semantic` against the metrics façade. TODO(Pitfall 4):
//     emit `mcp_edge_kind_surface_other_total{internal_kind=internal}` from
//     the default branch below once the seam is available.
//
// The mapping is one-way (internal → surface); the reverse is intentionally
// not provided because no Phase 71/72 tool accepts surface-enum input.

// EdgeKindSurface is the closed lowercase MCP-surface enum classifying edges
// in tool responses. Source: 71-CONTEXT.md D3.
type EdgeKindSurface string

const (
	// EdgeKindCalls — caller→callee invocation edge.
	EdgeKindCalls EdgeKindSurface = "calls"
	// EdgeKindReferences — generic name reference (also covers IMPORTS).
	EdgeKindReferences EdgeKindSurface = "references"
	// EdgeKindImplements — class/struct implements interface.
	EdgeKindImplements EdgeKindSurface = "implements"
	// EdgeKindExtends — class extends superclass / interface extends interface.
	EdgeKindExtends EdgeKindSurface = "extends"
	// EdgeKindHasType — symbol's resolved type edge (RESOLVES_TO). Pitfall 2:
	// the type-resolver writes RESOLVES_TO meaning "this symbol HAS this type",
	// not "uses this type".
	EdgeKindHasType EdgeKindSurface = "has_type"
	// EdgeKindUsesType — symbol uses a type without owning it (e.g.,
	// parameter or local-variable type usage where the symbol is not the
	// declared owner of the type).
	EdgeKindUsesType EdgeKindSurface = "uses_type"
	// EdgeKindContains — file contains symbol / parent contains child (also
	// covers DEFINED_IN which some extractors emit as the reverse synonym).
	EdgeKindContains EdgeKindSurface = "contains"
	// EdgeKindOther — sink for any internal kind not in the mapping table.
	// Drives Pitfall 4 bounded-label metric once instrumentation is wired.
	EdgeKindOther EdgeKindSurface = "other"
)

// MapInternalKind translates an internal extractor/resolver edge-kind string
// to its closed MCP-surface enum value. Unmapped strings (including the empty
// string) collapse to EdgeKindOther without panic.
//
// One-way mapping by design: see file header.
func MapInternalKind(internal string) EdgeKindSurface {
	switch internal {
	case "CALLS":
		return EdgeKindCalls
	case "REFERENCES":
		return EdgeKindReferences
	case "IMPLEMENTS":
		return EdgeKindImplements
	case "EXTENDS":
		return EdgeKindExtends
	case "RESOLVES_TO":
		// Pitfall 2: type-resolver "this symbol has this type" — NOT uses_type.
		return EdgeKindHasType
	case "USES_TYPE":
		return EdgeKindUsesType
	case "CONTAINS":
		return EdgeKindContains
	case "DEFINED_IN":
		// Reverse-synonym some extractors emit for CONTAINS; collapses to the
		// same surface bucket per 71-RESEARCH.md lines 346-347.
		return EdgeKindContains
	case "IMPORTS":
		// Per 71-RESEARCH.md line 347 — import edges surface as references on
		// the MCP side; "imports" is not its own bucket in v1.10.
		return EdgeKindReferences
	default:
		// TODO(Pitfall 4): emit bounded-label metric
		// `mcp_edge_kind_surface_other_total{internal_kind=internal}` once
		// the obs.Metrics seam is wired into internal/skill/semantic.
		return EdgeKindOther
	}
}
