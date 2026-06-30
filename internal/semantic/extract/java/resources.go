package javaextract

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// resources.go detects JPA entities in Java source — a class_declaration
// annotated @Entity (optionally @Table(name="x")) — and synthesizes a
// ResourceFact per entity. factsFromExtracted promotes each into a synthetic
// Resource symbol node so the graph has typed data-entity nodes.
//
// A class annotated only with a service/controller stereotype (e.g.
// @Service, @RestController) is NOT a resource: it carries no persistence
// mapping and is intentionally skipped.

func detectJavaResources(root tree_sitter.Node, source []byte, filePath string) []extract.ResourceFact {
	var resources []extract.ResourceFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "class_declaration" {
			if rf, ok := javaResourceFromClass(n, source, filePath); ok {
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

// javaResourceFromClass inspects a class_declaration's annotations. When
// @Entity is present it returns a ResourceFact; the table name is taken from a
// @Table(name="x") annotation if any, else "".
func javaResourceFromClass(cls tree_sitter.Node, source []byte, filePath string) (extract.ResourceFact, bool) {
	isEntity := false
	table := ""
	for _, ann := range javaClassAnnotations(&cls) {
		nameNode := ann.ChildByFieldName("name")
		if nameNode == nil {
			continue
		}
		switch javaAnnotationSimpleName(nameNode, source) {
		case "Entity":
			isEntity = true
		case "Table":
			table = javaTableName(ann, source)
		}
	}
	if !isEntity {
		return extract.ResourceFact{}, false
	}
	nameNode := cls.ChildByFieldName("name")
	if nameNode == nil {
		return extract.ResourceFact{}, false
	}
	return extract.ResourceFact{
		Language: "java",
		Name:     nameNode.Utf8Text(source),
		Table:    table,
		ORM:      "jpa",
		File:     filePath,
		Range:    nodeRange(cls),
	}, true
}

// javaClassAnnotations returns the annotation/marker_annotation nodes attached
// to a class_declaration. Annotations live inside an optional modifiers node;
// tree-sitter-java does not field-label it, so we match by kind.
func javaClassAnnotations(cls *tree_sitter.Node) []*tree_sitter.Node {
	var out []*tree_sitter.Node
	for i := uint(0); i < cls.NamedChildCount(); i++ {
		c := cls.NamedChild(i)
		if c == nil || c.Kind() != "modifiers" {
			continue
		}
		for j := uint(0); j < c.NamedChildCount(); j++ {
			if ann := c.NamedChild(j); ann != nil {
				k := ann.Kind()
				if k == "annotation" || k == "marker_annotation" {
					out = append(out, ann)
				}
			}
		}
	}
	return out
}

// javaTableName extracts the name= element of a @Table annotation, e.g.
// @Table(name="users") → "users". Returns "" when no name element is present.
func javaTableName(ann *tree_sitter.Node, source []byte) string {
	args := ann.ChildByFieldName("arguments")
	if args == nil {
		return ""
	}
	for i := uint(0); i < args.NamedChildCount(); i++ {
		c := args.NamedChild(i)
		if c == nil || c.Kind() != "element_value_pair" {
			continue
		}
		key := c.ChildByFieldName("key")
		if key == nil || key.Utf8Text(source) != "name" {
			continue
		}
		v := c.ChildByFieldName("value")
		if v == nil || v.Kind() != "string_literal" {
			continue
		}
		return trimQuotes(v.Utf8Text(source))
	}
	return ""
}
