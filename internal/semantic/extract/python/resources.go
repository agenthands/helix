package pyextract

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// resources.go detects SQLAlchemy ORM-entity definitions in Python source
// and synthesizes a ResourceFact per mapped class. factsFromExtracted
// promotes each into a synthetic Resource symbol so the graph has typed
// data-entity nodes alongside Route nodes.
//
// SQLAlchemy declarative entities carry a class-level `__tablename__`
// assignment, e.g.
//
//	class User(Base):
//	    __tablename__ = "users"
//	    id = Column(Integer, primary_key=True)
//
// The presence of a `__tablename__` assignment is the canonical, reliable
// signal that a class is a mapped table — it is the SQLAlchemy declaration
// contract. A plain class without it is NOT a resource, which keeps every
// ordinary class (and every Base subclass that forgot its table name) out
// of the false-positive set.

func detectPyResources(root tree_sitter.Node, source []byte, filePath string) []extract.ResourceFact {
	var resources []extract.ResourceFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "class_definition" {
			if rf, ok := pyResourceFromClass(n, source, filePath); ok {
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

func pyResourceFromClass(cls tree_sitter.Node, source []byte, filePath string) (extract.ResourceFact, bool) {
	nameNode := cls.ChildByFieldName("name")
	if nameNode == nil {
		return extract.ResourceFact{}, false
	}
	table, ok := pyTableFromTablename(cls, source)
	if !ok {
		return extract.ResourceFact{}, false
	}
	return extract.ResourceFact{
		Language: "python",
		Name:     nameNode.Utf8Text(source),
		Table:    table,
		ORM:      "sqlalchemy",
		File:     filePath,
		Range:    nodeRange(cls),
	}, true
}

// pyTableFromTablename scans a class body for an `__tablename__` assignment
// and returns (table-name, true) when one is present. The table name is the
// string literal on the assignment's right side (quotes stripped); if the RHS
// is not a string literal the table name is "" but the class is still a
// mapped resource (the presence of __tablename__ is what matters).
func pyTableFromTablename(cls tree_sitter.Node, source []byte) (string, bool) {
	body := cls.ChildByFieldName("body")
	if body == nil {
		return "", false
	}
	for i := uint(0); i < body.NamedChildCount(); i++ {
		stmt := body.NamedChild(i)
		if stmt == nil {
			continue
		}
		// tree-sitter-python wraps every statement in an expression_statement;
		// the assignment is its single named child. Unwrap defensively.
		assign := stmt
		if assign.Kind() == "expression_statement" {
			if c := assign.NamedChild(0); c != nil {
				assign = c
			}
		}
		if assign.Kind() != "assignment" {
			continue
		}
		left := assign.ChildByFieldName("left")
		if left == nil || left.Kind() != "identifier" || left.Utf8Text(source) != "__tablename__" {
			continue
		}
		right := assign.ChildByFieldName("right")
		if right != nil && right.Kind() == "string" {
			return pyTrimString(right.Utf8Text(source)), true
		}
		return "", true // __tablename__ present, non-string RHS
	}
	return "", false
}
