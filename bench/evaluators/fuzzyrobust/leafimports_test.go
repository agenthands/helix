package fuzzyrobust

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// editsimImport is the SINGLE in-repo import the fuzzyrobust leaf is allowed
// (D-05: reuse editsim.ES, do not re-implement). Every other non-stdlib import
// is forbidden — especially internal/fuzzy (which imports internal/errors and is
// NOT stdlib-only — match.go:8), internal/repomap, and bench/runtime.
const editsimImport = "github.com/agenthands/helix/bench/evaluators/editsim"

// TestLeafImports asserts the fuzzyrobust leaf imports the Go standard library
// PLUS exactly editsim — nothing else. This self-test is the SOLE boundary
// enforcer: vet-ablation-leakage (internal/lint/ablationleakage, checkedPkgPrefix
// = ".../bench/runners") does NOT gate bench/evaluators/*, so nothing else fails
// the build if a forbidden import creeps in (RESEARCH Pitfall 1/5). It scans the
// package's own non-test .go files (a *_test.go file may import test-only
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
			if !allowedLeafImport(path) {
				t.Errorf("%s: forbidden import %q — the leaf must be stdlib + editsim only", name, path)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("scanned zero non-test .go files — the self-test would vacuously pass")
	}
}

// allowedLeafImport reports whether importPath is permitted in the shipped leaf:
// a Go standard-library package, or exactly the editsim package.
func allowedLeafImport(importPath string) bool {
	return isStdlibImport(importPath) || importPath == editsimImport
}

// isStdlibImport reports whether importPath is a Go standard-library package.
// Stdlib paths have NO dot in their first path segment (e.g. "encoding/json",
// "strings"); any in-repo or third-party path has a dotted domain in the first
// segment (e.g. "github.com/agenthands/helix/internal/fuzzy").
func isStdlibImport(importPath string) bool {
	first := importPath
	if i := strings.IndexByte(importPath, '/'); i >= 0 {
		first = importPath[:i]
	}
	return !strings.Contains(first, ".")
}

// TestLeafImports_SelfDiscriminates proves the import classifier BITES: the
// forbidden internal/* and bench/runtime paths MUST be rejected, stdlib MUST be
// accepted, and editsim MUST be accepted. Without this, TestLeafImports could
// vacuously pass if allowedLeafImport were mis-wired to always return true.
func TestLeafImports_SelfDiscriminates(t *testing.T) {
	forbidden := []string{
		"github.com/agenthands/helix/internal/fuzzy",
		"github.com/agenthands/helix/internal/repomap",
		"github.com/agenthands/helix/bench/runtime",
		"github.com/agenthands/helix/bench/datasets/aider-polyglot",
	}
	for _, p := range forbidden {
		if allowedLeafImport(p) {
			t.Errorf("allowedLeafImport(%q) = true; want false (forbidden import must be rejected)", p)
		}
	}
	if !allowedLeafImport(editsimImport) {
		t.Errorf("allowedLeafImport(%q) = false; want true (editsim is the one allowed in-repo import)", editsimImport)
	}
	for _, p := range []string{"strings", "sort", "encoding/json", "os", "path/filepath", "go/parser"} {
		if !allowedLeafImport(p) {
			t.Errorf("allowedLeafImport(%q) = false; want true (stdlib import must be accepted)", p)
		}
	}
}
