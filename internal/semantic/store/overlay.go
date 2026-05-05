// Package store: overlay write API (Phase 60).
//
// overlay.go owns the live-overlay-write contract:
//
//   - BeginOverlayTx(ctx, repoID) → *OverlayTx
//   - OverlayTx.UpsertOverlayFile(ctx, path, contentHash) — minimal Phase-60-P02
//     surface; the full FileFact upsert lands in Phase 60 P04 (handler).
//   - OverlayTx.MarkFileDeleted / MarkSymbolsDeleted / MarkReferencesDeleted /
//     MarkEdgesDeleted — tombstone helpers consumed by the live handler
//     (P04 HandleFileDeleted) and the watcher (P05A HandleFileDeleted via
//     P02-style tombstone). Single owner of the overlay write API per
//     CONTEXT.md domain item #4.
//   - OverlayTx.Commit / Rollback / Epoch
//   - FlushOverlay(ctx) — no-op today; reserved for future cooperative drain.
//
// D-04 epoch contract (60-CONTEXT.md): every BeginOverlayTx atomically
// allocates a fresh, monotone, per-workspace write_epoch. Concurrent calls on
// the SAME workspace serialize via a per-workspace mutex; calls on DIFFERENT
// workspaces do NOT serialize. Rollback does NOT rewind the epoch — the
// counter is monotone-forever; a rolled-back epoch is just an unused slot.
//
// The increment is issued on s.db (NOT inside the per-tx *sql.Tx) so it
// commits independently of the user-visible transaction. This is the load-
// bearing detail for Phase 63's compaction CAS read; getting it wrong here =
// silent overlay row loss during compaction (Pitfall 60-RESEARCH.md).

package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

// OverlayTx is the per-tx handle returned by BeginOverlayTx. Each tx carries
// a freshly-allocated, monotone, per-workspace write_epoch (D-04) that is
// stamped onto every fact row written through this handle.
//
// Lifecycle: the caller MUST terminate via Commit() or Rollback(). Both
// release the per-workspace mutex. Failing to terminate leaks the mutex and
// blocks all subsequent same-workspace BeginOverlayTx calls forever.
type OverlayTx struct {
	tx     *sql.Tx
	repoID string
	epoch  uint64
	// unlock releases the per-workspace mutex acquired by BeginOverlayTx.
	// It is invoked exactly once by Commit() or Rollback() (defer-style)
	// regardless of underlying tx outcome, so the lock is always freed.
	unlock     func()
	unlockOnce sync.Once
}

// Epoch returns the write_epoch stamped on every fact row written through
// this tx. Callers may inspect it for logging / metrics; the writer methods
// stamp it automatically.
func (t *OverlayTx) Epoch() uint64 { return t.epoch }

// RepoID returns the workspace identifier this tx is bound to.
func (t *OverlayTx) RepoID() string { return t.repoID }

