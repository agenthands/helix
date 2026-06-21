// Stub package used only by analysistest fixtures. The real
// `internal/semantic/store` depends on the wider workspace and is irrelevant to
// the analyzer's import-path check — analysistest only needs the import to
// resolve to *some* package so type-checking succeeds.
package store
