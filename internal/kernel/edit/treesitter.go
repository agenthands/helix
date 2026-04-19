// Package edit implements symbol editing tools: replace body, insert before/after,
// rename, safe delete, and post-edit diagnostic verification.
package edit

import (
	"strings"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/treesitter"
	gen "github.com/postfix/serena/protocol/gen"
)

// langConfig holds edit-specific tree-sitter metadata for a language.
type langConfig struct {
	declarationTypes map[string]bool // node types that are declarations
	bodyFieldName    string          // field name for the body child node
	bodyNodeKind     string          // fallback: find body by node kind when field name not available
}

// BodyExtractor uses tree-sitter to precisely extract symbol body byte ranges.
// Per D-14: tree-sitter-first for replace-body operations.
type BodyExtractor struct {
	mu       sync.RWMutex
	registry *treesitter.GrammarRegistry
	configs  map[string]*langConfig
}

// NewBodyExtractor creates a BodyExtractor using the shared grammar registry.
func NewBodyExtractor(registry *treesitter.GrammarRegistry) *BodyExtractor {
	be := &BodyExtractor{
		registry: registry,
		configs:  make(map[string]*langConfig),
	}

	// Go: function_declaration, method_declaration -> body
	be.configs["go"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_declaration": true,
			"method_declaration":   true,
		},
		bodyFieldName: "body",
	}

	// Python: function_definition, class_definition -> body
	be.configs["python"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_definition": true,
			"class_definition":    true,
		},
		bodyFieldName: "body",
	}

	// TypeScript: function_declaration, method_definition, arrow_function -> body
	be.configs["typescript"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_declaration": true,
			"method_definition":    true,
			"arrow_function":       true,
		},
		bodyFieldName: "body",
	}

	// Rust: function_item, impl_item -> body
	be.configs["rust"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_item": true,
			"impl_item":     true,
		},
		bodyFieldName: "body",
	}

	// Java: method_declaration, constructor_declaration -> body
	be.configs["java"] = &langConfig{
		declarationTypes: map[string]bool{
			"method_declaration":      true,
			"constructor_declaration": true,
		},
		bodyFieldName: "body",
	}

	// C: function_definition -> body (compound_statement)
	be.configs["c"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_definition": true,
		},
		bodyFieldName: "body",
	}

	// C++: function_definition -> body
	be.configs["cpp"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_definition": true,
		},
		bodyFieldName: "body",
	}

	// C#: method_declaration, constructor_declaration -> body
	be.configs["c_sharp"] = &langConfig{
		declarationTypes: map[string]bool{
			"method_declaration":      true,
			"constructor_declaration": true,
		},
		bodyFieldName: "body",
	}

	// Ruby: method, singleton_method -> body
	be.configs["ruby"] = &langConfig{
		declarationTypes: map[string]bool{
			"method":           true,
			"singleton_method": true,
		},
		bodyFieldName: "body",
	}

	// PHP: function_definition, method_declaration -> body
	be.configs["php"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_definition": true,
			"method_declaration":  true,
		},
		bodyFieldName: "body",
	}

	// JavaScript: function_declaration, method_definition -> body
	be.configs["javascript"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_declaration": true,
			"method_definition":    true,
		},
		bodyFieldName: "body",
	}

	// Scala: function_definition -> body (block)
	be.configs["scala"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_definition": true,
		},
		bodyFieldName: "body",
	}

	// Bash: function_definition -> body (compound_statement)
	be.configs["bash"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_definition": true,
		},
		bodyFieldName: "body",
	}

	// Julia: function_definition -> block (unnamed child, not a named field)
	be.configs["julia"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_definition": true,
		},
		bodyFieldName: "body",
		bodyNodeKind:  "block",
	}

	// Lua: function_declaration -> body (block)
	be.configs["lua"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_declaration": true,
		},
		bodyFieldName: "body",
	}

	// Zig: function_declaration -> body (block)
	be.configs["zig"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_declaration": true,
		},
		bodyFieldName: "body",
	}

	// R: function_definition -> body
	be.configs["r"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_definition": true,
		},
		bodyFieldName: "body",
	}

	// Swift: function_declaration, class_declaration -> body
	be.configs["swift"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_declaration": true,
			"class_declaration":    true,
		},
		bodyFieldName: "body",
	}

	// Kotlin: function_declaration -> function_body (unnamed child, not a named field)
	be.configs["kotlin"] = &langConfig{
		declarationTypes: map[string]bool{
			"function_declaration": true,
		},
		bodyFieldName: "body",
		bodyNodeKind:  "function_body",
	}

	return be
}

// SupportsLanguage returns true if the extractor has a grammar for the given language.
func (be *BodyExtractor) SupportsLanguage(lang string) bool {
	be.mu.RLock()
	defer be.mu.RUnlock()
	_, ok := be.configs[lang]
	return ok && be.registry.SupportsLanguage(lang)
}

