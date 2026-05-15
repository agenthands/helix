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
	cfg := configFor(t, wsDir)
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

	// schema_version row max() must equal CurrentSchemaVersion. Phase 60
	// bumped it to 3 (was 2 in Phase 59); the test asserts on the constant
	// rather than the literal so future migrations don't tug this assertion.
	var version int
	if err := db.QueryRow("SELECT max(version) FROM semantic_schema_version").Scan(&version); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Errorf("fresh schema_version: got %d, want %d (CurrentSchemaVersion)", version, CurrentSchemaVersion)
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
	cfg := configFor(t, wsDir)
	// configFor chdirs into wsDir (BL-01) and returns a relative path; the
	// physical file lives at the absolute dbPath we just seeded. Compare on
	// the relative form to lock the test-helper contract.
	wantRel := filepath.Join(".helix", "semantic.duckdb")
	if cfg.Store.Path != wantRel {
		t.Fatalf("test setup: expected configFor.Path=%s, got %s", wantRel, cfg.Store.Path)
	}
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open(existing v1): %v", err)
	}
	defer s.Close()

	db := storeUnderlyingDB(s)

	// schema_version is now CurrentSchemaVersion (Phase 60 bumped to 3).
	var version int
	if err := db.QueryRow("SELECT max(version) FROM semantic_schema_version").Scan(&version); err != nil {
		t.Fatalf("read schema_version after upgrade: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Errorf("upgraded schema_version: got %d, want %d (CurrentSchemaVersion)", version, CurrentSchemaVersion)
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
	cfg := configFor(t, wsDir)
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
	cfg := configFor(t, wsDir)
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

// TestMigration003_FreshLandsAtV3 asserts that opening a fresh DB (with no
// pre-existing file) lands at schema_version=3 and that all v3 columns +
// indexes exist. Phase 60 D-04: live-overlay epoch columns + write_epoch
// stamps + CAS scan indexes.
func TestMigration003_FreshLandsAtV3(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
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

	// schema_version row max() must be ≥ 3 after fresh open (001 stamped 1,
	// 002 stamped 2, 003 stamped 3, …). Phase 63 P63-02 Task 1 added
	// migration004 so fresh-open lands at 4; this test cares only that
	// the v3 contract is in place (columns + indexes), so we assert ≥ 3.
	var version int
	if err := db.QueryRow("SELECT max(version) FROM semantic_schema_version").Scan(&version); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if version < 3 {
		t.Errorf("fresh schema_version: got %d, want >= 3", version)
	}

	// v3 columns must be visible.
	wantCols := []struct{ table, column string }{
		{"semantic_live_overlay_meta", "current_epoch"},
		{"semantic_live_overlay_files", "write_epoch"},
		{"semantic_live_overlay_symbols", "write_epoch"},
		{"semantic_live_overlay_references", "write_epoch"},
		{"semantic_live_overlay_edges", "write_epoch"},
	}
	for _, wc := range wantCols {
		if !columnExists(t, db, wc.table, wc.column) {
			t.Errorf("missing v3 column: %s.%s", wc.table, wc.column)
		}
	}

	// All four (repo_id, write_epoch) indexes must exist.
	wantIndexes := []string{
		"idx_overlay_files_write_epoch",
		"idx_overlay_symbols_write_epoch",
		"idx_overlay_references_write_epoch",
		"idx_overlay_edges_write_epoch",
	}
	for _, idx := range wantIndexes {
		if !indexExists(t, db, idx) {
			t.Errorf("missing v3 index: %s", idx)
		}
	}
}

// TestMigration003_UpgradeFromV1 builds a v1-shaped DB directly (running only
// applyMigration001 against a raw *sql.DB, bypassing Open), then reopens via
// the public Open API. The registry must run 1→2 and 2→3 in order. Phase-57-
// populated DB starting state.
func TestMigration003_UpgradeFromV1(t *testing.T) {
	wsDir := t.TempDir()
	dbPath := filepath.Join(wsDir, ".helix", "semantic.duckdb")
	if err := mkdirAllForTest(t, filepath.Dir(dbPath)); err != nil {
		t.Fatal(err)
	}

	rawDB, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(v1 seed): %v", err)
	}
	if err := applyMigration001(context.Background(), rawDB); err != nil {
		t.Fatalf("applyMigration001 (seed): %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open(existing v1): %v", err)
	}
	defer s.Close()

	db := storeUnderlyingDB(s)
	var version int
	if err := db.QueryRow("SELECT max(version) FROM semantic_schema_version").Scan(&version); err != nil {
		t.Fatalf("read schema_version after upgrade: %v", err)
	}
	// Phase 63 P63-02 Task 1: registry now ends at v4; this test asserts
	// the v1→{≥3} upgrade path lands the v3 columns. >= 3 is sufficient.
	if version < 3 {
		t.Errorf("upgraded schema_version: got %d, want >= 3", version)
	}
	if !columnExists(t, db, "semantic_live_overlay_meta", "current_epoch") {
		t.Error("missing current_epoch on semantic_live_overlay_meta after v1→v3 upgrade")
	}
}

// TestMigration003_UpgradeFromV2 builds a v2-shaped DB directly (running
// applyMigration001 + applyMigration002 against a raw *sql.DB), then reopens
// via Open. The registry must run only 2→3. Phase-59-populated DB starting
// state.
func TestMigration003_UpgradeFromV2(t *testing.T) {
	wsDir := t.TempDir()
	dbPath := filepath.Join(wsDir, ".helix", "semantic.duckdb")
	if err := mkdirAllForTest(t, filepath.Dir(dbPath)); err != nil {
		t.Fatal(err)
	}

	rawDB, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(v2 seed): %v", err)
	}
	if err := applyMigration001(context.Background(), rawDB); err != nil {
		t.Fatalf("applyMigration001 (seed): %v", err)
	}
	if err := applyMigration002(context.Background(), rawDB); err != nil {
		t.Fatalf("applyMigration002 (seed): %v", err)
	}
	// Insert a representative pre-existing overlay-meta row so we can verify
	// the DEFAULT 0 backfill on current_epoch.
	if _, err := rawDB.Exec(`INSERT INTO semantic_live_overlay_meta (
		repo_id, base_snapshot_id, graph_version, freshness,
		overlay_file_count, pending_lsp_count, updated_at
	) VALUES ('seeded-ws', 0, 0, 'fresh', 0, 0, now())`); err != nil {
		t.Fatalf("seed semantic_live_overlay_meta: %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open(existing v2): %v", err)
	}
	defer s.Close()

	db := storeUnderlyingDB(s)
	var version int
	if err := db.QueryRow("SELECT max(version) FROM semantic_schema_version").Scan(&version); err != nil {
		t.Fatalf("read schema_version after upgrade: %v", err)
	}
	// Phase 63 P63-02 Task 1: registry now ends at v4; the test asserts
	// the v2→{≥3} upgrade path lands the v3 columns. >= 3 is sufficient.
	if version < 3 {
		t.Errorf("upgraded schema_version: got %d, want >= 3", version)
	}

	// Pre-existing meta row's current_epoch column took the DEFAULT 0.
	var epoch uint64
	if err := db.QueryRow(`SELECT current_epoch FROM semantic_live_overlay_meta
		WHERE repo_id = 'seeded-ws'`).Scan(&epoch); err != nil {
		t.Fatalf("read current_epoch on seeded row: %v", err)
	}
	if epoch != 0 {
		t.Errorf("pre-existing meta row current_epoch: got %d, want 0 (DEFAULT)", epoch)
	}
}

