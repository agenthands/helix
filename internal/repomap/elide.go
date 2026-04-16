package repomap

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/postfix/serena/internal/treesitter"
)

// ElisionRenderer produces compact, token-efficient views of source files by
// showing definition signatures with bodies replaced by an ellipsis marker.
// Per D-13: full signature + ellipsis. Per D-14: struct/class fields shown.
// Per D-15: elision at render time using byte ranges from tags.
type ElisionRenderer struct {
	registry *treesitter.GrammarRegistry
}

// NewElisionRenderer creates an ElisionRenderer using the shared grammar registry.
func NewElisionRenderer(registry *treesitter.GrammarRegistry) *ElisionRenderer {
	return &ElisionRenderer{registry: registry}
}

// bodyFieldNames lists the field names tree-sitter uses for body/block children
// across different languages.
var bodyFieldNames = []string{"body", "block"}

// structLikeTypes are AST node types where we show fields but elide method bodies (D-14).
var structLikeTypes = map[string]bool{
	"type_declaration":   true, // Go type ... struct {}
	"struct_item":        true, // Rust
	"class_declaration":  true, // TypeScript
	"class_definition":   true, // Python
	"interface_type":     true, // Go interface
	"enum_item":          true, // Rust enum
	"trait_item":         true, // Rust trait
	"interface_declaration": true, // TypeScript
}

// declarationTypes maps AST node types that represent declarations we want to
// find when walking up from a tag's position.
var declarationTypes = map[string]bool{
	// Go
	"function_declaration": true,
	"method_declaration":   true,
	"type_declaration":     true,
	// Python
	"function_definition": true,
	"class_definition":    true,
	// TypeScript
	"function_declaration_ts": true, // alias; real name same as Go
	"method_definition":       true,
	"class_declaration":       true,
	"interface_declaration":   true,
	// Rust
	"function_item": true,
	"struct_item":   true,
	"enum_item":     true,
	"impl_item":     true,
	"trait_item":    true,
}

// RenderFile produces a compact elided view of the def tags in a file.
// Only def tags are rendered; ref tags are ignored. Bodies of functions/methods
// are replaced with an ellipsis marker. Struct/class fields are preserved (D-14).
func (r *ElisionRenderer) RenderFile(source []byte, lang string, tags []Tag) string {
	if len(source) == 0 {
		return ""
	}

	// Filter to def-only tags.
	defs := filterDefs(tags)
	if len(defs) == 0 {
		return ""
	}

	// Sort by StartByte ascending.
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].StartByte < defs[j].StartByte
	})

	// Try tree-sitter path if language is supported.
	tsLang, supported := r.registry.GetLanguage(lang)
	if supported {
		return r.renderWithTreeSitter(source, tsLang, lang, defs)
	}

	// Fallback: line-based rendering.
	return r.renderLineBased(source, defs)
}

// ElideSingle takes pre-computed body range and returns the elided text for a single tag.
// Useful for rendering individual tags without re-parsing.
func ElideSingle(source []byte, tag Tag, bodyStart, bodyEnd uint) string {
	if bodyStart == 0 && bodyEnd == 0 {
		// No body range: return the full tag text.
		if tag.EndByte <= uint(len(source)) {
			return string(source[tag.StartByte:tag.EndByte])
		}
		return ""
	}

	if bodyStart > uint(len(source)) || bodyEnd > uint(len(source)) {
		return ""
	}

	// Signature is everything before the body.
	sig := strings.TrimRight(string(source[tag.StartByte:bodyStart]), " \t")
	closing := closingForBody(source, bodyStart, bodyEnd)
	return sig + " { ... }" + closing
}

// renderWithTreeSitter parses source and renders each def tag with body elision.
func (r *ElisionRenderer) renderWithTreeSitter(source []byte, tsLang *tree_sitter.Language, lang string, defs []Tag) string {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tsLang); err != nil {
		// Fall back to line-based on error.
		return r.renderLineBased(source, defs)
	}

	tree := parser.Parse(source, nil)
	if tree == nil {
		return r.renderLineBased(source, defs)
	}
	defer tree.Close()

	root := tree.RootNode()
	var buf bytes.Buffer
	rendered := make(map[uint]bool) // track StartByte to avoid duplicates

	for _, tag := range defs {
		if rendered[tag.StartByte] {
			continue
		}
		rendered[tag.StartByte] = true

		// For tags without byte ranges, use line-based.
		if tag.StartByte == 0 && tag.EndByte == 0 {
			line := extractLine(source, tag.Line)
			if line != "" {
				if buf.Len() > 0 {
					buf.WriteByte('\n')
				}
				fmt.Fprintf(&buf, "// L%d:\n%s", tag.Line+1, line)
			}
			continue
		}

		text := r.elideTag(source, root, tag, lang)
		if text != "" {
			if buf.Len() > 0 {
				buf.WriteByte('\n')
			}
			fmt.Fprintf(&buf, "// L%d:\n%s", tag.Line+1, text)
		}
	}

	return buf.String()
}

// elideTag produces the elided text for a single def tag using tree-sitter.
func (r *ElisionRenderer) elideTag(source []byte, root *tree_sitter.Node, tag Tag, lang string) string {
	// Find the AST node at the tag's byte range.
	node := root.DescendantForByteRange(tag.StartByte, tag.EndByte)
	if node == nil {
		return fallbackTagText(source, tag)
	}

	// Walk up to find a declaration-type parent.
	decl := findDeclParent(node)
	if decl == nil {
		return fallbackTagText(source, tag)
	}

	// Check if this is a struct-like type (D-14: show fields, elide method bodies).
	if structLikeTypes[decl.Kind()] {
		return r.renderStructLike(source, decl, lang)
	}

	// Find body child.
	body := findBodyChild(decl)
	if body == nil {
		// No body (e.g., type alias, signature) -- show full text.
		return fallbackTagText(source, tag)
	}

	return elideBody(source, decl, body, lang)
}

