// Package treesitter provides a shared grammar registry for tree-sitter languages.
// Both the edit package (BodyExtractor) and repomap package (TagExtractor) consume
// this registry to avoid duplicating grammar initialization.
package treesitter

import (
	"sort"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go" //nolint:importmismatch
)

// GrammarRegistry holds tree-sitter language grammars keyed by language name.
// Thread-safe for concurrent reads.
type GrammarRegistry struct {
	mu        sync.RWMutex
	languages map[string]*tree_sitter.Language
}

// NewGrammarRegistry creates a GrammarRegistry with Go, Python, TypeScript, TSX, and Rust grammars.
func NewGrammarRegistry() *GrammarRegistry {
	r := &GrammarRegistry{
		languages: make(map[string]*tree_sitter.Language),
	}

	r.languages["go"] = tree_sitter.NewLanguage(tree_sitter_go.Language())
	r.languages["python"] = tree_sitter.NewLanguage(tree_sitter_python.Language())
	r.languages["typescript"] = tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())
	r.languages["tsx"] = tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTSX())
	r.languages["rust"] = tree_sitter.NewLanguage(tree_sitter_rust.Language())

	return r
}

// GetLanguage returns the tree-sitter language for the given language name.
// Returns (nil, false) if the language is not registered.
func (r *GrammarRegistry) GetLanguage(lang string) (*tree_sitter.Language, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	l, ok := r.languages[lang]
	return l, ok
}

// SupportsLanguage returns true if the registry has a grammar for the given language.
func (r *GrammarRegistry) SupportsLanguage(lang string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.languages[lang]
	return ok
}

// SupportedLanguages returns a sorted slice of all registered language names.
func (r *GrammarRegistry) SupportedLanguages() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	langs := make([]string, 0, len(r.languages))
	for k := range r.languages {
		langs = append(langs, k)
	}
	sort.Strings(langs)
	return langs
}
