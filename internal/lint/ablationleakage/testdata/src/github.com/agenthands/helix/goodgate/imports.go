// goodgate is the GREEN call-site fixture. It performs the SAME semantic read as
// badgate (`.ExpandFrom(`) but routes it through `integ.ChooseSource` first — the
// gate that renders the read tree-sitter when the semantic index is disabled.
// Because this file syntactically contains an `integ.ChooseSource(` call, the
// narrow-AST analyzer treats the read methods here as gated and MUST stay silent.
// There is NO analysistest expectation directive on any line; analysistest fails
// if a diagnostic is emitted.
package goodgate

import "github.com/agenthands/helix/internal/semantic/integ"

// readGated routes through the ChooseSource gate before reading. The presence of
// the ChooseSource call in this file is the syntactic evidence the analyzer keys
// off to permit the subsequent SemanticLookup read.
func readGated(cfg integ.ConfigGate, lookup integ.SemanticLookup) []string {
	if integ.ChooseSource(cfg, lookup) != "semantic" {
		return nil
	}
	return lookup.ExpandFrom("sym", 2)
}
