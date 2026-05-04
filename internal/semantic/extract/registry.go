package extract

import (
	"fmt"

	"github.com/agenthands/helix/internal/treesitter"
)

// Registry is the daemon-owned catalogue of per-language Provider
// implementations. Keyed by Provider.Language(). Constructed exactly once
// in internal/daemon/daemon.go (Phase 59 P05) — never via init() (D-02).
type Registry struct {
	grammars  *treesitter.GrammarRegistry
	providers map[string]Provider // keyed by Provider.Language()
}

// NewExtractorRegistry constructs a Registry from explicitly-passed
// providers. The daemon owns the singleton *treesitter.GrammarRegistry
// (BUG-04 invariant) and injects it here.
//
// Panics on:
//   - nil GrammarRegistry — wiring bug (T-59-02-01 mitigation).
//   - duplicate Provider.Language() — wiring bug (T-59-02-01 mitigation).
//
// Both are start-time conditions, never runtime; surfacing as panics
// makes the wiring bug fail-fast at daemon bootstrap.
func NewExtractorRegistry(grammars *treesitter.GrammarRegistry, providers ...Provider) *Registry {
	if grammars == nil {
		panic("extract.NewExtractorRegistry: nil GrammarRegistry")
	}
	r := &Registry{
		grammars:  grammars,
		providers: make(map[string]Provider, len(providers)),
	}
	for _, p := range providers {
		if _, dup := r.providers[p.Language()]; dup {
			panic(fmt.Sprintf("extract.NewExtractorRegistry: duplicate provider for language %q", p.Language()))
		}
		r.providers[p.Language()] = p
	}
	return r
}

// Provider returns the registered Provider for the given language and
// true if registered, nil and false otherwise.
func (r *Registry) Provider(lang string) (Provider, bool) {
	p, ok := r.providers[lang]
	return p, ok
}

// Grammars returns the daemon-injected *treesitter.GrammarRegistry the
// Registry was constructed with. Provider implementations call this to
// resolve their tree-sitter language pointer instead of constructing
// their own grammar registry (BUG-04 invariant).
func (r *Registry) Grammars() *treesitter.GrammarRegistry {
	return r.grammars
}

// Languages returns the registered language identifiers. Order is not
// guaranteed; callers that need stable ordering must sort.
func (r *Registry) Languages() []string {
	out := make([]string, 0, len(r.providers))
	for lang := range r.providers {
		out = append(out, lang)
	}
	return out
}