// indexExists returns true iff the named index is present in
// duckdb_indexes(). DuckDB's index inventory is per-schema.
func indexExists(t *testing.T, db *sql.DB, indexName string) bool {
	t.Helper()
	var n int
	err := db.QueryRow(`SELECT count(*) FROM duckdb_indexes()
		WHERE index_name = ?`, indexName).Scan(&n)
	if err != nil {
		t.Fatalf("duckdb_indexes() lookup for %s: %v", indexName, err)
	}
	return n > 0
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

// TestApplyMigration001_UsesTransactionalHelper anchors the WR-01
// rollback contract to the REAL bootstrap migration (IN-NEW-03). The
// sibling TestApplyMigration001_RollsBackOnFailure feeds synthetic DDL
// into applyStatementsTx, which proves the helper is correct but does
// NOT prove applyMigration001 itself routes through that helper. A
// regression that wires applyMigration001 back to a bare db.ExecContext
// loop (skipping the BEGIN/COMMIT envelope) would slip past the synthetic
// test because the schema-1 happy path exercises no failure case.
//
// Build-time grep gate: read migrations.go at test time and assert
// applyStatementsTx appears at least twice (once in the helper definition,
// once in applyMigration001). This is a coarse sentinel — the test exists
// so a future refactor that decouples the two prompts an explicit decision
// rather than a silent regression.
func TestApplyMigration001_UsesTransactionalHelper(t *testing.T) {
	src, err := os.ReadFile("migrations.go")
	if err != nil {
		t.Fatalf("read migrations.go: %v", err)
	}
	got := strings.Count(string(src), "applyStatementsTx")
	if got < 2 {
		t.Fatalf("migrations.go references applyStatementsTx %d times; want >= 2 "+
			"(definition + applyMigration001 call). A regression that bypasses the "+
			"transactional helper would slip past TestApplyMigration001_RollsBackOnFailure.", got)
	}
}

// TestApplyMigration001_RollsBackOnFailure proves the bootstrap migration
// is atomic: a mid-migration failure leaves no partial schema. WR-01.
//
// The test injects a deliberately broken DDL between two valid statements
// and calls applyStatementsTx directly. After the rollback, none of the
// three target tables must exist.
func TestApplyMigration001_RollsBackOnFailure(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}

	bad := []string{
		`CREATE TABLE t1 (a INTEGER)`,
		`CREATE TABLE t2 (BROKEN SYNTAX HERE)`,
		`CREATE TABLE t3 (a INTEGER)`,
	}
	if err := applyStatementsTx(context.Background(), db, bad); err == nil {
		t.Fatal("applyStatementsTx: want error from bad DDL, got nil")
	}

	var count int
	if err := db.QueryRowContext(context.Background(),
		"SELECT count(*) FROM information_schema.tables WHERE table_name IN ('t1','t2','t3')",
	).Scan(&count); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if count != 0 {
		t.Fatalf("after rollback want 0 tables matching {t1,t2,t3}, got %d", count)
	}
}

