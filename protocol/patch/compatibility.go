// Package patch provides compatibility shims for cases where the LSP metamodel
// does not accurately reflect real-world language server behavior.
//
// These shims document known discrepancies and provide helper functions
// to work around them. The generated types in protocol/gen/ are produced
// directly from metaModel.json; this package layers behavioral fixes on top.
package patch

// CompatibilityNotes lists known metamodel-to-reality discrepancies.
var CompatibilityNotes = []string{
	"RenameParams.newName: optional in metamodel, required in practice",
	"ServerCapabilities: some fields differ between LS implementations",
}
