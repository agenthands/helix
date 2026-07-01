// Package c implements the Phase 62 P05 / v2.11 Phase 131 C type resolver.
//
// C has no static type analyzer available to Helix (D-12 v1), so Tier 1 (LSP)
// stays LSP-conditional exactly as the prior stub. The v2.11 uplift adds the
// non-LSP tiers the shared ladder defines that are MEANINGFUL for C:
//
//   - Tier 2 annotation  — typed declaration (`struct Foo x`, `Foo *x`).
//   - Tier 4 assignment  — `y = x` value-flow within fixpoint scope.
//   - Tier 6 heuristic   — `_t`-suffixed typedef / name-shape match.
//
// C has NO constructor tier (Tier 3 — no `new`/ctor syntax) and NO comment
// tier in v2.11 (Tier 5 — no `doc_comment` schema column exists; deferred with
// a future migration, red-team B2). Tier 7 (unresolved) is the always-emit
// floor (D-12).
//
// Per CLAUDE.md no tree-sitter grammar import lives here — facts are already
// extracted by Phase 59; the resolver reads the extracted Signature/StableKey
// only (RESEARCH Anti-Pattern "AST access in type resolver").
package c

// TranslationUnitScope returns the scope key for a C file. C has no
// package/namespace construct: type names declared in headers have
// program-wide linkage across every translation unit that includes them.
// v2.11 therefore models C scope as FLAT (whole-repo) — the D-13
// cross-package guard that caps Java/C# intra-package does NOT cap C, which
// is exactly why the polyglot uplift is "C/C++-heavy" (red-team M1). There
// is no I/O.
//
// (`static`-linkage file-local types are a known v1 over-resolution edge; the
// heuristic/annotation confidence tiers, not scope, bound that risk.)
func TranslationUnitScope(filePath string) string {
	return "" // flat: every C file shares one program-wide scope
}

// SameScope reports whether two C files resolve within the same scope. Always
// true for C (flat program-wide linkage) — see TranslationUnitScope.
func SameScope(a, b string) bool {
	return TranslationUnitScope(a) == TranslationUnitScope(b)
}
