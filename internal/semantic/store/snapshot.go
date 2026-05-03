package store

// snapshot.go is the future home of the snapshot-write API (P59):
//
//   - BeginSnapshot(ctx) (SnapshotID, error)
//   - InsertFiles / InsertSymbols / InsertReferences / InsertEdges
//   - CommitSnapshot(ctx, SnapshotID) error
//   - AbortSnapshot(ctx, SnapshotID) error
//
// Phase 57 ships Schema 1 empty-but-correct (no write paths). Plan P59 will
// fill this file with the typed insert helpers; P63 (compaction) will own
// snapshot retention.
//
// TODO(P59): implement snapshot write path on top of database/sql + DuckDB
// COPY/INSERT. The Phase 57 task is intentionally read-only.
