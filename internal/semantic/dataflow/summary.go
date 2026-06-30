// Package dataflow computes a case-1 intraprocedural flow summary over a
// function body: for each parameter, the set of flow targets (the return, and
// call-argument positions) that parameter's value reaches via syntactic
// def-use. It is the input the Phase 126 batch pass composes into
// interprocedural DATA_FLOWS edges (caller.param -> callee.param).
//
// "Case-1" means EXACT syntactic param-to-X data dependence, with no
// over-approximation: a param reaches a target only when a value derived (via
// assignment chains) from that param is returned or passed as a call argument.
// In-body origins (e.g. the return value of an internal call feeding a later
// argument) are NOT modeled — that needs variable-level nodes and is deferred.
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

// Cross-language tree-sitter node-kind unions (mirrors the idiom in
// internal/semantic/classifier/classifier.go walkProfile).
var (
	paramListKinds = stringSet{
		"parameter_list": true, "formal_parameters": true,
		"function_value_parameters": true, "parameters": true,
		"parameter_declarations": true,
	}
	identKinds = stringSet{
		"identifier": true, "variable_name": true,
		"simple_identifier": true, "field_identifier": true,
	}
	assignKinds = stringSet{
		"assignment_expression": true, "short_var_declaration": true,
		"assignment_statement": true, "variable_declaration": true,
		"let_declaration": true, "const_declaration": true,
		"init_declaration": true, "variable_declarator": true,
		"local_variable_statement": true, "var_spec": true,
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
		"argument_lists": true,
	}
)

type stringSet map[string]bool

// AnalyzeFlow walks a function-declaration tree-sitter node and computes the
// case-1 flow summary. The node is the declaration the provider passes to
// FingerprintBody (it contains both the parameter list and the body block for
// Go/C-family; for Python/TS it is the declaration node the name's parent
// resolves to). Returns nil when there are no params or no param reaches a
// target.
func AnalyzeFlow(node *tree_sitter.Node, source []byte) *Summary {
	if node == nil {
		return nil
	}
	paramNames := collectParams(node, source)
	if len(paramNames) == 0 {
		return nil
	}

	// Seed taint: variable name -> set of origin param indices. Each param
	// taints itself; assignments propagate taint to derived locals.
	taint := make(map[string]map[int]bool, len(paramNames))
	for i, name := range paramNames {
		if taint[name] == nil {
			taint[name] = map[int]bool{}
		}
		taint[name][i] = true
	}

	flows := make([]ParamFlow, len(paramNames))
	for i, name := range paramNames {
		flows[i] = ParamFlow{Name: name, Index: i}
	}

	walkFlow(node, source, taint, &flows)

	// Build the summary, dropping dead params (anti-vacuity) and dedup/sort
	// each param's CallArgs for determinism.
	out := &Summary{}
	for i := range flows {
		if flows[i].Returns || len(flows[i].CallArgs) > 0 {
			flows[i].CallArgs = dedupSortCallArgs(flows[i].CallArgs)
			out.Params = append(out.Params, flows[i])
		}
	}
	if len(out.Params) == 0 {
		return nil
	}
	return out
}

// walkFlow recurses over the declaration, skipping parameter-list subtrees
// (so param identifiers are not treated as value reads), and processes
// assignment / return / call nodes to propagate taint and record targets.
func walkFlow(n *tree_sitter.Node, source []byte, taint map[string]map[int]bool, flows *[]ParamFlow) {
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
		processCall(n, source, taint, flows)
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		walkFlow(n.Child(i), source, taint, flows)
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

// processAssignment propagates taint from RHS identifiers to the LHS
// identifier. Only direct-identifier LHS is handled (field/index assignments
// degrade gracefully — no taint recorded, no false edge).
func processAssignment(n *tree_sitter.Node, source []byte, taint map[string]map[int]bool) {
	lhs, rhs := splitAssignment(n, source)
	if lhs == "" || rhs == nil {
		return
	}
	origins := collectOrigins(rhs, source, taint)
	if len(origins) == 0 {
		return
	}
	if taint[lhs] == nil {
		taint[lhs] = map[int]bool{}
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
func processReturn(n *tree_sitter.Node, source []byte, taint map[string]map[int]bool, flows *[]ParamFlow) {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		for o := range collectOrigins(c, source, taint) {
			(*flows)[o].Returns = true
		}
	}
}

// processCall records CallArgTarget{Callee, ArgPos} on every param whose value
// reaches an argument of the call. The callee name is the last segment (so
// obj.method(...) -> "method"), matching the daemon's name-resolution idiom.
func processCall(n *tree_sitter.Node, source []byte, taint map[string]map[int]bool, flows *[]ParamFlow) {
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
			(*flows)[o].CallArgs = append((*flows)[o].CallArgs, CallArgTarget{
				Callee: callee, ArgPos: int(i),
			})
		}
	}
}

// collectOrigins walks an expression node and returns the union of taint
// origins over every tainted identifier it contains.
func collectOrigins(n *tree_sitter.Node, source []byte, taint map[string]map[int]bool) map[int]bool {
	out := map[int]bool{}
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
