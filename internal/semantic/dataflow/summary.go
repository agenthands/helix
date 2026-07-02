// Package dataflow computes a case-1 intraprocedural flow summary over a
// function body: for each parameter, the set of flow targets (the return, and
// call-argument positions) that parameter's value reaches via syntactic
// def-use. It is the input the Phase 126 batch pass composes into
// interprocedural DATA_FLOWS edges (caller.param -> callee.param).
//
// "Case-1" means EXACT syntactic param-to-X data dependence, with no
// over-approximation: a param reaches a target only when a value derived (via
// assignment chains) from that param is returned or passed as a call argument.
// In-body origins — the return value of an in-body call feeding a later call
// argument (e.g. `y := producer(); sink(y)`) — are ALSO modeled, recorded in
// Summary.InBodyFlows (producer callee -> consumer callee argument position).
//
// The package is a leaf: stdlib + tree-sitter only, mirroring the minhash /
// relatedidx / classifier leaf-package boundary. It is invoked from
// extract.FingerprintBody (the shared seam every provider calls), so all 11
// language providers inherit flow-summary computation with no per-provider edit.
package dataflow

import (
	"sort"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Summary is the case-1 flow summary for one function: per parameter, the
// targets its value reaches. A parameter with no targets is omitted (the
// anti-vacuity property: a dead parameter yields no flow). AnalyzeFlow returns
// nil when the function has no parameters or no parameter reaches anything.
type Summary struct {
	Params []ParamFlow
	// InBodyFlows records producer->consumer in-body data dependences: the
	// return value of an in-body call C (the "producer") flowing into argument
	// ArgPos of a later call (the "consumer"). Dedup+sorted for determinism;
	// nil when the body has no such flow.
	InBodyFlows []InBodyFlow
}

// ParamFlow is the per-parameter flow record. Index is the 0-based parameter
// position, which MUST agree with the param-node emit order Phase 126 uses to
// resolve callee parameter nodes (FLOW-01f pin).
type ParamFlow struct {
	Name     string
	Index    int
	Returns  bool           // value derived from this param reaches a return
	CallArgs []CallArgTarget // value derived from this param reaches a call arg
}

// CallArgTarget records that a value derived from a param reaches argument
// position ArgPos of a call to Callee (unresolved name; Phase 126 resolves it
// against the batch name->node index).
type CallArgTarget struct {
	Callee string
	ArgPos int
}

// Origin discriminates the source of a tainted value. Callee=="" means a
// PARAM origin (Param is the 0-based parameter index). Callee!="" means a
// callReturn origin: the value is the return of an in-body call to Callee
// (Param unused, 0). Origin is a comparable struct so it keys a set map.
type Origin struct {
	Param  int
	Callee string
}

// InBodyFlow records that the return value of in-body call Producer flows into
// argument position ArgPos of a call to Consumer. Producer/Consumer are
// last-segment callee names (unresolved; Phase 139 resolves them to nodes).
type InBodyFlow struct {
	Producer string
	Consumer string
	ArgPos   int
}

// Cross-language tree-sitter node-kind unions (mirrors the idiom in
// internal/semantic/classifier/classifier.go walkProfile).
var (
	paramListKinds = stringSet{
		"parameter_list": true, "formal_parameters": true,
		"function_value_parameters": true, "parameters": true,
		"parameter_declarations": true, "method_parameters": true,
	}
	identKinds = stringSet{
		"identifier": true, "variable_name": true,
		"simple_identifier": true, "field_identifier": true,
	}
	assignKinds = stringSet{
		"assignment_expression": true, "short_var_declaration": true,
		"assignment_statement": true, "variable_declaration": true,
		"let_declaration": true, "const_declaration": true,
		"init_declarator": true, "variable_declarator": true,
		"local_variable_statement": true, "var_spec": true,
		"assignment": true, "property_declaration": true,
	}
	returnKinds = stringSet{
		"return_statement": true, "return_expression": true,
	}
	callKinds = stringSet{
		"call_expression": true, "method_invocation": true,
		"function_call_expression": true, "invocation_expression": true,
		"call": true,
	}
	argListKinds = stringSet{
		"argument_list": true, "arguments": true,
		"argument_lists": true, "value_arguments": true,
	}
)

type stringSet map[string]bool

// AnalyzeFlow walks a function-declaration tree-sitter node and computes the
// case-1 flow summary. The node is the declaration the provider passes to
// FingerprintBody (it contains both the parameter list and the body block for
// Go/C-family; for Python/TS it is the declaration node the name's parent
// resolves to). It ALWAYS walks the body (even for a param-less function) so
// in-body origins are captured. Returns nil only when neither a param reaches
// a target NOR any in-body flow is recorded (the generalized anti-vacuity gate).
func AnalyzeFlow(node *tree_sitter.Node, source []byte) *Summary {
	if node == nil {
		return nil
	}
	paramNames := collectParams(node, source)

	// Seed taint: variable name -> set of origins. Each param taints itself
	// with a param origin; assignments propagate/introduce origins to derived
	// locals (including callReturn origins for in-body call results).
	taint := make(map[string]map[Origin]bool, len(paramNames))
	for i, name := range paramNames {
		if taint[name] == nil {
			taint[name] = map[Origin]bool{}
		}
		taint[name][Origin{Param: i}] = true
	}

	flows := make([]ParamFlow, len(paramNames))
	for i, name := range paramNames {
		flows[i] = ParamFlow{Name: name, Index: i}
	}

	var inBody []InBodyFlow
	walkFlow(node, source, taint, &flows, &inBody)

	// Build the summary, dropping dead params (anti-vacuity) and dedup/sort
	// each param's CallArgs for determinism.
	out := &Summary{}
	for i := range flows {
		if flows[i].Returns || len(flows[i].CallArgs) > 0 {
			flows[i].CallArgs = dedupSortCallArgs(flows[i].CallArgs)
			out.Params = append(out.Params, flows[i])
		}
	}
	out.InBodyFlows = dedupSortInBody(inBody)
	if len(out.Params) == 0 && len(out.InBodyFlows) == 0 {
		return nil
	}
	return out
}

// walkFlow recurses over the declaration, skipping parameter-list subtrees
// (so param identifiers are not treated as value reads), and processes
// assignment / return / call nodes to propagate taint and record targets.
func walkFlow(n *tree_sitter.Node, source []byte, taint map[string]map[Origin]bool, flows *[]ParamFlow, inBody *[]InBodyFlow) {
	if n == nil {
		return
	}
	kind := n.Kind()
	if paramListKinds[kind] {
		return // do not descend into the parameter list
	}
	switch {
	case assignKinds[kind]:
		processAssignment(n, source, taint)
	case returnKinds[kind]:
		processReturn(n, source, taint, flows)
	case callKinds[kind]:
		processCall(n, source, taint, flows, inBody)
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		walkFlow(n.Child(i), source, taint, flows, inBody)
	}
}

// collectParams finds the (first) parameter list within the declaration and
// returns its parameter identifier names in source order. If none is found in
// the subtree and the node has a parent, it retries on the parent (covers a
// provider that passes only the body block).
func collectParams(decl *tree_sitter.Node, source []byte) []string {
	var names []string
	if found := findFirst(decl, func(n *tree_sitter.Node) bool {
		return paramListKinds[n.Kind()]
	}); found != nil {
		collectIdentNames(found, source, &names)
	} else if p := decl.Parent(); p != nil {
		if found := findFirst(p, func(n *tree_sitter.Node) bool {
			return paramListKinds[n.Kind()]
		}); found != nil {
			collectIdentNames(found, source, &names)
		}
	}
	return names
}

// processAssignment records taint on the LHS identifier from the RHS. When the
// RHS unwraps (via the strict {expression_list, parenthesized_expression}
// whitelist) to a call C, the LHS receives a callReturn(C) origin (D-COMPOSE:
// the outermost call's return, NOT the inner args). Otherwise it unions the
// generic identifier origins found in the RHS (param + callReturn forwarding).
// Field/index LHS and selector/member/multi-call RHS degrade gracefully — no
// taint recorded, no false origin.
func processAssignment(n *tree_sitter.Node, source []byte, taint map[string]map[Origin]bool) {
	lhs, rhs := splitAssignment(n, source)
	if lhs == "" || rhs == nil {
		return
	}
	if call := unwrapToCall(rhs); call != nil {
		if name := calleeName(call, source); name != "" {
			if taint[lhs] == nil {
				taint[lhs] = map[Origin]bool{}
			}
			taint[lhs][Origin{Callee: name}] = true
			return
		}
	}
	origins := collectOrigins(rhs, source, taint)
	if len(origins) == 0 {
		return
	}
	if taint[lhs] == nil {
		taint[lhs] = map[Origin]bool{}
	}
	for o := range origins {
		taint[lhs][o] = true
	}
}

// splitAssignment returns the LHS identifier name and the RHS expression node.
// It tries named fields first (left/right, name/value, name/initializer), then
// falls back to "first named child = LHS, last named child = RHS" which covers
// Go short_var_declaration / assignment_statement / var_declaration.
func splitAssignment(n *tree_sitter.Node, source []byte) (string, *tree_sitter.Node) {
	if l := n.ChildByFieldName("left"); l != nil {
		if r := n.ChildByFieldName("right"); r != nil {
			return identText(l, source), r
		}
	}
	if nm := n.ChildByFieldName("name"); nm != nil {
		if v := n.ChildByFieldName("value"); v != nil {
			return identText(nm, source), v
		}
		if init := n.ChildByFieldName("initializer"); init != nil {
			return identText(nm, source), init
		}
	}
	cnt := n.NamedChildCount()
	if cnt < 2 {
		return "", nil
	}
	first := n.NamedChild(0)
	last := n.NamedChild(cnt - 1)
	return identText(first, source), last
}

// processReturn marks Returns on every param whose value (possibly via a
// derived local) appears in the returned expression.
func processReturn(n *tree_sitter.Node, source []byte, taint map[string]map[Origin]bool, flows *[]ParamFlow) {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		for o := range collectOrigins(c, source, taint) {
			if o.Callee == "" {
				(*flows)[o.Param].Returns = true
			}
			// callReturn origins reaching a return are NOT recorded (locked
			// out: no edge maps to it in v2.13).
		}
	}
}

