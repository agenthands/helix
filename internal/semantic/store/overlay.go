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
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/agenthands/helix/internal/workspace"
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

	// Phase 63 P63-02 Task 1: closure into the parent *Store's
	// overlayPendingRowsFor(repoID).Add(1) so each overlay-row write
	// increments the in-memory counter consumed by the compaction gate
	// (BlockedOverlayEmpty). Nil-safe: when set to nil (test fakes that
	// don't go through BeginOverlayTx) bumpPending becomes a no-op.
	bumpPendingFn func()
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

	// Phase 63 P63-02 Task 1: bump the per-workspace open-tx counter and
	// wrap mu.Unlock so the counter decrements in lockstep with the lock
	// release. The counter feeds the compaction gate's
	// BlockedOverlayTxActive check (in-memory atomic; no I/O).
	txCounter := s.overlayTxCountFor(repoID)
	txCounter.Add(1)
	unlock := func() {
		txCounter.Add(-1)
		mu.Unlock()
	}

	// Phase 63 P63-02 Task 1: pending-rows bumper closure. Consumed by
	// BlockedOverlayEmpty (in-memory proxy). Reset to 0 inside
	// Snapshot.ClearOverlayLE after the per-table DELETEs.
	pendingCounter := s.overlayPendingRowsFor(repoID)
	bumpPendingFn := func() { pendingCounter.Add(1) }

	return &OverlayTx{
		tx:            tx,
		repoID:        repoID,
		epoch:         epoch,
		unlock:        unlock,
		bumpPendingFn: bumpPendingFn,
	}, nil
}

// overlayTxCountFor returns the lazy-installed open-tx counter for repoID.
// Phase 63 P63-02 Task 1: feeds BlockedOverlayTxActive without I/O.
func (s *Store) overlayTxCountFor(repoID string) *atomic.Int32 {
	s.overlayCountsMu.Lock()
	defer s.overlayCountsMu.Unlock()
	if s.overlayTxCounts == nil {
		s.overlayTxCounts = map[string]*atomic.Int32{}
	}
	c, ok := s.overlayTxCounts[repoID]
	if !ok {
		c = &atomic.Int32{}
		s.overlayTxCounts[repoID] = c
	}
	return c
}

// overlayPendingRowsFor returns the lazy-installed pending-rows counter
// for repoID. Phase 63 P63-02 Task 1: in-memory proxy for
// "OverlayHasPendingRows" — bumped on every successful overlay row write,
// reset to 0 inside Snapshot.ClearOverlayLE after the per-table DELETEs
// remove rows. Backs the gate's BlockedOverlayEmpty check without I/O.
func (s *Store) overlayPendingRowsFor(repoID string) *atomic.Int64 {
	s.overlayCountsMu.Lock()
	defer s.overlayCountsMu.Unlock()
	if s.overlayPendingRows == nil {
		s.overlayPendingRows = map[string]*atomic.Int64{}
	}
	c, ok := s.overlayPendingRows[repoID]
	if !ok {
		c = &atomic.Int64{}
		s.overlayPendingRows[repoID] = c
	}
	return c
}

// OverlayTxOpenCount returns the number of currently-open OverlayTx
// handles for ws.RepoRoot. Phase 63 P63-02 Task 1: O(1) atomic read; the
// gate consumes this for BlockedOverlayTxActive without I/O.
func (s *Store) OverlayTxOpenCount(ws workspace.WorkspaceKey) int {
	if s == nil {
		return 0
	}
	s.overlayCountsMu.Lock()
	c, ok := s.overlayTxCounts[ws.RepoRoot]
	s.overlayCountsMu.Unlock()
	if !ok || c == nil {
		return 0
	}
	return int(c.Load())
}

