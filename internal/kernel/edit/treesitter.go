// Package edit implements symbol editing tools: replace body, insert before/after,
// rename, safe delete, and post-edit diagnostic verification.
package edit

import (
	"fmt"
	"strings"
	"sync"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go" //nolint:importmismatch

	gen "github.com/postfix/serena/protocol/gen"
)

// langConfig holds tree-sitter metadata for a language.
type langConfig struct {
	language         *tree_sitter.Language
	declarationTypes map[string]bool // node types that are declarations
	bodyFieldName    string          // field name for the body child node
}

// BodyExtractor uses tree-sitter to precisely extract symbol body byte ranges.
// Per D-14: tree-sitter-first for replace-body operations.
type BodyExtractor struct {
	mu        sync.RWMutex
	languages map[string]*langConfig
}

// NewBodyExtractor creates a BodyExtractor with Go, Python, TypeScript, and Rust grammars.
func NewBodyExtractor() *BodyExtractor {
	be := &BodyExtractor{
		languages: make(map[string]*langConfig),
	}

	// Go: function_declaration, method_declaration -> body
	be.languages["go"] = &langConfig{
		language: tree_sitter.NewLanguage(tree_sitter_go.Language()),
		declarationTypes: map[string]bool{
			"function_declaration": true,
			"method_declaration":   true,
		},
		bodyFieldName: "body",
	}

	// Python: function_definition, class_definition -> body
	be.languages["python"] = &langConfig{
		language: tree_sitter.NewLanguage(tree_sitter_python.Language()),
		declarationTypes: map[string]bool{
			"function_definition": true,
			"class_definition":    true,
		},
		bodyFieldName: "body",
	}

	// TypeScript: function_declaration, method_definition, arrow_function -> body
	be.languages["typescript"] = &langConfig{
		language: tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript()),
		declarationTypes: map[string]bool{
			"function_declaration": true,
			"method_definition":    true,
			"arrow_function":       true,
		},
		bodyFieldName: "body",
	}

	// Rust: function_item, impl_item -> body
	be.languages["rust"] = &langConfig{
		language: tree_sitter.NewLanguage(tree_sitter_rust.Language()),
		declarationTypes: map[string]bool{
			"function_item": true,
			"impl_item":     true,
		},
		bodyFieldName: "body",
	}

	return be
}

// SupportsLanguage returns true if the extractor has a grammar for the given language.
func (be *BodyExtractor) SupportsLanguage(lang string) bool {
	be.mu.RLock()
	defer be.mu.RUnlock()
	_, ok := be.languages[lang]
	return ok
}

// ExtractBody parses source with the language grammar and returns the byte range
// of the body of the declaration that matches symbolName and overlaps symbolRange.
// Per D-14: tree-sitter-first for precise body surgery.
func (be *BodyExtractor) ExtractBody(source []byte, lang string, symbolName string, symbolRange gen.Range) (startByte, endByte uint, err error) {
	be.mu.RLock()
	cfg, ok := be.languages[lang]
	be.mu.RUnlock()
	if !ok {
		return 0, 0, fmt.Errorf("unsupported language for tree-sitter: %s", lang)
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(cfg.language); err != nil {
		return 0, 0, fmt.Errorf("set language %s: %w", lang, err)
	}

	tree := parser.Parse(source, nil)
	if tree == nil {
		return 0, 0, fmt.Errorf("tree-sitter parse failed for %s", lang)
	}
	defer tree.Close()

	root := tree.RootNode()
	decl := findDeclaration(root, source, symbolName, symbolRange, cfg)
	if decl == nil {
		return 0, 0, fmt.Errorf("declaration %q not found in tree-sitter AST", symbolName)
	}

	body := decl.ChildByFieldName(cfg.bodyFieldName)
	if body == nil {
		return 0, 0, fmt.Errorf("no %q field in %s node for %q", cfg.bodyFieldName, decl.Kind(), symbolName)
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
func extractNodeName(node *tree_sitter.Node, source []byte) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return ""
	}
	name := string(source[nameNode.StartByte():nameNode.EndByte()])
	// Strip receiver for Go methods: "(r *Type).Method" -> just the method name.
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}
