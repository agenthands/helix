package javaextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectJavaRoutes(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`package com.example;

@RestController
@RequestMapping(value = "/api")
public class UserController {

    @GetMapping("/users")
    public String listUsers() { return "users"; }

    @PostMapping("/items")
    public String createItem() { return "item"; }

    @PutMapping("/things/{id}")
    public String updateThing() { return "ok"; }

    @DeleteMapping("/things/{id}")
    public String deleteThing() { return "ok"; }

    @PatchMapping("/things/{id}")
    public String patchThing() { return "ok"; }

    @RequestMapping("/root")
    public String rootHandler() { return "root"; }

    public String helper() { return "helper"; }
}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "UserController.java", Language: "java"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	got := map[string]string{}
	handlers := map[string]bool{}
	paths := map[string]bool{}
	for _, r := range ef.Routes {
		key := r.Method + " " + r.Path
		got[key] = r.Handler
		handlers[r.Handler] = true
		paths[r.Path] = true
	}

	cases := []struct{ key, handler string }{
		{"GET /users", "listUsers"},
		{"POST /items", "createItem"},
		{"PUT /things/{id}", "updateThing"},
		{"DELETE /things/{id}", "deleteThing"},
		{"PATCH /things/{id}", "patchThing"},
		{" /root", "rootHandler"}, // @RequestMapping → method undetermined
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

	// False positives: the non-annotated helper must not be a handler, and the
	// class-level @RequestMapping prefix must not register its own route.
	if handlers["helper"] {
		t.Errorf("non-annotated method helper detected as route handler; handlers=%v", handlers)
	}
	if paths["/api"] {
		t.Errorf("class-level @RequestMapping prefix leaked as a route; paths=%v", paths)
	}
	if len(ef.Routes) != len(cases) {
		t.Errorf("route count = %d, want %d; routes=%+v", len(ef.Routes), len(cases), ef.Routes)
	}

	// Every detected route must report its language and file.
	for _, r := range ef.Routes {
		if r.Language != "java" {
			t.Errorf("route %+v language = %q, want java", r, r.Language)
		}
		if r.File != "UserController.java" {
			t.Errorf("route %+v file = %q, want UserController.java", r, r.File)
		}
	}
}
