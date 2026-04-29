package treesitter

// Compile-time signature assertions: these interface satisfaction checks
// run in BOTH cgo and !cgo builds and catch silent drift between
// registry_cgo.go and registry_nocgo.go. If a cgo signature changes,
// update the assertion AND the !cgo stub together — `go build` will fail
// loudly in either build mode rather than only in CI's nocgo job.
//
// GetLanguage is intentionally omitted: its return type
// *tree_sitter.Language lives in the cgo-only upstream module, and all
// callers of GetLanguage are themselves gated //go:build cgo.
var _ interface {
	SupportsLanguage(string) bool
	SupportedLanguages() []string
} = (*GrammarRegistry)(nil)
