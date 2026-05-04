//go:build cgo

package store

import (
	"context"
	"database/sql"
	"fmt"
)

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
	stmts := schema1Statements()
	for i, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("applyMigration001: stmt %d (%s): %w", i+1, firstLine(stmt), err)
		}
	}
	return nil
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
