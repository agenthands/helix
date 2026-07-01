// Package c_sharp implements the Phase 62 P05 / v2.11 Phase 133 C# type resolver.
//
// C# populates tiers 1 (LSP), 2 (annotation — explicit typed declaration),
// 3 (constructor — `new Foo()` / records), 4 (assignment — `var x = y`),
// 6 (heuristic — PascalCase / I-prefix / suffix) and 7 (unresolved). There is
// NO comment tier in v2.11 (Tier 5 — no doc_comment schema column, red-team
// B2). Per M3 the annotation tier is the TYPED DECLARATION; `[Attribute]`
// decorations are auxiliary and stripped, not the type signal.
//
// Per CLAUDE.md no tree-sitter grammar import lives here — facts are already
// extracted by Phase 59; the resolver reads the extracted Signature/StableKey
// only.
package c_sharp

import "path/filepath"

// NamespaceScope returns the C# namespace scope for a file. Per red-team M1,
// v2.11 C# resolves INTRA-namespace only. The narrow types.SymbolFact
// projection does not carry the declared `namespace`, so scope is approximated
// by the file's directory — the conventional C# folder≈namespace layout. A
// future projection carrying qualified_name/package_path would replace this
// with the true declared namespace. No I/O.
func NamespaceScope(filePath string) string {
	if filePath == "" {
		return ""
	}
	return filepath.Dir(filePath)
}

// SameNamespace reports whether two C# files resolve within the same namespace
// scope (directory approximation — see NamespaceScope).
func SameNamespace(a, b string) bool {
	return NamespaceScope(a) == NamespaceScope(b)
}
