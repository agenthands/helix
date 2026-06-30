package tsextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectTSRoutes(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`import express from "express";
const app = express();

app.get("/users", listUsers);
app.post("/users", createUser);

class UserController {
  @Get("/items")
  getItems() {}

  @Post("/items")
  createItems() {}
}

function listUsers(req, res) {}
function createUser(req, res) {}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "server.ts", Language: "typescript"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := map[string]string{}
	for _, r := range ef.Routes {
		got[r.Method+" "+r.Path] = r.Handler
	}
	cases := []struct{ key, handler string }{
		{"GET /users", "listUsers"}, // Express member call
		{"POST /users", "createUser"},
		{"GET /items", "getItems"}, // NestJS decorator
		{"POST /items", "createItems"},
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
