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

	// Wave 1 languages (per D-02)
	tree_sitter_kotlin "github.com/tree-sitter-grammars/tree-sitter-kotlin/bindings/go"
	tree_sitter_csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
	tree_sitter_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
	tree_sitter_cpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	tree_sitter_ruby "github.com/tree-sitter/tree-sitter-ruby/bindings/go"

	// Wave 2a languages
	tree_sitter_bash "github.com/tree-sitter/tree-sitter-bash/bindings/go"
	tree_sitter_haskell "github.com/tree-sitter/tree-sitter-haskell/bindings/go"
	tree_sitter_julia "github.com/tree-sitter/tree-sitter-julia/bindings/go"
	tree_sitter_ocaml "github.com/tree-sitter/tree-sitter-ocaml/bindings/go"
	tree_sitter_scala "github.com/tree-sitter/tree-sitter-scala/bindings/go"

	// Wave 2b languages
	tree_sitter_hcl "github.com/tree-sitter-grammars/tree-sitter-hcl/bindings/go"
	tree_sitter_lua "github.com/tree-sitter-grammars/tree-sitter-lua/bindings/go"
	tree_sitter_zig "github.com/tree-sitter-grammars/tree-sitter-zig/bindings/go"

	// Wave 2b gap closure: local vendored bindings (upstream Go bindings broken)
	tree_sitter_r_local "github.com/postfix/serena/internal/treesitter/bindings/r"
	tree_sitter_swift_local "github.com/postfix/serena/internal/treesitter/bindings/swift"
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

	// Wave 1 languages (per D-02)
	r.languages["java"] = tree_sitter.NewLanguage(tree_sitter_java.Language())
	r.languages["c"] = tree_sitter.NewLanguage(tree_sitter_c.Language())
	r.languages["cpp"] = tree_sitter.NewLanguage(tree_sitter_cpp.Language())
	r.languages["c_sharp"] = tree_sitter.NewLanguage(tree_sitter_csharp.Language())
	r.languages["ruby"] = tree_sitter.NewLanguage(tree_sitter_ruby.Language())
	r.languages["php"] = tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())
	r.languages["javascript"] = tree_sitter.NewLanguage(tree_sitter_javascript.Language())
	r.languages["kotlin"] = tree_sitter.NewLanguage(tree_sitter_kotlin.Language())

	// Wave 2a languages
	r.languages["scala"] = tree_sitter.NewLanguage(tree_sitter_scala.Language())
	r.languages["bash"] = tree_sitter.NewLanguage(tree_sitter_bash.Language())
	r.languages["haskell"] = tree_sitter.NewLanguage(tree_sitter_haskell.Language())
	r.languages["julia"] = tree_sitter.NewLanguage(tree_sitter_julia.Language())
	r.languages["ocaml"] = tree_sitter.NewLanguage(tree_sitter_ocaml.LanguageOCaml())

	// Wave 2b languages
	r.languages["lua"] = tree_sitter.NewLanguage(tree_sitter_lua.Language())
	r.languages["zig"] = tree_sitter.NewLanguage(tree_sitter_zig.Language())
	r.languages["hcl"] = tree_sitter.NewLanguage(tree_sitter_hcl.Language())

	// Wave 2b gap closure: local vendored bindings.
	// CGO-only: under CGO_ENABLED=0, Language() returns nil from the binding_nocgo.go
	// stubs and we skip registration so a release-build binary supports 21 languages
	// instead of panicking. See .planning/phases/51-packaging-goreleaser/deferred-items.md
	// DEF-51-01 (resolved by Plan 51-03).
	if ptr := tree_sitter_r_local.Language(); ptr != nil {
		r.languages["r"] = tree_sitter.NewLanguage(ptr)
	}
	if ptr := tree_sitter_swift_local.Language(); ptr != nil {
		r.languages["swift"] = tree_sitter.NewLanguage(ptr)
	}

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
