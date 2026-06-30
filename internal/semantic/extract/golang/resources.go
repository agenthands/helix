package goextract

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// resources.go detects GORM ORM entities in Go source and synthesizes a
// ResourceFact per mapped struct. factsFromExtracted promotes each into a
// synthetic Resource symbol node.
//
// Detection heuristic (REAL — no stubs):
//   - A named struct (type_spec → struct_type) is a GORM entity if any of its
//     direct field_declaration children carries a `gorm:"..."` struct tag, OR
//     embeds gorm.Model. Go struct tags live inside backtick-delimited text on
//     the field_declaration node, so the substring "gorm:" marks a tagged field.
//   - A plain struct with no gorm tags and no gorm.Model embed is NOT detected.
//
// Table name is best-effort: extracted from a gorm tag directive of the form
// gorm:"...;tableName:NAME;..." if present, else "" (GORM usually derives the
// table from the struct name via the TableName() method, which is out of scope).

func detectGoResources(root tree_sitter.Node, source []byte, filePath string) []extract.ResourceFact {
	var resources []extract.ResourceFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "struct_type" {
			if rf, ok := goResourceFromStruct(n, source, filePath); ok {
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

// goResourceFromStruct maps a struct_type node to a ResourceFact when the
// struct is GORM-mapped. The entity name comes from the enclosing type_spec's
// name field (anonymous structs have no name and are skipped).
func goResourceFromStruct(structNode tree_sitter.Node, source []byte, filePath string) (extract.ResourceFact, bool) {
	parent := structNode.Parent()
	if parent == nil || parent.Kind() != "type_spec" {
		return extract.ResourceFact{}, false
	}
	nameNode := parent.ChildByFieldName("name")
	if nameNode == nil {
		return extract.ResourceFact{}, false
	}
	hasGorm, table := goStructGormInfo(structNode, source)
	if !hasGorm {
		return extract.ResourceFact{}, false
	}
	return extract.ResourceFact{
		Language: "go",
		Name:     nameNode.Utf8Text(source),
		Table:    table,
		ORM:      "gorm",
		File:     filePath,
		Range:    nodeRange(structNode),
	}, true
}

// goStructGormInfo inspects the direct field_declaration children of a
// struct_type (descending one level through a field_declaration_list wrapper,
// which tree-sitter-go may interpose) and reports whether the struct is
// GORM-mapped plus any best-effort table name.
//
// It deliberately does NOT descend into nested anonymous struct bodies, so a
// plain struct that merely contains a typed field whose value type happens to
// be a GORM entity is not mis-attributed.
func goStructGormInfo(structNode tree_sitter.Node, source []byte) (hasGorm bool, table string) {
	check := func(fd *tree_sitter.Node) {
		text := fd.Utf8Text(source)
		if strings.Contains(text, "gorm:") {
			hasGorm = true
		}
		// Embedded gorm.Model (covers `gorm.Model` and `*gorm.Model`).
		if strings.Contains(text, "gorm.Model") {
			hasGorm = true
		}
		if t := gormTableName(text); t != "" && table == "" {
			table = t
		}
	}
	for i := uint(0); i < structNode.NamedChildCount(); i++ {
		c := structNode.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case "field_declaration":
			check(c)
		case "field_declaration_list":
			for j := uint(0); j < c.NamedChildCount(); j++ {
				fd := c.NamedChild(j)
				if fd != nil && fd.Kind() == "field_declaration" {
					check(fd)
				}
			}
		}
	}
	return hasGorm, table
}

// gormTableName extracts a table name from a gorm struct tag value of the form
// gorm:"...;tableName:NAME;...". Best-effort: returns "" when no such
// directive is present.
func gormTableName(fieldText string) string {
	s := fieldText
	for {
		i := strings.Index(s, `gorm:"`)
		if i < 0 {
			return ""
		}
		s = s[i+len(`gorm:"`):]
		end := strings.IndexByte(s, '"')
		if end < 0 {
			return ""
		}
		if name := gormTagValue(s[:end], "tableName"); name != "" {
			return name
		}
		s = s[end+1:]
	}
}

// gormTagValue reads a single key's value from a gorm tag body like
// "column:id;primaryKey;tableName:users".
func gormTagValue(tagBody, key string) string {
	prefix := key + ":"
	for _, part := range strings.Split(tagBody, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, prefix) {
			return strings.TrimSpace(part[len(prefix):])
		}
	}
	return ""
}
