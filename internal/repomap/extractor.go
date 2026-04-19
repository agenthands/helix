package repomap

import (
	_ "embed"
	"fmt"
	"log"
	"regexp"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/treesitter"
)

//go:embed queries/go_tags.scm
var goTagsQuery string

//go:embed queries/python_tags.scm
var pythonTagsQuery string

//go:embed queries/typescript_tags.scm
var typescriptTagsQuery string

//go:embed queries/rust_tags.scm
var rustTagsQuery string

//go:embed queries/java_tags.scm
var javaTagsQuery string

//go:embed queries/c_tags.scm
var cTagsQuery string

//go:embed queries/cpp_tags.scm
var cppTagsQuery string

//go:embed queries/csharp_tags.scm
var csharpTagsQuery string

//go:embed queries/ruby_tags.scm
var rubyTagsQuery string

//go:embed queries/php_tags.scm
var phpTagsQuery string

//go:embed queries/javascript_tags.scm
var javascriptTagsQuery string

//go:embed queries/kotlin_tags.scm
var kotlinTagsQuery string

//go:embed queries/scala_tags.scm
var scalaTagsQuery string

//go:embed queries/bash_tags.scm
var bashTagsQuery string

//go:embed queries/haskell_tags.scm
var haskellTagsQuery string

//go:embed queries/julia_tags.scm
var juliaTagsQuery string

//go:embed queries/ocaml_tags.scm
var ocamlTagsQuery string

//go:embed queries/lua_tags.scm
var luaTagsQuery string

//go:embed queries/zig_tags.scm
var zigTagsQuery string

//go:embed queries/hcl_tags.scm
var hclTagsQuery string

//go:embed queries/r_tags.scm
var rTagsQuery string

//go:embed queries/swift_tags.scm
var swiftTagsQuery string

// TagExtractor extracts def/ref tags from source files using tree-sitter queries.
// Queries are compiled once per language and reused across files.
type TagExtractor struct {
	registry *treesitter.GrammarRegistry
	queries  map[string]*tree_sitter.Query
}

// NewTagExtractor creates a TagExtractor with compiled queries for all supported languages.
// Returns an error if any query fails to compile.
func NewTagExtractor(registry *treesitter.GrammarRegistry) (*TagExtractor, error) {
	querySources := map[string]string{
		"go":         goTagsQuery,
		"python":     pythonTagsQuery,
		"typescript": typescriptTagsQuery,
		"tsx":        typescriptTagsQuery,
		"rust":       rustTagsQuery,
		// Wave 1 languages
		"java":       javaTagsQuery,
		"c":          cTagsQuery,
		"cpp":        cppTagsQuery,
		"c_sharp":    csharpTagsQuery,
		"ruby":       rubyTagsQuery,
		"php":        phpTagsQuery,
		"javascript": javascriptTagsQuery,
		"kotlin":     kotlinTagsQuery,
		// Wave 2a languages
		"scala":   scalaTagsQuery,
		"bash":    bashTagsQuery,
		"haskell": haskellTagsQuery,
		"julia":   juliaTagsQuery,
		"ocaml":   ocamlTagsQuery,
		// Wave 2b languages
		"lua":   luaTagsQuery,
		"zig":   zigTagsQuery,
		"hcl":   hclTagsQuery,
		"r":     rTagsQuery,
		"swift": swiftTagsQuery,
	}

	queries := make(map[string]*tree_sitter.Query, len(querySources))
	for lang, src := range querySources {
		tsLang, ok := registry.GetLanguage(lang)
		if !ok {
			return nil, fmt.Errorf("grammar not found for language %q", lang)
		}
		q, qerr := tree_sitter.NewQuery(tsLang, src)
		if qerr != nil {
			// Error-tolerant: log warning and skip this language rather than failing all extraction.
			log.Printf("WARNING: tag query for %s failed to compile: %s (skipping)", lang, qerr.Message)
			continue
		}
		queries[lang] = q
	}

	return &TagExtractor{
		registry: registry,
		queries:  queries,
	}, nil
}

