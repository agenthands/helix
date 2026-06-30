package kotlinextract

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// resources.go detects ORM / data-entity definitions in Kotlin source and
// synthesizes a ResourceFact per entity. factsFromExtracted promotes each into
// a synthetic Resource symbol. Two shapes:
//   - JPA: a class_declaration carrying an @Entity annotation
//     (jakarta.persistence.Entity / javax.persistence.Entity). The mapped
//     table name, if any, comes from an accompanying @Table(name="...").
//   - Jetbrains Exposed: a class_declaration whose delegation (superclass)
//     is an Exposed table type — Table, LongIdTable, IntIdTable, IdTable,
//     SizedIdTable, UUIDTable. The table name is the first positional
//     string argument of the Table(...) constructor when present.

// exposedTableTypes marks Exposed base classes that map a class to a table.
var exposedTableTypes = map[string]bool{
	"Table":          true,
	"LongIdTable":    true,
	"IntIdTable":     true,
	"IdTable":        true,
	"SizedIdTable":   true,
	"UUIDTable":      true,
	"CompositeTable": true,
}

func detectKotlinResources(root tree_sitter.Node, source []byte, filePath string) []extract.ResourceFact {
	var resources []extract.ResourceFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "class_declaration" {
			if rf, ok := kotlinResourceFromClass(n, source, filePath); ok {
				resources = append(resources, rf)
			}
		}
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if c := n.NamedChild(i); c != nil {
				walk(*c)
			}
		}
	}
	walk(root)
	return resources
}

// kotlinResourceFromClass classifies a single class_declaration as either a
// JPA @Entity or an Exposed table subclass. A plain class yields (zero, false).
func kotlinResourceFromClass(cls tree_sitter.Node, source []byte, filePath string) (extract.ResourceFact, bool) {
	name := kotlinClassName(cls, source)
	if name == "" {
		return extract.ResourceFact{}, false
	}

	// JPA: scan annotations in the modifiers for @Entity.
	mods := kotlinChildByKind(cls, "modifiers")
	if mods != nil {
		for i := uint(0); i < mods.NamedChildCount(); i++ {
			ann := mods.NamedChild(i)
			if ann == nil || ann.Kind() != "annotation" {
				continue
			}
			if kotlinAnnotationName(ann, source) == "Entity" {
				return extract.ResourceFact{
					Language: "kotlin",
					Name:     name,
					Table:    kotlinTableFromAnnotations(mods, source),
					ORM:      "jpa_kotlin",
					File:     filePath,
					Range:    nodeRange(cls),
				}, true
			}
		}
	}

	// Exposed: a delegation specifier whose type is an Exposed table type.
	if ds := kotlinChildByKind(cls, "delegation_specifiers"); ds != nil {
		for i := uint(0); i < ds.NamedChildCount(); i++ {
			child := ds.NamedChild(i)
			if child == nil {
				continue
			}
			typeName, ctor := kotlinDelegationType(child, source)
			if !exposedTableTypes[typeName] {
				continue
			}
			return extract.ResourceFact{
				Language: "kotlin",
				Name:     name,
				Table:    kotlinFirstStringArg(kotlinValueArguments(ctor), source),
				ORM:      "exposed",
				File:     filePath,
				Range:    nodeRange(cls),
			}, true
		}
	}

	return extract.ResourceFact{}, false
}

// kotlinClassName returns the declared name of a class_declaration (the "name"
// field identifier), falling back to the first direct identifier child.
func kotlinClassName(cls tree_sitter.Node, source []byte) string {
	if n := cls.ChildByFieldName("name"); n != nil {
		return strings.TrimSpace(n.Utf8Text(source))
	}
	for i := uint(0); i < cls.NamedChildCount(); i++ {
		if c := cls.NamedChild(i); c != nil && c.Kind() == "identifier" {
			return strings.TrimSpace(c.Utf8Text(source))
		}
	}
	return ""
}

// kotlinChildByKind returns the first direct named child of n whose Kind
// matches, or nil. Kotlin's grammar exposes modifiers / delegation_specifiers
// as node kinds rather than field names, so ChildByFieldName will not find them.
func kotlinChildByKind(n tree_sitter.Node, kind string) *tree_sitter.Node {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c != nil && c.Kind() == kind {
			return c
		}
	}
	return nil
}

