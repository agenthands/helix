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

	assert.True(t, r.SupportsLanguage("scala"))
	assert.True(t, r.SupportsLanguage("bash"))
	assert.True(t, r.SupportsLanguage("haskell"))
	assert.True(t, r.SupportsLanguage("julia"))
	assert.True(t, r.SupportsLanguage("ocaml"))

	assert.True(t, r.SupportsLanguage("lua"))
	assert.True(t, r.SupportsLanguage("zig"))
	assert.True(t, r.SupportsLanguage("hcl"))

	assert.True(t, r.SupportsLanguage("r"))
	assert.True(t, r.SupportsLanguage("swift"))

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

	lang, ok = r.GetLanguage("scala")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("bash")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("haskell")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("julia")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("ocaml")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("lua")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("zig")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("hcl")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("r")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("swift")
	require.True(t, ok)
	assert.NotNil(t, lang)

	lang, ok = r.GetLanguage("unknown")
	assert.False(t, ok)
	assert.Nil(t, lang)
}

func TestSupportedLanguages(t *testing.T) {
	r := NewGrammarRegistry()

	langs := r.SupportedLanguages()
	assert.Equal(t, []string{"bash", "c", "c_sharp", "cpp", "go", "haskell", "hcl", "java", "javascript", "julia", "kotlin", "lua", "ocaml", "php", "python", "r", "ruby", "rust", "scala", "swift", "tsx", "typescript", "zig"}, langs)
}
