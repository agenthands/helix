package dataflow_test

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/dataflow"
	"github.com/agenthands/helix/internal/semantic/extract"
	cextract "github.com/agenthands/helix/internal/semantic/extract/c"
	cppextract "github.com/agenthands/helix/internal/semantic/extract/cpp"
	csharpextract "github.com/agenthands/helix/internal/semantic/extract/csharp"
	goextract2 "github.com/agenthands/helix/internal/semantic/extract/golang"
	javaextract "github.com/agenthands/helix/internal/semantic/extract/java"
	kotlinextract "github.com/agenthands/helix/internal/semantic/extract/kotlin"
	phpextract "github.com/agenthands/helix/internal/semantic/extract/php"
	pyextract "github.com/agenthands/helix/internal/semantic/extract/python"
	rubyextract "github.com/agenthands/helix/internal/semantic/extract/ruby"
	rustextract "github.com/agenthands/helix/internal/semantic/extract/rust"
	tsextract "github.com/agenthands/helix/internal/semantic/extract/typescript"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// findByKind returns the first node (preorder) whose Kind matches kind, or nil.
func findByKind(n *tree_sitter.Node, kind string) *tree_sitter.Node {
	if n == nil {
		return nil
	}
	if n.Kind() == kind {
		return n
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		if r := findByKind(n.Child(i), kind); r != nil {
			return r
		}
	}
	return nil
}

// TestAnalyzeFlow_AllElevenMatrix is the M5/FLOW-04i breadth proof: run the
// pure-tree-sitter AnalyzeFlow on the idiomatic `local := producer(); sink(local)`
// fixture in ALL 11 grammars and assert the in-body flow producer->sink.arg0 is
// recorded. This surfaces the kind-union breadth risk at the cheapest phase.
// A language that genuinely cannot resolve is t.Skip-ped with a recorded reason.
func TestAnalyzeFlow_AllElevenMatrix(t *testing.T) {
	reg := testutil.NewTestRegistry(t)
	type tc struct {
		lang         string
		prov         extract.Provider
		src          string
		funcDeclKind string
		skipReason   string // non-empty => t.Skip with this reason (honest breadth ledger)
	}
	// Fixtures are idiomatic per language; Java/C#/Kotlin need a class wrapper so
	// exactly one function/method exists to analyze.
	cases := []tc{
		{lang: "go", prov: goextract2.NewProvider(reg),
			src: "package p\nfunc f() { local := producer(); sink(local) }\n", funcDeclKind: "function_declaration"},
		{lang: "typescript", prov: tsextract.NewProvider(reg),
			src: "function f() { const local = producer(); sink(local); }\n", funcDeclKind: "function_declaration"},
		{lang: "java", prov: javaextract.NewProvider(reg),
			src: "class C { void f() { var local = producer(); sink(local); } }\n", funcDeclKind: "method_declaration"},
		{lang: "csharp", prov: csharpextract.NewProvider(reg),
			src: "class C { void f() { var local = producer(); sink(local); } }\n", funcDeclKind: "method_declaration"},
		{lang: "kotlin", prov: kotlinextract.NewProvider(reg),
			src: "class C { fun f() { val local = producer(); sink(local) } }\n", funcDeclKind: "function_declaration"},
		{lang: "php", prov: phpextract.NewProvider(reg),
			src: "<?php function f() { $local = producer(); sink($local); }\n", funcDeclKind: "function_definition"},
		{lang: "python", prov: pyextract.NewProvider(reg),
			src: "def f():\n    local = producer()\n    sink(local)\n", funcDeclKind: "function_definition"},
		{lang: "ruby", prov: rubyextract.NewProvider(reg),
			src: "def f\n  local = producer()\n  sink(local)\nend\n", funcDeclKind: "method"},
		{lang: "rust", prov: rustextract.NewProvider(reg),
			src: "fn f() { let local = producer(); sink(local); }\n", funcDeclKind: "function_item"},
		{lang: "c", prov: cextract.NewProvider(reg),
			src: "void f() { int local = producer(); sink(local); }\n", funcDeclKind: "function_definition"},
		{lang: "cpp", prov: cppextract.NewProvider(reg),
			src: "void f() { int local = producer(); sink(local); }\n", funcDeclKind: "function_definition"},
	}

	want := dataflow.InBodyFlow{Producer: "producer", Consumer: "sink", ArgPos: 0}

	for _, c := range cases {
		t.Run(c.lang, func(t *testing.T) {
			if c.skipReason != "" {
				t.Skipf("breadth-ledger skip: %s", c.skipReason)
			}
			p := tree_sitter.NewParser()
			defer p.Close()
			if err := p.SetLanguage(c.prov.TreeSitterLanguage()); err != nil {
				t.Fatalf("%s: setlanguage: %v", c.lang, err)
			}
			tree := p.Parse([]byte(c.src), nil)
			defer tree.Close()
			fn := findByKind(tree.RootNode(), c.funcDeclKind)
			if fn == nil {
				t.Fatalf("%s: no %q node in:\n%s", c.lang, c.funcDeclKind, c.src)
			}
			s := dataflow.AnalyzeFlow(fn, []byte(c.src))
			if s == nil {
				t.Fatalf("%s: nil summary; want in-body flow %+v", c.lang, want)
			}
			if !hasInBody(s, want) {
				t.Fatalf("%s: InBodyFlows = %+v, want to contain %+v", c.lang, s.InBodyFlows, want)
			}
		})
	}
}