// processCall records, per call argument, either a v2.9 CallArgTarget (param
// origin) or an InBodyFlow (callReturn origin). The callee name is the last
// segment (so obj.method(...) -> "method"), matching the daemon's idiom.
func processCall(n *tree_sitter.Node, source []byte, taint map[string]map[Origin]bool, flows *[]ParamFlow, inBody *[]InBodyFlow) {
	callee := calleeName(n, source)
	if callee == "" {
		return
	}
	args := n.ChildByFieldName("arguments")
	if args == nil {
		args = findFirst(n, func(c *tree_sitter.Node) bool { return argListKinds[c.Kind()] })
	}
	if args == nil {
		return
	}
	for i := uint(0); i < args.NamedChildCount(); i++ {
		arg := args.NamedChild(i)
		if arg == nil {
			continue
		}
		for o := range collectOrigins(arg, source, taint) {
			if o.Callee == "" {
				(*flows)[o.Param].CallArgs = append((*flows)[o.Param].CallArgs, CallArgTarget{
					Callee: callee, ArgPos: int(i),
				})
			} else {
				*inBody = append(*inBody, InBodyFlow{
					Producer: o.Callee, Consumer: callee, ArgPos: int(i),
				})
			}
		}
	}
}

// collectOrigins walks an expression node and returns the union of taint
// origins over every tainted identifier it contains.
func collectOrigins(n *tree_sitter.Node, source []byte, taint map[string]map[Origin]bool) map[Origin]bool {
	out := map[Origin]bool{}
	var w func(nd *tree_sitter.Node)
	w = func(nd *tree_sitter.Node) {
		if nd == nil {
			return
		}
		if identKinds[nd.Kind()] {
			if origins, ok := taint[nd.Utf8Text(source)]; ok {
				for o := range origins {
					out[o] = true
				}
			}
		}
		for i := uint(0); i < nd.ChildCount(); i++ {
			w(nd.Child(i))
		}
	}
	w(n)
	return out
}

