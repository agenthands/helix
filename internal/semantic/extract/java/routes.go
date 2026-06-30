package javaextract

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// routes.go detects HTTP route registrations in Java source from Spring Web
// mapping annotations on methods and synthesizes a RouteFact per
// (method, path, handler). factsFromExtracted promotes each into a Route
// symbol plus a HANDLES edge from the handler symbol.
//
// Annotations covered (heuristic — the path is taken from a value= named
// element when present, else the first positional string_literal argument):
//   - @GetMapping("/p"), @PostMapping("/p"), @PutMapping, @DeleteMapping,
//     @PatchMapping  → method fixed to the verb.
//   - @RequestMapping(value="/p") or @RequestMapping("/p")  → method
//     undetermined ("" — Spring allows the verb via method=, which the
//     annotation alone does not fix).
//
// A class-level @RequestMapping is a path PREFIX, not a handler route, so it
// is intentionally skipped: only annotations with an enclosing
// method_declaration are emitted.

// javaSpringMethods maps a Spring mapping annotation's simple name to the HTTP
// method it implies; "" = undetermined. Presence in the map marks the
// annotation as a route registrar.
var javaSpringMethods = map[string]string{
	"GetMapping":     "GET",
	"PostMapping":    "POST",
	"PutMapping":     "PUT",
	"DeleteMapping":  "DELETE",
	"PatchMapping":   "PATCH",
	"RequestMapping": "",
}

func detectJavaRoutes(root tree_sitter.Node, source []byte, filePath string) []extract.RouteFact {
	var routes []extract.RouteFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		switch n.Kind() {
		case "annotation", "marker_annotation":
			if rf, ok := javaRouteFromAnnotation(n, source, filePath); ok {
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

// javaRouteFromAnnotation extracts a route from a Spring mapping annotation.
func javaRouteFromAnnotation(ann tree_sitter.Node, source []byte, filePath string) (extract.RouteFact, bool) {
	nameNode := ann.ChildByFieldName("name")
	if nameNode == nil {
		return extract.RouteFact{}, false
	}
	method, isRoute := javaSpringMethods[javaAnnotationSimpleName(nameNode, source)]
	if !isRoute {
		return extract.RouteFact{}, false
	}
	// Only method-level mappings are routes; a class-level @RequestMapping is
	// a path prefix, not a handler.
	methodDecl := javaEnclosingMethod(&ann)
	if methodDecl == nil {
		return extract.RouteFact{}, false
	}
	handler := ""
	if name := methodDecl.ChildByFieldName("name"); name != nil {
		handler = name.Utf8Text(source)
	}
	return extract.RouteFact{
		Language: "java",
		Method:   method,
		Path:     javaAnnotationPath(&ann, source),
		Handler:  handler,
		File:     filePath,
		Range:    nodeRange(ann),
	}, true
}

// javaEnclosingMethod walks parents until it finds a method_declaration, or
// returns nil when the annotation is not on a method (e.g. class-level).
func javaEnclosingMethod(n *tree_sitter.Node) *tree_sitter.Node {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == "method_declaration" {
			return p
		}
	}
	return nil
}

// javaAnnotationSimpleName returns the simple (final) segment of an annotation
// name, handling bare identifier as well as scoped_identifier / field_access
// shapes (e.g. org.springframework.web.bind.annotation.GetMapping).
func javaAnnotationSimpleName(n *tree_sitter.Node, source []byte) string {
	text := n.Utf8Text(source)
	if i := strings.LastIndex(text, "."); i >= 0 {
		return text[i+1:]
	}
	return text
}

// javaAnnotationPath extracts the route path from an annotation: a value=
// named element is preferred, else the first positional string_literal arg.
// Returns "" when the annotation carries no path.
func javaAnnotationPath(ann *tree_sitter.Node, source []byte) string {
	args := ann.ChildByFieldName("arguments")
	if args == nil {
		return ""
	}
	var first string
	var found bool
	for i := uint(0); i < args.NamedChildCount(); i++ {
		c := args.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() == "element_value_pair" {
			key := c.ChildByFieldName("key")
			if key != nil && key.Utf8Text(source) == "value" {
				if v := c.ChildByFieldName("value"); v != nil && v.Kind() == "string_literal" {
					return trimQuotes(v.Utf8Text(source))
				}
			}
			continue
		}
		if c.Kind() == "string_literal" && !found {
			first = trimQuotes(c.Utf8Text(source))
			found = true
		}
	}
	return first
}
