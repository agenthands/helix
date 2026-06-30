package goextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// TestDetectGoRoutes covers net/http + mux HandleFunc/Handle and gin/echo-style
// method routers (GET/POST). Tree-sitter parses syntax only, so the gin types
// need not resolve — the call shape is what matters.
func TestDetectGoRoutes(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`package main

import "net/http"

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", listUsers)
	mux.Handle("/api", apiHandler{})
	r := gin.New()
	r.GET("/items", getItems)
	r.POST("/items", createItem)
	r.DELETE("/items/:id", deleteItem)
	_ = mux
	_ = r
}

func listUsers(w http.ResponseWriter, r *http.Request) {}
func getItems(c *gin.Context)  {}
func createItem(c *gin.Context) {}
func deleteItem(c *gin.Context) {}
type apiHandler struct{}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "main.go", Language: "go"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	got := map[string]string{} // "METHOD /path" -> handler
	for _, r := range ef.Routes {
		got[r.Method+" "+r.Path] = r.Handler
	}
	cases := []struct {
		key, handler string
	}{
		{" /users", "listUsers"},   // mux.HandleFunc — method undetermined
		{"GET /items", "getItems"}, // gin method router
		{"POST /items", "createItem"},
		{"DELETE /items/:id", "deleteItem"},
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
	// mux.Handle with a composite-literal handler yields no resolvable name.
	if h, ok := got[" /api"]; !ok || h != "" {
		t.Errorf(" /api route should be detected with empty handler; got ok=%v handler=%q", ok, h)
	}
	// http.NewServeMux() must NOT be misdetected as a route.
	for _, r := range ef.Routes {
		if r.Path == "ServeMux" || r.Path == "" {
			t.Errorf("false-positive route: %+v", r)
		}
	}
}
