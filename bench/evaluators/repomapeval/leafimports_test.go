package repomapeval

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestLeafImports asserts the repomapeval leaf imports the Go standard library
// ONLY — no internal/repomap, internal/fuzzy, bench/runtime, bench/datasets, or
// any other in-repo package. This self-test is the SOLE boundary enforcer:
// vet-ablation-leakage (internal/lint/ablationleakage, checkedPkgPrefix =
// ".../bench/runners") does NOT gate bench/evaluators/*, so nothing else fails
// the build if a forbidden import creeps in (RESEARCH Pitfall 1). It scans the
// package's own non-test .go files (an *_test.go file may import test-only
// helpers; the shipped leaf may not).
func TestLeafImports(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: unquote import %q: %v", name, imp.Path.Value, err)
			}
			if !isStdlibImport(path) {
				t.Errorf("%s: forbidden non-stdlib import %q — the leaf must be stdlib-only", name, path)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("scanned zero non-test .go files — the self-test would vacuously pass")
	}
}

// isStdlibImport reports whether importPath is a Go standard-library package.
// Stdlib paths have NO dot in their first path segment (e.g. "encoding/json",
// "math", "path/filepath"); any in-repo or third-party path has a dotted domain
// in the first segment (e.g. "github.com/agenthands/helix/internal/repomap").
func isStdlibImport(importPath string) bool {
	first := importPath
	if i := strings.IndexByte(importPath, '/'); i >= 0 {
		first = importPath[:i]
	}
	return !strings.Contains(first, ".")
}

// TestLeafImports_SelfDiscriminates proves the import classifier BITES: a known
// in-repo import path MUST be rejected, and a known stdlib path MUST be accepted.
// Without this, TestLeafImports could vacuously pass if isStdlibImport were
// mis-wired to always return true.
func TestLeafImports_SelfDiscriminates(t *testing.T) {
	forbidden := []string{
		"github.com/agenthands/helix/internal/repomap",
		"github.com/agenthands/helix/internal/fuzzy",
		"github.com/agenthands/helix/bench/runtime",
		"github.com/agenthands/helix/bench/datasets/aider-polyglot",
	}
	for _, p := range forbidden {
		if isStdlibImport(p) {
			t.Errorf("isStdlibImport(%q) = true; want false (in-repo import must be rejected)", p)
		}
	}
	stdlib := []string{"math", "sort", "encoding/json", "os", "path/filepath", "strings"}
	for _, p := range stdlib {
		if !isStdlibImport(p) {
			t.Errorf("isStdlibImport(%q) = false; want true (stdlib import must be accepted)", p)
		}
	}
}
