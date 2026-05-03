package store

// overlay.go is the future home of the live-overlay-write API (P60):
//
//   - UpsertOverlayFile(ctx, FileFact) error
//   - UpsertOverlaySymbol(ctx, SymbolFact) error
//   - DeleteOverlayFile(ctx, FileID) error  // tombstone
//   - FlushOverlay(ctx) error               // periodic / on-shutdown drain
//
// Phase 57 ships Schema 1 empty-but-correct (no overlay write paths). Plan
// P60 will fill this file with the fsnotify-driven upsert helpers; the
// Schema 1 tables semantic_live_overlay_{meta,files,symbols,references,edges}
// already exist so P60 has somewhere to write.
//
// TODO(P60): implement overlay write path on top of database/sql + DuckDB
// transactional UPSERTs. The Phase 57 task is intentionally read-only.
