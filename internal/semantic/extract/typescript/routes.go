package tsextract

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// routes.go detects HTTP route registrations in TypeScript/JavaScript and
// synthesizes a RouteFact per (method, path, handler). Two shapes:
//   - Express/Fastify/Koa member calls: app.get("/p", h), router.post("/p", h)
//   - NestJS decorators on methods:     @Get("/p") handler() { ... }

// tsRouteMethods maps a lowercase callee name to the HTTP method it implies;
// "" = undetermined. Lookup is case-insensitive so Express "get" and NestJS
// "Get" both match. Presence marks the callee as a route registrar.
var tsRouteMethods = map[string]string{
	"get": "GET", "post": "POST", "put": "PUT", "delete": "DELETE",
	"patch": "PATCH", "head": "HEAD", "options": "OPTIONS", "use": "", "all": "",
}

func detectTSRoutes(root tree_sitter.Node, source []byte, filePath string) []extract.RouteFact {
	var routes []extract.RouteFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		switch n.Kind() {
		case "call_expression":
			if rf, ok := tsRouteFromMemberCall(n, source, filePath); ok {
				routes = append(routes, rf)
			}
		case "decorator":
			if rf, ok := tsRouteFromDecorator(n, source, filePath); ok {
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

// tsRouteFromMemberCall handles Express-style app.METHOD("/p", h).
func tsRouteFromMemberCall(call tree_sitter.Node, source []byte, filePath string) (extract.RouteFact, bool) {
	fn := call.ChildByFieldName("function")
	args := call.ChildByFieldName("arguments")
	if fn == nil || args == nil || fn.Kind() != "member_expression" {
		return extract.RouteFact{}, false
	}
	prop := fn.ChildByFieldName("property")
	if prop == nil {
		return extract.RouteFact{}, false
	}
	method, isRoute := tsRouteMethods[strings.ToLower(prop.Utf8Text(source))]
	if !isRoute {
		return extract.RouteFact{}, false
	}
	path, ok := tsFirstStringArg(args, source)
	if !ok {
		return extract.RouteFact{}, false
	}
	return extract.RouteFact{
		Language: "typescript",
		Method:   method,
		Path:     path,
		Handler:  tsHandlerName(args, 1, source),
		File:     filePath,
		Range:    nodeRange(call),
	}, true
}

// tsRouteFromDecorator handles NestJS-style @METHOD("/p") on a method.
func tsRouteFromDecorator(dec tree_sitter.Node, source []byte, filePath string) (extract.RouteFact, bool) {
	call := dec.NamedChild(0)
	if call == nil {
		return extract.RouteFact{}, false
	}
	if k := call.Kind(); k != "call_expression" && k != "call" {
		return extract.RouteFact{}, false
	}
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return extract.RouteFact{}, false
	}
	var method string
	var isRoute bool
	switch fn.Kind() {
	case "identifier":
		method, isRoute = tsRouteMethods[strings.ToLower(fn.Utf8Text(source))]
	case "member_expression":
		if prop := fn.ChildByFieldName("property"); prop != nil {
			method, isRoute = tsRouteMethods[strings.ToLower(prop.Utf8Text(source))]
		}
	default:
		return extract.RouteFact{}, false
	}
	if !isRoute {
		return extract.RouteFact{}, false
	}
	args := call.ChildByFieldName("arguments")
	if args == nil {
		return extract.RouteFact{}, false
	}
	path, ok := tsFirstStringArg(args, source)
	if !ok {
		return extract.RouteFact{}, false
	}
	// Decorators are siblings of method_definition inside class_body; the
	// decorated method is the first method_definition following this decorator.
	handler := ""
	if parent := dec.Parent(); parent != nil {
		decEnd := dec.EndPosition()
		for i := uint(0); i < parent.NamedChildCount(); i++ {
			c := parent.NamedChild(i)
			if c == nil || c.Kind() != "method_definition" {
				continue
			}
			start := c.StartPosition()
			if start.Row > decEnd.Row || (start.Row == decEnd.Row && start.Column >= decEnd.Column) {
				if name := c.ChildByFieldName("name"); name != nil {
					handler = name.Utf8Text(source)
				}
				break
			}
		}
	}
	return extract.RouteFact{
		Language: "typescript",
		Method:   method,
		Path:     path,
		Handler:  handler,
		File:     filePath,
		Range:    nodeRange(*call),
	}, true
}

// tsFirstStringArg returns the first positional argument if it is a string
// literal (route paths are string literals; a non-string first arg means this
// is not a route registration).
func tsFirstStringArg(args *tree_sitter.Node, source []byte) (string, bool) {
	a := args.NamedChild(0)
	if a == nil || a.Kind() != "string" {
		return "", false
	}
	return tsTrimString(a.Utf8Text(source)), true
}

func tsHandlerName(args *tree_sitter.Node, i int, source []byte) string {
	n := args.NamedChild(uint(i))
	if n == nil {
		return ""
	}
	switch n.Kind() {
	case "identifier":
		return n.Utf8Text(source)
	case "member_expression":
		if prop := n.ChildByFieldName("property"); prop != nil {
			return prop.Utf8Text(source)
		}
	}
	return ""
}

// tsTrimString strips surrounding quotes (", ', `) from a TS/JS string literal.
func tsTrimString(s string) string {
	if len(s) < 2 {
		return s
	}
	first, last := s[0], s[len(s)-1]
	if first == last && (first == '"' || first == '\'' || first == '`') {
		return s[1 : len(s)-1]
	}
	return s
}
