package rustextract

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// routes.go detects HTTP route registrations in Rust source and
// synthesizes a RouteFact per (method, path, handler). factsFromExtracted
// promotes each into a Route symbol + a HANDLES edge to the handler.
//
// Frameworks covered (heuristic — a string-literal path must be present):
//   - Rocket function attributes: #[get("/p")], #[post("/p")], #[put],
//     #[delete], #[patch]. The attribute is an outer attribute whose meta
//     is a macro-like call: verb("/path").
//   - Rocket #[route(METHOD, "/p")] form.

// rustRouteMethods maps a lowercase Rocket macro name to the HTTP method it
// implies. Presence in the map marks the attribute as a route registrar.
var rustRouteMethods = map[string]string{
	"get": "GET", "post": "POST", "put": "PUT", "delete": "DELETE", "patch": "PATCH",
}

func detectRustRoutes(root tree_sitter.Node, source []byte, filePath string) []extract.RouteFact {
	var routes []extract.RouteFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "attribute_item" {
			if rf, ok := rustRouteFromAttributeItem(n, source, filePath); ok {
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

// rustRouteFromAttributeItem inspects an #[verb("/path")] or
// #[route(METHOD, "/path")] outer attribute. It returns false for any
// attribute that is not an HTTP route registrar (cfg, derive, test, ...).
func rustRouteFromAttributeItem(item tree_sitter.Node, source []byte, filePath string) (extract.RouteFact, bool) {
	attr := item.NamedChild(0)
	if attr == nil || attr.Kind() != "attribute" {
		return extract.RouteFact{}, false
	}
	verb := rustAttrVerb(attr.NamedChild(0), source)
	tt := rustAttrTokenTree(attr)
	if verb == "" || tt == nil {
		return extract.RouteFact{}, false
	}
	var method, path string
	switch verb {
	case "route":
		method, path = rustRouteArgs(*tt, source)
	default:
		m, isRoute := rustRouteMethods[verb]
		if !isRoute {
			return extract.RouteFact{}, false
		}
		method = m
		path = rustFirstStringInTokenTree(*tt, source)
	}
	if path == "" {
		return extract.RouteFact{}, false
	}
	return extract.RouteFact{
		Language: "rust",
		Method:   method,
		Path:     path,
		Handler:  rustHandlerAfterAttribute(item, source),
		File:     filePath,
		Range:    nodeRange(item),
	}, true
}

// rustAttrVerb returns the lowercase macro name of an attribute (the
// identifier heading the attribute, e.g. "get"). It accepts a bare
// identifier and a scoped name (rocket::get -> "get").
func rustAttrVerb(nameNode *tree_sitter.Node, source []byte) string {
	if nameNode == nil {
		return ""
	}
	switch nameNode.Kind() {
	case "identifier":
		return strings.ToLower(nameNode.Utf8Text(source))
	case "scoped_identifier":
		var last string
		for i := uint(0); i < nameNode.NamedChildCount(); i++ {
			if c := nameNode.NamedChild(i); c != nil && c.Kind() == "identifier" {
				last = c.Utf8Text(source)
			}
		}
		return strings.ToLower(last)
	}
	return ""
}

// rustAttrTokenTree returns the attribute's token tree — the parenthesized
// arguments. It prefers the "arguments" field and falls back to scanning.
func rustAttrTokenTree(attr *tree_sitter.Node) *tree_sitter.Node {
	if tt := attr.ChildByFieldName("arguments"); tt != nil {
		return tt
	}
	for i := uint(0); i < attr.NamedChildCount(); i++ {
		if c := attr.NamedChild(i); c != nil && c.Kind() == "token_tree" {
			return c
		}
	}
	return nil
}

// rustFirstStringInTokenTree returns the first string literal inside a token
// tree, quotes stripped (the route path).
func rustFirstStringInTokenTree(tt tree_sitter.Node, source []byte) string {
	for i := uint(0); i < tt.NamedChildCount(); i++ {
		c := tt.NamedChild(i)
		if c != nil && c.Kind() == "string_literal" {
			return rustStripString(c.Utf8Text(source))
		}
	}
	return ""
}

// rustRouteArgs parses a #[route(METHOD, "/path")] token tree into
// (uppercased METHOD, path). METHOD is the first identifier and the path is
// the first string literal.
func rustRouteArgs(tt tree_sitter.Node, source []byte) (string, string) {
	var method, path string
	for i := uint(0); i < tt.NamedChildCount(); i++ {
		c := tt.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case "identifier":
			if method == "" {
				method = strings.ToUpper(c.Utf8Text(source))
			}
		case "string_literal":
			if path == "" {
				path = rustStripString(c.Utf8Text(source))
			}
		}
	}
	return method, path
}

// rustHandlerAfterAttribute resolves the handler name from the function_item
// that immediately follows the attribute_item among its siblings (Rocket
// attaches the attribute to the next function). Mirrors the NestJS decorator
// handler lookup in the TypeScript provider.
func rustHandlerAfterAttribute(item tree_sitter.Node, source []byte) string {
	parent := item.Parent()
	if parent == nil {
		return ""
	}
	itemEnd := item.EndPosition()
	for i := uint(0); i < parent.NamedChildCount(); i++ {
		c := parent.NamedChild(i)
		if c == nil || c.Kind() != "function_item" {
			continue
		}
		start := c.StartPosition()
		if start.Row > itemEnd.Row || (start.Row == itemEnd.Row && start.Column >= itemEnd.Column) {
			if name := c.ChildByFieldName("name"); name != nil {
				return name.Utf8Text(source)
			}
			return ""
		}
	}
	return ""
}

// rustStripString strips surrounding double quotes from a Rust string literal.
func rustStripString(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
