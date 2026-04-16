package treesitter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGrammarRegistry(t *testing.T) {
	r := NewGrammarRegistry()

	assert.True(t, r.SupportsLanguage("go"))
	assert.True(t, r.SupportsLanguage("python"))
	assert.True(t, r.SupportsLanguage("typescript"))
	assert.True(t, r.SupportsLanguage("tsx"))
	assert.True(t, r.SupportsLanguage("rust"))

	assert.False(t, r.SupportsLanguage("java"))
	assert.False(t, r.SupportsLanguage("unknown"))
	assert.False(t, r.SupportsLanguage(""))
}

func TestGetLanguage(t *testing.T) {
	r := NewGrammarRegistry()

	lang, ok := r.GetLanguage("go")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("python")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("typescript")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("tsx")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("rust")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("unknown")
	assert.False(t, ok)
	assert.Nil(t, lang)
}

func TestSupportedLanguages(t *testing.T) {
	r := NewGrammarRegistry()

	langs := r.SupportedLanguages()
	assert.Equal(t, []string{"go", "python", "rust", "tsx", "typescript"}, langs)
}
