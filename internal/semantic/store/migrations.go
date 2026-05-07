package store

import (
	"context"
	"database/sql"
	"fmt"
)

// applyStatementsTx executes stmts inside a single BEGIN/COMMIT. On any
// error the transaction is rolled back and the database file is left in
// its pre-migration state — preventing the partial-schema scenario
// described in REVIEW WR-01. DuckDB supports DDL inside transactions
// (verified in 57-02 SUMMARY's classifyExisting design), so a failed
// CREATE never gets committed.
func applyStatementsTx(ctx context.Context, db *sql.DB, stmts []string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("applyStatementsTx: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit
	for i, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("applyStatementsTx: stmt %d (%s): %w", i+1, firstLine(stmt), err)
		}
	}
	return tx.Commit()
}

// applyMigration001 is the bootstrap migration (version 0 → 1). It executes
// every CREATE TABLE statement from SPEC-DRAFT.md §9.1-§9.11 verbatim and
// then INSERTs the schema-version row.
//
// 16 tables created (matches the SPEC §8 inventory enumerated in
// 57-02-PLAN.md Task 2b acceptance criteria):
//
//  1. semantic_schema_version (§9.1)
//  2. semantic_snapshots (§9.2) + idx_semantic_snapshots_repo_status
//  3. semantic_nodes (§9.3) + 2 indexes
//  4. semantic_files (§9.4) + idx_semantic_files_path
//  5. semantic_symbols (§9.5) + 3 indexes
//  6. semantic_references (§9.6) + 2 indexes
//  7. semantic_edges (§9.7) + 3 indexes
//  8. semantic_diagnostics (§9.8)
//  9. semantic_graph_scores (§9.9) + idx_semantic_scores_snapshot
//
// 10. semantic_clusters (§9.10a)
// 11. semantic_cluster_members (§9.10b)
// 12. semantic_live_overlay_meta (§9.11a)
// 13. semantic_live_overlay_files (§9.11b)
// 14. semantic_live_overlay_symbols (§9.11c)
// 15. semantic_live_overlay_references (§9.11d)
// 16. semantic_live_overlay_edges (§9.11e) + 2 overlay indexes
//
// Phase 57 ships Schema 1 empty-but-correct: tables exist, no data write
// paths. P59 (snapshot writes) and P60 (overlay writes) populate the
// content downstream readers serve.
func applyMigration001(ctx context.Context, db *sql.DB) error {
	return applyStatementsTx(ctx, db, schema1Statements())
}