// kotlinAnnotationName returns the simple (final) type segment of an
// annotation: @Entity -> "Entity", @Table(name="x") -> "Table",
// @jakarta.persistence.Entity -> "Entity".
func kotlinAnnotationName(ann *tree_sitter.Node, source []byte) string {
	if ann == nil {
		return ""
	}
	c := ann.NamedChild(0)
	if c == nil {
		return ""
	}
	if c.Kind() == "constructor_invocation" {
		c = c.NamedChild(0) // user_type
		if c == nil {
			return ""
		}
	}
	if c.Kind() != "user_type" {
		return ""
	}
	return kotlinLastSegment(c.Utf8Text(source))
}

// kotlinTableFromAnnotations extracts the name= string from a @Table(...)
// annotation among the class modifiers. Returns "" when absent.
func kotlinTableFromAnnotations(mods *tree_sitter.Node, source []byte) string {
	for i := uint(0); i < mods.NamedChildCount(); i++ {
		ann := mods.NamedChild(i)
		if ann == nil || ann.Kind() != "annotation" {
			continue
		}
		if kotlinAnnotationName(ann, source) != "Table" {
			continue
		}
		ctor := ann.NamedChild(0)
		if ctor == nil || ctor.Kind() != "constructor_invocation" {
			return ""
		}
		args := kotlinValueArguments(ctor)
		if args == nil {
			return ""
		}
		// Prefer a name= argument; fall back to the first positional string.
		if t, ok := kotlinNamedStringArg(args, "name", source); ok {
			return t
		}
		return kotlinFirstStringArg(args, source)
	}
	return ""
}

// kotlinDelegationType inspects a delegation_specifier node and returns its
// type name plus the constructor_invocation (for table-name extraction) when
// the delegation is a constructor call like Table("users").
func kotlinDelegationType(ds *tree_sitter.Node, source []byte) (string, *tree_sitter.Node) {
	if ds == nil {
		return "", nil
	}
	c := ds.NamedChild(0)
	if c == nil {
		return "", nil
	}
	if c.Kind() == "constructor_invocation" {
		ut := c.NamedChild(0)
		if ut == nil {
			return "", c
		}
		return kotlinLastSegment(ut.Utf8Text(source)), c
	}
	if c.Kind() == "user_type" {
		return kotlinLastSegment(c.Utf8Text(source)), nil
	}
	return "", nil
}

// kotlinValueArguments returns the value_arguments child of a
// constructor_invocation, or nil.
func kotlinValueArguments(n *tree_sitter.Node) *tree_sitter.Node {
	if n == nil {
		return nil
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c != nil && c.Kind() == "value_arguments" {
			return c
		}
	}
	return nil
}

// kotlinFirstStringArg returns the unquoted content of the first positional
// value_argument if it is a string literal, else "".
func kotlinFirstStringArg(args *tree_sitter.Node, source []byte) string {
	if args == nil {
		return ""
	}
	arg := args.NamedChild(0)
	if arg == nil || arg.Kind() != "value_argument" {
		return ""
	}
	expr := arg.NamedChild(0)
	if expr == nil || expr.Kind() != "string_literal" {
		return ""
	}
	return kotlinTrimString(expr.Utf8Text(source))
}

// kotlinNamedStringArg returns the unquoted content of the value_argument whose
// name identifier equals `field` (e.g. name = "users").
func kotlinNamedStringArg(args *tree_sitter.Node, field string, source []byte) (string, bool) {
	if args == nil {
		return "", false
	}
	for i := uint(0); i < args.NamedChildCount(); i++ {
		arg := args.NamedChild(i)
		if arg == nil || arg.Kind() != "value_argument" {
			continue
		}
		var (
			hasField bool
			value    *tree_sitter.Node
		)
		for j := uint(0); j < arg.NamedChildCount(); j++ {
			c := arg.NamedChild(j)
			if c == nil {
				continue
			}
			switch {
			case c.Kind() == "identifier":
				if c.Utf8Text(source) == field {
					hasField = true
				}
			case c.Kind() == "string_literal":
				value = c
			}
		}
		if hasField && value != nil {
			return kotlinTrimString(value.Utf8Text(source)), true
		}
	}
	return "", false
}

// kotlinLastSegment returns the final segment of a possibly-qualified type
// name (kotlin.Entity -> Entity). Whitespace is trimmed first.
func kotlinLastSegment(text string) string {
	text = strings.TrimSpace(text)
	if idx := strings.LastIndex(text, "."); idx >= 0 {
		text = text[idx+1:]
	}
	return text
}