// renderStructLike renders a struct/class/interface showing fields but eliding method bodies.
func (r *ElisionRenderer) renderStructLike(source []byte, decl *tree_sitter.Node, lang string) string {
	declStart := decl.StartByte()
	declEnd := decl.EndByte()

	if declEnd > uint(len(source)) {
		return ""
	}

	body := findBodyChild(decl)
	if body == nil {
		// No body -- show the whole declaration (e.g., empty struct).
		return string(source[declStart:declEnd])
	}

	// For Go structs, the body is the field_declaration_list which IS the type shape.
	// Show it entirely (fields are the signature per D-14).
	if lang == "go" {
		return string(source[declStart:declEnd])
	}

	// For Python classes, TypeScript classes, Rust structs/enums:
	// Show the header + iterate body children. Include field-like nodes,
	// elide function/method bodies within.
	var buf bytes.Buffer

	// Write everything from declaration start to body start (the header).
	headerEnd := body.StartByte()
	buf.Write(source[declStart:headerEnd])

	// Walk body's named children.
	for i := uint(0); i < body.NamedChildCount(); i++ {
		child := body.NamedChild(i)
		if child == nil {
			continue
		}
		childKind := child.Kind()

		// If this child is a function/method definition, elide its body.
		if isFunctionLike(childKind) {
			childBody := findBodyChild(child)
			if childBody != nil {
				sig := strings.TrimRight(string(source[child.StartByte():childBody.StartByte()]), " \t")
				closing := closingForBody(source, childBody.StartByte(), childBody.EndByte())
				buf.WriteString("\n    " + sig + " { ... }" + closing)
			} else {
				buf.WriteString("\n    " + string(source[child.StartByte():child.EndByte()]))
			}
		} else {
			// Field declarations, etc. -- show as-is.
			buf.WriteString("\n    " + strings.TrimSpace(string(source[child.StartByte():child.EndByte()])))
		}
	}

	// Close the body if it has a closing delimiter.
	lastByte := source[declEnd-1]
	if lastByte == '}' || lastByte == ')' {
		buf.WriteString("\n" + string(lastByte))
	}

	return buf.String()
}

// elideBody replaces a function/method body with an ellipsis marker.
func elideBody(source []byte, decl *tree_sitter.Node, body *tree_sitter.Node, lang string) string {
	declStart := decl.StartByte()
	bodyStart := body.StartByte()

	if bodyStart > uint(len(source)) {
		return ""
	}

	// The signature is everything from declaration start to body start.
	sig := strings.TrimRight(string(source[declStart:bodyStart]), " \t\n")

	// Python uses colon + indented body, not braces.
	if lang == "python" {
		return sig + ": ..."
	}

	return sig + " { ... }"
}

// renderLineBased renders tags using only line extraction (no tree-sitter).
func (r *ElisionRenderer) renderLineBased(source []byte, defs []Tag) string {
	var buf bytes.Buffer
	for _, tag := range defs {
		line := extractLine(source, tag.Line)
		if line != "" {
			if buf.Len() > 0 {
				buf.WriteByte('\n')
			}
			fmt.Fprintf(&buf, "// L%d:\n%s", tag.Line+1, line)
		}
	}
	return buf.String()
}

// filterDefs returns only def tags.
func filterDefs(tags []Tag) []Tag {
	var defs []Tag
	for _, t := range tags {
		if t.Kind == TagDef {
			defs = append(defs, t)
		}
	}
	return defs
}

// findDeclParent walks up the AST from node until it finds a declaration-type parent.
func findDeclParent(node *tree_sitter.Node) *tree_sitter.Node {
	current := node
	for current != nil {
		if declarationTypes[current.Kind()] || structLikeTypes[current.Kind()] {
			return current
		}
		p := current.Parent()
		if p == nil {
			break
		}
		current = p
	}
	return nil
}

// findBodyChild looks for a body or block field child on the node.
func findBodyChild(node *tree_sitter.Node) *tree_sitter.Node {
	for _, name := range bodyFieldNames {
		child := node.ChildByFieldName(name)
		if child != nil {
			return child
		}
	}
	return nil
}

// isFunctionLike returns true for AST node types that represent function/method definitions.
func isFunctionLike(kind string) bool {
	switch kind {
	case "function_declaration", "method_declaration", "function_definition",
		"method_definition", "function_item", "arrow_function":
		return true
	}
	return false
}

// closingForBody returns any closing characters after the body (e.g., trailing newline).
func closingForBody(source []byte, bodyStart, bodyEnd uint) string {
	// Check if there's content between body end and the next significant character.
	_ = source
	_ = bodyStart
	_ = bodyEnd
	return ""
}

// fallbackTagText returns the source text for a tag's byte range.
func fallbackTagText(source []byte, tag Tag) string {
	end := tag.EndByte
	if end > uint(len(source)) {
		end = uint(len(source))
	}
	start := tag.StartByte
	if start > end {
		return ""
	}
	return string(source[start:end])
}

// extractLine returns the content of a 0-indexed line number from source.
func extractLine(source []byte, line int) string {
	lines := bytes.Split(source, []byte("\n"))
	if line < 0 || line >= len(lines) {
		return ""
	}
	return string(bytes.TrimRight(lines[line], "\r"))
}
