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

	assert.True(t, r.SupportsLanguage("java"))
	assert.True(t, r.SupportsLanguage("c"))
	assert.True(t, r.SupportsLanguage("cpp"))
	assert.True(t, r.SupportsLanguage("c_sharp"))
	assert.True(t, r.SupportsLanguage("ruby"))
	assert.True(t, r.SupportsLanguage("php"))
	assert.True(t, r.SupportsLanguage("javascript"))
	assert.True(t, r.SupportsLanguage("kotlin"))

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

	lang, ok = r.GetLanguage("java")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("c")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("cpp")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("c_sharp")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("ruby")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("php")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("javascript")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("kotlin")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("unknown")
	assert.False(t, ok)
	assert.Nil(t, lang)
}

func TestSupportedLanguages(t *testing.T) {
	r := NewGrammarRegistry()

	langs := r.SupportedLanguages()
	assert.Equal(t, []string{"c", "c_sharp", "cpp", "go", "java", "javascript", "kotlin", "php", "python", "ruby", "rust", "tsx", "typescript"}, langs)
}