// TestMigration006_BaseOverlayEpochColumn asserts that opening a fresh DB
// lands at schema_version=6 and that semantic_snapshots carries the
// `base_overlay_epoch` UBIGINT column DEFAULT 0 prescribed by Phase 70
// CONTEXT.md D3 (overlay-drain baseline epoch). The column is the
// persistent baseline the next incremental build queries via
// OverlayChangedPathsSince(repoID, baseEpoch).
func TestMigration006_BaseOverlayEpochColumn(t *testing.T) {
	wsDir := t.TempDir()
	cfg := configFor(t, wsDir)
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

	// CurrentSchemaVersion must be 6 (Phase 70 P70-02 bump).
	if CurrentSchemaVersion != 6 {
		t.Errorf("CurrentSchemaVersion: got %d, want 6", CurrentSchemaVersion)
	}

	// schema_version row max() must equal CurrentSchemaVersion.
	var version int
	if err := db.QueryRow("SELECT max(version) FROM semantic_schema_version").Scan(&version); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Errorf("fresh schema_version: got %d, want %d (CurrentSchemaVersion)", version, CurrentSchemaVersion)
	}

	// base_overlay_epoch column must exist on semantic_snapshots.
	if !columnExists(t, db, "semantic_snapshots", "base_overlay_epoch") {
		t.Fatal("missing v6 column: semantic_snapshots.base_overlay_epoch")
	}

	// data_type must be UBIGINT (matching schema 3's current_epoch
	// convention on semantic_live_overlay_meta).
	var dataType string
	if err := db.QueryRow(`SELECT data_type FROM information_schema.columns
		WHERE table_name = 'semantic_snapshots' AND column_name = 'base_overlay_epoch'`).Scan(&dataType); err != nil {
		t.Fatalf("info_schema lookup for semantic_snapshots.base_overlay_epoch: %v", err)
	}
	if !strings.EqualFold(dataType, "UBIGINT") {
		t.Errorf("semantic_snapshots.base_overlay_epoch data_type: got %q, want %q", dataType, "UBIGINT")
	}

	// INSERT a row WITHOUT setting base_overlay_epoch — DEFAULT 0 must
	// apply. The semantic_snapshots row uses BeginSnapshot's NOT NULL
	// column set; we mirror it.
	if _, err := db.Exec(`
		INSERT INTO semantic_snapshots (
			snapshot_id, repo_id, repo_root, base_snapshot_id, kind,
			worktree_hash, schema_version, indexer_version, status,
			partial, created_at
		) VALUES (
			9999, 'r-default', '', 0, 'compact', '', 6, 'test', 'pending',
			false, now()
		)
	`); err != nil {
		t.Fatalf("seed snapshot row: %v", err)
	}
	var defaultEpoch uint64
	if err := db.QueryRow(`SELECT base_overlay_epoch FROM semantic_snapshots
		WHERE snapshot_id = 9999`).Scan(&defaultEpoch); err != nil {
		t.Fatalf("read default base_overlay_epoch: %v", err)
	}
	if defaultEpoch != 0 {
		t.Errorf("default base_overlay_epoch on new row: got %d, want 0", defaultEpoch)
	}
}

