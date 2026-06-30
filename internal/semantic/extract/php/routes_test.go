package phpextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectPhpRoutes(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`<?php

use Illuminate\Support\Facades\Route;

// Laravel static-call routes.
Route::get("/users", [UserController::class, "index"]);
Route::post("/items", "handler");
Route::put("/items/{id}", [ItemController::class, "update"]);
Route::delete("/items/{id}", [ItemController::class, "destroy"]);
Route::patch("/items/{id}", [ItemController::class, "patch"]);

// Slim member-call routes.
$app->get("/widgets", listWidgets);
$app->post("/widgets", $createWidgets);

// Not a route: static call whose method is not a route verb.
$config::set("key", "value");
// Not a route: verb is fine but first arg is not a string-literal path.
Route::get($dynamicPath, "h");
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "routes/web.php", Language: "php"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	got := map[string]string{}
	for _, r := range ef.Routes {
		key := r.Method + " " + r.Path
		// Flag any unexpected duplicate key.
		if _, dup := got[key]; dup {
			t.Errorf("duplicate route key %q", key)
		}
		got[key] = r.Handler
	}

	cases := []struct{ key, handler string }{
		{"GET /users", "index"},       // [Controller::class, "method"]
		{"POST /items", "handler"},    // plain callable string
		{"PUT /items/{id}", "update"}, // array form, single quotes
		{"DELETE /items/{id}", "destroy"},
		{"PATCH /items/{id}", "patch"},
		{"GET /widgets", "listWidgets"}, // Slim member call, bareword
		{"POST /widgets", ""},           // Slim member call, variable handler
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
		delete(got, c.key)
	}

	// Anything left is a false positive.
	for key, h := range got {
		t.Errorf("unexpected route detected: %q -> %q", key, h)
	}
}

// TestDetectPhpRoutes_FalsePositive confirms a same-shape static call with a
// non-route method name is not mistaken for a route.
func TestDetectPhpRoutes_FalsePositive(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`<?php
App::make("/something", "else");
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "a.php", Language: "php"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(ef.Routes) != 0 {
		t.Errorf("expected no routes, got %d: %+v", len(ef.Routes), ef.Routes)
	}
}
