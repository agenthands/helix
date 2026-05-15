package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrForwardIncompatible is the sentinel returned by runMigrations (and
// surfaced by Open) when the on-disk schema_version exceeds the binary's
// CurrentSchemaVersion. Open propagates this error rather than falling
// through to the quarantine-and-rebuild path so an operator running a
// downgraded binary against a newer DB sees an explicit failure they can
// fix (rebuild via the documented quarantine path) instead of silent data
// loss from automatic quarantine.
var ErrForwardIncompatible = errors.New("semantic store: schema_version is forward-incompatible with binary CurrentSchemaVersion; rebuild required")

// migrations is the registry of in-place schema transitions, applied in From
// ascending order at Open time. Phase 59 lights up the mechanism for the
// first time: applyMigration001 (Phase 57 bootstrap) and applyMigration002
// (Phase 59 partial-extraction columns) are bound here.
//
// Adding a new version: append a new Migration entry, bump
// CurrentSchemaVersion in migrations_types.go, and write the corresponding
// applyMigration00N body in migrations.go.
var migrations = []Migration{
	{From: 0, To: 1, Kind: MigrationInPlace, Apply: applyMigration001},
	{From: 1, To: 2, Kind: MigrationInPlace, Apply: applyMigration002},
	{From: 2, To: 3, Kind: MigrationInPlace, Apply: applyMigration003},
	{From: 3, To: 4, Kind: MigrationInPlace, Apply: applyMigration004},
	{From: 4, To: 5, Kind: MigrationInPlace, Apply: applyMigration005},
	{From: 5, To: 6, Kind: MigrationInPlace, Apply: applyMigration006},
}

// runMigrations progressively applies every Migration in the registry whose
// From >= currentVersion (read fresh from semantic_schema_version) and whose
// To <= CurrentSchemaVersion. After each Apply it re-reads schema_version
// and asserts the body actually stamped the new version (defensive: catches
// a migration body that forgot to INSERT the version row).
//
// Forward-incompatibility check: if the on-disk schema_version is greater
// than the binary's CurrentSchemaVersion, runMigrations returns an explicit
// "forward-incompatible" error and refuses to proceed. This preserves the
// Phase 57 STORE-03 invariant inside the registry loop as a defensive
// double-check; the primary forward-incompat handling lives in
// classifyExisting (which quarantines+rebuilds on Open).
//
// On a fresh DB the schema_version table does not exist yet → currentVersion
// returns 0 → migration 0→1 (creates the table among everything else) runs
// first, then 1→2.
func runMigrations(ctx context.Context, db *sql.DB) error {
	currentVersion, err := readSchemaVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("runMigrations: read current schema_version: %w", err)
	}
	if currentVersion > CurrentSchemaVersion {
		return fmt.Errorf("%w: stored=%d binary=%d", ErrForwardIncompatible, currentVersion, CurrentSchemaVersion)
	}

	for _, m := range migrations {
		if m.From < currentVersion {
			continue
		}
		if m.To > CurrentSchemaVersion {
			continue
		}
		if m.Apply == nil {
			return fmt.Errorf("runMigrations: migration %d→%d has nil Apply (registry not wired)", m.From, m.To)
		}
		if err := m.Apply(ctx, db); err != nil {
			return fmt.Errorf("runMigrations: %d→%d: %w", m.From, m.To, err)
		}
		// Defensive re-read: ensure the migration body stamped the new
		// version row.
		got, rerr := readSchemaVersion(ctx, db)
		if rerr != nil {
			return fmt.Errorf("runMigrations: re-read schema_version after %d→%d: %w", m.From, m.To, rerr)
		}
		if got != m.To {
			return fmt.Errorf("runMigrations: %d→%d completed but schema_version=%d (expected %d); migration body forgot to stamp version", m.From, m.To, got, m.To)
		}
		currentVersion = got
	}
	return nil
}

// readSchemaVersion returns the maximum value in semantic_schema_version, or
// 0 if the table does not exist (or is empty). Multiple rows can accumulate
// over time as each migration appends a row; the canonical "current" version
// is the maximum.
func readSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	row := db.QueryRowContext(ctx, "SELECT max(version) FROM semantic_schema_version")
	if err := row.Scan(&version); err != nil {
		// Table missing → treat as version 0 (fresh DB). DuckDB surfaces
		// missing-table as an error; we collapse to 0 only when the error
		// message indicates a missing table or no-rows. Any unexpected
		// error is propagated.
		if isMissingTableErr(err) {
			return 0, nil
		}
		return 0, err
	}
	return version, nil
}

// isMissingTableErr returns true when err looks like DuckDB reporting that
// semantic_schema_version does not exist. DuckDB messages are not stable
// strings across versions, so we substring-match the most common shapes.
func isMissingTableErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, needle := range []string{
		"does not exist",
		"no such table",
		"Table with name semantic_schema_version does not exist",
	} {
		if containsFold(msg, needle) {
			return true
		}
	}
	return false
}

// containsFold is a tiny case-insensitive substring helper to avoid pulling
// in strings.EqualFold on every Scan error path.
func containsFold(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}
	// Lowercase both halves once and use a simple substring search.
	lh := make([]byte, len(haystack))
	for i := 0; i < len(haystack); i++ {
		c := haystack[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		lh[i] = c
	}
	ln := make([]byte, len(needle))
	for i := 0; i < len(needle); i++ {
		c := needle[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		ln[i] = c
	}
	for i := 0; i+len(ln) <= len(lh); i++ {
		match := true
		for j := 0; j < len(ln); j++ {
			if lh[i+j] != ln[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
