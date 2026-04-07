package symbols

import (
	"context"

	"github.com/postfix/serena/internal/kernel/lspool"
)

// BlastRadius represents the combined impact analysis for a symbol.
type BlastRadius struct {
	DirectRefs      []SymbolLocation
	Callers         []HierarchyNode
	Implementations []SymbolLocation
	AffectedFiles   []string // unique files across all results
	TotalImpact     int      // total unique locations
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