// OverlayHasPendingRows reports whether the in-memory pending-rows
// counter for repoID is positive. Phase 63 P63-02 Task 1: O(1) atomic
// read; CONTEXT.md D-04 hard invariant — NO I/O. Consumed by the
// compaction gate's BlockedOverlayEmpty check.
//
// Bumped on every successful overlay row write (UpsertOverlayFile,
// MarkFileDeleted, MarkSymbolsDeleted, MarkReferencesDeleted,
// MarkEdgesDeleted) by Add(1). Reset to 0 by Snapshot.ClearOverlayLE
// AFTER the per-table DELETEs remove rows.
func (s *Store) OverlayHasPendingRows(repoID string) bool {
	if s == nil {
		return false
	}
	s.overlayCountsMu.Lock()
	c, ok := s.overlayPendingRows[repoID]
	s.overlayCountsMu.Unlock()
	if !ok || c == nil {
		return false
	}
	return c.Load() > 0
}

// OverlayRowCount issues the I/O-bound `SELECT COUNT(*) ... WHERE
// write_epoch <= ?` per overlay table and sums the four results. Phase
// 63 P63-02 Task 1: this is the SIZE GUARD path used ONLY by the
// compactor's pre-flight (runCompaction) — it is NEVER consumed by the
// gate. The gate uses OverlayHasPendingRows (in-memory) per CONTEXT.md
// D-04 hard invariant.
//
// Bound by capturedEpoch so rows committed during the compaction window
// (write_epoch > capturedEpoch) are excluded — same CAS contract as
// Snapshot.ClearOverlayLE.
func (s *Store) OverlayRowCount(ctx context.Context, repoID string, capturedEpoch uint64) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("OverlayRowCount: nil store")
	}
	if repoID == "" {
		return 0, fmt.Errorf("OverlayRowCount: empty repoID")
	}
	queries := []struct {
		table string
		sql   string
	}{
		{"semantic_live_overlay_files", `SELECT COUNT(*) FROM semantic_live_overlay_files WHERE repo_id = ? AND write_epoch <= ?`},
		{"semantic_live_overlay_symbols", `SELECT COUNT(*) FROM semantic_live_overlay_symbols WHERE repo_id = ? AND write_epoch <= ?`},
		{"semantic_live_overlay_references", `SELECT COUNT(*) FROM semantic_live_overlay_references WHERE repo_id = ? AND write_epoch <= ?`},
		{"semantic_live_overlay_edges", `SELECT COUNT(*) FROM semantic_live_overlay_edges WHERE repo_id = ? AND write_epoch <= ?`},
	}
	total := 0
	for _, q := range queries {
		var n int
		if err := s.queryRowContext(ctx, q.sql, repoID, capturedEpoch).Scan(&n); err != nil {
			return 0, fmt.Errorf("OverlayRowCount(%s): %w", q.table, err)
		}
		total += n
	}
	return total, nil
}

// Checkpoint issues a DuckDB CHECKPOINT statement on the store handle
// (outside any tx). Phase 63 P63-02 Task 1: invoked by the compactor
// AFTER CommitSnapshot to flush the WAL — bounds .duckdb growth across
// repeated commit cycles (COMPACT-03 invariant).
//
// Returned errors are non-fatal at the caller level (compactor logs and
// continues); the next idle window retries. nil-safe.
func (s *Store) Checkpoint(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("Checkpoint: nil store")
	}
	if _, err := s.db.ExecContext(ctx, "CHECKPOINT"); err != nil {
		return fmt.Errorf("Checkpoint: %w", err)
	}
	return nil
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

// bumpPending increments the per-workspace pending-rows counter through
// the parent *Store. Phase 63 P63-02 Task 1: consumed by the compaction
// gate's BlockedOverlayEmpty check. Called by every overlay-write path
// (UpsertOverlayFile / MarkFileDeleted / Mark*Deleted) after a successful
// ExecContext.
//
// We need a back-reference to *Store; OverlayTx doesn't currently hold
// one — but BeginOverlayTx is the only constructor and it has access.
// Rather than threading a *Store pointer through OverlayTx (and disturbing
// the existing struct shape), we close over `bumpPending` via the unlock
// closure path: the unlock func already captures `txCounter` (the
// per-workspace open-tx counter). We do the same for pending rows by
// stamping a `bumpPendingFn` field on OverlayTx. Below the path is a
// no-op when bumpPendingFn is nil (test paths that construct an
// OverlayTx without going through BeginOverlayTx).
func (t *OverlayTx) bumpPending() {
	if t == nil || t.bumpPendingFn == nil {
		return
	}
	t.bumpPendingFn()
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
// minimal payload set (path, content_hash, file_id=0, language=”, status='live')
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
	t.bumpPending()
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
	t.bumpPending()
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
		t.bumpPending()
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
		t.bumpPending()
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
		t.bumpPending()
	}
	return nil
}

