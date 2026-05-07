package store

import (
	"context"
	"database/sql"
)

// CurrentSchemaVersion is the schema version stamped into
// `semantic_schema_version` by Open when creating a fresh database, and the
// upper bound for "clean reopen" classification (D-03).
//
// Phase 57 shipped version 1. Phase 59 lights up the registry mechanism for
// the first time and bumps to version 2 (the partial-extraction columns
// prescribed by 59-CONTEXT.md D-05). Phase 60 lights up v2→v3 (live-overlay
// epoch + write_epoch stamps; 60-CONTEXT.md D-04). Phase 63 P63-02 Task 1
// adds v3→v4 (semantic_live_overlay_meta.last_vacuum_at column for the
// VACUUM-cadence storage; 63-CONTEXT.md D-05). Phase 63 review CR-03 adds
// v4→v5 (semantic_snapshot_id_seq SEQUENCE: replaces the racy MAX+1
// allocation in BeginSnapshot). Future versions append entries to the
// migrations slice (see migrations_registry.go).
const CurrentSchemaVersion = 5

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
// is From=0, To=1, Kind=InPlace; Phase 59 adds From=1, To=2, Kind=InPlace
// for the partial-extraction column delta (59-CONTEXT.md D-05).
//
// Apply is the migration body. It is bound only in CGO=1 source files
// (see migrations_registry_cgo.go) because the bodies reference DuckDB SQL.
// The CGO=0 build does not exercise the registry — Open under !cgo returns
// serr.ErrUnsupported before any migration runs.
type Migration struct {
	From  int
	To    int
	Kind  MigrationKind
	Apply func(ctx context.Context, db *sql.DB) error
}