// TestMigration006_UpgradeFromV5_DefaultsExistingRows seeds a v5-shaped DB
// (running migrations 001..005), inserts a snapshot row that pre-dates the
// v6 column add, then reopens via Open. The pre-existing row must take
// base_overlay_epoch = 0 (DEFAULT 0 backfill); the new column must exist.
func TestMigration006_UpgradeFromV5_DefaultsExistingRows(t *testing.T) {
	wsDir := t.TempDir()
	dbPath := filepath.Join(wsDir, ".helix", "semantic.duckdb")
	if err := mkdirAllForTest(t, filepath.Dir(dbPath)); err != nil {
		t.Fatal(err)
	}

	rawDB, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(v5 seed): %v", err)
	}
	if err := applyMigration001(context.Background(), rawDB); err != nil {
		t.Fatalf("applyMigration001 (seed): %v", err)
	}
	if err := applyMigration002(context.Background(), rawDB); err != nil {
		t.Fatalf("applyMigration002 (seed): %v", err)
	}
	if err := applyMigration003(context.Background(), rawDB); err != nil {
		t.Fatalf("applyMigration003 (seed): %v", err)
	}
	if err := applyMigration004(context.Background(), rawDB); err != nil {
		t.Fatalf("applyMigration004 (seed): %v", err)
	}
	if err := applyMigration005(context.Background(), rawDB); err != nil {
		t.Fatalf("applyMigration005 (seed): %v", err)
	}
	// Seed a snapshot row that pre-dates the v6 column add.
	if _, err := rawDB.Exec(`
		INSERT INTO semantic_snapshots (
			snapshot_id, repo_id, repo_root, base_snapshot_id, kind,
			worktree_hash, schema_version, indexer_version, status,
			partial, created_at
		) VALUES (
			42, 'seeded-ws', '', 0, 'compact', '', 5, 'seed', 'committed',
			false, now()
		)
	`); err != nil {
		t.Fatalf("seed pre-v6 snapshot: %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	cfg := configFor(t, wsDir)
	m := newTestObsMetrics(t)
	s, err := Open(context.Background(), cfg, silentLogger(), m)
	if err != nil {
		t.Fatalf("Open(existing v5): %v", err)
	}
	defer s.Close()

	db := storeUnderlyingDB(s)
	var version int
	if err := db.QueryRow("SELECT max(version) FROM semantic_schema_version").Scan(&version); err != nil {
		t.Fatalf("read schema_version after upgrade: %v", err)
	}
	if version != 6 {
		t.Errorf("upgraded schema_version: got %d, want 6", version)
	}

	// Pre-existing row's base_overlay_epoch took the DEFAULT 0 backfill.
	var epoch uint64
	if err := db.QueryRow(`SELECT base_overlay_epoch FROM semantic_snapshots
		WHERE snapshot_id = 42`).Scan(&epoch); err != nil {
		t.Fatalf("read base_overlay_epoch on seeded row: %v", err)
	}
	if epoch != 0 {
		t.Errorf("pre-existing snapshot row base_overlay_epoch: got %d, want 0 (DEFAULT)", epoch)
	}
}
