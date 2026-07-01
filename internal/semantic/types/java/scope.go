// Package java implements the Phase 62 P05 / v2.11 Phase 134 Java type resolver.
//
// Java populates tiers 1 (LSP), 2 (annotation — typed declaration), 3
// (constructor — `new Foo()` / diamond), 4 (assignment — `var x = y`), 6
// (heuristic — PascalCase / *Impl / Abstract* / get*) and 7 (unresolved).
// There is NO comment tier in v2.11 (Tier 5 — no doc_comment schema column,
// red-team B2). Per M3 the annotation tier is the TYPED DECLARATION;
// `@Override`/`@Inject` annotations are auxiliary CALLS-confirmation signals,
// stripped, not the type signal.
//
// Per CLAUDE.md no tree-sitter grammar import lives here — facts are already
// extracted by Phase 59; the resolver reads the extracted Signature/StableKey.
package java

import "path/filepath"

// PackageScope returns the Java package scope for a file. Java's package names
// map to directory structure by strong convention (package a.b.c ⇒ dir
// a/b/c), so the file's containing directory is a faithful package proxy for
// the intra-package cap (red-team M1: Java resolves INTRA-package only in
// v2.11). No I/O.
func PackageScope(filePath string) string {
	if filePath == "" {
		return ""
	}
	return filepath.Dir(filePath)
}

// SamePackage reports whether two Java files live in the same package
// (directory proxy — see PackageScope).
func SamePackage(a, b string) bool {
	return PackageScope(a) == PackageScope(b)
}
