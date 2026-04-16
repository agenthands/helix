package repomap

import (
	"testing"

	"github.com/postfix/serena/internal/treesitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// extractAndRender is a helper that extracts tags from source using TagExtractor,
// then renders them with ElisionRenderer. This tests the end-to-end pipeline.
func extractAndRender(t *testing.T, source []byte, lang string) string {
	t.Helper()
	registry := treesitter.NewGrammarRegistry()
	ext, err := NewTagExtractor(registry)
	require.NoError(t, err)
	defer ext.Close()

	tags, err := ext.Extract(source, "test."+lang, lang)
	require.NoError(t, err)

	renderer := NewElisionRenderer(registry)
	return renderer.RenderFile(source, lang, tags)
}

func TestRenderFile_GoFunction(t *testing.T) {
	source := []byte(`package main

func Hello(name string) string {
    greeting := "Hello, " + name
    return greeting
}
`)
	output := extractAndRender(t, source, "go")

	assert.Contains(t, output, "func Hello(name string) string")
	assert.Contains(t, output, "...")
	assert.NotContains(t, output, `greeting := "Hello, "`)
	assert.NotContains(t, output, "return greeting")
}

func TestRenderFile_GoStruct(t *testing.T) {
	source := []byte(`package main

type Server struct {
    Host string
    Port int
}
`)
	output := extractAndRender(t, source, "go")

	assert.Contains(t, output, "type Server struct")
	// D-14: struct fields should be shown.
	assert.Contains(t, output, "Host string")
	assert.Contains(t, output, "Port int")
}

func TestRenderFile_GoMethod(t *testing.T) {
	source := []byte(`package main

type Server struct{}

func (s *Server) Run() error {
    // long implementation
    return nil
}
`)
	output := extractAndRender(t, source, "go")

	assert.Contains(t, output, "func (s *Server) Run() error")
	assert.Contains(t, output, "...")
	assert.NotContains(t, output, "long implementation")
	assert.NotContains(t, output, "return nil")
}

func TestRenderFile_PythonFunction(t *testing.T) {
	source := []byte(`def calculate(x, y):
    result = x + y
    return result
`)
	output := extractAndRender(t, source, "python")

	assert.Contains(t, output, "def calculate(x, y)")
	assert.Contains(t, output, "...")
	assert.NotContains(t, output, "result = x + y")
	assert.NotContains(t, output, "return result")
}

func TestRenderFile_PythonClass(t *testing.T) {
	source := []byte(`class Calculator:
    def __init__(self, value):
        self.value = value

    def add(self, x):
        return self.value + x
`)
	output := extractAndRender(t, source, "python")

	assert.Contains(t, output, "class Calculator")
	// Method bodies should be elided.
	assert.NotContains(t, output, "self.value = value")
	assert.NotContains(t, output, "return self.value + x")
}

func TestRenderFile_TypeScriptFunction(t *testing.T) {
	source := []byte("function greet(name: string): string {\n    const msg = `Hello ${name}`;\n    return msg;\n}\n")
	output := extractAndRender(t, source, "typescript")

	assert.Contains(t, output, "function greet(name: string): string")
	assert.Contains(t, output, "...")
	assert.NotContains(t, output, "const msg")
	assert.NotContains(t, output, "return msg")
}

func TestRenderFile_RustFunction(t *testing.T) {
	source := []byte(`fn calculate(x: i32, y: i32) -> i32 {
    let result = x + y;
    result
}
`)
	output := extractAndRender(t, source, "rust")

	assert.Contains(t, output, "fn calculate(x: i32, y: i32) -> i32")
	assert.Contains(t, output, "...")
	assert.NotContains(t, output, "let result = x + y")
}

func TestRenderFile_NoDefTags(t *testing.T) {
	registry := treesitter.NewGrammarRegistry()
	renderer := NewElisionRenderer(registry)

	source := []byte(`package main

func main() {
    Hello()
}
`)
	// Only ref tags, no def tags.
	tags := []Tag{
		{Name: "Hello", Kind: TagRef, File: "test.go", Line: 3, Column: 4, StartByte: 30, EndByte: 37},
	}
	output := renderer.RenderFile(source, "go", tags)
	assert.Empty(t, output, "no def tags should produce empty output")

	// Empty tags slice.
	output = renderer.RenderFile(source, "go", nil)
	assert.Empty(t, output, "nil tags should produce empty output")
}

func TestRenderFile_FallbackLineBased(t *testing.T) {
	registry := treesitter.NewGrammarRegistry()
	renderer := NewElisionRenderer(registry)

	source := []byte(`package main

func Hello(name string) string {
    return "hello " + name
}
`)
	// Tags with StartByte == 0 and EndByte == 0 (LSP fallback).
	tags := []Tag{
		{Name: "Hello", Kind: TagDef, File: "test.go", Line: 2, Column: 5, StartByte: 0, EndByte: 0},
	}
	output := renderer.RenderFile(source, "go", tags)

	// Should fall back to line-based rendering showing the line content.
	assert.Contains(t, output, "func Hello(name string) string")
	assert.Contains(t, output, "L3") // 0-indexed line 2 -> display as L3
}

func TestRenderFile_EmptySource(t *testing.T) {
	registry := treesitter.NewGrammarRegistry()
	renderer := NewElisionRenderer(registry)
	output := renderer.RenderFile([]byte{}, "go", []Tag{{Kind: TagDef}})
	assert.Empty(t, output)
}

func TestElideSingle(t *testing.T) {
	source := []byte(`func Hello() {
    return "world"
}`)
	tag := Tag{Name: "Hello", Kind: TagDef, StartByte: 0, EndByte: uint(len(source))}
	// Body starts at the opening brace content, assume body is bytes 14-37.
	result := ElideSingle(source, tag, 14, uint(len(source))-1)
	assert.Contains(t, result, "func Hello()")
	assert.Contains(t, result, "...")
	assert.NotContains(t, result, `"world"`)
}

func TestElideSingle_NoBody(t *testing.T) {
	source := []byte(`type Alias = int`)
	tag := Tag{Name: "Alias", Kind: TagDef, StartByte: 0, EndByte: uint(len(source))}
	result := ElideSingle(source, tag, 0, 0)
	assert.Equal(t, "type Alias = int", result)
}