// schema1Statements returns the SPEC §9 DDL in deterministic order. Each
// statement is a single CREATE TABLE or CREATE INDEX. The grep gates in
// 57-02-PLAN.md Task 2b acceptance criteria scan THIS function — keep the
// `CREATE TABLE semantic_<name>` strings on their own logical lines so the
// per-table presence regex matches.
func schema1Statements() []string {
	return []string{
		// 1. semantic_schema_version (SPEC §9.1)
		`CREATE TABLE semantic_schema_version (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL
		)`,

		// 2. semantic_snapshots (SPEC §9.2)
		`CREATE TABLE semantic_snapshots (
			snapshot_id UBIGINT PRIMARY KEY,
			repo_id TEXT NOT NULL,
			repo_root TEXT NOT NULL,
			base_snapshot_id UBIGINT,
			kind TEXT NOT NULL,
			vcs_kind TEXT,
			branch TEXT,
			commit_hash TEXT,
			worktree_hash TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			indexer_version TEXT NOT NULL,
			tree_sitter_version TEXT,
			lsp_fingerprint TEXT,
			status TEXT NOT NULL,
			partial BOOLEAN NOT NULL DEFAULT false,
			partial_reason TEXT,
			created_at TIMESTAMP NOT NULL,
			committed_at TIMESTAMP,
			aborted_at TIMESTAMP,
			abort_reason TEXT
		)`,
		`CREATE INDEX idx_semantic_snapshots_repo_status
			ON semantic_snapshots(repo_id, status, committed_at)`,

		// 3. semantic_nodes (SPEC §9.3)
		`CREATE TABLE semantic_nodes (
			snapshot_id UBIGINT NOT NULL,
			node_id UBIGINT NOT NULL,
			node_kind TEXT NOT NULL,
			stable_key TEXT NOT NULL,
			display_name TEXT,
			file_id UBIGINT,
			symbol_id UBIGINT,
			diagnostic_id UBIGINT,
			memory_id TEXT,
			created_at TIMESTAMP NOT NULL,
			PRIMARY KEY (snapshot_id, node_id)
		)`,
		`CREATE INDEX idx_semantic_nodes_kind
			ON semantic_nodes(snapshot_id, node_kind)`,
		`CREATE INDEX idx_semantic_nodes_stable_key
			ON semantic_nodes(snapshot_id, stable_key)`,

		// 4. semantic_files (SPEC §9.4)
		`CREATE TABLE semantic_files (
			snapshot_id UBIGINT NOT NULL,
			file_id UBIGINT NOT NULL,
			repo_id TEXT NOT NULL,
			path TEXT NOT NULL,
			language TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			size_bytes UBIGINT NOT NULL,
			line_count INTEGER NOT NULL,
			generated BOOLEAN NOT NULL DEFAULT false,
			ignored BOOLEAN NOT NULL DEFAULT false,
			ignore_reason TEXT,
			indexed_at TIMESTAMP NOT NULL,
			PRIMARY KEY (snapshot_id, file_id)
		)`,
		`CREATE INDEX idx_semantic_files_path
			ON semantic_files(snapshot_id, path)`,

		// 5. semantic_symbols (SPEC §9.5)
		`CREATE TABLE semantic_symbols (
			snapshot_id UBIGINT NOT NULL,
			symbol_id UBIGINT NOT NULL,
			node_id UBIGINT NOT NULL,
			file_id UBIGINT NOT NULL,
			language TEXT NOT NULL,
			kind TEXT NOT NULL,
			name TEXT NOT NULL,
			qualified_name TEXT NOT NULL,
			stable_key TEXT NOT NULL,
			owner_symbol_id UBIGINT,
			parent_scope_id UBIGINT,
			package_path TEXT,
			start_byte INTEGER NOT NULL,
			end_byte INTEGER NOT NULL,
			start_line INTEGER NOT NULL,
			start_col INTEGER NOT NULL,
			end_line INTEGER NOT NULL,
			end_col INTEGER NOT NULL,
			signature TEXT,
			signature_hash TEXT,
			exported BOOLEAN,
			visibility TEXT,
			extraction_source TEXT NOT NULL,
			confidence DOUBLE NOT NULL,
			content_hash TEXT,
			lsp_identity TEXT,
			PRIMARY KEY (snapshot_id, symbol_id)
		)`,
		`CREATE INDEX idx_semantic_symbols_name
			ON semantic_symbols(snapshot_id, name)`,
		`CREATE INDEX idx_semantic_symbols_qname
			ON semantic_symbols(snapshot_id, qualified_name)`,
		`CREATE INDEX idx_semantic_symbols_stable_key
			ON semantic_symbols(snapshot_id, stable_key)`,

		// 6. semantic_references (SPEC §9.6)
		`CREATE TABLE semantic_references (
			snapshot_id UBIGINT NOT NULL,
			ref_id UBIGINT NOT NULL,
			node_id UBIGINT NOT NULL,
			file_id UBIGINT NOT NULL,
			scope_symbol_id UBIGINT,
			name TEXT NOT NULL,
			ref_kind TEXT NOT NULL,
			receiver_text TEXT,
			start_byte INTEGER NOT NULL,
			end_byte INTEGER NOT NULL,
			start_line INTEGER NOT NULL,
			start_col INTEGER NOT NULL,
			end_line INTEGER NOT NULL,
			end_col INTEGER NOT NULL,
			resolved_symbol_id UBIGINT,
			resolution_source TEXT,
			validation_state TEXT NOT NULL,
			confidence DOUBLE NOT NULL,
			reason TEXT,
			PRIMARY KEY (snapshot_id, ref_id)
		)`,
		`CREATE INDEX idx_semantic_refs_name
			ON semantic_references(snapshot_id, name)`,
		`CREATE INDEX idx_semantic_refs_resolved
			ON semantic_references(snapshot_id, resolved_symbol_id)`,

		// 7. semantic_edges (SPEC §9.7)
		`CREATE TABLE semantic_edges (
			snapshot_id UBIGINT NOT NULL,
			edge_id UBIGINT NOT NULL,
			src_node_id UBIGINT NOT NULL,
			dst_node_id UBIGINT NOT NULL,
			src_kind TEXT NOT NULL,
			dst_kind TEXT NOT NULL,
			edge_kind TEXT NOT NULL,
			weight DOUBLE NOT NULL,
			confidence DOUBLE NOT NULL,
			validation_state TEXT NOT NULL,
			evidence_ref_id UBIGINT,
			evidence_file_id UBIGINT,
			source TEXT NOT NULL,
			reason TEXT,
			created_at TIMESTAMP NOT NULL,
			PRIMARY KEY (snapshot_id, edge_id)
		)`,
		`CREATE INDEX idx_semantic_edges_src
			ON semantic_edges(snapshot_id, src_node_id, edge_kind)`,
		`CREATE INDEX idx_semantic_edges_dst
			ON semantic_edges(snapshot_id, dst_node_id, edge_kind)`,
		`CREATE INDEX idx_semantic_edges_kind
			ON semantic_edges(snapshot_id, edge_kind)`,

		// 8. semantic_diagnostics (SPEC §9.8)
		`CREATE TABLE semantic_diagnostics (
			snapshot_id UBIGINT NOT NULL,
			diagnostic_id UBIGINT NOT NULL,
			node_id UBIGINT NOT NULL,
			file_id UBIGINT NOT NULL,
			language TEXT NOT NULL,
			severity TEXT NOT NULL,
			code TEXT,
			message TEXT NOT NULL,
			start_line INTEGER NOT NULL,
			start_col INTEGER NOT NULL,
			end_line INTEGER NOT NULL,
			end_col INTEGER NOT NULL,
			source TEXT,
			related_symbol_id UBIGINT,
			PRIMARY KEY (snapshot_id, diagnostic_id)
		)`,

		// 9. semantic_graph_scores (SPEC §9.9)
		`CREATE TABLE semantic_graph_scores (
			repo_id TEXT NOT NULL,
			snapshot_id UBIGINT NOT NULL,
			graph_version UBIGINT NOT NULL,
			node_id UBIGINT NOT NULL,
			score_name TEXT NOT NULL,
			score DOUBLE NOT NULL,
			rank INTEGER,
			status TEXT NOT NULL,
			computed_at TIMESTAMP NOT NULL,
			algorithm_version TEXT NOT NULL,
			PRIMARY KEY (repo_id, graph_version, node_id, score_name)
		)`,
		`CREATE INDEX idx_semantic_scores_snapshot
			ON semantic_graph_scores(snapshot_id, score_name, status)`,

		// 10. semantic_clusters (SPEC §9.10a)
		`CREATE TABLE semantic_clusters (
			repo_id TEXT NOT NULL,
			snapshot_id UBIGINT NOT NULL,
			graph_version UBIGINT NOT NULL,
			cluster_id UBIGINT NOT NULL,
			algorithm TEXT NOT NULL,
			label TEXT,
			summary TEXT,
			score DOUBLE,
			status TEXT NOT NULL,
			computed_at TIMESTAMP NOT NULL,
			PRIMARY KEY (repo_id, graph_version, cluster_id)
		)`,

		// 11. semantic_cluster_members (SPEC §9.10b)
		`CREATE TABLE semantic_cluster_members (
			repo_id TEXT NOT NULL,
			graph_version UBIGINT NOT NULL,
			cluster_id UBIGINT NOT NULL,
			node_id UBIGINT NOT NULL,
			weight DOUBLE NOT NULL,
			role TEXT,
			PRIMARY KEY (repo_id, graph_version, cluster_id, node_id)
		)`,

		// 12. semantic_live_overlay_meta (SPEC §9.11a)
		`CREATE TABLE semantic_live_overlay_meta (
			repo_id TEXT PRIMARY KEY,
			base_snapshot_id UBIGINT NOT NULL,
			graph_version UBIGINT NOT NULL,
			freshness TEXT NOT NULL,
			overlay_file_count INTEGER NOT NULL,
			pending_lsp_count INTEGER NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,

		// 13. semantic_live_overlay_files (SPEC §9.11b)
		`CREATE TABLE semantic_live_overlay_files (
			repo_id TEXT NOT NULL,
			path TEXT NOT NULL,
			file_id UBIGINT NOT NULL,
			content_hash TEXT,
			language TEXT,
			status TEXT NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (repo_id, path)
		)`,

		// 14. semantic_live_overlay_symbols (SPEC §9.11c)
		`CREATE TABLE semantic_live_overlay_symbols (
			repo_id TEXT NOT NULL,
			symbol_id UBIGINT NOT NULL,
			node_id UBIGINT NOT NULL,
			file_id UBIGINT NOT NULL,
			stable_key TEXT NOT NULL,
			name TEXT NOT NULL,
			qualified_name TEXT NOT NULL,
			kind TEXT NOT NULL,
			status TEXT NOT NULL,
			validation_state TEXT NOT NULL,
			confidence DOUBLE NOT NULL,
			fact_json JSON,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (repo_id, symbol_id)
		)`,

		// 15. semantic_live_overlay_references (SPEC §9.11d)
		`CREATE TABLE semantic_live_overlay_references (
			repo_id TEXT NOT NULL,
			ref_id UBIGINT NOT NULL,
			node_id UBIGINT NOT NULL,
			file_id UBIGINT NOT NULL,
			name TEXT NOT NULL,
			ref_kind TEXT NOT NULL,
			resolved_symbol_id UBIGINT,
			status TEXT NOT NULL,
			validation_state TEXT NOT NULL,
			confidence DOUBLE NOT NULL,
			fact_json JSON,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (repo_id, ref_id)
		)`,

		// 16. semantic_live_overlay_edges (SPEC §9.11e)
		`CREATE TABLE semantic_live_overlay_edges (
			repo_id TEXT NOT NULL,
			edge_id UBIGINT NOT NULL,
			src_node_id UBIGINT NOT NULL,
			dst_node_id UBIGINT NOT NULL,
			edge_kind TEXT NOT NULL,
			status TEXT NOT NULL,
			validation_state TEXT NOT NULL,
			confidence DOUBLE NOT NULL,
			weight DOUBLE NOT NULL,
			source TEXT NOT NULL,
			fact_json JSON,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (repo_id, edge_id)
		)`,
		`CREATE INDEX idx_overlay_edges_src
			ON semantic_live_overlay_edges(repo_id, src_node_id, edge_kind)`,
		`CREATE INDEX idx_overlay_edges_dst
			ON semantic_live_overlay_edges(repo_id, dst_node_id, edge_kind)`,

		// Stamp the schema version row.
		`INSERT INTO semantic_schema_version (version, applied_at) VALUES (1, now())`,
	}
}

// applyMigration002 adds the partial-extraction columns prescribed by
// 59-CONTEXT.md D-05 to the three Phase 57 fact tables, then stamps
// schema_version=2.
//
// 10 columns total:
//
//	semantic_files: 6 columns
//	  extraction_status   TEXT DEFAULT ''
//	  extraction_partial  BOOLEAN DEFAULT false
//	  partial_reason      TEXT  (nullable; closed enum from D-05)
//	  extractor_name      TEXT
//	  extractor_version   TEXT
//	  error_message       TEXT
//	semantic_symbols: 2 columns
//	  partial             BOOLEAN DEFAULT false
//	  partial_reason      TEXT
//	semantic_references: 2 columns
//	  partial             BOOLEAN DEFAULT false
//	  partial_reason      TEXT
//
// Plus one row INSERT into semantic_schema_version stamping version=2.
//
// MigrationKind=InPlace per Phase 57 D-02 — runs at Open time, no reindex,
// no data backfill (existing rows take the column DEFAULTs).
//
// DuckDB ALTER TABLE constraint limitation: DuckDB rejects `ALTER TABLE ...
// ADD COLUMN ... NOT NULL DEFAULT <expr>` ("Adding columns with constraints
// not yet supported"). For ADD COLUMN we use DEFAULT alone — the DEFAULT
// supplies a value for both existing rows (backfill) and future INSERTs
// that omit the column, so the practical outcome is the same as NOT NULL
// DEFAULT: writers cannot leave the column unset. The application layer
// (Phase 59 P02 fact emitter) explicitly sets every column on every write.
// CREATE TABLE in schema 1 is unaffected — NOT NULL DEFAULT inside CREATE
// works fine; the constraint limitation only applies to ALTER TABLE ADD
// COLUMN. Research §A1 in 59-RESEARCH.md called this out as a planning-
// time risk.
//
// Rollback: DuckDB does not support `ALTER TABLE ... DROP COLUMN` for all
// column types reliably; rollback from v2 → v1 requires a full reindex via
// the existing quarantine-and-rebuild path (Phase 57 D-04). Operators on a
// downgrade path: stop daemon, `mv .helix/semantic.duckdb
// .helix/semantic.duckdb.v2.bak`, restart with the older binary which will
// rebuild from sources at schema_version=1.
func applyMigration002(ctx context.Context, db *sql.DB) error {
	stmts := schema2Statements()
	for i, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("applyMigration002: stmt %d (%s): %w", i+1, firstLine(stmt), err)
		}
	}
	return nil
}

