//go:build !cgo

// Package treesitter — !cgo stub. Under CGO_ENABLED=0 the daemon refuses to start
// (see internal/daemon/daemon.go), so these methods are unreachable at runtime.
// They exist only to keep packages that reference *treesitter.GrammarRegistry
// compilable. Note: GetLanguage is intentionally NOT declared here because its
// return type *tree_sitter.Language lives in the CGO-only upstream module; all
// callers of GetLanguage are themselves gated //go:build cgo.
package treesitter

// GrammarRegistry under !cgo carries no grammars.
type GrammarRegistry struct{}

// NewGrammarRegistry returns an empty registry. The daemon refuses to start
// before this is exercised at runtime.
func NewGrammarRegistry() *GrammarRegistry { return &GrammarRegistry{} }

// SupportsLanguage always returns false under !cgo.
func (*GrammarRegistry) SupportsLanguage(string) bool { return false }

// SupportedLanguages returns nil under !cgo.
func (*GrammarRegistry) SupportedLanguages() []string { return nil }
