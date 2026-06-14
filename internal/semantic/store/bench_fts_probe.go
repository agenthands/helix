// Bench-only DuckDB FTS5 probe — intentionally lives inside
// internal/semantic/store/ to honor the STORE-06 noduckdb boundary
// (cmd/vet-noduckdb). Used exclusively by Phase 64-01's bleve-vs-DuckDB
// gate benchmark in internal/semantic/bench/semantic_bench_test.go.
//
// This file does NOT participate in production code paths. The build tag
// keeps it out of the standard build; bench tests can still pull it in
// via the same tag. The exposed function is the absolute minimum needed
// to time DuckDB FTS5 row insertion against an identical 50k-row corpus.

//go:build benchfts

package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	// Driver registration is shared with the rest of the store package via
	// the existing blank import in duckdb.go.
	_ "github.com/duckdb/duckdb-go/v2"
)

// BenchSymbolDoc is the row shape inserted by BenchDuckDBFTSIngest. Mirrors
// the bleve SymbolDoc shape so the two benchmarks index byte-equivalent
// content.
type BenchSymbolDoc struct {
	ID            string
	Name          string
	Path          string
	Doc           string
	CommentWindow string
}

// BenchDuckDBFTSIngest opens a fresh DuckDB database under tmpDir, installs
// and loads the FTS extension, creates a 5-column table, bulk-inserts the
// supplied docs in a single transaction, and then builds the FTS index.
// Returns a closer that the caller defers.
//
// The caller is responsible for timing whichever sub-phase it cares about
// (the function itself does not measure). For the bleve-vs-FTS5 gate we
// measure the full ingest including FTS index build, since bleve's batch
// API does the same.
func BenchDuckDBFTSIngest(ctx context.Context, tmpDir string, docs []BenchSymbolDoc) (closer func() error, err error) {
	dbPath := filepath.Join(tmpDir, "bench_fts.duckdb")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}
	closer = db.Close

	// FTS extension ships with DuckDB. Auto-install on first use.
	if _, err := db.ExecContext(ctx, `INSTALL fts;`); err != nil {
		return closer, fmt.Errorf("install fts: %w", err)
	}
	if _, err := db.ExecContext(ctx, `LOAD fts;`); err != nil {
		return closer, fmt.Errorf("load fts: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE symbols (
			id              VARCHAR PRIMARY KEY,
			name            VARCHAR,
			path            VARCHAR,
			doc             VARCHAR,
			comment_window  VARCHAR
		);`); err != nil {
		return closer, fmt.Errorf("create table: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return closer, fmt.Errorf("begin: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO symbols (id, name, path, doc, comment_window) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return closer, fmt.Errorf("prepare: %w", err)
	}
	for _, d := range docs {
		if _, err := stmt.ExecContext(ctx, d.ID, d.Name, d.Path, d.Doc, d.CommentWindow); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return closer, fmt.Errorf("insert: %w", err)
		}
	}
	if err := stmt.Close(); err != nil {
		_ = tx.Rollback()
		return closer, fmt.Errorf("close stmt: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return closer, fmt.Errorf("commit: %w", err)
	}

	// Build the FTS index over the indexable text columns. PRAGMA per
	// duckdb.org/docs/extensions/full_text_search.
	if _, err := db.ExecContext(ctx, `
		PRAGMA create_fts_index(
			'symbols', 'id', 'name', 'path', 'doc', 'comment_window'
		);`); err != nil {
		return closer, fmt.Errorf("create_fts_index: %w", err)
	}

	return closer, nil
}