// schema2Statements returns the v1→v2 DDL in deterministic order: 6 ALTER
// TABLE statements on semantic_files, 2 each on semantic_symbols and
// semantic_references, then one INSERT into semantic_schema_version.
//
// Acceptance grep gates in 59-01-PLAN.md scan THIS function — keep the
// `ALTER TABLE semantic_<name> ADD COLUMN` strings on their own logical
// lines so the per-table count regex matches.
//
// Closed enum for partial_reason TEXT (extended in Phase 61):
//   - "budget exhausted"      (P59 D-05; per-file timeout / max-symbols)
//   - "preempted"             (P61 D-03; cascade yielded between calls)
//   - "bulk_update_pending"   (P61 D-05; producer suppressed enqueue)
//   - "lsp_unavailable"       (P61 D-08; readiness timeout / circuit open)
//
// The column itself remains permissive TEXT — callers MUST validate against
// this enum (overlay.go partialReasonClosedEnum is the authoritative
// runtime validator; *OverlayTx.MarkFileSemanticPending consumes it).
// No schema change accompanies the Phase 61 extension.
func schema2Statements() []string {
	return []string{
		// semantic_files: 6 partial-extraction columns (D-05).
		// DuckDB rejects NOT NULL on ADD COLUMN even with DEFAULT (Parser
		// Error: "Adding columns with constraints not yet supported"); use
		// DEFAULT alone — see applyMigration002 doc comment.
		`ALTER TABLE semantic_files ADD COLUMN extraction_status TEXT DEFAULT ''`,
		`ALTER TABLE semantic_files ADD COLUMN extraction_partial BOOLEAN DEFAULT false`,
		`ALTER TABLE semantic_files ADD COLUMN partial_reason TEXT`,
		`ALTER TABLE semantic_files ADD COLUMN extractor_name TEXT`,
		`ALTER TABLE semantic_files ADD COLUMN extractor_version TEXT`,
		`ALTER TABLE semantic_files ADD COLUMN error_message TEXT`,

		// semantic_symbols: 2 partial columns.
		`ALTER TABLE semantic_symbols ADD COLUMN partial BOOLEAN DEFAULT false`,
		`ALTER TABLE semantic_symbols ADD COLUMN partial_reason TEXT`,

		// semantic_references: 2 partial columns.
		`ALTER TABLE semantic_references ADD COLUMN partial BOOLEAN DEFAULT false`,
		`ALTER TABLE semantic_references ADD COLUMN partial_reason TEXT`,

		// Stamp the new schema version.
		`INSERT INTO semantic_schema_version (version, applied_at) VALUES (2, now())`,
	}
}

