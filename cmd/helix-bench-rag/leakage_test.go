package main

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// forbiddenTransitivePrefixes are the daemon-side subsystems cmd/helix-bench-rag
// must not reach, even transitively (Pitfall 2). internal/mcp is implied: it
// transitively links both, so if either appears the boundary is breached.
var forbiddenTransitivePrefixes = []string{
	"github.com/agenthands/helix/internal/kernel",
	"github.com/agenthands/helix/internal/semantic",
}

// TestNoKernelSemanticImport loads the FULL TRANSITIVE import set of
// cmd/helix-bench-rag via go/packages (NeedImports|NeedDeps) and asserts no
// transitive dependency is a forbidden daemon-side package. A direct-imports-only
// check would be a false pass: a bench helper could re-introduce the edge
// transitively (criterion #1c, the dynamic half).
func TestNoKernelSemanticImport(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedImports | packages.NeedDeps,
	}
	pkgs, err := packages.Load(cfg, "github.com/agenthands/helix/cmd/helix-bench-rag")
	if err != nil {
		t.Fatalf("packages.Load: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected exactly 1 root package, got %d", len(pkgs))
	}
	if errs := collectErrors(pkgs); len(errs) > 0 {
		t.Fatalf("package load errors: %v", errs)
	}

	// BFS over the full transitive import graph.
	seen := map[string]bool{}
	var visit func(p *packages.Package)
	visit = func(p *packages.Package) {
		if seen[p.PkgPath] {
			return
		}
		seen[p.PkgPath] = true
		for _, forbidden := range forbiddenTransitivePrefixes {
			if p.PkgPath == forbidden || strings.HasPrefix(p.PkgPath, forbidden+"/") {
				t.Errorf("cmd/helix-bench-rag transitively imports forbidden package %s", p.PkgPath)
			}
		}
		for _, imp := range p.Imports {
			visit(imp)
		}
	}
	for _, p := range pkgs {
		visit(p)
	}
}

func collectErrors(pkgs []*packages.Package) []string {
	var errs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			errs = append(errs, e.Error())
		}
	})
	return errs
}
