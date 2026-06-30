package kotlinextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectKotlinResources(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`package demo

import jakarta.persistence.Entity
import jakarta.persistence.Id
import jakarta.persistence.Table
import org.jetbrains.exposed.sql.Table

@Entity
@Table(name = "app_users")
class User(
    @Id val id: Long,
    val email: String
)

@Entity
class Product(@Id val id: Long, val name: String)

// Exposed table subclass: positional table name argument.
class Posts : Table("posts") {
    val id = long("id")
}

// Exposed LongIdTable subclass without an explicit table name.
class Comments : LongIdTable() {
    val body = text("body")
}

class Plain(val x: Int)

data class ValueObject(val n: Int)
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "demo.kt", Language: "kotlin"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	type key struct{ name, orm string }
	got := map[key]extract.ResourceFact{}
	for _, r := range ef.Resources {
		got[key{r.Name, r.ORM}] = r
	}

	cases := []struct {
		name  string
		orm   string
		table string
	}{
		{"User", "jpa_kotlin", "app_users"}, // @Entity + @Table(name=...)
		{"Product", "jpa_kotlin", ""},       // @Entity, no @Table
		{"Posts", "exposed", "posts"},       // Table("posts")
		{"Comments", "exposed", ""},         // LongIdTable(), no name
	}
	for _, c := range cases {
		r, ok := got[key{c.name, c.orm}]
		if !ok {
			t.Errorf("missing resource %s (%s); detected=%+v", c.name, c.orm, ef.Resources)
			continue
		}
		if r.Table != c.table {
			t.Errorf("resource %s table = %q, want %q", c.name, r.Table, c.table)
		}
		if r.File != "demo.kt" {
			t.Errorf("resource %s file = %q, want demo.kt", c.name, r.File)
		}
		if r.Language != "kotlin" {
			t.Errorf("resource %s language = %q, want kotlin", c.name, r.Language)
		}
	}

	if len(ef.Resources) != len(cases) {
		t.Errorf("resource count = %d, want %d (false positive); got %+v", len(ef.Resources), len(cases), ef.Resources)
	}
}
