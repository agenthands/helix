// Package python implements the Phase 62 P05 Python type resolver.
//
// Per D-13 the package scope is the directory of the INNERMOST __init__.py
// walking up from the request file. When no __init__.py is reachable the
// scope falls back to the file's containing directory. The only allowed
// I/O is `os.Stat` on candidate `__init__.py` paths.
//
// Per D-12 the comment / annotation parser is hand-rolled (regex-only); no
// third-party dependencies. Per CLAUDE.md no AST-grammar import lives here
// — facts are already extracted by Phase 59 (RESEARCH Anti-Pattern "AST
// access in type resolver").
package python

import (
	"os"
	"path/filepath"
)

// PackageScope returns the package directory for a Python file. Walks UP
// from filepath.Dir(filePath); the FIRST (innermost) ancestor containing
// __init__.py wins. Falls back to filepath.Dir(filePath) when no
// __init__.py is reachable before the filesystem root.
func PackageScope(filePath string) string {
	if filePath == "" {
		return ""
	}
	dir := filepath.Dir(filePath)
	for {
		init := filepath.Join(dir, "__init__.py")
		if info, err := os.Stat(init); err == nil && !info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Dir(filePath)
		}
		dir = parent
	}
}

// SamePackage returns true iff a and b live within the same Python
// package scope.
func SamePackage(a, b string) bool {
	return PackageScope(a) == PackageScope(b)
}
