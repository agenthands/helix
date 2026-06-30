package csharpextract

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// routes.go detects ASP.NET route attributes on methods and synthesizes a
// RouteFact per (method, path, handler). Two attribute families:
//   - HTTP verb attributes: [HttpGet("/p")], [HttpPost("/p")],
//     [HttpPut], [HttpDelete], [HttpPatch], ... → method is the verb.
//   - [Route("/p")] → method is "" (the framework does not fix one here).
//
// The path is taken from the first string-literal attribute argument when
// present, else "". The handler is the name of the enclosing
// method_declaration (attributes attach to the method, so walking up via
// Parent() reaches it).

// csHttpVerbs maps an ASP.NET HTTP-verb attribute name to its HTTP method.
// Presence marks the attribute as a verb route registrar.
var csHttpVerbs = map[string]string{
	"HttpGet":     "GET",
	"HttpPost":    "POST",
	"HttpPut":     "PUT",
	"HttpDelete":  "DELETE",
	"HttpPatch":   "PATCH",
	"HttpHead":    "HEAD",
	"HttpOptions": "OPTIONS",
}

func detectCSharpRoutes(root tree_sitter.Node, source []byte, filePath string) []extract.RouteFact {
	var routes []extract.RouteFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "attribute" {
			if rf, ok := csRouteFromAttribute(n, source, filePath); ok {
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

// csRouteFromAttribute builds a RouteFact from an attribute node whose name is
// an HTTP-verb attribute (HttpGet/...) or Route. The bool is false for any
// other attribute ([ApiController], [Authorize], ...).
func csRouteFromAttribute(attr tree_sitter.Node, source []byte, filePath string) (extract.RouteFact, bool) {
	nameNode := attr.ChildByFieldName("name")
	if nameNode == nil {
		return extract.RouteFact{}, false
	}
	name := csAttributeName(nameNode, source)

	var method string
	if m, ok := csHttpVerbs[name]; ok {
		method = m
	} else if name != "Route" {
		return extract.RouteFact{}, false
	}

	path := ""
	if args := csAttributeArgs(attr); args != nil {
		if p, ok := csFirstStringArg(args, source); ok {
			path = p
		}
	}

	return extract.RouteFact{
		Language: "c_sharp",
		Method:   method,
		Path:     path,
		Handler:  csEnclosingMethodName(attr, source),
		File:     filePath,
		Range:    nodeRange(attr),
	}, true
}

// csAttributeName resolves an attribute's name node to its final identifier
// segment, so [HttpGet], [Microsoft.AspNetCore.Mvc.HttpGet], and
// [HttpGet<string>] all reduce to "HttpGet".
func csAttributeName(n *tree_sitter.Node, source []byte) string {
	if n == nil {
		return ""
	}
	if n.Kind() == "identifier" {
		return n.Utf8Text(source)
	}
	// qualified_name / generic_name / alias_qualified_name: take the trailing
	// identifier segment from the raw text.
	text := n.Utf8Text(source)
	if i := strings.Index(text, "<"); i >= 0 {
		text = text[:i] // strip generic arguments
	}
	if i := strings.LastIndex(text, "."); i >= 0 {
		text = text[i+1:] // keep the final dotted segment
	}
	return text
}

// csAttributeArgs returns the attribute_argument_list child of an attribute,
// or nil when the attribute has no arguments (e.g. [HttpPost]).
func csAttributeArgs(attr tree_sitter.Node) *tree_sitter.Node {
	for i := uint(0); i < attr.NamedChildCount(); i++ {
		if c := attr.NamedChild(i); c != nil && c.Kind() == "attribute_argument_list" {
			return c
		}
	}
	return nil
}

// csFirstStringArg returns the first string-literal attribute argument with its
// quotes stripped, e.g. [HttpGet("/users")] → "/users". A non-string first
// argument yields (..., false) so the caller records the route with path "".
func csFirstStringArg(args *tree_sitter.Node, source []byte) (string, bool) {
	for i := uint(0); i < args.NamedChildCount(); i++ {
		arg := args.NamedChild(i)
		if arg == nil || arg.Kind() != "attribute_argument" {
			continue
		}
		for j := uint(0); j < arg.NamedChildCount(); j++ {
			c := arg.NamedChild(j)
			if c != nil && c.Kind() == "string_literal" {
				return csTrimString(c.Utf8Text(source)), true
			}
		}
	}
	return "", false
}

// csEnclosingMethodName walks up from an attribute to its enclosing
// method_declaration and returns the method's name, or "" when the attribute
// is not on a method (e.g. a class-level [Route] prefix).
func csEnclosingMethodName(attr tree_sitter.Node, source []byte) string {
	for p := attr.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == "method_declaration" {
			if nm := p.ChildByFieldName("name"); nm != nil {
				return nm.Utf8Text(source)
			}
			return ""
		}
	}
	return ""
}

// csTrimString strips the surrounding quotes from a C# string literal,
// handling regular ("/p"), verbatim (@"/p"), and interpolated ($"/p") forms.
func csTrimString(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "$@\"") && len(s) >= 4 && strings.HasSuffix(s, "\""):
		return s[3 : len(s)-1]
	case strings.HasPrefix(s, "@\"") && len(s) >= 3 && strings.HasSuffix(s, "\""):
		return s[2 : len(s)-1]
	case strings.HasPrefix(s, "$\"") && len(s) >= 3 && strings.HasSuffix(s, "\""):
		return s[2 : len(s)-1]
	case len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"':
		return s[1 : len(s)-1]
	}
	return s
}
