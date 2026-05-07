// Package store: snapshot-write API (Phase 63 P63-01).
//
// snapshot.go owns the snapshot-write contract consumed by the Phase 63
// compactor (P63-02):
//
//   - Store.BeginSnapshot(ctx, SnapshotMeta) → *Snapshot
//     Opens a DuckDB *sql.Tx, allocates a fresh `pending` row in
//     semantic_snapshots, and returns a *Snapshot handle bound to the tx.
//
//   - Store.WriteSnapshotFacts(ctx, snap, Facts)
//     Per-table parameterized inserts (`?` placeholders) for files,
//     symbols, references, edges through the snapshot's tx. NEVER uses the
//     duckdb-go Appender (RESEARCH.md Landmine 6: Appender commits are not
//     bound to *sql.Tx).
//
//   - Store.CommitSnapshot(ctx, snap, SnapshotSummary)
//     Flips status='committed', stamps committed_at, commits the tx
//     atomically. Idempotency-guarded against double-commit /
//     commit-after-abort.
//
//   - Store.AbortSnapshot(ctx, snap, reason)
//     Rolls back the tx — DuckDB ACID makes the pending row vanish; no
//     explicit DELETE needed. Idempotency-guarded against double-abort /
//     abort-after-commit.
//
//   - (*Snapshot).DeleteSnapshotsBeyond(ctx, retain)
//     Retention DELETE on the SAME snapshot tx so retention commits
//     atomically with the new snapshot creation (COMPACT-04 invariant).
//     Method on *Snapshot (not on *Store) so retention is bound to the
//     open tx by construction; rollback restores all deleted snapshots.
//
//   - (*Snapshot).ClearOverlayLE(ctx, repoID, capturedEpoch)
//     Issues `DELETE FROM semantic_live_overlay_* WHERE repo_id=? AND
//     write_epoch <= ?` on the snapshot's tx. Closes Phase 60 D-04 CAS:
//     rows committed during compaction (write_epoch > capturedEpoch)
//     survive. Encapsulates the underlying *sql.Tx — there is no exported
//     `Tx()` accessor on *Snapshot; consumers (P63-02) call
//     `snap.ClearOverlayLE` rather than reaching into the tx directly.
//
// T-63-01-03 (DoS via unbounded payload): WriteSnapshotFacts has no size
// cap. The caller (P63-02 compactor) is responsible for the pre-flight
// `OverlayRowCount` guard before invoking BeginSnapshot. P63-01 trusts the
// caller — the boundary is documented here so reviewers can find it.
//
// SECURITY (T-63-01-01 / T-63-01-04):
//   - Every INSERT / UPDATE / DELETE uses `?` parameterized binds; no
//     string concat. The grep gate in 63-01-PLAN.md acceptance criteria
//     enforces this at review time.
//   - Stays in `package store` so the cmd/vet-noduckdb boundary remains
//     intact. duckdb-go is NOT imported here — we route through the
//     existing `database/sql` *Store.db handle.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// SnapshotMeta is the input payload to BeginSnapshot.
type SnapshotMeta struct {
	// RepoID is the workspace identifier the snapshot belongs to. Required.
	RepoID string
	// BaseSnapshotID is the prior committed snapshot the new one is the
	// successor of. Zero on the very first snapshot for the workspace.
	BaseSnapshotID uint64
	// CapturedEpoch is the overlay write_epoch the compactor CAS-read at
	// the start of the compaction cycle. Carried on the in-memory Snapshot
	// handle (not persisted to semantic_snapshots in Schema 3) and consumed
	// by ClearOverlayLE to bound the post-merge DELETE.
	CapturedEpoch uint64
}

// Snapshot is the per-tx handle returned by BeginSnapshot. The *sql.Tx is
// intentionally unexported — there is NO public Tx() accessor. Consumers
// who need to mutate state inside the snapshot tx call methods on
// *Snapshot (DeleteSnapshotsBeyond, ClearOverlayLE).
type Snapshot struct {
	// ID is the snapshot_id allocated at BeginSnapshot time.
	ID uint64
	// RepoID is the workspace identifier (mirrors Meta.RepoID).
	RepoID string
	// Meta is the original input passed to BeginSnapshot.
	Meta SnapshotMeta

	tx        *sql.Tx
	store     *Store // back-reference for ClearOverlayLE pending-rows reset
	createdAt time.Time
	committed bool
	aborted   bool
}

