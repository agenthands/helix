package dataflow_test

import (
	"reflect"
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

// inBodyEqual compares two InBodyFlow slices order-sensitively (both are
// dedup+sorted by AnalyzeFlow, so ordering is canonical).
func inBodyEqual(a, b []dataflow.InBodyFlow) bool {
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

func hasInBody(s *dataflow.Summary, want dataflow.InBodyFlow) bool {
	if s == nil {
		return false
	}
	for _, f := range s.InBodyFlows {
		if f == want {
			return true
		}
	}
	return false
}

// FLOW-04h: single in-body flow — y := producer(); sink(y) => producer->sink.arg0.
func TestAnalyzeFlow_InBody_Single(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f() { y := producer(); sink(y) }\n")
	if s == nil {
		t.Fatal("nil summary; want in-body flow producer->sink")
	}
	want := []dataflow.InBodyFlow{{Producer: "producer", Consumer: "sink", ArgPos: 0}}
	if !inBodyEqual(s.InBodyFlows, want) {
		t.Fatalf("InBodyFlows = %+v, want %+v", s.InBodyFlows, want)
	}
	// A param-less function: no param flows.
	if len(s.Params) != 0 {
		t.Errorf("want no param flows; got %+v", s.Params)
	}
}

// FLOW-04h: in-body + param coexisting — y := producer(x); sink(y) => in-body
// producer->sink.arg0 AND v2.9 param flow x->producer.arg0 (both present).
func TestAnalyzeFlow_InBody_CoexistWithParam(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f(x int) { y := producer(x); sink(y) }\n")
	if s == nil {
		t.Fatal("nil summary; want both in-body and param flow")
	}
	if !hasInBody(s, dataflow.InBodyFlow{Producer: "producer", Consumer: "sink", ArgPos: 0}) {
		t.Errorf("want in-body producer->sink.arg0; got %+v", s.InBodyFlows)
	}
	// v2.9 param flow: x reaches producer arg0.
	if len(s.Params) != 1 || len(s.Params[0].CallArgs) != 1 {
		t.Fatalf("want one param flow x->producer.arg0; got %+v", s.Params)
	}
	ca := s.Params[0].CallArgs[0]
	if ca.Callee != "producer" || ca.ArgPos != 0 {
		t.Errorf("param CallArg = %+v, want {producer,0}", ca)
	}
}

// FLOW-04h: 2-hop chain — a := producer(); b := transform(a); sink(b) =>
// two in-body flows producer->transform.arg0 and transform->sink.arg0 (D-COMPOSE
// chaining: b's origin is callReturn(transform), NOT callReturn(producer)).
func TestAnalyzeFlow_InBody_TwoHopChain(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f() { a := producer(); b := transform(a); sink(b) }\n")
	if s == nil {
		t.Fatal("nil summary; want two in-body flows")
	}
	want := []dataflow.InBodyFlow{
		{Producer: "producer", Consumer: "transform", ArgPos: 0},
		{Producer: "transform", Consumer: "sink", ArgPos: 0},
	}
	if !inBodyEqual(s.InBodyFlows, want) {
		t.Fatalf("InBodyFlows = %+v, want %+v", s.InBodyFlows, want)
	}
}

// FLOW-04h m2 RED guard (positive): nested wrap y := wrap(producer()); sink(y)
// => callReturn(wrap) only (the OUTERMOST call). The InBodyFlow is
// wrap->sink.arg0, NOT producer->sink. producer() flows INTO wrap (no arg here).
func TestAnalyzeFlow_InBody_NestedWrapOutermost(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f() { y := wrap(producer()); sink(y) }\n")
	if s == nil {
		t.Fatal("nil summary; want in-body flow wrap->sink")
	}
	if hasInBody(s, dataflow.InBodyFlow{Producer: "producer", Consumer: "sink", ArgPos: 0}) {
		t.Errorf("must NOT bind producer->sink (D-COMPOSE = outermost); got %+v", s.InBodyFlows)
	}
	want := []dataflow.InBodyFlow{{Producer: "wrap", Consumer: "sink", ArgPos: 0}}
	if !inBodyEqual(s.InBodyFlows, want) {
		t.Fatalf("InBodyFlows = %+v, want %+v", s.InBodyFlows, want)
	}
}

// FLOW-04h m2 RED guard (negative): selector RHS y := producer().field; sink(y)
// => NO in-body origin (selector_expression not in the unwrap whitelist).
func TestAnalyzeFlow_InBody_SelectorRHS_NoOrigin(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f() { y := producer().field; sink(y) }\n")
	if s != nil && len(s.InBodyFlows) != 0 {
		t.Fatalf("selector RHS must yield no in-body origin; got %+v", s.InBodyFlows)
	}
}

// FLOW-04h m2 RED guard (negative): multi-call RHS a, b := f(), g() =>
// NO in-body origin (multi-child expression_list refused by unwrapToCall).
func TestAnalyzeFlow_InBody_MultiCallRHS_NoOrigin(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc c() { a, b := f(), g(); sink(a); sink(b) }\n")
	if s != nil && len(s.InBodyFlows) != 0 {
		t.Fatalf("multi-call RHS must yield no in-body origin; got %+v", s.InBodyFlows)
	}
}

// FLOW-04g anti-vacuity: dead local — y := producer(); return (y unused) =>
// no in-body flow. Revert-and-fail RED: an always-record walk breaks this.
func TestAnalyzeFlow_InBody_DeadLocal_AntiVacuity(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f() { y := producer(); return }\n")
	if s != nil && len(s.InBodyFlows) != 0 {
		t.Fatalf("dead local must yield no in-body flow; got %+v", s.InBodyFlows)
	}
}

// FLOW-04g anti-vacuity: pure param — f(x){ sink(x) } => only a v2.9 param
// flow, ZERO in-body flows.
func TestAnalyzeFlow_InBody_PureParam_NoInBody(t *testing.T) {
	s := analyzeGo(t, "package p\nfunc f(x int) { sink(x) }\n")
	if s == nil {
		t.Fatal("nil summary; want a param flow")
	}
	if len(s.InBodyFlows) != 0 {
		t.Fatalf("pure param must yield no in-body flow; got %+v", s.InBodyFlows)
	}
	if len(s.Params) != 1 || len(s.Params[0].CallArgs) != 1 {
		t.Errorf("want v2.9 param flow x->sink.arg0; got %+v", s.Params)
	}
}

// FLOW-04f determinism: re-analyzing the same source yields a DeepEqual summary
// (no Go-map iteration in the output path).
func TestAnalyzeFlow_InBody_Determinism(t *testing.T) {
	src := "package p\nfunc f(x int) { a := producer(x); b := transform(a); sink(b); other(x) }\n"
	s1 := analyzeGo(t, src)
	s2 := analyzeGo(t, src)
	if !reflect.DeepEqual(s1, s2) {
		t.Fatalf("non-deterministic summary:\n s1=%+v\n s2=%+v", s1, s2)
	}
	if s1 == nil || len(s1.InBodyFlows) != 2 {
		t.Fatalf("want 2 in-body flows; got %+v", s1)
	}
}