// ScoreRow is the per-node ranked score row written by Phase 62 P02
// UpsertGraphScores. The semantic_graph_scores schema (SPEC §9.9 / migration
// 001 lines 248-263) keys on (repo_id, graph_version, node_id, score_name);
// we map score_name from the Phase 62 "projection" identifier.
//
// Status is the closed-enum write-time value: "exact" | "approximate" |
// "stale". "missing" is computed at READ time only (D-07) and is rejected
// at the storage boundary.
type ScoreRow struct {
	NodeID       uint64
	Score        float64
	GraphVersion uint64
	Status       string // "exact" | "approximate" | "stale"
}

// EdgeRow is the per-edge upsert row used by UpsertEdgesWithMerge. The merge
// predicate is (src_node_id, dst_node_id, edge_kind). Source carries the
// provenance prefix ("lsp.<call>" or "comment.<kind>") that drives the D-14
// refutation logic.
type EdgeRow struct {
	SrcNodeID, DstNodeID uint64
	EdgeKind             string  // e.g., "CALLS", "RESOLVES_TO"
	Source               string  // "lsp.<call>" | "comment.<kind>"
	Confidence           float64 // 1.0 for LSP, 0.60 for comment, etc.
	Weight               float64
	ValidationState      string // "validated" | "unresolved"
	FactJSON             []byte // optional; nil writes JSON NULL
}

// scoreStatusWriteEnum is the closed-enum allowlist for ScoreRow.Status at
// write time. "missing" is intentionally absent — D-07: it is computed at
// read time when no row exists.
var scoreStatusWriteEnum = map[string]struct{}{
	"exact":       {},
	"approximate": {},
	"stale":       {},
}

// UpsertGraphScores writes ranked-score rows under (repo_id, projection,
// node_id). Empty rows is a no-op. Each row's Status MUST be one of
// {"exact","approximate","stale"} — D-07 closed enum at the write boundary.
//
// D-06 invariant: this method does NOT advance graph_version. Only
// Engine.ApplyRepair (via OverlayTx.BumpGraphVersion) advances it.
//
// snapshot_id is set to 0 — Phase 62 score rows are overlay-side state
// keyed on (repo_id, graph_version). The base-snapshot relationship is
// re-established by Phase 63 compaction.
func (t *OverlayTx) UpsertGraphScores(ctx context.Context, projection string, rows []ScoreRow) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("UpsertGraphScores: nil tx")
	}
	if projection == "" {
		return fmt.Errorf("UpsertGraphScores: empty projection")
	}
	if len(rows) == 0 {
		return nil
	}
	for _, r := range rows {
		if _, ok := scoreStatusWriteEnum[r.Status]; !ok {
			return fmt.Errorf(
				"UpsertGraphScores: invalid status %q (allowed: exact, approximate, stale; missing is read-time only)",
				r.Status)
		}
		if _, err := t.tx.ExecContext(ctx, `
			INSERT INTO semantic_graph_scores (
				repo_id, snapshot_id, graph_version, node_id, score_name,
				score, rank, status, computed_at, algorithm_version
			) VALUES (?, 0, ?, ?, ?, ?, NULL, ?, now(), 'phase62.p02')
			ON CONFLICT (repo_id, graph_version, node_id, score_name) DO UPDATE SET
				score             = excluded.score,
				status            = excluded.status,
				computed_at       = excluded.computed_at,
				algorithm_version = excluded.algorithm_version
		`, t.repoID, r.GraphVersion, r.NodeID, projection, r.Score, r.Status); err != nil {
			return fmt.Errorf("UpsertGraphScores(%q, %q, node=%d): %w", t.repoID, projection, r.NodeID, err)
		}
	}
	return nil
}

