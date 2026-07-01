// Package cpp implements the Phase 62 P05 / v2.11 Phase 132 C++ type resolver.
//
// C++ populates tiers 1 (LSP), 2 (annotation), 3 (constructor), 4 (assignment),
// 6 (heuristic) and 7 (unresolved). There is NO comment tier in v2.11 (Tier 5 —
// no doc_comment schema column, red-team B2).
//
// Per CLAUDE.md no tree-sitter grammar import lives here — facts are already
// extracted by Phase 59; the resolver reads the extracted Signature/StableKey
// only (RESEARCH Anti-Pattern "AST access in type resolver").
package cpp

// TranslationUnitScope returns the scope key for a C++ file. v2.11 models C++
// scope as FLAT (whole-repo): types declared in headers have program-wide
// linkage across every translation unit that includes them, so the D-13
// cross-package guard that caps Java/C# intra-package does NOT cap C++ — this
// is why the polyglot uplift is "C/C++-heavy" (red-team M1). Namespace-aware
// scoping is deferred (the guard seam below would activate it). No I/O.
func TranslationUnitScope(filePath string) string {
	return "" // flat: every C++ file shares one program-wide scope
}

// SameScope reports whether two C++ files resolve within the same scope.
// Always true for C++ (flat program-wide linkage) — see TranslationUnitScope.
func SameScope(a, b string) bool {
	return TranslationUnitScope(a) == TranslationUnitScope(b)
}