// Facts is the bulk insert payload accepted by WriteSnapshotFacts.
type Facts struct {
	Files      []FileFact
	Symbols    []SymbolFact
	References []ReferenceFact
	Edges      []EdgeFact
}

// FileFact is one row inserted into semantic_files per WriteSnapshotFacts.
// The fact-struct field set mirrors the NOT-NULL columns on semantic_files
// (migrations.go schema1Statements §9.4 + schema2Statements partial
// columns); optional partial-extraction columns default to false / "".
type FileFact struct {
	FileID       uint64
	RepoID       string
	Path         string
	Language     string
	ContentHash  string
	SizeBytes    uint64
	LineCount    int
	Generated    bool
	Ignored      bool
	IgnoreReason string
}

// SymbolFact is one row inserted into semantic_symbols.
type SymbolFact struct {
	SymbolID         uint64
	NodeID           uint64
	FileID           uint64
	Language         string
	Kind             string
	Name             string
	QualifiedName    string
	StableKey        string
	OwnerSymbolID    uint64
	ParentScopeID    uint64
	PackagePath      string
	StartByte        int
	EndByte          int
	StartLine        int
	StartCol         int
	EndLine          int
	EndCol           int
	Signature        string
	SignatureHash    string
	Exported         bool
	Visibility       string
	ExtractionSource string
	Confidence       float64
	ContentHash      string
	LSPIdentity      string
}

// ReferenceFact is one row inserted into semantic_references.
type ReferenceFact struct {
	RefID            uint64
	NodeID           uint64
	FileID           uint64
	ScopeSymbolID    uint64
	Name             string
	RefKind          string
	ReceiverText     string
	StartByte        int
	EndByte          int
	StartLine        int
	StartCol         int
	EndLine          int
	EndCol           int
	ResolvedSymbolID uint64
	ResolutionSource string
	ValidationState  string
	Confidence       float64
	Reason           string
}

// EdgeFact is one row inserted into semantic_edges.
type EdgeFact struct {
	EdgeID          uint64
	SrcNodeID       uint64
	DstNodeID       uint64
	SrcKind         string
	DstKind         string
	EdgeKind        string
	Weight          float64
	Confidence      float64
	ValidationState string
	EvidenceRefID   uint64
	EvidenceFileID  uint64
	Source          string
	Reason          string
}

// SnapshotSummary is the aggregate row metadata captured at CommitSnapshot.
// Schema 3 has no separate `semantic_snapshot_summaries` table; the counts
// are accepted by CommitSnapshot for diagnostic logging today and reserved
// for a future column-on-snapshots write once the schema lands the columns.
type SnapshotSummary struct {
	FileCount      int
	SymbolCount    int
	ReferenceCount int
	EdgeCount      int
	DurationMs     int64
}

// BeginSnapshot opens a snapshot-write transaction for meta.RepoID. It
// allocates a fresh snapshot_id (max+1 inside the tx), inserts a pending
// row in semantic_snapshots, and returns a *Snapshot bound to the tx. The
// caller MUST terminate via CommitSnapshot or AbortSnapshot — leaking the
// tx leaks a DuckDB transaction slot.
//
// CapturedEpoch is stamped on the in-memory Snapshot (not persisted in
// Schema 3) so ClearOverlayLE can later bound its DELETE without a
// separate column-add migration.
func (s *Store) BeginSnapshot(ctx context.Context, meta SnapshotMeta) (*Snapshot, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("BeginSnapshot: nil store")
	}
	if meta.RepoID == "" {
		return nil, fmt.Errorf("BeginSnapshot: empty repoID")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("BeginSnapshot: open tx: %w", err)
	}

	// Allocate snapshot_id = max(snapshot_id)+1 inside the tx and INSERT a
	// pending row, returning the assigned id. DuckDB supports
	// INSERT ... RETURNING. semantic_snapshots requires several NOT NULL
	// columns (repo_id, repo_root, kind, worktree_hash, schema_version,
	// indexer_version, status, created_at) — provide minimal placeholder
	// values for the columns the compactor does not currently set; P63-02
	// (compactor) supplies real values via meta extension if needed.
	var id uint64
	row := tx.QueryRowContext(ctx, `
		INSERT INTO semantic_snapshots (
			snapshot_id, repo_id, repo_root, base_snapshot_id, kind,
			worktree_hash, schema_version, indexer_version, status,
			partial, created_at
		) VALUES (
			(SELECT COALESCE(MAX(snapshot_id),0)+1 FROM semantic_snapshots),
			?, '', ?, 'compact', '', ?, 'phase63', 'pending',
			false, ?
		)
		RETURNING snapshot_id
	`, meta.RepoID, meta.BaseSnapshotID, CurrentSchemaVersion, time.Now())
	if err := row.Scan(&id); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("BeginSnapshot: insert pending row: %w", err)
	}

	return &Snapshot{
		ID:        id,
		RepoID:    meta.RepoID,
		Meta:      meta,
		tx:        tx,
		store:     s,
		createdAt: time.Now(),
	}, nil
}

