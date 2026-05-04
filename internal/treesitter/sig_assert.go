package treesitter

// Compile-time signature assertion: catches silent drift in the
// GrammarRegistry public API. Phase 59.1 retired the //go:build cgo
// gating; the registry is now CGO=1-only at the build-tag level (the
// binary cannot be built without CGO), so this assertion is the sole
// surviving compile-time signature check on the package.
var _ interface {
	SupportsLanguage(string) bool
	SupportedLanguages() []string
} = (*GrammarRegistry)(nil)