// UpsertEdgesWithMerge enforces the D-14 (src_node_id, dst_node_id,
// edge_kind) merge predicate at the SQL boundary. LSP rows ALWAYS win:
//
//   - If the incoming row's source matches "lsp.%": DELETE every existing
//     comment.* row for the same (src_node_id, edge_kind) pair regardless of
//     dst_node_id (the D-14 refutation rule — comment edges must NOT survive
//     at lower confidence when LSP refutes them by writing a different dst).
//     Then INSERT/UPSERT the LSP row.
//
//   - If the incoming row's source matches "comment.%": skip the insert iff
//     a validated lsp.* row already exists for the same triple at
//     confidence ≥ 1.0; otherwise INSERT/UPSERT the comment row.
//
//   - Otherwise (neither prefix): straight UPSERT.
//
// Empty edges is a no-op.
func (t *OverlayTx) UpsertEdgesWithMerge(ctx context.Context, edges []EdgeRow) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("UpsertEdgesWithMerge: nil tx")
	}
	if len(edges) == 0 {
		return nil
	}
	for _, e := range edges {
		isLSP := strings.HasPrefix(e.Source, "lsp.")
		isComment := strings.HasPrefix(e.Source, "comment.")

		if isComment {
			// LSP-already-validated check: if any validated lsp.* row exists
			// for this (src,dst,kind), skip the comment insert silently.
			var dummy int
			err := t.tx.QueryRowContext(ctx, `
				SELECT 1 FROM semantic_live_overlay_edges
				 WHERE repo_id = ? AND src_node_id = ? AND dst_node_id = ?
				   AND edge_kind = ? AND source LIKE 'lsp.%'
				   AND validation_state = 'validated' AND confidence >= 1.0
				   AND status = 'live'
				 LIMIT 1
			`, t.repoID, e.SrcNodeID, e.DstNodeID, e.EdgeKind).Scan(&dummy)
			if err == nil {
				continue // LSP already covers — drop the comment row.
			}
			// sql.ErrNoRows or any other read failure: proceed to insert.
		}

		if isLSP {
			// D-14 refutation: delete any comment row with the same
			// (src_node_id, edge_kind), regardless of dst_node_id. This
			// covers the case where the comment pointed at the wrong dst.
			if _, err := t.tx.ExecContext(ctx, `
				DELETE FROM semantic_live_overlay_edges
				 WHERE repo_id = ? AND src_node_id = ? AND edge_kind = ?
				   AND source LIKE 'comment.%' AND status = 'live'
			`, t.repoID, e.SrcNodeID, e.EdgeKind); err != nil {
				return fmt.Errorf("UpsertEdgesWithMerge(%q, refute comment %d→%d %s): %w",
					t.repoID, e.SrcNodeID, e.DstNodeID, e.EdgeKind, err)
			}
		}

		// UPSERT keyed on (repo_id, src_node_id, dst_node_id, edge_kind).
		// The base schema PRIMARY KEY is (repo_id, edge_id); we synthesize a
		// stable edge_id from the natural triple via xxhash to keep
		// idempotency semantics. Use a deterministic hash so repeated writes
		// for the same triple converge on a single row.
		edgeID := edgeIDForTriple(t.repoID, e.SrcNodeID, e.DstNodeID, e.EdgeKind)
		if _, err := t.tx.ExecContext(ctx, `
			INSERT INTO semantic_live_overlay_edges (
				repo_id, edge_id, src_node_id, dst_node_id, edge_kind,
				status, validation_state, confidence, weight, source,
				fact_json, updated_at, write_epoch
			) VALUES (?, ?, ?, ?, ?, 'live', ?, ?, ?, ?, ?, now(), ?)
			ON CONFLICT (repo_id, edge_id) DO UPDATE SET
				src_node_id      = excluded.src_node_id,
				dst_node_id      = excluded.dst_node_id,
				edge_kind        = excluded.edge_kind,
				status           = excluded.status,
				validation_state = excluded.validation_state,
				confidence       = excluded.confidence,
				weight           = excluded.weight,
				source           = excluded.source,
				fact_json        = excluded.fact_json,
				updated_at       = excluded.updated_at,
				write_epoch      = excluded.write_epoch
		`, t.repoID, edgeID, e.SrcNodeID, e.DstNodeID, e.EdgeKind,
			e.ValidationState, e.Confidence, e.Weight, e.Source,
			edgeFactJSONOrNil(e.FactJSON), t.epoch); err != nil {
			return fmt.Errorf("UpsertEdgesWithMerge(%q, %d→%d %s): %w",
				t.repoID, e.SrcNodeID, e.DstNodeID, e.EdgeKind, err)
		}
		t.bumpPending()
	}
	return nil
}

