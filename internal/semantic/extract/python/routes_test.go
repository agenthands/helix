package pyextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectPyRoutes(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`from flask import Flask
app = Flask(__name__)

@app.route("/users")
def list_users():
    pass

@app.get("/items")
def get_items():
    pass

@app.post("/items")
def create_items():
    pass
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "app.py", Language: "python"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := map[string]string{}
	for _, r := range ef.Routes {
		got[r.Method+" "+r.Path] = r.Handler
	}
	cases := []struct{ key, handler string }{
		{" /users", "list_users"},   // Flask @app.route — method undetermined
		{"GET /items", "get_items"}, // FastAPI-style method decorator
		{"POST /items", "create_items"},
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
}