// applyMigration003 lights up the live-overlay epoch contract (Phase 60 D-04).
//
// Adds current_epoch to semantic_live_overlay_meta and write_epoch to each
// of the four overlay fact tables, plus per-table (repo_id, write_epoch)
// indexes for Phase 63's CAS scan path. Stamps schema_version=3.
//
// 9 columns + 4 indexes + 1 schema_version row = 10 ALTER + 4 CREATE INDEX +
// 1 INSERT = 10 statements (ALTER count is 5: meta + 4 fact tables; the
// remaining count comes from indexes and the version stamp).
//
// DuckDB ALTER TABLE constraint limitation (carried over from Phase 59
// applyMigration002): DuckDB rejects `ALTER TABLE ... ADD COLUMN ... NOT NULL
// DEFAULT <expr>` ("Adding columns with constraints not yet supported"). We
// use DEFAULT 0 alone — the application layer (Phase 60 OverlayTx) explicitly
// stamps write_epoch on every write, so the runtime invariant (no NULL or
// zero epochs on writer-touched rows) is enforced in code rather than schema.
//
// MigrationKind=InPlace per Phase 57 D-02 — runs at Open time, no reindex,
// no data backfill (existing rows take the column DEFAULT 0).
//
// Rollback: same as v1→v2 (Phase 59 applyMigration002 doc). DuckDB's
// ALTER TABLE DROP COLUMN support is incomplete; downgrade requires the
// quarantine-and-rebuild path documented in Phase 57 D-04.
func applyMigration003(ctx context.Context, db *sql.DB) error {
	stmts := schema3Statements()
	for i, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("applyMigration003: stmt %d (%s): %w", i+1, firstLine(stmt), err)
		}
	}
	return nil
}

