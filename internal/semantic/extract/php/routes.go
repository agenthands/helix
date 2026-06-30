package phpextract

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// routes.go detects HTTP route registrations in PHP and synthesizes a
// RouteFact per (method, path, handler). Two shapes:
//   - Laravel static calls:    Route::get("/p", h), Route::post("/p", h)
//   - Slim/Silex member calls: $app->get("/p", h), $app->post("/p", h)
//
// The verb is the callee member name (get/post/put/delete/patch), uppercased
// to the HTTP method. The first argument must be a string-literal path. The
// handler: for the array form [Controller::class, "method"] the string element
// names the handler method; for a plain callable/string argument that name;
// otherwise "" (closures, variables).

// phpRouteMethods maps a lowercase verb to the uppercase HTTP method. Presence
// in the map marks the callee member as a route registrar.
var phpRouteMethods = map[string]string{
	"get": "GET", "post": "POST", "put": "PUT",
	"delete": "DELETE", "patch": "PATCH",
}

func detectPhpRoutes(root tree_sitter.Node, source []byte, filePath string) []extract.RouteFact {
	var routes []extract.RouteFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		switch n.Kind() {
		case "scoped_call_expression", "member_call_expression":
			if rf, ok := phpRouteFromCall(n, source, filePath); ok {
				routes = append(routes, rf)
			}
		}
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if c := n.NamedChild(i); c != nil {
				walk(*c)
			}
		}
	}
	walk(root)
	return routes
}

// phpRouteFromCall handles both Route::VERB("/p", h) (scoped_call_expression)
// and $app->VERB("/p", h) (member_call_expression). Both expose a "name"
// field (the verb) and an "arguments" field.
func phpRouteFromCall(call tree_sitter.Node, source []byte, filePath string) (extract.RouteFact, bool) {
	name := call.ChildByFieldName("name")
	args := call.ChildByFieldName("arguments")
	if name == nil || args == nil {
		return extract.RouteFact{}, false
	}
	method, isRoute := phpRouteMethods[strings.ToLower(name.Utf8Text(source))]
	if !isRoute {
		return extract.RouteFact{}, false
	}
	// First argument must be a string literal (the route path); a non-string
	// first arg means this is not a route registration.
	path, ok := phpStringArg(args, 0, source)
	if !ok {
		return extract.RouteFact{}, false
	}
	return extract.RouteFact{
		Language: "php",
		Method:   method,
		Path:     path,
		Handler:  phpHandlerName(args, source),
		File:     filePath,
		Range:    nodeRange(call),
	}, true
}

// phpStringArg returns the unquoted content of the i-th positional argument if
// it is a string literal.
func phpStringArg(args *tree_sitter.Node, i int, source []byte) (string, bool) {
	arg := args.NamedChild(uint(i))
	if arg == nil {
		return "", false
	}
	return phpStringContent(phpArgValue(arg), source)
}

// phpHandlerName extracts the handler symbol from the second positional
// argument: a plain callable string gives its name; the array form
// [Controller::class, "method"] gives the string element; anything else
// (closures, variables) yields "".
func phpHandlerName(args *tree_sitter.Node, source []byte) string {
	arg := args.NamedChild(1)
	if arg == nil {
		return ""
	}
	val := phpArgValue(arg)
	if val == nil {
		return ""
	}
	// Plain callable: "name" (string literal) or a bareword function name.
	if s, ok := phpStringContent(val, source); ok {
		return s
	}
	if val.Kind() == "name" {
		return val.Utf8Text(source)
	}
	// Array form: [Controller::class, "method"] — the string element names
	// the handler method.
	if val.Kind() == "array_creation_expression" {
		for i := uint(0); i < val.NamedChildCount(); i++ {
			el := val.NamedChild(i)
			if el == nil {
				continue
			}
			if s, ok := phpStringContent(phpArgValue(el), source); ok {
				return s
			}
		}
	}
	return ""
}

// phpArgValue unwraps an `argument` or `array_element_initializer` wrapper to
// its inner value node. PHP wraps each call argument in `argument` and each
// array element in `array_element_initializer`. Returns the node itself when
// it is not such a wrapper.
func phpArgValue(n *tree_sitter.Node) *tree_sitter.Node {
	if n != nil && (n.Kind() == "argument" || n.Kind() == "array_element_initializer") {
		if c := n.NamedChild(0); c != nil {
			return c
		}
	}
	return n
}

// phpStringContent returns the unquoted content of a PHP string-literal node,
// or (text, false) if the node is not a string literal. Handles both
// double-quoted (encapsed_string) and single-quoted (string) literals.
func phpStringContent(n *tree_sitter.Node, source []byte) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.Kind() {
	case "encapsed_string", "string":
		return phpUnquote(n.Utf8Text(source)), true
	}
	return "", false
}

// phpUnquote strips the surrounding quotes from a PHP string literal.
func phpUnquote(s string) string {
	if len(s) < 2 {
		return s
	}
	first, last := s[0], s[len(s)-1]
	if (first == '"' || first == '\'') && last == first {
		return s[1 : len(s)-1]
	}
	return s
}
