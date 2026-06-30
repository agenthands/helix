package goextract

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// routes.go detects HTTP route-registration call sites in Go source and
// synthesizes a RouteFact per (method, path, handler). factsFromExtracted
// promotes each into a Route symbol + a HANDLES edge to the handler.
//
// Frameworks covered (heuristic — first arg must be a string-literal path):
//   - net/http + gorilla + chi + custom mux: mux.HandleFunc("/p", h),
//     mux.Handle("/p", h)  → method undetermined (registered for all methods).
//   - gin + echo + chi method routers: r.GET("/p", h), r.POST("/p", h), ...

// goRouterMethods maps a receiver-method field name to the HTTP method it
// implies; "" means "all methods / undetermined". Presence in the map marks
// the field as a route-registration call.
var goRouterMethods = map[string]string{
	"GET": "GET", "POST": "POST", "PUT": "PUT", "DELETE": "DELETE",
	"PATCH": "PATCH", "HEAD": "HEAD", "OPTIONS": "OPTIONS",
	"Any": "", "ANY": "", "Handle": "", "HandleFunc": "",
}

func detectGoRoutes(root tree_sitter.Node, source []byte, filePath string) []extract.RouteFact {
	var routes []extract.RouteFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "call_expression" {
			if rf, ok := goRouteFromCall(n, source, filePath); ok {
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

func goRouteFromCall(call tree_sitter.Node, source []byte, filePath string) (extract.RouteFact, bool) {
	fn := call.ChildByFieldName("function")
	args := call.ChildByFieldName("arguments")
	if fn == nil || args == nil {
		return extract.RouteFact{}, false
	}
	var method string
	switch fn.Kind() {
	case "selector_expression":
		f := fn.ChildByFieldName("field")
		if f == nil {
			return extract.RouteFact{}, false
		}
		m, isRoute := goRouterMethods[f.Utf8Text(source)]
		if !isRoute {
			return extract.RouteFact{}, false
		}
		method = m
	case "identifier":
		m, isRoute := goRouterMethods[fn.Utf8Text(source)]
		if !isRoute {
			return extract.RouteFact{}, false
		}
		method = m
	default:
		return extract.RouteFact{}, false
	}
	// First argument must be a string literal (the route path).
	path, ok := goRouteStringArg(args, 0, source)
	if !ok {
		return extract.RouteFact{}, false
	}
	return extract.RouteFact{
		Language: "go",
		Method:   method,
		Path:     path,
		Handler:  goRouteHandlerName(args, 1, source),
		File:     filePath,
		Range:    nodeRange(call),
	}, true
}

func goRouteStringArg(args *tree_sitter.Node, i int, source []byte) (string, bool) {
	n := args.NamedChild(uint(i))
	if n == nil || n.Kind() != "interpreted_string_literal" {
		return "", false
	}
	return trimQuotes(n.Utf8Text(source)), true
}

func goRouteHandlerName(args *tree_sitter.Node, i int, source []byte) string {
	n := args.NamedChild(uint(i))
	if n == nil {
		return ""
	}
	switch n.Kind() {
	case "identifier":
		return n.Utf8Text(source)
	case "selector_expression":
		if f := n.ChildByFieldName("field"); f != nil {
			return f.Utf8Text(source)
		}
	}
	return ""
}
