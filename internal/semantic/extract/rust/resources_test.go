package rustextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// TestDetectRustResources verifies that structs deriving a Diesel (Queryable,
// Insertable) or sqlx (FromRow) entity trait are emitted as Resources, and
// that plain structs / non-entity derives do not produce false positives.
func TestDetectRustResources(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`use diesel::prelude::*;

#[derive(Queryable)]
pub struct User {
    pub id: i32,
    pub name: String,
}

#[derive(Insertable)]
#[table_name = "users"]
struct NewUser<'a> {
    name: &'a str,
}

struct PlainConfig {
    debug: bool,
}

#[derive(Debug, Clone)]
struct ValueObject {
    x: i32,
}

#[derive(FromRow)]
struct Product {
    id: i64,
}

#[get("/health")]
fn health() -> &'static str {
    "ok"
}

fn main() {}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "models.rs", Language: "rust"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	got := map[string]extract.ResourceFact{}
	for _, r := range ef.Resources {
		got[r.Name] = r
	}

	cases := []struct {
		name string
		orm  string
	}{
		{"User", "diesel"},    // #[derive(Queryable)]
		{"NewUser", "diesel"}, // #[derive(Insertable)] + a non-derive #[table_name] attr
		{"Product", "diesel"}, // #[derive(FromRow)] (sqlx)
	}
	for _, c := range cases {
		r, ok := got[c.name]
		if !ok {
			t.Errorf("missing resource %q; detected=%+v", c.name, got)
			continue
		}
		if r.ORM != c.orm {
			t.Errorf("resource %q ORM = %q, want %q", c.name, r.ORM, c.orm)
		}
		if r.Language != "rust" {
			t.Errorf("resource %q Language = %q, want %q", c.name, r.Language, "rust")
		}
		if r.File != "models.rs" {
			t.Errorf("resource %q File = %q, want %q", c.name, r.File, "models.rs")
		}
		if r.Name != c.name {
			t.Errorf("resource %q Name = %q, want %q", c.name, r.Name, c.name)
		}
	}

	// No false positives: a plain struct, a non-entity derive (Debug/Clone),
	// a Rocket #[get] route attribute, and a bare function must NOT be
	// detected as Resources.
	for _, bad := range []string{"PlainConfig", "ValueObject", "health", "main"} {
		if r, ok := got[bad]; ok {
			t.Errorf("plain/non-entity item %q should not be a resource: %+v", bad, r)
		}
	}
}