// schema3Statements returns the v2→v3 DDL in deterministic order: 5 ALTER
// TABLE statements (one for the meta table, one for each of the four fact
// tables), 4 CREATE INDEX statements, then one INSERT into
// semantic_schema_version.
//
// Acceptance grep gates in 60-02-PLAN.md scan THIS function — keep the
// `ALTER TABLE semantic_<name> ADD COLUMN write_epoch` strings on their
// own logical lines so per-table presence regexes match.
func schema3Statements() []string {
	return []string{
		// Per-workspace monotone epoch counter.
		`ALTER TABLE semantic_live_overlay_meta ADD COLUMN current_epoch UBIGINT DEFAULT 0`,

		// Per-row write_epoch stamps on the four overlay fact tables.
		`ALTER TABLE semantic_live_overlay_files ADD COLUMN write_epoch UBIGINT DEFAULT 0`,
		`ALTER TABLE semantic_live_overlay_symbols ADD COLUMN write_epoch UBIGINT DEFAULT 0`,
		`ALTER TABLE semantic_live_overlay_references ADD COLUMN write_epoch UBIGINT DEFAULT 0`,
		`ALTER TABLE semantic_live_overlay_edges ADD COLUMN write_epoch UBIGINT DEFAULT 0`,

		// Phase-63 CAS scan path indexes — (repo_id, write_epoch) per fact
		// table. The 'idx_overlay_*_write_epoch' naming convention is the
		// grep-gate target documented in 60-02-PLAN.md verification.
		`CREATE INDEX idx_overlay_files_write_epoch ON semantic_live_overlay_files(repo_id, write_epoch)`,
		`CREATE INDEX idx_overlay_symbols_write_epoch ON semantic_live_overlay_symbols(repo_id, write_epoch)`,
		`CREATE INDEX idx_overlay_references_write_epoch ON semantic_live_overlay_references(repo_id, write_epoch)`,
		`CREATE INDEX idx_overlay_edges_write_epoch ON semantic_live_overlay_edges(repo_id, write_epoch)`,

		// Stamp the new schema version.
		`INSERT INTO semantic_schema_version (version, applied_at) VALUES (3, now())`,
	}
}