// DeleteScoresForProjection removes every semantic_graph_scores row for
// (repo_id, score_name=projection) under the active overlay tx. Used by the
// Phase 62 P03 full-recompute path (D-10) which writes a fresh generation
// of rows under the current graph_version and discards every prior
// generation in the same tx so readers never see a half-merged set.
func (t *OverlayTx) DeleteScoresForProjection(ctx context.Context, projection string) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("DeleteScoresForProjection: nil tx")
	}
	if projection == "" {
		return fmt.Errorf("DeleteScoresForProjection: empty projection")
	}
	if _, err := t.tx.ExecContext(ctx, `
		DELETE FROM semantic_graph_scores
		 WHERE repo_id = ? AND score_name = ?
	`, t.repoID, projection); err != nil {
		return fmt.Errorf("DeleteScoresForProjection(%q, %q): %w", t.repoID, projection, err)
	}
	return nil
}

// ClusterSummary is one row written by UpsertClusters. It is the boundary
// type at the cluster ↔ store seam: `internal/semantic/cluster` cannot
// import `internal/semantic/store` without a circular import (the store
// would have to import cluster.Cluster), so the cluster package converts
// its `[]Cluster` into `[]ClusterSummary` at the call site.
//
// MemberCount is the size of the cluster (cardinality of the Members slice
// in cluster.Cluster); we copy it here so UpsertClusters can write it
// without re-iterating the source.
type ClusterSummary struct {
	ID          uint64
	MemberCount int
}

// ClusterMemberRow is one (cluster_id, node_id) tuple written by
// UpsertClusterMembers. Same boundary-type rationale as ClusterSummary.
type ClusterMemberRow struct {
	ClusterID uint64
	NodeID    uint64
}

// UpsertClusters writes one row per cluster to semantic_clusters under the
// active overlay tx. Algorithm name is `projection` (Phase 62 P04 picks the
// projection identifier as the algorithm carrier so re-runs at the same
// projection idempotently overwrite). Empty clusters is a no-op.
//
// The schema (migration 001 §9.10a) has no write_epoch column on
// semantic_clusters; the row is keyed on (repo_id, graph_version,
// cluster_id) and rewritten in full on each detection run via
// DeleteClustersForGraphVersion → UpsertClusters within the same tx.
//
// status is set to "exact" — clustering is computed from the effective
// graph at this graph_version and is precise; D-07's "stale" / "missing"
// closed-enum values do not apply to clusters.
//
// snapshot_id = 0 — clusters are overlay-side state keyed on
// (repo_id, graph_version) (mirrors the Phase 62 P02 score row contract).
func (t *OverlayTx) UpsertClusters(ctx context.Context, projection string, graphVersion uint64, clusters []ClusterSummary) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("UpsertClusters: nil tx")
	}
	if projection == "" {
		return fmt.Errorf("UpsertClusters: empty projection")
	}
	if len(clusters) == 0 {
		return nil
	}
	for _, c := range clusters {
		if _, err := t.tx.ExecContext(ctx, `
			INSERT INTO semantic_clusters (
				repo_id, snapshot_id, graph_version, cluster_id,
				algorithm, label, summary, score, status, computed_at
			) VALUES (?, 0, ?, ?, ?, NULL, NULL, ?, 'exact', now())
			ON CONFLICT (repo_id, graph_version, cluster_id) DO UPDATE SET
				algorithm   = excluded.algorithm,
				score       = excluded.score,
				status      = excluded.status,
				computed_at = excluded.computed_at
		`, t.repoID, graphVersion, c.ID, projection, float64(c.MemberCount)); err != nil {
			return fmt.Errorf("UpsertClusters(%q, %q, cluster=%d): %w",
				t.repoID, projection, c.ID, err)
		}
	}
	return nil
}

