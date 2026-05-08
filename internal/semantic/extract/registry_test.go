package extract

import (
	"context"
	"strings"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/treesitter"
)

// fakeProvider is a minimal Provider for registry-construction tests.
// It does NOT exercise extraction; the registry only indexes by language.
// The Extract method exists solely to satisfy the D-06-widened Provider
// interface — registry-construction tests never invoke it.
type fakeProvider struct {
	lang string
}

func (f *fakeProvider) Language() string                          { return f.lang }
func (f *fakeProvider) Extensions() []string                      { return nil }
func (f *fakeProvider) TreeSitterLanguage() *tree_sitter.Language { return nil }
func (f *fakeProvider) Queries() string                           { return "" }
func (f *fakeProvider) SupportsLSPEnrichment() bool               { return false }
func (f *fakeProvider) Extract(ctx context.Context, source []byte, file SourceFile) (*ExtractedFile, error) {
	return nil, nil
}

// TestRegistry_NilGrammarPanics asserts NewExtractorRegistry panics when
// passed a nil GrammarRegistry — wiring bugs must surface at daemon start,
// not at runtime (D-02 mitigation T-59-02-01).
func TestRegistry_NilGrammarPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on nil GrammarRegistry, got nil")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value not a string: %T %v", r, r)
		}
		if !strings.Contains(msg, "nil GrammarRegistry") {
			t.Errorf("panic message %q does not contain 'nil GrammarRegistry'", msg)
		}
	}()
	_ = NewExtractorRegistry(nil)
}

// TestRegistry_DuplicateLanguagePanics asserts the registry refuses two
// providers claiming the same Language() — duplicate registration is a
// wiring bug (mitigation T-59-02-01).
func TestRegistry_DuplicateLanguagePanics(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate provider, got nil")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value not a string: %T %v", r, r)
		}
		if !strings.Contains(msg, "duplicate provider for language") {
			t.Errorf("panic message %q does not contain 'duplicate provider for language'", msg)
		}
	}()
	_ = NewExtractorRegistry(grammars,
		&fakeProvider{lang: "go"},
		&fakeProvider{lang: "go"},
	)
}

// TestRegistry_ProviderLookup asserts providers are addressable by
// Language() and missing languages return (_, false).
func TestRegistry_ProviderLookup(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	r := NewExtractorRegistry(grammars,
		&fakeProvider{lang: "go"},
	)
	got, ok := r.Provider("go")
	if !ok {
		t.Fatal("expected (provider, true) for registered language")
	}
	if got.Language() != "go" {
		t.Errorf("Provider.Language() = %q, want %q", got.Language(), "go")
	}
	if _, ok := r.Provider("rust"); ok {
		t.Error("expected (_, false) for unregistered language")
	}
}
