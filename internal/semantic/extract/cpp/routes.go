package cppextract

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// routes.go detects HTTP route registrations in C++ source and synthesizes a
// RouteFact per (method, path). Frameworks:
//   - Crow:           CROW_ROUTE(app, "/users")(...)
//   - cpp-httplib:    svr.Get("/users", handler), svr.Post("/users", h)
//
// (C has no dominant tree-sitter-detectable web framework; route detection is
// C++-only here.)

// cppRouterMethods maps an httplib member-call field to its HTTP method.
var cppRouterMethods = map[string]string{
	"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE",
	"Patch": "PATCH", "Head": "HEAD", "Options": "OPTIONS",
}

func detectCppRoutes(root tree_sitter.Node, source []byte, filePath string) []extract.RouteFact {
	var routes []extract.RouteFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "call_expression" {
			if rf, ok := cppRouteFromCall(n, source, filePath); ok {
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

func cppRouteFromCall(call tree_sitter.Node, source []byte, filePath string) (extract.RouteFact, bool) {
	fn := call.ChildByFieldName("function")
	args := call.ChildByFieldName("arguments")
	if fn == nil || args == nil {
		return extract.RouteFact{}, false
	}
	var method string
	switch fn.Kind() {
	case "identifier":
		// Crow macro-style call: CROW_ROUTE(app, "/path")(...)
		if fn.Utf8Text(source) != "CROW_ROUTE" {
			return extract.RouteFact{}, false
		}
		method = "" // Crow fixes no method at the CROW_ROUTE call
	case "field_expression":
		field := fn.ChildByFieldName("field")
		if field == nil {
			return extract.RouteFact{}, false
		}
		m, isRoute := cppRouterMethods[field.Utf8Text(source)]
		if !isRoute {
			return extract.RouteFact{}, false
		}
		method = m
	default:
		return extract.RouteFact{}, false
	}
	path, ok := cppFirstStringArg(args, source)
	if !ok {
		return extract.RouteFact{}, false
	}
	return extract.RouteFact{
		Language: "cpp",
		Method:   method,
		Path:     path,
		Handler:  "", // C++ handlers are often lambdas — best-effort "" for now
		File:     filePath,
		Range:    nodeRange(call),
	}, true
}

// cppFirstStringArg returns the first string_literal argument (the route path).
func cppFirstStringArg(args *tree_sitter.Node, source []byte) (string, bool) {
	for i := uint(0); i < args.NamedChildCount(); i++ {
		a := args.NamedChild(i)
		if a == nil {
			continue
		}
		if a.Kind() == "string_literal" {
			return trimQuotes(a.Utf8Text(source)), true
		}
	}
	return "", false
}