// UpsertClusterMembers writes (cluster_id, node_id) tuples to
// semantic_cluster_members under the active overlay tx. Empty rows is a
// no-op.
//
// The schema (migration 001 §9.10b) requires weight DOUBLE NOT NULL; we
// write 1.0 since the weak-component algorithm has no per-member weight
// (every member belongs unconditionally). role is left NULL.
func (t *OverlayTx) UpsertClusterMembers(ctx context.Context, projection string, graphVersion uint64, rows []ClusterMemberRow) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("UpsertClusterMembers: nil tx")
	}
	if len(rows) == 0 {
		return nil
	}
	// projection is reserved for symmetry with UpsertClusters and possible
	// future schema extensions; semantic_cluster_members is keyed on
	// (repo_id, graph_version, cluster_id, node_id) only.
	_ = projection
	for _, r := range rows {
		if _, err := t.tx.ExecContext(ctx, `
			INSERT INTO semantic_cluster_members (
				repo_id, graph_version, cluster_id, node_id, weight, role
			) VALUES (?, ?, ?, ?, 1.0, NULL)
			ON CONFLICT (repo_id, graph_version, cluster_id, node_id) DO UPDATE SET
				weight = excluded.weight,
				role   = excluded.role
		`, t.repoID, graphVersion, r.ClusterID, r.NodeID); err != nil {
			return fmt.Errorf("UpsertClusterMembers(%q, cluster=%d, node=%d): %w",
				t.repoID, r.ClusterID, r.NodeID, err)
		}
	}
	return nil
}

// DeleteClustersForGraphVersion removes every cluster + member row for
// (repo_id, graph_version) under the active overlay tx. Members are
// deleted first to honor any future FK; the order is also safe for
// the current FK-less schema.
//
// Used by Phase 62 P04 RunClusterDetection to discard prior cluster rows
// before writing the fresh generation in the same tx — readers never see a
// half-merged set.
func (t *OverlayTx) DeleteClustersForGraphVersion(ctx context.Context, projection string, graphVersion uint64) error {
	if t == nil || t.tx == nil {
		return fmt.Errorf("DeleteClustersForGraphVersion: nil tx")
	}
	// projection is reserved for symmetry and future per-projection
	// scoping; current schema scopes by (repo_id, graph_version) alone, so
	// every algorithm's clusters at this graph_version are cleared together.
	// This is intentional for the algorithm-only v1 surface.
	_ = projection
	if _, err := t.tx.ExecContext(ctx, `
		DELETE FROM semantic_cluster_members
		 WHERE repo_id = ? AND graph_version = ?
	`, t.repoID, graphVersion); err != nil {
		return fmt.Errorf("DeleteClustersForGraphVersion(%q, gv=%d): members: %w",
			t.repoID, graphVersion, err)
	}
	if _, err := t.tx.ExecContext(ctx, `
		DELETE FROM semantic_clusters
		 WHERE repo_id = ? AND graph_version = ?
	`, t.repoID, graphVersion); err != nil {
		return fmt.Errorf("DeleteClustersForGraphVersion(%q, gv=%d): clusters: %w",
			t.repoID, graphVersion, err)
	}
	return nil
}

