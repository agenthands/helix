package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigration_Fresh_v2 asserts that opening a fresh DB lands at
// schema_version=2 with all v1 tables AND the 10 partial-extraction columns
// that v2 introduces.
func TestMigration_Fresh_v2(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(wsDir)
	m := newTestObsMetrics(t)

	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open(fresh): %v", err)
	}
	defer s.Close()

	db := storeUnderlyingDB(s)
	if db == nil {
		t.Fatal("Open(fresh) produced a Store with nil db")
	}

	// schema_version row exists with version=2 (max across rows handles the
	// case where 001 stamped 1 and 002 stamped 2).
	var version int
	if err := db.QueryRow("SELECT max(version) FROM semantic_schema_version").Scan(&version); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if version != 2 {
		t.Errorf("fresh schema_version: got %d, want 2", version)
	}

	// Sample 4 of the 16 Phase 57 tables to confirm v1 tables still exist
	// after running both migrations.
	for _, table := range []string{
		"semantic_files", "semantic_symbols", "semantic_references", "semantic_snapshots",
	} {
		if !tableExists(t, db, table) {
			t.Errorf("missing v1 table after fresh open: %s", table)
		}
	}

	// 10 new columns must be visible in information_schema.columns.
	wantCols := []struct{ table, column string }{
		{"semantic_files", "extraction_status"},
		{"semantic_files", "extraction_partial"},
		{"semantic_files", "partial_reason"},
		{"semantic_files", "extractor_name"},
		{"semantic_files", "extractor_version"},
		{"semantic_files", "error_message"},
		{"semantic_symbols", "partial"},
		{"semantic_symbols", "partial_reason"},
		{"semantic_references", "partial"},
		{"semantic_references", "partial_reason"},
	}
	for _, wc := range wantCols {
		if !columnExists(t, db, wc.table, wc.column) {
			t.Errorf("missing v2 column: %s.%s", wc.table, wc.column)
		}
	}
}

