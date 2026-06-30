package pyextract

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// routes.go detects HTTP route-registration decorators on Python functions
// (Flask @app.route, FastAPI @app.get/@router.post, ...) and synthesizes a
// RouteFact per (method, path, handler).
//
// tree-sitter-python: a decorated_definition wraps one or more decorator
// nodes plus a function_definition. Each route decorator is a call whose
// function is an attribute (app.route / app.get / router.post / ...).

// pyRouteAttrs maps the trailing attribute name to the HTTP method it implies;
// "" means undetermined (Flask @app.route — method comes from methods= kwarg).
var pyRouteAttrs = map[string]string{
	"route": "", "get": "GET", "post": "POST", "put": "PUT",
	"delete": "DELETE", "patch": "PATCH", "head": "HEAD",
	"options": "OPTIONS", "websocket": "", "api_route": "",
}

func detectPyRoutes(root tree_sitter.Node, source []byte, filePath string) []extract.RouteFact {
	var routes []extract.RouteFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "decorated_definition" {
			routes = append(routes, pyRoutesFromDecorated(n, source, filePath)...)
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

func pyRoutesFromDecorated(dd tree_sitter.Node, source []byte, filePath string) []extract.RouteFact {
	handler := ""
	for i := uint(0); i < dd.NamedChildCount(); i++ {
		c := dd.NamedChild(i)
		if c != nil && c.Kind() == "function_definition" {
			if name := c.ChildByFieldName("name"); name != nil {
				handler = name.Utf8Text(source)
			}
		}
	}
	var routes []extract.RouteFact
	for i := uint(0); i < dd.NamedChildCount(); i++ {
		c := dd.NamedChild(i)
		if c == nil || c.Kind() != "decorator" {
			continue
		}
		call := c.NamedChild(0)
		if call == nil || call.Kind() != "call" {
			continue
		}
		fn := call.ChildByFieldName("function")
		if fn == nil || fn.Kind() != "attribute" {
			continue
		}
		attr := fn.ChildByFieldName("attribute")
		if attr == nil {
			continue
		}
		method, isRoute := pyRouteAttrs[attr.Utf8Text(source)]
		if !isRoute {
			continue
		}
		args := call.ChildByFieldName("arguments")
		if args == nil {
			continue
		}
		path, ok := pyFirstStringArg(args, source)
		if !ok {
			continue
		}
		routes = append(routes, extract.RouteFact{
			Language: "python",
			Method:   method,
			Path:     path,
			Handler:  handler,
			File:     filePath,
			Range:    nodeRange(*call),
		})
	}
	return routes
}

// pyFirstStringArg returns the text of the first positional string argument,
// stopping at the first keyword_argument.
func pyFirstStringArg(args *tree_sitter.Node, source []byte) (string, bool) {
	for i := uint(0); i < args.NamedChildCount(); i++ {
		a := args.NamedChild(i)
		if a == nil {
			continue
		}
		switch a.Kind() {
		case "string":
			return pyTrimString(a.Utf8Text(source)), true
		case "keyword_argument":
			return "", false
		}
	}
	return "", false
}

// pyTrimString strips string prefixes (f/r/b/u) and surrounding quotes,
// handling single, double, and triple-quoted forms.
func pyTrimString(s string) string {
	for len(s) > 0 && strings.IndexByte("frbuFRBU", s[0]) >= 0 {
		s = s[1:]
	}
	switch {
	case len(s) >= 6 && strings.HasPrefix(s, `"""`) && strings.HasSuffix(s, `"""`):
		return s[3 : len(s)-3]
	case len(s) >= 6 && strings.HasPrefix(s, `'''`) && strings.HasSuffix(s, `'''`):
		return s[3 : len(s)-3]
	case len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"':
		return s[1 : len(s)-1]
	case len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'':
		return s[1 : len(s)-1]
	}
	return s
}