// calleeName resolves the called function's name (last segment) from a call
// node's function/name field, falling back to the first named child.
func calleeName(n *tree_sitter.Node, source []byte) string {
	var fn *tree_sitter.Node
	if f := n.ChildByFieldName("function"); f != nil {
		fn = f
	} else if f := n.ChildByFieldName("name"); f != nil {
		fn = f
	} else if n.NamedChildCount() > 0 {
		fn = n.NamedChild(0)
	}
	if fn == nil {
		return ""
	}
	return lastSegment(fn.Utf8Text(source))
}

// identText returns the first identifier name under n — used to extract the LHS
// target of an assignment. Handles both bare identifiers (Go
// assignment_statement `a = b`) and identifier lists (Go short_var_declaration
// `a := b`, whose LHS is an identifier_list node, not a bare identifier).
// Returns the first identifier in source order; multi-assign LHS degrades to
// the first name only (acceptable for case-1 single-target forwarding).
func identText(n *tree_sitter.Node, source []byte) string {
	if n == nil {
		return ""
	}
	if identKinds[n.Kind()] {
		return n.Utf8Text(source)
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		if name := identText(n.Child(i), source); name != "" {
			return name
		}
	}
	return ""
}

// collectIdentNames appends identifier names under a parameter-list node, in
// source order, to *out.
func collectIdentNames(plist *tree_sitter.Node, source []byte, out *[]string) {
	var w func(nd *tree_sitter.Node)
	w = func(nd *tree_sitter.Node) {
		if nd == nil {
			return
		}
		if identKinds[nd.Kind()] {
			*out = append(*out, nd.Utf8Text(source))
			return // an identifier is a leaf for this purpose
		}
		for i := uint(0); i < nd.ChildCount(); i++ {
			w(nd.Child(i))
		}
	}
	w(plist)
}