// Extract parses the given source and returns all def/ref tags for the specified language.
func (e *TagExtractor) Extract(source []byte, filePath string, lang string) ([]Tag, error) {
	query, ok := e.queries[lang]
	if !ok {
		return nil, serr.New(serr.Unsupported, "no tag query for language").WithDetail(lang)
	}

	tsLang, ok := e.registry.GetLanguage(lang)
	if !ok {
		return nil, serr.New(serr.Internal, "grammar not found for language").WithDetail(lang)
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tsLang); err != nil {
		return nil, serr.Wrap(serr.Internal, "set tree-sitter language", err).WithDetail(lang)
	}

	tree := parser.Parse(source, nil)
	if tree == nil {
		return nil, serr.New(serr.Internal, "tree-sitter parse failed").WithDetail(lang)
	}
	defer tree.Close()

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	captureNames := query.CaptureNames()
	var tags []Tag

	matches := cursor.Matches(query, tree.RootNode(), source)
	for m := matches.Next(); m != nil; m = matches.Next() {
		var nameText string
		var nameNode *tree_sitter.Node
		var kind TagKind
		var defNode *tree_sitter.Node
		var captureName string

		for _, capture := range m.Captures {
			cn := captureNames[capture.Index]
			node := capture.Node
			switch {
			case cn == "name":
				nameText = node.Utf8Text(source)
				nameNode = &node
			case strings.HasPrefix(cn, "definition."):
				kind = TagDef
				defNode = &node
				captureName = cn
			case strings.HasPrefix(cn, "reference."):
				kind = TagRef
				defNode = &node
				captureName = cn
			}
		}

		if nameText == "" || kind == "" || nameNode == nil || defNode == nil {
			continue
		}

		// Build qualified name for methods per D-03.
		qualName := buildQualifiedName(*nameNode, source, lang, captureName)
		if qualName != "" {
			nameText = qualName
		}

		tags = append(tags, Tag{
			Name:      nameText,
			Kind:      kind,
			File:      filePath,
			Line:      int(nameNode.StartPosition().Row),
			Column:    int(nameNode.StartPosition().Column),
			StartByte: defNode.StartByte(),
			EndByte:   defNode.EndByte(),
		})
	}

	return tags, nil
}

// Close releases all compiled queries.
func (e *TagExtractor) Close() {
	for _, q := range e.queries {
		q.Close()
	}
}

// receiverTypeRe extracts the type name from a Go receiver parameter like "(s *Server)" or "(s Server)".
var receiverTypeRe = regexp.MustCompile(`\*?(\w+)\s*\)`)

// buildQualifiedName produces a qualified "Type.method" name for method definitions.
// For non-method definitions, it returns "" (caller uses the raw name).
func buildQualifiedName(nameNode tree_sitter.Node, source []byte, lang string, captureName string) string {
	name := nameNode.Utf8Text(source)

	switch {
	case lang == "go" && captureName == "definition.method":
		return qualifyGoMethod(nameNode, source, name)
	case lang == "python" && captureName == "definition.function":
		return qualifyPythonMethod(nameNode, source, name)
	case (lang == "typescript" || lang == "tsx") && captureName == "definition.method":
		return qualifyTypeScriptMethod(nameNode, source, name)
	case lang == "rust" && captureName == "definition.function":
		return qualifyRustMethod(nameNode, source, name)
	case lang == "java" && captureName == "definition.method":
		return qualifyJavaMethod(nameNode, source, name)
	case lang == "c_sharp" && captureName == "definition.method":
		return qualifyCSharpMethod(nameNode, source, name)
	case lang == "ruby" && captureName == "definition.method":
		return qualifyRubyMethod(nameNode, source, name)
	case lang == "kotlin" && captureName == "definition.function":
		return qualifyKotlinFunction(nameNode, source, name)
	case lang == "javascript" && captureName == "definition.method":
		return qualifyJavaScriptMethod(nameNode, source, name)
	case lang == "scala" && captureName == "definition.function":
		return qualifyScalaFunction(nameNode, source, name)
	}
	return ""
}

