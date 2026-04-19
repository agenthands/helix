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

func TestExtract_JavaClass(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`public class Calculator {
    public int add(int a, int b) {
        return a + b;
    }

    public int subtract(int a, int b) {
        return a - b;
    }
}
`)
	tags, err := ext.Extract(source, "test.java", "java")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Calculator")
	require.NotNil(t, found, "should find Calculator class def")
	assert.Equal(t, TagDef, found.Kind)

	found = findTagByName(defs, "Calculator.add")
	require.NotNil(t, found, "should find qualified method name Calculator.add")

	found = findTagByName(defs, "Calculator.subtract")
	require.NotNil(t, found, "should find qualified method name Calculator.subtract")
}

func TestExtract_CFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`struct Point {
    int x;
    int y;
};

int add(int a, int b) {
    return a + b;
}

typedef unsigned long size_t;
`)
	tags, err := ext.Extract(source, "test.c", "c")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Point")
	require.NotNil(t, found, "should find Point struct def")

	found = findTagByName(defs, "add")
	require.NotNil(t, found, "should find add function def")
}

func TestExtract_CppClass(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`class Shape {
public:
    int area();
};

int compute(int x) {
    return x * 2;
}
`)
	tags, err := ext.Extract(source, "test.cpp", "cpp")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Shape")
	require.NotNil(t, found, "should find Shape class def")

	found = findTagByName(defs, "compute")
	require.NotNil(t, found, "should find compute function def")
}

func TestExtract_CSharpMethod(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`class Service {
    void Start() {
        Console.WriteLine("started");
    }

    void Stop() {
        Console.WriteLine("stopped");
    }
}
`)
	tags, err := ext.Extract(source, "test.cs", "c_sharp")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Service")
	require.NotNil(t, found, "should find Service class def")

	found = findTagByName(defs, "Service.Start")
	require.NotNil(t, found, "should find qualified method name Service.Start")

	found = findTagByName(defs, "Service.Stop")
	require.NotNil(t, found, "should find qualified method name Service.Stop")
}

func TestExtract_RubyMethod(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`class Greeter
  def hello
    puts "hello"
  end

  def goodbye
    puts "goodbye"
  end
end
`)
	tags, err := ext.Extract(source, "test.rb", "ruby")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Greeter")
	require.NotNil(t, found, "should find Greeter class def")

	found = findTagByName(defs, "Greeter.hello")
	require.NotNil(t, found, "should find qualified method name Greeter.hello")
}

func TestExtract_PhpFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`<?php
class User {
    public function getName() {
        return $this->name;
    }
}

function greet() {
    echo "hello";
}
`)
	tags, err := ext.Extract(source, "test.php", "php")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "User")
	require.NotNil(t, found, "should find User class def")

	found = findTagByName(defs, "greet")
	require.NotNil(t, found, "should find greet function def")
}

func TestExtract_JavaScriptFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`class App {
  start() {
    console.log("started");
  }
}

function greet() {
  console.log("hello");
}

const add = (a, b) => a + b;
`)
	tags, err := ext.Extract(source, "test.js", "javascript")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "App")
	require.NotNil(t, found, "should find App class def")

	found = findTagByName(defs, "App.start")
	require.NotNil(t, found, "should find qualified method name App.start")

	found = findTagByName(defs, "greet")
	require.NotNil(t, found, "should find greet function def")

	found = findTagByName(defs, "add")
	require.NotNil(t, found, "should find add arrow function def")
}

func TestExtract_KotlinFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`class Calculator {
    fun add(a: Int, b: Int): Int {
        return a + b
    }
}

fun greet() {
    println("hello")
}
`)
	tags, err := ext.Extract(source, "test.kt", "kotlin")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Calculator")
	require.NotNil(t, found, "should find Calculator class def")

	found = findTagByName(defs, "Calculator.add")
	require.NotNil(t, found, "should find qualified method name Calculator.add")

	found = findTagByName(defs, "greet")
	require.NotNil(t, found, "should find greet function def")
}

func TestExtract_ScalaFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`object Calculator {
  def add(a: Int, b: Int): Int = {
    a + b
  }

  def multiply(a: Int, b: Int): Int = {
    a * b
  }
}

class Person(val name: String)

trait Greeter {
  def greet(): Unit
}
`)
	tags, err := ext.Extract(source, "test.scala", "scala")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Calculator")
	require.NotNil(t, found, "should find Calculator object def")
	assert.Equal(t, TagDef, found.Kind)

	found = findTagByName(defs, "Calculator.add")
	require.NotNil(t, found, "should find qualified method Calculator.add")

	found = findTagByName(defs, "Person")
	require.NotNil(t, found, "should find Person class def")

	found = findTagByName(defs, "Greeter")
	require.NotNil(t, found, "should find Greeter trait def")
}

