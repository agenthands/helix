package csharpextract

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// resources.go detects Entity Framework (EF Core) data-entity classes and
// synthesizes a ResourceFact per mapped class. factsFromExtracted promotes
// each into a synthetic Resource symbol node.
//
// An entity is a class_declaration that directly carries a mapping attribute:
//   - [Table("users")] class User {} → Table="users"
//   - [Entity]        class Product {} → Table="" (no explicit table name)
//
// Attribute-name and string-argument resolution reuse the same helpers as the
// route detector (csAttributeName, csAttributeArgs, csFirstStringArg) so
// qualified ([System.ComponentModel.DataAnnotations.Schema.Table]) and generic
// attribute forms collapse to their final identifier segment.

func detectCSharpResources(root tree_sitter.Node, source []byte, filePath string) []extract.ResourceFact {
	// A class may carry both [Table("x")] and [Entity]; index by class name and
	// prefer the entry that supplies an explicit table name.
	byName := make(map[string]extract.ResourceFact)
	var order []string
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "attribute" {
			if rf, ok := csResourceFromAttribute(n, source, filePath); ok {
				if existing, found := byName[rf.Name]; found {
					if existing.Table == "" && rf.Table != "" {
						byName[rf.Name] = rf
					}
				} else {
					byName[rf.Name] = rf
					order = append(order, rf.Name)
				}
			}
		}
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if c := n.NamedChild(i); c != nil {
				walk(*c)
			}
		}
	}
	walk(root)

	out := make([]extract.ResourceFact, 0, len(order))
	for _, name := range order {
		out = append(out, byName[name])
	}
	return out
}

// csResourceFromAttribute builds a ResourceFact when attr is a class-level
// [Table(...)] or [Entity] attribute. The bool is false for any other
// attribute name, or when the attribute is not directly decorating a
// class_declaration (e.g. it sits on a property or method).
func csResourceFromAttribute(attr tree_sitter.Node, source []byte, filePath string) (extract.ResourceFact, bool) {
	nameNode := attr.ChildByFieldName("name")
	if nameNode == nil {
		return extract.ResourceFact{}, false
	}
	name := csAttributeName(nameNode, source)

	var table string
	switch name {
	case "Table":
		if args := csAttributeArgs(attr); args != nil {
			if t, ok := csFirstStringArg(args, source); ok {
				table = t
			}
		}
	case "Entity":
		// no explicit table name
	default:
		return extract.ResourceFact{}, false
	}

	cls := csEnclosingClassName(attr, source)
	if cls == "" {
		return extract.ResourceFact{}, false
	}
	return extract.ResourceFact{
		Language: "c_sharp",
		Name:     cls,
		Table:    table,
		ORM:      "ef",
		File:     filePath,
		Range:    nodeRange(attr),
	}, true
}

// csEnclosingClassName returns the name of the class_declaration the attribute
// directly decorates, or "" when the attribute is not on a class. Directness is
// enforced by requiring the attribute's attribute_list parent to itself be a
// direct child of a class_declaration — a [Table] on a property or nested type
// is rejected so it never flags the enclosing class.
func csEnclosingClassName(attr tree_sitter.Node, source []byte) string {
	list := attr.Parent()
	if list == nil || list.Kind() != "attribute_list" {
		return ""
	}
	cls := list.Parent()
	if cls == nil || cls.Kind() != "class_declaration" {
		return ""
	}
	if nm := cls.ChildByFieldName("name"); nm != nil {
		return nm.Utf8Text(source)
	}
	return ""
}