// edgeIDForTriple is a deterministic 64-bit hash over (repo_id, src, dst,
// kind) used to synthesize the schema's edge_id PK from the natural merge
// key. FNV-1a is intentionally used to avoid the xxhash dep at the storage
// boundary — collisions across distinct triples are vanishingly improbable
// for the per-workspace cardinality (< 10M edges) and a same-triple write
// always converges on the same edge_id.
func edgeIDForTriple(repoID string, src, dst uint64, kind string) uint64 {
	const (
		offset64 uint64 = 14695981039346656037
		prime64  uint64 = 1099511628211
	)
	h := offset64
	for i := 0; i < len(repoID); i++ {
		h ^= uint64(repoID[i])
		h *= prime64
	}
	for _, v := range [...]uint64{src, dst} {
		for i := 0; i < 8; i++ {
			h ^= (v >> (i * 8)) & 0xFF
			h *= prime64
		}
	}
	for i := 0; i < len(kind); i++ {
		h ^= uint64(kind[i])
		h *= prime64
	}
	// duckdb-go's database/sql binding rejects uint64 values with the high
	// bit set ("uint64 values with high bit set are not supported"). Mask to
	// 63 bits — collision probability across the per-workspace edge surface
	// (< 10M edges) remains vanishingly small.
	return h & 0x7FFFFFFFFFFFFFFF
}

// edgeFactJSONOrNil returns the byte slice unchanged when non-empty, or a
// nil interface so the duckdb-go driver writes a JSON NULL.
func edgeFactJSONOrNil(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return b
}

// BumpGraphVersion advances semantic_live_overlay_meta.graph_version for
// this tx's repo_id and returns the new value. Caller MUST hold the per-
// workspace overlay mutex (via Store.LockOverlayWorkspace) before opening
// the tx; the caller is also responsible for ensuring the meta row exists
// (BeginOverlayTx ensures this on every open).
//
// D-06 invariant: ONLY Engine.ApplyRepair calls this. No other code path
// in the codebase advances graph_version.
func (t *OverlayTx) BumpGraphVersion(ctx context.Context) (uint64, error) {
	if t == nil || t.tx == nil {
		return 0, fmt.Errorf("BumpGraphVersion: nil tx")
	}
	var gv uint64
	if err := t.tx.QueryRowContext(ctx, `
		UPDATE semantic_live_overlay_meta
		   SET graph_version = graph_version + 1,
		       updated_at    = now()
		 WHERE repo_id = ?
		 RETURNING graph_version
	`, t.repoID).Scan(&gv); err != nil {
		return 0, fmt.Errorf("BumpGraphVersion(%q): %w", t.repoID, err)
	}
	return gv, nil
}

// CurrentGraphVersion reads the current graph_version for repoID from
// semantic_live_overlay_meta. No mutex required — DuckDB MVCC reads do not
// race the (single-writer-per-workspace) advance path. Returns (0, nil)
// when the meta row does not exist (the meta row is upserted by
// BeginOverlayTx, so this is the pre-init state).
func (s *Store) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("CurrentGraphVersion: nil store")
	}
	if repoID == "" {
		return 0, fmt.Errorf("CurrentGraphVersion: empty repoID")
	}
	var gv uint64
	err := s.queryRowContext(ctx, `
		SELECT graph_version FROM semantic_live_overlay_meta
		 WHERE repo_id = ?
	`, repoID).Scan(&gv)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("CurrentGraphVersion(%q): %w", repoID, err)
	}
	return gv, nil
}