// applyMigration004 lights up Phase 63 P63-02 Task 1 / D-05 storage: a
// `last_vacuum_at TIMESTAMP DEFAULT NULL` column on
// semantic_live_overlay_meta so the compactor's VACUUM piggyback can
// store the previous run's wall-clock time and check the configured
// vacuum_interval against it without having to keep state in process
// memory across daemon restarts.
//
// MigrationKind=InPlace per Phase 57 D-02 — runs at Open time, no
// reindex, no data backfill. Existing rows take DEFAULT NULL (which
// signals "never vacuumed" → the next eligible window fires).
//
// Rollback follows the same model as Phase 60 applyMigration003:
// DuckDB's ALTER TABLE DROP COLUMN support is incomplete; downgrade
// requires the quarantine-and-rebuild path documented in Phase 57 D-04.
func applyMigration004(ctx context.Context, db *sql.DB) error {
	stmts := schema4Statements()
	for i, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("applyMigration004: stmt %d (%s): %w", i+1, firstLine(stmt), err)
		}
	}
	return nil
}

// schema4Statements returns the v3→v4 DDL: one ALTER TABLE adding the
// last_vacuum_at column, then the schema_version stamp. Two statements.
//
// Acceptance grep gates in 63-02-PLAN.md scan THIS function — keep the
// `last_vacuum_at` literal on its own logical line so per-column presence
// regexes match.
func schema4Statements() []string {
	return []string{
		// VACUUM-cadence storage column.
		`ALTER TABLE semantic_live_overlay_meta ADD COLUMN last_vacuum_at TIMESTAMP DEFAULT NULL`,

		// Stamp the new schema version.
		`INSERT INTO semantic_schema_version (version, applied_at) VALUES (4, now())`,
	}
}

