package repomap

import (
	"testing"

	"github.com/postfix/serena/internal/treesitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestExtractor(t *testing.T) *TagExtractor {
	t.Helper()
	registry := treesitter.NewGrammarRegistry()
	ext, err := NewTagExtractor(registry)
	require.NoError(t, err)
	t.Cleanup(func() { ext.Close() })
	return ext
}

func TestExtract_GoFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`package main

func Hello() {
	fmt.Println("hello")
}
`)
	tags, err := ext.Extract(source, "test.go", "go")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	require.NotEmpty(t, defs, "should have at least one def tag")

	found := findTagByName(defs, "Hello")
	require.NotNil(t, found, "should find Hello def")
	assert.Equal(t, TagDef, found.Kind)
	assert.Equal(t, 2, found.Line) // 0-indexed line
	assert.Equal(t, "test.go", found.File)
}

func TestExtract_GoMethod(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`package main

type Server struct{}

func (s *Server) Run() {
	s.running = true
}
`)
	tags, err := ext.Extract(source, "test.go", "go")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Server.Run")
	require.NotNil(t, found, "should find qualified method name Server.Run")
	assert.Equal(t, TagDef, found.Kind)
}

func TestExtract_GoType(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`package main

type Config struct {
	Name string
}
`)
	tags, err := ext.Extract(source, "test.go", "go")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Config")
	require.NotNil(t, found, "should find Config type def")
	assert.Equal(t, TagDef, found.Kind)
}

func TestExtract_GoReferences(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`package main

func main() {
	Hello()
	s.Run()
}
`)
	tags, err := ext.Extract(source, "test.go", "go")
	require.NoError(t, err)

	refs := filterTags(tags, TagRef)
	require.NotEmpty(t, refs, "should have reference tags")

	found := findTagByName(refs, "Hello")
	require.NotNil(t, found, "should find Hello ref")
	assert.Equal(t, TagRef, found.Kind)
}

func TestExtract_PythonFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`def hello():
    print("hello")

def add(a, b):
    return a + b
`)
	tags, err := ext.Extract(source, "test.py", "python")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "hello")
	require.NotNil(t, found, "should find hello def")
	assert.Equal(t, 0, found.Line)

	found = findTagByName(defs, "add")
	require.NotNil(t, found, "should find add def")
}

func TestExtract_PythonClass(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`class Foo:
    x = 1
`)
	tags, err := ext.Extract(source, "test.py", "python")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Foo")
	require.NotNil(t, found, "should find Foo class def")
	assert.Equal(t, TagDef, found.Kind)
}

func TestExtract_PythonMethod(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`class Foo:
    def hello(self):
        print("hello")
`)
	tags, err := ext.Extract(source, "test.py", "python")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Foo.hello")
	require.NotNil(t, found, "should find qualified method name Foo.hello")
}

func TestExtract_TypeScriptFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`function greet() {
  console.log("hello");
}
`)
	tags, err := ext.Extract(source, "test.ts", "typescript")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "greet")
	require.NotNil(t, found, "should find greet def")
	assert.Equal(t, TagDef, found.Kind)
}

func TestExtract_TypeScriptClass(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`class App {
  start() {}
}
`)
	tags, err := ext.Extract(source, "test.ts", "typescript")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "App")
	require.NotNil(t, found, "should find App class def")

	found = findTagByName(defs, "App.start")
	require.NotNil(t, found, "should find qualified method name App.start")
}

func TestExtract_RustFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`fn main() {
    println!("hello");
}
`)
	tags, err := ext.Extract(source, "test.rs", "rust")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "main")
	require.NotNil(t, found, "should find main def")
	assert.Equal(t, TagDef, found.Kind)
}

func TestExtract_RustStruct(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`struct Point {
    x: f64,
    y: f64,
}
`)
	tags, err := ext.Extract(source, "test.rs", "rust")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Point")
	require.NotNil(t, found, "should find Point struct def")
}

func TestExtract_UnsupportedLanguage(t *testing.T) {
	ext := newTestExtractor(t)
	_, err := ext.Extract([]byte("code"), "test.java", "java")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tag query for language")
}

// --- helpers ---

func filterTags(tags []Tag, kind TagKind) []Tag {
	var result []Tag
	for _, t := range tags {
		if t.Kind == kind {
			result = append(result, t)
		}
	}
	return result
}

func findTagByName(tags []Tag, name string) *Tag {
	for i := range tags {
		if tags[i].Name == name {
			return &tags[i]
		}
	}
	return nil
}
