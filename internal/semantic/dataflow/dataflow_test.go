package dataflow_test

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/dataflow"
	goextract "github.com/agenthands/helix/internal/semantic/extract/golang"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// analyzeGo parses a Go source snippet and runs dataflow.AnalyzeFlow over the
// first function/method declaration, all while the tree-sitter tree is alive
// (AnalyzeFlow touches *tree_sitter.Node, which is invalid after tree.Close —
// mirroring FingerprintBody's "must be called while the tree is alive" rule).
func analyzeGo(t *testing.T, src string) *dataflow.Summary {
	t.Helper()
	reg := testutil.NewTestRegistry(t)
	lang := goextract.NewProvider(reg).TreeSitterLanguage()
	p := tree_sitter.NewParser()
	defer p.Close()
	if err := p.SetLanguage(lang); err != nil {
		t.Fatalf("setlanguage: %v", err)
	}
	tree := p.Parse([]byte(src), nil)
	defer tree.Close()
	root := tree.RootNode()
	fn := findFuncDecl(root)
	if fn == nil {
		t.Fatalf("no function_declaration found in:\n%s", src)
	}
	return dataflow.AnalyzeFlow(fn, []byte(src))
}

func findFuncDecl(n *tree_sitter.Node) *tree_sitter.Node {
	if n == nil {
		return nil
	}
	if k := n.Kind(); k == "function_declaration" || k == "method_declaration" {
		return n
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		if r := findFuncDecl(n.Child(i)); r != nil {
			return r
		}
	}
	return nil
}

func TestAnalyzeFlow_ParamReachesReturn(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f(x int) int { return x }\n")
	if s == nil {
		t.Fatal("nil summary; want return-flow for x")
	}
	if len(s.Params) != 1 || !s.Params[0].Returns {
		t.Fatalf("want param x reaches return; got %+v", s)
	}
	if s.Params[0].Name != "x" || s.Params[0].Index != 0 {
		t.Errorf("param = %+v, want {x,0}", s.Params[0])
	}
}

func TestAnalyzeFlow_ParamReachesCallArg(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f(x int) { sink(x) }\n")
	if s == nil {
		t.Fatal("nil summary; want call-arg flow for x")
	}
	if len(s.Params) != 1 || len(s.Params[0].CallArgs) != 1 {
		t.Fatalf("want one CallArgTarget; got %+v", s)
	}
	ca := s.Params[0].CallArgs[0]
	if ca.Callee != "sink" || ca.ArgPos != 0 {
		t.Errorf("CallArg = %+v, want {sink,0}", ca)
	}
}

func TestAnalyzeFlow_TransitiveThroughLocal(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f(x int) { y := x; g(y) }\n")
	if s == nil {
		t.Fatal("nil summary; want transitive flow x->y->g arg0")
	}
	if len(s.Params) != 1 || len(s.Params[0].CallArgs) != 1 {
		t.Fatalf("want transitive call-arg; got %+v", s)
	}
	ca := s.Params[0].CallArgs[0]
	if ca.Callee != "g" || ca.ArgPos != 0 {
		t.Errorf("CallArg = %+v, want {g,0}", ca)
	}
}

func TestAnalyzeFlow_DeadParamNoFlow_AntiVacuity(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f(x int) int { return 7 }\n")
	// x is never forwarded -> anti-vacuity: nil summary (no param reaches anything).
	if s != nil {
		t.Fatalf("want nil summary for dead param (anti-vacuity); got %+v", s)
	}
}

func TestAnalyzeFlow_NoParamsNil(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f() { sink() }\n")
	if s != nil {
		t.Fatalf("want nil summary for no-param function; got %+v", s)
	}
}

func TestAnalyzeFlow_Determinism(t *testing.T) {
	src := "package p\nfunc f(a int, b int) { y := a; sink(y); z := b; return z }\n"
	s1 := analyzeGo(t, src)
	s2 := analyzeGo(t, src)
	if (s1 == nil) != (s2 == nil) {
		t.Fatalf("determinism: s1 nil=%v s2 nil=%v", s1 == nil, s2 == nil)
	}
	if s1 == nil {
		return
	}
	if len(s1.Params) != len(s2.Params) {
		t.Fatalf("param count differs: %d vs %d", len(s1.Params), len(s2.Params))
	}
	for i := range s1.Params {
		a, b := s1.Params[i], s2.Params[i]
		if a.Name != b.Name || a.Index != b.Index || a.Returns != b.Returns || !callArgsEqual(a.CallArgs, b.CallArgs) {
			t.Errorf("param %d differs:\n s1=%+v\n s2=%+v", i, a, b)
		}
	}
}

func callArgsEqual(a, b []dataflow.CallArgTarget) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAnalyzeFlow_Nil(t *testing.T) {
	if s := dataflow.AnalyzeFlow(nil, nil); s != nil {
		t.Fatalf("AnalyzeFlow(nil,nil) = %+v, want nil", s)
	}
}

// TestAnalyzeFlow_TwoParamsDistinct verifies two params each reach distinct
// targets and are not conflated — the precision property the taint substrate
// relies on.
func TestAnalyzeFlow_TwoParamsDistinct(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f(a int, b int) { sinkA(a); sinkB(b) }\n")
	if s == nil {
		t.Fatal("nil summary; want two param flows")
	}
	if len(s.Params) != 2 {
		t.Fatalf("want 2 param flows; got %d: %+v", len(s.Params), s)
	}
	for _, pf := range s.Params {
		if len(pf.CallArgs) != 1 {
			t.Fatalf("param %s: want 1 CallArg; got %+v", pf.Name, pf.CallArgs)
		}
	}
}