// BeginOverlayTx opens an overlay write transaction for repoID. Allocates a
// fresh monotone write_epoch under the per-workspace lock and stamps it on
// every fact row written through the returned *OverlayTx.
//
// Cross-workspace BeginOverlayTx calls do NOT serialize (D-04 invariant).
// Same-workspace concurrent calls serialize and each receive a unique
// monotone epoch (verified under -race by TestOverlayEpochConcurrent).
//
// The returned OverlayTx MUST be terminated via Commit() or Rollback().
// Rollback does NOT rewind the epoch (D-04 invariant: "if the tx rolls back,
// that epoch is just unused" — the counter is monotone-forever).
//
// The epoch bump (`UPDATE ... SET current_epoch = current_epoch + 1
// RETURNING current_epoch`) is issued on s.db, NOT on the per-tx *sql.Tx,
// so it commits independently of the user-visible transaction. This is the
// T-60-02-05 mitigation: rollback of the OverlayTx must NOT rewind the
// epoch counter.
func (s *Store) BeginOverlayTx(ctx context.Context, repoID string) (*OverlayTx, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("BeginOverlayTx: nil store")
	}
	if repoID == "" {
		return nil, fmt.Errorf("BeginOverlayTx: empty repoID")
	}

	// Acquire (or lazily create) the per-workspace mutex. The outer mutex
	// guards the map; we hold it only long enough to look up / install the
	// inner mutex so the registry itself never serializes cross-workspace
	// callers.
	mu := s.overlayLockFor(repoID)
	mu.Lock()

	// D-04 LOCKED: epoch bump runs OUTSIDE the user-visible *sql.Tx so
	// rollback does NOT rewind it. The per-workspace mutex (just acquired)
	// provides serialization. Ensure-meta-row + bump are issued on s.db.
	//
	// The meta row carries Phase-57 NOT-NULL columns (base_snapshot_id,
	// graph_version, freshness, overlay_file_count, pending_lsp_count,
	// updated_at) that pre-date the epoch contract. We supply minimal
	// initial values (0 / "unknown" / now()) for the bootstrap insert; the
	// freshness-tracker (Phase 60 P03) is the canonical owner of these
	// columns and will refresh them on every overlay write. The
	// ON CONFLICT clause makes this a no-op when the row already exists.
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO semantic_live_overlay_meta (
			repo_id, base_snapshot_id, graph_version, freshness,
			overlay_file_count, pending_lsp_count, updated_at, current_epoch
		) VALUES (?, 0, 0, 'unknown', 0, 0, now(), 0)
		ON CONFLICT (repo_id) DO NOTHING
	`, repoID); err != nil {
		mu.Unlock()
		return nil, fmt.Errorf("BeginOverlayTx: ensure meta row for %q: %w", repoID, err)
	}

	var epoch uint64
	if err := s.db.QueryRowContext(ctx, `
		UPDATE semantic_live_overlay_meta
		   SET current_epoch = current_epoch + 1
		 WHERE repo_id = ?
		 RETURNING current_epoch
	`, repoID).Scan(&epoch); err != nil {
		mu.Unlock()
		return nil, fmt.Errorf("BeginOverlayTx: bump+read epoch for %q: %w", repoID, err)
	}

	// Open the user-visible tx AFTER the epoch is committed. If BeginTx
	// fails, the epoch slot is just unused (monotone-forever invariant).
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		mu.Unlock()
		return nil, fmt.Errorf("BeginOverlayTx: open tx: %w", err)
	}

	return &OverlayTx{
		tx:     tx,
		repoID: repoID,
		epoch:  epoch,
		unlock: mu.Unlock,
	}, nil
}

// overlayLockFor returns the per-workspace mutex for repoID, lazily
// installing one if not already present. Callers MUST take the returned
// mutex (Lock); the outer registry mutex is released before returning so
// cross-workspace BeginOverlayTx calls never serialize.
func (s *Store) overlayLockFor(repoID string) *sync.Mutex {
	s.overlayLocksMu.Lock()
	defer s.overlayLocksMu.Unlock()
	if s.overlayLocks == nil {
		s.overlayLocks = map[string]*sync.Mutex{}
	}
	mu, ok := s.overlayLocks[repoID]
	if !ok {
		mu = &sync.Mutex{}
		s.overlayLocks[repoID] = mu
	}
	return mu
}

// Commit finalizes the tx and releases the per-workspace lock.
func (t *OverlayTx) Commit() error {
	defer t.releaseLock()
	return t.tx.Commit()
}

// Rollback aborts the tx and releases the per-workspace lock. The
// current_epoch increment that opened the tx remains committed (D-04:
// monotone-forever, never rolled back; the epoch slot is just unused).
func (t *OverlayTx) Rollback() error {
	defer t.releaseLock()
	return t.tx.Rollback()
}

// releaseLock fires the unlock callback exactly once. Belt-and-suspenders:
// callers should invoke Commit() XOR Rollback(); if a defensive double-call
// happens we tolerate it without panicking on a double-unlock.
func (t *OverlayTx) releaseLock() {
	if t == nil || t.unlock == nil {
		return
	}
	t.unlockOnce.Do(t.unlock)
}

// FlushOverlay is a no-op today; reserved for future cooperative drain
// semantics if Phase 63's compaction pipeline needs them. Always returns
// nil. Callers may invoke at shutdown without harm.
//
// The overlay writer commits per-tx; there is no buffered state to drain.
// Phase 63 may extend the contract to (e.g.) wait for in-flight transactions
// to land before opening a compaction window — the API stub exists today so
// the call site can be wired without a future signature break.
func (s *Store) FlushOverlay(_ context.Context) error { return nil }

// UpsertOverlayFile inserts (or replaces) one row in semantic_live_overlay_files
// for this tx's (repo_id, path), stamped with this tx's write_epoch. The
// minimal payload set (path, content_hash, file_id=0, language='', status='live')
// is what Phase 60 P02 ships for the epoch contract test; the full FileFact
// upsert (real file_id from semantic_files, language detection) is filled by
// Phase 60 P04 (the live handler).
//
// status='live' marks the row as a non-tombstone; tombstone semantics are
// supplied by MarkFileDeleted below (status='deleted').
func (t *OverlayTx) UpsertOverlayFile(ctx context.Context, path, contentHash string) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("UpsertOverlayFile: nil tx")
	}
	_, err := t.tx.ExecContext(ctx, `
		INSERT INTO semantic_live_overlay_files (
			repo_id, path, file_id, content_hash, language, status, updated_at, write_epoch
		) VALUES (?, ?, 0, ?, '', 'live', now(), ?)
		ON CONFLICT (repo_id, path) DO UPDATE SET
			content_hash = excluded.content_hash,
			status       = excluded.status,
			updated_at   = excluded.updated_at,
			write_epoch  = excluded.write_epoch
	`, t.repoID, path, contentHash, t.epoch)
	if err != nil {
		return fmt.Errorf("UpsertOverlayFile(%q, %q): %w", t.repoID, path, err)
	}
	return nil
}

// MarkFileDeleted writes a tombstone row for (repoID, path). Sets
// status='deleted' and stamps this tx's write_epoch.
//
// Symbol/reference/edge tombstones cascade through the corresponding Mark*
// helpers below; the watcher / live handler (Phase 60 P04 / P05A) may invoke
// them when classifier outputs determine the granularity of deletion. P04's
// HandleFileDeleted is the canonical caller (CONTEXT.md domain item #4 —
// P02 is the SOLE OWNER of the overlay write API; P04 consumes).
func (t *OverlayTx) MarkFileDeleted(ctx context.Context, path string) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("MarkFileDeleted: nil tx")
	}
	_, err := t.tx.ExecContext(ctx, `
		INSERT INTO semantic_live_overlay_files (
			repo_id, path, file_id, content_hash, language, status, updated_at, write_epoch
		) VALUES (?, ?, 0, '', '', 'deleted', now(), ?)
		ON CONFLICT (repo_id, path) DO UPDATE SET
			status       = 'deleted',
			updated_at   = now(),
			write_epoch  = excluded.write_epoch
	`, t.repoID, path, t.epoch)
	if err != nil {
		return fmt.Errorf("MarkFileDeleted(%q, %q): %w", t.repoID, path, err)
	}
	return nil
}

// MarkSymbolsDeleted tombstones every overlay-symbol row whose file_id
// matches one of the supplied IDs. The classifier (P04) supplies file_ids
// resolved against semantic_files; on a bare path-deletion the handler
// resolves the live or base file_id before invoking this helper.
//
// Empty list is a no-op (returns nil) — saves a round-trip when the
// classifier resolves zero file_ids.
//
// Implementation note: DuckDB supports list parameters via the duckdb-go
// list binding, but the database/sql interface gives us inconsistent results
// across DuckDB driver versions for `IN (?)` with slice params. We iterate
// per-id to keep the SQL portable; for the realistic per-file fan-out (≤ a
// few hundred symbols on typical edits) the overhead is negligible.
func (t *OverlayTx) MarkSymbolsDeleted(ctx context.Context, fileIDs []uint64) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("MarkSymbolsDeleted: nil tx")
	}
	if len(fileIDs) == 0 {
		return nil
	}
	for _, id := range fileIDs {
		if _, err := t.tx.ExecContext(ctx, `
			UPDATE semantic_live_overlay_symbols
			   SET status      = 'deleted',
			       write_epoch = ?,
			       updated_at  = now()
			 WHERE repo_id = ? AND file_id = ?
		`, t.epoch, t.repoID, id); err != nil {
			return fmt.Errorf("MarkSymbolsDeleted(%q, file_id=%d): %w", t.repoID, id, err)
		}
	}
	return nil
}

// MarkReferencesDeleted is the references-table analog of MarkSymbolsDeleted.
// Empty list is a no-op.
func (t *OverlayTx) MarkReferencesDeleted(ctx context.Context, fileIDs []uint64) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("MarkReferencesDeleted: nil tx")
	}
	if len(fileIDs) == 0 {
		return nil
	}
	for _, id := range fileIDs {
		if _, err := t.tx.ExecContext(ctx, `
			UPDATE semantic_live_overlay_references
			   SET status      = 'deleted',
			       write_epoch = ?,
			       updated_at  = now()
			 WHERE repo_id = ? AND file_id = ?
		`, t.epoch, t.repoID, id); err != nil {
			return fmt.Errorf("MarkReferencesDeleted(%q, file_id=%d): %w", t.repoID, id, err)
		}
	}
	return nil
}

// partialReasonClosedEnum is the set of valid partial_reason values that
// MarkFileSemanticPending accepts. Phase 61 D-05/D-06; mirrored in the
// closed-enum doc comment in migrations.go schema2Statements.
//
// "budget exhausted" is carried over from Phase 59 D-05 (per-file timeout /
// max-symbols cap); the other three are introduced by Phase 61. The string
// "budget exhausted" intentionally contains a space (matches the existing
// Phase 59 fact-emitter literal) — callers who pass "budget_exhausted"
// (underscore form) will get the closed-enum reject.
var partialReasonClosedEnum = map[string]struct{}{
	"preempted":           {},
	"bulk_update_pending": {},
	"lsp_unavailable":     {},
	"budget exhausted":    {},
}

// MarkFileSemanticPending stamps semantic_files for (t.repoID, path) with
// extraction_partial=true and partial_reason=reason. Reason MUST be one of
// the closed-enum values (see partialReasonClosedEnum); unknown reasons
// return an error without writing.
//
// Schema unchanged — partial_reason TEXT and extraction_partial BOOLEAN
// columns were added in Phase 59 migration 002. The column on semantic_files
// is named extraction_partial (NOT bare `partial`); the bare `partial`
// column lives on semantic_symbols and semantic_references. See
// migrations.go schema2Statements for the column inventory.
//
// Update semantics: WHERE repo_id=? AND path=? — this matches every
// snapshot row for the path. Semantically: when a file is marked pending
// for LSP enrichment we are stating "the structural facts emitted for this
// path (across all snapshots) are partial pending re-enrichment" — that is
// the correct cross-snapshot semantics for the freshness-tracker.
//
// When no row exists for (repo_id, path) the UPDATE affects zero rows and
// MarkFileSemanticPending returns nil. This is a deliberate no-op rather
// than an error: enrichment may be triggered for paths the Phase 59
// extractor has not yet processed (e.g., a new file in a workspace that
// hasn't been re-scanned), and the next extraction cycle will surface the
// row with the correct extraction state.
//
// Phase 61 D-05.
func (t *OverlayTx) MarkFileSemanticPending(ctx context.Context, path, reason string) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("MarkFileSemanticPending: nil tx")
	}
	if _, ok := partialReasonClosedEnum[reason]; !ok {
		return fmt.Errorf(
			"MarkFileSemanticPending: unknown reason %q (allowed: preempted, bulk_update_pending, lsp_unavailable, %q)",
			reason, "budget exhausted")
	}
	_, err := t.tx.ExecContext(ctx, `
		UPDATE semantic_files
		   SET extraction_partial = TRUE,
		       partial_reason     = ?
		 WHERE repo_id = ? AND path = ?
	`, reason, t.repoID, path)
	if err != nil {
		return fmt.Errorf("MarkFileSemanticPending(%q, %q, %q): %w", t.repoID, path, reason, err)
	}
	return nil
}

// MarkEdgesDeleted tombstones every overlay-edge row whose src_node_id OR
// dst_node_id matches one of the supplied node IDs. Empty list is a no-op.
//
// Edges are not directly attributable to a single file_id (they connect two
// nodes that may live in different files), so the handler must resolve
// affected node IDs and pass them here rather than file IDs.
func (t *OverlayTx) MarkEdgesDeleted(ctx context.Context, nodeIDs []uint64) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("MarkEdgesDeleted: nil tx")
	}
	if len(nodeIDs) == 0 {
		return nil
	}
	for _, id := range nodeIDs {
		if _, err := t.tx.ExecContext(ctx, `
			UPDATE semantic_live_overlay_edges
			   SET status      = 'deleted',
			       write_epoch = ?,
			       updated_at  = now()
			 WHERE repo_id = ?
			   AND (src_node_id = ? OR dst_node_id = ?)
		`, t.epoch, t.repoID, id, id); err != nil {
			return fmt.Errorf("MarkEdgesDeleted(%q, node_id=%d): %w", t.repoID, id, err)
		}
	}
	return nil
}