// ExtractBody parses source with the language grammar and returns the byte range
// of the body of the declaration that matches symbolName and overlaps symbolRange.
// Per D-14: tree-sitter-first for precise body surgery.
func (be *BodyExtractor) ExtractBody(source []byte, lang string, symbolName string, symbolRange gen.Range) (startByte, endByte uint, err error) {
	be.mu.RLock()
	cfg, ok := be.configs[lang]
	be.mu.RUnlock()
	if !ok {
		return 0, 0, serr.New(serr.Unsupported, "unsupported language for tree-sitter").WithDetail(lang)
	}

	tsLang, ok := be.registry.GetLanguage(lang)
	if !ok {
		return 0, 0, serr.New(serr.Unsupported, "unsupported language for tree-sitter").WithDetail(lang)
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tsLang); err != nil {
		return 0, 0, serr.Wrap(serr.Internal, "set tree-sitter language", err).WithDetail(lang)
	}

	tree := parser.Parse(source, nil)
	if tree == nil {
		return 0, 0, serr.New(serr.Internal, "tree-sitter parse failed").WithDetail(lang)
	}
	defer tree.Close()

	root := tree.RootNode()
	decl := findDeclaration(root, source, symbolName, symbolRange, cfg)
	if decl == nil {
		return 0, 0, serr.New(serr.NotFound, "declaration not found in tree-sitter AST").WithDetail(symbolName)
	}

	body := decl.ChildByFieldName(cfg.bodyFieldName)
	if body == nil && cfg.bodyNodeKind != "" {
		// Fallback: find body by node kind (e.g., Kotlin function_body is unnamed)
		for i := uint(0); i < decl.ChildCount(); i++ {
			child := decl.Child(i)
			if child != nil && child.Kind() == cfg.bodyNodeKind {
				body = child
				break
			}
		}
	}
	if body == nil {
		return 0, 0, serr.New(serr.NotFound, "body field not found in declaration node").WithDetail(symbolName)
	}

	return body.StartByte(), body.EndByte(), nil
}

// findDeclaration walks the AST to find a declaration node that:
// 1. Has a type in the declaration types set
// 2. Contains the symbol range
// 3. Has a name child matching symbolName
func findDeclaration(node *tree_sitter.Node, source []byte, symbolName string, symbolRange gen.Range, cfg *langConfig) *tree_sitter.Node {
	if node == nil {
		return nil
	}

	kind := node.Kind()

	if cfg.declarationTypes[kind] {
		// Check if this node encompasses the symbol range.
		if nodeContainsRange(node, symbolRange) {
			// Check if name matches.
			name := extractNodeName(node, source)
			if name == symbolName {
				return node
			}
		}
	}

	// Recurse into children.
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if result := findDeclaration(child, source, symbolName, symbolRange, cfg); result != nil {
			return result
		}
	}
	return nil
}

// nodeContainsRange checks if a tree-sitter node's range encompasses the LSP range.
func nodeContainsRange(node *tree_sitter.Node, r gen.Range) bool {
	sp := node.StartPosition()
	ep := node.EndPosition()

	// Node start must be at or before symbol range start.
	if sp.Row > uint(r.Start.Line) {
		return false
	}
	if sp.Row == uint(r.Start.Line) && sp.Column > uint(r.Start.Character) {
		return false
	}

	// Node end must be at or after symbol range end.
	if ep.Row < uint(r.End.Line) {
		return false
	}
	if ep.Row == uint(r.End.Line) && ep.Column < uint(r.End.Character) {
		return false
	}

	return true
}

// extractNodeName gets the name of a declaration node.
// It looks for a "name" field child, which is the standard tree-sitter convention.
// For C/C++ function_definition nodes, it traverses declarator -> function_declarator -> declarator.
func extractNodeName(node *tree_sitter.Node, source []byte) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		// C/C++ function_definition: name is at declarator -> function_declarator -> declarator (identifier)
		decl := node.ChildByFieldName("declarator")
		if decl != nil && decl.Kind() == "function_declarator" {
			innerDecl := decl.ChildByFieldName("declarator")
			if innerDecl != nil && (innerDecl.Kind() == "identifier" || innerDecl.Kind() == "field_identifier") {
				return innerDecl.Utf8Text(source)
			}
		}
		// Julia function_definition: name is at signature -> call_expression -> identifier
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && child.Kind() == "signature" {
				for j := uint(0); j < child.ChildCount(); j++ {
					gc := child.Child(j)
					if gc != nil && gc.Kind() == "call_expression" {
						for k := uint(0); k < gc.ChildCount(); k++ {
							ggc := gc.Child(k)
							if ggc != nil && ggc.Kind() == "identifier" {
								return ggc.Utf8Text(source)
							}
						}
					}
				}
			}
		}
		return ""
	}
	name := string(source[nameNode.StartByte():nameNode.EndByte()])
	// Strip receiver for Go methods: "(r *Type).Method" -> just the method name.
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}
