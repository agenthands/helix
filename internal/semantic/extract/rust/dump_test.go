package rustextract

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// THROWAWAY: dump AST to confirm member-call node shape.
func TestDump_MemberCall(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte("fn main() {\n    obj.method();\n    bare();\n}\n")
	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(p.grammar); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse(src, nil)
	defer tree.Close()
	t.Logf("SEXP:\n%s", tree.RootNode().ToSexp())

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()
	matches := cursor.Matches(p.query, tree.RootNode(), src)
	captureNames := p.query.CaptureNames()
	for m := matches.Next(); m != nil; m = matches.Next() {
		for _, c := range m.Captures {
			cn := captureNames[c.Index]
			if cn != "reference.call" {
				continue
			}
			node := c.Node
			t.Logf("call name=%q kind=%s", node.Utf8Text(src), node.Kind())
			parent := node.Parent()
			if parent == nil {
				t.Logf("  parent=nil")
				continue
			}
			t.Logf("  parent.kind=%s", parent.Kind())
			grand := parent.Parent()
			if grand != nil {
				t.Logf("  grandparent.kind=%s", grand.Kind())
			}
			if obj := parent.ChildByFieldName("value"); obj != nil {
				t.Logf("  value=%q", obj.Utf8Text(src))
			} else {
				t.Logf("  value=nil")
			}
			if obj := parent.ChildByFieldName("object"); obj != nil {
				t.Logf("  object=%q", obj.Utf8Text(src))
			}
		}
	}
}
