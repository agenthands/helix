// badgate is the RED call-site fixture. It reads the semantic store directly by
// calling a SemanticLookup read method (`.ExpandFrom(`) WITHOUT first routing
// through `integ.ChooseSource` — the exact gate-bypass leak D-06's call-site
// check exists to catch. The analyzer MUST flag the direct read. The deliberate
// violation lives ONLY under testdata/ (Pitfall 3), which the go tool ignores,
// so `go vet ./...` / `make vet` on the real tree stay green.
package badgate

import "github.com/agenthands/helix/internal/semantic/integ"

// readDirect performs an un-gated semantic read. Because this file contains no
// `integ.ChooseSource(` call, the analyzer treats every SemanticLookup read
// method call here as an un-gated bypass.
func readDirect(lookup integ.SemanticLookup) []string {
	return lookup.ExpandFrom("sym", 2) // want `must route through integ\.ChooseSource`
}
