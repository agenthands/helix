package rustextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectRustRoutes(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`#[get("/users")]
fn users() {}

#[post("/items")]
fn create_items() {}

#[route(GET, "/x")]
fn routed() {}

fn plain() {}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "server.rs", Language: "rust"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := map[string]string{}
	for _, r := range ef.Routes {
		got[r.Method+" "+r.Path] = r.Handler
	}
	cases := []struct{ key, handler string }{
		{"GET /users", "users"},         // Rocket #[get]
		{"POST /items", "create_items"}, // Rocket #[post]
		{"GET /x", "routed"},            // Rocket #[route(GET, "/x")]
	}
	for _, c := range cases {
		h, ok := got[c.key]
		if !ok {
			t.Errorf("missing route %q; detected=%v", c.key, got)
			continue
		}
		if h != c.handler {
			t.Errorf("route %q handler = %q, want %q", c.key, h, c.handler)
		}
	}
	// No false positives: the plain function (no route attribute) is never
	// captured, and unrelated attributes are ignored.
	if len(ef.Routes) != 3 {
		t.Errorf("expected exactly 3 routes, got %d: %+v", len(ef.Routes), ef.Routes)
	}
}
