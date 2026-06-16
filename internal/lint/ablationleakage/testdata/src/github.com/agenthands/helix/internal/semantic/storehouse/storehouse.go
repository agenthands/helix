// Stub package used only by analysistest fixtures. Its import path,
// `internal/semantic/storehouse`, shares the bare-`HasPrefix` collision surface
// with the forbidden `internal/semantic/store` prefix but lacks the slash
// boundary. The analyzer MUST NOT flag an import of this package (slash-
// boundary discipline). Mirrors the noduckdb duckdb-go-sibling regression guard.
package storehouse
