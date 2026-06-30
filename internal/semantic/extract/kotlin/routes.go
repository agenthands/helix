package kotlinextract

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// routes.go detects HTTP route registrations in Kotlin source (Ktor routing
// DSL) and synthesizes a RouteFact per (method, path, handler). factsFromExtracted
// promotes each into a Route symbol + a HANDLES edge to the handler.
//
// Ktor route registrations are call expressions of the form
//
//	routing {
//	    get("/users") { ... }
//	    post("/items") { ... }
//	    route("/prefix") { ... }
//	}
//
// The callee is a bare identifier (get/post/put/delete/...) with a
// string-literal first argument; the handler is an anonymous trailing lambda
// (Handler ""). tree-sitter-kotlin parses a trailing-lambda call as a nested
// call_expression{ call_expression{ ident, args }, annotated_lambda }; the
// INNER call_expression carries the identifier + value_arguments we match on,
// so the outer wrapper is naturally skipped (it has no direct identifier /
// value_arguments child).

// kotlinRouteMethods maps a Ktor route-DSL callee name to the HTTP method it
// implies; "" means the method is not fixed by the framework (route() opens a
// path prefix). Presence marks the identifier as a route registrar.
var kotlinRouteMethods = map[string]string{
	"get":     "GET",
	"post":    "POST",
	"put":     "PUT",
	"delete":  "DELETE",
	"patch":   "PATCH",
	"head":    "HEAD",
	"options": "OPTIONS",
	"route":   "",
}

func detectKotlinRoutes(root tree_sitter.Node, source []byte, filePath string) []extract.RouteFact {
	var routes []extract.RouteFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "call_expression" {
			if rf, ok := kotlinRouteFromCall(n, source, filePath); ok {
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

// kotlinRouteFromCall matches a single call_expression against the Ktor route
// DSL. Only bare-identifier callees are considered so that unrelated member
// calls (e.g. httpClient.get("/url")) are not mistaken for routes.
func kotlinRouteFromCall(call tree_sitter.Node, source []byte, filePath string) (extract.RouteFact, bool) {
	// The callee identifier and the value_arguments are direct named children
	// of the call_expression that owns them (the inner call_expression when a
	// trailing lambda wraps the call). The outer wrapper's named children are a
	// nested call_expression + an annotated_lambda, so it has neither and is
	// skipped here.
	var callee *tree_sitter.Node
	var args *tree_sitter.Node
	for i := uint(0); i < call.NamedChildCount(); i++ {
		c := call.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case "identifier":
			if callee == nil {
				callee = c
			}
		case "value_arguments":
			if args == nil {
				args = c
			}
		}
	}
	if callee == nil || args == nil {
		return extract.RouteFact{}, false
	}
	method, isRoute := kotlinRouteMethods[callee.Utf8Text(source)]
	if !isRoute {
		return extract.RouteFact{}, false
	}
	// First positional argument must be a string literal (the route path).
	path, ok := kotlinRouteStringArg(args, source)
	if !ok {
		return extract.RouteFact{}, false
	}
	return extract.RouteFact{
		Language: "kotlin",
		Method:   method,
		Path:     path,
		Handler:  "", // Ktor handlers are trailing lambdas — anonymous.
		File:     filePath,
		Range:    nodeRange(call),
	}, true
}

// kotlinRouteStringArg returns the first positional argument if it is a Kotlin
// string literal. value_arguments wraps each argument in a value_argument node,
// whose first named child is the expression.
func kotlinRouteStringArg(args *tree_sitter.Node, source []byte) (string, bool) {
	arg := args.NamedChild(0)
	if arg == nil || arg.Kind() != "value_argument" {
		return "", false
	}
	expr := arg.NamedChild(0)
	if expr == nil || expr.Kind() != "string_literal" {
		return "", false
	}
	return kotlinTrimString(expr.Utf8Text(source)), true
}

// kotlinTrimString strips the surrounding double quotes from a Kotlin string
// literal (e.g. `"/users"` -> `/users`).
func kotlinTrimString(s string) string {
	if len(s) < 2 {
		return s
	}
	if s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
