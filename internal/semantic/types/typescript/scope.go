// Package typescript implements the Phase 62 P05 TypeScript / JavaScript
// type resolver. The Dispatcher aliases "javascript" to this package so a
// single resolver covers both surfaces (D-11).
//
// Per D-13 the package scope is the directory containing the nearest
// `tsconfig.json` walking up from the request file. When no tsconfig is
// reachable the scope falls back to the file's containing directory. The
// only allowed I/O is `os.Stat` on candidate `tsconfig.json` paths.
//
// Per D-12 the comment parser is hand-rolled (regex-only); no third-party
// dependencies. Per CLAUDE.md no tree-sitter import lives here — facts are
// already extracted by Phase 59.
package typescript

import (
	"os"
	"path/filepath"
)

// PackageScope returns the package directory for a TypeScript / JavaScript
// file. Walks UP from filepath.Dir(filePath); the first ancestor containing
// a `tsconfig.json` wins. Falls back to filepath.Dir(filePath) when no
// tsconfig is found before the filesystem root.
func PackageScope(filePath string) string {
	if filePath == "" {
		return ""
	}
	dir := filepath.Dir(filePath)
	for {
		candidate := filepath.Join(dir, "tsconfig.json")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root — fall back to file's dir.
			return filepath.Dir(filePath)
		}
		dir = parent
	}
}

// SamePackage returns true iff a and b live within the same TS/JS package
// scope (i.e., share the nearest tsconfig.json or the same fallback dir).
func SamePackage(a, b string) bool {
	return PackageScope(a) == PackageScope(b)
}
