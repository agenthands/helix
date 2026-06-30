package phpextract

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// resources.go detects ORM / data-entity definitions in PHP source and
// synthesizes a ResourceFact per entity. Three shapes:
//   - Doctrine attribute:   #[ORM\Entity] / #[Entity] / #[ORM\Mapping\Entity]
//   - Doctrine docblock:    /** @ORM\Entity */ / /** @Entity */
//   - Eloquent:             class Foo extends Model / ActiveRecord
//
// A plain class is not a resource. Table names, when present, are extracted
// best-effort from a #[ORM\Table(name="...")] attribute or a
// @ORM\Table(name="...") docblock tag.

func detectPhpResources(root tree_sitter.Node, source []byte, filePath string) []extract.ResourceFact {
	var resources []extract.ResourceFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "class_declaration" {
			if rf, ok := phpResourceFromClass(n, source, filePath); ok {
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

// phpResourceFromClass classifies a single class_declaration as a Doctrine
// entity (attribute or docblock) or an Eloquent model. Returns false for any
// other class.
func phpResourceFromClass(cls tree_sitter.Node, source []byte, filePath string) (extract.ResourceFact, bool) {
	name := phpClassName(cls, source)
	if name == "" {
		return extract.ResourceFact{}, false
	}

	// Doctrine attribute: scan attribute_list → attribute_group → attribute for
	// one whose final name segment is "Entity".
	if al := phpChildByKind(cls, "attribute_list"); al != nil {
		if attr := phpFindAttributeBySimpleName(al, source, "Entity"); attr != nil {
			return extract.ResourceFact{
				Language: "php",
				Name:     name,
				Table:    phpTableFromAttributes(al, source),
				ORM:      "doctrine",
				File:     filePath,
				Range:    nodeRange(cls),
			}, true
		}
	}

	// Doctrine docblock: the preceding named sibling is a comment with an
	// @Entity / @ORM\Entity annotation.
	if prev := cls.PrevNamedSibling(); prev != nil && prev.Kind() == "comment" {
		if phpDocBlockHasEntity(prev.Utf8Text(source)) {
			return extract.ResourceFact{
				Language: "php",
				Name:     name,
				Table:    phpTableFromDocBlock(prev.Utf8Text(source)),
				ORM:      "doctrine",
				File:     filePath,
				Range:    nodeRange(cls),
			}, true
		}
	}

	// Eloquent: base_clause names Model / ActiveRecord.
	if base := phpChildByKind(cls, "base_clause"); base != nil {
		if seg := phpBaseClassName(base, source); seg == "Model" || seg == "ActiveRecord" {
			return extract.ResourceFact{
				Language: "php",
				Name:     name,
				Table:    "",
				ORM:      "eloquent",
				File:     filePath,
				Range:    nodeRange(cls),
			}, true
		}
	}

	return extract.ResourceFact{}, false
}

// phpClassName returns the declared name of a class_declaration via the "name"
// field, falling back to the first direct (name) named child.
func phpClassName(cls tree_sitter.Node, source []byte) string {
	if n := cls.ChildByFieldName("name"); n != nil {
		return strings.TrimSpace(n.Utf8Text(source))
	}
	for i := uint(0); i < cls.NamedChildCount(); i++ {
		if c := cls.NamedChild(i); c != nil && c.Kind() == "name" {
			return strings.TrimSpace(c.Utf8Text(source))
		}
	}
	return ""
}

// phpChildByKind returns the first direct named child of n whose Kind matches,
// or nil. PHP exposes base_clause / attribute_list as node kinds.
func phpChildByKind(n tree_sitter.Node, kind string) *tree_sitter.Node {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c != nil && c.Kind() == kind {
			return c
		}
	}
	return nil
}

// phpFindAttributeBySimpleName walks an attribute_list for an attribute whose
// simple (final \-segment) name equals want (e.g. "Entity", "Table").
func phpFindAttributeBySimpleName(al *tree_sitter.Node, source []byte, want string) *tree_sitter.Node {
	for i := uint(0); i < al.NamedChildCount(); i++ {
		grp := al.NamedChild(i)
		if grp == nil || grp.Kind() != "attribute_group" {
			continue
		}
		for j := uint(0); j < grp.NamedChildCount(); j++ {
			attr := grp.NamedChild(j)
			if attr == nil || attr.Kind() != "attribute" {
				continue
			}
			if head := attr.NamedChild(0); head != nil {
				if phpLastSegment(head.Utf8Text(source)) == want {
					return attr
				}
			}
		}
	}
	return nil
}

// phpBaseClassName returns the simple (final \-segment) name of a base_clause's
// parent class.
func phpBaseClassName(base *tree_sitter.Node, source []byte) string {
	for i := uint(0); i < base.NamedChildCount(); i++ {
		c := base.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() == "name" || c.Kind() == "qualified_name" {
			return phpLastSegment(c.Utf8Text(source))
		}
	}
	return ""
}

// phpTableFromAttributes extracts the table name from a #[ORM\Table(...)]
// attribute in the class's attribute list. It scans the Table attribute's
// subtree for the first string literal (handles both name="x" and "x" forms).
func phpTableFromAttributes(al *tree_sitter.Node, source []byte) string {
	attr := phpFindAttributeBySimpleName(al, source, "Table")
	if attr == nil {
		return ""
	}
	var table string
	var walk func(n *tree_sitter.Node) bool
	walk = func(n *tree_sitter.Node) bool {
		if n == nil {
			return false
		}
		if n.Kind() == "string" || n.Kind() == "encapsed_string" {
			if t, ok := phpStringContent(n, source); ok {
				table = t
				return true
			}
		}
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if walk(n.NamedChild(i)) {
				return true
			}
		}
		return false
	}
	walk(attr)
	return table
}

// phpTableFromDocBlock extracts the table name from a @ORM\Table(name="...")
// docblock tag, if present.
func phpTableFromDocBlock(text string) string {
	// Match @ORM\Table(name="...") / @Table(name="...") with single or double quotes.
	for _, marker := range []string{`name="`, `name='`} {
		if i := strings.Index(text, marker); i >= 0 {
			rest := text[i+len(marker):]
			if j := strings.IndexAny(rest, "\"'"); j >= 0 {
				return rest[:j]
			}
		}
	}
	return ""
}

// phpDocBlockHasEntity reports whether a docblock comment carries a Doctrine
// @Entity annotation: @Entity, @ORM\Entity, @ORM\Mapping\Entity, or the fully
// qualified @Doctrine\ORM\Mapping\Entity. @Entities / @EntityRepository do not
// match (the bare form requires a word boundary after "Entity").
func phpDocBlockHasEntity(text string) bool {
	for _, m := range []string{
		`@ORM\Entity`,
		`@ORM\Mapping\Entity`,
		`@Doctrine\ORM\Mapping\Entity`,
	} {
		if strings.Contains(text, m) {
			return true
		}
	}
	if i := strings.Index(text, "@Entity"); i >= 0 {
		after := text[i+len("@Entity"):]
		if after == "" {
			return true
		}
		// Reject identifier continuations like @Entities / @EntityRepository.
		switch after[0] {
		case ' ', '\t', '\n', '\r', '(', '*', '/':
			return true
		}
	}
	return false
}

// phpLastSegment returns the final segment of a possibly namespaced PHP name
// (ORM\Entity -> Entity, \Model -> Model). Whitespace and a leading backslash
// are trimmed first.
func phpLastSegment(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, `\`)
	if idx := strings.LastIndex(text, `\`); idx >= 0 {
		text = text[idx+1:]
	}
	return text
}
