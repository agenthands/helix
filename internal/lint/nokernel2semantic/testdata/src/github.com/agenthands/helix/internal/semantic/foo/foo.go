// Stub package used only by analysistest fixtures. The real
// `internal/semantic/...` packages depend on the wider workspace and are
// irrelevant to the analyzer's import-path check — analysistest only needs
// the import to resolve to *some* package so type-checking succeeds.
package foo
