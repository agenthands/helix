package kotlinextract

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// THROWAWAY: dump AST to confirm member-call node shape.
func TestDump_MemberCall(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte("fun main() {\n    obj.foo()\n    a.b.bar()\n    bare()\n}\n")
	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(p.grammar); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse(src, nil)
	defer tree.Close()
	t.Logf("SEXP:\n%s", tree.RootNode().ToSexp())

	// Walk the tree to find every call_expression and inspect callee shape.
	var walk func(n *tree_sitter.Node)
	walk = func(n *tree_sitter.Node) {
		if n == nil {
			return
		}
		if n.Kind() == "call_expression" {
			t.Logf("call_expression @%d text=%q", n.StartByte(), n.Utf8Text(src))
			nc := n.NamedChildCount()
			for i := uint(0); i < nc; i++ {
				c := n.NamedChild(i)
				if c == nil {
					continue
				}
				t.Logf("  child[%d] kind=%s text=%q", i, c.Kind(), c.Utf8Text(src))
				if c.Kind() == "navigation_expression" {
					ncc := c.NamedChildCount()
					for j := uint(0); j < ncc; j++ {
						gc := c.NamedChild(j)
						if gc == nil {
							continue
						}
						t.Logf("    nav.child[%d] kind=%s text=%q", j, gc.Kind(), gc.Utf8Text(src))
					}
				}
			}
		}
		for i := uint(0); i < n.NamedChildCount(); i++ {
			walk(n.NamedChild(i))
		}
	}
	root := tree.RootNode()
	walk(root)
}
