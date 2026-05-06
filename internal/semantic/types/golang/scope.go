// Package golang implements the Phase 62 P05 Go type resolver.
//
// The resolver walks the SPEC §38.2 7-tier confidence ladder against the
// shared types.Resolver contract. Per D-13 the package scope IS the file's
// containing directory — Go's "package per directory" rule. Cross-package
// chains never resolve at validated tiers; they cap at last-in-package
// confidence with validation_state="unresolved" (D-13 + Pitfall 5).
//
// Per D-12 the comment parser is hand-rolled (regex-only); no third-party
// dependencies. Per CLAUDE.md no tree-sitter import lives here — facts are
// already extracted by Phase 59.
package golang

import "path/filepath"

// PackageScope returns the Go package scope for a file path. Go's
// "package per directory" rule means the file's containing directory is the
// package. There is no I/O.
func PackageScope(filePath string) string {
	if filePath == "" {
		return ""
	}
	return filepath.Dir(filePath)
}

// SamePackage returns true iff a and b live in the same Go package
// (i.e., the same directory).
func SamePackage(a, b string) bool {
	return PackageScope(a) == PackageScope(b)
}