// applyMigration005 lights up the snapshot-id allocation SEQUENCE (Phase
// 63 review CR-03). Pre-CR-03 BeginSnapshot allocated snapshot_id via
// `(SELECT COALESCE(MAX(snapshot_id),0)+1 FROM semantic_snapshots)`
// inside the snapshot tx — two concurrent BeginSnapshot calls (different
// repos, same store) both observed the same MAX and produced a primary-
// key collision when the second tx committed. The fix mirrors the
// `current_epoch` pattern in overlay.go:116-136: a DuckDB SEQUENCE
// allocates non-conflicting values across concurrent txs without taking
// a process-wide lock around the SELECT+INSERT.
//
// `START` is set high enough to avoid collisions with rows seeded by
// pre-migration code (test fixtures that bypass BeginSnapshot still use
// `MAX+1` directly; the SEQUENCE primes from the existing MAX so future
// BeginSnapshot allocations land above any historical row).
//
// MigrationKind=InPlace per Phase 57 D-02 — runs at Open time, no
// reindex, no data backfill.
//
// Rollback follows the same model as Phase 60 applyMigration003: DuckDB's
// DROP SEQUENCE support is incomplete; downgrade requires the
// quarantine-and-rebuild path documented in Phase 57 D-04.
func applyMigration005(ctx context.Context, db *sql.DB) error {
	// Read the existing MAX(snapshot_id) so the SEQUENCE starts above any
	// row already in semantic_snapshots (preserves uniqueness across the
	// migration boundary even if pre-migration code allocated some IDs).
	var startAt int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(snapshot_id),0)+1 FROM semantic_snapshots`).Scan(&startAt); err != nil {
		return fmt.Errorf("applyMigration005: read MAX(snapshot_id): %w", err)
	}
	if startAt < 1 {
		startAt = 1
	}
	stmts := schema5Statements(startAt)
	for i, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("applyMigration005: stmt %d (%s): %w", i+1, firstLine(stmt), err)
		}
	}
	return nil
}

// schema5Statements returns the v4→v5 DDL: CREATE SEQUENCE for
// snapshot_id allocation, then the schema_version stamp. The sequence
// is referenced by BeginSnapshot via `nextval('semantic_snapshot_id_seq')`.
func schema5Statements(startAt int64) []string {
	return []string{
		// Snapshot-id allocator — used by BeginSnapshot in place of the
		// racy MAX+1 SELECT (Phase 63 review CR-03).
		fmt.Sprintf(`CREATE SEQUENCE IF NOT EXISTS semantic_snapshot_id_seq START %d`, startAt),

		// Stamp the new schema version.
		`INSERT INTO semantic_schema_version (version, applied_at) VALUES (5, now())`,
	}
}

// firstLine returns the first non-empty trimmed line of stmt for use in
// error messages (avoids dumping multi-hundred-byte SQL on every failure).
func firstLine(stmt string) string {
	for i, r := range stmt {
		if r == '\n' {
			return stmt[:i]
		}
	}
	if len(stmt) > 80 {
		return stmt[:80]
	}
	return stmt
}
