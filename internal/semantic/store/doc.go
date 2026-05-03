// Package store is the SOLE owner of the github.com/duckdb/duckdb-go/v2
// import in this module. Phase 57 decision D-12 locks the duckdb-go module
// path to `github.com/duckdb/duckdb-go/v2` at version `v2.10502.0` (the
// canonical DuckDB Foundation Go binding repo; the historical
// `marcboeker/go-duckdb` repository was archived 2025-10-20 and MUST NOT be
// used).
//
// Boundary enforcement: the cmd/vet-noduckdb/ analyzer (Phase 57 plan P04)
// fails the build if any package outside this directory imports duckdb-go.
// The analyzer's `forbiddenImport` constant is the prefix
// `github.com/duckdb/duckdb-go`, so any future major-version bump still
// trips the gate.
//
// # Three-tier open contract (STORE-01)
//
// Open(ctx, cfg, logger, metrics) classifies the workspace's
// `<workspace>/.helix/semantic.duckdb` into one of three tiers:
//
//  1. existing+clean — file exists, opens cleanly, schema-version row is
//     present and ≤ CurrentSchemaVersion. Open returns the Store. Counter:
//     helix_semantic_store_open_total{outcome="opened"}.
//
//  2. quarantine+rebuild — file exists but is corrupt, has an unreadable
//     schema-version table, or has a forward-incompatible version. Open
//     renames the file to `<path>.corrupt.<unix-ts>` (T-57-02-02: refuses
//     to follow symlinks at the rename target), emits slog.Warn, increments
//     helix_semantic_store_quarantine_total{reason=...}, and creates a fresh
//     DB at the original path. Counter:
//     helix_semantic_store_open_total{outcome="quarantined"}.
//
//  3. hard fail — Tier-2 rebuild itself fails (e.g., disk full, permission
//     denied). Open returns an error so the daemon refuses to start.
//
// Quarantine reasons are a closed enum (D-07): {corrupt_file,
// schema_forward_incompat, schema_unreadable, unknown}.
//
// # Schema versioning (D-01, D-02, D-03)
//
// `semantic_schema_version` holds an INTEGER PRIMARY KEY column stamped to
// CurrentSchemaVersion (= 1 in P57). Each snapshot row in
// `semantic_snapshots` ALSO records `schema_version INTEGER NOT NULL` so
// readers can detect mid-rebuild inconsistency. Migrations are declared in
// migrations.go as a slice of Migration{From, To, Kind} where Kind ∈
// {InPlace, Reindex}.
//
// # CGO=0 stub policy (D-12, STORE-04)
//
// duckdb-go requires CGO. Under CGO_ENABLED=0, duckdb_nocgo.go provides a
// stub Store whose every method returns serr.ErrUnsupported. The daemon's
// step 6a refuses to start under CGO=0 (Phase 51.1), so the stub is only
// exercised in unit tests built with CGO_ENABLED=0 explicitly.
//
// # Schema 1 contract
//
// Phase 57 ships Schema 1 empty-but-correct: every SPEC §8 table is created
// (16 tables enumerated in migrations.go), `semantic_schema_version` row =
// 1, and the QueryEffective* read API returns empty results because there
// are no data write paths in P57. P59 (snapshot writes) and P60 (overlay
// writes) populate the data the read API serves.
//
// # Concurrency (T-57-02-04)
//
// DuckDB acquires its own file lock at open time. A second daemon instance
// opening the same workspace's store will get a clear error and refuse to
// start (Tier-3 hard fail). This is intentional — single-daemon-per-workspace
// is a v1.10 invariant.
package store
