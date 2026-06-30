package rubyextract

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract"
)

// resources.go detects ORM / data-entity definitions in Ruby source and
// synthesizes a ResourceFact per entity. One shape:
//   - ActiveRecord: a `class` node whose superclass is ApplicationRecord or
//     ActiveRecord::Base (the conventional Rails model base classes).
//
// A plain class (no superclass, or an unrelated superclass) is not a resource.
// ActiveRecord table names follow a naming convention the source does not
// state explicitly, so Table is left "".

func detectRubyResources(root tree_sitter.Node, source []byte, filePath string) []extract.ResourceFact {
	var resources []extract.ResourceFact
	var walk func(n tree_sitter.Node)
	walk = func(n tree_sitter.Node) {
		if n.Kind() == "class" {
			if rf, ok := rubyResourceFromClass(n, source, filePath); ok {
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

// rubyResourceFromClass classifies a `class` node as an ActiveRecord model when
// its superclass is ApplicationRecord or ActiveRecord::Base.
func rubyResourceFromClass(cls tree_sitter.Node, source []byte, filePath string) (extract.ResourceFact, bool) {
	super := cls.ChildByFieldName("superclass")
	if super == nil {
		return extract.ResourceFact{}, false
	}
	superName := rubySuperclassName(super, source)
	if !rubyIsActiveRecordBase(superName) {
		return extract.ResourceFact{}, false
	}
	name := rubyClassName(cls, source)
	if name == "" {
		return extract.ResourceFact{}, false
	}
	return extract.ResourceFact{
		Language: "ruby",
		Name:     name,
		Table:    "",
		ORM:      "activerecord",
		File:     filePath,
		Range:    nodeRange(cls),
	}, true
}

// rubyClassName returns the declared name of a `class` node via the "name"
// field, falling back to the first direct (constant) named child.
func rubyClassName(cls tree_sitter.Node, source []byte) string {
	if n := cls.ChildByFieldName("name"); n != nil {
		return strings.TrimSpace(n.Utf8Text(source))
	}
	for i := uint(0); i < cls.NamedChildCount(); i++ {
		if c := cls.NamedChild(i); c != nil && c.Kind() == "constant" {
			return strings.TrimSpace(c.Utf8Text(source))
		}
	}
	return ""
}

// rubySuperclassName returns the text of a superclass node's first named child
// (a `constant` like ApplicationRecord or a `scope_resolution` like
// ActiveRecord::Base), trimmed of surrounding whitespace.
func rubySuperclassName(super *tree_sitter.Node, source []byte) string {
	if super == nil {
		return ""
	}
	// The superclass node includes the leading "<"; its named child is the
	// actual base-class expression.
	for i := uint(0); i < super.NamedChildCount(); i++ {
		if c := super.NamedChild(i); c != nil {
			return strings.TrimSpace(c.Utf8Text(source))
		}
	}
	return ""
}

// rubyIsActiveRecordBase reports whether a superclass expression names an
// ActiveRecord base class: ApplicationRecord, ActiveRecord::Base, or any
// ActiveRecord::-prefixed type (e.g. a custom ActiveRecord::Base subclass).
func rubyIsActiveRecordBase(superName string) bool {
	switch superName {
	case "ApplicationRecord", "ActiveRecord::Base":
		return true
	}
	return strings.HasPrefix(superName, "ActiveRecord::")
}
