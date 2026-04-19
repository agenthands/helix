package edit

import (
	"testing"

	gen "github.com/postfix/serena/protocol/gen"
	"github.com/postfix/serena/internal/treesitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBody_GoFunction(t *testing.T) {
	source := []byte(`package main

func Hello() {
	fmt.Println("hello")
}

func Add(a, b int) int {
	return a + b
}
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	// Extract body of Hello function.
	start, end, err := be.ExtractBody(source, "go", "Hello", gen.Range{
		Start: gen.Position{Line: 2, Character: 0},
		End:   gen.Position{Line: 4, Character: 1},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, `fmt.Println("hello")`)
	assert.True(t, body[0] == '{', "Go function body should start with '{'")

	// Extract body of Add function.
	start, end, err = be.ExtractBody(source, "go", "Add", gen.Range{
		Start: gen.Position{Line: 6, Character: 0},
		End:   gen.Position{Line: 8, Character: 1},
	})
	require.NoError(t, err)
	body = string(source[start:end])
	assert.Contains(t, body, "return a + b")
}

func TestExtractBody_GoMethod(t *testing.T) {
	source := []byte(`package main

type Server struct{}

func (s *Server) Start() {
	s.running = true
}
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	start, end, err := be.ExtractBody(source, "go", "Start", gen.Range{
		Start: gen.Position{Line: 4, Character: 0},
		End:   gen.Position{Line: 6, Character: 1},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, "s.running = true")
}

func TestExtractBody_PythonFunction(t *testing.T) {
	source := []byte(`def greet(name):
    print(f"Hello, {name}")
    return name

def add(a, b):
    return a + b
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	start, end, err := be.ExtractBody(source, "python", "greet", gen.Range{
		Start: gen.Position{Line: 0, Character: 0},
		End:   gen.Position{Line: 2, Character: 15},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, "print(f\"Hello, {name}\")")
	assert.Contains(t, body, "return name")
}

func TestExtractBody_TypeScriptFunction(t *testing.T) {
	source := []byte(`function greet(name: string): string {
  console.log("hello " + name);
  return name;
}

function add(a: number, b: number): number {
  return a + b;
}
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	start, end, err := be.ExtractBody(source, "typescript", "greet", gen.Range{
		Start: gen.Position{Line: 0, Character: 0},
		End:   gen.Position{Line: 3, Character: 1},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, "console.log")
	assert.Contains(t, body, "return name")
}

func TestExtractBody_RustFunction(t *testing.T) {
	source := []byte(`fn greet(name: &str) -> String {
    println!("Hello, {}", name);
    name.to_string()
}

fn add(a: i32, b: i32) -> i32 {
    a + b
}
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	start, end, err := be.ExtractBody(source, "rust", "greet", gen.Range{
		Start: gen.Position{Line: 0, Character: 0},
		End:   gen.Position{Line: 3, Character: 1},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, "println!")
	assert.Contains(t, body, "name.to_string()")
}

func TestExtractBody_JavaMethod(t *testing.T) {
	source := []byte(`public class Calculator {
    public int add(int a, int b) {
        return a + b;
    }
}
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	start, end, err := be.ExtractBody(source, "java", "add", gen.Range{
		Start: gen.Position{Line: 1, Character: 4},
		End:   gen.Position{Line: 3, Character: 5},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, "return a + b")
}

func TestExtractBody_CFunction(t *testing.T) {
	source := []byte(`int add(int a, int b) {
    return a + b;
}
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	start, end, err := be.ExtractBody(source, "c", "add", gen.Range{
		Start: gen.Position{Line: 0, Character: 0},
		End:   gen.Position{Line: 2, Character: 1},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, "return a + b")
}

func TestExtractBody_CppFunction(t *testing.T) {
	source := []byte(`int compute(int x) {
    return x * 2;
}
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	start, end, err := be.ExtractBody(source, "cpp", "compute", gen.Range{
		Start: gen.Position{Line: 0, Character: 0},
		End:   gen.Position{Line: 2, Character: 1},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, "return x * 2")
}

func TestExtractBody_CSharpMethod(t *testing.T) {
	source := []byte(`class Service {
    void Start() {
        Console.WriteLine("started");
    }
}
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	start, end, err := be.ExtractBody(source, "c_sharp", "Start", gen.Range{
		Start: gen.Position{Line: 1, Character: 4},
		End:   gen.Position{Line: 3, Character: 5},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, "Console.WriteLine")
}

func TestExtractBody_RubyMethod(t *testing.T) {
	source := []byte(`class Greeter
  def hello
    puts "hello"
  end
end
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	start, end, err := be.ExtractBody(source, "ruby", "hello", gen.Range{
		Start: gen.Position{Line: 1, Character: 2},
		End:   gen.Position{Line: 3, Character: 5},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, `puts "hello"`)
}

func TestExtractBody_PhpFunction(t *testing.T) {
	source := []byte(`<?php
function greet() {
    echo "hello";
}
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	start, end, err := be.ExtractBody(source, "php", "greet", gen.Range{
		Start: gen.Position{Line: 1, Character: 0},
		End:   gen.Position{Line: 3, Character: 1},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, `echo "hello"`)
}

func TestExtractBody_JavaScriptFunction(t *testing.T) {
	source := []byte(`function greet() {
  console.log("hello");
}
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	start, end, err := be.ExtractBody(source, "javascript", "greet", gen.Range{
		Start: gen.Position{Line: 0, Character: 0},
		End:   gen.Position{Line: 2, Character: 1},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, "console.log")
}

func TestExtractBody_KotlinFunction(t *testing.T) {
	source := []byte(`fun greet() {
    println("hello")
}
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	start, end, err := be.ExtractBody(source, "kotlin", "greet", gen.Range{
		Start: gen.Position{Line: 0, Character: 0},
		End:   gen.Position{Line: 2, Character: 1},
	})
	require.NoError(t, err)
	body := string(source[start:end])
	assert.Contains(t, body, `println("hello")`)
}

func TestExtractBody_SymbolNotFound(t *testing.T) {
	source := []byte(`package main

func Hello() {
	fmt.Println("hello")
}
`)
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	_, _, err := be.ExtractBody(source, "go", "NonExistent", gen.Range{
		Start: gen.Position{Line: 2, Character: 0},
		End:   gen.Position{Line: 4, Character: 1},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExtractBody_UnsupportedLanguage(t *testing.T) {
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	_, _, err := be.ExtractBody([]byte("code"), "haskell", "main", gen.Range{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported language")
}

func TestSupportsLanguage(t *testing.T) {
	be := NewBodyExtractor(treesitter.NewGrammarRegistry())

	assert.True(t, be.SupportsLanguage("go"))
	assert.True(t, be.SupportsLanguage("python"))
	assert.True(t, be.SupportsLanguage("typescript"))
	assert.True(t, be.SupportsLanguage("rust"))
	assert.True(t, be.SupportsLanguage("java"))
	assert.True(t, be.SupportsLanguage("c"))
	assert.True(t, be.SupportsLanguage("cpp"))
	assert.True(t, be.SupportsLanguage("c_sharp"))
	assert.True(t, be.SupportsLanguage("ruby"))
	assert.True(t, be.SupportsLanguage("php"))
	assert.True(t, be.SupportsLanguage("javascript"))
	assert.True(t, be.SupportsLanguage("kotlin"))
	assert.False(t, be.SupportsLanguage("haskell"))
	assert.False(t, be.SupportsLanguage(""))
}
