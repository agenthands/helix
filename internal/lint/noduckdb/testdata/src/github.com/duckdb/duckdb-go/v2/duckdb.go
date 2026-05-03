// Stub package used only by analysistest fixtures. The real
// `github.com/duckdb/duckdb-go/v2` driver requires CGO and is irrelevant to
// the analyzer's import-path check — analysistest only needs the import to
// resolve to *some* package so type-checking succeeds.
package duckdb
