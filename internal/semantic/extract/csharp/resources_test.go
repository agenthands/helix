package csharpextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectCSharpResources(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`using System.ComponentModel.DataAnnotations.Schema;
using System.ComponentModel.DataAnnotations;

[Table("users")]
public class User
{
    public int Id { get; set; }
    public string Name { get; set; }
}

[Entity]
public class Product
{
    public int Id { get; set; }
}

[Table("orders"), Entity]
public class Order
{
    public int Id { get; set; }
}

public class PlainDto
{
    // A [Table] on a property must NOT flag the enclosing class.
    [Column("v")]
    public int Value { get; set; }
}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "Models.cs", Language: "c_sharp"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	got := map[string]extract.ResourceFact{}
	for _, r := range ef.Resources {
		got[r.Name] = r
	}

	cases := []struct {
		name, table string
	}{
		{"User", "users"},   // [Table("users")]
		{"Product", ""},     // [Entity] → no explicit table
		{"Order", "orders"}, // [Table("orders"), Entity] → table name wins, single resource
	}

	// Require the exact resource set — no stray detections from the plain DTO,
	// its [Column] property attribute, or un-decorated classes.
	if len(ef.Resources) != len(cases) {
		t.Errorf("detected %d resources, want %d; detected=%+v", len(ef.Resources), len(cases), ef.Resources)
	}

	for _, c := range cases {
		r, ok := got[c.name]
		if !ok {
			t.Errorf("missing resource %q; detected=%v", c.name, got)
			continue
		}
		if r.Table != c.table {
			t.Errorf("resource %q Table = %q, want %q", c.name, r.Table, c.table)
		}
		if r.ORM != "ef" {
			t.Errorf("resource %q ORM = %q, want ef", c.name, r.ORM)
		}
		if r.Language != "c_sharp" {
			t.Errorf("resource %q Language = %q, want c_sharp", c.name, r.Language)
		}
		if r.File != "Models.cs" {
			t.Errorf("resource %q File = %q, want Models.cs", c.name, r.File)
		}
	}

	// No false positives: the plain DTO class must never be a resource.
	if _, ok := got["PlainDto"]; ok {
		t.Errorf("plain class PlainDto should not be a resource; detected=%+v", ef.Resources)
	}
}
