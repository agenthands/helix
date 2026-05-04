package extract_test

// TestNoNewGrammarRegistry walks internal/semantic/extract/* (excluding
// _test.go files and the testutil/ subdirectory) and fails if any file
// calls treesitter.NewGrammarRegistry. Enforces D-01b acceptance criterion
// #11 / EXTRACT-05 / BUG-04 invariant statically.
//
// Why static, in addition to the runtime pointer-equality test in
// internal/daemon/daemon_grammar_test.go:
//   - Runtime test only exercises the executed code path; a dead-code
//     constructor or a future reachable-but-untested path would leak.
//   - Static walk catches the leak at the compile-test boundary, before
//     any new code path lands in production.
//
// Why exclude testutil/:
//   - Tests legitimately need to construct a GrammarRegistry to drive
//     provider tests in isolation; CONTEXT.md "Claude's Discretion" places
//     this helper outside the regression scope. Phase 59 P04 ships
//     internal/semantic/extract/testutil/ for exactly this purpose.
//
// Why exclude _test.go:
//   - Test code is allowed to construct grammars locally for narrow-scope
//     unit tests (e.g., a single provider test that doesn't need the daemon
//     wiring). Production code is not.

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoNewGrammarRegistry(t *testing.T) {
	root := pkgPath(t, "github.com/agenthands/helix/internal/semantic/extract")

	var offenders []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == "testutil" {
				return filepath.SkipDir
			}
			return nil
		}
		// Only inspect Go source; skip _test.go and queries.scm / testdata / etc.
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// Strip trailing line comments and check the remaining substring per line.
		// Block comments are imperfectly handled — a multi-line block containing
		// `treesitter.NewGrammarRegistry` would flag conservatively, which is
		// the desired safe-side behaviour. The current package contains no
		// such block comments; if a future maintainer adds one for legitimate
		// documentation reasons, they can adjust this check or the comment.
		for i, line := range strings.Split(string(data), "\n") {
			stripped := stripLineComment(line)
			if strings.Contains(stripped, "treesitter.NewGrammarRegistry") {
				offenders = append(offenders, fmt.Sprintf("%s:%d", path, i+1))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf(`EXTRACT-05 violation: NewGrammarRegistry call(s) inside internal/semantic/extract/ non-test, non-testutil files:
  %s

The daemon-singleton GrammarRegistry MUST be injected via NewExtractorRegistry / NewProvider constructors. See:
  - .planning/phases/59-tree-sitter-extraction-stable-symbol-ids/59-CONTEXT.md (D-01b acceptance #11, D-02 hard invariant)
  - internal/daemon/daemon.go step 6 (the singleton's only construction site)
  - internal/daemon/daemon_grammar_test.go (runtime pointer-equality companion test)`,
			strings.Join(offenders, "\n  "))
	}
}

// stripLineComment returns line with the // ... suffix removed. Does not
// handle multi-line block comments — close enough for grep gate.
func stripLineComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		return line[:i]
	}
	return line
}

// pkgPath resolves a Go import path to the corresponding source dir on disk.
// Uses go/build.Default.Import (works in module mode without GOPATH).
func pkgPath(t *testing.T, importPath string) string {
	t.Helper()
	p, err := build.Default.Import(importPath, "", build.FindOnly)
	if err != nil {
		t.Fatal(err)
	}
	return p.Dir
}
