package cppextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// TestDetectCppRoutes covers Crow (CROW_ROUTE macro) and cpp-httplib (svr.Get).
func TestDetectCppRoutes(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`#include "crow.h"
#include "httplib.h"
void setup(Crow::SimpleApp& app, httplib::Server& svr) {
    CROW_ROUTE(app, "/users")([](){ return "users"; });
    svr.Get("/items", [](const httplib::Request&, httplib::Response&){});
    svr.Post("/items", [](const httplib::Request&, httplib::Response&){});
    app.loglevel(crow::LogLevel::Info);
}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "main.cpp", Language: "cpp"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := map[string]bool{}
	for _, r := range ef.Routes {
		got[r.Method+" "+r.Path] = true
	}
	for _, key := range []string{" /users", "GET /items", "POST /items"} {
		if !got[key] {
			t.Errorf("missing route %q; detected=%v", key, got)
		}
	}
	// app.loglevel (a non-route member call) must NOT produce a route.
	for _, r := range ef.Routes {
		if r.Path == "Info" || r.Path == "loglevel" {
			t.Errorf("false-positive route: %+v", r)
		}
	}
}
