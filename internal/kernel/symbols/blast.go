package symbols

import (
	"context"

	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/semantic/integ"
)

// NodeImpact is one entry in a BlastRadius.PerNode slice — the per-symbol
// confidence + evidence pair the Phase 65 strangler-fig orchestrator surfaces
// on the MCP envelope (INTEG-03 / INTEG-05). The shape mirrors integ.Impact
// so the v1.9 LSP fallback path can synthesize NodeImpact entries from
// DirectRefs / Callers / Implementations and have the same hard-cap
// (capConfidences) and envelope-render helper apply uniformly across the
// semantic / tree_sitter / fallback paths (D-08).
//
// SymbolID is stored as a plain string (not integ.SymbolID) so this type can
// also represent LSP-derived synthetic nodes that have no canonical Phase 59
// EXTRACT-02 SymbolID — the integ.SymbolID conversion happens at the
// orchestrator boundary.
type NodeImpact struct {
	SymbolID   string
	Confidence float64
	Evidence   integ.Evidence
	Refuted    bool
}

// BlastRadius represents the combined impact analysis for a symbol.
type BlastRadius struct {
	DirectRefs      []SymbolLocation
	Callers         []HierarchyNode
	Implementations []SymbolLocation
	AffectedFiles   []string // unique files across all results
	TotalImpact     int      // total unique locations
	// PerNode is the per-symbol confidence + evidence list rendered on the
	// MCP envelope (Phase 65 INTEG-03 / INTEG-05). Synthesized from
	// DirectRefs / Callers / Implementations on the v1.9 LSP path with
	// confidence=1.0 (LSP-derived); the strangler-fig orchestrator then
	// applies capConfidences(0.6) uniformly to enforce the D-08 cap on every
	// non-semantic path.
	PerNode []NodeImpact
}

// AnalyzeBlastRadius combines references, call hierarchy, and implementations
// to estimate the impact of changing a symbol at the given position.
// SYM-09: Comprehensive blast radius analysis with deduplication.
func AnalyzeBlastRadius(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int) (*BlastRadius, error) {
	br := &BlastRadius{}

	// 1. Find all references (include declaration)
	refs, err := FindReferences(ctx, lease, uri, line, col, true)
	if err != nil {
		// Non-fatal: some servers may not support references
		refs = nil
	}
	br.DirectRefs = refs

	// 2. Get incoming call hierarchy (callers)
	callers, err := GetCallHierarchy(ctx, lease, uri, line, col, "incoming")
	if err != nil {
		// Non-fatal: some servers may not support call hierarchy
		callers = nil
	}
	br.Callers = callers

	// 3. Find implementations
	impls, err := FindImplementations(ctx, lease, uri, line, col)
	if err != nil {
		// Non-fatal: some servers may not support implementations
		impls = nil
	}
	br.Implementations = impls

	// 4. Deduplicate and compute affected files
	fileSet := make(map[string]struct{})
	locationSet := make(map[string]struct{})

	addLocations := func(locs []SymbolLocation) {
		for _, loc := range locs {
			key := locationKey(loc)
			if _, exists := locationSet[key]; !exists {
				locationSet[key] = struct{}{}
				fileSet[loc.URI] = struct{}{}
			}
		}
	}

	addLocations(br.DirectRefs)
	addLocations(br.Implementations)

	// Also collect files from hierarchy nodes
	var walkNodes func(nodes []HierarchyNode)
	walkNodes = func(nodes []HierarchyNode) {
		for _, node := range nodes {
			fileSet[node.URI] = struct{}{}
			locationSet[node.URI+":"+node.Name] = struct{}{}
			walkNodes(node.Children)
		}
	}
	walkNodes(br.Callers)

	br.AffectedFiles = make([]string, 0, len(fileSet))
	for f := range fileSet {
		br.AffectedFiles = append(br.AffectedFiles, f)
	}
	br.TotalImpact = len(locationSet)

	// Synthesize PerNode entries from the LSP-derived DirectRefs +
	// Implementations + Callers so the strangler-fig confidence cap +
	// envelope render apply uniformly. LSP-derived → confidence 1.0 by the
	// Phase 62 D-12 ladder; the strangler-fig caller hard-caps to ≤ 0.6 on
	// every non-semantic path (D-08).
	br.PerNode = make([]NodeImpact, 0, len(br.DirectRefs)+len(br.Implementations)+len(br.Callers))
	for _, loc := range br.DirectRefs {
		br.PerNode = append(br.PerNode, NodeImpact{
			SymbolID:   locationKey(loc),
			Confidence: 1.0,
			Evidence: integ.Evidence{
				LSPLocations: []integ.LSPLocation{{
					Path: loc.URI,
					Line: uint32(loc.Range.Start.Line + 1),
					Col:  uint32(loc.Range.Start.Character + 1),
				}},
			},
		})
	}
	for _, loc := range br.Implementations {
		br.PerNode = append(br.PerNode, NodeImpact{
			SymbolID:   locationKey(loc),
			Confidence: 1.0,
			Evidence: integ.Evidence{
				LSPLocations: []integ.LSPLocation{{
					Path: loc.URI,
					Line: uint32(loc.Range.Start.Line + 1),
					Col:  uint32(loc.Range.Start.Character + 1),
				}},
			},
		})
	}
	var walkCallers func(nodes []HierarchyNode)
	walkCallers = func(nodes []HierarchyNode) {
		for _, n := range nodes {
			br.PerNode = append(br.PerNode, NodeImpact{
				SymbolID:   n.URI + ":" + n.Name,
				Confidence: 1.0,
				Evidence: integ.Evidence{
					LSPLocations: []integ.LSPLocation{{Path: n.URI}},
				},
			})
			walkCallers(n.Children)
		}
	}
	walkCallers(br.Callers)

	return br, nil
}

// locationKey generates a unique key for deduplication based on URI and range.
func locationKey(loc SymbolLocation) string {
	return loc.URI + ":" +
		itoa(int(loc.Range.Start.Line)) + ":" +
		itoa(int(loc.Range.Start.Character))
}

// itoa is a minimal int-to-string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	buf := make([]byte, 0, 12)
	for i > 0 {
		buf = append(buf, byte('0'+i%10))
		i /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	// reverse
	for l, r := 0, len(buf)-1; l < r; l, r = l+1, r-1 {
		buf[l], buf[r] = buf[r], buf[l]
	}
	return string(buf)
}
