package store

// CurrentSchemaVersion is the schema version stamped into
// `semantic_schema_version` by Open when creating a fresh database, and the
// upper bound for "clean reopen" classification (D-03).
//
// Phase 57 ships version 1. Future versions append entries to the migrations
// slice in this file (see Migration kind discussion).
const CurrentSchemaVersion = 1

// MigrationKind classifies a Migration entry's effect.
//
// D-02: keeping the kind explicit (rather than inferring it from From/To
// integers) makes the forward-vs-rebuild decision auditable in source.
type MigrationKind string

const (
	// MigrationInPlace adds columns / indexes / tables without rewriting
	// existing rows. Cheap; runs at Open time.
	MigrationInPlace MigrationKind = "in_place"
	// MigrationReindex requires re-extracting facts from the workspace.
	// The store ALONE cannot perform this — it returns a sentinel that the
	// daemon (or a future migration runner) handles by rebuilding.
	MigrationReindex MigrationKind = "reindex"
)

// Migration declares one version transition. Phase 57's bootstrap migration
// is From=0, To=1, Kind=InPlace; future entries append to the registry.
type Migration struct {
	From int
	To   int
	Kind MigrationKind
	// Apply is implemented in CGO=1 source files only (the migration body
	// references DuckDB SQL). The CGO=0 stub does not need a registry.
}

// TODO(P57-02 Task 2b): wire the Migration registry slice + applyMigration001
// helper into duckdb.go (CGO=1) so Open can run the bootstrap migration on
// fresh DBs and (when P58+ adds further versions) progressively apply
// in-place migrations on existing-but-old DBs.