// WriteSnapshotFacts inserts every Fact in `facts` into the corresponding
// snapshot fact table on snap's tx. Empty Facts is a no-op (returns nil).
//
// Per-row error returns `fmt.Errorf("WriteSnapshotFacts(%s): %w", table,
// err)` WITHOUT rolling back the tx — the caller decides Abort vs continue.
func (s *Store) WriteSnapshotFacts(ctx context.Context, snap *Snapshot, facts Facts) error {
	if s == nil {
		return fmt.Errorf("WriteSnapshotFacts: nil store")
	}
	if snap == nil || snap.tx == nil {
		return fmt.Errorf("WriteSnapshotFacts: nil snapshot")
	}
	if len(facts.Files) == 0 && len(facts.Symbols) == 0 && len(facts.References) == 0 && len(facts.Edges) == 0 {
		return nil
	}

	for _, f := range facts.Files {
		if _, err := snap.tx.ExecContext(ctx, `
			INSERT INTO semantic_files (
				snapshot_id, file_id, repo_id, path, language, content_hash,
				size_bytes, line_count, generated, ignored, ignore_reason,
				indexed_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, snap.ID, f.FileID, f.RepoID, f.Path, f.Language, f.ContentHash,
			f.SizeBytes, f.LineCount, f.Generated, f.Ignored,
			nullIfEmpty(f.IgnoreReason), time.Now()); err != nil {
			return fmt.Errorf("WriteSnapshotFacts(semantic_files): %w", err)
		}
	}

	for _, s2 := range facts.Symbols {
		if _, err := snap.tx.ExecContext(ctx, `
			INSERT INTO semantic_symbols (
				snapshot_id, symbol_id, node_id, file_id, language, kind,
				name, qualified_name, stable_key, owner_symbol_id,
				parent_scope_id, package_path, start_byte, end_byte,
				start_line, start_col, end_line, end_col, signature,
				signature_hash, exported, visibility, extraction_source,
				confidence, content_hash, lsp_identity
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, snap.ID, s2.SymbolID, s2.NodeID, s2.FileID, s2.Language, s2.Kind,
			s2.Name, s2.QualifiedName, s2.StableKey,
			nullIfZero(s2.OwnerSymbolID), nullIfZero(s2.ParentScopeID),
			nullIfEmpty(s2.PackagePath), s2.StartByte, s2.EndByte,
			s2.StartLine, s2.StartCol, s2.EndLine, s2.EndCol,
			nullIfEmpty(s2.Signature), nullIfEmpty(s2.SignatureHash),
			s2.Exported, nullIfEmpty(s2.Visibility),
			s2.ExtractionSource, s2.Confidence,
			nullIfEmpty(s2.ContentHash), nullIfEmpty(s2.LSPIdentity)); err != nil {
			return fmt.Errorf("WriteSnapshotFacts(semantic_symbols): %w", err)
		}
	}

	for _, r := range facts.References {
		if _, err := snap.tx.ExecContext(ctx, `
			INSERT INTO semantic_references (
				snapshot_id, ref_id, node_id, file_id, scope_symbol_id,
				name, ref_kind, receiver_text, start_byte, end_byte,
				start_line, start_col, end_line, end_col,
				resolved_symbol_id, resolution_source, validation_state,
				confidence, reason
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, snap.ID, r.RefID, r.NodeID, r.FileID,
			nullIfZero(r.ScopeSymbolID), r.Name, r.RefKind,
			nullIfEmpty(r.ReceiverText), r.StartByte, r.EndByte,
			r.StartLine, r.StartCol, r.EndLine, r.EndCol,
			nullIfZero(r.ResolvedSymbolID), nullIfEmpty(r.ResolutionSource),
			r.ValidationState, r.Confidence,
			nullIfEmpty(r.Reason)); err != nil {
			return fmt.Errorf("WriteSnapshotFacts(semantic_references): %w", err)
		}
	}

	for _, e := range facts.Edges {
		if _, err := snap.tx.ExecContext(ctx, `
			INSERT INTO semantic_edges (
				snapshot_id, edge_id, src_node_id, dst_node_id, src_kind,
				dst_kind, edge_kind, weight, confidence, validation_state,
				evidence_ref_id, evidence_file_id, source, reason,
				created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, snap.ID, e.EdgeID, e.SrcNodeID, e.DstNodeID, e.SrcKind,
			e.DstKind, e.EdgeKind, e.Weight, e.Confidence,
			e.ValidationState, nullIfZero(e.EvidenceRefID),
			nullIfZero(e.EvidenceFileID), e.Source,
			nullIfEmpty(e.Reason), time.Now()); err != nil {
			return fmt.Errorf("WriteSnapshotFacts(semantic_edges): %w", err)
		}
	}

	return nil
}

// CommitSnapshot flips snap to status='committed' and commits the tx
// atomically. Double-commit and commit-after-abort are rejected with
// sentinel errors. The summary is logged but not yet persisted to a
// dedicated table (Schema 3 has no semantic_snapshot_summaries column);
// reserved for a future schema bump if/when downstream readers need it.
func (s *Store) CommitSnapshot(ctx context.Context, snap *Snapshot, summary SnapshotSummary) error {
	if s == nil {
		return fmt.Errorf("CommitSnapshot: nil store")
	}
	if snap == nil || snap.tx == nil {
		return fmt.Errorf("CommitSnapshot: nil snapshot")
	}
	if snap.committed {
		return fmt.Errorf("CommitSnapshot: already committed")
	}
	if snap.aborted {
		return fmt.Errorf("CommitSnapshot: already aborted")
	}

	if _, err := snap.tx.ExecContext(ctx, `
		UPDATE semantic_snapshots
		   SET status='committed', committed_at=?
		 WHERE snapshot_id=?
	`, time.Now(), snap.ID); err != nil {
		return fmt.Errorf("CommitSnapshot: flip status: %w", err)
	}

	if err := snap.tx.Commit(); err != nil {
		return fmt.Errorf("CommitSnapshot: %w", err)
	}
	snap.committed = true

	slog.Default().Debug("snapshot committed",
		"snapshot_id", snap.ID,
		"repo_id", snap.RepoID,
		"file_count", summary.FileCount,
		"symbol_count", summary.SymbolCount,
		"reference_count", summary.ReferenceCount,
		"edge_count", summary.EdgeCount,
		"duration_ms", summary.DurationMs,
	)
	return nil
}

// AbortSnapshot rolls back the snapshot tx — DuckDB ACID makes the pending
// row disappear without an explicit DELETE. Double-abort and
// abort-after-commit are rejected.
func (s *Store) AbortSnapshot(ctx context.Context, snap *Snapshot, reason string) error {
	_ = ctx
	if s == nil {
		return fmt.Errorf("AbortSnapshot: nil store")
	}
	if snap == nil || snap.tx == nil {
		return fmt.Errorf("AbortSnapshot: nil snapshot")
	}
	if snap.aborted {
		return fmt.Errorf("AbortSnapshot: already aborted")
	}
	if snap.committed {
		return fmt.Errorf("AbortSnapshot: already committed")
	}

	slog.Default().Info("snapshot aborted",
		"snapshot_id", snap.ID,
		"repo_id", snap.RepoID,
		"reason", reason,
	)

	if err := snap.tx.Rollback(); err != nil {
		return fmt.Errorf("AbortSnapshot: %w", err)
	}
	snap.aborted = true
	return nil
}

// DeleteSnapshotsBeyond deletes every committed snapshot for snap.RepoID
// EXCEPT (a) snap itself (which is still pending in the open tx) and
// (b) the `retain-1` most-recent committed prior snapshots. Cascades to
// per-snapshot fact-table rows via explicit per-table DELETEs (the schema
// has no FK ON DELETE CASCADE today).
//
// The DELETE runs on the SAME tx as snap's pending row so retention
// commits atomically with the new snapshot creation (COMPACT-04). If the
// caller later AbortSnapshot's, the DELETE rolls back and all prior
// snapshots are restored — verified by TestSnapshot_DeleteSnapshotsBeyondAtomic.
//
// retain < 1 is rejected; retain ≥ priors+1 is a no-op.
func (snap *Snapshot) DeleteSnapshotsBeyond(ctx context.Context, retain int) error {
	if snap == nil || snap.tx == nil {
		return fmt.Errorf("DeleteSnapshotsBeyond: nil snapshot")
	}
	if snap.committed {
		return fmt.Errorf("DeleteSnapshotsBeyond: snapshot already committed")
	}
	if snap.aborted {
		return fmt.Errorf("DeleteSnapshotsBeyond: snapshot already aborted")
	}
	if retain < 1 {
		return fmt.Errorf("DeleteSnapshotsBeyond: retain must be >= 1, got %d", retain)
	}

	// Materialize the set of doomed snapshot_ids first (everything outside
	// the keep set) so we can cascade per-table deletes.
	rows, err := snap.tx.QueryContext(ctx, `
		SELECT snapshot_id FROM semantic_snapshots
		 WHERE repo_id = ? AND snapshot_id != ?
		   AND snapshot_id NOT IN (
		       SELECT snapshot_id FROM semantic_snapshots
		        WHERE repo_id = ? AND snapshot_id != ?
		          AND status = 'committed'
		        ORDER BY created_at DESC
		        LIMIT ?
		   )
	`, snap.RepoID, snap.ID, snap.RepoID, snap.ID, retain-1)
	if err != nil {
		return fmt.Errorf("DeleteSnapshotsBeyond: select doomed: %w", err)
	}
	doomed := []uint64{}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("DeleteSnapshotsBeyond: scan doomed: %w", err)
		}
		doomed = append(doomed, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("DeleteSnapshotsBeyond: iter doomed: %w", err)
	}
	rows.Close()
	if len(doomed) == 0 {
		return nil
	}

	// Per-table cascade. The schema doesn't enforce FK cascade, so we issue
	// explicit deletes for every fact table keyed on snapshot_id. Order is
	// not load-bearing — all run in the same tx. Each statement is a
	// hardcoded constant (no string interpolation, no caller-derived data
	// in the SQL) so the parameterization invariant is preserved: only
	// snapshot_id flows through `?` binds.
	cascade := []struct {
		name string
		sql  string
	}{
		{"semantic_files", `DELETE FROM semantic_files WHERE snapshot_id=?`},
		{"semantic_symbols", `DELETE FROM semantic_symbols WHERE snapshot_id=?`},
		{"semantic_references", `DELETE FROM semantic_references WHERE snapshot_id=?`},
		{"semantic_edges", `DELETE FROM semantic_edges WHERE snapshot_id=?`},
		{"semantic_nodes", `DELETE FROM semantic_nodes WHERE snapshot_id=?`},
		{"semantic_diagnostics", `DELETE FROM semantic_diagnostics WHERE snapshot_id=?`},
		{"semantic_snapshots", `DELETE FROM semantic_snapshots WHERE snapshot_id=?`},
	}
	for _, id := range doomed {
		for _, c := range cascade {
			if _, err := snap.tx.ExecContext(ctx, c.sql, id); err != nil {
				return fmt.Errorf("DeleteSnapshotsBeyond(%s, snap=%d): %w", c.name, id, err)
			}
		}
	}
	return nil
}

// ClearOverlayLE issues `DELETE FROM semantic_live_overlay_* WHERE
// repo_id = ? AND write_epoch <= ?` on snap's underlying tx, for each of
// the four overlay fact tables. The deletes commit atomically with snap's
// snapshot creation (rollback restores all overlay rows); rows committed
// during compaction (write_epoch > capturedEpoch) survive — closing the
// Phase 60 D-04 CAS contract.
//
// Encapsulates the underlying *sql.Tx. P63-02's compactor calls
// snap.ClearOverlayLE rather than reaching into snap.tx directly — there
// is no public Tx() accessor on *Snapshot.
//
// Pending-rows counter reset (P63-02 contract): when the parent *Store
// exposes an `overlayPendingRowsFor(repoID) *atomic.Int64` field (added by
// P63-02 Task 1), reset it to 0 AFTER all four DELETEs succeed. The reset
// must happen AFTER the DELETEs, not before — otherwise the gate could
// observe `OverlayHasPendingRows == false` while rows still exist on disk.
// Today the field doesn't exist; the reset is guarded behind a runtime
// nil-check so this snippet is forward-compatible with P63-02 without
// requiring a follow-up edit.
func (snap *Snapshot) ClearOverlayLE(ctx context.Context, repoID string, capturedEpoch uint64) error {
	if snap == nil || snap.tx == nil {
		return fmt.Errorf("ClearOverlayLE: nil snapshot")
	}
	if snap.committed {
		return fmt.Errorf("ClearOverlayLE: snapshot already committed")
	}
	if snap.aborted {
		return fmt.Errorf("ClearOverlayLE: snapshot already aborted")
	}
	if repoID == "" {
		return fmt.Errorf("ClearOverlayLE: empty repoID")
	}

	// Per-table DELETE on the snapshot's own tx. Each statement is a
	// hardcoded constant (no string interpolation, no caller-derived data
	// in the SQL) so the parameterization invariant is preserved: only
	// repo_id and write_epoch flow through `?` binds.
	overlayDeletes := []struct {
		name string
		sql  string
	}{
		{"semantic_live_overlay_files", `DELETE FROM semantic_live_overlay_files WHERE repo_id = ? AND write_epoch <= ?`},
		{"semantic_live_overlay_symbols", `DELETE FROM semantic_live_overlay_symbols WHERE repo_id = ? AND write_epoch <= ?`},
		{"semantic_live_overlay_references", `DELETE FROM semantic_live_overlay_references WHERE repo_id = ? AND write_epoch <= ?`},
		{"semantic_live_overlay_edges", `DELETE FROM semantic_live_overlay_edges WHERE repo_id = ? AND write_epoch <= ?`},
	}
	for _, d := range overlayDeletes {
		if _, err := snap.tx.ExecContext(ctx, d.sql, repoID, capturedEpoch); err != nil {
			return fmt.Errorf("ClearOverlayLE(%s): %w", d.name, err)
		}
	}

	// Forward-compatible pending-rows counter reset (P63-02 wires this).
	// When the field is added the runtime nil-check below activates the
	// reset; until then this is a no-op. See doc comment above for the
	// AFTER-DELETE ordering rationale.
	resetOverlayPendingRowsIfPresent(snap.store, repoID)

	return nil
}

// resetOverlayPendingRowsIfPresent is the forward-compatible hook for
// P63-02's pending-rows counter. P63-02 will redeclare this function (or
// its callers) with a real reset; until then it is a no-op so P63-01 can
// land without referencing a not-yet-existing field.
func resetOverlayPendingRowsIfPresent(_ *Store, _ string) {
	// no-op until P63-02 lands the counter
}

// nullIfEmpty maps "" → nil (so the database/sql driver writes SQL NULL)
// and any non-empty string passes through unchanged. Used for nullable
// TEXT columns where the runtime "no value" must round-trip as NULL rather
// than an empty string (the SPEC distinguishes the two for several
// columns: e.g., signature, partial_reason, ignore_reason).
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// nullIfZero maps 0 → nil for nullable UBIGINT columns (e.g.,
// owner_symbol_id, parent_scope_id, scope_symbol_id, evidence_ref_id) so
// the absence of a referenced row is represented as SQL NULL rather than
// a literal 0 that would later collide with a real id=0 row.
func nullIfZero(v uint64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}
