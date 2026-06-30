package csharpextract

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

func TestDetectCSharpRoutes(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`using Microsoft.AspNetCore.Mvc;

[ApiController]
public class UserController : ControllerBase
{
    [HttpGet("/users")]
    public IActionResult ListUsers() { return Ok(); }

    [HttpPost("/users")]
    public IActionResult CreateUser() { return Ok(); }

    [HttpPut("/users/{id}")]
    public IActionResult UpdateUser() { return Ok(); }

    [HttpDelete]
    public IActionResult DeleteUser() { return Ok(); }

    [Route("/z")]
    public IActionResult Fallback() { return Ok(); }

    public IActionResult NoRoute() { return Ok(); }
}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "UserController.cs", Language: "c_sharp"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Index detected routes by "METHOD PATH" → handler.
	got := map[string]string{}
	for _, r := range ef.Routes {
		got[r.Method+" "+r.Path] = r.Handler
	}

	cases := []struct{ key, handler string }{
		{"GET /users", "ListUsers"},       // [HttpGet("/users")]
		{"POST /users", "CreateUser"},     // [HttpPost("/users")]
		{"PUT /users/{id}", "UpdateUser"}, // [HttpPut("/users/{id}")]
		{"DELETE ", "DeleteUser"},         // [HttpDelete] → no path arg
		{" /z", "Fallback"},               // [Route("/z")] → method "" (undetermined)
	}

	// Require the exact route set — no stray detections from [ApiController],
	// the base type, or un-annotated methods.
	if len(ef.Routes) != len(cases) {
		t.Errorf("detected %d routes, want %d; detected=%+v", len(ef.Routes), len(cases), ef.Routes)
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

	// Language tag and file path must be set on every route.
	for _, r := range ef.Routes {
		if r.Language != "c_sharp" {
			t.Errorf("route %+v Language = %q, want c_sharp", r, r.Language)
		}
		if r.File != "UserController.cs" {
			t.Errorf("route %+v File = %q, want UserController.cs", r, r.File)
		}
	}

	// No false positives: the un-annotated method must never appear as a route.
	for _, r := range ef.Routes {
		if r.Handler == "NoRoute" {
			t.Errorf("un-annotated method NoRoute should not be a route; detected=%+v", ef.Routes)
		}
	}
}
