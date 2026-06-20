// Stub `integ` package used only by the badgate/goodgate analysistest
// fixtures. It mirrors just enough of the real internal/semantic/integ surface
// for the call-site analyzer to resolve the SemanticLookup read methods and the
// ChooseSource gate by name. The real package depends on the wider workspace
// and is irrelevant to the analyzer's name-based AST check — analysistest only
// needs the imports to type-check.
package integ

// SemanticLookup is the trimmed stand-in for the production interface. Only the
// three data-bearing read methods the analyzer flags are present; the analyzer
// keys off the selector method NAME, not the concrete receiver type.
type SemanticLookup interface {
	RankFiles() []string
	ExpandFrom(sym string, depth int) []string
	ValidateCriticalEdges(edges []string) []string
}

// ConfigGate mirrors the production gate contract. ChooseSource consults it as
// the first priority before any semantic read is permitted.
type ConfigGate interface {
	SemanticIndexEnabled() bool
}

// ChooseSource is the routing gate the call-site analyzer pins. A semantic read
// is only legitimate when it is syntactically routed through this gate.
func ChooseSource(cfg ConfigGate, lookup SemanticLookup) string {
	if cfg == nil || !cfg.SemanticIndexEnabled() {
		return "tree_sitter"
	}
	return "semantic"
}