// qualifyScalaFunction walks up to find if the function is inside a class/object definition.
func qualifyScalaFunction(nameNode tree_sitter.Node, source []byte, name string) string {
	funcDef := nameNode.Parent()
	if funcDef == nil || funcDef.Kind() != "function_definition" {
		return ""
	}
	// Walk up through template_body to class/object
	parent := funcDef.Parent()
	if parent == nil || parent.Kind() != "template_body" {
		return ""
	}
	container := parent.Parent()
	if container == nil {
		return ""
	}
	if container.Kind() != "class_definition" && container.Kind() != "object_definition" {
		return ""
	}
	containerNameNode := container.ChildByFieldName("name")
	if containerNameNode == nil {
		return ""
	}
	return containerNameNode.Utf8Text(source) + "." + name
}

// qualifyGoMethod walks up from the method name to find the receiver type.
// "func (s *Server) Run()" -> "Server.Run"
func qualifyGoMethod(nameNode tree_sitter.Node, source []byte, name string) string {
	parent := nameNode.Parent()
	if parent == nil || parent.Kind() != "method_declaration" {
		return ""
	}
	recv := parent.ChildByFieldName("receiver")
	if recv == nil {
		return ""
	}
	recvText := recv.Utf8Text(source)
	typeName := extractReceiverType(recvText)
	if typeName != "" {
		return typeName + "." + name
	}
	return ""
}

