package rubyextract

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// routes.go detects HTTP route registrations in Ruby (Sinatra-style) and
// synthesizes a RouteFact per (method, path, handler). factsFromExtracted
// promotes each into a Route symbol plus a HANDLES edge.
//
// tree-sitter-ruby parses both Sinatra forms as a single `call` node whose
// `method` field is an identifier naming the HTTP verb and whose first
// argument is a string-literal path:
//   - get "/users" do ... end → call { method: get, arguments: ("/users"), block: do_block }
//   - get "/users"            → call { method: get, arguments: ("/users") }
//
// The handler is the block body, which is anonymous, so Handler stays "".
// Rails `get "/x", to: "ctrl#act"` is detected for free (same `call` shape).

// rubyRouteMethods maps a Sinatra verb identifier to its HTTP method.
// Lookup is case-insensitive so "get" and an unusual "Get" both match.
// Presence in the map marks the identifier as a route registrar.
var rubyRouteMethods = map[string]string{
	"get":    "GET",
	"post":   "POST",
	"put":    "PUT",
	"patch":  "PATCH",
	"delete": "DELETE",
}

func detectRubyRoutes(root tree_sitter.Node, source []byte, filePath string) []extract.RouteFact {
	var routes []extract.RouteFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "call" {
			if rf, ok := rubyRouteFromCall(n, source, filePath); ok {
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

// rubyRouteFromCall handles Sinatra-style verb "/path" [do ... end].
func rubyRouteFromCall(call tree_sitter.Node, source []byte, filePath string) (extract.RouteFact, bool) {
	methodNode := call.ChildByFieldName("method")
	if methodNode == nil || methodNode.Kind() != "identifier" {
		return extract.RouteFact{}, false
	}
	method, isRoute := rubyRouteMethods[strings.ToLower(methodNode.Utf8Text(source))]
	if !isRoute {
		return extract.RouteFact{}, false
	}
	args := call.ChildByFieldName("arguments")
	if args == nil {
		return extract.RouteFact{}, false
	}
	// The first argument must be a string literal (the route path).
	path, ok := rubyStringLiteral(args.NamedChild(0), source)
	if !ok {
		return extract.RouteFact{}, false
	}
	return extract.RouteFact{
		Language: "ruby",
		Method:   method,
		Path:     path,
		Handler:  "", // Sinatra route bodies are anonymous blocks.
		File:     filePath,
		Range:    nodeRange(call),
	}, true
}

// rubyStringLiteral returns the unquoted text of a Ruby string node. The
// grammar nests the raw content in a `string_content` child; fall back to
// stripping a single layer of surrounding quotes if that child is absent.
func rubyStringLiteral(str *tree_sitter.Node, source []byte) (string, bool) {
	if str == nil || str.Kind() != "string" {
		return "", false
	}
	for i := uint(0); i < str.NamedChildCount(); i++ {
		c := str.NamedChild(i)
		if c != nil && c.Kind() == "string_content" {
			return c.Utf8Text(source), true
		}
	}
	return rubyTrimString(str.Utf8Text(source)), true
}

// rubyTrimString strips one layer of surrounding quotes from a literal.
func rubyTrimString(s string) string {
	if len(s) < 2 {
		return s
	}
	first, last := s[0], s[len(s)-1]
	if first == last && (first == '"' || first == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}