// CurrentOverlayEpoch reads the current overlay write_epoch for repoID
// from semantic_live_overlay_meta. Mirrors CurrentGraphVersion's
// no-mutex MVCC read pattern.
//
// Phase 63 review CR-03: consumed by the compactor's captureEpoch path
// to read the current epoch BEFORE BeginSnapshot opens its tx. The
// captured value bounds Snapshot.ClearOverlayLE (`WHERE write_epoch <=
// ?`) so rows committed during compaction (write_epoch > capturedEpoch)
// SURVIVE — closes the Phase 60 D-04 CAS contract. The previous
// implementation in compact.captureEpoch returned a `1<<62` sentinel
// that included future epochs and silently deleted concurrent overlay
// writes.
//
// Returns (0, nil) when the meta row does not exist (no overlay writes
// have occurred yet for repoID — the row is upserted by BeginOverlayTx).
func (s *Store) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("CurrentOverlayEpoch: nil store")
	}
	if repoID == "" {
		return 0, fmt.Errorf("CurrentOverlayEpoch: empty repoID")
	}
	var ep uint64
	err := s.queryRowContext(ctx, `
		SELECT current_epoch FROM semantic_live_overlay_meta
		 WHERE repo_id = ?
	`, repoID).Scan(&ep)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("CurrentOverlayEpoch(%q): %w", repoID, err)
	}
	return ep, nil
}

// OverlayChangedPathsSince returns the set of distinct overlay paths whose
// write_epoch is strictly greater than baseEpoch, plus the current overlay
// epoch for repoID. This is the read seam shared by
// index_semantic_graph(mode=incremental) and refresh_semantic_graph per
// Phase 70 CONTEXT.md D1 + D2.
//
// Semantics:
//   - baseEpoch == 0 returns ALL current overlay paths for repoID
//     (cold-start signal: caller has not yet observed any epoch).
//   - When no semantic_live_overlay_meta row exists (fresh workspace, no
//     overlay activity), returns (nil, 0, nil) — mirrors CurrentOverlayEpoch.
//   - An empty paths slice with non-zero currentEpoch means "workspace quiet
//     since baseline"; callers MUST treat this as contract-preserving (no
//     fallback to full-walk).
//
// D-09 invariant: pure read path. No Begin/Commit/Abort/Write/Tx tokens —
// uses the no-tx index idx_overlay_files_write_epoch on
// (repo_id, write_epoch) from migrations.go:527.
func (s *Store) OverlayChangedPathsSince(
	ctx context.Context,
	repoID string,
	baseEpoch uint64,
) (paths []string, currentEpoch uint64, err error) {
	if s == nil || s.db == nil {
		return nil, 0, fmt.Errorf("OverlayChangedPathsSince: nil store")
	}
	if repoID == "" {
		return nil, 0, fmt.Errorf("OverlayChangedPathsSince: empty repoID")
	}

	err = s.queryRowContext(ctx, `
		SELECT current_epoch FROM semantic_live_overlay_meta
		 WHERE repo_id = ?
	`, repoID).Scan(&currentEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("OverlayChangedPathsSince(%q): epoch read: %w", repoID, err)
	}

	rows, err := s.queryContext(ctx, `
		SELECT DISTINCT path FROM semantic_live_overlay_files
		 WHERE repo_id = ? AND write_epoch > ?
	`, repoID, baseEpoch)
	if err != nil {
		return nil, 0, fmt.Errorf("OverlayChangedPathsSince(%q): path query: %w", repoID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, 0, fmt.Errorf("OverlayChangedPathsSince(%q): path scan: %w", repoID, err)
		}
		paths = append(paths, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("OverlayChangedPathsSince(%q): path rows: %w", repoID, err)
	}

	return paths, currentEpoch, nil
}

// LockOverlayWorkspace acquires the per-workspace overlay mutex for repoID
// and returns a release function. The mutex is the SAME one used by
// BeginOverlayTx (D-04 / D-06 invariant: graph_version advance MUST run
// under the same lock that protects current_epoch and the overlay-tx
// pipeline). Callers (Phase 62 Engine.ApplyRepair) take this BEFORE opening
// an OverlayTx so the bump and the score-row writes are serialized.
//
// The release function MUST be invoked exactly once after the corresponding
// tx terminates (Commit or Rollback) — leaking it blocks all subsequent
// same-workspace overlay writes forever.
func (s *Store) LockOverlayWorkspace(repoID string) func() {
	mu := s.overlayLockFor(repoID)
	mu.Lock()
	return mu.Unlock
}
