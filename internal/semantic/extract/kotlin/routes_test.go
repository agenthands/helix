package kotlinextract

import (
	"context"
	"sort"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/extract/testutil"
)

// kotlinRouteKey is a (method, path) pair used to dedupe/compare detected routes.
type kotlinRouteKey struct{ method, path string }

func TestDetectKotlinRoutes(t *testing.T) {
	registry := testutil.NewTestRegistry(t)
	p := NewProvider(registry).(*Provider)
	src := []byte(`fun Application.module() {
    routing {
        get("/users") {
            handle()
        }
        post("/items") { }
        put("/items/{id}") { }
        delete("/items/{id}") { }
        route("/admin") {
            get("/stats") { }
        }
        // not a route: bare call with a non-route name.
        helper(doWork())
        // not a route: known verb but the first arg is not a string literal.
        get(somePath())
    }
}
`)
	ef, err := p.Extract(context.Background(), src, extract.SourceFile{Path: "App.kt", Language: "kotlin"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	got := map[kotlinRouteKey]bool{}
	for _, r := range ef.Routes {
		got[kotlinRouteKey{r.Method, r.Path}] = true
		// Ktor handlers are trailing lambdas — always anonymous.
		if r.Handler != "" {
			t.Errorf("route %+v handler = %q, want empty (anonymous lambda)", r, r.Handler)
		}
		if r.Language != "kotlin" {
			t.Errorf("route %+v language = %q, want kotlin", r, r.Language)
		}
		if r.File != "App.kt" {
			t.Errorf("route %+v file = %q, want App.kt", r, r.File)
		}
	}

	want := []kotlinRouteKey{
		{"GET", "/users"},
		{"POST", "/items"},
		{"PUT", "/items/{id}"},
		{"DELETE", "/items/{id}"},
		{"", "/admin"},
		{"GET", "/stats"},
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing route %+v; detected=%v", w, routeKeys(got))
		}
	}

	// No false positives: helper()/doWork() and get(somePath()) must not appear,
	// and no (method, path) should be over-counted.
	if len(ef.Routes) != len(want) {
		t.Errorf("route count = %d, want %d; detected=%v", len(ef.Routes), len(want), routeKeys(got))
	}
	for _, bad := range []string{"doWork", "helper", "somePath"} {
		for _, r := range ef.Routes {
			if r.Path == bad {
				t.Errorf("false-positive route %+v (path %q)", r, bad)
			}
		}
	}
}

func routeKeys(m map[kotlinRouteKey]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k.method+" "+k.path)
	}
	sort.Strings(out)
	return out
}
