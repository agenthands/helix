package rustextract

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// resources.go detects Diesel / sqlx ORM entities in Rust source and
// synthesizes a ResourceFact per entity struct. A struct_item preceded by an
// outer #[derive(Queryable | Insertable | FromRow)] attribute is treated as a
// mapped database entity. factsFromExtracted promotes each into a synthetic
// Resource SymbolFact so the graph has typed data-entity nodes.
//
// The Diesel table! macro (which names the table) is a separate top-level
// item, so Table is left empty; the entity struct's own name is the resource
// name. Queryable / Insertable are Diesel derives; FromRow is the sqlx derive
// — all three map to the "diesel" ORM tag (sqlx has no slot in the closed
// ResourceFact.ORM enum). This mirrors the Rust route detector's
// attribute_item handling exactly.

// rustEntityDerives is the set of derive macros that mark a struct as an ORM
// entity. Presence of any one inside a #[derive(...)] preceding a struct_item
// qualifies it as a Resource; any other derive (Debug, Clone, Default, ...)
// does not.
var rustEntityDerives = map[string]bool{
	"Queryable":  true,
	"Insertable": true,
	"FromRow":    true,
}

func detectRustResources(root tree_sitter.Node, source []byte, filePath string) []extract.ResourceFact {
	var resources []extract.ResourceFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "attribute_item" {
			if rf, ok := rustResourceFromAttributeItem(n, source, filePath); ok {
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

// rustResourceFromAttributeItem inspects a #[derive(Queryable|Insertable|FromRow)]
// outer attribute. It returns false for any attribute that is not an entity
// derive (cfg, test, Debug, Clone, table_name, route, ...). On a match it
// resolves the following struct_item sibling to harvest the entity name.
func rustResourceFromAttributeItem(item tree_sitter.Node, source []byte, filePath string) (extract.ResourceFact, bool) {
	attr := item.NamedChild(0)
	if attr == nil || attr.Kind() != "attribute" {
		return extract.ResourceFact{}, false
	}
	if rustAttrVerb(attr.NamedChild(0), source) != "derive" {
		return extract.ResourceFact{}, false
	}
	tt := rustAttrTokenTree(attr)
	if tt == nil {
		return extract.ResourceFact{}, false
	}
	if !rustDeriveHasEntityTrait(*tt, source) {
		return extract.ResourceFact{}, false
	}
	name := rustStructAfterAttribute(item, source)
	if name == "" {
		return extract.ResourceFact{}, false
	}
	return extract.ResourceFact{
		Language: "rust",
		Name:     name,
		Table:    "", // Diesel table name lives in the separate table! macro
		ORM:      "diesel",
		File:     filePath,
		Range:    nodeRange(item),
	}, true
}

// rustDeriveHasEntityTrait reports whether a derive macro's token tree names
// any of the entity-marker traits (Queryable, Insertable, FromRow). The traits
// appear as identifier / type_identifier nodes inside the token tree, possibly
// nested (e.g. a path like diesel::QueryDsl would not match, but a bare
// Queryable trait name will).
func rustDeriveHasEntityTrait(tt tree_sitter.Node, source []byte) bool {
	if rustEntityDerives[tt.Utf8Text(source)] {
		return true
	}
	for i := uint(0); i < tt.NamedChildCount(); i++ {
		c := tt.NamedChild(i)
		if c != nil && rustDeriveHasEntityTrait(*c, source) {
			return true
		}
	}
	return false
}

// rustStructAfterAttribute resolves the struct name from the struct_item that
// immediately follows the attribute_item among its siblings — mirrors
// rustHandlerAfterAttribute for the entity case. Returns "" if no struct_item
// follows (e.g. a standalone derive on a function or module).
func rustStructAfterAttribute(item tree_sitter.Node, source []byte) string {
	parent := item.Parent()
	if parent == nil {
		return ""
	}
	itemEnd := item.EndPosition()
	for i := uint(0); i < parent.NamedChildCount(); i++ {
		c := parent.NamedChild(i)
		if c == nil || c.Kind() != "struct_item" {
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