func TestExtract_BashFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`#!/bin/bash

greet() {
  echo "hello"
}

function cleanup {
  rm -rf /tmp/test
}
`)
	tags, err := ext.Extract(source, "test.sh", "bash")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	require.NotEmpty(t, defs, "should have at least one def tag")

	found := findTagByName(defs, "greet")
	require.NotNil(t, found, "should find greet function def")
	assert.Equal(t, TagDef, found.Kind)
}

func TestExtract_HaskellFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`add :: Int -> Int -> Int
add x y = x + y

greet :: String -> String
greet name = "Hello, " ++ name
`)
	tags, err := ext.Extract(source, "test.hs", "haskell")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	require.NotEmpty(t, defs, "should have at least one def tag")

	found := findTagByName(defs, "add")
	require.NotNil(t, found, "should find add def")
	assert.Equal(t, TagDef, found.Kind)
}

func TestExtract_JuliaFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`struct Point
    x::Float64
    y::Float64
end

function add(a, b)
    return a + b
end

macro show_value(x)
    :(println($(string(x)), " = ", $(esc(x))))
end
`)
	tags, err := ext.Extract(source, "test.jl", "julia")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "Point")
	require.NotNil(t, found, "should find Point struct def")

	found = findTagByName(defs, "add")
	require.NotNil(t, found, "should find add function def")

	found = findTagByName(defs, "show_value")
	require.NotNil(t, found, "should find show_value macro def")
}

func TestExtract_OcamlFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`let add x y = x + y

let greet name = "Hello, " ^ name
`)
	tags, err := ext.Extract(source, "test.ml", "ocaml")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	require.NotEmpty(t, defs, "should have at least one def tag")

	found := findTagByName(defs, "add")
	require.NotNil(t, found, "should find add def")
	assert.Equal(t, TagDef, found.Kind)
}

func TestExtract_LuaFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`function greet(name)
  print("Hello, " .. name)
end

local function add(a, b)
  return a + b
end
`)
	tags, err := ext.Extract(source, "test.lua", "lua")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "greet")
	require.NotNil(t, found, "should find greet function def")
	assert.Equal(t, TagDef, found.Kind)
}

func TestExtract_ZigFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`fn add(a: i32, b: i32) i32 {
    return a + b;
}

const x: i32 = 42;
`)
	tags, err := ext.Extract(source, "test.zig", "zig")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "add")
	require.NotNil(t, found, "should find add function def")
	assert.Equal(t, TagDef, found.Kind)

	found = findTagByName(defs, "x")
	require.NotNil(t, found, "should find x variable def")
}

func TestExtract_HclBlock(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`resource "aws_instance" "web" {
  ami           = "abc-123"
  instance_type = "t2.micro"
}

variable "name" {
  default = "test"
}
`)
	tags, err := ext.Extract(source, "test.tf", "hcl")
	require.NoError(t, err)

	defs := filterTags(tags, TagDef)
	require.NotEmpty(t, defs, "should have at least one def tag for HCL blocks")

	// HCL minimal query captures block type identifiers (resource, variable)
	found := findTagByName(defs, "resource")
	require.NotNil(t, found, "should find resource block type")

	found = findTagByName(defs, "variable")
	require.NotNil(t, found, "should find variable block type")
}

func TestExtract_RFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`
greet <- function(name) {
  paste("Hello", name)
}

add <- function(a, b) {
  a + b
}
`)
	tags, err := ext.Extract(source, "test.r", "r")
	require.NoError(t, err)
	defs := filterTags(tags, TagDef)
	found := findTagByName(defs, "greet")
	require.NotNil(t, found, "should find greet def")
	assert.Equal(t, TagDef, found.Kind)

	found = findTagByName(defs, "add")
	require.NotNil(t, found, "should find add def")
}

func TestExtract_SwiftFunction(t *testing.T) {
	ext := newTestExtractor(t)
	source := []byte(`
class Greeter {
    func greet(name: String) -> String {
        return "Hello, " + name
    }
}

func add(a: Int, b: Int) -> Int {
    return a + b
}
`)
	tags, err := ext.Extract(source, "test.swift", "swift")
	require.NoError(t, err)
	defs := filterTags(tags, TagDef)
	require.NotEmpty(t, defs, "should have def tags for Swift")
	found := findTagByName(defs, "add")
	require.NotNil(t, found, "should find add def")
	assert.Equal(t, TagDef, found.Kind)
}

func TestExtract_UnsupportedLanguage(t *testing.T) {
	ext := newTestExtractor(t)
	_, err := ext.Extract([]byte("code"), "test.f90", "fortran")
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
