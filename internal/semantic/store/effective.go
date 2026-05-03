package store

// effective.go is the home of the snapshot ⊕ overlay − tombstones read API
// declared in SPEC-DRAFT.md §10. Phase 57 ships the API surface on Schema 1
// (empty-but-correct: every query returns an empty result because no data
// has been written yet). The CGO=1 implementation lives in duckdb.go
// (Task 2b); the CGO=0 stub returns serr.ErrUnsupported from the same
// signatures (duckdb_nocgo.go).
//
// Read API (mirrors SPEC §10):
//
//   - QueryEffectiveFiles(ctx, repoID, path) → file fact merged across
//     snapshot + overlay, minus any tombstoned overlay file.
//   - QueryEffectiveSymbols(ctx, query) → symbol facts.
//   - QueryEffectiveReferences(ctx, query) → reference facts.
//   - QueryEffectiveEdges(ctx, query) → edge facts.
//
// All four queries follow the same shape: read the snapshot rows for the
// requested key, EXCEPT any rows tombstoned in the overlay, UNION the
// overlay's live rows. P59 writes the snapshot side; P60 writes the overlay
// side; P57 ships the read API contract so downstream tooling (P64+) can
// take a stable dependency on the signatures.
//
// The actual implementations live in duckdb.go and duckdb_nocgo.go; this
// file holds shared types and documentation only so the CGO=0 stub does not
// need to redeclare query-input/result structs.
