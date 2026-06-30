package rubyextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectRubyRoutes(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`require "sinatra"

get "/users" do
  list_users
end

post "/items" do
  create_item
end

put "/items/:id" do
  update_item
end

patch "/items/:id" do
  patch_item
end

delete "/items/:id" do
  remove_item
end

# Command form with no block — still a route registration.
get "/health"

# Non-route call: "notify" is not an HTTP verb → must NOT be detected.
notify "/nope", "message"
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "app.rb", Language: "ruby"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Expected (method, path) pairs. Handler is always "" (anonymous block).
	want := map[string]bool{
		"GET /users":        true, // get "/users" do end
		"POST /items":       true, // post "/items" do end
		"PUT /items/:id":    true,
		"PATCH /items/:id":  true,
		"DELETE /items/:id": true,
		"GET /health":       true, // command form, no block
	}

	got := map[string]string{}
	for _, r := range ef.Routes {
		key := r.Method + " " + r.Path
		got[key] = r.Handler
		if r.Language != "ruby" {
			t.Errorf("route %q language = %q, want ruby", key, r.Language)
		}
		// Sinatra route bodies are anonymous blocks.
		if r.Handler != "" {
			t.Errorf("route %q handler = %q, want empty (anonymous block)", key, r.Handler)
		}
	}

	for key := range want {
		if _, ok := got[key]; !ok {
			t.Errorf("missing route %q; detected=%v", key, got)
		}
	}

	// No false positives: every detected route must be one we expect.
	for key := range got {
		if !want[key] {
			t.Errorf("unexpected route %q detected (false positive); detected=%v", key, got)
		}
	}
}

// TestDetectRubyRoutes_NoFalsePositiveOnNonRoute verifies that a plain method
// call sharing the call shape but with no string path / non-verb name is not
// misdetected as a route.
func TestDetectRubyRoutes_NoFalsePositiveOnNonRoute(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`class Worker
  def perform
    process "/tmp/file"
  end
end
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "worker.rb", Language: "ruby"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(ef.Routes) != 0 {
		t.Errorf("expected no routes for non-route source; got %+v", ef.Routes)
	}
}