// TestMigration_Existing_v2 constructs a v1-shaped DB by directly running
// applyMigration001 against a raw *sql.DB (bypassing Open so we control the
// starting version), inserts a representative row into each fact table,
// closes, then reopens via Open. The registry should run only the 1→2
// migration (001→1 is skipped because From=0 < currentVersion=1). Existing
// rows must survive; new columns must take their DEFAULT values.
func TestMigration_Existing_v2(t *testing.T) {
	wsDir := t.TempDir()
	dbPath := filepath.Join(wsDir, ".helix", "semantic.duckdb")
	if err := mkdirAllForTest(t, filepath.Dir(dbPath)); err != nil {
		t.Fatal(err)
	}

	// Build a v1 DB directly: open raw sql.DB, run applyMigration001.
	rawDB, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(v1 seed): %v", err)
	}
	if err := applyMigration001(context.Background(), rawDB); err != nil {
		t.Fatalf("applyMigration001 (seed): %v", err)
	}

	// Insert a representative row into the three fact tables we ALTER.
	// Match the v1 column lists from migrations.go schema1Statements.
	if _, err := rawDB.Exec(`INSERT INTO semantic_files (
		snapshot_id, file_id, repo_id, path, language, content_hash,
		size_bytes, line_count, indexed_at
	) VALUES (1, 1, 'r', 'x.go', 'go', 'h', 0, 0, now())`); err != nil {
		t.Fatalf("seed semantic_files: %v", err)
	}
	if _, err := rawDB.Exec(`INSERT INTO semantic_symbols (
		snapshot_id, symbol_id, node_id, file_id, language, kind,
		name, qualified_name, stable_key,
		start_byte, end_byte, start_line, start_col, end_line, end_col,
		extraction_source, confidence
	) VALUES (1, 1, 1, 1, 'go', 'function', 'F', 'pkg.F', 'k',
		0, 0, 0, 0, 0, 0, 'tree_sitter', 0.7)`); err != nil {
		t.Fatalf("seed semantic_symbols: %v", err)
	}
	if _, err := rawDB.Exec(`INSERT INTO semantic_references (
		snapshot_id, ref_id, node_id, file_id, name, ref_kind,
		start_byte, end_byte, start_line, start_col, end_line, end_col,
		validation_state, confidence
	) VALUES (1, 1, 1, 1, 'F', 'call',
		0, 0, 0, 0, 0, 0, 'unverified', 0.7)`); err != nil {
		t.Fatalf("seed semantic_references: %v", err)
	}

	// Confirm starting state is exactly v1.
	var startingVersion int
	if err := rawDB.QueryRow("SELECT max(version) FROM semantic_schema_version").Scan(&startingVersion); err != nil {
		t.Fatalf("seed schema_version: %v", err)
	}
	if startingVersion != 1 {
		t.Fatalf("seed schema_version: got %d, want 1 (test setup bug)", startingVersion)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	// Reopen via the public Open API. The registry should upgrade in place.
	cfg := configFor(wsDir)
	if cfg.Store.Path != dbPath {
		t.Fatalf("test setup: expected configFor.Path=%s, got %s", dbPath, cfg.Store.Path)
	}
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open(existing v1): %v", err)
	}
	defer s.Close()

	db := storeUnderlyingDB(s)

	// schema_version is now 2.
	var version int
	if err := db.QueryRow("SELECT max(version) FROM semantic_schema_version").Scan(&version); err != nil {
		t.Fatalf("read schema_version after upgrade: %v", err)
	}
	if version != 2 {
		t.Errorf("upgraded schema_version: got %d, want 2", version)
	}

	// Existing rows preserved (data preservation invariant).
	var fileCount int
	if err := db.QueryRow("SELECT count(*) FROM semantic_files").Scan(&fileCount); err != nil {
		t.Fatalf("count semantic_files: %v", err)
	}
	if fileCount != 1 {
		t.Errorf("semantic_files row count after upgrade: got %d, want 1 (data loss)", fileCount)
	}

	// New columns default-valued on the upgraded row.
	var (
		extractionStatus  string
		extractionPartial bool
		symbolPartial     bool
		refPartial        bool
	)
	if err := db.QueryRow(`SELECT extraction_status, extraction_partial
		FROM semantic_files WHERE file_id = 1`).Scan(&extractionStatus, &extractionPartial); err != nil {
		t.Fatalf("read v2 cols on semantic_files: %v", err)
	}
	if extractionStatus != "" {
		t.Errorf("upgraded row extraction_status: got %q, want '' (DEFAULT)", extractionStatus)
	}
	if extractionPartial {
		t.Errorf("upgraded row extraction_partial: got true, want false (DEFAULT)")
	}
	if err := db.QueryRow(`SELECT partial FROM semantic_symbols WHERE symbol_id = 1`).Scan(&symbolPartial); err != nil {
		t.Fatalf("read partial on semantic_symbols: %v", err)
	}
	if symbolPartial {
		t.Errorf("upgraded symbol partial: got true, want false (DEFAULT)")
	}
	if err := db.QueryRow(`SELECT partial FROM semantic_references WHERE ref_id = 1`).Scan(&refPartial); err != nil {
		t.Fatalf("read partial on semantic_references: %v", err)
	}
	if refPartial {
		t.Errorf("upgraded reference partial: got true, want false (DEFAULT)")
	}
}