// findFirst returns the first node (preorder) under n satisfying pred, or nil.
func findFirst(n *tree_sitter.Node, pred func(*tree_sitter.Node) bool) *tree_sitter.Node {
	if n == nil {
		return nil
	}
	if pred(n) {
		return n
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		if r := findFirst(n.Child(i), pred); r != nil {
			return r
		}
	}
	return nil
}

// lastSegment returns the trailing identifier of a (possibly) qualified name:
// "obj.method" -> "method", "pkg::Func" -> "Func", "a/b" -> "b".
func lastSegment(q string) string {
	for _, sep := range []string{".", "::", "/", "\\", "->"} {
		if i := strings.LastIndex(q, sep); i >= 0 {
			return q[i+len(sep):]
		}
	}
	return q
}

// dedupSortCallArgs makes a param's CallArg list deterministic and duplicate-free.
func dedupSortCallArgs(in []CallArgTarget) []CallArgTarget {
	if len(in) == 0 {
		return in
	}
	sort.Slice(in, func(i, j int) bool {
		if in[i].Callee != in[j].Callee {
			return in[i].Callee < in[j].Callee
		}
		return in[i].ArgPos < in[j].ArgPos
	})
	out := in[:1]
	for i := 1; i < len(in); i++ {
		if in[i] != out[len(out)-1] {
			out = append(out, in[i])
		}
	}
	return out
}

// unwrapToCall unwraps an RHS expression through the STRICT single-child
// whitelist {expression_list, parenthesized_expression} to a call node, or nil.
// A multi-child expression_list (`a, b := f(), g()`) or a non-whitelisted
// wrapper (selector/member/index/binary) refuses — no in-body origin (D-COMPOSE
// + m2 guard). Go's short_var_declaration RHS is an expression_list wrapping the
// call; most other languages hand the call directly.
func unwrapToCall(n *tree_sitter.Node) *tree_sitter.Node {
	for n != nil && (n.Kind() == "expression_list" || n.Kind() == "parenthesized_expression") {
		if n.NamedChildCount() != 1 {
			return nil // multi-call / ambiguous -> refuse
		}
		n = n.NamedChild(0)
	}
	if n != nil && callKinds[n.Kind()] {
		return n
	}
	return nil
}

// dedupSortInBody makes the in-body flow list deterministic and duplicate-free,
// mirroring dedupSortCallArgs. No Go-map iteration in the output path.
func dedupSortInBody(in []InBodyFlow) []InBodyFlow {
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool {
		if in[i].Producer != in[j].Producer {
			return in[i].Producer < in[j].Producer
		}
		if in[i].Consumer != in[j].Consumer {
			return in[i].Consumer < in[j].Consumer
		}
		return in[i].ArgPos < in[j].ArgPos
	})
	out := in[:1]
	for i := 1; i < len(in); i++ {
		if in[i] != out[len(out)-1] {
			out = append(out, in[i])
		}
	}
	return out
}