// extractReceiverType extracts the type name from a Go receiver parameter list.
// "(s *Server)" -> "Server", "(s Server)" -> "Server"
func extractReceiverType(recvText string) string {
	m := receiverTypeRe.FindStringSubmatch(recvText)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// qualifyPythonMethod walks up to find if the function is inside a class_definition.
func qualifyPythonMethod(nameNode tree_sitter.Node, source []byte, name string) string {
	// Walk up: identifier -> function_definition -> class_body -> class_definition
	funcDef := nameNode.Parent()
	if funcDef == nil || funcDef.Kind() != "function_definition" {
		return ""
	}
	classBody := funcDef.Parent()
	if classBody == nil || classBody.Kind() != "block" {
		return ""
	}
	classDef := classBody.Parent()
	if classDef == nil || classDef.Kind() != "class_definition" {
		return ""
	}
	classNameNode := classDef.ChildByFieldName("name")
	if classNameNode == nil {
		return ""
	}
	className := classNameNode.Utf8Text(source)
	return className + "." + name
}

// qualifyTypeScriptMethod walks up to find if the method is inside a class_declaration.
func qualifyTypeScriptMethod(nameNode tree_sitter.Node, source []byte, name string) string {
	// Walk up: property_identifier -> method_definition -> class_body -> class_declaration
	methodDef := nameNode.Parent()
	if methodDef == nil || methodDef.Kind() != "method_definition" {
		return ""
	}
	classBody := methodDef.Parent()
	if classBody == nil || classBody.Kind() != "class_body" {
		return ""
	}
	classDef := classBody.Parent()
	if classDef == nil || classDef.Kind() != "class_declaration" {
		return ""
	}
	classNameNode := classDef.ChildByFieldName("name")
	if classNameNode == nil {
		return ""
	}
	className := classNameNode.Utf8Text(source)
	return className + "." + name
}

// qualifyRustMethod walks up to find if the function is inside an impl_item.
func qualifyRustMethod(nameNode tree_sitter.Node, source []byte, name string) string {
	// Walk up: identifier -> function_item -> declaration_list -> impl_item
	funcItem := nameNode.Parent()
	if funcItem == nil || funcItem.Kind() != "function_item" {
		return ""
	}
	declList := funcItem.Parent()
	if declList == nil || declList.Kind() != "declaration_list" {
		return ""
	}
	implItem := declList.Parent()
	if implItem == nil || implItem.Kind() != "impl_item" {
		return ""
	}
	typeNode := implItem.ChildByFieldName("type")
	if typeNode == nil {
		return ""
	}
	typeName := typeNode.Utf8Text(source)
	return typeName + "." + name
}

// qualifyJavaMethod walks up: identifier -> method_declaration -> class_body -> class_declaration
func qualifyJavaMethod(nameNode tree_sitter.Node, source []byte, name string) string {
	methodDecl := nameNode.Parent()
	if methodDecl == nil || methodDecl.Kind() != "method_declaration" {
		return ""
	}
	classBody := methodDecl.Parent()
	if classBody == nil || classBody.Kind() != "class_body" {
		return ""
	}
	classDef := classBody.Parent()
	if classDef == nil || classDef.Kind() != "class_declaration" {
		return ""
	}
	classNameNode := classDef.ChildByFieldName("name")
	if classNameNode == nil {
		return ""
	}
	return classNameNode.Utf8Text(source) + "." + name
}

// qualifyCSharpMethod walks up: identifier -> method_declaration -> declaration_list -> class_declaration
func qualifyCSharpMethod(nameNode tree_sitter.Node, source []byte, name string) string {
	methodDecl := nameNode.Parent()
	if methodDecl == nil || methodDecl.Kind() != "method_declaration" {
		return ""
	}
	declList := methodDecl.Parent()
	if declList == nil || declList.Kind() != "declaration_list" {
		return ""
	}
	classDef := declList.Parent()
	if classDef == nil || classDef.Kind() != "class_declaration" {
		return ""
	}
	classNameNode := classDef.ChildByFieldName("name")
	if classNameNode == nil {
		return ""
	}
	return classNameNode.Utf8Text(source) + "." + name
}

// qualifyRubyMethod walks up to find if method is inside a class node.
func qualifyRubyMethod(nameNode tree_sitter.Node, source []byte, name string) string {
	method := nameNode.Parent()
	if method == nil || method.Kind() != "method" {
		return ""
	}
	// Ruby: method -> body_statement or class body -> class
	parent := method.Parent()
	for parent != nil {
		if parent.Kind() == "class" {
			classNameNode := parent.ChildByFieldName("name")
			if classNameNode != nil {
				return classNameNode.Utf8Text(source) + "." + name
			}
		}
		parent = parent.Parent()
	}
	return ""
}

// qualifyKotlinFunction walks up to find if function is inside a class_declaration.
func qualifyKotlinFunction(nameNode tree_sitter.Node, source []byte, name string) string {
	funcDecl := nameNode.Parent()
	if funcDecl == nil || funcDecl.Kind() != "function_declaration" {
		return ""
	}
	classBody := funcDecl.Parent()
	if classBody == nil || classBody.Kind() != "class_body" {
		return ""
	}
	classDef := classBody.Parent()
	if classDef == nil || classDef.Kind() != "class_declaration" {
		return ""
	}
	// Kotlin class name is an identifier child (not a named field in this grammar version)
	for i := uint(0); i < classDef.ChildCount(); i++ {
		child := classDef.Child(i)
		if child != nil && child.Kind() == "identifier" {
			return child.Utf8Text(source) + "." + name
		}
	}
	return ""
}

// qualifyJavaScriptMethod walks up: property_identifier -> method_definition -> class_body -> class/class_declaration
func qualifyJavaScriptMethod(nameNode tree_sitter.Node, source []byte, name string) string {
	methodDef := nameNode.Parent()
	if methodDef == nil || methodDef.Kind() != "method_definition" {
		return ""
	}
	classBody := methodDef.Parent()
	if classBody == nil || classBody.Kind() != "class_body" {
		return ""
	}
	classDef := classBody.Parent()
	if classDef == nil {
		return ""
	}
	if classDef.Kind() != "class_declaration" && classDef.Kind() != "class" {
		return ""
	}
	classNameNode := classDef.ChildByFieldName("name")
	if classNameNode == nil {
		return ""
	}
	return classNameNode.Utf8Text(source) + "." + name
}
