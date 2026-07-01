package daemon

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	cextract "github.com/agenthands/helix/internal/semantic/extract/c"
	"github.com/agenthands/helix/internal/treesitter"
)

// cExtract runs the REAL C provider over src and returns the ExtractedFile.
// Task 4 drives the production extractor (not hand-built facts) so the test
// exercises the co-capture path end to end.
func cExtract(t *testing.T, path, src string) *extract.ExtractedFile {
	t.Helper()
	grammars := treesitter.NewGrammarRegistry()
	p := cextract.NewProvider(grammars)
	ef, err := p.Extract(context.Background(), []byte(src), extract.SourceFile{Path: path, Language: "c"})
	if err != nil {
		t.Fatalf("c Extract(%s): %v", path, err)
	}
	if ef == nil {
		t.Fatalf("c Extract(%s): nil ExtractedFile", path)
	}
	return ef
}

func findLink(links []varTypeLink, refName string) (varTypeLink, bool) {
	for _, l := range links {
		if l.RefName == refName {
			return l, true
		}
	}
	return varTypeLink{}, false
}

// TestLinkVarTypes_StructParam is the Task 4.1 positive case: a struct
// definition + a parameter typed by that struct produces a (p, "Foo") link.
func TestLinkVarTypes_StructParam(t *testing.T) {
	ef := cExtract(t, "a.c", "struct Foo { int x; };\nvoid g(struct Foo* p) { }\n")
	links := linkVarTypes([]*extract.ExtractedFile{ef})

	l, ok := findLink(links, "p")
	if !ok {
		t.Fatalf("no link for parameter %q; links=%+v", "p", links)
	}
	if l.TypeName != "Foo" {
		t.Errorf("link(p).TypeName = %q, want %q", l.TypeName, "Foo")
	}
	if l.RefStableKey.QualifiedName != "p" {
		t.Errorf("link(p).RefStableKey.QualifiedName = %q, want %q", l.RefStableKey.QualifiedName, "p")
	}
}

// TestLinkVarTypes_TypedefVar is the Task 4.2 case: a typedef'd type used as
// a variable's declared type produces a (x, "Foo") link.
func TestLinkVarTypes_TypedefVar(t *testing.T) {
	ef := cExtract(t, "b.c", "typedef struct Foo Foo;\nFoo x = init;\n")
	links := linkVarTypes([]*extract.ExtractedFile{ef})

	l, ok := findLink(links, "x")
	if !ok {
		t.Fatalf("no link for variable %q; links=%+v", "x", links)
	}
	if l.TypeName != "Foo" {
		t.Errorf("link(x).TypeName = %q, want %q", l.TypeName, "Foo")
	}
}

// TestLinkVarTypes_AntiVacuity is the Task 4.3 break-the-linkage guard: a
// type-less / primitive declaration yields NO link. If a link appears for a
// primitive-typed name, the mechanism is fabricating a target → RED.
func TestLinkVarTypes_AntiVacuity(t *testing.T) {
	// int n = 0; — primitive type, not a type reference.
	// int add(int a, int b) — primitive params.
	ef := cExtract(t, "c.c", "int n = 0;\nint add(int a, int b) { return a + b; }\n")
	links := linkVarTypes([]*extract.ExtractedFile{ef})

	if len(links) != 0 {
		t.Fatalf("anti-vacuity violated: primitive-typed decls produced %d link(s): %+v", len(links), links)
	}
	for _, name := range []string{"n", "a", "b"} {
		if _, ok := findLink(links, name); ok {
			t.Errorf("primitive-typed %q must produce NO link (fabricated target)", name)
		}
	}
}

// TestLinkVarTypes_MixedAntiVacuity guards that within a single file the
// named-type symbol links while the primitive-typed symbol does not — the
// linkage discriminates, it does not link-everything or link-nothing.
func TestLinkVarTypes_MixedAntiVacuity(t *testing.T) {
	ef := cExtract(t, "d.c",
		"struct Foo { int a; };\n"+
			"int mix(struct Foo* fp, int n) { return n; }\n")
	links := linkVarTypes([]*extract.ExtractedFile{ef})

	if l, ok := findLink(links, "fp"); !ok || l.TypeName != "Foo" {
		t.Errorf("named-type param fp must link to Foo; got ok=%v link=%+v", ok, l)
	}
	if _, ok := findLink(links, "n"); ok {
		t.Errorf("primitive-typed param n must NOT link (anti-vacuity)")
	}
}

// TestLinkVarTypes_Determinism is the Task 4.4 case: repeated extraction +
// linkage over the same fixture yields an identical slice (order + content).
func TestLinkVarTypes_Determinism(t *testing.T) {
	src := "struct Foo { int x; };\nstruct Bar { int y; };\n" +
		"int f(struct Foo* p, struct Bar* q, int n) { return n; }\n"

	run := func() []varTypeLink {
		ef := cExtract(t, "e.c", src)
		return linkVarTypes([]*extract.ExtractedFile{ef})
	}
	a := run()
	b := run()

	if len(a) != len(b) {
		t.Fatalf("determinism: len differs a=%d b=%d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("determinism: link[%d] differs:\n a=%+v\n b=%+v", i, a[i], b[i])
		}
	}
	// Sanity: the fixture yields exactly the two named-type params, in order.
	if len(a) != 2 || a[0].RefName != "p" || a[0].TypeName != "Foo" ||
		a[1].RefName != "q" || a[1].TypeName != "Bar" {
		t.Errorf("unexpected link set (order-stable expectation): %+v", a)
	}
}

// TestLinkVarTypes_NilSafety guards nil entries and empty input.
func TestLinkVarTypes_NilSafety(t *testing.T) {
	if got := linkVarTypes(nil); got != nil {
		t.Errorf("linkVarTypes(nil) = %+v, want nil", got)
	}
	if got := linkVarTypes([]*extract.ExtractedFile{nil}); len(got) != 0 {
		t.Errorf("linkVarTypes([nil]) = %+v, want empty", got)
	}
}
