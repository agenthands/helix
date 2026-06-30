package tsextract

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// resources.go detects TypeORM entity definitions in TypeScript/JavaScript
// source and synthesizes a ResourceFact per entity. Shape:
//   - @Entity() / @Entity("table") on a class_declaration
//     → {Name: class name, Table: @Entity string arg or "", ORM: "typeorm"}

// detectTSResources walks the parse tree for class-level @Entity decorators
// and emits one ResourceFact per decorated class_declaration.
func detectTSResources(root tree_sitter.Node, source []byte, filePath string) []extract.ResourceFact {
	var resources []extract.ResourceFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "decorator" {
			if rf, ok := tsResourceFromDecorator(n, source, filePath); ok {
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

// tsResourceFromDecorator handles TypeORM @Entity() / @Entity("table") on a
// class_declaration. Returns the entity fact plus ok; ok is false for any
// decorator whose callee is not "Entity" or which decorates no class.
func tsResourceFromDecorator(dec tree_sitter.Node, source []byte, filePath string) (extract.ResourceFact, bool) {
	if tsDecoratorCallee(&dec, source) != "Entity" {
		return extract.ResourceFact{}, false
	}
	table := tsDecoratorFirstStringArg(&dec, source)
	cls := tsDecoratedClass(dec, "class_declaration")
	if cls == nil {
		return extract.ResourceFact{}, false
	}
	name := ""
	if nm := cls.ChildByFieldName("name"); nm != nil {
		name = nm.Utf8Text(source)
	}
	return extract.ResourceFact{
		Language: "typescript",
		Name:     name,
		Table:    table,
		ORM:      "typeorm",
		File:     filePath,
		Range:    nodeRange(*cls),
	}, true
}

// tsDecoratorCallee returns the identifier the decorator applies to: for
// @Foo(...) the call's function identifier, for a bare @Foo the identifier
// itself. Returns "" when it cannot be determined.
func tsDecoratorCallee(dec *tree_sitter.Node, source []byte) string {
	c := dec.NamedChild(0)
	if c == nil {
		return ""
	}
	switch c.Kind() {
	case "call_expression", "call":
		if fn := c.ChildByFieldName("function"); fn != nil {
			return fn.Utf8Text(source)
		}
	case "identifier":
		return c.Utf8Text(source)
	}
	return ""
}

// tsDecoratorFirstStringArg returns the first positional argument if it is a
// string literal (TypeORM table names are string literals), else "".
func tsDecoratorFirstStringArg(dec *tree_sitter.Node, source []byte) string {
	c := dec.NamedChild(0)
	if c == nil || (c.Kind() != "call_expression" && c.Kind() != "call") {
		return ""
	}
	args := c.ChildByFieldName("arguments")
	if args == nil {
		return ""
	}
	a := args.NamedChild(0)
	if a == nil || a.Kind() != "string" {
		return ""
	}
	return tsTrimString(a.Utf8Text(source))
}

// tsDecoratedClass resolves the node a decorator decorates. A class-level
// decorator is either attached (a named child of the class_declaration) or a
// sibling preceding it (mirroring the position-based lookup in
// tsRouteFromDecorator). Returns nil when no decorated node is found.
func tsDecoratedClass(dec tree_sitter.Node, kind string) *tree_sitter.Node {
	parent := dec.Parent()
	if parent == nil {
		return nil
	}
	// Attached: the decorator is a named child of the decorated node itself.
	if parent.Kind() == kind {
		return parent
	}
	// Sibling: the decorated node is the first <kind> child of the parent at
	// or after the decorator's end position.
	decEnd := dec.EndPosition()
	for i := uint(0); i < parent.NamedChildCount(); i++ {
		c := parent.NamedChild(i)
		if c == nil || c.Kind() != kind {
			continue
		}
		start := c.StartPosition()
		if start.Row > decEnd.Row || (start.Row == decEnd.Row && start.Column >= decEnd.Column) {
			return c
		}
	}
	return nil
}