// TestMigration_ForwardIncompatible asserts that an explicit
// schema_version greater than CurrentSchemaVersion causes Open to return a
// "forward-incompatible" error.
//
// Setup: open a fresh DB (lands at v=2 via the migration registry), then
// INSERT an extra row stamping version=99 alongside the existing rows.
// classifyExisting's `LIMIT 1` reads the first physical row (v=2 in DuckDB
// insertion order) and passes the DB through to openExisting. The registry
// loop's reader uses max(version) and trips the forward-incompat guard.
func TestMigration_ForwardIncompatible(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(wsDir)
	m := newTestObsMetrics(t)

	// Land a clean v=2 DB via Open.
	s1, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open seed: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close seed: %v", err)
	}

	// Inject a version=99 row alongside the existing v1, v2 rows.
	rawDB, err := sql.Open("duckdb", cfg.Store.Path)
	if err != nil {
		t.Fatalf("sql.Open for inject: %v", err)
	}
	if _, err := rawDB.Exec("INSERT INTO semantic_schema_version (version, applied_at) VALUES (99, now())"); err != nil {
		t.Fatalf("INSERT v=99: %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close inject db: %v", err)
	}

	// Reopen — Open must surface a forward-incompatible error.
	_, err = Open(context.Background(), cfg, silentLogger(), m)
	if err == nil {
		t.Fatal("Open with version=99 row: want error, got nil")
	}
	if !strings.Contains(err.Error(), "forward-incompatible") {
		t.Errorf("error must mention 'forward-incompatible'; got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("error must mention concrete bad version 99; got %q", err.Error())
	}
}

// TestMigration002_AllColumnsPresent walks the 10 v2 columns one-by-one via
// information_schema.columns and asserts (a) each exists and (b) reports
// the expected DuckDB data type. DuckDB normalizes TEXT to VARCHAR and
// BOOLEAN stays BOOLEAN; if a future DuckDB release reports differently
// the failure surfaces here on first run.
func TestMigration002_AllColumnsPresent(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(wsDir)
	m := newTestObsMetrics(t)

	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	db := storeUnderlyingDB(s)

	cases := []struct{ table, column, dataType string }{
		{"semantic_files", "extraction_status", "VARCHAR"},
		{"semantic_files", "extraction_partial", "BOOLEAN"},
		{"semantic_files", "partial_reason", "VARCHAR"},
		{"semantic_files", "extractor_name", "VARCHAR"},
		{"semantic_files", "extractor_version", "VARCHAR"},
		{"semantic_files", "error_message", "VARCHAR"},
		{"semantic_symbols", "partial", "BOOLEAN"},
		{"semantic_symbols", "partial_reason", "VARCHAR"},
		{"semantic_references", "partial", "BOOLEAN"},
		{"semantic_references", "partial_reason", "VARCHAR"},
	}

	for _, tc := range cases {
		t.Run(tc.table+"_"+tc.column, func(t *testing.T) {
			var dataType string
			err := db.QueryRow(`SELECT data_type FROM information_schema.columns
				WHERE table_name = ? AND column_name = ?`, tc.table, tc.column).Scan(&dataType)
			if err != nil {
				t.Fatalf("info_schema lookup for %s.%s: %v", tc.table, tc.column, err)
			}
			if !strings.EqualFold(dataType, tc.dataType) {
				t.Errorf("%s.%s data_type: got %q, want %q", tc.table, tc.column, dataType, tc.dataType)
			}
		})
	}
}

// --- Local helpers (use a unique suffix to avoid colliding with helpers in
// store_test.go) ---

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	err := db.QueryRow(`SELECT count(*) FROM information_schema.tables
		WHERE table_name = ?`, name).Scan(&n)
	if err != nil {
		t.Fatalf("information_schema.tables lookup for %s: %v", name, err)
	}
	return n > 0
}

func columnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	var n int
	err := db.QueryRow(`SELECT count(*) FROM information_schema.columns
		WHERE table_name = ? AND column_name = ?`, table, column).Scan(&n)
	if err != nil {
		t.Fatalf("information_schema.columns lookup for %s.%s: %v", table, column, err)
	}
	return n > 0
}

// mkdirAllForTest creates the parent directory chain for the seed DB path.
func mkdirAllForTest(t *testing.T, dir string) error {
	t.Helper()
	return os.MkdirAll(dir, 0o755)
}
