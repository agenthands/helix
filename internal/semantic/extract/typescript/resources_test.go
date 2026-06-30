package tsextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectTSResources(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`@Entity("users")
class User {
  id: number;
  name: string;
}

@Entity()
class Account {
  id: number;
  email: string;
}

// Bare decorator form (no call parens) still names the entity class.
@Entity
class Session {
  token: string;
}

class PlainConfig {
  port: number;
}

class TodoService {
  @Get("/todos")
  list() {}
}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "entities.ts", Language: "typescript"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	got := map[string]extract.ResourceFact{}
	for _, r := range ef.Resources {
		got[r.Name] = r
	}

	// Expect exactly the three @Entity-decorated classes.
	want := map[string]struct {
		Table string
		ORM   string
	}{
		"User":    {Table: "users", ORM: "typeorm"},
		"Account": {Table: "", ORM: "typeorm"},
		"Session": {Table: "", ORM: "typeorm"},
	}
	if len(ef.Resources) != len(want) {
		t.Fatalf("detected %d resources, want %d: %+v", len(ef.Resources), len(want), ef.Resources)
	}
	for name, w := range want {
		r, ok := got[name]
		if !ok {
			t.Errorf("missing entity %q; detected=%+v", name, got)
			continue
		}
		if r.Table != w.Table {
			t.Errorf("entity %q Table = %q, want %q", name, r.Table, w.Table)
		}
		if r.ORM != w.ORM {
			t.Errorf("entity %q ORM = %q, want %q", name, r.ORM, w.ORM)
		}
		if r.Language != "typescript" {
			t.Errorf("entity %q Language = %q, want typescript", name, r.Language)
		}
		if r.File != "entities.ts" {
			t.Errorf("entity %q File = %q, want entities.ts", name, r.File)
		}
		if r.Name == "" {
			t.Errorf("entity %q has empty Name", name)
		}
	}

	// No false positives: plain classes and route-decorated methods are not
	// resources.
	for _, n := range []string{"PlainConfig", "TodoService"} {
		if _, ok := got[n]; ok {
			t.Errorf("plain/non-entity class %q must not be a resource", n)
		}
	}
}
