package extract

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Provider mirrors SPEC §13.3 LanguageProvider — the surface a per-language
// extraction provider exposes to the daemon-owned Registry.
//
// Phase 59 Wave 1 freezes the lookup-side surface (Language, Extensions,
// TreeSitterLanguage, Queries, SupportsLSPEnrichment). Per-language helper
// signatures (ImportResolver, ScopeBuilder, SymbolNormalizer,
// ReferenceClassifier) are intentionally NOT here yet — they are finalized
// in Phase 59 P04 when the per-language providers land. Defining them
// before they are needed risks lock-in to incorrect shapes.
type Provider interface {
	// Language returns the canonical language identifier (e.g. "go",
	// "typescript", "python"). Used as the Registry key. MUST be stable
	// across daemon restarts.
	Language() string

	// Extensions lists the file extensions claimed by this provider
	// (e.g. ".go" or ".ts"). Used by the scheduler to dispatch source
	// files to the right provider when language is not pre-classified.
	Extensions() []string

	// TreeSitterLanguage returns the tree-sitter language pointer the
	// provider parses with. The pointer is owned by the daemon-injected
	// *treesitter.GrammarRegistry singleton (BUG-04 invariant); the
	// provider does NOT construct or cache its own grammar registry.
	TreeSitterLanguage() *tree_sitter.Language

	// Queries returns the raw embedded queries.scm text for this
	// language. Compilation to *tree_sitter.Query is done once at
	// provider construction time and cached on the concrete struct
	// (Pitfall #2 in 59-RESEARCH.md).
	Queries() string

	// SupportsLSPEnrichment indicates whether Phase 61's enrichment
	// worker may attempt to merge this language's facts with LSP
	// callbacks. Languages whose LSP coverage is unstable can return
	// false to opt out.
	SupportsLSPEnrichment() bool
}
